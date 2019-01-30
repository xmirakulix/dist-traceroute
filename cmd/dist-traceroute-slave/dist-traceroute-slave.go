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
	"time"
)

var cfg disttrace.SlaveConfig
var txProcRunning = make(chan bool, 1)

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

func runMeasurement(target disttrace.TraceTarget) (result disttrace.TraceResult, err error) {
	result.ID = uuid.New()
	result.DateTime = time.Now()
	result.Target = target

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
	return

	// need to supply chan with sufficient buffer, not used
	c := make(chan tracert.TracerouteHop, (cfg.Options.MaxHops() + 1))

	res, err := tracert.Traceroute(target.Address, &cfg.Options, c)
	if err != nil {
		fmt.Println("Error: ", err)
	}

	if len(res.Hops) == 0 {
		fmt.Println("Error: no hops")
		result.Success = false

	} else {
		fmt.Printf("Target: %v (%v), Hops: %v, Time: %v\n",
			target.Name, target.Address,
			res.Hops[len(res.Hops)-1].TTL,
			res.Hops[len(res.Hops)-1].ElapsedTime,
		)
		result.Success = res.Hops[len(res.Hops)-1].Success
	}

	result.Hops = res.Hops
	result.HopCount = len(res.Hops)

	return
}

func txResultsToMaster(buf chan disttrace.TraceResult, doExit chan bool) {
	txProcRunning <- true
	workReceived := false
	cleanupAndExit := false
	currentResult := disttrace.TraceResult{}
	httpClient := &http.Client{
		Timeout: time.Second * 10,
	}
	var workErr error
	var workErrCount int
	var numMaxRetries = 3

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
				fmt.Println("txResultsToMaster: Received workload: ", traceRes.Target.Name)
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
			fmt.Println("txResultsToMaster: Sending: ", currentResult.Target.Name)
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
		}
		// pause between work
		time.Sleep(1 * time.Second)
	}
}

func main() {

	fmt.Println("Main: Starting...")

	resultSendBuffer := make(chan disttrace.TraceResult, 100)
	doExitSignal := make(chan bool)

	go func() {
		txResultsToMaster(resultSendBuffer, doExitSignal)
	}()

	for _, target := range cfg.Targets {
		result, err := runMeasurement(target)
		if err != nil {
			fmt.Println("Error: ", err)
		}

		fmt.Printf("Main: Handing element '%v' to txProc\n", result.Target.Name)
		resultSendBuffer <- result
	}
	//time.Sleep(5 * time.Second)
	fmt.Println("Main: Sending exit signal...")
	doExitSignal <- true

	fmt.Println("Main: Waiting for txProc to Exit")
	txProcRunning <- true
	fmt.Println("Main: Bye.")
}
