package client

import (
	"reflect"
	"testing"

	"github.com/glowlabs-org/gca-backend/glow"
)

func TestSerializationDeserialization(t *testing.T) {
	// Create a test map
	originalMap := map[glow.PublicKey]GCAServer{
		glow.PublicKey{1, 2, 3, 4, 5}: { // Just an example key
			Banned:   true,
			Location: "Node A",
		},
		glow.PublicKey{5, 4, 3, 2, 1}: {
			Banned:   false,
			Location: "Node B",
		},
	}

	// Serialize the map
	serializedMap, err := SerializeGCAServerMap(originalMap)
	if err != nil {
		t.Fatalf("Serialization failed: %s", err)
	}

	// Deserialize the map
	deserializedMap, err := DeserializeGCAServerMap(serializedMap)
	if err != nil {
		t.Fatalf("Deserialization failed: %s", err)
	}

	// Compare the original map with the deserialized map
	if !reflect.DeepEqual(originalMap, deserializedMap) {
		t.Errorf("Deserialized map is different from the original. Got %v, want %v", deserializedMap, originalMap)
	}
}

// We could also add more tests to check edge cases, for example:
func TestEmptyMapSerialization(t *testing.T) {
	emptyMap := map[glow.PublicKey]GCAServer{}

	serializedMap, err := SerializeGCAServerMap(emptyMap)
	if err != nil {
		t.Fatalf("Serialization failed: %s", err)
	}

	if len(serializedMap) != 0 {
		t.Errorf("Serialized empty map should be empty, got %d bytes", len(serializedMap))
	}
}

func TestLocationLengthLimit(t *testing.T) {
	// Create a test map with a location string that is too long
	tooLongMap := map[glow.PublicKey]GCAServer{
		glow.PublicKey{1, 2, 3, 4, 5}: {
			Banned:   true,
			Location: string(make([]byte, 0x10000)), // 65536 bytes, which is 1 more than uint16 max value
		},
	}

	_, err := SerializeGCAServerMap(tooLongMap)
	if err == nil {
		t.Fatalf("Expected serialization to fail for too long location, but it did not")
	}
}

// Test cases for malformed data could also be added, such as:
func TestDeserializeMalformedData(t *testing.T) {
	malformedData := []byte{0, 1, 2, 3} // Insufficient bytes to form a proper GCAMap

	_, err := DeserializeGCAServerMap(malformedData)
	if err == nil {
		t.Errorf("Expected deserialization to fail for malformed data, but it did not")
	}
}

// Run all tests with verbose mode
// $ go test -v
