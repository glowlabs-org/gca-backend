package client

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glowlabs-org/gca-backend/glow"
)

// Create the environment that every client will expect to have upon startup.
// This environment is going to include:
//
// + a public key and private key for the hardware
// + a public key for the GCA
// + a list of GCA servers where reports can be submitted
//
// The public key and private key of the GCA is what gets returned.
func setupTestEnvironment(baseDir string) (glow.PublicKey, glow.PrivateKey, error) {
	// Create the public key and private key for the hardware.
	pub, priv := glow.GenerateKeyPair()
	gcaPub, gcaPriv := glow.GenerateKeyPair()

	// Save the keys for the client.
	path := filepath.Join(baseDir, ClientKeyfile)
	f, err := os.Create(path)
	if err != nil {
		return glow.PublicKey{}, glow.PrivateKey{}, fmt.Errorf("unable to create the client keyfile: %v", err)
	}
	var data [96]byte
	copy(data[:32], pub[:])
	copy(data[32:], priv[:])
	_, err = f.Write(data[:])
	if err != nil {
		return glow.PublicKey{}, glow.PrivateKey{}, fmt.Errorf("unable to write the keys to disk: %v", err)
	}
	err = f.Close()
	if err != nil {
		return glow.PublicKey{}, glow.PrivateKey{}, fmt.Errorf("unable to close the file: %v", err)
	}
	return gcaPub, gcaPriv, nil
}

// TestBasicClient does minimal testing of the client object.
func TestBasicClient(t *testing.T) {

}
