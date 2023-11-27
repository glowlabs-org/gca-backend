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
			HttpPort: 8080,
			TcpPort:  9090,
			UdpPort:  4040,
		},
		glow.PublicKey{5, 4, 3, 2, 1}: {
			Banned:   false,
			Location: "Node B",
			HttpPort: 8000,
			TcpPort:  9000,
			UdpPort:  4000,
		},
	}

	// Serialize the map
	serializedMap, err := SerializeGCAServerMap(originalMap)
	if err != nil {
		t.Fatalf("Serialization failed: %s", err)
	}

	// Deserialize the map
	deserializedMap, err := UntrustedDeserializeGCAServerMap(serializedMap)
	if err != nil {
		t.Fatalf("Deserialization failed: %s", err)
	}

	// Compare the original map with the deserialized map
	if !reflect.DeepEqual(originalMap, deserializedMap) {
		t.Errorf("Deserialized map is different from the original. Got %v, want %v", deserializedMap, originalMap)
	}
}

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
	tooLongMap := map[glow.PublicKey]GCAServer{
		glow.PublicKey{1, 2, 3, 4, 5}: {
			Banned:   true,
			Location: string(make([]byte, 0x10000)), // 65536 bytes, which is 1 more than uint16 max value
			HttpPort: 8080,
			TcpPort:  9090,
			UdpPort:  4040,
		},
	}

	_, err := SerializeGCAServerMap(tooLongMap)
	if err == nil {
		t.Fatalf("Expected serialization to fail for too long location, but it did not")
	}
}

func TestDeserializeMalformedData(t *testing.T) {
	malformedData := []byte{0, 1, 2, 3} // Insufficient bytes to form a proper GCAMap

	_, err := UntrustedDeserializeGCAServerMap(malformedData)
	if err == nil {
		t.Errorf("Expected deserialization to fail for malformed data, but it did not")
	}
}

// Additional tests
func TestPortsSerializationDeserialization(t *testing.T) {
	originalMap := map[glow.PublicKey]GCAServer{
		glow.PublicKey{6, 7, 8, 9, 10}: {
			Banned:   false,
			Location: "Node C",
			HttpPort: 0,     // Test edge case of 0 port
			TcpPort:  65535, // Test edge case of max uint16 port
			UdpPort:  12345,
		},
	}

	serializedMap, err := SerializeGCAServerMap(originalMap)
	if err != nil {
		t.Fatalf("Serialization of ports failed: %s", err)
	}

	deserializedMap, err := UntrustedDeserializeGCAServerMap(serializedMap)
	if err != nil {
		t.Fatalf("Deserialization of ports failed: %s", err)
	}

	if !reflect.DeepEqual(originalMap, deserializedMap) {
		t.Errorf("Deserialized map ports are different from the original. Got %v, want %v", deserializedMap, originalMap)
	}
}

func TestDeserializationIncompleteData(t *testing.T) {
	// Serialize a proper GCAServer first to ensure we know the expected length
	originalMap := map[glow.PublicKey]GCAServer{
		glow.PublicKey{1, 2, 3, 4, 5}: {
			Banned:   true,
			Location: "Node A",
			HttpPort: 8080,
			TcpPort:  9090,
			UdpPort:  4040,
		},
	}

	serializedMap, err := SerializeGCAServerMap(originalMap)
	if err != nil {
		t.Fatalf("Serialization failed: %s", err)
	}

	// Now truncate the serialized data to simulate incomplete data
	truncatedData := serializedMap[:len(serializedMap)-2] // Removing bytes arbitrarily

	_, err = UntrustedDeserializeGCAServerMap(truncatedData)
	if err == nil {
		t.Errorf("Expected deserialization to fail for incomplete data, but it did not")
	}
}
