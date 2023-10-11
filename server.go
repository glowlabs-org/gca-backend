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

// EquipmentReport defines the structure for a report received from a piece of equipment.
type EquipmentReport struct {
	ShortID     uint32   // A unique identifier for the equipment
	Timeslot    uint32   // A field denoting the time of the report
	PowerOutput uint64   // The power output from the equipment
	Signature   [64]byte // A digital signature for the report's authenticity
}

// GCAServer defines the structure for our Grid Control Authority Server.
type GCAServer struct {
	baseDir       string                       // Base directory for server files
	equipmentKeys map[uint32]ed25519.PublicKey // Mapping from equipment ID to its public key
	recentReports []EquipmentReport            // Storage for recently received equipment reports
	gcaPubkey     ed25519.PublicKey            // Public key of the Grid Control Authority
	logger        *Logger                      // Custom logger for the server
	httpServer    *http.Server                 // Web server for handling API requests
	mux           *http.ServeMux               // Routing for HTTP requests
	conn          *net.UDPConn                 // UDP connection for listening to equipment reports
	quit          chan bool                    // A channel to initiate server shutdown
}

// NewGCAServer initializes a new instance of GCAServer.
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
	logger, err := NewLogger(defaultLogLevel, loggerPath)
	if err != nil {
		logger.Fatal("Logger initialization failed: ", err)
	}

	// Initialize GCAServer with the proper fields
	server := &GCAServer{
		baseDir:       baseDir,
		equipmentKeys: make(map[uint32]ed25519.PublicKey),
		recentReports: make([]EquipmentReport, 0, maxRecentReports),
		logger:        logger,
		mux:           mux,
		httpServer: &http.Server{
			Addr:    httpPort,
			Handler: mux,
		},
		quit: make(chan bool),
	}

	// Load the Grid Control Authority public key
	if err := server.loadGCAPubkey(); err != nil {
		logger.Fatal("Failed to load GCA public key: ", err)
	}

	// Load equipment public keys (Note: loadEquipmentKeys is assumed to exist elsewhere)
	equipments := make([]Equipment, 0)
	server.loadEquipmentKeys(equipments)

	// Start the UDP and HTTP servers
	go server.launchUDPServer()
	server.launchAPI()

	// Wait for a short duration to let everything load
	time.Sleep(time.Millisecond * 100)

	return server
}

// Close cleanly shuts down the GCAServer instance.
func (server *GCAServer) Close() {
	// Signal to terminate the server
	close(server.quit)

	// Shutdown the HTTP server gracefully
	if server.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.httpServer.Shutdown(ctx)
	}

	// Close the UDP connection
	if server.conn != nil {
		server.conn.Close()
	}

	// Close the logger
	server.logger.Close()
}

// loadGCAPubkey loads the Grid Control Authority public key from a file.
func (server *GCAServer) loadGCAPubkey() error {
	pubkeyPath := filepath.Join(server.baseDir, "gca.pubkey")
	pubkeyData, err := ioutil.ReadFile(pubkeyPath)
	if err != nil {
		return fmt.Errorf("unable to read public key from file: %v", err)
	}
	server.gcaPubkey = ed25519.PublicKey(pubkeyData)
	return nil
}

// parseReport converts raw bytes into an EquipmentReport and validates its signature.
func (server *GCAServer) parseReport(rawData []byte) (EquipmentReport, error) {
	var report EquipmentReport

	// Check for the correct data length
	if len(rawData) != 80 {
		return report, fmt.Errorf("unexpected data length: expected 80 bytes, got %d bytes", len(rawData))
	}

	// Populate the EquipmentReport fields
	report.ShortID = binary.BigEndian.Uint32(rawData[0:4])
	report.Timeslot = binary.BigEndian.Uint32(rawData[4:8])
	report.PowerOutput = binary.BigEndian.Uint64(rawData[8:16])
	copy(report.Signature[:], rawData[16:80])

	// Validate the signature and the ShortID
	pubKey, ok := server.equipmentKeys[report.ShortID]
	if !ok {
		return report, fmt.Errorf("unknown equipment ID: %d", report.ShortID)
	}
	if !ed25519.Verify(pubKey, rawData[:16], report.Signature[:]) {
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
