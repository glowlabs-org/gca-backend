package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
)

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

// integrateReport will take an equipment report and use it to update the live
// state of the server.
func (server *GCAServer) integrateReport(report EquipmentReport) {
	// Nothing to integrate if the report is too old.
	if report.Timeslot < server.equipmentReportsOffset {
		return
	}
	// Panic if the timeslot is too new.
	if report.Timeslot > server.equipmentReportsOffset + 4032 {
		panic("received a report that's too far in the future to integrate")
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

	// Add the report to the list of recent reports, and truncate the list
	// if it's too large.
	server.recentReports = append(server.recentReports, report)
	if len(server.recentReports) > maxRecentReports {
		halfIndex := len(server.recentReports) / 2
		copy(server.recentReports[:], server.recentReports[halfIndex:])
		server.recentReports = server.recentReports[:halfIndex]
	}
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
		server.logger.Warn("Received report with a sentinel power output")
		return
	}

	// Integrate and save the report.
	server.integrateReport(report)
	server.saveEquipmentReport(report)
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
