package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"io/ioutil"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/xmirakulix/dist-traceroute/disttrace"

	tracert "github.com/aeden/traceroute"
	valid "github.com/asaskevich/govalidator"
)

// TODO option to log to syslog
// TODO set http user agent

// global logger
var log = logrus.New()

// mutexes for states of goroutines
var txProcRunning = make(chan bool, 1)
var tracePollerProcRunning = make(chan bool, 1)

var measurementRunningLock = sync.Mutex{}

// shall we exit?
var doExit = false

// debug mode set as cmdline argument?
var debugMode = false

// runMeasurement runs the traceroute for the given target and hands results directly to txProcess
func runMeasurement(target disttrace.TraceTarget, cfg disttrace.SlaveConfig, txBuffer chan disttrace.TraceResult, txBufferSize *int32) {

	// init results struct
	var result = disttrace.TraceResult{}
	result.ID = uuid.New()
	result.DateTime = time.Now()
	result.Target = target

	log.Debugf("runMeasurement[%s]: Beginning measurement for target '%v'", target.ID, target.Name)

	//TODO update fake results to new data format
	// shall we create fake results?
	if debugMode {
		jsonStr := "{" +
			"\"Creds\":{\"Name\":\"slave\",\"Secret\":\"123\"}," +
			"\"ID\":\"d9bbc544-eabe-4536-a9ed-ec6cbdaaedb0\"," +
			"\"DateTime\":\"2019-02-07T21:06:10.803086+01:00\"," +
			"\"Target\":{\"Name\":\"Google\",\"Address\":\"www.google.at\"}," +
			"\"Success\":true," +
			"\"HopCount\":17," +
			"\"Hops\":[" +
			"	{\"Success\":true,\"Address\":[192,168,1,1],\"Host\":\"modem.home.\",\"N\":52,\"ElapsedTime\":2040972,\"TTL\":1}," +
			"	{\"Success\":true,\"Address\":[193,9,252,201],\"Host\":\"\",\"N\":52,\"ElapsedTime\":10509652,\"TTL\":2}," +
			"	{\"Success\":true,\"Address\":[193,9,252,241],\"Host\":\"wixrou8.mrsn.at.\",\"N\":52,\"ElapsedTime\":7861033,\"TTL\":3}," +
			"	{\"Success\":true,\"Address\":[185,208,12,5],\"Host\":\"\",\"N\":52,\"ElapsedTime\":21188503,\"TTL\":4}," +
			"	{\"Success\":true,\"Address\":[92,60,0,44],\"Host\":\"ae100-0-r01.inx.vie.nextlayer.net.\",\"N\":52,\"ElapsedTime\":16633831,\"TTL\":5}," +
			"	{\"Success\":true,\"Address\":[92,60,0,154],\"Host\":\"ae1-0-r11.inx.vie.nextlayer.net.\",\"N\":52,\"ElapsedTime\":68627777,\"TTL\":6}," +
			"	{\"Success\":true,\"Address\":[92,60,1,233],\"Host\":\"ae2-0-r60.inx.vie.nextlayer.net.\",\"N\":52,\"ElapsedTime\":9347528,\"TTL\":7}," +
			"	{\"Success\":true,\"Address\":[92,60,0,237],\"Host\":\"ae1-0-r60.inx.fra.nextlayer.net.\",\"N\":52,\"ElapsedTime\":20861397,\"TTL\":8}," +
			"	{\"Success\":true,\"Address\":[92,60,6,18],\"Host\":\"\",\"N\":52,\"ElapsedTime\":20969066,\"TTL\":9}," +
			"	{\"Success\":true,\"Address\":[108,170,252,19],\"Host\":\"\",\"N\":52,\"ElapsedTime\":23301858,\"TTL\":10}," +
			"	{\"Success\":true,\"Address\":[209,85,241,231],\"Host\":\"\",\"N\":52,\"ElapsedTime\":21846962,\"TTL\":11}," +
			"	{\"Success\":true,\"Address\":[216,239,56,131],\"Host\":\"\",\"N\":52,\"ElapsedTime\":29712560,\"TTL\":12}," +
			"	{\"Success\":true,\"Address\":[172,253,50,111],\"Host\":\"\",\"N\":52,\"ElapsedTime\":35513459,\"TTL\":13}," +
			"	{\"Success\":true,\"Address\":[108,170,225,179],\"Host\":\"\",\"N\":52,\"ElapsedTime\":34569087,\"TTL\":14}," +
			"	{\"Success\":true,\"Address\":[108,170,253,49],\"Host\":\"\",\"N\":52,\"ElapsedTime\":39223782,\"TTL\":15}," +
			"	{\"Success\":true,\"Address\":[209,85,245,203],\"Host\":\"\",\"N\":52,\"ElapsedTime\":43379004,\"TTL\":16}," +
			"	{\"Success\":true,\"Address\":[172,217,19,67],\"Host\":\"ham02s17-in-f3.1e100.net.\",\"N\":52,\"ElapsedTime\":43045659,\"TTL\":17}" +
			"	]" +
			"}"

		json.Unmarshal([]byte(jsonStr), &result)
		result.Target.Name = target.Name
		result.Target.Address = target.Address
		txBuffer <- result
		atomic.AddInt32(txBufferSize, 1)
		log.Debugf("runMeasurement[%v]: returning fake measurement for target '%v'", target.ID, target.Name)
		return
	}

	// need to supply chan with sufficient buffer, not used
	c := make(chan tracert.TracerouteHop, (target.MaxHops + 1))

	// create Traceroute options from config
	opts := tracert.TracerouteOptions{}
	opts.SetMaxHops(target.MaxHops)
	opts.SetRetries(target.Retries)
	opts.SetTimeoutMs(target.TimeoutMs)

	// don't run two measurements simultaneously!
	measurementRunningLock.Lock()
	defer measurementRunningLock.Unlock()

	// do measurement
	res, err := tracert.Traceroute(target.Address, &opts, c)
	if err != nil {
		log.Warnf("runMeasurement[%v]: Error while doing traceroute to target '%v': %v", target.ID, target.Name, err)
		return
	}

	if len(res.Hops) == 0 {
		log.Warnf("runMeasurement[%v]: Strange, no hops received for target '%v'. Success: false", target.ID, target.Name)
		result.Success = false

	} else {
		log.Debugf("runMeasurement[%v]: Success, Target: %v (%v), Hops: %v, Time: %v",
			target.ID, target.Name, target.Address,
			res.Hops[len(res.Hops)-1].TTL,
			res.Hops[len(res.Hops)-1].ElapsedTime,
		)
		result.Success = res.Hops[len(res.Hops)-1].Success
	}

	result.Hops = res.Hops
	result.HopCount = len(res.Hops)

	select {
	case txBuffer <- result:
		queuesize := atomic.AddInt32(txBufferSize, 1)
		log.Infof("runMeasurement[%v]: Added item '%v' to tx queue, new queue size: %v. Target: %v, Success:%v, Hops: %v",
			target.ID, target.Name, queuesize,
			target.Address, result.Success, result.HopCount,
		)
	default:
		log.Warnf("Couldn't add result for '%v' to queue (current queue size: %v), result discarded. Possibly transmission to master stalled?", result.Target.Name, *txBufferSize)
	}
	return
}

