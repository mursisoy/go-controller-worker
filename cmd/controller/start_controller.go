package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/mursisoy/go-controller-worker/pkg/clock"
	"github.com/mursisoy/go-controller-worker/pkg/controller"
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
		ClockLogConfig: clock.ClockLogConfig{
			Priority:    clock.DEBUG,
			FileOutput:  true,
			LogFilename: fmt.Sprintf("%s-c.log", id),
		},
		FailureDetectorConfig: controller.FailureDetectorConfig{
			HeartBeatTicker: 20 * time.Second,
			ClockLogConfig: clock.ClockLogConfig{
				Priority:    clock.DEBUG,
				FileOutput:  true,
				LogFilename: fmt.Sprintf("%s-fd.log", id),
			},
		},
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
