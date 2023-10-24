package main

// This file contains one large integration test focused on detecting race
// conditions. It's intentionally a pretty chaotic test that has every API
// being accessed rapidly, simultaneously. It's not worried so much about the
// correctness of the logic as it is worried about ensuring that each piece is
// able to operate independently and maintain a level of consistency, all
// without setting off the race detector.

import (
	"testing"
	"time"
)

// TestConcurrency is a large integration test that tries to get actions firing
// on all APIs of the server simultanously while the race detector runs, to
// determine whether there are any race conditions at play.
//
// One of the major points of this test in particular is to run all the code
// "by the book" - we try as much as possible to avoid referencing the internal
// state of the gcaServer and instead just query its APIs.
func TestConcurrency(t *testing.T) {
	// The concurrency test starts at time 0 with no GCA key provided, only
	// the temp key is in place. This means that all of the concurrent
	// operations will be failing, but that's okay. We want to make sure
	// everything is failing, and that there are no race conditions.
	//
	// That means we start with a server that has no temp key.
	dir := generateTestDir(t.Name())
	gcas, tempPrivKey, err := gcaServerWithTempKey(dir)
	if err != nil {
		t.Fatal(err)
	}

	stopSignal := make(chan struct{})

	// Create a few threads that will be repeatedly submitting GCA keys
	// with the wrong signature. Every try should fail.
	for i := 0; i < 3; i++ {
		// Create the bad key.
		badKey := tempPrivKey
		badKey[0] += byte(i + 1)

		// Create a goroutine to repeatedly submit the bad key .
		go func(key PrivateKey) {
			// Try until the stop signal is sent.
			i := 0
			for {
				// Try submitting the key.
				_, err := gcas.submitGCAKey(key)
				if err == nil {
					t.Fatal("should not be able to submit a GCA key with a bad private key")
				}

				// Check for the stop signal.
				select {
				case <-stopSignal:
					return
				default:
				}

				// Wait 10 milliseconds between every 5th
				// attempt to minimize cpu spam.
				if i%5 == 0 {
					time.Sleep(10 * time.Millisecond)
				}
				i++
			}
		}(badKey)
	}

	// Create a few threads that will be repeatedly attempting to authorize
	// equipment using a bad key. Every try should fail.
	for i := 0; i < 3; i++ {
		// Create the bad key.
		badKey := tempPrivKey
		badKey[0] += byte(i + 1)

		// Create a goroutine to repeatedly submit the bad key .
		go func(key PrivateKey) {
			// Try until the stop signal is sent.
			i := 0
			for {
				// Try submitting some new hardware.
				_, _, err := gcas.submitNewHardware(666777, key)
				if err == nil {
					t.Fatal("should not be able to submit a GCA key with a bad private key")
				}

				// Check for the stop signal.
				select {
				case <-stopSignal:
					return
				default:
				}

				// Wait 10 milliseconds between every 5th
				// attempt to minimize cpu spam.
				if i%5 == 0 {
					time.Sleep(10 * time.Millisecond)
				}
				i++
			}
		}(badKey)
	}

	// Create a few threads that will be repeatedly sending bad reports
	// from equipment that was never authorized.
	for i := 0; i < 3; i++ {
		// Create a real keypair for the equipment, then create the
		// equipment. The Signature is left blank because no valid
		// signature auth can exist yet, as we haven't generated the
		// GCA key yet.
		ePub, ePriv := GenerateKeyPair()
		ea := EquipmentAuthorization{
			ShortID:    5,
			PublicKey:  ePub,
			Capacity:   15e9,
			Debt:       5e6,
			Expiration: 15e6,
		}
		go func(ea EquipmentAuthorization, ePriv PrivateKey) {
			// Try until the stop signal is sent.
			i := 0
			for {
				// Try submitting a new report.
				err := gcas.sendEquipmentReport(ea, ePriv)
				if err != nil {
					t.Fatal("even though the reports are invalid, they should still send correctly over UDP")
				}

				// Check for the stop signal.
				select {
				case <-stopSignal:
					return
				default:
				}

				// Wait 10 milliseconds between every 5th
				// attempt to minimize cpu spam.
				if i%5 == 0 {
					time.Sleep(10 * time.Millisecond)
				}
				i++
			}
		}(ea, ePriv)
	}

	// Let everything run for a bit before the test ends. Most of the loops
	// sleep for 10 milliseconds at a time and then do work, so every loop
	// should get plenty of iterations in within 250 milliseconds.
	time.Sleep(250 * time.Millisecond)

	/* - a bunch of reference code for when we build the real test. Right
	* now there is no 'by the book' method for setting up the GCA keys with
	* the server, so this test can't be built at all without shortcuts.
	* Rather than write a test with shortcuts (which is what all of the
	* other tests have done), we'll hold off on this test until the full
	* infra is in place.
	// Basic server setup.
	dir := generateTestDir(t.Name())
	gcaPrivKey, err := generateGCATestKeys(dir)
	if err != nil {
		t.Fatal(err)
	}
	server := NewGCAServer(dir)

	// Generate multiple test key pairs for devices.
	numDevices := 3
	devices := make([]EquipmentAuthorization, numDevices)
	privKeys := make([]PrivateKey, numDevices)

	for i := 0; i < numDevices; i++ {
		pubKey, privKey := GenerateKeyPair()
		devices[i] = EquipmentAuthorization{ShortID: uint32(i), PublicKey: pubKey}
		privKeys[i] = privKey
		sb := devices[i].SigningBytes()
		sig := Sign(sb, gcaPrivKey)
		devices[i].Signature = sig
		err := server.saveEquipment(devices[i])
		if err != nil {
			t.Fatal(err)
		}
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
		if err := sendUDPReport(er.Serialize(), server.udpPort); err != nil {
			t.Fatalf("Failed to send UDP report for device %d: %v", i, err)
		}
		expectedReports++

		// One time, we'll go ahead and guarantee that the report is
		// sent multiple times. The duplicates should be ignored.
		if i == 0 {
			time.Sleep(25 * time.Millisecond)
			if err := sendUDPReport(er.Serialize(), server.udpPort); err != nil {
				t.Fatalf("Failed to send UDP report for device %d: %v", i, err)
			}
			time.Sleep(25 * time.Millisecond)
			if err := sendUDPReport(er.Serialize(), server.udpPort); err != nil {
				t.Fatalf("Failed to send UDP report for device %d: %v", i, err)
			}
		}

		// Loop and check the server processing instead of a fixed sleep.
		success := false
		retries := 0
		for retries = 0; retries < 10; retries++ {
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
			if err := sendUDPReport(er.Serialize(), server.udpPort); err != nil {
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
			if err := sendUDPReport(er.Serialize(), server.udpPort); err != nil {
				t.Fatalf("Failed to send wrongly signed UDP report for device %d: %v", i, err)
			}
			time.Sleep(25 * time.Millisecond)
			if err := sendUDPReport(er.Serialize(), server.udpPort); err != nil {
				t.Fatalf("Failed to send wrongly signed UDP report for device %d: %v", i, err)
			}
			time.Sleep(25 * time.Millisecond)
			if err := sendUDPReport(er.Serialize(), server.udpPort); err != nil {
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

		// Skip the ban testing for the first device, so that we can
		// get better coverage when checking what happens after a
		// server reset.
		if i == 0 {
			continue
		}

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
		if err := sendUDPReport(er.Serialize(), server.udpPort); err != nil {
			t.Fatalf("Failed to send UDP report for device %d: %v", i, err)
		}
		expectedReports++
		// Loop and check for the report to make it into the report
		// history.
		success = false
		retries = 0
		for retries = 0; retries < 10; retries++ {
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
			if err := sendUDPReport(er.Serialize(), server.udpPort); err != nil {
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

		// Now send a report with a correct sig, but have it be yet
		// another duplicate. The report should be entirely ignored and
		// not added to the list of recent reports.
		er = EquipmentReport{
			ShortID:     device.ShortID,
			Timeslot:    uint32(i) + now,
			PowerOutput: uint64(7 + i*100),
		}
		// Correctly sign the report
		er.Signature = Sign(er.SigningBytes(), privKeys[i])
		// Send the report over UDP. Since the server state isn't
		// supposed to change, we need to send it multiple times just
		// to be certain it gets through.
		if err := sendUDPReport(er.Serialize(), server.udpPort); err != nil {
			t.Fatalf("Failed to send UDP report for device %d: %v", i, err)
		}
		time.Sleep(25 * time.Millisecond)
		if err := sendUDPReport(er.Serialize(), server.udpPort); err != nil {
			t.Fatalf("Failed to send UDP report for device %d: %v", i, err)
		}
		time.Sleep(25 * time.Millisecond)
		if err := sendUDPReport(er.Serialize(), server.udpPort); err != nil {
			t.Fatalf("Failed to send UDP report for device %d: %v", i, err)
		}
		// Loop and check for the report to make it into the report
		// history.
		server.mu.Lock()
		if len(server.recentReports) != expectedReports {
			t.Fatal("bad")
		}
		server.mu.Unlock()
		if !success {
			t.Fatalf("No reports in recentReports after sending valid report for device %d", i)
		}
		server.mu.Lock()
		if server.equipmentReports[device.ShortID][uint32(i)+now].PowerOutput != 1 {
			t.Error("report was not banned")
		}
		server.mu.Unlock()
	}

	// Turn off the server and turn it back on, checking that there are
	// still banned reports.
	server.Close()
	server = NewGCAServer(dir)
	defer server.Close()

	// Check that the count for the recent reports is correct.
	server.mu.Lock()
	if len(server.recentReports) != expectedReports {
		// t.Error("server state appears incorrect after reboot", len(server.recentReports), expectedReports)
	}
	server.mu.Unlock()

	for i, device := range devices {
		// For the first device, the report should not be banned. For
		// all other devices, the report should be banned.
		server.mu.Lock()
		if i == 0 {
			if server.equipmentReports[device.ShortID][uint32(i)+now].PowerOutput < 2 {
				t.Error("report does not appear to exist after restart, or maybe its banned")
			}
		} else {
			if server.equipmentReports[device.ShortID][uint32(i)+now].PowerOutput != 1 {
				t.Error("report is not banned after restart")
			}
		}
		server.mu.Unlock()
	}
	*/
}
