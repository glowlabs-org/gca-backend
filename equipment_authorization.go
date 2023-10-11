package main

import (
	"bytes"
	"encoding/binary"
	"crypto/ed25519"
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
	binary.Write(&buf, binary.LittleEndian, ea.Capacity)
	binary.Write(&buf, binary.LittleEndian, ea.Debt)
	binary.Write(&buf, binary.LittleEndian, ea.Expiration)
	binary.Write(&buf, binary.LittleEndian, uint16(len(ea.PublicKey)))
	buf.Write(ea.PublicKey)
	binary.Write(&buf, binary.LittleEndian, uint16(len(ea.Signature)))
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
	if err := binary.Read(buf, binary.LittleEndian, &ea.Capacity); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.LittleEndian, &ea.Debt); err != nil {
		return nil, err
	}
	if err := binary.Read(buf, binary.LittleEndian, &ea.Expiration); err != nil {
		return nil, err
	}
	var pubKeyLen uint16
	if err := binary.Read(buf, binary.LittleEndian, &pubKeyLen); err != nil {
		return nil, err
	}
	ea.PublicKey = make([]byte, pubKeyLen)
	if err := binary.Read(buf, binary.LittleEndian, &ea.PublicKey); err != nil {
		return nil, err
	}
	var sigLen uint16
	if err := binary.Read(buf, binary.LittleEndian, &sigLen); err != nil {
		return nil, err
	}
	ea.Signature = make([]byte, sigLen)
	if err := binary.Read(buf, binary.LittleEndian, &ea.Signature); err != nil {
		return nil, err
	}
	return ea, nil
}
