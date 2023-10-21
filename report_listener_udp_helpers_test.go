package main

import (
	"crypto/ecdsa"
	"encoding/binary"
	"fmt"
	"net"

	"github.com/ethereum/go-ethereum/crypto"
)

// generateTestKeys generates an ecdsa public-private key pair.
// This is used to simulate equipment in our tests.
func generateTestKeys() (*ecdsa.PublicKey, *ecdsa.PrivateKey) {
	privKey, err := crypto.GenerateKey()
	if err != nil {
		panic(err)
	}
	return &privKey.PublicKey, privKey
}

// generateTestReport creates a mock report for testing purposes.
// The report includes a signature based on the provided private key.
func generateTestReport(shortID uint32, timeslot uint32, privKey *ecdsa.PrivateKey) []byte {
	data := make([]byte, 80)
	binary.BigEndian.PutUint32(data[0:4], shortID)
	binary.BigEndian.PutUint32(data[4:8], timeslot)
	// PowerOutput can remain as zero since they don't impact the behavior we're testing.

	// Sign the data using the private key and insert the signature into the report
	hash := crypto.Keccak256Hash(data[:16]).Bytes()
	signature, err := crypto.Sign(hash, privKey)
	if err != nil {
		panic(err)
	}
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
