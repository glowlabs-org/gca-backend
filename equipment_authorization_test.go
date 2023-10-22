package main

import (
	"testing"
)

// Test function to check the serialization and deserialization
//
// This function serializes and then deserializes an EquipmentAuthorization
// object to ensure that the serialization and deserialization functions work as expected.
func TestEquipmentAuthSerialization(t *testing.T) {
	// Sample EquipmentAuthorization object for testing
	ea := EquipmentAuthorization{
		ShortID:    12345,
		PublicKey:  [32]byte{1, 2, 3, 4, 5}, // Shortened for demonstration
		Capacity:   67890,
		Debt:       111213,
		Expiration: 141516,
		Signature:  [64]byte{17, 18, 19, 20}, // Shortened for demonstration
	}

	// Serialize the object
	serialized := ea.Serialize()

	// Deserialize the object
	deserialized, err := DeserializeEquipmentAuthorization(serialized)
	if err != nil {
		t.Fatal("Error deserializing:", err)
		return
	}

	// Compare the original and deserialized objects
	if ea != deserialized {
		t.Fatal("Serialization and deserialization failed.")
	}
}

// Perform an integration test for the equipment authorizations.
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
	ea := EquipmentAuthorization{
		ShortID:    1,
		PublicKey:  [32]byte{1},
		Capacity:   100,
		Debt:       0,
		Expiration: 1000,
	}
	signingBytes := ea.SigningBytes()
	ea.Signature = Sign(signingBytes, gcaPrivateKey)

	// Test case 1: Valid EquipmentAuthorization should pass verification
	if err := server.verifyEquipmentAuthorization(ea); err != nil {
		t.Errorf("Failed to verify a valid EquipmentAuthorization: %v", err)
	}

	// Create and sign an invalid EquipmentAuthorization
	eaInvalid := EquipmentAuthorization{
		ShortID:    2,
		PublicKey:  [32]byte{2},
		Capacity:   200,
		Debt:       50,
		Expiration: 2000,
	}
	eaInvalidBytes := eaInvalid.SigningBytes()
	ea.Signature = Sign(eaInvalidBytes, gcaPrivateKey)

	// Tamper with the EquipmentAuthorization to make it invalid
	eaInvalid.Debt = 100

	// Test case 2: Invalid EquipmentAuthorization should fail verification
	if err := server.verifyEquipmentAuthorization(eaInvalid); err == nil {
		t.Errorf("Verified an invalid EquipmentAuthorization without error")
	}
}
