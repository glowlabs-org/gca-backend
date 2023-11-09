package server

// sync_listener_tcp.go creates a listener for TCP requests, specifically one
// type of request that allows a hardware device to see what reports have been
// received by the server.
//
// This is important because the hardware devices send reports over UDP, which
// does not guarantee that the reports will arrive. In the event that the
// report packets are dropped, the hardware device needs some way to detect
// that they were dropped and attempt to resubmit them. This file provides the
// synchronization data.
//
// The synchronization data itself takes the form of a bitfield, one bit per
// timeslot. A '0' indicates that the server does not have a report for that
// timeslot, and a '1' indicates that the server does have a report for that
// timeslot, allowing the hardware (which runs on an IoT mobile network) to see
// what reports are missing while using minimal bandwidth.
//
// This endpoint is expected to be called roughly every 4 hours by each
// hardware device.

import (
	"encoding/binary"
	"io"
	"net"
	"time"

	"github.com/glowlabs-org/gca-backend/glow"
)

// threadedListenForSyncRequests creates a TCP listener that will listen for
// queries that want to see which timeslots have reports for a given piece of
// hardware.
func (gcas *GCAServer) threadedListenForSyncRequests(tcpReady chan struct{}) {
	// Listen on TCP port
	listener, err := net.Listen("tcp", tcpPort)
	if err != nil {
		gcas.logger.Fatalf("Failed to start server: %s", err)
	}
	defer listener.Close()
	gcas.tcpPort = uint16(listener.Addr().(*net.TCPAddr).Port)
	close(tcpReady)

	for {
		// Check for a shutdown signal.
		select {
		case <-gcas.quit:
			return
		default:
			// Wait for the next incoming request
		}

		// Wait for a connection
		conn, err := listener.Accept()
		if err != nil {
			gcas.logger.Infof("Failed to accept connection: %s", err)
			continue
		}

		// Handle the connection in a new goroutine
		go gcas.managedHandleSyncConn(conn)
	}
}

// managedHandleSyncConn will handle the incoming tcp request. The incoming
// request is expected to have a 4 byte payload, representing the ShortID of
// the equipment that we want history from.
//
// If successful, the response will be:
//   - 32 bytes, contianing the public key of the equipment
//   - 4 bytes, containing the timeslot where the history starts
//   - 504 bytes, containing the bitfield exposing the missing history
//   - 8 bytes, containing a Unix timestamp for when the response was authorized
//   - 64 bytes, containing a signature from the GCA server asserting the authenticity of the data
//
// If unsuccessful, the response will be a single zero byte followed by the
// connection closing.
func (gcas *GCAServer) managedHandleSyncConn(conn net.Conn) {
	defer conn.Close()

	// Create a buffer to store incoming data
	buf := make([]byte, 4)
	_, err := io.ReadFull(conn, buf)
	if err != nil {
		gcas.logger.Infof("Unable to read request")
		return
	}

	// Read the ShortID from the request.
	id := binary.BigEndian.Uint32(buf)

	// Fetch the corresponding data.
	var bitfield [504]byte
	gcas.mu.Lock()
	reports, exists := gcas.equipmentReports[id]
	if exists {
		for i, report := range reports {
			byteIndex := i / 8
			bitIndex := i % 8
			if report.PowerOutput > 0 {
				bitfield[byteIndex] |= 1 << bitIndex
			}
		}
	}
	equipment, exists2 := gcas.equipment[id]
	reportsOffset := gcas.equipmentReportsOffset
	gcas.mu.Unlock()

	// If there is no hardware for the provided short id, write a zero byte
	// and close the connection.
	if !exists || !exists2 {
		var ded [1]byte
		conn.Write(ded[:])
		return
	}

	// Prepare the response.
	var resp [32 + 4 + 504 + 8 + 64]byte
	// Copy in the public key.
	copy(resp[:32], equipment.PublicKey[:])
	// Copy in the reports offset
	binary.BigEndian.PutUint32(resp[32:36], reportsOffset)
	// Copy in the bitfield.
	copy(resp[36:540], bitfield[:])
	// Copy in the unix timestamp
	timestamp := time.Now().Unix()
	binary.BigEndian.PutUint64(resp[540:548], uint64(timestamp))
	// Create the signature
	sig := glow.Sign(resp[:548], gcas.staticPrivateKey)
	copy(resp[548:], sig[:])
	_, err = conn.Write(resp[:])
	if err != nil {
		gcas.logger.Errorf("Failed to write response: %v", err)
		return
	}
	return
}
