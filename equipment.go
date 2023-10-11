package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
)

// loadEquipmentKeys is responsible for populating the equipmentKeys map
// using the provided array of Equipment.
func (gca *GCAServer) loadEquipmentKeys(equipment []EquipmentAuthorization) {
	// Iterate through each piece of equipment in the provided list
	for _, e := range equipment {
		// Add the equipment's public key to the equipmentKeys map using its ShortID as the key
		gca.equipmentKeys[e.ShortID] = e.PublicKey
	}
}

// loadEquipment reads the serialized EquipmentAuthorizations from disk,
// deserializes them, and then verifies each of them.
//
// It reads from a file called 'equipment-authorizations.dat' within the 'baseDir' folder.
// The method creates a new file if the file does not exist.
func (gca *GCAServer) loadEquipment() error {
	// Determine the full path of the 'equipment-authorizations.dat' file within 'baseDir'
	filePath := filepath.Join(gca.baseDir, "equipment-authorizations.dat")

	// Attempt to read the file. If it doesn't exist, create it.
	rawData, err := ioutil.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create the file if it does not exist
			if _, err := os.Create(filePath); err != nil {
				return err
			}
			return nil
		}
		return err
	}

	// Proceed to deserialize and verify the EquipmentAuthorizations if any data was read
	var equipment []EquipmentAuthorization
	buffer := bytes.NewBuffer(rawData)
	for buffer.Len() > 0 {
		// Deserialize the EquipmentAuthorization
		ea, err := Deserialize(buffer.Next(120)) // 120 bytes = 4 (ShortID) + 32 (PublicKey) + 8 (Capacity) + 8 (Debt) + 4 (Expiration) + 64 (Signature)
		if err != nil {
			return err
		}

		// Verify the EquipmentAuthorization
		if err := gca.verifyEquipmentAuthorization(ea); err != nil {
			return err
		}
		
		equipment = append(equipment, ea)
	}
	gca.loadEquipmentKeys(equipment)

	return nil
}
