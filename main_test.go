package main

import (
	"crypto/ed25519"
	// "net"
	"testing"
	// "time"
)

func generateTestKeys() (ed25519.PublicKey, ed25519.PrivateKey) {
	pubKey, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		panic(err)
	}
	return pubKey, privKey
}

func TestParseReport(t *testing.T) {
	// Generate test key for a device.
	pubKey, privKey := generateTestKeys()

	// Setup the GCAServer with the test key.
	server := NewGCAServer()
	server.loadDeviceKeys([]Device{{ShortID: 0, Key: pubKey}})

	// Create a mock valid report.
	reportData := make([]byte, 16) // make a blank report
	signature := ed25519.Sign(privKey, reportData)
	fullReport := append(reportData, signature...)
	println(len(fullReport))

	report, err := server.parseReport(fullReport)
	if err != nil {
		t.Fatalf("Failed to parse valid report: %v", err)
	}

	if report.ShortID != 0 {
		t.Errorf("Unexpected ShortID: got %v, want %v", report.ShortID, 12345)
	}
	
	// Test with an invalid signature.
	invalidSignature := make([]byte, 64) // Just an example of an invalid signature
	fullReportInvalidSignature := append(reportData, invalidSignature...)
	_, err = server.parseReport(fullReportInvalidSignature)
	if err == nil || err.Error() != "signature verification failed" {
		t.Errorf("Expected signature verification failed error, got: %v", err)
	}
}

/*
func TestIntegrationReceiveReport(t *testing.T) {
	// Generate test key for a device.
	pubKey, privKey := generateTestKeys()

	// Setup the GCAServer with the test key.
	server := NewGCAServer()
	server.loadDeviceKeys([]Device{{ShortID: 12345, Key: pubKey}})

	// Use a channel to know when the server has processed the report.
	reportProcessed := make(chan bool, 1)
	originalHandler := server.handleEquipmentReport

	// Overwrite handleEquipmentReport for this test to signal when report is processed.
	server.handleEquipmentReport = func(report EquipmentReport) {
		originalHandler(report) // Call original handler.
		reportProcessed <- true
	}

	// Start the UDP server in a separate goroutine.
	go server.startUDPServer()

	// Create and send a mock valid report to the server.
	reportData := make([]byte, 16) // excluding signature
	// ... you can populate the reportData with specific values if needed
	signature := ed25519.Sign(privKey, reportData)
	fullReport := append(reportData, signature...)

	conn, err := net.Dial("udp", "localhost:35030")
	if err != nil {
		t.Fatalf("Failed to dial server: %v", err)
	}
	defer conn.Close()

	_, err = conn.Write(fullReport)
	if err != nil {
		t.Fatalf("Failed to write to server: %v", err)
	}

	// Wait for report processing or timeout.
	select {
	case <-reportProcessed:
		// Continue with any further checks or assertions on the report processing result.
	case <-time.After(time.Second * 5): // 5-second timeout.
		t.Fatal("Timeout waiting for report processing")
	}

	// Optionally, stop the server. If the startUDPServer function has a stop mechanism.
	// server.stop()
}
*/