// txResultsToMaster runs as process. Takes results and transmits them to master server.
func txResultsToMaster(buf chan disttrace.TraceResult, bufSize *int32, slave disttrace.Slave, ppCfg **disttrace.SlaveConfig) {

	// lock mutex
	txProcRunning <- true

	disttrace.WaitForValidConfig("txResultsToMaster", ppCfg)

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
				atomic.AddInt32(bufSize, -1)
				log.Infof("txResultsToMaster: Transmitting workload: '%v', remaining items in queue: %v", traceRes.Target.Name, *bufSize)
				currentResult = traceRes
				workReceived = true
			default:
				// pause between checks
				time.Sleep(1 * time.Second)
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

			// get current config
			cfg := **ppCfg

			// prepare data to be sent
			currentResult.Slave = slave
			currentResult.Slave.ID = cfg.ID
			resultJSON, err := json.Marshal(currentResult)
			if err != nil {
				log.Warn("txResultsToMaster: Error: Couldn't create result json: ", err)
				workErr = err
				goto endWork
			}

			log.Debugf("txResultsToMaster: Transmitting, Content: %s", resultJSON)

			// send data to master
			url := "http://" + cfg.MasterHost + ":" + cfg.MasterPort + "/slave/results"

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
				workErr = errors.New("Received non-OK status: " + httpResp.Status)
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

				log.Warnf("txResultsToMaster: Can't parse body '%v' (first 100 char), Error: %v", trace, err)
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
				log.Debugf("txResultsToMaster: Successfully transmitted results for item '%v', items in queue: %v", currentResult.Target.Name, *bufSize)
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
			time.Sleep(10 * time.Second)
		}
		if workErrCount >= numMaxRetries {
			log.Warnf("txResultsToMaster: Too many retries reached for workitem '%v'. Discarding item and continuing...", currentResult.Target.Name)
			currentResult = disttrace.TraceResult{}
			workReceived = false
			workErrCount = 0
			workErr = *new(error)
		}
	}
}

