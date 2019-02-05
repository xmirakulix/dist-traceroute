package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	valid "github.com/asaskevich/govalidator"
	log "github.com/sirupsen/logrus"
	"github.com/xmirakulix/dist-traceroute/disttrace"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

var httpProcQuitDone = make(chan bool, 1)

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

func httpDefaultHandler(ppCfg **disttrace.GenericConfig) http.HandlerFunc {
	return func(writer http.ResponseWriter, req *http.Request) {
		log.Info("httpDefaultHandler: Received request for unknown URL: ", req.URL)
	}
}

func httpRxResultHandler(ppCfg **disttrace.GenericConfig) http.HandlerFunc {
	return func(writer http.ResponseWriter, req *http.Request) {
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
}

func httpTxConfigHandler(ppCfg **disttrace.GenericConfig) http.HandlerFunc {
	return func(writer http.ResponseWriter, req *http.Request) {
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

		// read config from disk
		cfgFile := "dt-targets.json"
		body, err := ioutil.ReadFile(cfgFile)
		if err != nil {
			http.Error(writer, "Error: Couldn't read config file!", http.StatusInternalServerError)
			log.Warn("httpTxConfigHandler: Error: Couldn't read config file: ", err)
			return
		}

		slaveConf := disttrace.SlaveConfig{}
		err = json.Unmarshal(body, &slaveConf)
		if err != nil {
			http.Error(writer, "Error: Can't unmarshal config", http.StatusInternalServerError)
			log.Warn("httpTxConfigHandler: Loaded config can't be unmarshalled, Error: ", err)
		}

		if ok, err := valid.ValidateStruct(slaveConf); !ok || err != nil {
			http.Error(writer, "Error: Loaded config is invalid", http.StatusInternalServerError)
			log.Warn("httpTxConfigHandler: Loaded config is invalid, Error: ", err)
		}

		// send config to slave
		_, err = io.WriteString(writer, string(body))
		if err != nil {
			log.Warn("httpTxConfigHandler: Couldn't write success response: ", err)
		}

		log.Debug("httpTxConfigHandler: Replying configuration.")
		return
	}
}

func httpServer(ppCfg **disttrace.GenericConfig) {

	log.Info("httpServer: Start...")

	disttrace.WaitForValidConfig("httpServer", "master", ppCfg)

	srv := &http.Server{
		Addr: ":8990",
	}

	// handle results from slaves
	http.HandleFunc("/results/", httpRxResultHandler(ppCfg))

	// handle config requests from slaves
	http.HandleFunc("/config/", httpTxConfigHandler(ppCfg))

	// handle everything else
	http.HandleFunc("/", httpDefaultHandler(ppCfg))

	// start server...
	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatal("httpServer: HTTP Server failure, ListenAndServe: ", err)
		}
	}()

	// wait for quit signal...
	for {
		if disttrace.CheckForQuit() {
			log.Warn("httpServer: Received signal to shutdown...")
			ctx, cFunc := context.WithTimeout(context.Background(), 5*time.Second)
			if err := srv.Shutdown(ctx); err != nil {
				log.Warn("httpServer: Error while shutdown of HTTP server, Error: ", err)
			}
			cFunc()

			log.Info("httpServer: Shutdown complete.")
			httpProcQuitDone <- true
			return
		}

		time.Sleep(1 * time.Second)
	}
}

func main() {

	// setup logging
	log.SetLevel(log.DebugLevel)

	// let's Go! :)
	log.Warn("Main: Starting...")

	// setup listener for OS exit signals
	disttrace.ListenForOSSignals()

	// parse cmdline arguments
	var configFileName string
	var sendHelp bool
	fSet := flag.FlagSet{}
	outBuf := bytes.NewBuffer([]byte{})
	fSet.SetOutput(outBuf)
	fSet.StringVar(&configFileName, "config", "dt-slaves.json", "Set config `filename`")
	fSet.BoolVar(&sendHelp, "help", false, "display this message")
	fSet.Parse(os.Args[1:])

	// valid cmdline arguments or exit
	switch {
	case valid.SafeFileName(configFileName) != configFileName:
		log.Warn("Error: No or invalid commandline arguments, can't run, Bye.")
		disttrace.PrintMasterUsageAndExit(fSet, true)
	case sendHelp:
		disttrace.PrintMasterUsageAndExit(fSet, false)
	}

	// create master configuration
	var pCfg = new(disttrace.GenericConfig)
	pCfg.MasterConfig = new(disttrace.MasterConfig)
	var ppCfg = &pCfg

	log.Info("Main: Launching config poller process...")
	go disttrace.MasterConfigPoller(configFileName, ppCfg)

	log.Info("Main: Launching http server process...")
	go httpServer(ppCfg)

	// wait here until told to quit by os signal
	log.Info("Main: startup finished, going to sleep...")
	disttrace.WaitForOSSignalAndQuit()

	// wait for graceful shutdown of HTTP server
	log.Info("Main: waiting for HTTP server shutdown...")
	<-httpProcQuitDone

	log.Warn("Main: Everything has gracefully ended...")
	log.Warn("Main: Bye.")
}
