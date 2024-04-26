package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/glowlabs-org/gca-backend/client"
)

func main() {
	// Create a new client, using the current directory as the basedir.
	baseDir := "/opt/glow-monitor/"
	c, err := client.NewClient(baseDir)
	if err != nil {
		fmt.Println("unable to create client: ", err)
		return
	}

	// Wait for a shutdown signal from the OS.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1, syscall.SIGUSR2)

	done := false
	for !done {
		s := <-sigChan
		switch s {
		case syscall.SIGINT:
			done = true
		case syscall.SIGTERM:
			done = true
		case syscall.SIGUSR1:
			// Dump status to the terminal.
			fmt.Printf("%v", c.DumpStatus())
		case syscall.SIGUSR2:
			// Write the status to a file "status.txt" in the client directory. File will be
			// created and/or truncated before writing.
			path := filepath.Join(baseDir, "status.txt")
			fmt.Printf("Dumping server status to %v\n", path)
			os.WriteFile(path, []byte(c.DumpStatus()), 0644)
		}
	}

	// Close the client.
	err = c.Close()
	if err != nil {
		fmt.Println("Issue during shutdown:", err)
	}
}
