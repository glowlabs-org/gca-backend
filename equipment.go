package main

import (
	"bytes"
	"fmt"
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

// loadEquipment reads the serialized EquipmentAuthorizations from disk,
// deserializes them, and then verifies each of them.
//
// It reads from a file called 'equipment-authorizations.dat' within the 'baseDir' folder.
// The method creates a new file if the file does not exist.
func (gcas *GCAServer) loadEquipment() error {
	// Determine the full path of the 'equipment-authorizations.dat' file within 'baseDir'
	filePath := filepath.Join(gcas.baseDir, "equipment-authorizations.dat")

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
		ea, err := Deserialize(buffer.Next(122)) // 122 bytes = 4 (ShortID) + 33 (PublicKey) + 8 (Capacity) + 8 (Debt) + 4 (Expiration) + 65 (Signature)
		if err != nil {
			return err
		}

		// Verify the EquipmentAuthorization
		if err := gcas.verifyEquipmentAuthorization(ea); err != nil {
			return err
		}

		equipment = append(equipment, ea)
	}
	for _, ea := range equipment {
		// Load the equipment, paying close attention to the ban rules.
		_, exists := gcas.equipmentBans[ea.ShortID]
		if exists {
			continue
		}
		// Now check if there's already equipment with the same ShortID
		current, exists := gcas.equipment[ea.ShortID]
		if exists {
			// If this is the same equipment, there's no problem.
			a := current.Serialize()
			b := ea.Serialize()
			if bytes.Equal(a, b) {
				continue
			}
		}
		// Add the auth to the recents either way.
		gcas.addRecentEquipmentAuth(ea)
		// If no conflict exists, add the equipment
		if !exists {
			gcas.equipment[ea.ShortID] = ea
			continue
		}
		// If a conflict exists, ban the equipment.
		delete(gcas.equipment, ea.ShortID)
		gcas.equipmentBans[ea.ShortID] = struct{}{}
	}

	return nil
}

// saveEquipment serializes a given EquipmentAuthorization and appends it to
// the 'equipment-authorizations.dat' file. The function opens the file in append
// mode, or creates it if it does not exist.
//
// The method returns an error if it fails to open the file, write to it,
// or close it after writing.
func (gcas *GCAServer) saveEquipment(ea EquipmentAuthorization) error {
	// Before saving, check if the equipment is already on the banlist.
	_, exists := gcas.equipmentBans[ea.ShortID]
	if exists {
		return fmt.Errorf("equipment with this ShortID is banned")
	}

	// Now check if there's already equipment with the same ShortID
	current, exists := gcas.equipment[ea.ShortID]
	if exists {
		// If this is the same equipment, there's no problem.
		a := current.Serialize()
		b := ea.Serialize()
		if bytes.Equal(a, b) {
			// This exact authorization is already known and saved.
			// Exit without complaining.
			return nil
		}
	}

	// Save the authorization, whether or not there's a conflict. If there is a
	// conflict, we need to save the auth so we can prove to other parties that
	// this ShortID needs to be banned.
	//
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
	// Also add this to the recent equipment list so that the evidence will propagate
	// to other servers that are trying to sync.
	gcas.addRecentEquipmentAuth(ea)

	// If there is no conflict, add the new auth and exit.
	if !exists {
		gcas.equipment[ea.ShortID] = ea
		return nil
	}

	// There is a conflict, so we need to delete the equipment from the list of
	// equipment and also add a ban.
	delete(gcas.equipment, ea.ShortID)
	gcas.equipmentBans[ea.ShortID] = struct{}{}
	return fmt.Errorf("duplicate authorization received, banning equipment")
}
