package server

// This file contains most of the brains of the server, it creates and launches
// all of the key components and background threads, and it handles shutting
// them all down as well.

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

	"github.com/glowlabs-org/gca-backend/glow"
)

// TODO: Every GET endpoint from the server needs to be signed by the server
// key with a timestamp.
//
// TODO: Write the endpoints that allow the GCA to distribute the failover
// list, and allow the client to maintain the failover list.
//
// TODO: Need to write an endpoint that will allow the GCA (and maybe anyone)
// to fetch all of the data from the server. This would actually just consist
// of loading up the respective persist files and zipping them together. That
// would allow anyone to reconstruct all of the equipment, the bans, and the
// reports.
//
// TODO: Need some sort of logrotate or other log protection that prevents spam
// logging from filling up the server's disk space.
//
// TODO: Need to write endpoints that will allow the IoT devices to find other
// GCA servers and Veto Council servers to publish their reports to. I haven't
// figured out yet what happens to IoT devices when their GCA gets slashed -
// the devices need to find a new home, but they also need some way to realize
// that their GCA is gone, and they need some way to determine what their new
// home should be.
//
// TODO: We want to create a coordination mechanism that makes it unlikely that
// different GCAs will overlap when they assign ShortIDs. That way we can
// easily migrate hardware from one GCA to another.

// GCAServer defines the structure for our Glow Certification Agent Server.
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
	gcaPubkey          glow.PublicKey
	gcaPubkeyAvailable bool
	gcaTempKey         glow.PublicKey

	// These are the signing keys for the GCA server. The GCA server will
	// sign all GET requests so that the caller knows the data is
	// authentic.
	staticPrivateKey glow.PrivateKey
	staticPublicKey glow.PublicKey

	baseDir    string         // Base directory for server files
	logger     *Logger        // Custom logger for the server
	httpServer *http.Server   // Web server for handling API requests
	httpPort   string         // Records the port that is being used to serve the api
	mux        *http.ServeMux // Routing for HTTP requests
	udpConn    *net.UDPConn   // UDP connection for listening to equipment reports
	udpPort    int            // The port that the UDP conn is listening on
	tcpPort    int            // The port that the TCP listener is using
	quit       chan bool      // A channel to initiate server shutdown
	mu         sync.Mutex
}

// NewGCAServer initializes a new instance of GCAServer and returns either
// the GCAServer or an error.
//
// baseDir specifies the directory where all server files will be stored.
// The function will create this directory if it does not exist.
func NewGCAServer(baseDir string) (*GCAServer, error) {
	// Create the directory if it doesn't exist
	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		if err := os.MkdirAll(baseDir, 0755); err != nil {
			return nil, fmt.Errorf("Failed to create base directory: %v", err)
		}
	}

	// Initialize the HTTP routing and logging functionalities
	mux := http.NewServeMux()
	loggerPath := filepath.Join(baseDir, "server.log")
	logger, err := NewLogger(defaultLogLevel, loggerPath)
	if err != nil {
		return nil, fmt.Errorf("Logger initialization failed: %v", err)
	}

	// Compute the timeslot that should be used as the reports offset.
	// This is aimed to be 0:00:00 on Monday UTC.
	now := currentTimeslot()
	closestWeek := uint32(0)
	for now-closestWeek > 3200 {
		closestWeek += 2016
	}

	// Initialize GCAServer with the necessary fields
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

	// Load the GCA Server keys.
	pub, priv, err := server.loadGCAServerKeys()
	if err != nil {
		return nil, fmt.Errorf("failed to load gca server keys: %v", err)
	}
	server.staticPublicKey = pub
	server.staticPrivateKey = priv
	// Load the temporary Glow Certification Agent public key.
	if err := server.loadGCATempKey(); err != nil {
		return nil, fmt.Errorf("failed to load GCA public key: %v", err)
	}
	// Load the Glow Certification Agent permanent public key
	if err := server.loadGCAPubkey(); err != nil {
		return nil, fmt.Errorf("failed to load GCA public key: %v", err)
	}
	// Load equipment public keys
	if err := server.loadEquipment(); err != nil {
		return nil, fmt.Errorf("failed to load server equipment: %v", err)
	}
	// Load all equipment reports
	if err := server.loadEquipmentReports(); err != nil {
		return nil, fmt.Errorf("failed to load equipment reports: %v", err)
	}

	udpReady := make(chan struct{})
	tcpReady := make(chan struct{})

	// Start the background threads for various server functionalities
	go server.threadedLaunchUDPServer(udpReady)
	go server.threadedMigrateReports()
	go server.threadedListenForSyncRequests(tcpReady)
	server.launchAPI()

	<-udpReady
	<-tcpReady

	// Return the initialized server
	return server, nil
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

// loadGCAServerKeys will load the keys for the GCA server from disk, creating
// new keys if no keys are found.
func (server *GCAServer) loadGCAServerKeys() (glow.PublicKey, glow.PrivateKey, error) {
	path := filepath.Join(server.baseDir, "server.keys")
	data, err := ioutil.ReadFile(path)
	if os.IsNotExist(err) {
		server.logger.Info("Creating keys for the GCA server")
		pub, priv := glow.GenerateKeyPair()
		f, err := os.Create(path)
		if err != nil {
			return glow.PublicKey{}, glow.PrivateKey{}, fmt.Errorf("unable to create file for gca keys: %v", err)
		}
		var data [96]byte
		copy(data[:32], pub[:])
		copy(data[32:], priv[:])
		_, err = f.Write(data[:])
		if err != nil {
			return glow.PublicKey{}, glow.PrivateKey{}, fmt.Errorf("unable to write keys to disk: %v", err)
		}
		err = f.Close()
		if err != nil {
			return glow.PublicKey{}, glow.PrivateKey{}, fmt.Errorf("unable to close gca key server file: %v", err)
		}
		return pub, priv, nil
	}
	if err != nil {
		return glow.PublicKey{}, glow.PrivateKey{}, fmt.Errorf("unable to read server keyfile: %v", err)
	}
	var pub glow.PublicKey
	var priv glow.PrivateKey
	copy(pub[:], data[:32])
	copy(priv[:], data[32:])
	return pub, priv, nil
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
