package client

// The client package implements the code that runs on GCA monitoring
// equipment. The main purpose of this software is to regularly read the output
// of the monitoring sensors (hosted on-device and published to a specific
// file), sign that output, and submit the signed output as a 'report' to the
// GCA servers.
//
// The client is expected to be running on a lightweight IoT device that is
// heavily bandwidth constrained. Most of the reports are submitted over UDP,
// which is unreliable. Therefore a background thread checks the server every
// ~6 hours to see what reports didn't make it. Udp was chosen to save
// bandwidth, and to be more tolerant of latency.
//
// There are a few other tasks that need to be completed regularly, and the
// code is structured to complete everything at the same time. The sync loop
// will look for reports that didn't make it all the way to the server, it will
// look for new servers that have been authorized by the GCA, it will look for
// banned servers, and it will look for GCA migration orders.
//
// The general threat model of the client is to assume that one of the servers
// may go rogue. The client wants to ensure that if one server goes rogue, the
// client will still be able to submit reports to the GCA. If the GCA
// themselves goes rogue, the client can be fully corrupted and the only fix is
// to replace the software with a brand new instance. Since replacing the
// software just requires swapping out an SD card, even this is not really that
// scary of a problem.
//
// The client will randomly switch to a new server every time it syncs. This is
// because it makes the client more resilient to a rogue GCA server. If a GCA
// server goes rogue, the GCA can ban it. And then the client will eventually
// (typically within a day or less) discover that the server has been banned
// and it will switch away. If the client does not switch servers at every sync
// operation, it could take much, much longer for the client to discover that
// it's using a banned server. And that could prevent its reports from getting
// to the GCA, which will prevent the owner from receiving rewards for
// producing carbon credits.
//
// Another design consideration was to ensure that the client would not ever
// overwhelm the servers with too many requests. The client explicitly spreads
// out its messages so that large swarms of clients are hitting the servers at
// random times throughout each 5 minute interval, rather than all hitting the
// server right at the 5 minute mark for each timeslot.
//
// These clients are designed to last a long time without any code updates,
// which is why there is a lot more defensive programming than may otherwise be
// typical.

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/glowlabs-org/gca-backend/glow"
	"github.com/glowlabs-org/threadgroup"
)

// The stateful object for the client.
type Client struct {
	// Dynamic state
	gcaPubKey     glow.PublicKey
	gcaServers    map[glow.PublicKey]GCAServer
	primaryServer glow.PublicKey
	shortID       uint32

	// Setup parameters
	staticBaseDir       string
	staticHistoryFile   *os.File
	staticHistoryOffset uint32
	staticPubKey        glow.PublicKey
	staticPrivKey       glow.PrivateKey

	// Energy multiplier
	energyMultiplier float64
	energyDivider    float64

	// Channel to signal a successful sync.
	syncChan chan bool

	// Sync primitives.
	mu sync.Mutex
	tg threadgroup.ThreadGroup
}

// NewClient will return a new client that is running smoothly.
func NewClient(baseDir string) (*Client, error) {
	// Create an empty client.
	c := &Client{
		staticBaseDir: baseDir,
	}
	if testMode {
		// Create a background thread that will panic if the client is
		// open for more than 120 seconds. This is helps detect
		// unclosed clients during testing. Run the test suite with
		// -count=100 so that each test is alive long enough to figure
		// out if a client isn't closed.
		c.tg.Launch(func() {
			if c.tg.Sleep(time.Second * 120) {
				panic("client was not closed during testing: " + baseDir)
			}
		})
	}

	// Load the persist data for the client.
	err := c.loadKeypair()
	if err != nil {
		return nil, fmt.Errorf("unable to load client keypair: %v", err)
	}
	err = c.loadGCAPub()
	if err != nil {
		return nil, fmt.Errorf("unable to load GCA public key: %v", err)
	}
	err = c.loadGCAServers()
	if err != nil {
		return nil, fmt.Errorf("unable to load GCA server list: %v", err)
	}
	err = c.loadHistory()
	if err != nil {
		return nil, fmt.Errorf("unable to open the history file: %v", err)
	}
	err = c.loadShortID()
	if err != nil {
		return nil, fmt.Errorf("unable to load the short id: %v", err)
	}
	err = c.readCTSettingsFile()
	if err != nil {
		return nil, fmt.Errorf("error reading CT file: %v", err)
	}

	// Launch a loop that will monitor for successful syncs, and create a request
	// restart file after 24 hours.
	c.launchRequestRestartFile()

	// Launch the loop that will send UDP reports to the GCA server. The
	// regular synchronzation checks also happen inside this loop.
	c.launchSendReports()

	return c, nil
}

