package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	tracert "github.com/aeden/traceroute"
	"github.com/google/uuid"
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
var pollerProcRunning = make(chan bool, 1)

// getConfigFromMaster fetches the slave's configuration from the master server
func getConfigFromMaster(masterURL string) (cfg disttrace.SlaveConfig, err error) {

	fmt.Printf("getConfigFromMaster: Attempting to read configuration from '%v'\n", masterURL)
	var httpClient = &http.Client{
		Timeout: time.Second * 10,
	}

	// download configuration file from master
	httpResp, err := httpClient.Get(masterURL)
	if err != nil {
		fmt.Println("getConfigFromMaster: Error sending HTTP Request: ", err)
		return disttrace.SlaveConfig{}, errors.New("Error sending HTTP Request")
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode >= 400 {
		fmt.Printf("getConfigFromMaster: Error getting configuration, received HTTP status: %v\n", httpResp.Status)
		return disttrace.SlaveConfig{}, errors.New("Error getting configuration, received HTTP error")
	}

	// read response from master
	httpRespBody, err := ioutil.ReadAll(httpResp.Body)
	if err != nil {
		fmt.Println("getConfigFromMaster: Can't read response body: ", err)
		return disttrace.SlaveConfig{}, errors.New("Can't read response body")
	}

	// parse result
	err = json.Unmarshal(httpRespBody, &cfg)
	if err != nil {
		fmt.Printf("getConfigFromMaster: Can't parse body '%v' (first 100 char), Error: %v\n", string(httpRespBody)[:100], err)
		return disttrace.SlaveConfig{}, errors.New("Can't parse response body")
	}

	fmt.Printf("getConfigFromMaster: Got config from master, number of configured targets: %v\n", len(cfg.Targets))
	return cfg, nil
}

func getCfgTargetByID(id uuid.UUID, cfg disttrace.SlaveConfig) (target *disttrace.TraceTarget) {

	return
}

// runMeasurement is run for every target simultaneously as a seperate process. Hands results directly to txProcess
func runMeasurement(targetID uuid.UUID, target disttrace.TraceTarget, cfg *disttrace.SlaveConfig, txBuffer chan disttrace.TraceResult) {
	var result = disttrace.TraceResult{}
	result.ID = uuid.New()
	result.DateTime = time.Now()
	result.Target = target

	//TODO targetID not unique over time!
	fmt.Printf("runMeasurement[%s]: Beginning measurement for target '%v'\n", targetID, target.Name)

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
	fmt.Printf("runMeasurement[%v]: Finished measurement for target '%v'\n", targetID, target.Name)
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
		fmt.Printf("runMeasurement[%v]: Error while doing traceroute to target '%v': %v\n", targetID, target.Name, err)

		// TODO permanently broken targets to be removed from config?
		// cfg.Targets[targetID].ErrorCount++

		return
	}

	if len(res.Hops) == 0 {
		fmt.Printf("runMeasurement[%v]: Strange, no hops received for target '%v'. Success: false\n", targetID, target.Name)
		result.Success = false

	} else {
		fmt.Printf("runMeasurement[%v]: Success, Target: %v (%v), Hops: %v, Time: %v\n",
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
func txResultsToMaster(buf chan disttrace.TraceResult, doExit chan bool, cfg *disttrace.SlaveConfig) {

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
	fmt.Println("txResultsToMaster: Start...")
	for {
		// check if we need to exit
		select {
		case <-doExit:
			fmt.Println("txResultsToMaster: Received exit signal")
			cleanupAndExit = true
		default:
		}

		// check for work, if we don't still have workitems
		if !workReceived {
			select {
			case traceRes := <-buf:
				fmt.Printf("txResultsToMaster: Received workload: '%v'\n", traceRes.Target.Name)
				currentResult = traceRes
				workReceived = true
			default:
			}
		} else {
			fmt.Println("txResultsToMaster: not checking for new work, not done yet...")
		}

		// only exit, when all work is done
		if cleanupAndExit && !workReceived {
			fmt.Println("txResultsToMaster: No new work to do and was told to exit, bye.")
			<-txProcRunning
			return
		}

		// work, work
		if workReceived {
			fmt.Printf("txResultsToMaster: Sending '%v'\n", currentResult.Target.Name)
			time.Sleep(3 * time.Second)

			// prepare data to be sent
			resultJSON, err := json.Marshal(currentResult)
			if err != nil {
				fmt.Println("txResultsToMaster: Error: Couldn't create result json: ", err)
				workErr = err
				goto endWork
			}

			// send data to master
			httpResp, err := httpClient.Post(cfg.ReportURL, "application/json", bytes.NewBuffer(resultJSON))
			if err != nil {
				fmt.Println("txResultsToMaster: Error sending HTTP Request: ", err)
				workErr = err
				goto endWork
			}
			defer httpResp.Body.Close()

			// read response from master
			httpRespBody, err := ioutil.ReadAll(httpResp.Body)
			if err != nil {
				fmt.Println("txResultsToMaster: Can't read response body: ", err)
				workErr = err
				goto endWork
			}

			// parse result
			txResult := disttrace.SubmitResult{}
			err = json.Unmarshal(httpRespBody, &txResult)
			if err != nil {
				fmt.Printf("txResultsToMaster: Can't parse body '%v' (first 100 char), Error: %v\n", string(httpRespBody)[:100], err)
				workErr = err
				goto endWork
			}
			if !txResult.Success && txResult.RetryPossible {
				fmt.Println("txResultsToMaster: Master replied unsuccessful but retry possible, Error: ", txResult.Error)
				goto endWork
			} else if !txResult.Success && !txResult.RetryPossible {
				fmt.Println("txResultsToMaster: Master replied unsuccessful and shall not retry, Error: ", txResult.Error)
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
			fmt.Printf("txResultsToMaster: An error occurred when handling workitem '%v'. Will retry, retrycount: %v/%v...\n", currentResult.Target.Name, workErrCount, numMaxRetries)
		}
		if workErrCount >= numMaxRetries {
			fmt.Printf("txResultsToMaster: Too many retries reached for workitem '%v'. Discarding item and continuing...\n", currentResult.Target.Name)
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
func tracePoller(txBuffer chan disttrace.TraceResult, doExit chan bool, cfg *disttrace.SlaveConfig) {

	// lock mutex
	pollerProcRunning <- true

	// init vars
	var nextTime time.Time

	// infinite loop
	fmt.Println("tracePoller: Start...")
	for {
		// check if we need to exit
		select {
		case <-doExit:
			fmt.Println("tracePoller: Received exit signal, bye.")
			<-pollerProcRunning
			return
		default:
		}

		// is it time to run?
		if nextTime.Before(time.Now()) {

			// get a copy of current config
			confTargets := cfg.Targets

			// loop through configured targets
			for i, target := range confTargets {
				fmt.Printf("tracePoller: Running measurement proc [%v] for element '%v'\n", i, target.Name)
				go runMeasurement(i, target, cfg, txBuffer)
			}

			// run again on next full minute
			nextTime = time.Now().Truncate(time.Minute)
			nextTime = nextTime.Add(time.Minute)
		}

		// zzz...
		time.Sleep(1 * time.Second)
	}
}

func printUsageAndExit() {
	fmt.Println("Usage: ")
	flag.PrintDefaults()
	fmt.Println()

	def := new(disttrace.SlaveConfig)
	targets := make(map[uuid.UUID]disttrace.TraceTarget)
	targets[uuid.New()] = disttrace.TraceTarget{}
	def.Targets = targets
	defJSON, _ := json.MarshalIndent(def, "", "  ")

	fmt.Println("Sample configuration file: ")
	fmt.Println("", string(defJSON))

	// can't run without master server URL
	os.Exit(1)
}

func main() {

	fmt.Println("Main: Starting...")

	// setup inter-proc communication channels
	var txSendBuffer = make(chan disttrace.TraceResult, 100)
	var txProcDoExitSignal = make(chan bool)
	var pollerProcDoExitSignal = make(chan bool)

	// setup listener for OS exit signals
	osSignal := make(chan os.Signal, 1)
	osSigReceived := make(chan bool, 1)
	signal.Notify(osSignal, syscall.SIGINT, syscall.SIGTERM)

	// wait for signal in background...
	go func() {
		sig := <-osSignal
		fmt.Println("Main: Received os signal: ", sig)
		osSigReceived <- true
	}()

	// get cmdline arguments
	var masterURL string
	flag.StringVar(&masterURL, "master-server", "", "Set the http(s) `URL` to the configuration file on master server")
	flag.Parse()

	// didn't receive a master URL, exit
	if masterURL == "" {
		printUsageAndExit()
	}

	// check if valid URL was supplied or exit
	if _, err := url.ParseRequestURI(masterURL); err != nil {
		fmt.Printf("Error: Not an URL: \"%v\"\n", masterURL)
		printUsageAndExit()
	}

	// read configuration from master server
	cfg, err := getConfigFromMaster(masterURL)
	if err != nil {
		fmt.Println("Main: Fatal: Couldn't get configuration from master. Bye.")
		os.Exit(1)
	}

	fmt.Println("Main: Launching transmit process...")
	go txResultsToMaster(txSendBuffer, txProcDoExitSignal, &cfg)

	fmt.Println("Main: Launching poller process...")
	go tracePoller(txSendBuffer, pollerProcDoExitSignal, &cfg)

	// TODO: periodically poll config from server?

	// wait here until told to quit by os signal
	fmt.Println("Main: startup finished, going to sleep...")
	<-osSigReceived

	fmt.Println("Main: Sending exit signal to transmit process...")
	txProcDoExitSignal <- true

	fmt.Println("Main: Waiting for transmit process to quit...")
	txProcRunning <- true
	fmt.Println("Main: Everything has gracefully ended...")
	fmt.Println("Main: Bye.")
}
