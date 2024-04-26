package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

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
			// Print the event log to the terminal.
			fmt.Println("Event Log Dump", time.Now().UTC().Format("2006-01-02 15:04:05 UTC"))
			fmt.Println(c.Log.String())
		case syscall.SIGUSR2:
			// Write the event log to a file "event.log" in the client directory.
			path := filepath.Join(baseDir, "event.log")
			fmt.Println("Event Log Dump to file", path)
			s := fmt.Sprintf("Event Log Dump %v\n%v", time.Now().UTC().Format("2006-01-02 15:04:05 UTC"), c.Log.String())
			os.WriteFile(path, []byte(s), 0644)
		}
	}

	// Close the client.
	err = c.Close()
	if err != nil {
		fmt.Println("Issue during shutdown:", err)
	}
}
