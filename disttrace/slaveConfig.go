package disttrace

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"time"

	valid "github.com/asaskevich/govalidator"
)

type pollerConfig struct {
	MasterHost string
	MasterPort string
	Slave      Slave
}

// ConfigPollerProcRunning mutex for graceful shutdown
var ConfigPollerProcRunning = make(chan bool, 1)

// ConfigPoller runs as process, periodically polls slave configuration on master
func ConfigPoller(masterHost string, masterPort string, slave Slave, ppCfg **SlaveConfig) {

	// lock mutex
	ConfigPollerProcRunning <- true

	// init vars
	var nextTime time.Time
	pollerCfg := pollerConfig{MasterHost: masterHost, MasterPort: masterPort, Slave: slave}

	// infinite loop
	log.Info("ConfigPoller: Start...")
	for {
		// check if we need to exit
		if CheckForQuit() {
			log.Warn("ConfigPoller: Received exit signal, bye.")
			<-ConfigPollerProcRunning
			return
		}

		// is it time to run?
		if nextTime.Before(time.Now()) {
			log.Debug("ConfigPoller: Checking for new configuration...")

			pNewCfg := new(SlaveConfig)
			ppNewCfg := &pNewCfg
			var err error

			err = getConfigFromMaster(pollerCfg.MasterHost, pollerCfg.MasterPort, pollerCfg.Slave, ppNewCfg)

			if err != nil {
				log.Warn("ConfigPoller: Couldn't get configuration")

			} else {
				newCfgJSON, _ := json.Marshal(**ppNewCfg)
				oldCfgJSON, _ := json.Marshal(**ppCfg)

				if string(newCfgJSON) != string(oldCfgJSON) {
					// config changed
					log.Infof("ConfigPoller: Application configuration changed, applying new configuration and going to sleep...")
					pCfg := *ppCfg
					*pCfg = **ppNewCfg

				} else {
					// no config change
					log.Debug("ConfigPoller: Application configuration on didn't change, going to sleep...")
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

// getConfigFromMaster fetches the slave's configuration from the master server
func getConfigFromMaster(masterHost string, masterPort string, slave Slave, ppCfg **SlaveConfig) error {

	var slaveJSON, _ = json.Marshal(slave)
	var masterURL = "http://" + masterHost + ":" + masterPort + "/slave/config"

	if !valid.IsURL(masterURL) {
		log.Warnf("getConfigFromMaster: Cant' get config, master URL '%v' is invalid", masterURL)
		return errors.New("Can't get config, master URL is invalid")
	}

	var pCfg = *ppCfg
	var newCfg = SlaveConfig{}
	*pCfg = newCfg

	log.Debug("getConfigFromMaster: Attempting to read configuration from URL: ", masterURL)
	var httpClient = &http.Client{
		Timeout: time.Second * 10,
	}

	// download configuration file from master
	httpResp, err := httpClient.Post(masterURL, "application/json", bytes.NewBuffer(slaveJSON))
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
		*pCfg = newCfg
		return errors.New("Can't parse response body")
	}

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
	*pCfg = newCfg
	return nil
}

// WaitForValidConfig blocks until ppCfg holds a valid config
func WaitForValidConfig(name string, ppCfg **SlaveConfig) {

	// TODO validate properly!
	for {
		time.Sleep(1 * time.Second)
		tempCfg := **ppCfg

		if len(tempCfg.Targets) > 0 {
			return
		}

		log.Debugf("WaitForValidConfig: proc %v is waiting for valid config...", name)

		if CheckForQuit() {
			log.Fatal("WaitForValidConfig: Interrupted while waiting for valid config, exiting...")
		}
	}
}
