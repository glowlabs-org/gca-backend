package main

import (
	"context"
	"crypto/ed25519"
	"encoding/binary"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// EquipmentReport outlines the schema for a report received from a piece of equipment.
type EquipmentReport struct {
	ShortID     uint32   // A unique identifier for the equipment
	Timeslot    uint32   // A field denoting the time of the report
	PowerOutput uint64   // The power output from the equipment
	Signature   [64]byte // A digital signature for the report's authenticity
}

// GCAServer defines the structure for our Grid Control Authority Server.
type GCAServer struct {
	baseDir       string                       // Base directory for server files
	deviceKeys    map[uint32]ed25519.PublicKey // Mapping from device ID to its public key
	recentReports []EquipmentReport            // Storage for recently received equipment reports
	gcaPubkey     ed25519.PublicKey            // Public key of the Grid Control Authority
	logger        *Logger                      // Custom logger for the server
	httpServer    *http.Server                 // Web server for handling API requests
	mux           *http.ServeMux               // Routing for HTTP requests
	conn          *net.UDPConn                 // UDP connection for listening to equipment reports
	quit          chan bool                    // A channel to initiate server shutdown
}

// NewGCAServer initializes a new instance of GCAServer and sets it up.
//
// baseDir specifies the directory where all server files will be stored.
// The function will create this directory if it does not exist.
func NewGCAServer(baseDir string) *GCAServer {
	// Create the directory if it doesn't exist
	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		os.MkdirAll(baseDir, 0755)
	}

	// Initialize the HTTP routing and logging functionalities
	mux := http.NewServeMux()
	loggerPath := filepath.Join(baseDir, "server.log")
	logger, err := NewLogger(INFO, loggerPath)
	if err != nil {
		logger.Fatal("Logger initialization failed: ", err)
	}

	// Populate GCAServer fields
	server := &GCAServer{
		baseDir:       baseDir,
		deviceKeys:    make(map[uint32]ed25519.PublicKey),
		recentReports: make([]EquipmentReport, 0, maxRecentReports),
		logger:        logger,
		mux:           mux,
		httpServer: &http.Server{
			Addr:    httpPort,
			Handler: mux,
		},
		quit: make(chan bool),
	}

	if err := server.loadGCAPubkey(); err != nil {
		logger.Fatal("Failed to load GCA public key: ", err)
	}

	// Load device public keys
	// This is just a placeholder, you might want to read the keys from a file
	devices := make([]Device, 0)
	server.loadDeviceKeys(devices)

	// Start the UDP and HTTP servers
	go server.launchUDPServer()
	server.launchAPI()
	
	// Hang on for a bit to let everything load.
	time.Sleep(time.Millisecond * 100)

	return server
}

// Close conducts an orderly shutdown of the server.
func (server *GCAServer) Close() {
	// Signal the server to terminate
	close(server.quit)

	// Gracefully terminate the HTTP server
	if server.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.httpServer.Shutdown(ctx)
	}

	// Close the UDP connection
	if server.conn != nil {
		server.conn.Close()
	}

	// Finally, close the logger
	server.logger.Close()
}

// loadGCAPubkey sets the public key for the Grid Control Authority.
// Previously, this function took the public key as an argument. Now it reads
// the public key from a file located in the server's base directory.
func (server *GCAServer) loadGCAPubkey() error {
	// Construct the path to the public key file
	pubkeyPath := filepath.Join(server.baseDir, "gca.pubkey")

	// Read the public key from the file
	pubkeyData, err := ioutil.ReadFile(pubkeyPath)
	if err != nil {
		// If an error occurs, return it to the caller
		return fmt.Errorf("unable to read public key from file: %v", err)
	}

	// Update the public key in the GCAServer struct
	server.gcaPubkey = ed25519.PublicKey(pubkeyData)
	return nil
}

// parseReport translates raw bytes into an EquipmentReport and validates its signature.
func (server *GCAServer) parseReport(rawData []byte) (EquipmentReport, error) {
	// Create a new EquipmentReport to populate
	var report EquipmentReport

	// Ensure the received data is of the expected length
	if len(rawData) != 80 {
		return report, fmt.Errorf("unexpected data length: expected 80 bytes, got %d bytes", len(rawData))
	}

	// Correctly read the data into the EquipmentReport
	report.ShortID = binary.BigEndian.Uint32(rawData[0:4])
	report.Timeslot = binary.BigEndian.Uint32(rawData[4:8])
	report.PowerOutput = binary.BigEndian.Uint64(rawData[8:16])
	copy(report.Signature[:], rawData[16:80])

	// Validate the ShortID and the Signature
	pubKey, ok := server.deviceKeys[report.ShortID]
	if !ok {
		return report, fmt.Errorf("unknown device ID: %d", report.ShortID)
	}

	if !ed25519.Verify(pubKey, rawData[:16], report.Signature[:]) {
		return report, errors.New("failed to verify signature")
	}

	return report, nil
}

// handleEquipmentReport interprets the raw data received from equipment.
func (server *GCAServer) handleEquipmentReport(rawData []byte) {
	// Translate the raw data into an EquipmentReport
	report, err := server.parseReport(rawData)
	if err != nil {
		server.logger.Error("Report decoding failed: ", err)
		return
	}

	// Append the report to recentReports
	server.recentReports = append(server.recentReports, report)

	// If the report log has grown too large, trim it
	if len(server.recentReports) > maxRecentReports {
		halfIndex := len(server.recentReports) / 2
		copy(server.recentReports[:], server.recentReports[halfIndex:])
		server.recentReports = server.recentReports[:halfIndex]
	}
}

// launchUDPServer sets up and begins the UDP server for the GCAServer.
func (server *GCAServer) launchUDPServer() {
	// Define the UDP server configuration
	udpAddress := net.UDPAddr{
		Port: udpPort,
		IP:   net.ParseIP(serverIP),
	}

	// Open the UDP connection
	var err error
	server.conn, err = net.ListenUDP("udp", &udpAddress)
	if err != nil {
		server.logger.Fatal("UDP server launch failed: ", err)
	}
	defer server.conn.Close()

	// Initialize a buffer for incoming data
	buffer := make([]byte, equipmentReportSize)

	// Main loop to listen for incoming UDP packets
	for {
		select {
		case <-server.quit:
			return
		default:
			// Wait for the next packet
		}

		// Read from the UDP socket
		readBytes, _, err := server.conn.ReadFromUDP(buffer)
		if err != nil {
			server.logger.Error("UDP read error: ", err)
			continue
		}

		// Make sure the packet is the expected size
		if readBytes != equipmentReportSize {
			server.logger.Warn("Received unexpected data size")
			continue
		}

		// Process the incoming packet
		go server.handleEquipmentReport(buffer[:readBytes])
	}
}
