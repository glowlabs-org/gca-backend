package main

import (
	"crypto/ed25519"
	"encoding/binary"
	"testing"
	"time"
)

// TestParseReportIntegration tests the GCAServer's ability to correctly
// process and record device reports that are sent over UDP.
func TestParseReportIntegration(t *testing.T) {
	// Setup the GCAServer with the test keys. This happens first so that it has time to initialize
	// before we generate all of the keypairs. We also sleep for 250ms because we found it decreases
	// flaking.
	dir := generateTestDir(t.Name())
	_, err := generateGCATestKeys(dir)
	if err != nil {
		t.Fatal(err)
	}
	server := NewGCAServer(dir)
	defer server.Close()
	time.Sleep(250 * time.Millisecond)

	// Generate multiple test key pairs for devices.
	numDevices := 3
	devices := make([]EquipmentAuthorization, numDevices)
	privKeys := make([]ed25519.PrivateKey, numDevices)

	for i := 0; i < numDevices; i++ {
		pubKey, privKey := generateTestKeys()
		devices[i] = EquipmentAuthorization{ShortID: uint32(i), PublicKey: pubKey}
		privKeys[i] = privKey
	}
	server.loadEquipmentKeys(devices)

	for i, device := range devices {
		reportData := make([]byte, 16)
		binary.BigEndian.PutUint32(reportData[0:4], device.ShortID)
		binary.BigEndian.PutUint32(reportData[4:8], uint32(i*10))
		binary.BigEndian.PutUint64(reportData[8:16], uint64(i*100))

		// Correctly signed report
		signature := ed25519.Sign(privKeys[i], reportData)
		fullReport := append(reportData, signature...)

		// Send the report over UDP
		if err := sendUDPReport(fullReport); err != nil {
			t.Fatalf("Failed to send UDP report for device %d: %v", i, err)
		}

		// Loop and check the server processing instead of a fixed sleep.
		success := false
		for retries := 0; retries < 200; retries++ {
			if len(server.recentReports) == i+1 {
				lastReport := server.recentReports[len(server.recentReports)-1]
				if lastReport.ShortID == device.ShortID && lastReport.Timeslot == uint32(i*10) && lastReport.PowerOutput == uint64(i*100) {
					success = true
					break
				}
			}
			time.Sleep(10 * time.Millisecond)

			// TODO: Try sending the report again over UDP, but only after implmenting code that prevents
			// two of the same report from being recorded twice.
		}
		if !success {
			t.Fatalf("No reports in recentReports after sending valid report for device %d", i)
		}

		lastReport := server.recentReports[len(server.recentReports)-1]
		if lastReport.ShortID != device.ShortID || lastReport.Timeslot != uint32(i*10) || lastReport.PowerOutput != uint64(i*100) {
			t.Errorf("Unexpected report details for device %d: got %+v", i, lastReport)
		}

		// Report signed by the wrong device (using next device's private key for signature)
		if i < numDevices-1 {
			wrongSignature := ed25519.Sign(privKeys[i+1], reportData)
			wrongFullReport := append(reportData, wrongSignature...)
			if err := sendUDPReport(wrongFullReport); err != nil {
				t.Fatalf("Failed to send wrongly signed UDP report for device %d: %v", i, err)
			}

			// Loop and check the server processing after sending wrongly signed report.
			success = false
			for retries := 0; retries < 100; retries++ {
				if len(server.recentReports) == i+1 {
					success = true
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			if !success {
				t.Errorf("Unexpected number of reports in recentReports after sending wrongly signed report for device %d: got %d, expected %d", i, len(server.recentReports), i+1)
			}
		}
	}
}
