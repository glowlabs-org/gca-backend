package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/glowlabs-org/gca-backend/client"
)

func main() {
	// Create a new client, using the current directory as the basedir.
	c, err := client.NewClient(".")
	if err != nil {
		fmt.Println("unable to create client: ", err)
		return
	}

	// Wait for a shutdown signal from the OS.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	// Close the client.
	err = c.Close()
	if err != nil {
		fmt.Println("Issue during shutdown:", err)
	}
}
