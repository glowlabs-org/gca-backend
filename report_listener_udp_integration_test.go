package main

import (
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
	privKeys := make([]PrivateKey, numDevices)

	for i := 0; i < numDevices; i++ {
		pubKey, privKey := GenerateKeyPair()
		devices[i] = EquipmentAuthorization{ShortID: uint32(i), PublicKey: pubKey}
		privKeys[i] = privKey
	}
	for _, d := range devices {
		server.loadEquipmentAuth(d)
	}

	now := currentTimeslot()
	for i, device := range devices {
		er := EquipmentReport{
			ShortID:     device.ShortID,
			Timeslot:    uint32(i) + now,
			PowerOutput: uint64(5 + i*100),
		}
		// Correctly signed report
		er.Signature = Sign(er.SigningBytes(), privKeys[i])

		// Send the report over UDP
		if err := sendUDPReport(er.Serialize()); err != nil {
			t.Fatalf("Failed to send UDP report for device %d: %v", i, err)
		}

		// Loop and check the server processing instead of a fixed sleep.
		success := false
		retries := 0
		for retries = 0; retries < 200; retries++ {
			if len(server.recentReports) == i+1 {
				lastReport := server.recentReports[len(server.recentReports)-1]
				if lastReport.ShortID == device.ShortID && lastReport.Timeslot == uint32(i)+now && lastReport.PowerOutput == uint64(5+i*100) {
					success = true
					break
				}
			}

			// Sleep a bit and try sending the report again.
			time.Sleep(10 * time.Millisecond)
			if err := sendUDPReport(er.Serialize()); err != nil {
				t.Fatalf("Failed to send UDP report for device %d: %v", i, err)
			}
		}
		if !success {
			t.Fatalf("No reports in recentReports after sending valid report for device %d", i)
		}
		if retries > 3 {
			t.Log("retries:", retries)
		}

		lastReport := server.recentReports[len(server.recentReports)-1]
		if lastReport.ShortID != device.ShortID || lastReport.Timeslot != uint32(i)+now || lastReport.PowerOutput != uint64(5+i*100) {
			t.Errorf("Unexpected report details for device %d: got %+v", i, lastReport)
		}

		// Report signed by the wrong device (using next device's private key for signature)
		if i < numDevices-1 {
			er.Signature = Sign(er.Serialize(), privKeys[i+1])
			if err := sendUDPReport(er.Serialize()); err != nil {
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
