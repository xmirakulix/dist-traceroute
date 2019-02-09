package disttrace

import (
	"bytes"
	"encoding/json"
	"flag"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"syscall"
)

// OSSigReceived mutex to show OS signal was received
var OSSigReceived = make(chan bool, 1)

var doExit = false

// ListenForOSSignals registers for OS signals and waits for them
func ListenForOSSignals() {

	osSignal := make(chan os.Signal, 1)

	signal.Notify(osSignal, syscall.SIGINT, syscall.SIGTERM)

	// wait for signal in background...
	go func() {
		sig := <-osSignal
		log.Warn("ListenForOSSignals: Received os signal: ", sig)
		OSSigReceived <- true
	}()

}

// WaitForOSSignalAndQuit blocks until a signal from OS is received, then sends exit signal
func WaitForOSSignalAndQuit() {
	<-OSSigReceived
	log.Warn("WaitForOSSignalAndQuit: Sending exit signal to all processes...")
	QuitGracefully()
}

// QuitGracefully sets exit signal for everyone
func QuitGracefully() {
	doExit = true
}

// CheckForQuit checks if should initiate quit
func CheckForQuit() bool {
	return doExit
}

// printUsage prints usage instructions for cmdline arguments
func printUsage(fSet flag.FlagSet) {

	buf := bytes.NewBuffer([]byte{})
	fSet.SetOutput(buf)
	fSet.PrintDefaults()

	log.Warn("Usage: \n", string(buf.Bytes()))
	log.Warn()

}

// PrintMasterUsageAndExit prints usage instructions for cmdline arguments
func PrintMasterUsageAndExit(fSet flag.FlagSet, exitWithError bool) {
	printUsage(fSet)

	switch exitWithError {
	case true:
		os.Exit(1)
	default:
		os.Exit(0)
	}
}

// PrintSlaveUsageAndExit prints usage instructions for cmdline arguments
func PrintSlaveUsageAndExit(fSet flag.FlagSet, exitWithError bool) {

	printUsage(fSet)

	// create sample config file
	def := new(SlaveConfig)
	targets := make(map[uuid.UUID]TraceTarget)
	targets[uuid.New()] = TraceTarget{}
	def.Targets = targets
	defJSON, _ := json.MarshalIndent(def, "", "  ")

	log.Warn("Sample configuration file: \n", string(defJSON))

	switch exitWithError {
	case true:
		os.Exit(1)
	default:
		os.Exit(0)
	}
}

// SetLogLevel sets the logging detail level
func SetLogLevel(logLevel string) {
	ll, _ := log.ParseLevel(logLevel)
	log.SetLevel(ll)
}
