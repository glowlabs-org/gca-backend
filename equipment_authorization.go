package main

import (
	"encoding/binary"
	"errors"

	"github.com/ethereum/go-ethereum/crypto"
)

// EquipmentAuthorization struct reflects an authorization request,
// except PublicKey and Signature are byte slices for the secp256k1 algorithm.
type EquipmentAuthorization struct {
	ShortID    uint32
	PublicKey  [33]byte
	Capacity   uint64
	Debt       uint64
	Expiration uint32
	Signature  [65]byte
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
	data := make([]byte, len(prefixBytes)+4+33+8+8+4)

	// Add the prefix to the byte slice
	copy(data[0:len(prefixBytes)], prefixBytes)

	// Serialize ShortID
	binary.LittleEndian.PutUint32(data[len(prefixBytes):len(prefixBytes)+4], ea.ShortID)

	// Serialize PublicKey (updated to 33 bytes)
	copy(data[len(prefixBytes)+4:len(prefixBytes)+37], ea.PublicKey[:])

	// Serialize Capacity
	binary.LittleEndian.PutUint64(data[len(prefixBytes)+37:len(prefixBytes)+45], ea.Capacity)

	// Serialize Debt
	binary.LittleEndian.PutUint64(data[len(prefixBytes)+45:len(prefixBytes)+53], ea.Debt)

	// Serialize Expiration
	binary.LittleEndian.PutUint32(data[len(prefixBytes)+53:len(prefixBytes)+57], ea.Expiration)

	// Return the byte slice for signing
	hash := crypto.Keccak256Hash(data).Bytes()
	return hash
}

// Serialize serializes the EquipmentAuthorization into a byte slice.
//
// This function takes an EquipmentAuthorization struct and serializes it directly
// into a byte slice, in the same order as the fields appear in the struct.
// It returns the byte slice containing the serialized data.
func (ea *EquipmentAuthorization) Serialize() []byte {
	// Initialize a byte slice with a length sufficient to hold all serialized fields
	data := make([]byte, 4+33+8+8+4+65) // Updated to account for 33-byte PublicKey

	// Serialize ShortID
	binary.LittleEndian.PutUint32(data[0:4], ea.ShortID)

	// Serialize PublicKey (updated to 33 bytes)
	copy(data[4:37], ea.PublicKey[:]) // Updated indices

	// Serialize Capacity
	binary.LittleEndian.PutUint64(data[37:45], ea.Capacity) // Updated indices

	// Serialize Debt
	binary.LittleEndian.PutUint64(data[45:53], ea.Debt) // Updated indices

	// Serialize Expiration
	binary.LittleEndian.PutUint32(data[53:57], ea.Expiration) // Updated indices

	// Serialize Signature
	copy(data[57:], ea.Signature[:]) // Updated indices

	// Return the serialized byte slice
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
	if len(data) != 122 { // Updated total size
		return ea, errors.New("input is not the correct length to be an EquipmentAuthorization")
	}

	// Deserialize ShortID
	ea.ShortID = binary.LittleEndian.Uint32(data[0:4])

	// Deserialize PublicKey (updated to 33 bytes)
	copy(ea.PublicKey[:], data[4:37]) // Updated indices

	// Deserialize Capacity
	ea.Capacity = binary.LittleEndian.Uint64(data[37:45]) // Updated indices

	// Deserialize Debt
	ea.Debt = binary.LittleEndian.Uint64(data[45:53]) // Updated indices

	// Deserialize Expiration
	ea.Expiration = binary.LittleEndian.Uint32(data[53:57]) // Updated indices

	// Deserialize Signature
	copy(ea.Signature[:], data[57:]) // Updated indices

	// Return the deserialized object and nil error
	return ea, nil
}

// verifyEquipmentAuthorization checks the validity of the signature on an EquipmentAuthorization.
//
// It uses the public key of the Grid Control Authority (gcaPubkey) to verify the signature.
// The method returns an error if the verification fails.
func (gcas *GCAServer) verifyEquipmentAuthorization(ea EquipmentAuthorization) error {
	// Generate the signing bytes for the EquipmentAuthorization
	signingBytes := ea.SigningBytes()

	// Verify the signature using Ethereum's crypto.VerifySignature
	isValid := crypto.VerifySignature(crypto.FromECDSAPub(gcas.gcaPubkey), signingBytes, ea.Signature[:64])
	if !isValid {
		return errors.New("invalid signature on EquipmentAuthorization")
	}
	return nil
}
