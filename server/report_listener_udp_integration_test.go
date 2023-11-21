package server

import (
	"fmt"
	"testing"
	"time"

	"github.com/glowlabs-org/gca-backend/glow"
)

// sendEquipmentReport will take an equipment authorization and the private key
// of the equipment, and then send a valid report to the GCA server on behalf
// of the equipment. It will invent numbers for the power output based on the
// authorized limits of the equipment and use the current timeslot as the
// timeslot submission.
//
// NOTE: Because UDP is used as the protocol, this send may fail. It seems like
// the test suite will occasionally fail to send the udp packet even over
// localhost. Therefore this function doesn't do any checking itself to see
// whether the report successfully arrived.
func (gcas *GCAServer) sendEquipmentReport(ea glow.EquipmentAuthorization, ePriv glow.PrivateKey) error {
	// Generate a number between 2 and the capacity for the PowerOutput. We
	// cannot use 0 or 1 because both of those values are sentinel values
	// and thus the report will simply be ignored by the server.
	output := glow.GenerateSecureRandomInt(2, int(ea.Capacity))
	return gcas.sendEquipmentReportSpecific(ea, ePriv, glow.CurrentTimeslot(), uint64(output))
}

// sendEquipmentReportSpecific is the same as sendEquipmentReport, but takes
// specific values for the power output and the timeslot.
func (gcas *GCAServer) sendEquipmentReportSpecific(ea glow.EquipmentAuthorization, ePriv glow.PrivateKey, timeslot uint32, output uint64) error {
	// Create the report and sign it using the provided private key.
	er := glow.EquipmentReport{
		ShortID:     ea.ShortID,
		Timeslot:    timeslot,
		PowerOutput: output,
	}
	er.Signature = glow.Sign(er.SigningBytes(), ePriv)

	// Send the report over UDP.
	if err := sendUDPReport(er.Serialize(), gcas.udpPort); err != nil {
		return fmt.Errorf("Failed to send UDP report for device %d: %v", ea.ShortID, err)
	}
	gcas.logger.Infof("successful send: %v :: %v", er.ShortID, er.Timeslot)
	return nil
}

