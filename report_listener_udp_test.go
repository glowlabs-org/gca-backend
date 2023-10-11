package main

import (
	"crypto/ed25519"
	"encoding/binary"
	"fmt"
	"testing"
)

// TestParseReport tests the parseReport function of the GCAServer.
// It ensures that:
//   - Valid reports are successfully parsed.
//   - Reports with invalid signatures are rejected.
//   - Reports from unknown equipment are rejected.
//   - Reports signed by the wrong equipment are rejected.
//   - The values within the report are correctly parsed and verified.
func TestParseReport(t *testing.T) {
	// Generate multiple test key pairs for equipment.
	numEquipment := 3
	equipment := make([]Equipment, numEquipment)
	privKeys := make([]ed25519.PrivateKey, numEquipment)

	for i := 0; i < numEquipment; i++ {
		pubKey, privKey := generateTestKeys()
		equipment[i] = Equipment{ShortID: uint32(i), Key: pubKey}
		privKeys[i] = privKey
	}

	// Setup the GCAServer with the test keys.
	dir := generateTestDir(t.Name())
	_, err := generateGCATestKeys(dir)
	if err != nil {
		t.Fatal(err)
	}
	server := NewGCAServer(dir)
	defer server.Close()

	server.loadEquipmentKeys(equipment)

	for i, e := range equipment {
		// Create a mock valid report for each device.
		reportData := make([]byte, 16)
		binary.BigEndian.PutUint32(reportData[0:4], e.ShortID) // Set ShortID
		binary.BigEndian.PutUint32(reportData[4:8], uint32(i*10))   // Example Timeslot based on i
		binary.BigEndian.PutUint64(reportData[8:16], uint64(i*100)) // Example PowerOutput based on i

		// Correctly signed report
		signature := ed25519.Sign(privKeys[i], reportData)
		fullReport := append(reportData, signature...)

		report, err := server.parseReport(fullReport)
		if err != nil {
			t.Fatalf("Failed to parse valid report for device %d: %v", i, err)
		}

		if report.ShortID != e.ShortID {
			t.Errorf("Unexpected ShortID for device %d: got %v, want %v", i, report.ShortID, e.ShortID)
		}

		if report.Timeslot != uint32(i*10) {
			t.Errorf("Unexpected Timeslot for device %d: got %v, want %v", i, report.Timeslot, i*10)
		}

		if report.PowerOutput != uint64(i*100) {
			t.Errorf("Unexpected PowerOutput for device %d: got %v, want %v", i, report.PowerOutput, i*100)
		}

		// Report signed by the wrong device (using next device's private key for signature)
		if i < numEquipment-1 {
			wrongSignature := ed25519.Sign(privKeys[i+1], reportData)
			wrongFullReport := append(reportData, wrongSignature...)
			_, err = server.parseReport(wrongFullReport)
			if err == nil || err.Error() != "failed to verify signature" {
				t.Errorf("Expected signature verification failed error for wrong device signature, got: %v", err)
			}
		}
	}

	// Test with an invalid signature.
	invalidSignature := make([]byte, 64) // Just an example of an invalid signature
	blankReport := make([]byte, 16)
	fullReportInvalidSignature := append(blankReport, invalidSignature...)
	_, err = server.parseReport(fullReportInvalidSignature)
	if err == nil || err.Error() != "failed to verify signature" {
		t.Errorf("Expected signature verification failed error, got: %v", err)
	}

	// Test with a device not in the server's list.
	reportData := make([]byte, 16) // make a blank report for a non-existent device
	binary.BigEndian.PutUint32(reportData[0:4], uint32(numEquipment+1))
	_, err = server.parseReport(append(reportData, invalidSignature...))
	if err == nil || err.Error() != fmt.Sprintf("unknown equipment ID: %d", numEquipment+1) {
		t.Errorf("Expected unknown device ID error, got: %v", err)
	}
}

// TestHandleEquipmentReport_MaxRecentReports tests the GCAServer's
// behavior when the maximum number of recent reports is reached.
func TestHandleEquipmentReport_MaxRecentReports(t *testing.T) {
	// This test has an implicit assumption about the constants. Panic if the assumption is not maintained.
	if int(maxRecentReports)%50 != 0 {
		panic("bad constant")
	}

	// Create test devices
	var devices []Equipment
	var privKeys []ed25519.PrivateKey

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
		pubKey, privKey, _ := ed25519.GenerateKey(nil)
		device := Equipment{
			ShortID: uint32(i),
			Key:     pubKey,
		}
		devices = append(devices, device)
		privKeys = append(privKeys, privKey)
	}
	server.loadEquipmentKeys(devices)

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
