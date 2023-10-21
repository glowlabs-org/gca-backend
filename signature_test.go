package main

import (
	"testing"
)

// TestSignAndVerify tests that the Sign and Verify functions work together correctly.
func TestSignAndVerify(t *testing.T) {
	// Define test data
	data := []byte("Hello, world!")
	wrongData := []byte("Hello, everyone!")

	// Generate a key pair and sign some data.
	publicKey, privateKey := generateKeyPair()
	_, privateKey2 := generateKeyPair()
	signature, err := Sign(data, privateKey)
	if err != nil {
		t.Fatalf("Failed to sign data: %v", err)
	}
	wrongSignature, err := Sign(data, privateKey2)
	if err != nil {
		t.Fatalf("Failed to sign data: %v", err)
	}

	// Verify the signature
	isValid := Verify(publicKey, data, signature)
	if !isValid {
		t.Fatalf("Signature is not valid")
	}
	isValid = Verify(publicKey, wrongData, signature)
	if isValid {
		t.Fatalf("Signature is incorrectly valid")
	}
	isValid = Verify(publicKey, data, wrongSignature)
	if isValid {
		t.Fatalf("Signature is incorrectly valid")
	}
	signature[4]++
	isValid = Verify(publicKey, data, signature)
	if isValid {
		t.Fatalf("Signature is incorrectly valid")
	}
}

