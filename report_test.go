package main

import (
	"bytes"
	"testing"
)

// TestSerializationAndDeserialization aims to test both Serialize and Deserialize functions.
// It ensures that an EquipmentReport can be serialized and then deserialized to produce
// the same data.
func TestSerializationAndDeserialization(t *testing.T) {
	// Create a sample EquipmentReport for testing.
	originalReport := EquipmentReport{
		ShortID:     1,
		Timeslot:    42,
		PowerOutput: 100,
		Signature:   [64]byte{1, 2, 3}, // Sample values, actual signature generation would be more complex.
	}

	// Serialize the EquipmentReport.
	serialized := originalReport.Serialize()

	// Deserialize the byte slice back into an EquipmentReport.
	deserializedReport, err := DeserializeReport(serialized)
	if err != nil {
		t.Errorf("Deserialization failed: %v", err)
		return
	}

	// Compare the original and deserialized EquipmentReports.
	if originalReport.ShortID != deserializedReport.ShortID ||
		originalReport.Timeslot != deserializedReport.Timeslot ||
		originalReport.PowerOutput != deserializedReport.PowerOutput ||
		!bytes.Equal(originalReport.Signature[:], deserializedReport.Signature[:]) {
		t.Errorf("Original and deserialized EquipmentReports are not identical")
	}
}
