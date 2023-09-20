package main

import (
	"flag"
	"log"
	"mursisoy/wordcount/internal/controller"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Create a channel to receive signals.
	sigCh := make(chan os.Signal, 1)

	var listenAddress, id string
	flag.StringVar(&listenAddress, "listen", ":0", "The worker listen port")

	flag.StringVar(&id, "id", "", "The worker id")

	// Enable command-line parsing
	flag.Parse()

	controllerConfig := controller.ControllerConfig{
		ListenAddress: listenAddress,
	}

	// Notify the sigCh channel for SIGINT (Ctrl+C) and SIGTERM (termination) signals.
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	controller := controller.NewController(id, controllerConfig)

	// Goroutine to catch shutdown signals
	go func() {
		sig := <-sigCh
		log.Printf("Received signal: %v\n", sig)
		controller.Shutdown()
	}()

	controller.Start()
	log.Printf("Exited\n")
}