// TestParseReportIntegration tests the GCAServer's ability to correctly
// process and record device reports that are sent over UDP.
func TestParseReportIntegration(t *testing.T) {
	server, dir, _, gcaPrivKey, err := SetupTestEnvironment(t.Name())
	if err != nil {
		t.Fatal(err)
	}

	// Generate multiple test key pairs for devices.
	numDevices := 3
	devices := make([]glow.EquipmentAuthorization, numDevices)
	privKeys := make([]glow.PrivateKey, numDevices)

	for i := 0; i < numDevices; i++ {
		pubKey, privKey := glow.GenerateKeyPair()
		devices[i] = glow.EquipmentAuthorization{ShortID: uint32(i), PublicKey: pubKey, Capacity: 100e6}
		privKeys[i] = privKey
		sb := devices[i].SigningBytes()
		sig := glow.Sign(sb, gcaPrivKey)
		devices[i].Signature = sig
		_, err := server.saveEquipment(devices[i])
		if err != nil {
			t.Fatal(err)
		}
	}
	server.mu.Lock()
	for _, d := range devices {
		server.loadEquipmentAuth(d)
	}
	server.mu.Unlock()

	now := glow.CurrentTimeslot()
	expectedReports := 0
	for i, device := range devices {
		er := glow.EquipmentReport{
			ShortID:     device.ShortID,
			Timeslot:    uint32(i) + now,
			PowerOutput: uint64(5 + i*100),
		}
		// Correctly signed report
		er.Signature = glow.Sign(er.SigningBytes(), privKeys[i])

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
			er.Signature = glow.Sign(er.Serialize(), privKeys[i+1])
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
		er = glow.EquipmentReport{
			ShortID:     device.ShortID,
			Timeslot:    uint32(i) + now,
			PowerOutput: uint64(6 + i*100),
		}
		// Correctly sign the report
		er.Signature = glow.Sign(er.SigningBytes(), privKeys[i])
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
		er = glow.EquipmentReport{
			ShortID:     device.ShortID,
			Timeslot:    uint32(i) + now,
			PowerOutput: uint64(7 + i*100),
		}
		// Correctly sign the report
		er.Signature = glow.Sign(er.SigningBytes(), privKeys[i])
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
	server, err = NewGCAServer(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	// Check that the count for the recent reports is correct.
	server.mu.Lock()
	if len(server.recentReports) != expectedReports {
		t.Error("server state appears incorrect after reboot", len(server.recentReports), expectedReports)
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
}

// TestCapacityEnforcement will check that a report gets banned for exceeding
// the equipment capacity, and that it stays banned after a reset.
func TestCapacityEnforcement(t *testing.T) {
	server, dir, _, gcaPrivKey, err := SetupTestEnvironment(t.Name())
	if err != nil {
		t.Fatal(err)
	}

	pubKey, privKey := glow.GenerateKeyPair()
	device := glow.EquipmentAuthorization{ShortID: 3, PublicKey: pubKey, Capacity: 100e6}
	sb := device.SigningBytes()
	sig := glow.Sign(sb, gcaPrivKey)
	device.Signature = sig
	_, err = server.saveEquipment(device)
	if err != nil {
		t.Fatal(err)
	}
	server.mu.Lock()
	server.loadEquipmentAuth(device)
	server.mu.Unlock()

	// Create an equipment report that we expect to get banned for having
	// too high of a capacity.
	er := glow.EquipmentReport{
		ShortID:     device.ShortID,
		Timeslot:    5,
		PowerOutput: uint64(200e6),
	}
	// Sign report and send over UDP.
	er.Signature = glow.Sign(er.SigningBytes(), privKey)
	if err := sendUDPReport(er.Serialize(), server.udpPort); err != nil {
		t.Fatalf("Failed to send UDP report")
	}

	// Check that the server received the report.
	success := false
	retries := 0
	for retries = 0; retries < 10; retries++ {
		server.mu.Lock()
		if len(server.recentReports) == 1 {
			lastReport := server.recentReports[len(server.recentReports)-1]
			if lastReport.ShortID == device.ShortID && lastReport.Timeslot == 5 && lastReport.PowerOutput == uint64(200e6) {
				success = true
				server.mu.Unlock()
				break
			}
		}
		server.mu.Unlock()

		// Sleep a bit and try sending the report again.
		time.Sleep(10 * time.Millisecond)
		if err := sendUDPReport(er.Serialize(), server.udpPort); err != nil {
			t.Fatalf("Failed to send UDP report")
		}
	}
	if !success {
		t.Fatalf("No reports in recentReports after sending valid report")
	}
	if retries > 3 {
		t.Log("retries:", retries)
	}
	server.mu.Lock()
	if server.equipmentReports[device.ShortID][5].PowerOutput != 1 {
		t.Error("report is supposed to be banned now")
	}
	server.mu.Unlock()

	// Restart the server and check that the ban is still in place.
	server.CheckInvariants()
	server.Close()
	server, err = NewGCAServer(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	defer server.CheckInvariants()
	server.CheckInvariants()

	// Check that the count for the recent reports is correct.
	server.mu.Lock()
	if len(server.recentReports) != 1 {
		t.Error("server state appears incorrect after reboot", len(server.recentReports), 1)
	}
	server.mu.Unlock()

	// Check that the report is banned.
	server.mu.Lock()
	if server.equipmentReports[device.ShortID][5].PowerOutput != 1 {
		t.Error("report is supposed to be banned now")
	}
	server.mu.Unlock()

	// Check that the report appears when making a tcp request.
	offset, bitfield, err := server.requestEquipmentBitfield(device.ShortID)
	if err != nil {
		t.Fatal(err)
	}
	if offset != 0 {
		t.Fatal("offset not 0")
	}
	if bitfield[0] != 32 {
		t.Fatal(bitfield[0])
	}
}
