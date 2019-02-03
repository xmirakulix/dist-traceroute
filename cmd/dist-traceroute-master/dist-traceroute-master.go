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
	"time"
)

var configPollerProcRunning = make(chan bool, 1)

// TODO https/TLS

func checkCredentials(slaveCreds disttrace.SlaveCredentials, writer http.ResponseWriter, req *http.Request, ppCfg **disttrace.GenericConfig) (success bool) {

	success = false
	pCfg := *ppCfg

	// check for match in master config
	for _, trustedSlave := range pCfg.Slaves {
		if trustedSlave.Name == slaveCreds.Name && trustedSlave.Password == slaveCreds.Password {

			// success!
			log.Debugf("checkCredentials: Successfully authenticated slave '%v' from peer: %v", slaveCreds.Name, req.RemoteAddr)
			return true
		}
	}

	// no match found, unauthorized!
	log.Warnf("checkCredentials: Unauthorized peer '%v'", req.RemoteAddr)
	time.Sleep(2 * time.Second)
	http.Error(writer, "Unauthorized", http.StatusUnauthorized)
	return false
}

func httpDefaultHandler(writer http.ResponseWriter, req *http.Request, ppCfg **disttrace.GenericConfig) {
	log.Info("httpDefaultHandler: Received request for unknown URL: ", req.URL)
}

func httpRxResultHandler(writer http.ResponseWriter, req *http.Request, ppCfg **disttrace.GenericConfig) {
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
			http.Error(writer, "Error: Couldn't marshal error response into JSON", http.StatusBadRequest)
			log.Warn("httpRxResultHandler: Error: Couldn't marshal error response into JSON: ", err)
			return
		}

		// reply with error
		http.Error(writer, string(responseJSON), http.StatusBadRequest)
		return
	}

	// check authorization
	if !checkCredentials(result.Creds, writer, req, ppCfg) {
		return
	}

	log.Info("httpRxResultHandler: Received results for target: ", result.Target.Name)

	// TODO validate results
	// TODO use submitted result!

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

func httpTxConfigHandler(writer http.ResponseWriter, req *http.Request, ppCfg **disttrace.GenericConfig) {
	log.Info("httpTxConfigHandler: Received request for config, URL: ", req.URL)

	// read request body
	reqBody, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Warn("httpTxConfigHandler: Can't read request body, Error: ", err)
		http.Error(writer, "Can't read request", http.StatusInternalServerError)
		return
	}

	// parse JSON from request body
	slaveCreds := disttrace.SlaveCredentials{}
	err = json.Unmarshal(reqBody, &slaveCreds)
	if err != nil {
		log.Warn("httpTxConfigHandler: Can't unmarshal request body into slave creds, Error: ", err)
		http.Error(writer, "Can't unmarshal request body", http.StatusBadRequest)
		return
	}

	// check authorization
	if !checkCredentials(slaveCreds, writer, req, ppCfg) {
		return
	}

	// TODO validate config

	// read config from disk
	cfgFile := "dt-targets.json"
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

func httpServer(ppCfg **disttrace.GenericConfig) {

	log.Info("httpServer: Start...")

	// TODO: only handle content type json here?

	// handle results from slaves
	http.HandleFunc("/results/", func(w http.ResponseWriter, r *http.Request) {
		httpRxResultHandler(w, r, ppCfg)
	})

	// handle config requests from slaves
	http.HandleFunc("/config/", func(w http.ResponseWriter, r *http.Request) {
		httpTxConfigHandler(w, r, ppCfg)
	})

	// handle everything else
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		httpDefaultHandler(w, r, ppCfg)
	})

	// TODO shutdown handler https://golang.org/src/net/http/example_test.go
	log.Fatal(http.ListenAndServe(":8990", nil))
}

func main() {

	// setup logging
	log.SetLevel(log.DebugLevel)

	// let's Go! :)
	log.Warn("Main: Starting...")

	// setup inter-proc communication channels
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

	// TODO cmdline flag for config file name

	// create master configuration
	var pCfg = new(disttrace.GenericConfig)
	pCfg.MasterConfig = new(disttrace.MasterConfig)
	var ppCfg = &pCfg
	var configFileName = "dt-slaves.json"

	log.Info("Main: Launching config poller process...")
	go disttrace.MasterConfigPoller(configPollerProcDoExitSignal, configFileName, ppCfg)

	disttrace.WaitForValidConfig("master", ppCfg)

	log.Info("Main: Launching http server process...")
	go httpServer(ppCfg)

	// wait here until told to quit by os signal
	log.Info("Main: startup finished, going to sleep...")
	<-osSigReceived

	log.Info("Warn: Everything has gracefully ended...")
	log.Info("Warn: Bye.")
}
