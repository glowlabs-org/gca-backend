package main

// TODO: Need to implement and test logic for dealing with duplicated reports.

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
)

// EquipmentReport defines the structure for a report received from a piece of equipment.
type EquipmentReport struct {
	ShortID     uint32    // A unique identifier for the equipment
	Timeslot    uint32    // A field denoting the time of the report
	PowerOutput uint64    // The power output from the equipment
	Signature   Signature // A digital signature for the report's authenticity
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

// Serialize creates a compact binary representation of the data structure.
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

// managedHandleEquipmentReport processes the raw data received from equipment.
func (server *GCAServer) managedHandleEquipmentReport(rawData []byte) {
	server.mu.Lock()
	defer server.mu.Unlock()

	// Parse the raw data into an EquipmentReport
	report, err := server.parseReport(rawData)
	if err != nil {
		server.logger.Error("Report decoding failed: ", err)
		return
	}

	// Verify that the timeslot of the report is acceptable. This means
	// that the report must be within 432 timeslots of the current
	// timeslot, which is 72 hours.
	//
	// When doing the comparison, we cast everything to int64 to handle
	// potential overflows and underflows.
	now := currentTimeslot()
	if int64(report.Timeslot) < int64(now)-432 || int64(report.Timeslot) > int64(now)+432 {
		server.logger.Warn("Received out of bounds timeslot: ", now, " ", report.Timeslot)
		return
	}
	// Reports that don't have any power generated are ignored. A power of
	// '1' is effectively 0, and we use the '1' value to signal that a
	// report has been banned for duplicate attempts.
	if report.PowerOutput == 0 || report.PowerOutput == 1 {
		server.logger.Warn("Received report with 0 power output")
		return
	}

	// Check whether we've seen a duplicate of this report before.
	// Timeslots that have already been banned get ignored.
	if server.equipmentReports[report.ShortID][report.Timeslot-server.equipmentReportsOffset].PowerOutput == 1 {
		server.logger.Warn("Received report for banned timeslot")
		return
	}
	// Duplicate reports for a timeslot get ignored, assuming the reports
	// are exactly identical.
	if server.equipmentReports[report.ShortID][report.Timeslot-server.equipmentReportsOffset] == report {
		server.logger.Warn("Received duplicate report")
		return
	}
	// If there are no reports yet for the timeslot, put this report in.
	// Otherwise ban this timeslot. We set PowerOutput to 1 to indicate
	// that the timeslot is banned. We will need to remember that when
	// feeding reports out of the API, we will have to replace those
	// timeslots with blank reports.
	//
	// Whether or not a ban is happening, we need to record this report. If
	// this report gets banned, we need to save this particular report so
	// that we can provide proof to everyone else that the ban is
	// justified, therefore we let the function continue in both cases.
	if server.equipmentReports[report.ShortID][report.Timeslot-server.equipmentReportsOffset].PowerOutput == 0 {
		server.equipmentReports[report.ShortID][report.Timeslot-server.equipmentReportsOffset] = report
	} else {
		server.logger.Warn("Received second support for timeslot")
		server.equipmentReports[report.ShortID][report.Timeslot-server.equipmentReportsOffset].PowerOutput = 1
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

// threadedLaunchUDPServer sets up and starts the UDP server for listening to
// equipment reports.
func (server *GCAServer) threadedLaunchUDPServer() {
	udpAddress := net.UDPAddr{
		Port: udpPort,
		IP:   net.ParseIP(serverIP),
	}

	var err error
	server.mu.Lock()
	server.conn, err = net.ListenUDP("udp", &udpAddress)
	server.mu.Unlock()
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
		go server.managedHandleEquipmentReport(buffer)
	}
}
