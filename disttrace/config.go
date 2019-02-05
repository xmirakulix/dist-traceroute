package disttrace

import (
	"bytes"
	"encoding/json"
	"errors"
	valid "github.com/asaskevich/govalidator"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

type pollerConfig struct {
	Type string
	*masterPollerConfig
	*slavePollerConfig
}

type masterPollerConfig struct {
	FileName string
}

type slavePollerConfig struct {
	MasterHost string
	MasterPort string
	SlaveCreds SlaveCredentials
}

// ConfigPollerProcRunning mutex for graceful shutdown
var ConfigPollerProcRunning = make(chan bool, 1)

// MasterConfigPoller runs as process, checks master configuration file(s) periodically
func MasterConfigPoller(fileName string, ppCfg **GenericConfig) {

	mpc := masterPollerConfig{FileName: fileName}
	pc := pollerConfig{Type: "master", masterPollerConfig: &mpc}

	configPoller(pc, ppCfg)
}

// SlaveConfigPoller runs as process, periodically polls slave configuration on master
func SlaveConfigPoller(masterHost string, masterPort string, slaveCreds SlaveCredentials, ppCfg **GenericConfig) {

	spc := slavePollerConfig{MasterHost: masterHost, MasterPort: masterPort, SlaveCreds: slaveCreds}
	pc := pollerConfig{Type: "slave", slavePollerConfig: &spc}
	configPoller(pc, ppCfg)

}

// configPoller implements generic poller for slave and master
func configPoller(pollerCfg pollerConfig, ppCfg **GenericConfig) {

	// lock mutex
	ConfigPollerProcRunning <- true

	// init vars
	var nextTime time.Time

	// infinite loop
	log.Info("configPoller: Start...")
	for {
		// check if we need to exit
		if CheckForQuit() {
			log.Warn("configPoller: Received exit signal, bye.")
			<-ConfigPollerProcRunning
			return
		}

		// is it time to run?
		if nextTime.Before(time.Now()) {
			log.Debug("configPoller: Checking for new configuration server...")

			pNewCfg := new(GenericConfig)
			pNewCfg.SlaveConfig = new(SlaveConfig)
			pNewCfg.MasterConfig = new(MasterConfig)
			ppNewCfg := &pNewCfg
			var err error

			if pollerCfg.Type == "slave" {
				err = getConfigFromMaster(pollerCfg.MasterHost, pollerCfg.MasterPort, pollerCfg.SlaveCreds, ppNewCfg)
			} else {
				err = getConfigFromFile(pollerCfg.FileName, ppNewCfg)
			}

			if err != nil {
				log.Warn("configPoller: Couldn't get configuration")

			} else {
				newCfgJSON, _ := json.Marshal(**ppNewCfg)
				oldCfgJSON, _ := json.Marshal(**ppCfg)

				if string(newCfgJSON) != string(oldCfgJSON) {
					// config changed
					log.Infof("configPoller: Configuration changed, applying new configuration and going to sleep...")
					pCfg := *ppCfg
					*pCfg = **ppNewCfg

				} else {
					// no config change
					log.Debug("configPoller: Configuration on didn't change, going to sleep...")
				}
			}

			// run again on next full minute
			nextTime = time.Now().Truncate(time.Minute)
			nextTime = nextTime.Add(time.Minute)
		}

		// zzz...
		time.Sleep(1 * time.Second)
	}
}

// getConfigFromFile reads master configuration from file
func getConfigFromFile(fileName string, ppCfg **GenericConfig) error {

	// create new empty config
	var newCfg = MasterConfig{}
	var pCfg = *ppCfg
	*pCfg.MasterConfig = newCfg

	// open file
	jsonFile, err := os.Open(fileName)
	if err != nil {
		log.Fatalf("getConfigFromFile: Couldn't open file '%v', Error: %v", fileName, err)
	}
	defer jsonFile.Close()

	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		log.Fatalf("getConfigFromFile: Couldn't read from file '%v', Error: %v", fileName, err)
	}

	err = json.Unmarshal(byteValue, &newCfg)
	if err != nil {
		log.Fatalf("getConfigFromFile: Couldn't unmarshal content of file '%v', Error: %v", fileName, err)
	}

	log.Debugf("getConfigFromFile: Got config from file '%v', number of configured slaves: %v", fileName, len(newCfg.Slaves))
	*pCfg.MasterConfig = newCfg
	return nil
}

// getConfigFromMaster fetches the slave's configuration from the master server
func getConfigFromMaster(masterHost string, masterPort string, slaveCreds SlaveCredentials, ppCfg **GenericConfig) error {

	var slaveCredsJSON, _ = json.Marshal(slaveCreds)
	var masterURL = "http://" + masterHost + ":" + masterPort + "/config/"
	// TODO validate URL

	var newCfg = SlaveConfig{}
	var pCfg = *ppCfg
	*pCfg.SlaveConfig = newCfg

	log.Debug("getConfigFromMaster: Attempting to read configuration from URL: ", masterURL)
	var httpClient = &http.Client{
		Timeout: time.Second * 10,
	}

	// download configuration file from master
	httpResp, err := httpClient.Post(masterURL, "application/json", bytes.NewBuffer(slaveCredsJSON))
	if err != nil {
		log.Warn("getConfigFromMaster: Error sending HTTP Request: ", err)
		return errors.New("Error sending HTTP Request")
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode >= 400 {
		log.Warn("getConfigFromMaster: Error getting configuration, received HTTP status: ", httpResp.Status)
		return errors.New("Error getting configuration, received HTTP error")
	}

	// read response from master
	httpRespBody, err := ioutil.ReadAll(httpResp.Body)
	if err != nil {
		log.Warn("getConfigFromMaster: Can't read response body: ", err)
		return errors.New("Can't read response body")
	}

	// parse result
	err = json.Unmarshal(httpRespBody, &newCfg)
	if err != nil {
		log.Warnf("getConfigFromMaster: Can't parse body '%v' (first 100 char), Error: %v", string(httpRespBody)[:100], err)
		*pCfg.SlaveConfig = newCfg
		return errors.New("Can't parse response body")
	}

	// TODO make custom validator including targets, write tests
	// validate config
	success, err := valid.ValidateStruct(newCfg)
	if !success {
		log.Warn("getConfigFromMaster: Validation of received config failed. Error: ", err)
		return errors.New("Validation failed")
	}

	// validate targets
	for i, target := range newCfg.Targets {
		success, err := valid.ValidateStruct(target)
		if !success {
			log.Warnf("getConfigFromMaster: Validation of target '%v' in received config failed. Error: %v", i, err)
			return errors.New("Validation failed")
		}
	}

	// fill in master configuration, comes from cmdline arguments
	newCfg.MasterHost = masterHost
	newCfg.MasterPort = masterPort

	log.Debug("getConfigFromMaster: Got config from master, number of configured targets: ", len(newCfg.Targets))
	*pCfg.SlaveConfig = newCfg
	return nil
}

// WaitForValidConfig blocks until ppCfg holds a valid config
func WaitForValidConfig(name string, role string, ppCfg **GenericConfig) {

	// TODO validate properly!
	for {
		time.Sleep(1 * time.Second)
		tempCfg := **ppCfg
		switch role {
		case "master":
			if len(tempCfg.Slaves) > 0 {
				return
			}
		case "slave":
			if len(tempCfg.Targets) > 0 {
				return
			}
		}
		log.Debugf("WaitForValidConfig: proc %v is waiting for valid config...", name)

		if CheckForQuit() {
			log.Fatal("WaitForValidConfig: Interrupted while waiting for valid config, exiting...")
		}
	}
}
