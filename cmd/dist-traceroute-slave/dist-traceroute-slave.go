package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	tracert "github.com/aeden/traceroute"
	"github.com/google/uuid"
	"github.com/xmirakulix/dist-traceroute/disttrace"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var cfg disttrace.SlaveConfig
var txProcRunning = make(chan bool, 1)
var pollerProcRunning = make(chan bool, 1)

func init() {
	cfg = disttrace.SlaveConfig{
		ReportURL: "http://www.parnigoni.net",
		Targets: []disttrace.TraceTarget{
			disttrace.TraceTarget{
				Name:    "WixRou8",
				Address: "193.9.252.241",
			},
			disttrace.TraceTarget{
				Name:    "LNS",
				Address: "193.9.252.201",
			},
		},
		Options: tracert.TracerouteOptions{},
	}

	cfg.Options.SetRetries(1)
	cfg.Options.SetMaxHops(30)
	cfg.Options.SetTimeoutMs(500)
}

// runMeasurement is run for every target simultaneously as a seperate process. Hands results directly to txProcess
func runMeasurement(sequence int, target disttrace.TraceTarget, txBuffer chan disttrace.TraceResult) {
	var result = disttrace.TraceResult{}
	result.ID = uuid.New()
	result.DateTime = time.Now()
	result.Target = target

	fmt.Printf("runMeasurement[%v]: Beginning measurement for target '%v'\n", sequence, target.Name)

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
	fmt.Printf("runMeasurement[%v]: Finished measurement for target '%v'\n", sequence, target.Name)
	return

	// TODO permanently broke targets to be removed from config?
	// need to supply chan with sufficient buffer, not used
	c := make(chan tracert.TracerouteHop, (cfg.Options.MaxHops() + 1))

	res, err := tracert.Traceroute(target.Address, &cfg.Options, c)
	if err != nil {
		fmt.Printf("runMeasurement[%v]: Error while doing traceroute to target '%v': %v\n", sequence, target.Name, err)
		return
	}

	if len(res.Hops) == 0 {
		fmt.Printf("runMeasurement[%v]: Strange, no hops received for target '%v'. Success: false\n", sequence, target.Name)
		result.Success = false

	} else {
		fmt.Printf("runMeasurement[%v]: Success, Target: %v (%v), Hops: %v, Time: %v\n",
			sequence, target.Name, target.Address,
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
func txResultsToMaster(buf chan disttrace.TraceResult, doExit chan bool) {

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

// pollResults runs every minute and creates measurement processes for every target
func pollResults(txBuffer chan disttrace.TraceResult, doExit chan bool) {

	// lock mutex
	pollerProcRunning <- true

	// init vars
	var nextTime time.Time

	// infinite loop
	fmt.Println("pollResults: Start...")
	for {
		// check if we need to exit
		select {
		case <-doExit:
			fmt.Println("pollResults: Received exit signal, bye.")
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
				fmt.Printf("pollResults: Running measurement proc [%v] for element '%v'\n", i, target.Name)
				go runMeasurement(i, target, txBuffer)
			}

			// run again on next full minute
			nextTime = time.Now().Truncate(time.Minute)
			nextTime = nextTime.Add(time.Minute)
		}

		// zzz...
		time.Sleep(1 * time.Second)
	}
}

func main() {

	fmt.Println("Main: Starting...")

	var txSendBuffer = make(chan disttrace.TraceResult, 100)
	var txProcDoExitSignal = make(chan bool)

	var pollerProcDoExitSignal = make(chan bool)

	osSignal := make(chan os.Signal, 1)
	osSigReceived := make(chan bool, 1)
	signal.Notify(osSignal, syscall.SIGINT, syscall.SIGTERM)

	// wait for signal in background...
	go func() {
		sig := <-osSignal
		fmt.Println("Main: Received os signal: ", sig)
		osSigReceived <- true
	}()

	fmt.Println("Main: Launching transmit process...")
	go txResultsToMaster(txSendBuffer, txProcDoExitSignal)

	fmt.Println("Main: Launching poller process...")
	go pollResults(txSendBuffer, pollerProcDoExitSignal)

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
