package main

import (
	"testing"
	"time"
)

// TestThreadedMigrateReports tests the migration of equipment reports.
func TestThreadedMigrateReports(t *testing.T) {
	// Create a test directory for the GCAServer
	dir := generateTestDir(t.Name())
	_, err := generateGCATestKeys(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Initialize the GCAServer
	server := NewGCAServer(dir)
	defer server.Close()

	// Because we mess with the global time during this test, we need to
	// make sure it gets reset to 0 when the test ends.
	defer func() {
		manualCurrentTimeslot = 0
	}()

	// Generate a dummy EquipmentAuthorization
	ePubKey, _ := GenerateKeyPair()
	dummyEquipment := EquipmentAuthorization{ShortID: 1, PublicKey: ePubKey}
	server.loadEquipmentAuth(dummyEquipment)

	// Generate reports that will fill out the first 2 migrations.
	dummyReport := [4032]EquipmentReport{}
	for i := 0; i < len(dummyReport); i++ {
		dummyReport[i] = EquipmentReport{
			ShortID:     dummyEquipment.ShortID,
			Timeslot:    uint32(i),
			PowerOutput: uint64(1000 + i),
		}
	}
	// Just load the dummy reports right into the server.
	server.equipmentReports[dummyEquipment.ShortID] = &dummyReport

	// Verify that nothing got pruned.
	for i := 0; i < 4032; i++ {
		if server.equipmentReports[dummyEquipment.ShortID][i].PowerOutput < 2 {
			t.Fatal("equipment should still exist")
		}
	}
	// Wait 150 milliseconds, which should trigger a prune. Except that no
	// prune should be triggered because we aren't inside the prune window.
	time.Sleep(150 * time.Millisecond)
	// Verify that nothing got pruned.
	for i := 0; i < 4032; i++ {
		if server.equipmentReports[dummyEquipment.ShortID][i].PowerOutput < 2 {
			t.Fatal("equipment should still exist")
		}
	}

	// Update the timeslot just enough that we shouldn't be getting pruned still.
	manualCurrentTimeslot = 3000
	// Wait 150 milliseconds, which should trigger a prune. Except that no
	// prune should be triggered because we aren't inside the prune window.
	time.Sleep(150 * time.Millisecond)
	// Verify that nothing got pruned.
	for i := 0; i < 4032; i++ {
		if server.equipmentReports[dummyEquipment.ShortID][i].PowerOutput < 2 {
			t.Fatal("equipment should still exist")
		}
	}

	// Update the timeslot just enough that things should be getting pruned now.
	manualCurrentTimeslot = 3300
	// Wait 150 milliseconds, which should trigger a prune. Except that no
	// prune should be triggered because we aren't inside the prune window.
	time.Sleep(150 * time.Millisecond)
	// Verify that things got pruned
	for i := 0; i < 2016; i++ {
		if server.equipmentReports[dummyEquipment.ShortID][i].PowerOutput < 2 {
			t.Fatal("equipment should still exist")
		}
	}
	for i := 2016; i < 4032; i++ {
		if server.equipmentReports[dummyEquipment.ShortID][i].PowerOutput != 0 {
			t.Fatal("equipment should still exist")
		}
	}

	// Wait for another prune cycle, verify nothing happens.
	time.Sleep(150 * time.Millisecond)
	for i := 0; i < 2016; i++ {
		if server.equipmentReports[dummyEquipment.ShortID][i].PowerOutput < 2 {
			t.Fatal("equipment should still exist")
		}
	}
	for i := 2016; i < 4032; i++ {
		if server.equipmentReports[dummyEquipment.ShortID][i].PowerOutput != 0 {
			t.Fatal("equipment should still exist")
		}
	}

	// Update the time to 5000, which should still not cause a prune.
	time.Sleep(150 * time.Millisecond)
	for i := 0; i < 2016; i++ {
		if server.equipmentReports[dummyEquipment.ShortID][i].PowerOutput < 2 {
			t.Fatal("equipment should still exist")
		}
	}
	for i := 2016; i < 4032; i++ {
		if server.equipmentReports[dummyEquipment.ShortID][i].PowerOutput != 0 {
			t.Fatal("equipment should still exist")
		}
	}

	// Update the current timeslot to trigger another migration, now all of
	// the reports should be migrated out.
	manualCurrentTimeslot = 5300
	time.Sleep(150 * time.Millisecond)
	for i := 0; i < 4032; i++ {
		if server.equipmentReports[dummyEquipment.ShortID][i].PowerOutput != 0 {
			t.Fatal("equipment should still exist")
		}
	}
}
