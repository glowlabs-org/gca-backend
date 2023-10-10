package main

import (
	"crypto/ed25519"
	"encoding/binary"
	"fmt"
	"net"
	"testing"
	"time"
)

// generateTestKeys generates an ed25519 public-private key pair.
// This is used to simulate devices in our tests.
func generateTestKeys() (ed25519.PublicKey, ed25519.PrivateKey) {
	pubKey, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		panic(err)
	}
	return pubKey, privKey
}

// TestParseReport tests the parseReport function of the GCAServer.
// It ensures that:
//   - Valid reports are successfully parsed.
//   - Reports with invalid signatures are rejected.
//   - Reports from unknown devices are rejected.
//   - Reports signed by the wrong device are rejected.
//   - The values within the report are correctly parsed and verified.
func TestParseReport(t *testing.T) {
	// Generate multiple test key pairs for devices.
	numDevices := 3
	devices := make([]Device, numDevices)
	privKeys := make([]ed25519.PrivateKey, numDevices)

	for i := 0; i < numDevices; i++ {
		pubKey, privKey := generateTestKeys()
		devices[i] = Device{ShortID: uint32(i), Key: pubKey}
		privKeys[i] = privKey
	}

	// Setup the GCAServer with the test keys.
	server := NewGCAServer()
	defer server.Close()
	server.loadDeviceKeys(devices)

	for i, device := range devices {
		// Create a mock valid report for each device.
		reportData := make([]byte, 16)
		binary.BigEndian.PutUint32(reportData[0:4], device.ShortID) // Set ShortID
		binary.BigEndian.PutUint32(reportData[4:8], uint32(i*10))   // Example Timeslot based on i
		binary.BigEndian.PutUint64(reportData[8:16], uint64(i*100)) // Example PowerOutput based on i

		// Correctly signed report
		signature := ed25519.Sign(privKeys[i], reportData)
		fullReport := append(reportData, signature...)

		report, err := server.parseReport(fullReport)
		if err != nil {
			t.Fatalf("Failed to parse valid report for device %d: %v", i, err)
		}

		if report.ShortID != device.ShortID {
			t.Errorf("Unexpected ShortID for device %d: got %v, want %v", i, report.ShortID, device.ShortID)
		}

		if report.Timeslot != uint32(i*10) {
			t.Errorf("Unexpected Timeslot for device %d: got %v, want %v", i, report.Timeslot, i*10)
		}

		if report.PowerOutput != uint64(i*100) {
			t.Errorf("Unexpected PowerOutput for device %d: got %v, want %v", i, report.PowerOutput, i*100)
		}

		// Report signed by the wrong device (using next device's private key for signature)
		if i < numDevices-1 {
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
	_, err := server.parseReport(fullReportInvalidSignature)
	if err == nil || err.Error() != "failed to verify signature" {
		t.Errorf("Expected signature verification failed error, got: %v", err)
	}

	// Test with a device not in the server's list.
	reportData := make([]byte, 16) // make a blank report for a non-existent device
	binary.BigEndian.PutUint32(reportData[0:4], uint32(numDevices+1))
	_, err = server.parseReport(append(reportData, invalidSignature...))
	if err == nil || err.Error() != fmt.Sprintf("unknown device ID: %d", numDevices+1) {
		t.Errorf("Expected unknown device ID error, got: %v", err)
	}
}

// sendUDPReport simulates sending a report to the server via UDP.
// The server should be listening on the given IP and port.
func sendUDPReport(report []byte) error {
	conn, err := net.Dial("udp", fmt.Sprintf("%s:%d", serverIP, udpPort))
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = conn.Write(report)
	return err
}

// TestParseReportIntegration tests the GCAServer's ability to correctly
// process and record device reports that are sent over UDP.
func TestParseReportIntegration(t *testing.T) {
	// Setup the GCAServer with the test keys. This happens first so that it has time to initialize
	// before we generate all of the keypairs. We also sleep for 250ms because we found it decreases
	// flaking.
	server := NewGCAServer()
	defer server.Close()
	time.Sleep(250 * time.Millisecond)

	// Generate multiple test key pairs for devices.
	numDevices := 3
	devices := make([]Device, numDevices)
	privKeys := make([]ed25519.PrivateKey, numDevices)

	for i := 0; i < numDevices; i++ {
		pubKey, privKey := generateTestKeys()
		devices[i] = Device{ShortID: uint32(i), Key: pubKey}
		privKeys[i] = privKey
	}
	server.loadDeviceKeys(devices)

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

// generateTestReport creates a mock report for testing purposes.
// The report includes a signature based on the provided private key.
func generateTestReport(shortID uint32, timeslot uint32, privKey ed25519.PrivateKey) []byte {
	data := make([]byte, 80)
	binary.BigEndian.PutUint32(data[0:4], shortID)
	binary.BigEndian.PutUint32(data[4:8], timeslot)
	// PowerOutput can remain as zero since they don't impact the behavior we're testing.

	// Sign the data using the private key and insert the signature into the report
	signature := ed25519.Sign(privKey, data[:16])
	copy(data[16:], signature)

	return data
}

// TestHandleEquipmentReport_MaxRecentReports tests the GCAServer's
// behavior when the maximum number of recent reports is reached.
func TestHandleEquipmentReport_MaxRecentReports(t *testing.T) {
	// This test has an implicit assumption about the constants. Panic if the assumption is not maintained.
	if int(maxRecentReports)%50 != 0 {
		panic("bad constant")
	}

	// Create test devices
	var devices []Device
	var privKeys []ed25519.PrivateKey
	server := NewGCAServer()
	defer server.Close()

	// Create enough devices to fill out all the maxRecentReports in the current time period.
	for i := 0; i < 1+(maxRecentReports/50); i++ {
		pubKey, privKey, _ := ed25519.GenerateKey(nil)
		device := Device{
			ShortID: uint32(i),
			Key:     pubKey,
		}
		devices = append(devices, device)
		privKeys = append(privKeys, privKey)
	}
	server.loadDeviceKeys(devices)

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
