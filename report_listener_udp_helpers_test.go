package main

import (
	"crypto/ed25519"
	"encoding/binary"
	"fmt"
	"net"
)

// generateTestKeys generates an ed25519 public-private key pair.
// This is used to simulate equipment in our tests.
func generateTestKeys() (ed25519.PublicKey, ed25519.PrivateKey) {
	pubKey, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		panic(err)
	}
	return pubKey, privKey
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
