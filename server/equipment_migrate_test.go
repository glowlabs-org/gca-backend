package server

import (
	"testing"
	"time"

	"github.com/glowlabs-org/gca-backend/glow"
)

// TestThreadedMigrateReports tests the migration of equipment reports.
func TestThreadedMigrateReports(t *testing.T) {
	gcas, _, _, gcaPrivKey, err := SetupTestEnvironment(t.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer gcas.Close()
	defer func() {
		glow.SetCurrentTimeslot(0)
	}()

	// Generate a dummy EquipmentAuthorization
	ePubKey, _ := glow.GenerateKeyPair()
	dummyEquipment := glow.EquipmentAuthorization{ShortID: 1, PublicKey: ePubKey}
	err = gcas.AuthorizeEquipment(dummyEquipment, gcaPrivKey)
	if err != nil {
		t.Fatal(err)
	}

	// Generate reports that will fill out the first 2 migrations.
	dummyReport := [4032]glow.EquipmentReport{}
	for i := 0; i < len(dummyReport); i++ {
		dummyReport[i] = glow.EquipmentReport{
			ShortID:     dummyEquipment.ShortID,
			Timeslot:    uint32(i),
			PowerOutput: uint64(1000 + i),
		}
	}
	// Just load the dummy reports right into the gcas.
	gcas.equipmentReports[dummyEquipment.ShortID] = &dummyReport

	// Verify that nothing got pruned.
	for i := 0; i < 4032; i++ {
		gcas.mu.Lock()
		if gcas.equipmentReports[dummyEquipment.ShortID][i].PowerOutput < 2 {
			gcas.mu.Unlock()
			t.Fatal("equipment should still exist")
		}
		gcas.mu.Unlock()
	}
	// Wait 350 milliseconds, which should trigger a prune. Except that no
	// prune should be triggered because we aren't inside the prune window.
	time.Sleep(350 * time.Millisecond)
	// Verify that nothing got pruned.
	for i := 0; i < 4032; i++ {
		gcas.mu.Lock()
		if gcas.equipmentReports[dummyEquipment.ShortID][i].PowerOutput < 2 {
			gcas.mu.Unlock()
			t.Fatal("equipment should still exist")
		}
		gcas.mu.Unlock()
	}

	// Update the timeslot just enough that we shouldn't be getting pruned still.
	glow.SetCurrentTimeslot(3000)
	// Wait 350 milliseconds, which should trigger a prune. Except that no
	// prune should be triggered because we aren't inside the prune window.
	time.Sleep(350 * time.Millisecond)
	// Verify that nothing got pruned.
	for i := 0; i < 4032; i++ {
		gcas.mu.Lock()
		if gcas.equipmentReports[dummyEquipment.ShortID][i].PowerOutput < 2 {
			gcas.mu.Unlock()
			t.Fatal("equipment should still exist")
		}
		gcas.mu.Unlock()
	}

	// Update the timeslot just enough that things should be getting pruned now.
	glow.SetCurrentTimeslot(3300)
	// Wait 350 milliseconds, which should trigger a prune. Except that no
	// prune should be triggered because we aren't inside the prune window.
	time.Sleep(350 * time.Millisecond)
	// Verify that things got pruned
	for i := 0; i < 2016; i++ {
		gcas.mu.Lock()
		if gcas.equipmentReports[dummyEquipment.ShortID][i].PowerOutput < 2 {
			gcas.mu.Unlock()
			t.Fatal("equipment should have been pruned")
		}
		gcas.mu.Unlock()
	}
	for i := 2016; i < 4032; i++ {
		gcas.mu.Lock()
		if gcas.equipmentReports[dummyEquipment.ShortID][i].PowerOutput != 0 {
			gcas.mu.Unlock()
			t.Fatal("equipment should still exist")
		}
		gcas.mu.Unlock()
	}

	// Wait for another prune cycle, verify nothing happens.
	time.Sleep(350 * time.Millisecond)
	for i := 0; i < 2016; i++ {
		gcas.mu.Lock()
		if gcas.equipmentReports[dummyEquipment.ShortID][i].PowerOutput < 2 {
			gcas.mu.Unlock()
			t.Fatal("equipment should still exist")
		}
		gcas.mu.Unlock()
	}
	for i := 2016; i < 4032; i++ {
		gcas.mu.Lock()
		if gcas.equipmentReports[dummyEquipment.ShortID][i].PowerOutput != 0 {
			gcas.mu.Unlock()
			t.Fatal("equipment should still exist")
		}
		gcas.mu.Unlock()
	}

	// Update the time to 5000, which should still not cause a prune.
	time.Sleep(350 * time.Millisecond)
	for i := 0; i < 2016; i++ {
		gcas.mu.Lock()
		if gcas.equipmentReports[dummyEquipment.ShortID][i].PowerOutput < 2 {
			gcas.mu.Unlock()
			t.Fatal("equipment should still exist")
		}
		gcas.mu.Unlock()
	}
	for i := 2016; i < 4032; i++ {
		gcas.mu.Lock()
		if gcas.equipmentReports[dummyEquipment.ShortID][i].PowerOutput != 0 {
			gcas.mu.Unlock()
			t.Fatal("equipment should still exist")
		}
		gcas.mu.Unlock()
	}

	// Update the current timeslot to trigger another migration, now all of
	// the reports should be migrated out.
	glow.SetCurrentTimeslot(5300)
	time.Sleep(350 * time.Millisecond)
	for i := 0; i < 4032; i++ {
		gcas.mu.Lock()
		if gcas.equipmentReports[dummyEquipment.ShortID][i].PowerOutput != 0 {
			t.Error("all the reports should have been cycled out:", i)
		}
		gcas.mu.Unlock()
	}
}
