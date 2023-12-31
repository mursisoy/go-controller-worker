package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/mursisoy/go-controller-worker/pkg/clock"
	"github.com/mursisoy/go-controller-worker/pkg/worker"
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
		ClockLogConfig: clock.ClockLogConfig{
			Priority:    clock.DEBUG,
			FileOutput:  true,
			LogFilename: fmt.Sprintf("%s-w.log", id),
		},
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
