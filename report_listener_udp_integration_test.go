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

	// Generate multiple test key pairs for devices.
	numDevices := 3
	devices := make([]EquipmentAuthorization, numDevices)
	privKeys := make([]PrivateKey, numDevices)

	for i := 0; i < numDevices; i++ {
		pubKey, privKey := GenerateKeyPair()
		devices[i] = EquipmentAuthorization{ShortID: uint32(i), PublicKey: pubKey}
		privKeys[i] = privKey
	}
	server.mu.Lock()
	for _, d := range devices {
		server.loadEquipmentAuth(d)
	}
	server.mu.Unlock()

	now := currentTimeslot()
	expectedReports := 0
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
		expectedReports++

		// One time, we'll go ahead and guarantee that the report is
		// sent multiple times. The duplicates should be ignored.
		if i == 0 {
			time.Sleep(25 * time.Millisecond)
			if err := sendUDPReport(er.Serialize()); err != nil {
				t.Fatalf("Failed to send UDP report for device %d: %v", i, err)
			}
			time.Sleep(25 * time.Millisecond)
			if err := sendUDPReport(er.Serialize()); err != nil {
				t.Fatalf("Failed to send UDP report for device %d: %v", i, err)
			}
		}

		// Loop and check the server processing instead of a fixed sleep.
		success := false
		retries := 0
		for retries = 0; retries < 200; retries++ {
			server.mu.Lock()
			if len(server.recentReports) == expectedReports {
				lastReport := server.recentReports[len(server.recentReports)-1]
				if lastReport.ShortID == device.ShortID && lastReport.Timeslot == uint32(i)+now && lastReport.PowerOutput == uint64(5+i*100) {
					success = true
					server.mu.Unlock()
					break
				}
			}
			server.mu.Unlock()

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
		server.mu.Lock()
		if server.equipmentReports[device.ShortID][uint32(i)+now].PowerOutput < 2 {
			t.Error("report is either banned or didn't get added to the state, when it should exist")
		}
		server.mu.Unlock()

		lastReport := server.recentReports[len(server.recentReports)-1]
		if lastReport.ShortID != device.ShortID || lastReport.Timeslot != uint32(i)+now || lastReport.PowerOutput != uint64(5+i*100) {
			t.Errorf("Unexpected report details for device %d: got %+v", i, lastReport)
		}

		// Report signed by the wrong device (using next device's
		// private key for signature). Send the reports three times
		// with some sleeps in between. The report isn't supposed to
		// change the state of the server at all so we have to
		// non-deterministically catch this.
		if i < numDevices-1 {
			er.Signature = Sign(er.Serialize(), privKeys[i+1])
			if err := sendUDPReport(er.Serialize()); err != nil {
				t.Fatalf("Failed to send wrongly signed UDP report for device %d: %v", i, err)
			}
			time.Sleep(25 * time.Millisecond)
			if err := sendUDPReport(er.Serialize()); err != nil {
				t.Fatalf("Failed to send wrongly signed UDP report for device %d: %v", i, err)
			}
			time.Sleep(25 * time.Millisecond)
			if err := sendUDPReport(er.Serialize()); err != nil {
				t.Fatalf("Failed to send wrongly signed UDP report for device %d: %v", i, err)
			}
			if len(server.recentReports) != expectedReports {
				t.Fatal("picked up an extra report it seems")
			}
		}
		server.mu.Lock()
		if server.equipmentReports[device.ShortID][uint32(i)+now].PowerOutput == 1 {
			t.Error("report was banned, though it shouldn't have been banned because the sig was bad")
		}
		server.mu.Unlock()

		// Now send a report with a correct sig, but have it be a
		// duplicate, which should cause the report to get banned.
		er = EquipmentReport{
			ShortID:     device.ShortID,
			Timeslot:    uint32(i) + now,
			PowerOutput: uint64(6 + i*100),
		}
		// Correctly sign the report
		er.Signature = Sign(er.SigningBytes(), privKeys[i])
		// Send the report over UDP
		if err := sendUDPReport(er.Serialize()); err != nil {
			t.Fatalf("Failed to send UDP report for device %d: %v", i, err)
		}
		expectedReports++
		// Loop and check for the report to make it into the report
		// history.
		success = false
		retries = 0
		for retries = 0; retries < 200; retries++ {
			server.mu.Lock()
			if len(server.recentReports) == expectedReports {
				lastReport := server.recentReports[len(server.recentReports)-1]
				if lastReport.ShortID == device.ShortID && lastReport.Timeslot == uint32(i)+now && lastReport.PowerOutput == uint64(6+i*100) {
					success = true
					server.mu.Unlock()
					break
				}
			}
			server.mu.Unlock()

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
		server.mu.Lock()
		if server.equipmentReports[device.ShortID][uint32(i)+now].PowerOutput != 1 {
			t.Error("report was not banned")
		}
		server.mu.Unlock()
	}
}
