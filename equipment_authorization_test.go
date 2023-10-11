package main

import (
	"crypto/ed25519"
	"reflect"
	"testing"
)

// TestSerialization ensures that a serialized and deserialized EquipmentAuthorization
// have the same values.
func TestSerialization(t *testing.T) {
	pubKey, _ := generateTestKeys()
	ea := &EquipmentAuthorization{
		ShortID:    1,
		PublicKey:  pubKey,
		Capacity:   100,
		Debt:       50,
		Expiration: 50000,
		Signature:  make([]byte, 64),
	}
	data := ea.Serialize()
	deserializedEA, err := Deserialize(data)

	if err != nil {
		t.Errorf("Deserialize returned error: %v", err)
	}

	if !reflect.DeepEqual(ea, deserializedEA) {
		t.Errorf("Original and deserialized structs are not the same: got %v, want %v", deserializedEA, ea)
	}
}

func TestVerifyEquipmentAuthorization(t *testing.T) {
	// Test setup
	dir := generateTestDir(t.Name())
	gcaPrivateKey, err := generateGCATestKeys(dir)
	if err != nil {
		t.Fatal(err)
	}
	server := NewGCAServer(dir)
	defer server.Close()

	// Create and sign a valid EquipmentAuthorization
	ea := &EquipmentAuthorization{
		ShortID:    1,
		PublicKey:  ed25519.PublicKey{},
		Capacity:   100,
		Debt:       0,
		Expiration: 1000,
	}
	eaBytes := ea.Serialize() // Assuming Serialize() doesn't include Signature
	ea.Signature = ed25519.Sign(gcaPrivateKey, eaBytes)

	// Test case 1: Valid EquipmentAuthorization should pass verification
	if err := server.verifyEquipmentAuthorization(ea); err != nil {
		t.Errorf("Failed to verify a valid EquipmentAuthorization: %v", err)
	}

	// Create and sign an invalid EquipmentAuthorization
	eaInvalid := &EquipmentAuthorization{
		ShortID:    2,
		PublicKey:  ed25519.PublicKey{},
		Capacity:   200,
		Debt:       50,
		Expiration: 2000,
	}
	eaInvalidBytes := eaInvalid.Serialize()
	eaInvalid.Signature = ed25519.Sign(gcaPrivateKey, eaInvalidBytes)

	// Tamper with the EquipmentAuthorization to make it invalid
	eaInvalid.Debt = 100

	// Test case 2: Invalid EquipmentAuthorization should fail verification
	if err := server.verifyEquipmentAuthorization(eaInvalid); err == nil {
		t.Errorf("Verified an invalid EquipmentAuthorization without error")
	}
}
