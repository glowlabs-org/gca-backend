package main

import (
	"fmt"
	"net"
)

// generateTestReport creates a mock report for testing purposes.
// The report includes a signature based on the provided private key.
func generateTestReport(shortID uint32, timeslot uint32, privKey PrivateKey) []byte {
	er := EquipmentReport{
		ShortID:     shortID,
		Timeslot:    timeslot,
		PowerOutput: 5,
	}
	er.Signature = Sign(er.SigningBytes(), privKey)
	return er.Serialize()
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
