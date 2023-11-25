package glow

// This file contains helper functions related to managing equipment
// authorizations.

import (
	"encoding/binary"
	"fmt"
	"math"
)

// EquipmentAuthorization struct reflects an authorization request,
// except PublicKey and Signature are byte slices for the secp256k1 algorithm.
type EquipmentAuthorization struct {
	ShortID    uint32
	PublicKey  [32]byte
	Latitude   float64
	Longitude  float64
	Capacity   uint64
	Debt       uint64
	Expiration uint32
	Signature  [64]byte
}

// SigningBytes generates the byte slice for signing.
//
// This function is similar to Serialize, but it excludes the Signature field
// and adds the "EquipmentAuthorization" prefix to the output.
// The byte slice returned is intended for signing.
func (ea *EquipmentAuthorization) SigningBytes() []byte {
	prefix := "EquipmentAuthorization"
	prefixBytes := []byte(prefix)
	data := make([]byte, len(prefixBytes)+4+32+8+8+8+8+4)
	copy(data[0:len(prefixBytes)], prefixBytes)
	binary.LittleEndian.PutUint32(data[len(prefixBytes):len(prefixBytes)+4], ea.ShortID)
	copy(data[len(prefixBytes)+4:len(prefixBytes)+36], ea.PublicKey[:])
	binary.LittleEndian.PutUint64(data[len(prefixBytes)+36:len(prefixBytes)+44], math.Float64bits(ea.Latitude))
	binary.LittleEndian.PutUint64(data[len(prefixBytes)+44:len(prefixBytes)+52], math.Float64bits(ea.Longitude))
	binary.LittleEndian.PutUint64(data[len(prefixBytes)+52:len(prefixBytes)+60], ea.Capacity)
	binary.LittleEndian.PutUint64(data[len(prefixBytes)+60:len(prefixBytes)+68], ea.Debt)
	binary.LittleEndian.PutUint32(data[len(prefixBytes)+68:len(prefixBytes)+72], ea.Expiration)
	return data
}

// Serialize serializes the EquipmentAuthorization into a byte slice.
//
// This function takes an EquipmentAuthorization struct and serializes it directly
// into a byte slice, in the same order as the fields appear in the struct.
// It returns the byte slice containing the serialized data.
func (ea *EquipmentAuthorization) Serialize() []byte {
	data := make([]byte, 4+32+8+8+8+8+4+64)
	binary.LittleEndian.PutUint32(data[0:4], ea.ShortID)
	copy(data[4:36], ea.PublicKey[:])
	binary.LittleEndian.PutUint64(data[36:44], math.Float64bits(ea.Latitude))
	binary.LittleEndian.PutUint64(data[44:52], math.Float64bits(ea.Longitude))
	binary.LittleEndian.PutUint64(data[52:60], ea.Capacity)
	binary.LittleEndian.PutUint64(data[60:68], ea.Debt)
	binary.LittleEndian.PutUint32(data[68:72], ea.Expiration)
	copy(data[72:], ea.Signature[:])
	return data
}

// DeserializeEquipmentAuthorization deserializes a byte slice into an
// EquipmentAuthorization.
//
// This function takes a byte slice and deserializes it directly into an EquipmentAuthorization struct.
// The byte slice should be serialized in the same order as the fields in the EquipmentAuthorization struct.
// It returns the deserialized EquipmentAuthorization and any error encountered.
func DeserializeEquipmentAuthorization(data []byte) (ea EquipmentAuthorization, err error) {
	if len(data) != 136 {
		return ea, fmt.Errorf("input is not the correct length to be an EquipmentAuthorization: %v vs %v", len(data), 136)
	}

	ea.ShortID = binary.LittleEndian.Uint32(data[0:4])
	copy(ea.PublicKey[:], data[4:36])
	ea.Latitude = math.Float64frombits(binary.LittleEndian.Uint64(data[36:44]))
	ea.Longitude = math.Float64frombits(binary.LittleEndian.Uint64(data[44:52]))
	ea.Capacity = binary.LittleEndian.Uint64(data[52:60])
	ea.Debt = binary.LittleEndian.Uint64(data[60:68])
	ea.Expiration = binary.LittleEndian.Uint32(data[68:72])
	copy(ea.Signature[:], data[72:])
	return ea, nil
}
