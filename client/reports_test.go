package client

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	"github.com/glowlabs-org/gca-backend/glow"
)

// TODO: Need to make sure sending multiple reports at once is working.

// TODO: Need to make sure we are correctly handling cases where multiple
// monitoring reports are in the same bin. The strategy there is that the bin
// will always get the first report that appears in that bin. If 2 reports
// appear (or more), the extra report(s) will go into the next bin.

// updateMonitorFile is an apparatus that allows the monitor file to be changed
// during testing, simulating a new reading being taken.
func updateMonitorFile(dir string, newTimeslots []uint32, newReadings []uint64) error {
	if len(newTimeslots) != len(newReadings) {
		return fmt.Errorf("incorrect usage, timeslots and readings must have same len")
	}

	// Craft the new file.
	newFileDataStr := "timestamp,energy (mWh)"
	for i := 0; i < len(newTimeslots); i++ {
		newFileDataStr += "\n"
		newFileDataStr += fmt.Sprintf("%v", int64(newTimeslots[i]*300)+glow.GenesisTime)
		newFileDataStr += fmt.Sprintf("%v", newReadings[i])
	}

	// Write the new file.
	path := filepath.Join(dir, EnergyFile)
	err := ioutil.WriteFile(path, []byte(newFileDataStr), 0644)
	if err != nil {
		return fmt.Errorf("unable to write the new monitor file: %v", err)
	}

	return nil
}

// TestPeriodicMonitoring is a simple test to make sure that the periodic
// monitoring is working.
func TestPeriodicMonitoring(t *testing.T) {
	client, _, _, err := FullClientTestEnvironment(t.Name())
	if err != nil {
		t.Fatal(err)
	}

	// Update the monitoring file so that there is data to read.
	err = updateMonitorFile(client.baseDir, []uint32{1, 5}, []uint64{500, 3000})
	if err != nil {
		t.Fatal(err)
	}

	// Sleep for long enough that the client will send messages to the
	// server based on the readings if everything is working right.
	time.Sleep(2 * sendReportTime)

	// Check whether the server got the reports.
	//
	// TODO: This requires writing code to see what reports the server has.
}
