package glow

// testing.go contains exported functions that are intended to be used during
// testing but not in other ways.

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"os"
	"time"
)

// GenerateSecureRandomInt generates a secure random integer between min and max (inclusive).
// It uses the crypto/rand package for secure number generation.
//
// Returns:
// - int: The secure random integer.
// - error: Any error that occurs during the random number generation.
func GenerateSecureRandomInt(min, max int) int {
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

// GenerateTestDir generates a temporary directory path for placing test files.
// The directory will be located in the temp folder of the operating system.
// The path will include the name of the test, the UNIX timestamp, and a
// 6-digit random number.  The function also creates the directory and returns
// the path.
//
// Returns:
// - string: The path of the temporary directory.
func GenerateTestDir(testName string) string {
	// Get the temp directory path for the OS
	tempDir := os.TempDir()

	// Generate the current UNIX timestamp
	unixTime := time.Now().Unix()

	// Generate a 6-digit secure random number
	randNumber := GenerateSecureRandomInt(100000, 999999)

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
