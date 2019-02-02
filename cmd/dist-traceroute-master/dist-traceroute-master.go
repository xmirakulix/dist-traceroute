package main

import (
	log "github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"syscall"
)

func main() {

	log.SetLevel(log.InfoLevel)
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

	log.Info("Warn: Everything has gracefully ended...")
	log.Info("Warn: Bye.")
}
