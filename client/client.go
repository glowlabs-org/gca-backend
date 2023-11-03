package client

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/glowlabs-org/gca-backend/glow"
)

// The stateful object for the client.
type Client struct {
	baseDir string
	pubkey  glow.PublicKey
	privkey glow.PrivateKey
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
	path := filepath.Join(c.baseDir, "client.keys")
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
