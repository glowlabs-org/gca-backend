package server

// This file creates an API endpoint that monitoring equipment can hit over
// UDP. The purpose of the provided API is to collect reports from the
// monitoring equipment. UDP is used as the protocol because it is the most
// lightweight, and reports are going to be sent over IoT networks, which means
// that they need to be very light.
//
// Reports will *always* be just one packet, so we don't have to worry about
// packet fragmentation when rebuilding the UDP stream. We do have to worry
// about dropped packets, this is handled by a separate TCP endpoint that
// communicates to the IoT device which reports have been received vs not
// received for the equipment. The equipment can then resend anything that
// didn't successfully transfer across the network.
//
// This combination should keep long term bandwidth requirements low while
// still preserving long term reliability.

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"net"

	"github.com/glowlabs-org/gca-backend/glow"
)

// parseReport converts raw bytes into an EquipmentReport and validates its signature.
// This function assumes the server object has a map called 'equipment' which maps
// equipment ShortIDs to a struct containing their ECDSA public keys.
func (server *GCAServer) parseReport(rawData []byte) (glow.EquipmentReport, error) {
	var report glow.EquipmentReport
	if len(rawData) != 80 {
		return report, fmt.Errorf("unexpected data length: expected 80 bytes, got %d bytes", len(rawData))
	}

	// Populate the EquipmentReport fields
	report.ShortID = binary.LittleEndian.Uint32(rawData[0:4])
	report.Timeslot = binary.LittleEndian.Uint32(rawData[4:8])
	report.PowerOutput = binary.LittleEndian.Uint64(rawData[8:16])
	copy(report.Signature[:], rawData[16:])

	// Validate the signature and the ShortID
	equipment, ok := server.equipment[report.ShortID]
	if !ok {
		return report, fmt.Errorf("unknown equipment ID: %d", report.ShortID)
	}

	// Hash the data and then verify the signature.
	sb := report.SigningBytes()
	if !glow.Verify(equipment.PublicKey, sb, report.Signature) {
		return report, errors.New("failed to verify signature")
	}

	return report, nil
}

// integrateReport will take an equipment report and use it to update the live
// state of the server.
func (server *GCAServer) integrateReport(report glow.EquipmentReport) {
	// Nothing to integrate if the report is too old.
	if report.Timeslot < server.equipmentReportsOffset {
		return
	}
	// Panic if the timeslot is too new.
	if report.Timeslot > server.equipmentReportsOffset+4032 {
		server.logger.Warn("Received report that's too far in the future to integrate")
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
		server.logger.Warn("Received second report for timeslot")
		server.equipmentReports[report.ShortID][report.Timeslot-server.equipmentReportsOffset].PowerOutput = 1
	}
	// Ban the report timeslot if the production is greater than the
	// capacity. Reports declare negative numbers by underflowing the
	// uint64, so we have to check with a typecast whether the report has
	// been underflowed. We don't ban underflowed reports.
	if report.PowerOutput > server.equipment[report.ShortID].Capacity*MaxCapacityBuffer/100 && report.PowerOutput < math.MaxInt64 {
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
	server.saveEquipmentReport(report)
}

// managedHandleEquipmentReport processes the raw data received from equipment.
func (server *GCAServer) managedHandleEquipmentReport(rawData []byte) {
	server.mu.Lock()
	defer server.mu.Unlock()

	// We could do a check here to verify that the GCA pubkey has been
	// provided to the server, but if no GCA key has been provided, the
	// server should not have any authorized equipment on it anyway, and
	// therefore parseReport should fail.

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
	now := glow.CurrentTimeslot()
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
}

// launchUDPServer will create a udp server that listens for reports from
// clients.
func (server *GCAServer) launchUDPServer() {
	// Create the udpConn
	udpAddress := net.UDPAddr{
		Port: udpPort,
		IP:   net.ParseIP(serverIP),
	}
	udpConn, err := net.ListenUDP("udp", &udpAddress)
	addr, ok := udpConn.LocalAddr().(*net.UDPAddr)
	if !ok {
		panic("bad type on udpConn")
	}
	server.udpPort = uint16(addr.Port)
	if err != nil {
		server.logger.Fatal("UDP server launch failed: ", err)
	}
	server.logger.Infof("UDP server launched on port %v", server.udpPort)
	server.tg.OnStop(func() error {
		return udpConn.Close()
	})

	server.tg.Launch(func() {
		server.threadedListenUDP(udpConn)
	})
}

// threadedListenUDP manages the infinite loop that listens for UDP packets.
func (server *GCAServer) threadedListenUDP(udpConn *net.UDPConn) {
	for {
		// Check whether the server has been stopped.
		if server.tg.IsStopped() {
			return
		}

		// Read from the UDP socket
		buffer := make([]byte, equipmentReportSize)
		readBytes, _, err := udpConn.ReadFromUDP(buffer)
		if err != nil {
			server.logger.Error("Failed to read from UDP socket: ", err)
			continue
		}

		// Process the received packet if it has the correct length
		if readBytes != equipmentReportSize {
			server.logger.Warn("Received an incorrectly sized packet: expected ", equipmentReportSize, " bytes, got ", readBytes, " bytes")
			continue
		}
		server.tg.Launch(func() {
			server.managedHandleEquipmentReport(buffer)
		})
	}
}
