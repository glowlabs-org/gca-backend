package main

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

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
