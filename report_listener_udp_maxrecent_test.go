package main

import (
	"testing"
)

// TestHandleEquipmentReport_MaxRecentReports tests the GCAServer's
// behavior when the maximum number of recent reports is reached.
func TestHandleEquipmentReport_MaxRecentReports(t *testing.T) {
	// This test has an implicit assumption about the constants. Panic if the assumption is not maintained.
	if int(maxRecentReports)%50 != 0 {
		panic("bad constant")
	}

	// Create test devices
	var devices []EquipmentAuthorization
	var privKeys []PrivateKey

	// Generate test directory and GCA keys
	dir := generateTestDir(t.Name())
	_, err := generateGCATestKeys(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Create the GCA Server
	server := NewGCAServer(dir)
	defer server.Close() // Ensure resources are cleaned up after the test.

	// Create enough devices to fill out all the maxRecentReports in the current time period.
	for i := 0; i < 1+(maxRecentReports/50); i++ {
		pubKey, privKey := GenerateKeyPair()
		device := EquipmentAuthorization{
			ShortID:   uint32(i),
			PublicKey: pubKey,
		}
		devices = append(devices, device)
		privKeys = append(privKeys, privKey)
	}
	for _, d := range devices {
		server.loadEquipmentAuth(d)
	}

	// Submit enough reports to saturate the maxRecentReports field.
	for i := 0; i < maxRecentReports/50; i++ {
		for j := 0; j < 50; j++ {
			timeslot := uint32(j)
			report := generateTestReport(devices[i].ShortID, timeslot, privKeys[i])
			server.handleEquipmentReport(report)
		}
	}

	// Ensure we saturated the number of reports.
	if len(server.recentReports) != int(maxRecentReports) {
		t.Fatalf("Expected %f reports, but got %d", maxRecentReports, len(server.recentReports))
	}

	// Store a reference to the second half of the original recentReports list.
	expectedReports := append([]EquipmentReport(nil), server.recentReports[maxRecentReports/2:]...)

	// Submit another report, using the final device.
	report := generateTestReport(devices[maxRecentReports/50].ShortID, 0, privKeys[maxRecentReports/50]) // Use a new device ID, timeslot = 0
	server.handleEquipmentReport(report)

	if len(server.recentReports) != int(maxRecentReports)/2 {
		t.Fatalf("Expected %f reports after adding one more, but got %d", maxRecentReports/2+1, len(server.recentReports))
	}

	// Verify that the first half of the reports were removed
	firstReport := server.recentReports[0]
	if firstReport.ShortID != uint32(maxRecentReports)/50/2 || firstReport.Timeslot != uint32(maxRecentReports)%50 {
		t.Fatalf("Expected first report to be ShortID %d and Timeslot %d, but got ShortID %d and Timeslot %d",
			uint32(maxRecentReports/50/2), uint32(maxRecentReports)%50, firstReport.ShortID, firstReport.Timeslot)
	}

	// Now we'll iterate over the two slices (expectedReports and server.recentReports) and ensure they match.
	for i, report := range server.recentReports {
		if i == len(expectedReports) {
			// This is the new report we added after trimming the original list. We should not try to compare it against the expectedReports.
			break
		}

		expected := expectedReports[i]
		if report.ShortID != expected.ShortID || report.Timeslot != expected.Timeslot || report.PowerOutput != expected.PowerOutput || report.Signature != expected.Signature {
			t.Fatalf("Mismatch at index %d: Expected %+v, got %+v", i, expected, report)
		}
	}
}

