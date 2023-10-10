package main

import (
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"syscall"
	"time"
)

// generateTestDir generates a temporary directory path for placing test files.
// The directory will be located in the temp folder of the operating system.
// The path will include the name of the test, the UNIX timestamp, and a 6-digit random number.
// The function also creates the directory and returns the path.
//
// Returns:
// - string: The path of the temporary directory.
// - error: Any error that occurs during the directory creation.
func generateTestDir(testName string) string {
	// Get the temp directory path for the OS
	tempDir := os.TempDir()

	// Generate the current UNIX timestamp
	unixTime := time.Now().Unix()

	// Generate a 6-digit random number
	randNumber := rand.Intn(999999-100000) + 100000

	// Construct the directory name using the test name, UNIX timestamp, and random number
	dirName := fmt.Sprintf("%s-%d-%d", testName, unixTime, randNumber)

	// Full path of the temporary directory
	fullPath := fmt.Sprintf("%s/%s", tempDir, dirName)

	// Create the temporary directory
	err := os.MkdirAll(fullPath, 0755)
	if err != nil {
		panic(err)
	}

	return fullPath
}

// main is the entry point of the application.
func main() {
	// Get the user's home directory in an OS-agnostic manner.
	usr, err := user.Current()
	if err != nil {
		fmt.Println("Error obtaining user's home directory:", err)
		os.Exit(1)
	}

	// Create the server directory path within the user's home directory.
	serverDir := filepath.Join(usr.HomeDir, "gca-server")

	// Initialize a new GCAServer instance with the server directory.
	gcaServer := NewGCAServer(serverDir)

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

