package main

import (
	"bytes"
	"flag"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/xmirakulix/dist-traceroute/disttrace"
)

// MAYBE log results to seperate log
// MAYBE add option to post results to elastic
// TODO https/TLS
// TODO slave shutdown takes too long during measurements
// TODO cleanup when deleting slaves or targets
// TODO store failed traceroutes as well

// TODO GUI refresh trace history together with status on home page
// TODO GUI properly validate target dest address
// TODI GUI display recent slave activity

// global logger
var log = logrus.New()

var db *disttrace.DB

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

		var errMainLog, errAccessLog, errDb error
		mainLogNameAndPath, errMainLog = disttrace.CleanAndCheckFileNameAndPath(mainLogNameAndPath)
		accessLogNameAndPath, errAccessLog = disttrace.CleanAndCheckFileNameAndPath(accessLogNameAndPath)
		dbNameAndPath, errDb = disttrace.CleanAndCheckFileNameAndPath(dbNameAndPath)

		// valid cmdline arguments or exit
		switch {
		case errMainLog != nil || errAccessLog != nil:
			log.Warn("Error: Invalid log path specified, can't run, Bye.")
			disttrace.PrintUsageAndExit(fSet, true)
		case errDb != nil:
			log.Warn("Error: Invalid database path specified, can't run, Bye.")
			disttrace.PrintUsageAndExit(fSet, true)
		case logLevel != "warn" && logLevel != "info" && logLevel != "debug":
			log.Warn("Error: Invalid loglevel specified, can't run, Bye.")
			disttrace.PrintUsageAndExit(fSet, true)
		case sendHelp:
			disttrace.PrintUsageAndExit(fSet, false)
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
	disttrace.AlertInfof("master", "Startup complete...")
	disttrace.WaitForOSSignalAndQuit()

	// wait for graceful shutdown of HTTP server
	log.Info("Main: waiting for HTTP server shutdown...")
	<-httpProcQuitDone

	log.Warn("Main: Everything has gracefully ended...")
	log.Warn("Main: Bye.")
}
