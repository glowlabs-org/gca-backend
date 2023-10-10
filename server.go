package main

import (
	"context"
	"crypto/ed25519"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
)

// EquipmentReport defines the structure of the report received from equipment.
type EquipmentReport struct {
	ShortID     uint32   // Unique identifier for the device
	Timeslot    uint32   // Timestamp or other time-related field
	PowerOutput uint64   // Measured power output
	Signature   [64]byte // Digital signature to verify the report
}

// GCAServer is the main server structure.
type GCAServer struct {
	deviceKeys    map[uint32]ed25519.PublicKey // Map of device keys, indexed by ShortID
	recentReports []EquipmentReport            // Recently received equipment reports
	gcaPubkey     ed25519.PublicKey            // Public key of the GCA (Grid Control Authority)
	logger        *Logger                      // Custom logger
	httpServer    *http.Server                 // HTTP server for any web API
	mux           *http.ServeMux               // HTTP request multiplexer
	conn          *net.UDPConn                 // UDP connection for receiving equipment reports
	quit          chan bool                    // Channel to signal server to quit
}

// NewGCAServer creates and initializes a new GCAServer instance and loads device keys.
func NewGCAServer() *GCAServer {
	// Initialize HTTP request multiplexer and logger
	mux := http.NewServeMux()
	logger, err := NewLogger(INFO, "server.log")
	if err != nil {
		logger.Fatal("Failed to initialize logger:", err)
		os.Exit(1)
	}

	// Initialize the GCAServer struct
	server := &GCAServer{
		deviceKeys:    make(map[uint32]ed25519.PublicKey),
		recentReports: make([]EquipmentReport, 0, maxRecentReports),
		logger:        logger,
		mux:           mux,
		httpServer: &http.Server{
			Addr:    httpPort,
			Handler: mux, // Set the handler to the custom multiplexer
		},
		quit: make(chan bool),
	}

	// Load device keys (assuming devices is a slice of Device objects)
	devices := make([]Device, 0)
	server.loadDeviceKeys(devices)

	// Start UDP and HTTP servers
	go server.startUDPServer()
	server.startAPI()

	return server
}

// Close shuts down the server gracefully.
func (gca *GCAServer) Close() {
	close(gca.quit) // Signal the quit channel

	// Shutdown HTTP Server
	if gca.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		gca.httpServer.Shutdown(ctx)
	}

	// Wait for the listening loop to exit before closing the connection
	<-time.After(100 * time.Millisecond) // A small delay to allow the loop to exit. Adjust as necessary
	if gca.conn != nil {
		gca.conn.Close()
	}

	// Close the logger last so that we can log all the shutdown steps
	gca.logger.Close()
}

// LoadGCAPubkey loads the GCA's public key into the server.
func (gca *GCAServer) LoadGCAPubkey(pubkey ed25519.PublicKey) {
	gca.gcaPubkey = pubkey
}

// parseReport decodes raw data into an EquipmentReport and verifies its signature.
func (gca *GCAServer) parseReport(data []byte) (*EquipmentReport, error) {
	// Initialize a new EquipmentReport
	report := &EquipmentReport{}

	// Validate the length of the received data
	if len(data) != 80 { // 80 bytes is the expected size of the data
		return nil, fmt.Errorf("unexpected data length: got %d bytes, expected 80 bytes", len(data))
	}

	// Parsing received data into the EquipmentReport
	report.ShortID = binary.BigEndian.Uint32(data[0:4])      // First 4 bytes for ShortID
	report.Timeslot = binary.BigEndian.Uint32(data[4:8])     // Next 4 bytes for Timeslot
	report.PowerOutput = binary.BigEndian.Uint64(data[8:16]) // Next 8 bytes for PowerOutput
	copy(report.Signature[:], data[16:80])                   // Final 64 bytes for Signature

	// Validate the ShortID and Signature
	pubKey, ok := gca.deviceKeys[report.ShortID]
	if !ok {
		return nil, fmt.Errorf("unknown device ID: %d", report.ShortID)
	}

	if !ed25519.Verify(pubKey, data[:16], report.Signature[:]) {
		return nil, errors.New("signature verification failed")
	}

	return report, nil
}

// handleEquipmentReport processes the received raw data.
func (gca *GCAServer) handleEquipmentReport(data []byte) {
	// Parse the report from the raw data
	report, err := gca.parseReport(data)
	if err != nil {
		gca.logger.Error("Failed to process report: ", err)
		return
	}

	// Append the report to recentReports
	gca.recentReports = append(gca.recentReports, *report)

	// If the length of recentReports exceeds maxRecentReports, truncate the list
	if len(gca.recentReports) > maxRecentReports {
		halfIndex := len(gca.recentReports) / 2
		copy(gca.recentReports[:], gca.recentReports[halfIndex:])
		gca.recentReports = gca.recentReports[:halfIndex]
	}
}

// startUDPServer initializes and starts the UDP server.
func (gca *GCAServer) startUDPServer() {
	// Setup UDP server settings
	addr := net.UDPAddr{
		Port: port,
		IP:   net.ParseIP(ip),
	}

	// Initialize UDP connection
	var err error
	gca.conn, err = net.ListenUDP("udp", &addr)
	if err != nil {
		gca.logger.Fatal("Error starting UDP server: ", err)
		os.Exit(1)
	}
	defer gca.conn.Close()

	gca.logger.Info("Listening on UDP ", ip, ":", port)

	// Buffer to hold incoming data
	buffer := make([]byte, equipmentReportSize)

	// Main UDP listening loop
	for {
		select {
		case <-gca.quit:
			return
		default:
			// Continue as normal
		}

		// Read from UDP connection
		n, _, err := gca.conn.ReadFromUDP(buffer)
		if err != nil {
			gca.logger.Error("Error reading from UDP connection: ", err)
			continue
		}

		// Validate the length of the received data
		if n != equipmentReportSize {
			gca.logger.Warn("Received message of invalid length")
			continue
		}

		// Process the received data
		go gca.handleEquipmentReport(buffer[:n])
	}
}
