package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
)

// addRecentEquipmentAuth will add a recent auth to the list of recent auths in the
// GCAServer. If the number of recent auths exceeds maxRecentAuths, the least recent
// half of the auths will be tossed.
func (gcas *GCAServer) addRecentEquipmentAuth(ea EquipmentAuthorization) {
	gcas.recentEquipmentAuths = append(gcas.recentEquipmentAuths, ea)

	// Drop the first half of the recent equipments if the list is too long.
	if len(gcas.recentEquipmentAuths) > maxRecentEquipmentAuths {
		halfIndex := len(gcas.recentEquipmentAuths) / 2
		copy(gcas.recentEquipmentAuths[:], gcas.recentEquipmentAuths[halfIndex:])
		gcas.recentEquipmentAuths = gcas.recentEquipmentAuths[:halfIndex]
	}
}

// loadEquipmentAuths is responsible for populating the equipment map
// using the provided array of EquipmentAuths.
func (gcas *GCAServer) loadEquipmentAuths(equipment []EquipmentAuthorization) {
	// Iterate through each piece of equipment in the provided list
	for _, e := range equipment {
		// Add the equipment's public key to the equipment map using its ShortID as the key
		gcas.equipment[e.ShortID] = e
		gcas.addRecentEquipmentAuth(e)
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
	gca.loadEquipmentAuths(equipment)

	return nil
}

// saveEquipment serializes a given EquipmentAuthorization and appends it to
// the 'equipment-authorizations.dat' file. The function opens the file in append
// mode, or creates it if it does not exist.
//
// The method returns an error if it fails to open the file, write to it,
// or close it after writing.
func (gcas *GCAServer) saveEquipment(ea EquipmentAuthorization) error {
	// Determine the full path of the 'equipment-authorizations.dat' file within 'baseDir'
	filePath := filepath.Join(gcas.baseDir, "equipment-authorizations.dat")

	// Open the file in append mode.
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Serialize the EquipmentAuthorization to a byte slice
	serializedData := ea.Serialize()

	// Write the serialized data to the file
	_, err = file.Write(serializedData)
	if err != nil {
		return err
	}

	gcas.addRecentEquipmentAuth(ea)

	return nil
}
