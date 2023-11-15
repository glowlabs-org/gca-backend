package client

// The client package implements the code that runs on GCA monitoring
// equipment. The main purpose of this software is to regularly read the output
// of the monitoring sensors (hosted on-device and published to a specific
// file), sign that output, and submit the signed output as a 'report' to the
// GCA servers. TODO
//
// The client is expected to be running on a lightweight IoT device that is
// heavily bandwidth constrained. Most of the reports are submitted over UDP,
// so there needs to be another thread running which checks that the reports
// made it to the GCA server. TODO
//
// Because there's a non-trivial amount of money riding on the reports being
// published, the client needs to maintain a robust list of servers that can be
// used as failover servers in the event that the main GCA server goes down.
// There is therefore a background thread that routinely pings all of the known
// GCA servers to ask them for their list of backups. TODO
//
// Every 6 hours, the client needs to run a routine that detects whether the
// primary GCA server is still operational. If the GCA server is not
// operational, the client needs to failover to one of the backup servers. The
// client considers the primary server to be operational as long as the GCA has
// not issued a ban for the server, and as long as the server is responding to
// the TCP requests to check which reports were submitted successfully. The
// client checks whether a ban has been issued by asking all of the failover
// servers for a list of bans. TODO
//
// When the client fails over to a new server, it'll select a server randomly
// from the backups. That backup will become its new primary. TODO
//
// The GCA can optionally declare that a monitoring device is being migrated to
// a new GCA. The client will have to look for that signal from the GCA. If it
// receives that signal, it trusts the GCA and will move to the new GCA as its
// trusted GCA. When it moves to the new GCA, it will receive a new ShortID. TODO

// TODO: Need to build the systemd services that will automatically restart the
// client if it turns off for some reason.
//
// TODO: Should probably have the ssh port open just in case.

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"sync"

	"github.com/glowlabs-org/gca-backend/glow"
)

// The stateful object for the client.
type Client struct {
	gcaPubkey     glow.PublicKey
	gcaServers    map[glow.PublicKey]GCAServer
	primaryServer glow.PublicKey
	shortID       uint32
	serverMu      sync.Mutex

	// NOTE: technically all of these fields should have a 'static' prefix,
	// I guess at some point we can look into having an LLM go through and
	// fix it all.
	baseDir       string
	closeChan     chan struct{}
	historyFile   *os.File
	historyOffset uint32
	pubkey        glow.PublicKey
	privkey       glow.PrivateKey
	syncThread    chan struct{}
}

// NewClient will return a new client that is running smoothly.
func NewClient(baseDir string) (*Client, error) {
	// Step 3: Kick off the background loop that checks for monitoring data and sends UDP reports
	// Step 4: Kick off the background loop that checks for reports that failed to submit, and checks if a failover is needed
	// Step 5: Kick off the background loop that checks for new failover servers and new banned servers
	// Step 6: Kick off the background loop that checks for migration orders

	// Create an empty client.
	c := &Client{
		baseDir:    baseDir,
		closeChan:  make(chan struct{}),
		syncThread: make(chan struct{}),
	}

	// Load the keypair for the client.
	err := c.loadKeypair()
	if err != nil {
		return nil, fmt.Errorf("unable to load client keypair: %v", err)
	}

	// Load the public key of the GCA.
	err = c.loadGCAPub()
	if err != nil {
		return nil, fmt.Errorf("unable to load GCA public key: %v", err)
	}

	// Load the list of GCA servers and their corresponding public keys.
	err = c.loadGCAServers()
	if err != nil {
		return nil, fmt.Errorf("unable to load GCA server list: %v", err)
	}

	// Open the history file.
	err = c.loadHistory()
	if err != nil {
		return nil, fmt.Errorf("unable to open the history file: %v", err)
	}

	// Load the ShortID for the hardware.
	err = c.loadShortID()
	if err != nil {
		return nil, fmt.Errorf("unable to load the short id: %v", err)
	}

	// Launch the loop that will send UDP reports to the GCA server.
	go c.threadedSendReports()

	return c, nil
}

