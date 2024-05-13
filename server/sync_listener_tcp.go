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
	"strconv"
	"time"

	"github.com/glowlabs-org/gca-backend/glow"
)

// launchListenForSyncRequests creates a TCP listener that will listen for
// queries that want to see which timeslots have reports for a given piece of
// hardware.
func (gcas *GCAServer) launchListenForSyncRequests() {
	// Listen on TCP port
	listener, err := net.Listen("tcp", serverIP+":"+strconv.Itoa(tcpPort))
	if err != nil {
		gcas.logger.Fatalf("Failed to create tcp listener: %s", err)
	}
	gcas.tcpPort = uint16(listener.Addr().(*net.TCPAddr).Port)
	gcas.tg.OnStop(func() error {
		return listener.Close()
	})

	gcas.tg.Launch(func() {
		gcas.threadedListenForSyncRequests(listener)
	})
}

// Background thread to listen on TCP for sync requests.
func (gcas *GCAServer) threadedListenForSyncRequests(listener net.Listener) {
	for {
		// Check whether the gcas has been stopped.
		if gcas.tg.IsStopped() {
			return
		}

		// Wait for a connection
		conn, err := listener.Accept()
		if err != nil {
			// No need to log an error if the error is because of
			// shutdown.
			if !gcas.tg.IsStopped() {
				gcas.logger.Infof("Failed to accept connection: %s", err)
			}
			continue
		}

		// Handle the connection in a new goroutine
		err = gcas.tg.Launch(func() {
			gcas.managedHandleSyncConn(conn)
		})
		if err != nil {
			conn.Close()
			return
		}
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
	id := binary.LittleEndian.Uint32(buf)

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
	migration, migrationExists := gcas.equipmentMigrations[equipment.PublicKey]
	gcas.mu.Unlock()

	// If there is no hardware for the provided short id, write a zero byte
	// and close the connection.
	if !exists || !exists2 {
		var ded [1]byte
		conn.Write(ded[:])
		return
	}

	// Prepare the response. The first two bytes will be used as a length
	// prefix.
	resp := make([]byte, 578)
	// Copy in the public key.
	copy(resp[2:34], equipment.PublicKey[:])
	// Copy in the reports offset
	binary.LittleEndian.PutUint32(resp[34:38], reportsOffset)
	// Copy in the bitfield.
	copy(resp[38:542], bitfield[:])
	if migrationExists {
		// When copying in the migration, we can skip the first 32
		// bytes because it contains the equipment public key, which
		// already appears before the bitfield.
		mBytes := migration.Serialize()
		resp = resp[:542]
		resp = append(resp, mBytes[32:]...)
	} else {
		// Add the list of gcaServers.
		gcas.gcaServers.mu.Lock()
		for _, s := range gcas.gcaServers.servers {
			locationLen := len(s.Location)
			sBytes := make([]byte, 104+locationLen)
			copy(sBytes[:32], s.PublicKey[:])
			if s.Banned {
				sBytes[32] = 1
			}
			sBytes[33] = byte(locationLen)
			copy(sBytes[34:], []byte(s.Location))
			binary.LittleEndian.PutUint16(sBytes[34+locationLen:], s.HttpPort)
			binary.LittleEndian.PutUint16(sBytes[36+locationLen:], s.TcpPort)
			binary.LittleEndian.PutUint16(sBytes[38+locationLen:], s.UdpPort)
			copy(sBytes[40+locationLen:], s.GCAAuthorization[:])
			resp = append(resp, sBytes...)
		}
		// Copy in a blank GCA signature.
		var newGCASig glow.Signature
		resp = append(resp, newGCASig[:]...)
		gcas.gcaServers.mu.Unlock()
	}
	// Copy in the unix timestamp
	var timeBytes [8]byte
	timestamp := time.Now().Unix()
	binary.LittleEndian.PutUint64(timeBytes[:], uint64(timestamp))
	resp = append(resp, timeBytes[:]...)
	// Create the signature
	sig := glow.Sign(resp[2:], gcas.staticPrivateKey)
	resp = append(resp, sig[:]...)
	respLen := len(resp) - 2 // subtract 2 because the length prefix doesn't count
	binary.LittleEndian.PutUint16(resp[:2], uint16(respLen))
	_, err = conn.Write(resp)
	if err != nil {
		gcas.logger.Errorf("Failed to write response: %v", err)
		return
	}
	return
}
