package main

import (
	"context"
	"flag"
	"log"
	"mursisoy/wordcount/internal/clock"
	"mursisoy/wordcount/internal/worker"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Create a channel to receive signals.
	sigCh := make(chan os.Signal, 1)

	var controllerAddress, listenAddress, id string

	flag.StringVar(&controllerAddress, "controller", "", "The controller address")
	flag.StringVar(&listenAddress, "listen", ":0", "The worker listen port")

	flag.StringVar(&id, "id", "", "The worker id")

	// Enable command-line parsing
	flag.Parse()

	if controllerAddress == "" {
		flag.PrintDefaults()
		log.Fatalf("Controller address is mandatory")
	}

	workerConfig := worker.WorkerConfig{
		ControllerAddress: controllerAddress,
		ListenAddress:     listenAddress,
		LogPriority:       clock.DEBUG,
	}

	// Notify the sigCh channel for SIGINT (Ctrl+C) and SIGTERM (termination) signals.
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	worker := worker.NewWorker(id, workerConfig)

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	// Goroutine to catch shutdown signals
	go func() {
		sig := <-sigCh
		log.Printf("Received signal: %v\n", sig)
		cancel()
	}()

	if _, err := worker.Start(ctx); err != nil {
		cancel()
	}
	<-ctx.Done()
	<-worker.Done()
	log.Printf("Exited\n")
}