// Currently only closes the history file.
func (c *Client) Close() error {
	close(c.closeChan)
	return c.historyFile.Close()
}

// loadKeypair will load the client keys from disk. The GCA should have put
// keys on the device when the device was created, so not finding keys here is
// a pretty big deal. The GCA puts the keys on the device manually so that the
// GCA can authorize the keys in one simple step.
//
// This does mean that everyone is trusting the GCA not to retain the keys, on
// the other hand the GCA is pretty much the unilaterally trusted authority in
// this case anyway.
func (c *Client) loadKeypair() error {
	path := filepath.Join(c.baseDir, ClientKeyfile)
	data, err := ioutil.ReadFile(path)
	if os.IsNotExist(err) {
		return fmt.Errorf("client keys not found, the client was configured incorrectly")
	}
	if err != nil {
		return fmt.Errorf("unable to read keyfile: %v", err)
	}
	copy(c.pubkey[:], data[:32])
	copy(c.privkey[:], data[32:])
	return nil
}

// loadGCAPub will load the public key of the GCA, which the client then uses
// to verify that the servers it is reporting to are in good standing according
// to the GCA.
func (c *Client) loadGCAPub() error {
	path := filepath.Join(c.baseDir, GCAPubfile)
	data, err := ioutil.ReadFile(path)
	if os.IsNotExist(err) {
		return fmt.Errorf("client keys not found, the client was configured incorrectly")
	}
	if err != nil {
		return fmt.Errorf("unable to read keyfile: %v", err)
	}
	copy(c.gcaPubkey[:], data[:32])
	return nil
}

// loadGCAServers will load the set of servers that are known to the client as
// viable recipients of client data. The servers syncrhonize between
// themselves, so the client only needs to send to one of them.
func (c *Client) loadGCAServers() error {
	path := filepath.Join(c.baseDir, GCAServerMapFile)
	data, err := ioutil.ReadFile(path)
	if os.IsNotExist(err) {
		return fmt.Errorf("GCA server file not found, client was configured incorrectly: %v", err)
	}
	if err != nil {
		return fmt.Errorf("unable to read gca server file: %v", err)
	}

	c.gcaServers, err = DeserializeGCAServerMap(data)
	if err != nil {
		return fmt.Errorf("unable to decode the data in the gac server file: %v", err)
	}
	if len(c.gcaServers) == 0 {
		return fmt.Errorf("no GCA servers found, client was configured incorrectly")
	}

	// Extract all servers into an array
	servers := make([]glow.PublicKey, 0, len(c.gcaServers))
	for server, _ := range c.gcaServers {
		servers = append(servers, server)
	}

	// Randomly shuffle the array
	for i := range servers {
		j, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return fmt.Errorf("unable to randomize the array: %v", err)
		}
		servers[i], servers[j.Int64()] = servers[j.Int64()], servers[i]
	}

	// Iterate over the randomized array and return the first non-banned server
	for _, server := range servers {
		if !c.gcaServers[server].Banned {
			c.primaryServer = server
			break
		}
	}

	return nil
}

// loadShortID will load the short id of the client, which is used in lieu of a
// public key to communicate with the GCA servers. Using a shortID saves 28
// bytes per message, which is valuable on IoT networks sending messages every
// 5 minutes for 10 years.
func (c *Client) loadShortID() error {
	path := filepath.Join(c.baseDir, ShortIDFile)
	data, err := ioutil.ReadFile(path)
	if os.IsNotExist(err) {
		return fmt.Errorf("client shortID not found, the client was configured incorrectly")
	}
	if err != nil {
		return fmt.Errorf("unable to read short id file: %v", err)
	}
	c.shortID = binary.LittleEndian.Uint32(data)
	return nil
}
