package main

import (
	"os"
	"os/signal"
	"syscall"
)

func main() {
	gcaServer := NewGCAServer()

	devices := []Device{
		// Sample device setup.
		// Add devices here as per your requirement.
		// Device{ShortID: 12345, Key: somePublicKey},
	}
	gcaServer.loadDeviceKeys(devices)

	// Signal handling to call the Close method on termination.
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		gcaServer.Close()
		os.Exit(0)
	}()

	select {} // This keeps the main routine running indefinitely to keep the program alive.
}
