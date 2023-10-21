package main

import (
	"io/ioutil"
	"path/filepath"
	"testing"
)

// TestGenerateGCATestKeys is a test function for generateGCATestKeys.
// It generates an ECDSA key pair, signs some data using the private key,
// and verifies the signature using the generated public key.
func TestGenerateGCATestKeys(t *testing.T) {
	// Step 1: Generate the temporary test directory
	testDir := generateTestDir("TestGenerateGCATestKeys")

	// Step 2: Generate the ECDSA key pair using generateGCATestKeys
	privKey, err := generateGCATestKeys(testDir)
	if err != nil {
		t.Fatalf("Failed to generate ECDSA key pair: %v", err)
	}

	// Step 3: Create some data to sign
	data := []byte("Hello, world!")
	// Step 4: Sign the hash using the generated private key
	sig := Sign(data, privKey)

	// Step 5: Verify the signature using the generated public key
	// Load the public key from the test directory
	pubKeyPath := filepath.Join(testDir, "gca.pubkey") // Use filepath.Join for platform-independent path creation
	pubKeyBytes, err := ioutil.ReadFile(pubKeyPath)
	if err != nil {
		t.Fatalf("Failed to read public key from file: %v", err)
	}
	var pubKey PublicKey
	copy(pubKey[:], pubKeyBytes)

	// Step 6: Use Verify to confirm that the signature is valid
	isValidSignature := Verify(pubKey, data, sig) // Exclude the V value at the end of the signature
	if !isValidSignature {
		t.Fatalf("Signature is not valid")
	}
}
