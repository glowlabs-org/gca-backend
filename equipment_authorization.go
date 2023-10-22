package main

import (
	"encoding/binary"
	"errors"
)

// EquipmentAuthorization struct reflects an authorization request,
// except PublicKey and Signature are byte slices for the secp256k1 algorithm.
type EquipmentAuthorization struct {
	ShortID    uint32
	PublicKey  [32]byte
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
	// Initialize the prefix string and convert it to a byte slice
	prefix := "EquipmentAuthorization"
	prefixBytes := []byte(prefix)

	// Initialize a byte slice with a length sufficient to hold all serialized fields except Signature
	// Added length of prefixBytes for the "EquipmentAuthorization" prefix
	data := make([]byte, len(prefixBytes)+4+32+8+8+4)

	// Serialize all the fields.
	copy(data[0:len(prefixBytes)], prefixBytes)
	binary.LittleEndian.PutUint32(data[len(prefixBytes):len(prefixBytes)+4], ea.ShortID)
	copy(data[len(prefixBytes)+4:len(prefixBytes)+36], ea.PublicKey[:])
	binary.LittleEndian.PutUint64(data[len(prefixBytes)+36:len(prefixBytes)+44], ea.Capacity)
	binary.LittleEndian.PutUint64(data[len(prefixBytes)+44:len(prefixBytes)+52], ea.Debt)
	binary.LittleEndian.PutUint32(data[len(prefixBytes)+52:len(prefixBytes)+56], ea.Expiration)
	return data
}

// Serialize serializes the EquipmentAuthorization into a byte slice.
//
// This function takes an EquipmentAuthorization struct and serializes it directly
// into a byte slice, in the same order as the fields appear in the struct.
// It returns the byte slice containing the serialized data.
func (ea *EquipmentAuthorization) Serialize() []byte {
	// Initialize a byte slice with a length sufficient to hold all serialized fields
	data := make([]byte, 4+32+8+8+4+64) // Updated to account for 33-byte PublicKey

	// Serialize all the fields.
	binary.LittleEndian.PutUint32(data[0:4], ea.ShortID)
	copy(data[4:36], ea.PublicKey[:])                         // Updated indices
	binary.LittleEndian.PutUint64(data[36:44], ea.Capacity)   // Updated indices
	binary.LittleEndian.PutUint64(data[44:52], ea.Debt)       // Updated indices
	binary.LittleEndian.PutUint32(data[52:56], ea.Expiration) // Updated indices
	copy(data[56:], ea.Signature[:])                          // Updated indices
	return data
}

// Deserialize deserializes a byte slice into an EquipmentAuthorization.
//
// This function takes a byte slice and deserializes it directly into an EquipmentAuthorization struct.
// The byte slice should be serialized in the same order as the fields in the EquipmentAuthorization struct.
// It returns the deserialized EquipmentAuthorization and any error encountered.
func Deserialize(data []byte) (EquipmentAuthorization, error) {
	// Initialize an EquipmentAuthorization object to hold the deserialized data
	var ea EquipmentAuthorization

	// Check for minimum required length (updated to account for 33-byte PublicKey)
	if len(data) != 120 { // Updated total size
		return ea, errors.New("input is not the correct length to be an EquipmentAuthorization")
	}

	// Deserialize all the fields
	ea.ShortID = binary.LittleEndian.Uint32(data[0:4])
	copy(ea.PublicKey[:], data[4:36])                       // Updated indices
	ea.Capacity = binary.LittleEndian.Uint64(data[36:44])   // Updated indices
	ea.Debt = binary.LittleEndian.Uint64(data[44:52])       // Updated indices
	ea.Expiration = binary.LittleEndian.Uint32(data[52:56]) // Updated indices
	copy(ea.Signature[:], data[56:])                        // Updated indices
	return ea, nil
}

// verifyEquipmentAuthorization checks the validity of the signature on an EquipmentAuthorization.
//
// It uses the public key of the Grid Control Authority (gcaPubkey) to verify the signature.
// The method returns an error if the verification fails.
func (gcas *GCAServer) verifyEquipmentAuthorization(ea EquipmentAuthorization) error {
	signingBytes := ea.SigningBytes()
	isValid := Verify(gcas.gcaPubkey, signingBytes, ea.Signature)
	if !isValid {
		return errors.New("invalid signature on EquipmentAuthorization")
	}
	return nil
}
