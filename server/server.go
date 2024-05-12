package server

// This file contains most of the brains of the server, it creates and launches
// all of the key components and background threads, and it handles shutting
// them all down as well.

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/glowlabs-org/gca-backend/glow"
	"github.com/glowlabs-org/threadgroup"
)

// TODO: Every GET endpoint from the server needs to be signed by the server
// key with a timestamp.
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
//
// TODO: We need some way for the GCA to download the historic signed equipment
// reports so that the GCA knows the server isn't lying about the production of
// the solar farms. That definitely feels like more of a post-launch concern
// though. The recent reports endpoint sends signed reports but that doesn't
// help for historic data, and GCAs sometimes need access to that historic
// data.

// GCAServer defines the structure for our Glow Certification Agent Server.
type GCAServer struct {
	equipment              map[uint32]glow.EquipmentAuthorization // Map from a ShortID to the full equipment authorization
	equipmentShortID       map[glow.PublicKey]uint32              // Map from a public key to a ShortID
	equipmentBans          map[uint32]struct{}                    // Tracks which equipment is banned
	equipmentImpactRate    map[uint32]*[4032]float64              // Tracks the number of micrograms of CO2 offset per WattHour of energy
	equipmentMigrations    map[glow.PublicKey]EquipmentMigration  // Keeps track of migration orders that have been given to equipment
	equipmentReports       map[uint32]*[4032]glow.EquipmentReport // Keeps all recent reports in memory
	equipmentReportsOffset uint32                                 // What timeslot the equipmentReports arrays start at
	equipmentStatsHistory  []AllDeviceStats                       // A history of all the stats that were collected for each device
	equipmentHistoryOffset uint32                                 // Establishes the first timeslot where history is available

	recentEquipmentAuths []glow.EquipmentAuthorization // Keep recent auths to more easily synchronize with redundant servers
	recentReports        []glow.EquipmentReport        // Keep recent reports to more easily synchronize with redundant servers

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

	// We need to track other servers so that clients who ping us for a
	// list of backup servers have something they can retrieve.
	gcaServers AuthorizedServers

	// These are the signing keys for the GCA server. The GCA server will
	// sign all GET requests so that the caller knows the data is
	// authentic.
	staticPrivateKey glow.PrivateKey
	staticPublicKey  glow.PublicKey

	baseDir        string         // Base directory for server files
	logger         *Logger        // Custom logger for the server
	httpServer     *http.Server   // Web server for handling API requests
	httpPort       uint16         // Records the port that is being used to serve the api
	mux            *http.ServeMux // Routing for HTTP requests
	skipInvariants bool           // If set to true, 'CheckInvariants()' will not run on Close()
	udpPort        uint16         // The port that the UDP conn is listening on
	tcpPort        uint16         // The port that the TCP listener is using
	mu             sync.Mutex
	tg             threadgroup.ThreadGroup
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

	// Initialize GCAServer with the necessary fields
	server := &GCAServer{
		baseDir:             baseDir,
		equipment:           make(map[uint32]glow.EquipmentAuthorization),
		equipmentShortID:    make(map[glow.PublicKey]uint32),
		equipmentBans:       make(map[uint32]struct{}),
		equipmentImpactRate: make(map[uint32]*[4032]float64),
		equipmentMigrations: make(map[glow.PublicKey]EquipmentMigration),
		equipmentReports:    make(map[uint32]*[4032]glow.EquipmentReport),
		recentReports:       make([]glow.EquipmentReport, 0, maxRecentReports),
	}

	// Create the logger and provision its shutdown.
	loggerPath := filepath.Join(baseDir, "server.log")
	logger, err := NewLogger(defaultLogLevel, loggerPath)
	if err != nil {
		return nil, fmt.Errorf("Logger initialization failed: %v", err)
	}
	server.tg.AfterStop(func() error {
		return logger.Close()
	})
	server.logger = logger

	// Create the http server and provision its shutdown.
	server.mux = http.NewServeMux()
	server.httpServer = &http.Server{
		Addr: serverIP + ":" + strconv.Itoa(httpPort),
		Handler: server.mux,
	}
	server.tg.OnStop(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := server.httpServer.Shutdown(ctx)
		if err != nil {
			server.logger.Errorf("HTTP server shutdown error: %v", err)
			return fmt.Errorf("error shutting down the http server: %v", err)
		}
		return nil
	})

	// Load the GCA Server keys.
	server.staticPublicKey, server.staticPrivateKey, err = server.loadGCAServerKeys()
	if err != nil {
		return nil, fmt.Errorf("failed to load gca server keys: %v", err)
	}
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
	// Load the historic data for the equipment. This will also set the
	// 'equipmentReportsOffset` value.
	if err := server.loadEquipmentHistory(); err != nil {
		return nil, fmt.Errorf("failed to load server equipment history: %v", err)
	}
	// Load all equipment reports
	if err := server.loadEquipmentReports(); err != nil {
		return nil, fmt.Errorf("failed to load equipment reports: %v", err)
	}
	// TODO: Load the persisted list of authorized servers.
	//
	// TODO: Sync with all of the other servers and get their latest
	// equipmentReports before starting the threadedMigrateReports loop
	// which will permanently archive our data and prevent it from being
	// used again.


	// Load the watttime credentials.
	wtUsernamePath := filepath.Join(server.baseDir, "watttime_data", "username")
	wtPasswordPath := filepath.Join(server.baseDir, "watttime_data", "password")
	username, err := loadWattTimeCredentials(wtUsernamePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load username: %v", err)
	}
	password, err := loadWattTimeCredentials(wtPasswordPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load password: %v", err)
	}

	// Immediately grab all of the data for the most recent week to catch
	// up on anything that was missed.
	//
	// TODO: This could be a thread so it doesn't block startup.
	err = server.managedGetWattTimeWeekData(username, password)
	if err != nil {
		// This is unfortunate, but this is not cause to abort startup,
		// so we'll just log an error.
		server.logger.Errorf("Unable to get WattTime data for the most recent week: %v", err)
	}

	// Start the background threads for various server functionalities.
	server.launchUDPServer()
	server.launchMigrateReports(username, password)
	server.launchListenForSyncRequests()
	server.tg.Launch(func() {
		server.threadedCollectImpactData(username, password)
	})
	server.tg.Launch(func() {
		server.threadedGetWattTimeWeekData(username, password)
	})
	server.launchAPI()

	// Return the initialized server
	return server, nil
}

// Close cleanly shuts down the GCAServer instance.
func (server *GCAServer) Close() error {
	// By placing this here, we know that every time a server is closed
	// during testing, we are reviewing the state to make sure it's all in
	// order.
	if !server.skipInvariants {
		server.CheckInvariants()
	}
	return server.tg.Stop()
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
	path := filepath.Join(server.baseDir, "gcaTempPubKey.dat")
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return fmt.Errorf("unable to read temp gca key from file: %v", err)
	}
	copy(server.gcaTempKey[:], data)
	return nil
}

// loadGCAPubkey loads the Glow Certification Agent public key from a file.
func (server *GCAServer) loadGCAPubkey() error {
	pubkeyPath := filepath.Join(server.baseDir, "gcaPubKey.dat")
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
