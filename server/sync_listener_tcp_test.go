package server

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/glowlabs-org/gca-backend/glow"
)

// requestEquipmentBitfield opens a TCP connection, sends a request for a
// specific piece of equipment's bitfield using its ShortID, and then
// verifies the signature on the received data.
//
// It returns the timeslot offset and the bitfield. If the operation
// fails for any reason, it returns an error.
func (gcas *GCAServer) requestEquipmentBitfield(shortID uint32) (timeslotOffset uint32, bitfield [504]byte, err error) {
	// Dial the server
	conn, err := net.Dial("tcp", "localhost:"+fmt.Sprintf("%v", gcas.tcpPort))
	if err != nil {
		return 0, [504]byte{}, fmt.Errorf("unable to call net.Dail: %v", err)
	}
	defer conn.Close()

	// Prepare request payload
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], shortID)

	// Send the request
	_, err = conn.Write(buf[:])
	if err != nil {
		return 0, [504]byte{}, err
	}

	// Prepare a buffer to store the incoming response
	var respLenBytes [2]byte
	_, err = io.ReadFull(conn, respLenBytes[:])
	if err != nil {
		return 0, [504]byte{}, fmt.Errorf("unable to read response length: %v", err)
	}
	respLen := binary.BigEndian.Uint16(respLenBytes[:])
	resp := make([]byte, respLen)

	// Read the full response
	_, err = io.ReadFull(conn, resp)
	if err != nil {
		return 0, [504]byte{}, fmt.Errorf("unable to read full response: %v", err)
	}

	// Separate the signature from the data
	var sig glow.Signature
	copy(sig[:], resp[respLen-64:])

	// Extract the signing time.
	signingTime := binary.BigEndian.Uint64(resp[respLen-72:])
	now := uint64(time.Now().Unix())
	if now+24*3600 < signingTime || now-24*3600 > signingTime {
		return 0, [504]byte{}, fmt.Errorf("signature comes from an out of bounds time: %v", signingTime)
	}

	// Verify the signature
	if !glow.Verify(gcas.staticPublicKey, resp[:respLen-64], sig) {
		return 0, [504]byte{}, fmt.Errorf("Signature verification failed")
	}

	// Extract the timeslot offset
	timeslotOffset = binary.BigEndian.Uint32(resp[32:36])

	// Extract the bitfield
	copy(bitfield[:], resp[36:540])

	return timeslotOffset, bitfield, nil
}

// TestTCPListener does some basic testing to make sure that the TCP listener
// is returning the right values.
func TestTCPListener(t *testing.T) {
	gcas, _, _, gcaPrivKey, err := SetupTestEnvironment(t.Name())
	if err != nil {
		t.Fatal(err)
	}
	// This test is going to be messing with time, therefore defer a reset
	// of the time.
	defer func() {
		glow.SetCurrentTimeslot(0)
	}()

	// Generate a keypair for a device.
	authPub, authPriv := glow.GenerateKeyPair()
	auth := glow.EquipmentAuthorization{ShortID: 0, PublicKey: authPub}
	sb := auth.SigningBytes()
	sig := glow.Sign(sb, gcaPrivKey)
	auth.Signature = sig
	gcas.mu.Lock()
	_, err = gcas.saveEquipment(auth)
	gcas.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}

	// Submit reports for slots 0, 2, and 4.
	err = gcas.sendEquipmentReportSpecific(auth, authPriv, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	err = gcas.sendEquipmentReportSpecific(auth, authPriv, 2, 50)
	if err != nil {
		t.Fatal(err)
	}
	err = gcas.sendEquipmentReportSpecific(auth, authPriv, 4, 50)
	if err != nil {
		t.Fatal(err)
	}
	offset, bitfield, err := gcas.requestEquipmentBitfield(0)
	if err != nil {
		t.Fatal(err)
	}
	if offset != 0 {
		t.Fatal("bad")
	}
	if bitfield[0] != 21 {
		t.Fatal(bitfield[0])
	}
	for i := 1; i < len(bitfield); i++ {
		if bitfield[i] != 0 {
			t.Fatal("expecting 0")
		}
	}

	// Submit reports for slots 4031, 4030, and 4028. For these reports to
	// be accepted, time must be shifted. This will also trigger a report
	// migration.
	glow.SetCurrentTimeslot(4000)
	time.Sleep(150 * time.Millisecond) // Sleep so that report migrations happen.
	err = gcas.sendEquipmentReportSpecific(auth, authPriv, 4031, 50)
	if err != nil {
		t.Fatal(err)
	}
	err = gcas.sendEquipmentReportSpecific(auth, authPriv, 4030, 50)
	if err != nil {
		t.Fatal(err)
	}
	err = gcas.sendEquipmentReportSpecific(auth, authPriv, 4028, 50)
	if err != nil {
		t.Fatal(err)
	}
	offset, bitfield, err = gcas.requestEquipmentBitfield(0)
	if err != nil {
		t.Fatal(err)
	}
	if offset != 2016 {
		t.Fatal("bad")
	}
	if bitfield[503-252] != 128+64+16 {
		t.Error(bitfield[503-252])
	}
	for i := 0; i < len(bitfield); i++ {
		if bitfield[i] != 0 && i != 503-252 {
			t.Fatal("expecting 0")
		}
	}

	// Attempt to request the bitfield of an equipment that does not exit.
	_, _, err = gcas.requestEquipmentBitfield(541)
	if err == nil {
		t.Fatal("expecting error when reading non-existant bitfield")
	}
}
