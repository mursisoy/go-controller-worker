package main

import (
	"flag"
	"log"
	"mursisoy/wordcount/internal/clock"
	"mursisoy/wordcount/internal/controller"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func main() {
	// Create a channel to receive signals.
	sigCh := make(chan os.Signal, 1)

	var listenAddress, id string
	flag.StringVar(&listenAddress, "listen", ":0", "The worker listen port")

	flag.StringVar(&id, "id", "controller", "The worker id")

	// Enable command-line parsing
	flag.Parse()

	controllerConfig := controller.ControllerConfig{
		ListenAddress: listenAddress,
		LogPriority:   clock.DEBUG,
	}

	// Notify the sigCh channel for SIGINT (Ctrl+C) and SIGTERM (termination) signals.
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	controller := controller.NewController(id, controllerConfig)

	var wg sync.WaitGroup

	wg.Add(1)

	// Goroutine to catch shutdown signals
	go func() {
		defer wg.Done()
		sig := <-sigCh
		log.Printf("Received signal: %v\n", sig)
		<-controller.Shutdown()
	}()

	if _, err := controller.Start(); err != nil {
		log.Fatalf("Error starting controller: %v", err)
	} else {
		wg.Wait()
		log.Printf("Exited\n")
	}

}
