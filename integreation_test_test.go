package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
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
	hash := sha256.Sum256(data)

	// Step 4: Sign the hash using the generated private key
	sig, err := crypto.Sign(hash[:], privKey)
	if err != nil {
		t.Fatalf("Failed to sign data: %v", err)
	}

	// Step 5: Verify the signature using the generated public key
	// Load the public key from the test directory
	pubKeyPath := filepath.Join(testDir, "gca.pubkey") // Use filepath.Join for platform-independent path creation
	pubKeyBytes, err := ioutil.ReadFile(pubKeyPath)
	if err != nil {
		t.Fatalf("Failed to read public key from file: %v", err)
	}
	pubKey, err := crypto.UnmarshalPubkey(pubKeyBytes) // Properly unmarshal the public key
	if err != nil {
		t.Fatalf("Failed to unmarshal public key: %v", err)
	}

	// Perform the signature verification using crypto.SigToPub
	recoveredPubKey, err := crypto.SigToPub(hash[:], sig)
	if err != nil {
		t.Fatalf("Failed to recover public key from signature: %v", err)
	}

	// Compare the recovered public key with the generated public key
	if hex.EncodeToString(crypto.FromECDSAPub(pubKey)) != hex.EncodeToString(crypto.FromECDSAPub(recoveredPubKey)) {
		t.Fatalf("Recovered public key should match the generated public key")
	}

	// Step 6: Use crypto.VerifySignature to confirm that the signature is valid
	isValidSignature := crypto.VerifySignature(pubKeyBytes, hash[:], sig[:len(sig)-1]) // Exclude the V value at the end of the signature
	if !isValidSignature {
		t.Fatalf("Signature is not valid")
	}
}
