package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	tracert "github.com/aeden/traceroute"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/xmirakulix/dist-traceroute/disttrace"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var txProcRunning = make(chan bool, 1)
var tracePollerProcRunning = make(chan bool, 1)
var configPollerProcRunning = make(chan bool, 1)

// TODO split logging in normal and debug

// getConfigFromMaster fetches the slave's configuration from the master server
func getConfigFromMaster(masterURL string, ppCfg **disttrace.SlaveConfig) error {

	newCfg := disttrace.SlaveConfig{}
	pCfg := *ppCfg

	log.Debug("getConfigFromMaster: Attempting to read configuration from URL: ", masterURL)
	var httpClient = &http.Client{
		Timeout: time.Second * 10,
	}

	// download configuration file from master
	httpResp, err := httpClient.Get(masterURL)
	if err != nil {
		log.Warn("getConfigFromMaster: Error sending HTTP Request:", err)
		*pCfg = newCfg
		return errors.New("Error sending HTTP Request")
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode >= 400 {
		log.Warn("getConfigFromMaster: Error getting configuration, received HTTP status:", httpResp.Status)
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
		log.Warnf("getConfigFromMaster: Can't parse body '%v' (first 100 char), Error: %v\n", string(httpRespBody)[:100], err)
		*pCfg = newCfg
		return errors.New("Can't parse response body")
	}

	log.Debug("getConfigFromMaster: Got config from master, number of configured targets:", len(newCfg.Targets))
	*pCfg = newCfg
	return nil
}

// configPoller checks periodically, if a new configuration is available on master server
func configPoller(doExit chan bool, masterURL string, ppCfg **disttrace.SlaveConfig) {

	// lock mutex
	configPollerProcRunning <- true

	// init vars
	var nextTime time.Time

	// infinite loop
	log.Info("configPoller: Start...")
	for {
		// check if we need to exit
		select {
		case <-doExit:
			log.Warn("configPoller: Received exit signal, bye.")
			<-configPollerProcRunning
			return
		default:
		}

		// is it time to run?
		if nextTime.Before(time.Now()) {
			log.Debug("configPoller: Checking for new configuration on master server...")

			pNewCfg := new(disttrace.SlaveConfig)
			ppNewCfg := &pNewCfg
			err := getConfigFromMaster(masterURL, ppNewCfg)
			if err != nil {
				log.Warn("configPoller: Couldn't get current configuration from master server")

			} else {
				newCfgJSON, _ := json.Marshal(**ppNewCfg)
				oldCfgJSON, _ := json.Marshal(**ppCfg)

				if string(newCfgJSON) != string(oldCfgJSON) {
					// config changed
					log.Info("configPoller: Configuration on master changed, applying new configuration and going to sleep...")
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

// runMeasurement is run for every target simultaneously as a seperate process. Hands results directly to txProcess
func runMeasurement(targetID uuid.UUID, target disttrace.TraceTarget, cfg disttrace.SlaveConfig, txBuffer chan disttrace.TraceResult) {
	var result = disttrace.TraceResult{}
	result.ID = uuid.New()
	result.DateTime = time.Now()
	result.Target = target

	//TODO targetID not unique over time, concurrent runs will have same id -> bad as log reference
	log.Debugf("runMeasurement[%s]: Beginning measurement for target '%v'\n", targetID, target.Name)

	// generate fake measurements during development
	result.HopCount = 3
	result.Success = true
	dur1, _ := time.ParseDuration("100ms")
	dur2, _ := time.ParseDuration("200ms")
	dur3, _ := time.ParseDuration("300ms")
	result.Hops = []tracert.TracerouteHop{
		tracert.TracerouteHop{
			Success: true, Address: [4]byte{1, 2, 3, 4}, Host: "host1.at", N: 1, ElapsedTime: dur1, TTL: 1,
		},
		tracert.TracerouteHop{
			Success: true, Address: [4]byte{1, 2, 2, 1}, Host: "host2.at", N: 2, ElapsedTime: dur2, TTL: 2,
		},
		tracert.TracerouteHop{
			Success: true, Address: [4]byte{1, 2, 2, 3}, Host: "host3.at", N: 3, ElapsedTime: dur3, TTL: 3,
		},
	}
	txBuffer <- result
	log.Debugf("runMeasurement[%v]: Finished measurement for target '%v'\n", targetID, target.Name)
	return

	// need to supply chan with sufficient buffer, not used
	c := make(chan tracert.TracerouteHop, (cfg.MaxHops + 1))

	// create Traceroute options from config
	opts := tracert.TracerouteOptions{}
	opts.SetMaxHops(cfg.MaxHops)
	opts.SetRetries(cfg.Retries)
	opts.SetTimeoutMs(cfg.TimeoutMs)

	// do measurement
	res, err := tracert.Traceroute(target.Address, &opts, c)
	if err != nil {
		log.Warnf("runMeasurement[%v]: Error while doing traceroute to target '%v': %v\n", targetID, target.Name, err)
		// TODO permanently broken targets to be removed from config?
		return
	}

	if len(res.Hops) == 0 {
		log.Warnf("runMeasurement[%v]: Strange, no hops received for target '%v'. Success: false\n", targetID, target.Name)
		result.Success = false

	} else {
		log.Infof("runMeasurement[%v]: Success, Target: %v (%v), Hops: %v, Time: %v\n",
			targetID, target.Name, target.Address,
			res.Hops[len(res.Hops)-1].TTL,
			res.Hops[len(res.Hops)-1].ElapsedTime,
		)
		result.Success = res.Hops[len(res.Hops)-1].Success
	}

	result.Hops = res.Hops
	result.HopCount = len(res.Hops)

	txBuffer <- result
	return
}

// txResultsToMaster runs as process. Takes results and transmits them to master server.
func txResultsToMaster(buf chan disttrace.TraceResult, doExit chan bool, ppCfg **disttrace.SlaveConfig) {

	// lock mutex
	txProcRunning <- true

	// init
	var workReceived = false
	var cleanupAndExit = false
	var currentResult = disttrace.TraceResult{}
	var httpClient = &http.Client{
		Timeout: time.Second * 10,
	}
	var workErr error
	var workErrCount int
	var numMaxRetries = 3

	// launch infinite loop
	log.Info("txResultsToMaster: Start...")
	for {
		// check if we need to exit
		select {
		case <-doExit:
			log.Warn("txResultsToMaster: Received exit signal")
			cleanupAndExit = true
		default:
		}

		// check for work, if we don't still have workitems
		if !workReceived {
			select {
			case traceRes := <-buf:
				log.Debug("txResultsToMaster: Received workload:", traceRes.Target.Name)
				currentResult = traceRes
				workReceived = true
			default:
			}
		} else {
			log.Debug("txResultsToMaster: not checking for new work, not done yet...")
		}

		// only exit, when all work is done
		if cleanupAndExit && !workReceived {
			log.Info("txResultsToMaster: No new work to do and was told to exit, bye.")
			<-txProcRunning
			return
		}

		// work, work
		if workReceived {
			log.Debug("txResultsToMaster: Sending: ", currentResult.Target.Name)
			time.Sleep(3 * time.Second)

			// prepare data to be sent
			resultJSON, err := json.Marshal(currentResult)
			if err != nil {
				log.Warn("txResultsToMaster: Error: Couldn't create result json: ", err)
				workErr = err
				goto endWork
			}

			// get current config
			cfg := **ppCfg

			// send data to master
			httpResp, err := httpClient.Post(cfg.ReportURL, "application/json", bytes.NewBuffer(resultJSON))
			if err != nil {
				log.Warn("txResultsToMaster: Error sending HTTP Request: ", err)
				workErr = err
				goto endWork
			}
			defer httpResp.Body.Close()

			// read response from master
			httpRespBody, err := ioutil.ReadAll(httpResp.Body)
			if err != nil {
				log.Warn("txResultsToMaster: Can't read response body: ", err)
				workErr = err
				goto endWork
			}

			// parse result
			txResult := disttrace.SubmitResult{}
			err = json.Unmarshal(httpRespBody, &txResult)
			if err != nil {
				log.Warnf("txResultsToMaster: Can't parse body '%v' (first 100 char), Error: %v\n", string(httpRespBody)[:100], err)
				workErr = err
				goto endWork
			}
			if !txResult.Success && txResult.RetryPossible {
				log.Warn("txResultsToMaster: Master replied unsuccessful but retry possible, Error:", txResult.Error)
				goto endWork
			} else if !txResult.Success && !txResult.RetryPossible {
				log.Warn("txResultsToMaster: Master replied unsuccessful and shall not retry, Error:", txResult.Error)
			}

			log.Info("txResultsToMaster: Successfully transmitted results for item: ", currentResult.Target.Name)
			// finished handling, prepare for next item
			currentResult = disttrace.TraceResult{}
			workReceived = false
			workErrCount = 0
			workErr = *new(error)
		}
	endWork:

		if workErr != nil {
			workErrCount++
			log.Warnf("txResultsToMaster: An error occurred when handling workitem '%v'. Will retry, retrycount: %v/%v...", currentResult.Target.Name, workErrCount, numMaxRetries)
		}
		if workErrCount >= numMaxRetries {
			log.Warnf("txResultsToMaster: Too many retries reached for workitem '%v'. Discarding item and continuing...", currentResult.Target.Name)
			currentResult = disttrace.TraceResult{}
			workReceived = false
			workErrCount = 0
			workErr = *new(error)
		}
		// pause between work
		time.Sleep(1 * time.Second)
	}
}

// tracePoller runs every minute and creates measurement processes for every target
func tracePoller(txBuffer chan disttrace.TraceResult, doExit chan bool, ppCfg **disttrace.SlaveConfig) {

	// lock mutex
	tracePollerProcRunning <- true

	// init vars
	var nextTime time.Time

	// infinite loop
	log.Info("tracePoller: Start...")
	for {
		// check if we need to exit
		select {
		case <-doExit:
			log.Warn("tracePoller: Received exit signal, bye.")
			<-tracePollerProcRunning
			return
		default:
		}

		// is it time to run?
		if nextTime.Before(time.Now()) {

			// get a copy of current config
			tempCfg := **ppCfg
			tempCfgTargets := tempCfg.Targets

			// loop through configured targets
			for i, target := range tempCfgTargets {
				log.Infof("tracePoller: Running measurement proc [%v] for element '%v'\n", i, target.Name)
				go runMeasurement(i, target, tempCfg, txBuffer)
			}

			// run again on next full minute
			nextTime = time.Now().Truncate(time.Minute)
			nextTime = nextTime.Add(time.Minute).Add(10 * time.Second)
		}

		// zzz...
		time.Sleep(1 * time.Second)
	}
}

func printUsage(fSet flag.FlagSet) {
	log.Warn("Usage: ")

	buf := bytes.NewBuffer([]byte{})
	fSet.SetOutput(buf)
	fSet.PrintDefaults()
	log.Warn(string(buf.Bytes()))
	log.Warn()

	// create sample config file
	def := new(disttrace.SlaveConfig)
	targets := make(map[uuid.UUID]disttrace.TraceTarget)
	targets[uuid.New()] = disttrace.TraceTarget{}
	def.Targets = targets
	defJSON, _ := json.MarshalIndent(def, "", "  ")

	log.Warn("Sample configuration file: ")
	log.Warn("", string(defJSON))
}

func main() {
	// setup logging
	log.SetLevel(log.InfoLevel)

	// let's Go! :)
	log.Warn("Main: Starting...")

	// setup inter-proc communication channels
	var txSendBuffer = make(chan disttrace.TraceResult, 100)
	var txProcDoExitSignal = make(chan bool)
	var tracePollerProcDoExitSignal = make(chan bool)
	var configPollerProcDoExitSignal = make(chan bool)

	// setup listener for OS exit signals
	osSignal := make(chan os.Signal, 1)
	osSigReceived := make(chan bool, 1)
	signal.Notify(osSignal, syscall.SIGINT, syscall.SIGTERM)

	// wait for signal in background...
	go func() {
		sig := <-osSignal
		log.Warn("Main: Received os signal: ", sig)
		osSigReceived <- true
	}()

	// parse cmdline arguments
	var masterURL string
	fSet := flag.FlagSet{}
	outBuf := bytes.NewBuffer([]byte{})
	fSet.SetOutput(outBuf)
	fSet.StringVar(&masterURL, "master-server", "", "Set the http(s) `URL` to the configuration file on master server")
	fSet.Parse(os.Args[1:])

	// didn't receive a master URL, exit
	if masterURL == "" {
		printUsage(fSet)
		log.Fatal("Error: No master URL configured, can't run, Bye.")
	}

	// check if valid URL was supplied or exit
	if _, err := url.ParseRequestURI(masterURL); err != nil {
		printUsage(fSet)
		log.Fatal("Error: Master URL invalid, can't run: ", masterURL)
	}

	// read configuration from master server
	pCfg := new(disttrace.SlaveConfig)
	ppCfg := &pCfg
	err := getConfigFromMaster(masterURL, ppCfg)
	if err != nil {
		log.Fatal("Main: Fatal: Couldn't get configuration from master. Bye.")
	}

	log.Info("Main: Launching config poller process...")
	go configPoller(configPollerProcDoExitSignal, masterURL, ppCfg)

	for {
		time.Sleep(1 * time.Second)
		tempCfg := **ppCfg
		if len(tempCfg.Targets) > 0 {
			break
		}
		log.Debug("Main: Waiting for valid config...")
	}

	log.Info("Main: Launching transmit process...")
	go txResultsToMaster(txSendBuffer, txProcDoExitSignal, ppCfg)

	log.Info("Main: Launching trace poller process...")
	go tracePoller(txSendBuffer, tracePollerProcDoExitSignal, ppCfg)

	// wait here until told to quit by os signal
	log.Info("Main: startup finished, going to sleep...")
	<-osSigReceived

	log.Info("Warn: Sending exit signal to processes...")
	txProcDoExitSignal <- true
	tracePollerProcDoExitSignal <- true
	configPollerProcDoExitSignal <- true

	log.Info("Main: Waiting for config poller process to quit...")
	configPollerProcRunning <- true

	log.Info("Main: Waiting for trace poller process to quit...")
	tracePollerProcRunning <- true

	log.Info("Main: Waiting for transmit process to quit...")
	txProcRunning <- true

	log.Info("Warn: Everything has gracefully ended...")
	log.Info("Warn: Bye.")
}
