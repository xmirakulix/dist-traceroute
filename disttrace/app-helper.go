package disttrace

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

import (
	valid "github.com/asaskevich/govalidator"
)

// OSSigReceived mutex to show OS signal was received
var OSSigReceived = make(chan bool, 1)

// shall we exit?
var doExit = false

// global logger
var log = logrus.New()

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

// SetLogOptions sets the logging detail level
func SetLogOptions(logger *logrus.Logger, logPathAndName string, logLevel string) {
	// use the same logger in disttrace package
	log = logger

	// open logfile or panic!
	file, err := os.OpenFile(logPathAndName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		logger.Out = file
	} else {
		logger.Out = os.Stdout
		log.Panicf("SetLogOptions: Can't open file '%v' for write, Error: %v", logPathAndName, err)
	}

	// set loglevel
	ll, _ := logrus.ParseLevel(logLevel)
	logger.SetLevel(ll)

}

// CleanAndCheckFileNameAndPath validates a path and filename
func CleanAndCheckFileNameAndPath(path string) (string, error) {

	dir, file := filepath.Split(path)

	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}

	if ok, _ := valid.IsFilePath(dir); !ok {
		return "", errors.New("Invalid path: '" + dir + "'")
	}

	if valid.SafeFileName(file) != file {
		return "", errors.New("Invalid filename: '" + file + "'")
	}

	return filepath.Join(dir, file), nil
}

// DebugPrintAllArguments prints all supplied arguments with their valie
func DebugPrintAllArguments(args ...string) {
	log.Debug("Supplied cmdline arguments:")
	for _, arg := range args {
		log.Debugf("    %v", arg)
	}
}
