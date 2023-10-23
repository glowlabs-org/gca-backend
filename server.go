package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// TODO: Need to change the function signature of NewGCAServer so that it
// returns an error.
//
// TODO: Need to write the endpoints for the GCA server handshake (where the
// GCA tells the server who it is) and also the auth page where the GCA server
// can show its recent authorization from the GCA.
//
// TODO: All existing HTTP GET endpoints need to have a signature attached to
// them so that the caller knows that the data is coming from an authorized GCA
// server.
//
// TODO: Existing endpoints need to fail gracefully if no GCA key has been
// loaded yet. This means writing tests around hitting each endpoint.
//
// TODO: Need to write a high concurrency test where all of the major APIs and
// functions of the server are blasting at once, that way we can detect race
// conditions.

// GCAServer defines the structure for our Grid Control Authority Server.
type GCAServer struct {
	equipment              map[uint32]EquipmentAuthorization // Map from a ShortID to the full equipment authorization
	equipmentBans          map[uint32]struct{}               // Tracks which equipment is banned
	equipmentReports       map[uint32]*[4032]EquipmentReport // Keeps all recent reports in memory
	equipmentReportsOffset uint32                            // What timeslot the equipmentReports arrays start at

	recentEquipmentAuths []EquipmentAuthorization // Keep recent auths to more easily synchronize with redundant servers
	recentReports        []EquipmentReport        // Keep recent reports to more easily synchronize with redundant servers

	// The GCA interacts with the server in two stages. The first stage
	// uses a temporary key which is created by the technicians before the
	// GCA lockbook is presented to the GCA. The actual GCA key is created
	// *after* the GCA configures their lockbook. We don't require GCAs to
	// be techincal, therefore so we don't expect them to be able to
	// configure their own server to upload their real pubkey. Instead, the
	// technician puts a temporary key on the lockbook and server, then
	// after the real key is generated, it can be signed by the temp key to
	// minimize the risk of a MiTM attack against the server.
	gcaPubkey          PublicKey
	gcaPubkeyAvailable bool
	gcaTempKey         PublicKey

	baseDir    string         // Base directory for server files
	logger     *Logger        // Custom logger for the server
	httpServer *http.Server   // Web server for handling API requests
	httpPort   string         // Records the port that is being used to serve the api
	mux        *http.ServeMux // Routing for HTTP requests
	udpConn    *net.UDPConn   // UDP connection for listening to equipment reports
	udpPort    int            // The port that the UDP conn is listening on
	quit       chan bool      // A channel to initiate server shutdown
	mu         sync.Mutex
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

	// Compute the timeslot that should be used as the reports offset. We
	// want it to be at 0:00:00 on Monday UTC. We want it to be less than 2
	// weeks in the past but more than 4 days in the past.
	now := currentTimeslot()
	closestWeek := uint32(0)
	for now-closestWeek > 3200 {
		closestWeek += 2016
	}

	// Initialize GCAServer with the proper fields
	server := &GCAServer{
		baseDir:                baseDir,
		equipment:              make(map[uint32]EquipmentAuthorization),
		equipmentBans:          make(map[uint32]struct{}),
		equipmentReports:       make(map[uint32]*[4032]EquipmentReport),
		equipmentReportsOffset: closestWeek,
		recentReports:          make([]EquipmentReport, 0, maxRecentReports),
		logger:                 logger,
		mux:                    mux,
		httpServer: &http.Server{
			Addr:    httpPort,
			Handler: mux,
		},
		quit: make(chan bool),
	}

	// Load the temporary Glow Certification Agent public key.
	if err := server.loadGCATempKey(); err != nil {
		logger.Fatal("Failed to load GCA public key: ", err)
	}
	// Load the Glow Certification Agent public key
	if err := server.loadGCAPubkey(); err != nil {
		logger.Fatal("Failed to load GCA public key: ", err)
	}
	// Load equipment public keys
	if err := server.loadEquipment(); err != nil {
		logger.Fatal("Failed to load server equipment: ", err)
	}
	// Load all equipment reports
	if err := server.loadEquipmentReports(); err != nil {
		logger.Fatal("Failed to load equipment reports: ", err)
	}

	// Start all of the background threads. There's a UDP server to grab
	// reports, there's an HTTP server to interact with non-IOT equipment,
	// and there's a background thread which updates the equipment reports
	// infrequently.
	go server.threadedLaunchUDPServer()
	go server.threadedMigrateReports()
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Initiate the shutdown.
	err := server.httpServer.Shutdown(ctx)
	if err != nil {
		// Log the error if the shutdown fails.
		server.logger.Errorf("HTTP server shutdown error: %v", err)
	}

	// Close the UDP connection
	server.mu.Lock()
	if server.udpConn != nil {
		server.udpConn.Close()
	}
	server.mu.Unlock()

	// Close the logger
	server.logger.Close()
}

// loadGCATempKey loads the temporary key of the Glow Certification Agent.
func (server *GCAServer) loadGCATempKey() error {
	path := filepath.Join(server.baseDir, "gca.tempkey")
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return fmt.Errorf("unable to read temp gca key from file: %v", err)
	}
	copy(server.gcaTempKey[:], data)
	return nil
}

// loadGCAPubkey loads the Glow Certification Agent public key from a file.
func (server *GCAServer) loadGCAPubkey() error {
	pubkeyPath := filepath.Join(server.baseDir, "gca.pubkey")
	pubkeyData, err := ioutil.ReadFile(pubkeyPath)
	if os.IsNotExist(err) {
		server.logger.Info("GCA Temp Key not available, waiting to receive over the API")
		return nil
	}
	if err != nil {
		return fmt.Errorf("unable to read public key from file: %v", err)
	}
	server.gcaPubkeyAvailable = true
	copy(server.gcaPubkey[:], pubkeyData)
	return nil
}
