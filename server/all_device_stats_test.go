package server

import (
	"math"
	"math/rand"
	"reflect"
	"testing"
)

// Helper function to create a random DeviceStats object
func createRandomDeviceStats() DeviceStats {
	var ds DeviceStats
	for i := range ds.PublicKey {
		ds.PublicKey[i] = byte(rand.Intn(256))
	}
	for i := range ds.PowerOutputs {
		ds.PowerOutputs[i] = rand.Uint64()
	}
	for i := range ds.ImpactRates {
		ds.ImpactRates[i] = rand.Float64() * math.MaxFloat64
	}
	return ds
}

func TestSerializeDeserialize(t *testing.T) {
	// Creating a test object
	var testADS AllDeviceStats
	testADS.TimeslotOffset = rand.Uint32()
	for i := 0; i < 10; i++ { // example with 10 devices
		testADS.Devices = append(testADS.Devices, createRandomDeviceStats())
	}
	for i := range testADS.Signature {
		testADS.Signature[i] = byte(rand.Intn(256))
	}

	// Serialize
	serializedData := testADS.Serialize()

	// Deserialize
	deserializedADS, _, err := DeserializeStreamAllDeviceStats(serializedData)
	if err != nil {
		t.Fatalf("Deserialization failed: %v", err)
	}

	// Compare
	if !reflect.DeepEqual(testADS, deserializedADS) {
		t.Errorf("Original and deserialized AllDeviceStats do not match.")
	}
}

func TestEmptyData(t *testing.T) {
	var emptyADS AllDeviceStats
	serializedData := emptyADS.Serialize()
	deserializedADS, _, err := DeserializeStreamAllDeviceStats(serializedData)
	if err != nil {
		t.Fatalf("Deserialization failed for empty data: %v", err)
	}

	// Compare individual fields
	if len(emptyADS.Devices) != len(deserializedADS.Devices) {
		t.Errorf("Devices length do not match. Original: %d, Deserialized: %d", len(emptyADS.Devices), len(deserializedADS.Devices))
	}
	if emptyADS.TimeslotOffset != deserializedADS.TimeslotOffset {
		t.Errorf("TimeslotOffset does not match. Original: %d, Deserialized: %d", emptyADS.TimeslotOffset, deserializedADS.TimeslotOffset)
	}
	if !reflect.DeepEqual(emptyADS.Signature, deserializedADS.Signature) {
		t.Errorf("Signature does not match. Original: %v, Deserialized: %v", emptyADS.Signature, deserializedADS.Signature)
	}
}

func TestErrorHandling(t *testing.T) {
	_, _, err := DeserializeStreamAllDeviceStats([]byte{1, 2, 3}) // Insufficient data
	if err == nil {
		t.Errorf("Expected an error for insufficient data, but got none")
	}
}
