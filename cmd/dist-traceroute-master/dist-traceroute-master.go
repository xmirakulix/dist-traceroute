package main

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"github.com/xmirakulix/dist-traceroute/disttrace"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

// TODO https/TLS

func httpDefaultHandler(writer http.ResponseWriter, req *http.Request) {
	log.Info("httpDefaultHandler: Received request for unknown URL: ", req.URL)
}

func httpRxResultHandler(writer http.ResponseWriter, req *http.Request) {
	log.Info("httpRxResultHandler: Received request results, URL: ", req.URL)

	// init vars
	result := disttrace.TraceResult{}
	jsonDecoder := json.NewDecoder(req.Body)

	// decode request
	err := jsonDecoder.Decode(&result)
	if err != nil {
		log.Warn("httpRxResultHandler: Couldn't decode request body into JSON: ", err)

		// create error response
		response := disttrace.SubmitResult{
			Success:       false,
			Error:         "Couldn't decode request body into JSON: " + err.Error(),
			RetryPossible: false,
		}

		responseJSON, err := json.Marshal(response)
		if err != nil {
			http.Error(writer, "Error: Couldn't marshal error response into JSON", http.StatusInternalServerError)
			log.Warn("httpRxResultHandler: Error: Couldn't marshal error response into JSON: ", err)
			return
		}

		// reply with error
		http.Error(writer, string(responseJSON), http.StatusBadRequest)
		return
	}

	// TODO check credentials

	log.Info("httpRxResultHandler: Received results for target: ", result.Target.Name)

	// TODO: use submitted result!

	// reply with success
	response := disttrace.SubmitResult{
		Success:       true,
		Error:         "",
		RetryPossible: true,
	}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		http.Error(writer, "Error: Couldn't marshal success response into JSON", http.StatusInternalServerError)
		log.Warn("httpRxResultHandler: Error: Couldn't marshal success response into JSON: ", err)
		return
	}

	// Success!
	_, err = io.WriteString(writer, string(responseJSON))
	if err != nil {
		log.Warn("httpRxResultHandler: Couldn't write success response: ", err)
	}
	log.Debug("httpRxResultHandler: Replying success.")
	return
}

func httpTxConfigHandler(writer http.ResponseWriter, req *http.Request) {
	log.Info("httpTxConfigHandler: Received request for config, URL: ", req.URL)

	// TODO check credentials

	// read config from disk
	cfgFile := "dist-traceroute.json"
	body, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		http.Error(writer, "Error: Couldn't read config file!", http.StatusInternalServerError)
		log.Warn("httpTxConfigHandler: Error: Couldn't read config file: ", err)
		return
	}

	// send config to slave
	_, err = io.WriteString(writer, string(body))
	if err != nil {
		log.Warn("httpTxConfigHandler: Couldn't write success response: ", err)
	}

	log.Debug("httpTxConfigHandler: Replying configuration.")
	return
}

func httpServer() {

	log.Info("httpServer: Start...")
	http.HandleFunc("/", httpDefaultHandler)
	http.HandleFunc("/results/", httpRxResultHandler)
	http.HandleFunc("/config/", httpTxConfigHandler)

	// TODO shutdown handler https://golang.org/src/net/http/example_test.go
	log.Fatal(http.ListenAndServe(":8990", nil))
}

func main() {

	// setup logging
	log.SetLevel(log.DebugLevel)

	// let's Go! :)
	log.Warn("Main: Starting...")

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

	log.Info("Main: Launching http server process...")
	go httpServer()

	// wait here until told to quit by os signal
	log.Info("Main: startup finished, going to sleep...")
	<-osSigReceived

	log.Info("Warn: Everything has gracefully ended...")
	log.Info("Warn: Bye.")
}
