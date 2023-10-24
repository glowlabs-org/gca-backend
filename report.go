package main

// This file contains helper functions and structs related to the processing of
// equipment reports.

import (
	"encoding/binary"
	"errors"
)

// EquipmentReport defines the structure for a report received from a piece of equipment.
type EquipmentReport struct {
	ShortID     uint32    // A unique identifier for the equipment
	Timeslot    uint32    // A field denoting the time of the report
	PowerOutput uint64    // The power output from the equipment
	Signature   Signature // A digital signature for the report's authenticity
}

// SigningBytes returns the bytes that should be signed when sending an
// equipment report.
func (er EquipmentReport) SigningBytes() []byte {
	prefix := []byte("EquipmentReport")
	bytes := make([]byte, len(prefix)+16)
	copy(bytes, prefix)
	binary.BigEndian.PutUint32(bytes[15:], er.ShortID)
	binary.BigEndian.PutUint32(bytes[19:], er.Timeslot)
	binary.BigEndian.PutUint64(bytes[23:], er.PowerOutput)
	return bytes
}

// Serialize creates a compact binary representation of the data structure.
func (er EquipmentReport) Serialize() []byte {
	bytes := make([]byte, 80)
	binary.BigEndian.PutUint32(bytes[0:], er.ShortID)
	binary.BigEndian.PutUint32(bytes[4:], er.Timeslot)
	binary.BigEndian.PutUint64(bytes[8:], er.PowerOutput)
	copy(bytes[16:], er.Signature[:])
	return bytes
}

// DeserializeReport takes a byte slice and attempts to convert it back into an
// EquipmentReport structure.
//
// Returns an error if the byte slice is not the correct length.
func DeserializeReport(i []byte) (EquipmentReport, error) {
	// The serialized EquipmentReport should be exactly 80 bytes.
	if len(i) != 80 {
		return EquipmentReport{}, errors.New("input byte slice has incorrect length")
	}

	// Create an empty EquipmentReport struct to hold the deserialized values.
	var er EquipmentReport

	// Deserialize each field from the byte slice into the struct.
	er.ShortID = binary.BigEndian.Uint32(i[0:4])
	er.Timeslot = binary.BigEndian.Uint32(i[4:8])
	er.PowerOutput = binary.BigEndian.Uint64(i[8:16])
	copy(er.Signature[:], i[16:80])

	// Return the filled EquipmentReport struct.
	return er, nil
}
