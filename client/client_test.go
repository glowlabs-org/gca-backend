package client

import (
	"encoding/binary"
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
//
// TODO: We need to take the steps to authorize the client with the server /
// GCA.
func SetupTestEnvironment(baseDir string, gcaPubkey glow.PublicKey, gcaServers []*server.GCAServer) error {
	// Create the public key and private key for the hardware.
	pub, priv := glow.GenerateKeyPair()

	// Save the keys for the client.
	path := filepath.Join(baseDir, ClientKeyfile)
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("unable to create the client keyfile: %v", err)
	}
	var data [96]byte
	copy(data[:32], pub[:])
	copy(data[32:], priv[:])
	_, err = f.Write(data[:])
	if err != nil {
		return fmt.Errorf("unable to write the client keys to disk: %v", err)
	}
	err = f.Close()
	if err != nil {
		return fmt.Errorf("unable to close the client keys file: %v", err)
	}

	// Save the public key for the GCA.
	path = filepath.Join(baseDir, GCAPubfile)
	f, err = os.Create(path)
	if err != nil {
		return fmt.Errorf("unable to create the gca pubkey file: %v", err)
	}
	_, err = f.Write(gcaPubkey[:])
	if err != nil {
		return fmt.Errorf("unable to write the gca pubkey to disk: %v", err)
	}
	err = f.Close()
	if err != nil {
		return fmt.Errorf("unable to close the gca pubkey file: %v", err)
	}

	// Write the GCA server file for the client, based on the list of
	// servers that has been passed in.
	serverMap := make(map[glow.PublicKey]GCAServer)
	if len(gcaServers) == 0 {
		return fmt.Errorf("unable to create client with no servers")
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
	mapData, err := SerializeGCAServerMap(serverMap)
	if err != nil {
		return fmt.Errorf("unable to serialize the server map: %v", err)
	}
	path = filepath.Join(baseDir, GCAServerMapFile)
	f, err = os.Create(path)
	if err != nil {
		return fmt.Errorf("unable to create the gca pubkey file: %v", err)
	}
	_, err = f.Write(mapData)
	if err != nil {
		return fmt.Errorf("unable to write the gca pubkey to disk: %v", err)
	}
	err = f.Close()
	if err != nil {
		return fmt.Errorf("unable to close the gca pubkey file: %v", err)
	}

	// Create the history file.
	var offsetBytes [4]byte
	binary.LittleEndian.PutUint32(offsetBytes[:], glow.CurrentTimeslot())
	path = filepath.Join(baseDir, HistoryFile)
	f, err = os.Create(path)
	if err != nil {
		return fmt.Errorf("unable to create the history file: %v", err)
	}
	_, err = f.Write(offsetBytes[:])
	if err != nil {
		return fmt.Errorf("unable to write the history file: %v", err)
	}
	err = f.Close()
	if err != nil {
		return fmt.Errorf("unable to close the history file: %v", err)
	}

	// TODO: Authorize the device, which will give you a ShortID.

	// Save the ShortID for the device.
	path = filepath.Join(baseDir, ShortIDFile)
	f, err = os.Create(path)
	if err != nil {
		return fmt.Errorf("unable to create the gca pubkey file: %v", err)
	}
	var shortIDBytes [4]byte // Use ID of 0 for now. TODO: dynamically asign short id
	_, err = f.Write(shortIDBytes[:])
	if err != nil {
		return fmt.Errorf("unable to write the gca pubkey to disk: %v", err)
	}
	err = f.Close()
	if err != nil {
		return fmt.Errorf("unable to close the gca pubkey file: %v", err)
	}

	return nil
}

// FullClientTestEnvironment is a helper function for setting up a test
// environment for the client, including creating a server.
func FullClientTestEnvironment(name string) (*Client, error) {
	gcas, _, _, err := server.SetupTestEnvironment(name + "_server1")
	if err != nil {
		return nil, fmt.Errorf("unable to set up the test environment for a server: %v", err)
	}
	gcaPubkey := gcas.GCAPublicKey()
	clientDir := glow.GenerateTestDir(name + "_client1")
	err = SetupTestEnvironment(clientDir, gcaPubkey, []*server.GCAServer{gcas})
	if err != nil {
		return nil, fmt.Errorf("unable to set up the client test environment: %v", err)
	}
	c, err := NewClient(clientDir)
	if err != nil {
		return nil, fmt.Errorf("unable to create client: %v", err)
	}
	return c, nil
}

// TestBasicClient does minimal testing of the client object.
func TestBasicClient(t *testing.T) {
	gcas, _, _, err := server.SetupTestEnvironment(t.Name() + "_server1")
	if err != nil {
		t.Fatal(err)
	}
	gcaPubkey := gcas.GCAPublicKey()
	clientDir := glow.GenerateTestDir(t.Name() + "_client1")
	err = SetupTestEnvironment(clientDir, gcaPubkey, []*server.GCAServer{gcas})
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewClient(clientDir)
	if err != nil {
		t.Fatal(err)
	}
}
