package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	tracert "github.com/aeden/traceroute"
	valid "github.com/asaskevich/govalidator"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/xmirakulix/dist-traceroute/disttrace"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

var txProcRunning = make(chan bool, 1)
var tracePollerProcRunning = make(chan bool, 1)

var doExit = false

// runMeasurement is run for every target simultaneously as a seperate process. Hands results directly to txProcess
func runMeasurement(targetID uuid.UUID, target disttrace.TraceTarget, cfg disttrace.SlaveConfig, txBuffer chan disttrace.TraceResult) {
	var result = disttrace.TraceResult{}
	result.ID = uuid.New()
	result.DateTime = time.Now()
	result.Target = target

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
func txResultsToMaster(buf chan disttrace.TraceResult, slaveCreds disttrace.SlaveCredentials, ppCfg **disttrace.GenericConfig) {

	// lock mutex
	txProcRunning <- true

	disttrace.WaitForValidConfig("txResultsToMaster", "slave", ppCfg)

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
		if disttrace.CheckForQuit() {
			log.Warn("txResultsToMaster: Received exit signal")
			cleanupAndExit = true
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
			currentResult.Creds = slaveCreds
			resultJSON, err := json.Marshal(currentResult)
			if err != nil {
				log.Warn("txResultsToMaster: Error: Couldn't create result json: ", err)
				workErr = err
				goto endWork
			}

			log.Debugf("txResultsToMaster: Transmitting... \n%s", resultJSON)

			// get current config
			cfg := **ppCfg

			// send data to master
			url := "http://" + cfg.MasterHost + ":" + cfg.MasterPort + "/results/"

			httpResp, err := httpClient.Post(url, "application/json", bytes.NewBuffer(resultJSON))
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

			if httpResp.StatusCode != 200 {
				log.Warnf("txResultsToMaster: Received non-OK status '%v', response: %s", httpResp.Status, httpRespBody)
				workErr = err
				goto endWork
			}

			// parse result
			txResult := disttrace.SubmitResult{}
			err = json.Unmarshal(httpRespBody, &txResult)
			if err != nil {

				// only trace first 100 chars or response body
				var trace string
				if len(string(httpRespBody)) > 100 {
					trace = string(httpRespBody)[:100]
				} else {
					trace = string(httpRespBody)
				}

				log.Warnf("txResultsToMaster: Can't parse body '%v' (first 100 char), Error: %v\n", trace, err)
				workErr = err
				goto endWork
			}
			if !txResult.Success && txResult.RetryPossible {
				log.Warn("txResultsToMaster: Master replied that request was unsuccessful but retry possible, Error:", txResult.Error)
				workErr = errors.New("Master replied success=false, but retry ok")
				goto endWork
			} else if !txResult.Success && !txResult.RetryPossible {
				log.Warn("txResultsToMaster: Master replied that request was unsuccessful and shall not retry, Error:", txResult.Error)
			} else {
				log.Info("txResultsToMaster: Successfully transmitted results for item: ", currentResult.Target.Name)
			}

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
func tracePoller(txBuffer chan disttrace.TraceResult, ppCfg **disttrace.GenericConfig) {

	// lock mutex
	tracePollerProcRunning <- true

	disttrace.WaitForValidConfig("tracePoller", "slave", ppCfg)

	// init vars
	var nextTime time.Time

	// infinite loop
	log.Info("tracePoller: Start...")
	for {
		// check if we need to exit
		if disttrace.CheckForQuit() {
			log.Warn("tracePoller: Received exit signal, bye.")
			<-tracePollerProcRunning
			return
		}

		// is it time to run?
		if nextTime.Before(time.Now()) {

			// get a copy of current config
			pTempCfg := *ppCfg
			tempCfg := *pTempCfg.SlaveConfig
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

func main() {
	// setup logging
	log.SetLevel(log.DebugLevel)

	// let's Go! :)
	log.Warn("Main: Starting...")

	// setup inter-proc communication channels
	var txSendBuffer = make(chan disttrace.TraceResult, 100)

	// setup listener for OS exit signals
	disttrace.ListenForOSSignals()

	// parse cmdline arguments
	var masterHost, masterPort, slaveName, slavePwd string
	fSet := flag.FlagSet{}
	outBuf := bytes.NewBuffer([]byte{})
	fSet.SetOutput(outBuf)
	fSet.StringVar(&masterHost, "master", "", "Set the `hostname`/IP of the master server")
	fSet.StringVar(&masterPort, "master-port", "8990", "Set the listening `port (optional)` of the master server")
	fSet.StringVar(&slaveName, "name", "", "Unique `name` of this slave used on master for authentication and storage of results")
	fSet.StringVar(&slavePwd, "passwd", "", "Shared `secret` for slave on master")
	fSet.Parse(os.Args[1:])

	slaveCreds := disttrace.SlaveCredentials{Name: slaveName, Password: slavePwd}
	success, _ := valid.ValidateStruct(slaveCreds)

	// valid cmdline arguments or exit
	switch {
	case !success || !valid.IsDNSName(masterHost) || !valid.IsPort(masterPort):
		log.Warn("Error: No or invalid arguments, can't run, Bye.")
		disttrace.PrintSlaveUsageAndExit(fSet, true)
	}

	// read configuration from master server
	pCfg := &disttrace.GenericConfig{SlaveConfig: &disttrace.SlaveConfig{}}
	ppCfg := &pCfg

	log.Info("Main: Launching config poller process...")
	go disttrace.SlaveConfigPoller(masterHost, masterPort, slaveCreds, ppCfg)

	log.Info("Main: Launching transmit process...")
	go txResultsToMaster(txSendBuffer, slaveCreds, ppCfg)

	log.Info("Main: Launching trace poller process...")
	go tracePoller(txSendBuffer, ppCfg)

	// wait here until told to quit by os signal
	log.Info("Main: startup finished, going to sleep...")
	disttrace.WaitForOSSignalAndQuit()

	log.Info("Main: Waiting for config poller process to quit...")
	disttrace.ConfigPollerProcRunning <- true

	log.Info("Main: Waiting for trace poller process to quit...")
	tracePollerProcRunning <- true

	log.Info("Main: Waiting for transmit process to quit...")
	txProcRunning <- true

	log.Info("Warn: Everything has gracefully ended...")
	log.Info("Warn: Bye.")
}
