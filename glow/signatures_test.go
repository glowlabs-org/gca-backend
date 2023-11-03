package glow

import (
	"testing"
)

// TestSignAndVerify tests that the Sign and Verify functions work together correctly.
func TestSignAndVerify(t *testing.T) {
	// Define test data
	data := []byte("Hello, world!")
	wrongData := []byte("Hello, everyone!")

	// Generate a key pair and sign some data.
	publicKey, privateKey := GenerateKeyPair()
	_, privateKey2 := GenerateKeyPair()
	signature := Sign(data, privateKey)
	wrongSignature := Sign(data, privateKey2)

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
