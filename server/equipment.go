package server

// This file contains helper functions related to the creation and tracking of
// equipement.

// TODO: Need to test the persistence of the saveAllDeviceStats.

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

// saveAllDeviceStats will save the provided AllDeviceStats object to disk,
// adding it to the file that contains the history of all device stats.
func (gcas *GCAServer) saveAllDeviceStats(ads AllDeviceStats) error {
	file, err := os.OpenFile(filepath.Join(gcas.baseDir, AllDeviceStatsHistoryFile), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("unable to open all device stats file: %v", err)
	}
	defer file.Close()

	b := ads.Serialize()
	_, err = file.Write(b)
	if err != nil {
		return fmt.Errorf("unable to write data to disk: %v", err)
	}
	return nil
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
		ea, err := glow.DeserializeEquipmentAuthorization(buffer.Next(136))
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
			gcas.equipmentImpactRate[ea.ShortID] = new([4032]float64)
			gcas.equipmentReports[ea.ShortID] = new([4032]glow.EquipmentReport)
			continue
		}
		// If a conflict exists, ban the equipment.
		delete(gcas.equipment, ea.ShortID)
		delete(gcas.equipmentImpactRate, ea.ShortID)
		delete(gcas.equipmentReports, ea.ShortID)
		delete(gcas.equipmentShortID, ea.PublicKey)
		gcas.equipmentBans[ea.ShortID] = struct{}{}
	}

	return nil
}

// loadEquipmentHistory will open the history file that contains all of the
// DeviceStatsHistory entries, and load it into the server one element at a
// time. This loading process will also inform the equipmentReportsOffset value
// that gets set before all of the recentReports are loaded.
func (gcas *GCAServer) loadEquipmentHistory() error {
	// Read the full file into memory. We keep all of the structs in memory
	// already anyway, if it starts to get too big we can optimize later.
	path := filepath.Join(gcas.baseDir, AllDeviceStatsHistoryFile)
	data, err := ioutil.ReadFile(path)
	// Create the file if it doesn't exist.
	if os.IsNotExist(err) {
		f, err := os.Create(path)
		if err != nil {
			return fmt.Errorf("unable to create device stats history file: %v", err)
		}
		err = f.Close()
		if err != nil {
			return fmt.Errorf("unable to close device stats histroy file: %v", err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("unable to load the device stats history file: %v", err)
	}

	// Deserialize the device stats one week at at time.
	for {
		// Once we've parsed all the data, we can return.
		if len(data) == 0 {
			return nil
		}

		ads, i, err := DeserializeStreamAllDeviceStats(data)
		if err != nil {
			return fmt.Errorf("unable to decode all device stats: %v", err)
		}
		gcas.equipmentStatsHistory = append(gcas.equipmentStatsHistory, ads)
		data = data[i:]
	}
}

// saveEquipment serializes a given EquipmentAuthorization and appends it to
// the 'equipment-authorizations.dat' file. The function opens the file in append
// mode, or creates it if it does not exist.
//
// The method returns an error if it fails to open the file, write to it,
// or close it after writing.
//
// The bool indicates whether the equipment is new or not.
func (gcas *GCAServer) saveEquipment(ea glow.EquipmentAuthorization) (bool, error) {
	// Before saving, check if the equipment is already on the banlist.
	_, exists := gcas.equipmentBans[ea.ShortID]
	if exists {
		return false, fmt.Errorf("equipment with this ShortID is banned")
	}

	// Now check if there's already equipment with the same ShortID
	current, exists := gcas.equipment[ea.ShortID]
	if exists {
		if current == ea {
			// This exact authorization is already known and saved.
			// Exit without complaining.
			return false, nil
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
		return false, err
	}
	defer file.Close()
	// Serialize the EquipmentAuthorization to a byte slice
	serializedData := ea.Serialize()
	// Write the serialized data to the file
	_, err = file.Write(serializedData)
	if err != nil {
		return false, err
	}
	// Also add this to the recent equipment list so that the evidence will propagate
	// to other servers that are trying to sync.
	gcas.addRecentEquipmentAuth(ea)

	// If there is no conflict, add the new auth and exit.
	if !exists {
		gcas.equipmentShortID[ea.PublicKey] = ea.ShortID
		gcas.equipment[ea.ShortID] = ea
		gcas.equipmentImpactRate[ea.ShortID] = new([4032]float64)
		gcas.equipmentReports[ea.ShortID] = new([4032]glow.EquipmentReport)
		return true, nil
	}

	// There is a conflict, so we need to delete the equipment from the list of
	// equipment and also add a ban.
	delete(gcas.equipmentShortID, ea.PublicKey)
	delete(gcas.equipment, ea.ShortID)
	delete(gcas.equipmentImpactRate, ea.ShortID)
	delete(gcas.equipmentReports, ea.ShortID)
	gcas.equipmentBans[ea.ShortID] = struct{}{}
	return false, fmt.Errorf("duplicate authorization received, banning equipment")
}

// threadedMigrateReports will infrequently update the equipment reports so
// that the reports are always for the current week and the previous.
func (gcas *GCAServer) threadedMigrateReports(username, password string) {
	for {
		// This loop is pretty lightweight so every 3 seconds seems
		// fine, even though action is only taken once a week.
		time.Sleep(ReportMigrationFrequency)

		// We only update if we are progressed most of the way through
		// the second week.
		gcas.mu.Lock()
		ero := gcas.equipmentReportsOffset
		gcas.mu.Unlock()
		now := glow.CurrentTimeslot()
		if int64(now)-int64(ero) > 4000 {
			// TODO: Actually, the code has been adjusted so that
			// this should not be a problem. We should proceed,
			// business as usual, if this happens. But also, if
			// this happens we need to do it a bunch more times
			// so... we need to skip the reportMigrationFrequency
			// sleep on the next round. We probably need to
			// refactor this code a decent amount to make that
			// clean.
			panic("migration is late")
		}
		if int64(now)-int64(ero) > 3200 {
			// Fetch all of the moer values for the week.
			err := gcas.managedGetWattTimeWeekData(username, password)
			if err != nil {
				gcas.logger.Errorf("unable to fetch WattTime week data: %v", err)
				// All we can do is log the error, we still
				// need to rotate the time and save the data.
				// No control flow is used here.
			}
			// Save the device stats.
			gcas.mu.Lock()
			stats, err := gcas.buildDeviceStats(gcas.equipmentReportsOffset)
			if err != nil {
				panic("unable to build device stats: " + err.Error())
			}
			gcas.equipmentStatsHistory = append(gcas.equipmentStatsHistory, stats)
			err = gcas.saveAllDeviceStats(stats)
			if err != nil {
				panic("failed to save all device stats: " + err.Error())
			}
			// Copy the last half of every report into the first
			// half, then blank out the last half.
			for _, report := range gcas.equipmentReports {
				var blankReports [2016]glow.EquipmentReport
				copy(report[:2016], report[2016:])
				copy(report[2016:], blankReports[:])
			}
			// Repeat for the impact values.
			for _, rates := range gcas.equipmentImpactRate {
				var blankRates [2016]float64
				copy(rates[:2016], rates[2016:])
				copy(rates[2016:], blankRates[:])
			}
			// Update the reports offset.
			gcas.equipmentReportsOffset += 2016
			gcas.logger.Info("completed an equipment reports migration")
			gcas.mu.Unlock()
		}
	}
}
