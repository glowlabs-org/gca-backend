package server

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/glowlabs-org/gca-backend/glow"
)

// TestParseReport tests the parseReport function of the GCAServer.
// It ensures that:
//   - Valid reports are successfully parsed.
//   - Reports with invalid signatures are rejected.
//   - Reports from unknown equipment are rejected.
//   - Reports signed by the wrong equipment are rejected.
//   - The values within the report are correctly parsed and verified.
func TestParseReport(t *testing.T) {

	// Setup the GCAServer with the test keys.
	server, _, _, err := setupTestEnvironment(t.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	// Generate multiple test key pairs for equipment.
	numEquipment := 3
	equipment := make([]EquipmentAuthorization, numEquipment)
	privKeys := make([]glow.PrivateKey, numEquipment)
	for i := 0; i < numEquipment; i++ {
		pubKey, privKey := glow.GenerateKeyPair()
		equipment[i] = EquipmentAuthorization{ShortID: uint32(i)}
		equipment[i].PublicKey = pubKey
		privKeys[i] = privKey
	}

	// Load the equipment
	for _, e := range equipment {
		server.loadEquipmentAuth(e)
	}

	// Run some reports on each piece of equipment.
	for i, e := range equipment {
		er := EquipmentReport{
			ShortID:     e.ShortID,
			Timeslot:    uint32(i * 10),
			PowerOutput: uint64(i * 100),
		}
		sb := er.SigningBytes()
		er.Signature = glow.Sign(sb, privKeys[i])
		isValid := glow.Verify(e.PublicKey, sb, er.Signature)
		if !isValid {
			t.Fatal("Can't even verify my own signature")
		}

		report, err := server.parseReport(er.Serialize())
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
			er.Signature = glow.Sign(er.Serialize(), privKeys[i+1])
			_, err = server.parseReport(er.Serialize())
			if err == nil || err.Error() != "failed to verify signature" {
				t.Errorf("Expected signature verification failed error for wrong device signature, got: %v", err)
			}
		}
	}

	// Test with an invalid signature.
	invalidSignature := make([]byte, 65) // Just an example of an invalid signature
	blankReport := make([]byte, 16)
	fullReportInvalidSignature := append(blankReport, invalidSignature[:64]...)
	_, err = server.parseReport(fullReportInvalidSignature)
	if err == nil || err.Error() != "failed to verify signature" {
		t.Errorf("Expected signature verification failed error, got: %v", err)
	}

	// Test with a device not in the server's list.
	reportData := make([]byte, 16) // make a blank report for a non-existent device
	binary.BigEndian.PutUint32(reportData[0:4], uint32(numEquipment+1))
	_, err = server.parseReport(append(reportData, invalidSignature[:64]...))
	if err == nil || err.Error() != fmt.Sprintf("unknown equipment ID: %d", numEquipment+1) {
		t.Errorf("Expected unknown device ID error, got: %v", err)
	}
}
