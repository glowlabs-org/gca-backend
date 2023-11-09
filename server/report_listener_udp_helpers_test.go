package server

// This file contains helpers related to testing the UDP reports.

import (
	"fmt"
	"net"

	"github.com/glowlabs-org/gca-backend/glow"
)

// generateTestReport creates a mock report for testing purposes.
// The report includes a signature based on the provided private key.
func generateTestReport(shortID uint32, timeslot uint32, privKey glow.PrivateKey) []byte {
	er := EquipmentReport{
		ShortID:     shortID,
		Timeslot:    timeslot,
		PowerOutput: 5,
	}
	er.Signature = glow.Sign(er.SigningBytes(), privKey)
	return er.Serialize()
}

// sendUDPReport simulates sending a report to the server via UDP.
// The server should be listening on the given IP and port.
func sendUDPReport(report []byte, port uint16) error {
	conn, err := net.Dial("udp", fmt.Sprintf("%s:%d", serverIP, int(port)))
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = conn.Write(report)
	return err
}
