package main

import (
	"flag"
	"log"
	"mursisoy/wordcount/internal/worker"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Create a channel to receive signals.
	sigCh := make(chan os.Signal, 1)

	var controllerAddress, listenAddress string

	flag.StringVar(&controllerAddress, "controller", "", "The controller address")
	flag.StringVar(&listenAddress, "listen", ":0", "The worker listen port")

	id := flag.String("id", "", "The worker id")

	// Enable command-line parsing
	flag.Parse()

	if controllerAddress == "" {
		flag.PrintDefaults()
		log.Fatalf("Controller address is mandatory")
	}

	workerConfig := worker.WorkerConfig{
		ControllerAddress: controllerAddress,
		ListenAddress:     listenAddress,
	}

	// Notify the sigCh channel for SIGINT (Ctrl+C) and SIGTERM (termination) signals.
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	worker := worker.NewWorker(*id, workerConfig)

	// Goroutine to catch shutdown signals
	go func() {
		sig := <-sigCh
		log.Printf("Received signal: %v\n", sig)
		worker.Shutdown()
	}()

	worker.Start()
	log.Printf("Exited\n")
}
