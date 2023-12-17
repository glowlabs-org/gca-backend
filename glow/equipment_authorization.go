package glow

// This file contains helper functions related to managing equipment
// authorizations.

import (
	"encoding/binary"
	"fmt"
	"math"
)

// EquipmentAuthorization struct defines an authorization for a piece of
// equipment. It contains all of the genesis information for the equipment that
// a GCA will need to know when submitting reports for the corresponding solar
// farm.
type EquipmentAuthorization struct {
	// The equipment will refer to itself using its own ShortID to save
	// bandwidth when submitting reports to GCA servers.
	ShortID   uint32
	PublicKey PublicKey

	// The geographic location of the solar farm. Three decimals of
	// precision should be used.
	Latitude  float64
	Longitude float64

	// Capacity is the maximum power output of the solar farm, expressed in
	// milliwatthours per timeslot. A timeslot is 5 minutes. Debt is the
	// number of grams of CO2 debt that must be paid off every week by a
	// solar farm. Expiration is the timeslot where power reports are no
	// longer valid for this solar farm.
	Capacity   uint64
	Debt       uint64
	Expiration uint32

	// Initialization defines the moment when the ProtocolFee was paid and
	// the solar farm was allowed to begin submitting power reports to the
	// protocol.
	Initialization uint32
	ProtocolFee    uint64

	// The Signature is a signature from the GCA which confirms that a
	// device with the above properties is allowed to submit power reports
	// to the Glow protocol.
	Signature Signature
}

// Serialize serializes the EquipmentAuthorization into a byte slice.
func (ea *EquipmentAuthorization) Serialize() []byte {
	data := make([]byte, 4+32+8+8+8+8+4+4+8+64)
	binary.LittleEndian.PutUint32(data[0:4], ea.ShortID)
	copy(data[4:36], ea.PublicKey[:])
	binary.LittleEndian.PutUint64(data[36:44], math.Float64bits(ea.Latitude))
	binary.LittleEndian.PutUint64(data[44:52], math.Float64bits(ea.Longitude))
	binary.LittleEndian.PutUint64(data[52:60], ea.Capacity)
	binary.LittleEndian.PutUint64(data[60:68], ea.Debt)
	binary.LittleEndian.PutUint32(data[68:72], ea.Expiration)
	binary.LittleEndian.PutUint32(data[72:76], ea.Initialization)
	binary.LittleEndian.PutUint64(data[76:84], ea.ProtocolFee)
	copy(data[84:], ea.Signature[:])
	return data
}

// SigningBytes generates the byte slice that needs to be signed to validate
// the object. It's almost the same as the serialization, except that a signing
// prefix has been added, and the signature has been stripped off.
func (ea *EquipmentAuthorization) SigningBytes() []byte {
	prefix := "EquipmentAuthorization"
	prefixBytes := []byte(prefix)
	serialization := ea.Serialize()
	noSig := serialization[:len(serialization)-64]
	data := make([]byte, len(prefixBytes)+len(noSig))
	copy(data[0:len(prefixBytes)], prefixBytes)
	copy(data[len(prefixBytes):], noSig)
	return data
}

// DeserializeEquipmentAuthorization deserializes a byte slice into an
// EquipmentAuthorization.
//
// This function takes a byte slice and deserializes it directly into an EquipmentAuthorization struct.
// The byte slice should be serialized in the same order as the fields in the EquipmentAuthorization struct.
// It returns the deserialized EquipmentAuthorization and any error encountered.
func DeserializeEquipmentAuthorization(data []byte) (ea EquipmentAuthorization, err error) {
	if len(data) != 148 {
		return ea, fmt.Errorf("input is not the correct length to be an EquipmentAuthorization: %v vs %v", len(data), 148)
	}

	ea.ShortID = binary.LittleEndian.Uint32(data[0:4])
	copy(ea.PublicKey[:], data[4:36])
	ea.Latitude = math.Float64frombits(binary.LittleEndian.Uint64(data[36:44]))
	ea.Longitude = math.Float64frombits(binary.LittleEndian.Uint64(data[44:52]))
	ea.Capacity = binary.LittleEndian.Uint64(data[52:60])
	ea.Debt = binary.LittleEndian.Uint64(data[60:68])
	ea.Expiration = binary.LittleEndian.Uint32(data[68:72])
	ea.Initialization = binary.LittleEndian.Uint32(data[72:76])
	ea.ProtocolFee = binary.LittleEndian.Uint64(data[76:84])
	copy(ea.Signature[:], data[84:])
	return ea, nil
}
