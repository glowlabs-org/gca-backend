package client

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glowlabs-org/gca-backend/glow"
	"github.com/glowlabs-org/gca-backend/server"
)

// Create the environment that every client will expect to have upon startup.
// This environment is going to include:
//
// + a public key and private key for the hardware
// + a public key for the GCA
// + a list of GCA servers where reports can be submitted
//
// The public key and private key of the GCA is what gets returned.
func setupTestEnvironment(baseDir string, gcaServers []server.GCAServer) (glow.PublicKey, glow.PrivateKey, error) {
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
		return glow.PublicKey{}, glow.PrivateKey{}, fmt.Errorf("unable to write the client keys to disk: %v", err)
	}
	err = f.Close()
	if err != nil {
		return glow.PublicKey{}, glow.PrivateKey{}, fmt.Errorf("unable to close the client keys file: %v", err)
	}

	// Save the public key for the GCA.
	path = filepath.Join(baseDir, GCAPubfile)
	f, err = os.Create(path)
	if err != nil {
		return glow.PublicKey{}, glow.PrivateKey{}, fmt.Errorf("unable to create the gca pubkey file: %v", err)
	}
	copy(data[:], gcaPub[:])
	_, err = f.Write(data[:])
	if err != nil {
		return glow.PublicKey{}, glow.PrivateKey{}, fmt.Errorf("unable to write the gca pubkey to disk: %v", err)
	}
	err = f.Close()
	if err != nil {
		return glow.PublicKey{}, glow.PrivateKey{}, fmt.Errorf("unable to close the gca pubkey file: %v", err)
	}

	// Write the GCA server file for the client, based on the list of
	// servers that has been passed in.
	serverMap := make(map[glow.PublicKey]GCAServer)
	if len(gcaServers) == 0 {
		return glow.PublicKey{}, glow.PrivateKey{}, fmt.Errorf("unable to create client with no servers")
	}
	for _, server := range gcaServers {
		http, tcp, udp := server.Ports()
		serverMap[server.PublicKey()] = GCAServer{
			Banned:   false,
			Location: "localhost",
			HttpPort: http,
			TcpPort:  tcp,
			UdpPort:  udp,
		}
	}

	return gcaPub, gcaPriv, nil
}

// TestBasicClient does minimal testing of the client object.
func TestBasicClient(t *testing.T) {

}