// Currently only closes the history file and shuts down the sync thread.
func (c *Client) Close() error {
	return c.tg.Stop()
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
	path := filepath.Join(c.staticBaseDir, ClientKeyFile)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return fmt.Errorf("client keys not found, the client was configured incorrectly")
	}
	if err != nil {
		return fmt.Errorf("unable to read keyfile: %v", err)
	}
	copy(c.staticPubKey[:], data[:32])
	copy(c.staticPrivKey[:], data[32:])
	return nil
}

// loadGCAPub will load the public key of the GCA, which the client then uses
// to verify that the servers it is reporting to are in good standing according
// to the GCA.
func (c *Client) loadGCAPub() error {
	path := filepath.Join(c.staticBaseDir, GCAPubKeyFile)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return fmt.Errorf("client keys not found, the client was configured incorrectly")
	}
	if err != nil {
		return fmt.Errorf("unable to read keyfile: %v", err)
	}
	copy(c.gcaPubKey[:], data[:32])
	return nil
}

// loadGCAServers will load the set of servers that are known to the client as
// viable recipients of client data. The servers syncrhonize between
// themselves, so the client only needs to send to one of them.
func (c *Client) loadGCAServers() error {
	path := filepath.Join(c.staticBaseDir, GCAServerMapFile)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return fmt.Errorf("GCA server file not found, client was configured incorrectly: %v", err)
	}
	if err != nil {
		return fmt.Errorf("unable to read gca server file: %v", err)
	}

	c.gcaServers, err = UntrustedDeserializeGCAServerMap(data)
	if err != nil {
		return fmt.Errorf("unable to decode the data in the gac server file: %v", err)
	}
	if len(c.gcaServers) == 0 {
		return fmt.Errorf("no GCA servers found, client was configured incorrectly")
	}

	// Create a randomized array of the servers and pick the first
	// non-banned server from the randomized list. This ensures that even
	// at startup the client is being robust against bad actors.
	servers := make([]glow.PublicKey, 0, len(c.gcaServers))
	for server := range c.gcaServers {
		servers = append(servers, server)
	}
	for i := range servers {
		j, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return fmt.Errorf("unable to randomize the array: %v", err)
		}
		servers[i], servers[j.Int64()] = servers[j.Int64()], servers[i]
	}
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
	path := filepath.Join(c.staticBaseDir, ShortIDFile)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return fmt.Errorf("client shortID not found, the client was configured incorrectly")
	}
	if err != nil {
		return fmt.Errorf("unable to read short id file: %v", err)
	}
	c.shortID = binary.LittleEndian.Uint32(data)
	return nil
}

// readCTFile looks for a CT settings file, and if it is found,
// parses it by line for the energy multiplier and divider.
// If the file is not found sets defaults and exits.
func (c *Client) readCTSettingsFile() error {
	path := filepath.Join(c.staticBaseDir, CTSettingsFile)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		c.energyMultiplier = EnergyMultiplierDefault
		c.energyDivider = EnergyDividerDefault
		return nil
	}
	if err != nil {
		return fmt.Errorf("error opening ct settings file: %v", err)
	}
	buf := bytes.NewReader(data)
	scanner := bufio.NewScanner(buf)
	if !scanner.Scan() {
		return fmt.Errorf("ct settings file has no 1st line")
	}
	mult, err := strconv.ParseFloat(scanner.Text(), 64)
	if err != nil {
		return fmt.Errorf("could not parse 1st ct settings line: %v", err)
	}
	if !scanner.Scan() {
		return fmt.Errorf("ct settings file has no 2nd line")
	}
	div, err := strconv.ParseFloat(scanner.Text(), 64)
	if err != nil {
		return fmt.Errorf("could not parse 2nd ct settings line: %v", err)
	}
	c.energyMultiplier = mult
	c.energyDivider = div
	return nil
}

// launchRequestRestartFile starts a timer, which creates an empty request restart file after
// the specified duration. The file is removed if it exists whenever a successful
// sync happens, and the timer is reset. The file is removed on shutdown if it exists.
//
// The file is intended to be used by a systemd process.
func (c *Client) launchRequestRestartFile() {
	c.syncChan = make(chan bool)
	path := filepath.Join(c.staticBaseDir, RequestRestartFile)
	c.tg.AfterStop(func() error {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("can't remove request restart file %v: %v", path, err)
		}
		return nil
	})
	c.tg.Launch(func() {
		timer := time.NewTimer(RequestRestartFileDelay)
		for {
			select {
			case <-c.tg.StopChan():
				return
			case <-c.syncChan:
				if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
					panic("can't remove request restart file: " + path)
				}
				timer.Stop()
				// Consume any timer still in the channel.
				select {
				case <-timer.C:
				default:
				}
				timer.Reset(RequestRestartFileDelay)
			case <-timer.C:
				f, err := os.Create(path)
				if err != nil {
					panic("can't create request restart file: " + path)
				}
				if err := f.Close(); err != nil {
					panic("can't close request restart file: " + path)
				}
			}
		}
	})
}