// tracePoller runs every minute and creates measurement processes
func tracePoller(txBuffer chan disttrace.TraceResult, txBufferSize *int32, ppCfg **disttrace.SlaveConfig) {

	// lock mutex
	tracePollerProcRunning <- true

	disttrace.WaitForValidConfig("tracePoller", ppCfg)

	// init vars
	var nextTime time.Time

	// infinite loop
	log.Info("tracePoller: Start...")
	for {
		// check if we need to exit

		// is it time to run?
		if nextTime.Before(time.Now()) {

			// get a copy of current config
			pTempCfg := *ppCfg
			tempCfg := *pTempCfg
			tempCfgTargets := tempCfg.Targets

			// loop through configured targets
			for _, target := range tempCfgTargets {
				log.Debugf("tracePoller: Running measurement proc [%v] for element '%v'", target.ID, target.Name)
				runMeasurement(target, tempCfg, txBuffer, txBufferSize)

				if disttrace.CheckForQuit() {
					log.Warn("tracePoller: Received exit signal, bye.")
					<-tracePollerProcRunning
					return
				}
			}

			// run again on next full minute
			nextTime = time.Now().Truncate(time.Minute)
			nextTime = nextTime.Add(time.Minute).Add(10 * time.Second)
		} else {
			time.Sleep(1 * time.Second)
		}
	}
}

func main() {

	// setup inter-proc communication channels
	var txSendBuffer = make(chan disttrace.TraceResult, 100)
	var txSendBufferCnt = new(int32)

	// parse cmdline arguments
	var masterHost, masterPort, logLevel, logPathAndName string
	var slave disttrace.Slave

	// check cmdline args
	{
		var sendHelp bool
		var slaveName, slaveSecret string

		fSet := flag.FlagSet{}
		outBuf := bytes.NewBuffer([]byte{})
		fSet.SetOutput(outBuf)
		fSet.StringVar(&masterHost, "master", "", "Set the `hostname`/IP of the master server")
		fSet.StringVar(&masterPort, "master-port", "8990", "Set the listening `port (optional)` of the master server")
		fSet.StringVar(&slaveName, "name", "", "Unique `name` of this slave used on master for authentication and storage of results")
		fSet.StringVar(&slaveSecret, "secret", "", "Shared `secret` for slave on master")
		fSet.StringVar(&logPathAndName, "log", "./slave.log", "Logfile location `/path/to/file`")
		fSet.StringVar(&logLevel, "loglevel", "info", "Specify loglevel, one of `warn, info, debug`")
		fSet.BoolVar(&debugMode, "zDebugResults", false, "Generate fake results, e.g. when run without root permissions")
		fSet.BoolVar(&sendHelp, "help", false, "display this message")
		fSet.Parse(os.Args[1:])

		slave = disttrace.Slave{Name: slaveName, Secret: slaveSecret}
		okSlave, _ := valid.ValidateStruct(slave)
		var errLog error
		logPathAndName, errLog = disttrace.CleanAndCheckFileNameAndPath(logPathAndName)

		// valid cmdline arguments or exit
		switch {
		case !okSlave || (!valid.IsDNSName(masterHost) && !valid.IsIP(masterHost)) || !valid.IsPort(masterPort):
			log.Warn("Error: No or invalid arguments for master, master-port or credentials, can't run, Bye.")
			disttrace.PrintUsageAndExit(fSet, true)
		case errLog != nil:
			log.Warn("Error: Invalid log path specified, can't run, Bye.")
			disttrace.PrintUsageAndExit(fSet, true)
		case logLevel != "warn" && logLevel != "info" && logLevel != "debug":
			log.Warn("Error: Invalid loglevel specified, can't run, Bye.")
			disttrace.PrintUsageAndExit(fSet, true)
		case sendHelp:
			disttrace.PrintUsageAndExit(fSet, true)
		}
	}

	// setup logging
	disttrace.SetLogOptions(log, logPathAndName, logLevel)

	// let's Go! :)
	log.Warn("Main: Starting...")
	disttrace.DebugPrintAllArguments(masterHost, masterPort, slave.Name, slave.Secret, logPathAndName, logLevel)

	// setup listener for OS exit signals
	disttrace.ListenForOSSignals()

	// read configuration from master server
	pCfg := &disttrace.SlaveConfig{}
	ppCfg := &pCfg

	log.Info("Main: Launching config poller process...")
	go disttrace.ConfigPoller(masterHost, masterPort, slave, ppCfg)

	log.Info("Main: Launching transmit process...")
	go txResultsToMaster(txSendBuffer, txSendBufferCnt, slave, ppCfg)

	log.Info("Main: Launching trace poller process...")
	go tracePoller(txSendBuffer, txSendBufferCnt, ppCfg)

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
