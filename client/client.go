package client

import (
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"

	"github.com/glowlabs-org/gca-backend/glow"
)

// The stateful object for the client.
type Client struct {
	primaryServer glow.PublicKey
	gcaServers    map[glow.PublicKey]GCAServer

	baseDir   string
	pubkey    glow.PublicKey
	privkey   glow.PrivateKey
	gcaPubkey glow.PublicKey
}

// NewClient will return a new client that is running smoothly.
func NewClient(baseDir string) (*Client, error) {
	// Create an empty client.
	c := &Client{
		baseDir: baseDir,
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

	return c, nil
}

// Currently Close is a no-op.
func (c *Client) Close() error { return nil }

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
		if c.gcaServers[server].Banned {
			c.primaryServer = server
			break
		}
	}

	return nil
}
