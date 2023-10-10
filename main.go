package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// main is the entry point of the application.
func main() {
	// Initialize a new GCAServer instance.
	gcaServer := NewGCAServer()

	// Create a channel to listen for operating system signals.
	// The channel c is buffered with a size of 1.
	c := make(chan os.Signal, 1)

	// Notify the channel c upon receiving either an Interrupt signal or a SIGTERM signal.
	// This helps us gracefully shut down the application.
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	// Goroutine that waits for an Interrupt or SIGTERM signal.
	// It will call Close() on the GCAServer instance and then exit the program.
	go func() {
		<-c // Block until a signal is received.
		gcaServer.Close() // Close the GCAServer.
		fmt.Println() // Print a newline for cleaner terminal output.
		os.Exit(0)     // Exit the program with a successful status code.
	}()

	// An empty select block is used to keep the main function alive indefinitely.
	// This is necessary because the main function would exit otherwise, killing any child goroutines.
	select {} // Block forever.
}

