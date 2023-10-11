package main

import (
	"crypto/ed25519"
)

// Equipment represents a known piece of equipment with its ShortID and corresponding public key.
type Equipment struct {
	ShortID uint32
	Key     ed25519.PublicKey
}

// loadEquipmentKeys is responsible for populating the equipmentKeys map
// using the provided array of Equipment.
func (gca *GCAServer) loadEquipmentKeys(equipments []Equipment) {
	// Iterate through each piece of equipment in the provided list
	for _, equipment := range equipments {
		// Add the equipment's public key to the equipmentKeys map using its ShortID as the key
		gca.equipmentKeys[equipment.ShortID] = equipment.Key
	}
}
