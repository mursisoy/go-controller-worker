package main

import (
	"fmt"
	"mursisoy/wordcount/internal/worker"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Create a channel to receive signals.
	sigCh := make(chan os.Signal, 1)

	// Notify the sigCh channel for SIGINT (Ctrl+C) and SIGTERM (termination) signals.
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	worker := worker.NewWorker()

	// Goroutine to catch shutdown signals
	go func() {
		sig := <-sigCh
		fmt.Printf("Received signal: %v\n", sig)
		worker.Shutdown()
	}()

	worker.Start()
	fmt.Printf("Exited\n")
}
