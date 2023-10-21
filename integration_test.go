package main

// main_test.go contains a set of helpers for the various test files in this package.

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

// generateSecureRandomInt generates a secure random integer between min and max (inclusive).
// It uses the crypto/rand package for secure number generation.
//
// Returns:
// - int: The secure random integer.
// - error: Any error that occurs during the random number generation.
func generateSecureRandomInt(min, max int) (int, error) {
	// Calculate the range
	rangeSize := max - min + 1

	// Generate a secure random number
	var n uint32
	err := binary.Read(rand.Reader, binary.LittleEndian, &n)
	if err != nil {
		return 0, err
	}

	// Map the number to the desired range
	return int(n)%rangeSize + min, nil
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
	randNumber, err := generateSecureRandomInt(100000, 999999)
	if err != nil {
		panic(err)
	}

	// Construct the directory name using the test name, UNIX timestamp, and random number
	dirName := fmt.Sprintf("%s-%d-%d", testName, unixTime, randNumber)

	// Full path of the temporary directory
	fullPath := fmt.Sprintf("%s/%s", tempDir, dirName)

	// Create the temporary directory
	err = os.MkdirAll(fullPath, 0755)
	if err != nil {
		panic(err)
	}

	return fullPath
}

// generateGCATestKeys creates a new ECDSA key pair for the GCA using the secp256k1 curve,
// saves the public key into a file named "gca.pubkey" within the specified directory,
// and returns the private key.
//
// dir specifies the directory where the public key will be stored.
// If an error occurs, it returns nil along with the error.
func generateGCATestKeys(dir string) (PrivateKey, error) {
	// Generate a new ECDSA key pair with secp256k1 curve
	pubKey, privKey := GenerateKeyPair()

	// Make sure the directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.MkdirAll(dir, 0755)
	}

	// Construct the path where the public key should be saved
	pubKeyPath := filepath.Join(dir, "gca.pubkey")

	// Save the public key to a file
	if err := ioutil.WriteFile(pubKeyPath, pubKey[:], 0644); err != nil {
		return PrivateKey{}, fmt.Errorf("failed to write public key to file: %v", err)
	}

	return privKey, nil
}

// loadEquipmentAuths is responsible for populating the equipment map
// using the provided array of EquipmentAuths.
func (gcas *GCAServer) loadEquipmentAuth(ea EquipmentAuthorization) {
	// Add the equipment's public key to the equipment map using its ShortID as the key
	gcas.equipment[ea.ShortID] = ea
	gcas.addRecentEquipmentAuth(ea)
}