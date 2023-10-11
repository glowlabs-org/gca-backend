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
	equipment := make([]EquipmentAuthorization, numEquipment)
	privKeys := make([]ed25519.PrivateKey, numEquipment)

	for i := 0; i < numEquipment; i++ {
		pubKey, privKey := generateTestKeys()
		equipment[i] = EquipmentAuthorization{ShortID: uint32(i), PublicKey: pubKey}
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

	server.loadEquipmentAuths(equipment)

	for i, e := range equipment {
		// Create a mock valid report for each device.
		reportData := make([]byte, 16)
		binary.BigEndian.PutUint32(reportData[0:4], e.ShortID)      // Set ShortID
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
