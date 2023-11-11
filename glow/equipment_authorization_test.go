package glow

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
