package main

// This file contains some helper methods that are applicable across the whole
// test suite. The functions in this file are useful for testing in general and
// not specific to testing any particular function of the GCA server. Most test
// suite helper functions can be found in the respective test file that tests
// the core component that the helper function relates to.

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"os"
	"time"

	"github.com/glowlabs-org/gca-backend/glow"
)

// setupTestEnvironment will return a fully initialized gca server that is
// ready to be used.
func setupTestEnvironment(testName string) (gcas *GCAServer, dir string, gcaPrivKey glow.PrivateKey, err error) {
	dir = generateTestDir(testName)
	server, tempPrivKey, err := gcaServerWithTempKey(dir)
	if err != nil {
		return nil, "", glow.PrivateKey{}, fmt.Errorf("unable to create gca server with temp key: %v", err)
	}
	gcaPrivKey, err = server.submitGCAKey(tempPrivKey)
	if err != nil {
		return nil, "", glow.PrivateKey{}, fmt.Errorf("unable to submit gca priv key: %v", err)
	}
	return server, dir, gcaPrivKey, nil
}

// generateTestDir generates a temporary directory path for placing test files.
// The directory will be located in the temp folder of the operating system.
// The path will include the name of the test, the UNIX timestamp, and a 6-digit random number.
// The function also creates the directory and returns the path.
//
// Returns:
// - string: The path of the temporary directory.
func generateTestDir(testName string) string {
	// Get the temp directory path for the OS
	tempDir := os.TempDir()

	// Generate the current UNIX timestamp
	unixTime := time.Now().Unix()

	// Generate a 6-digit secure random number
	randNumber := generateSecureRandomInt(100000, 999999)

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

// generateSecureRandomInt generates a secure random integer between min and max (inclusive).
// It uses the crypto/rand package for secure number generation.
//
// Returns:
// - int: The secure random integer.
// - error: Any error that occurs during the random number generation.
func generateSecureRandomInt(min, max int) int {
	// Calculate the range
	rangeSize := max - min + 1

	// Generate a secure random number
	var n uint32
	err := binary.Read(rand.Reader, binary.LittleEndian, &n)
	if err != nil {
		panic("secure random number generation is not working")
	}

	// Map the number to the desired range
	return int(n)%rangeSize + min
}
