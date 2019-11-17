package main

import (
	"bytes"
	"context"
	"flag"
	"io"
	"net/http"
	"os"
	"time"

	ghandlers "github.com/gorilla/handlers"
	"github.com/sirupsen/logrus"
	"github.com/urfave/negroni"
	"github.com/xmirakulix/dist-traceroute/disttrace"
)

// TODO log results to seperate log
// TODO add option to post results to elastic
// TODO https/TLS
// TODO fix multiline traces when logging to logfile (e.g. cmdline arg usage)

// TODO GUI: refresh trace history together with status on home page

// global logger
var log = logrus.New()

var httpProcQuitDone = make(chan bool, 1)

// status vars for webinterface
var lastTransmittedSlaveConfig = "none yet"
var lastTransmittedSlaveConfigTime time.Time

var db *disttrace.DB

func httpServer(accessLog string) {
	var err error

	log.Info("httpServer: Start...")

	var accessWriter io.Writer
	if accessWriter, err = os.OpenFile(accessLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err != nil {
		log.Panicf("httpServer: Can't open access log '%v', Error: %v", accessLog, err)
	}

	// handle slaves
	slaveRouter := http.NewServeMux()
	slaveRouter.HandleFunc("/slave/results", httpHandleSlaveResults())
	slaveRouter.HandleFunc("/slave/config", httpHandleSlaveConfig())

	// handle api requests from webinterface
	authRouter := http.NewServeMux()
	authRouter.HandleFunc("/api/status", httpHandleAPIStatus())
	authRouter.HandleFunc("/api/traces", httpHandleAPITraceHistory())
	authRouter.HandleFunc("/api/graph", httpHandleAPIGraphData())

	authHandler := negroni.New()
	authHandler.Use(negroni.HandlerFunc(checkJWTAuth))
	authHandler.UseHandler(authRouter)

	// handle everything else
	rootRouter := http.NewServeMux()
	rootRouter.HandleFunc("/", httpDefaultHandler())
	rootRouter.HandleFunc("/api/auth", httpHandleAPIAuth())
	rootRouter.Handle("/slave/", slaveRouter)
	rootRouter.Handle("/api/", authHandler)

	// register middleware for all requests
	rootHandler := negroni.New()
	rootHandler.Use(negroni.HandlerFunc(handleAccessControl))
	rootHandler.Use(negroni.Wrap(ghandlers.CombinedLoggingHandler(accessWriter, rootRouter)))

	srv := &http.Server{
		Addr:    ":8990",
		Handler: rootHandler,
	}

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
	var mainLogNameAndPath, accessLogNameAndPath string
	var dbNameAndPath string
	var logLevel string

	// check cmdline args
	{
		var sendHelp bool

		fSet := flag.FlagSet{}
		outBuf := bytes.NewBuffer([]byte{})
		fSet.SetOutput(outBuf)
		fSet.StringVar(&dbNameAndPath, "db", "./disttrace.db", "Set database `filename`")
		fSet.StringVar(&mainLogNameAndPath, "log", "./master.log", "Main logfile location `/path/to/file`")
		fSet.StringVar(&accessLogNameAndPath, "accesslog", "./access.log", "HTTP access logfile location `/path/to/file`")
		fSet.StringVar(&logLevel, "loglevel", "info", "Specify loglevel, one of `warn, info, debug`")
		fSet.BoolVar(&sendHelp, "help", false, "display this message")
		fSet.Parse(os.Args[1:])

		var errMasterCfg, errTargetsCfg, errMainLog, errAccessLog, errDb error
		mainLogNameAndPath, errMainLog = disttrace.CleanAndCheckFileNameAndPath(mainLogNameAndPath)
		accessLogNameAndPath, errAccessLog = disttrace.CleanAndCheckFileNameAndPath(accessLogNameAndPath)
		dbNameAndPath, errDb = disttrace.CleanAndCheckFileNameAndPath(dbNameAndPath)

		// valid cmdline arguments or exit
		switch {
		case errMasterCfg != nil || errTargetsCfg != nil:
			log.Warn("Error: Invalid config file name, can't run, Bye.")
			disttrace.PrintMasterUsageAndExit(fSet, true)
		case errMainLog != nil || errAccessLog != nil:
			log.Warn("Error: Invalid log path specified, can't run, Bye.")
			disttrace.PrintMasterUsageAndExit(fSet, true)
		case errDb != nil:
			log.Warn("Error: Invalid database path specified, can't run, Bye.")
			disttrace.PrintMasterUsageAndExit(fSet, true)
		case logLevel != "warn" && logLevel != "info" && logLevel != "debug":
			log.Warn("Error: Invalid loglevel specified, can't run, Bye.")
			disttrace.PrintMasterUsageAndExit(fSet, true)
		case sendHelp:
			disttrace.PrintMasterUsageAndExit(fSet, false)
		}
	}

	// setup logging
	disttrace.SetLogOptions(log, mainLogNameAndPath, logLevel)

	// let's Go! :)
	log.Warn("Main: Starting...")
	disttrace.DebugPrintAllArguments(mainLogNameAndPath, logLevel)

	// setup listener for OS exit signals
	disttrace.ListenForOSSignals()

	// init database connection
	{
		var err error
		if db, err = disttrace.InitDBConnectionAndUpdate(dbNameAndPath); err != nil {
			log.Fatal("Main: Couldn't initiate database connection! Error: ", err)
		}
		log.Info("Main: Database connection initiated...")
	}

	log.Info("Main: Launching http server process...")
	go httpServer(accessLogNameAndPath)

	// wait here until told to quit by os signal
	log.Info("Main: startup finished, going to sleep...")
	disttrace.WaitForOSSignalAndQuit()

	// wait for graceful shutdown of HTTP server
	log.Info("Main: waiting for HTTP server shutdown...")
	<-httpProcQuitDone

	log.Warn("Main: Everything has gracefully ended...")
	log.Warn("Main: Bye.")
}
