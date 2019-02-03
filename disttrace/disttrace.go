package disttrace

import (
	"bytes"
	"encoding/json"
	"errors"
	valid "github.com/asaskevich/govalidator"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"time"
)

// ConfigPollerProcRunning mutex for graceful shutdown
var ConfigPollerProcRunning = make(chan bool, 1)

// ConfigPoller checks periodically, if a new configuration is available on master server
func ConfigPoller(doExit chan bool, masterURL string, slaveCreds SlaveCredentials, ppCfg **SlaveConfig) {

	// lock mutex
	ConfigPollerProcRunning <- true

	// init vars
	var nextTime time.Time

	// infinite loop
	log.Info("configPoller: Start...")
	for {
		// check if we need to exit
		select {
		case <-doExit:
			log.Warn("configPoller: Received exit signal, bye.")
			<-ConfigPollerProcRunning
			return
		default:
		}

		// is it time to run?
		if nextTime.Before(time.Now()) {
			log.Debug("configPoller: Checking for new configuration on master server...")

			pNewCfg := new(SlaveConfig)
			ppNewCfg := &pNewCfg
			err := GetConfigFromMaster(masterURL, slaveCreds, ppNewCfg)
			if err != nil {
				log.Warn("configPoller: Couldn't get current configuration from master server")

			} else {
				newCfgJSON, _ := json.Marshal(**ppNewCfg)
				oldCfgJSON, _ := json.Marshal(**ppCfg)

				if string(newCfgJSON) != string(oldCfgJSON) {
					// config changed
					newCfg := **ppNewCfg
					log.Infof("configPoller: Configuration on master changed, applying new configuration (%v targets) and going to sleep...", len(newCfg.Targets))
					pCfg := *ppCfg
					*pCfg = **ppNewCfg

				} else {
					// no config change
					log.Debug("configPoller: Configuration on master didn't change, going to sleep...")
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

// GetConfigFromMaster fetches the slave's configuration from the master server
func GetConfigFromMaster(masterURL string, slaveCreds SlaveCredentials, ppCfg **SlaveConfig) error {

	var slaveCredsJSON, _ = json.Marshal(slaveCreds)

	var newCfg = SlaveConfig{}
	var pCfg = *ppCfg

	log.Debug("getConfigFromMaster: Attempting to read configuration from URL: ", masterURL)
	var httpClient = &http.Client{
		Timeout: time.Second * 10,
	}

	// download configuration file from master
	httpResp, err := httpClient.Post(masterURL, "application/json", bytes.NewBuffer(slaveCredsJSON))
	if err != nil {
		log.Warn("getConfigFromMaster: Error sending HTTP Request: ", err)
		*pCfg = newCfg
		return errors.New("Error sending HTTP Request")
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode >= 400 {
		log.Warn("getConfigFromMaster: Error getting configuration, received HTTP status: ", httpResp.Status)
		*pCfg = newCfg
		return errors.New("Error getting configuration, received HTTP error")
	}

	// read response from master
	httpRespBody, err := ioutil.ReadAll(httpResp.Body)
	if err != nil {
		log.Warn("getConfigFromMaster: Can't read response body: ", err)
		*pCfg = newCfg
		return errors.New("Can't read response body")
	}

	// parse result
	err = json.Unmarshal(httpRespBody, &newCfg)
	if err != nil {
		log.Warnf("getConfigFromMaster: Can't parse body '%v' (first 100 char), Error: %v", string(httpRespBody)[:100], err)
		*pCfg = newCfg
		return errors.New("Can't parse response body")
	}

	// TODO make custom validator, write tests
	// validate config
	success, err := valid.ValidateStruct(newCfg)
	if !success {
		log.Warn("getConfigFromMaster: Validation of received config failed. Error: ", err)
		*pCfg = newCfg
		return errors.New("Validation failed")
	}

	// validate targets
	for i, target := range newCfg.Targets {
		success, err := valid.ValidateStruct(target)
		if !success {
			log.Warnf("getConfigFromMaster: Validation of target '%v' in received config failed. Error: %v", i, err)
			*pCfg = newCfg
			return errors.New("Validation failed")
		}
	}

	log.Debug("getConfigFromMaster: Got config from master, number of configured targets: ", len(newCfg.Targets))
	*pCfg = newCfg
	return nil
}
