package server

// This file contains helper functions related to the creation and tracking of
// equipement.

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/glowlabs-org/gca-backend/glow"
)

// addRecentEquipmentAuth will add a recent auth to the list of recent auths in the
// GCAServer. If the number of recent auths exceeds maxRecentAuths, the least recent
// half of the auths will be tossed.
func (gcas *GCAServer) addRecentEquipmentAuth(ea glow.EquipmentAuthorization) {
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
			f, err := os.Create(filePath)
			if err != nil {
				return err
			}
			f.Close()
			return nil
		}
		return err
	}

	// Proceed to deserialize and verify the EquipmentAuthorizations if any data was read
	var equipment []glow.EquipmentAuthorization
	buffer := bytes.NewBuffer(rawData)
	for buffer.Len() > 0 {
		// Deserialize the EquipmentAuthorization
		ea, err := glow.DeserializeEquipmentAuthorization(buffer.Next(120)) // 120 bytes = 4 (ShortID) + 32 (PublicKey) + 8 (Capacity) + 8 (Debt) + 4 (Expiration) + 64 (Signature)
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
			gcas.equipmentShortID[ea.PublicKey] = ea.ShortID
			gcas.equipment[ea.ShortID] = ea
			gcas.equipmentReports[ea.ShortID] = new([4032]glow.EquipmentReport)
			continue
		}
		// If a conflict exists, ban the equipment.
		delete(gcas.equipment, ea.ShortID)
		delete(gcas.equipmentReports, ea.ShortID)
		delete(gcas.equipmentShortID, ea.PublicKey)
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
func (gcas *GCAServer) saveEquipment(ea glow.EquipmentAuthorization) error {
	// Before saving, check if the equipment is already on the banlist.
	_, exists := gcas.equipmentBans[ea.ShortID]
	if exists {
		return fmt.Errorf("equipment with this ShortID is banned")
	}

	// Now check if there's already equipment with the same ShortID
	current, exists := gcas.equipment[ea.ShortID]
	if exists {
		if current == ea {
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
		gcas.equipmentShortID[ea.PublicKey] = ea.ShortID
		gcas.equipment[ea.ShortID] = ea
		gcas.equipmentReports[ea.ShortID] = new([4032]glow.EquipmentReport)
		return nil
	}

	// There is a conflict, so we need to delete the equipment from the list of
	// equipment and also add a ban.
	delete(gcas.equipmentShortID, ea.PublicKey)
	delete(gcas.equipment, ea.ShortID)
	delete(gcas.equipmentReports, ea.ShortID)
	gcas.equipmentBans[ea.ShortID] = struct{}{}
	return fmt.Errorf("duplicate authorization received, banning equipment")
}

// threadedMigrateReports will infrequently update the equipment reports so
// that the reports are always for the current week and the previous.
func (gcas *GCAServer) threadedMigrateReports() {
	for {
		// This loop is pretty lightweight so every 3 seconds seems
		// fine, even though action is only taken once a week.
		time.Sleep(reportMigrationFrequency)

		// We only update if we are progressed most of the way through
		// the second week.
		gcas.mu.Lock()
		now := glow.CurrentTimeslot()
		if int64(now)-int64(gcas.equipmentReportsOffset) > 4000 {
			// panic, because the system has entered incoherency.
			gcas.mu.Unlock()
			panic("migration got out of sync")
		}
		if int64(now)-int64(gcas.equipmentReportsOffset) > 3200 {
			// Copy the last half of every report into the first
			// half, then blank out the last half.
			for _, report := range gcas.equipmentReports {
				var blankReports [2016]glow.EquipmentReport
				copy(report[:2016], report[2016:])
				copy(report[2016:], blankReports[:])
			}
			// Update the reports offset.
			gcas.equipmentReportsOffset += 2016
			gcas.logger.Info("completed an equipment reports migration")
		}
		gcas.mu.Unlock()
	}
}
