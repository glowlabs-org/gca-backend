package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"errors"
)

// EquipmentAuthorization is a struct that mirrors EquipmentAuthorizationRequest,
// except PublicKey and Signature are ed25519 objects.
type EquipmentAuthorization struct {
	ShortID    uint32
	PublicKey  ed25519.PublicKey
	Capacity   uint64
	Debt       uint64
	Expiration uint32
	Signature  []byte
}

// Serialize serializes the EquipmentAuthorization into a byte slice.
func (ea *EquipmentAuthorization) Serialize() []byte {
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, ea.ShortID)
	buf.Write(ea.PublicKey)
	binary.Write(&buf, binary.LittleEndian, ea.Capacity)
	binary.Write(&buf, binary.LittleEndian, ea.Debt)
	binary.Write(&buf, binary.LittleEndian, ea.Expiration)
	buf.Write(ea.Signature)
	return buf.Bytes()
}

// Deserialize deserializes a byte slice into an EquipmentAuthorization.
func Deserialize(data []byte) (*EquipmentAuthorization, error) {
	buf := bytes.NewReader(data)
	ea := &EquipmentAuthorization{}
	if err := binary.Read(buf, binary.LittleEndian, &ea.ShortID); err != nil {
		return nil, err
	}
	ea.PublicKey = make([]byte, 32)
	if err := binary.Read(buf, binary.LittleEndian, &ea.PublicKey); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.LittleEndian, &ea.Capacity); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.LittleEndian, &ea.Debt); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.LittleEndian, &ea.Expiration); err != nil {
		return nil, err
	}
	ea.Signature = make([]byte, 64)
	if err := binary.Read(buf, binary.LittleEndian, &ea.Signature); err != nil {
		return nil, err
	}
	return ea, nil
}

// verifyEquipmentAuthorization checks the validity of the signature on an EquipmentAuthorization.
//
// It uses the public key of the Grid Control Authority (gcaPubkey) to verify the signature.
// The method returns an error if the verification fails.
func (gca *GCAServer) verifyEquipmentAuthorization(ea *EquipmentAuthorization) error {
	// Serialize the EquipmentAuthorization to get the original message.
	// We exclude the signature part for verification.
	originalMessage := ea.Serialize()

	// The length of the original message minus the length of the signature
	// gives us the slice of bytes to verify against the signature.
	originalMessageWithoutSignature := originalMessage[:len(originalMessage)-len(ea.Signature)]

	// Verify the signature using the Grid Control Authority's public key
	isValid := ed25519.Verify(gca.gcaPubkey, originalMessageWithoutSignature, ea.Signature)

	if !isValid {
		return errors.New("invalid signature on EquipmentAuthorization")
	}

	return nil
}
