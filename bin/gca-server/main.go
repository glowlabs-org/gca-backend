package main

// This file launches a GCA server. Most of the work is being done in
// 'NewGCAServer()', the main purpose of this file is to set up OS related
// tasks such as creating the homedir for the server and listening for quit
// signals from the OS.

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/glowlabs-org/gca-backend/server"
)

// main is the entry point of the application.
func main() {
	// Get the user's home directory in an OS-agnostic manner.
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Error obtaining user's home directory:", err)
		os.Exit(1)
	}

	// Create the server directory path within the user's home directory.
	serverDir := filepath.Join(homeDir, "gca-server")

	// Initialize a new GCAServer instance with the server directory.
	gcaServer, err := server.NewGCAServer(serverDir)
	if err != nil {
		fmt.Println("Unable to launch GCA server:", err)
		os.Exit(1)
	}

	// Create a channel to listen for operating system signals.
	// The channel c is buffered with a size of 1.
	c := make(chan os.Signal, 1)

	// Notify the channel c upon receiving either an Interrupt signal or a SIGTERM signal.
	// This helps us gracefully shut down the application.
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	// Goroutine that waits for an Interrupt or SIGTERM signal.
	// It will call Close() on the GCAServer instance and then exit the program.
	go func() {
		<-c               // Block until a signal is received.
		gcaServer.Close() // Close the GCAServer.
		fmt.Println()     // Print a newline for cleaner terminal output.
		os.Exit(0)        // Exit the program with a successful status code.
	}()

	// An empty select block is used to keep the main function alive indefinitely.
	// This is necessary because the main function would exit otherwise, killing any child goroutines.
	select {} // Block forever.
}
