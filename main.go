package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	gcaServer := NewGCAServer()

	// Signal handling to call the Close method on termination.
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		gcaServer.Close()
		fmt.Println()
		os.Exit(0)
	}()

	select {} // This keeps the main routine running indefinitely to keep the program alive.
}
