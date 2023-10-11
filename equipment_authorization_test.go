package main

import (
	"testing"
	"reflect"
)

func TestSerialization(t *testing.T) {
	pubKey, _ := generateTestKeys()
	ea := &EquipmentAuthorization{
		ShortID:    1,
		PublicKey:  pubKey,
		Capacity:   100,
		Debt:       50,
		Expiration: 50000,
		Signature:  []byte("test-signature"),
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
