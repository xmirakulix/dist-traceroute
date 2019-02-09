package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	valid "github.com/asaskevich/govalidator"
	"github.com/sirupsen/logrus"
	"github.com/xmirakulix/dist-traceroute/disttrace"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

// TODO log results to seperate log
// TOOD write access log
// TODO add option to post results to elastic
// TODO https/TLS
// TODO make targets config file path configurable
// TODO fix multiline traces when logging to logfile (e.g. cmdline arg usage)

// logging global
var log = logrus.New()

var httpProcQuitDone = make(chan bool, 1)

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
	log.Warnf("checkCredentials: Unauthorized slave '%v', peer: %v", slaveCreds.Name, req.RemoteAddr)
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
		log.Debug("httpRxResultHandler: Received request results, URL: ", req.URL)

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

		log.Infof("httpRxResultHandler: Received results from slave '%v' for target '%v'. Success: %v, Hops: %v.",
			result.Creds.Name, result.Target.Name,
			result.Success, result.HopCount,
		)

		if ok, err := disttrace.ValidateTraceResult(result); !ok || err != nil {
			log.Warn("httpRxResultHandler: Result validation failed, Error: ", err)
			http.Error(writer, "Result validation failed: "+err.Error(), http.StatusBadRequest)
			return
		}

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
		log.Debug("httpTxConfigHandler: Received request for config, URL: ", req.URL)

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

		log.Infof("httpTxConfigHandler: Transmitting currently configured targets to slave '%v' for %v targets", slaveCreds.Name, len(slaveConf.Targets))
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

	// parse cmdline arguments
	var configNameAndPath, logLevel, logPathAndName string

	// check cmdline args
	{
		var sendHelp bool

		fSet := flag.FlagSet{}
		outBuf := bytes.NewBuffer([]byte{})
		fSet.SetOutput(outBuf)
		fSet.StringVar(&configNameAndPath, "config", "./dt-slaves.json", "Set config `filename`")
		fSet.StringVar(&logPathAndName, "log", "./dt-master.log", "Logfile location `/path/to/file`")
		fSet.StringVar(&logLevel, "loglevel", "info", "Specify loglevel, one of `warn, info, debug`")
		fSet.BoolVar(&sendHelp, "help", false, "display this message")
		fSet.Parse(os.Args[1:])

		var errCfg, errLog error
		configNameAndPath, errCfg = disttrace.CleanAndCheckFileNameAndPath(configNameAndPath)
		logPathAndName, errLog = disttrace.CleanAndCheckFileNameAndPath(logPathAndName)

		// valid cmdline arguments or exit
		switch {
		case errCfg != nil:
			log.Warn("Error: Invalid config file name, can't run, Bye.")
			disttrace.PrintMasterUsageAndExit(fSet, true)
		case errLog != nil:
			log.Warn("Error: Invalid log path specified, can't run, Bye.")
			disttrace.PrintMasterUsageAndExit(fSet, true)
		case logLevel != "warn" && logLevel != "info" && logLevel != "debug":
			log.Warn("Error: Invalid loglevel specified, can't run, Bye.")
			disttrace.PrintMasterUsageAndExit(fSet, true)
		case sendHelp:
			disttrace.PrintMasterUsageAndExit(fSet, false)
		}
	}

	// setup logging
	disttrace.SetLogOptions(log, logPathAndName, logLevel)

	// let's Go! :)
	log.Warn("Main: Starting...")
	disttrace.DebugPrintAllArguments(configNameAndPath, logPathAndName, logLevel)

	// setup listener for OS exit signals
	disttrace.ListenForOSSignals()

	// create master configuration
	var pCfg = new(disttrace.GenericConfig)
	pCfg.MasterConfig = new(disttrace.MasterConfig)
	var ppCfg = &pCfg

	log.Info("Main: Launching config poller process...")
	go disttrace.MasterConfigPoller(configNameAndPath, ppCfg)

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
