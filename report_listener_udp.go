package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
)

// EquipmentReport defines the structure for a report received from a piece of equipment.
type EquipmentReport struct {
	ShortID     uint32 // A unique identifier for the equipment
	Timeslot    uint32 // A field denoting the time of the report
	PowerOutput uint64 // The power output from the equipment
	Signature   [64]byte // A digital signature for the report's authenticity
}

// SigningBytes returns the bytes that should be signed when sending an
// equipment report.
func (er EquipmentReport) SigningBytes() []byte {
	prefix := []byte("EquipmentReport")
	bytes := make([]byte, len(prefix)+16)
	copy(bytes, prefix)
	binary.BigEndian.PutUint32(bytes[15:], er.ShortID)
	binary.BigEndian.PutUint32(bytes[19:], er.Timeslot)
	binary.BigEndian.PutUint64(bytes[23:], er.PowerOutput)
	return bytes
}

func (er EquipmentReport) Serialize() []byte {
	bytes := make([]byte, 80)
	binary.BigEndian.PutUint32(bytes[0:], er.ShortID)
	binary.BigEndian.PutUint32(bytes[4:], er.Timeslot)
	binary.BigEndian.PutUint64(bytes[8:], er.PowerOutput)
	copy(bytes[16:], er.Signature[:])
	return bytes
}

// parseReport converts raw bytes into an EquipmentReport and validates its signature.
// This function assumes the server object has a map called 'equipment' which maps
// equipment ShortIDs to a struct containing their ECDSA public keys.
func (server *GCAServer) parseReport(rawData []byte) (EquipmentReport, error) {
	var report EquipmentReport
	if len(rawData) != 80 {
		return report, fmt.Errorf("unexpected data length: expected 80 bytes, got %d bytes", len(rawData))
	}

	// Populate the EquipmentReport fields
	report.ShortID = binary.BigEndian.Uint32(rawData[0:4])
	report.Timeslot = binary.BigEndian.Uint32(rawData[4:8])
	report.PowerOutput = binary.BigEndian.Uint64(rawData[8:16])
	copy(report.Signature[:], rawData[16:])

	// Validate the signature and the ShortID
	equipment, ok := server.equipment[report.ShortID]
	if !ok {
		return report, fmt.Errorf("unknown equipment ID: %d", report.ShortID)
	}

	// Hash the data and then verify the signature.
	sb := report.SigningBytes()
	if !Verify(equipment.PublicKey, sb, report.Signature) {
		return report, errors.New("failed to verify signature")
	}

	return report, nil
}

// handleEquipmentReport processes the raw data received from equipment.
func (server *GCAServer) handleEquipmentReport(rawData []byte) {
	// Parse the raw data into an EquipmentReport
	report, err := server.parseReport(rawData)
	if err != nil {
		server.logger.Error("Report decoding failed: ", err)
		return
	}

	// Append the report to recentReports
	server.recentReports = append(server.recentReports, report)

	// Truncate the recentReports slice if it gets too large
	if len(server.recentReports) > maxRecentReports {
		halfIndex := len(server.recentReports) / 2
		copy(server.recentReports[:], server.recentReports[halfIndex:])
		server.recentReports = server.recentReports[:halfIndex]
	}
}

// launchUDPServer sets up and starts the UDP server for listening to equipment reports.
func (server *GCAServer) launchUDPServer() {
	udpAddress := net.UDPAddr{
		Port: udpPort,
		IP:   net.ParseIP(serverIP),
	}

	var err error
	server.conn, err = net.ListenUDP("udp", &udpAddress)
	if err != nil {
		server.logger.Fatal("UDP server launch failed: ", err)
	}
	defer server.conn.Close()

	// Initialize the buffer to hold incoming data
	buffer := make([]byte, equipmentReportSize)

	// Continuously listen for incoming UDP packets
	for {
		select {
		case <-server.quit:
			return
		default:
			// Wait for the next incoming packet
		}

		// Read from the UDP socket
		readBytes, _, err := server.conn.ReadFromUDP(buffer)
		if err != nil {
			server.logger.Error("Failed to read from UDP socket: ", err)
			continue
		}

		// Process the received packet if it has the correct length
		if readBytes != equipmentReportSize {
			server.logger.Warn("Received an incorrectly sized packet: expected ", equipmentReportSize, " bytes, got ", readBytes, " bytes")
			continue
		}
		go server.handleEquipmentReport(buffer)
	}
}
