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
		ReportURL: "http://www.google.at",
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

func handleResults(buf chan disttrace.TraceResult, doExit chan bool) {
	txProcRunning <- true
	workReceived := false
	cleanupAndExit := false
	currentResult := disttrace.TraceResult{}
	httpClient := &http.Client{
		Timeout: time.Second * 10,
	}
	var workErr error

	fmt.Println("handleResults: Start...")
	for {
		// check if we need to exit
		select {
		case <-doExit:
			fmt.Println("handleResults: Received exit signal")
			cleanupAndExit = true
		default:
		}

		// check for work, if we don't still have workitems
		if !workReceived {
			select {
			case traceRes := <-buf:
				fmt.Println("handleResults: Received workload: ", traceRes.Target.Name)
				currentResult = traceRes
				workReceived = true
			default:
			}
		} else {
			fmt.Println("handleResults: not checking for new work, not done yet...")
		}

		// only exit, when all work is done
		if cleanupAndExit && !workReceived {
			fmt.Println("handleResults: Bye.")
			<-txProcRunning
			return
		}

		// work, work
		if workReceived {
			fmt.Println("handleResults: Sending: ", currentResult.Target.Name)
			time.Sleep(3 * time.Second)

			// prepare data to be sent
			resultJSON, err := json.Marshal(currentResult)
			if err != nil {
				fmt.Println("handleResults: Error: Couldn't create result json: ", err)
				workErr = err
				goto endWork
			}

			// send data to master
			httpResp, err := httpClient.Post(cfg.ReportURL, "application/json", bytes.NewBuffer(resultJSON))
			if err != nil {
				fmt.Println("handleResults: Error sending HTTP Request: ", err)
				workErr = err
				goto endWork
			}
			defer httpResp.Body.Close()

			// read response from master
			httpRespBody, err := ioutil.ReadAll(httpResp.Body)
			if err != nil {
				fmt.Println("handleResults: Can't read response body: ", err)
				workErr = err
				goto endWork
			}

			// parse result
			txResult := disttrace.SubmitResult{}
			err = json.Unmarshal(httpRespBody, &txResult)
			if err != nil {
				fmt.Println("handleResults: Can't parse reply: ", err)
				workErr = err
				goto endWork
			}
			if !txResult.Success && txResult.RetryPossible {
				fmt.Println("handleResults: Master replied unsuccessful but retry possible, Error: ", txResult.Error)
				goto endWork
			} else if !txResult.Success && !txResult.RetryPossible {
				fmt.Println("handleResults: Master replied unsuccessful and shall not retry, Error: ", txResult.Error)
			}

			// finished handling, prepare for next item
			currentResult = disttrace.TraceResult{}
			workReceived = false
		}
	endWork:

		if workErr != nil {
			fmt.Printf("handleResults: An error occurred when handling workitem '%v'. Will retry...\n", currentResult.Target.Name)
		}
		// pause between work
		time.Sleep(1 * time.Second)
	}
}

func main() {

	fmt.Println("Starting...")

	resultSendBuffer := make(chan disttrace.TraceResult, 100)
	doExitSignal := make(chan bool)

	go func() {
		handleResults(resultSendBuffer, doExitSignal)
	}()

	for _, target := range cfg.Targets {
		result, err := runMeasurement(target)
		if err != nil {
			fmt.Println("Error: ", err)
		}

		fmt.Printf("Handing element '%v' to txProc\n", result.Target.Name)
		resultSendBuffer <- result
	}
	//time.Sleep(5 * time.Second)
	fmt.Println("Sending exit signal...")
	doExitSignal <- true

	fmt.Println("Waiting for txProc to Exit")
	txProcRunning <- true
	fmt.Println("Bye.")
}
