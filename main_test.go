package main

import (
	"crypto/ed25519"
	"encoding/binary"
	"fmt"
	"net"
	"testing"
	"time"
)

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
	server.loadDeviceKeys(devices)

	for i, device := range devices {
		// Create a mock valid report for each device.
		reportData := make([]byte, 16)
		binary.BigEndian.PutUint32(reportData[0:4], device.ShortID) // Set ShortID
		binary.BigEndian.PutUint32(reportData[4:8], uint32(i*10))  // Example Timeslot based on i
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
			if err == nil || err.Error() != "signature verification failed" {
				t.Errorf("Expected signature verification failed error for wrong device signature, got: %v", err)
			}
		}
	}

	// Test with an invalid signature.
	invalidSignature := make([]byte, 64) // Just an example of an invalid signature
	blankReport := make([]byte, 16)
	fullReportInvalidSignature := append(blankReport, invalidSignature...)
	_, err := server.parseReport(fullReportInvalidSignature)
	if err == nil || err.Error() != "signature verification failed" {
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

func sendUDPReport(report []byte) error {
	conn, err := net.Dial("udp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = conn.Write(report)
	return err
}

func TestParseReportIntegration(t *testing.T) {
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

		// Sleep for a bit to let server process the report.
		time.Sleep(1000 * time.Millisecond)

		// Check if the report was added to recentReports
		if len(server.recentReports) != i+1 {
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
			time.Sleep(100 * time.Millisecond)

			// Since the report is invalid, it should not be added to recentReports
			if len(server.recentReports) != i+1 {
				t.Errorf("Unexpected number of reports in recentReports after sending wrongly signed report for device %d: got %d, expected %d", i, len(server.recentReports), i+1)
			}
		}
	}
}
