package main

import (
	"fmt"
	"mursisoy/wordcount/internal/controller"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Create a channel to receive signals.
	sigCh := make(chan os.Signal, 1)

	// Notify the sigCh channel for SIGINT (Ctrl+C) and SIGTERM (termination) signals.
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	controller := controller.NewController()

	// Goroutine to catch shutdown signals
	go func() {
		sig := <-sigCh
		fmt.Printf("Received signal: %v\n", sig)
		controller.Shutdown()
	}()

	controller.Start()
	fmt.Printf("Exited\n")
}
