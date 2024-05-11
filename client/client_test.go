package client

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
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
func SetupTestEnvironment(baseDir string, gcaPubkey glow.PublicKey, gcaPrivKey glow.PrivateKey, gcaServers []*server.GCAServer) error {
	// Create the public key and private key for the hardware.
	pub, priv := glow.GenerateKeyPair()

	// Save the keys for the client.
	path := filepath.Join(baseDir, ClientKeyFile)
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
	path = filepath.Join(baseDir, GCAPubKeyFile)
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
			Location: "127.0.0.1",
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

	// Authorize the device.
	shortID := uint32(1)
	ea := glow.EquipmentAuthorization{
		ShortID:    shortID,
		PublicKey:  pub,
		Latitude:   38,
		Longitude:  -100,
		Capacity:   123412341234,
		Debt:       11223344,
		Expiration: 100e6 + glow.CurrentTimeslot(),
	}
	sb := ea.SigningBytes()
	sig := glow.Sign(sb, gcaPrivKey)
	ea.Signature = sig
	jsonEA, err := json.Marshal(ea)
	if err != nil {
		return fmt.Errorf("unable to marshal the auth request")
	}
	for _, server := range gcaServers {
		httpX, _, _ := server.Ports()
		resp, err := http.Post(fmt.Sprintf("http://127.0.0.1:%v/api/v1/authorize-equipment", httpX), "application/json", bytes.NewBuffer(jsonEA))
		if err != nil {
			return fmt.Errorf("unable to authorize device on GCA server: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("got non-OK status code when authorizing gca client")
		}
	}

	// Save the ShortID for the device.
	path = filepath.Join(baseDir, ShortIDFile)
	f, err = os.Create(path)
	if err != nil {
		return fmt.Errorf("unable to create the gca pubkey file: %v", err)
	}
	var shortIDBytes [4]byte
	binary.LittleEndian.PutUint32(shortIDBytes[:], shortID)
	_, err = f.Write(shortIDBytes[:])
	if err != nil {
		return fmt.Errorf("unable to write the gca pubkey to disk: %v", err)
	}
	err = f.Close()
	if err != nil {
		return fmt.Errorf("unable to close the gca pubkey file: %v", err)
	}

	// Save a blank monitoring file.
	newFileDataStr := "timestamp,energy (mWh)"
	path = filepath.Join(baseDir, EnergyFile)
	err = ioutil.WriteFile(path, []byte(newFileDataStr), 0644)
	if err != nil {
		return fmt.Errorf("unable to write the new monitor file: %v", err)
	}

	return nil
}

// FullClientTestEnvironment is a helper function for setting up a test
// environment for the client, including creating a server.
func FullClientTestEnvironment(name string) (*Client, *server.GCAServer, glow.PrivateKey, error) {
	gcas, _, gcaPubkey, gcaPrivKey, err := server.SetupTestEnvironment(name + "_server1")
	if err != nil {
		return nil, nil, glow.PrivateKey{}, fmt.Errorf("unable to set up the test environment for a server: %v", err)
	}
	clientDir := glow.GenerateTestDir(name + "_client1")
	err = SetupTestEnvironment(clientDir, gcaPubkey, gcaPrivKey, []*server.GCAServer{gcas})
	if err != nil {
		return nil, nil, glow.PrivateKey{}, fmt.Errorf("unable to set up the client test environment: %v", err)
	}
	c, err := NewClient(clientDir)
	if err != nil {
		return nil, nil, glow.PrivateKey{}, fmt.Errorf("unable to create client: %v", err)
	}
	return c, gcas, gcaPrivKey, nil
}

// TestBasicClient does minimal testing of the client object.
func TestBasicClient(t *testing.T) {
	gcas, _, gcaPubkey, gcaPrivkey, err := server.SetupTestEnvironment(t.Name() + "_server1")
	if err != nil {
		t.Fatal(err)
	}
	clientDir := glow.GenerateTestDir(t.Name() + "_client1")
	err = SetupTestEnvironment(clientDir, gcaPubkey, gcaPrivkey, []*server.GCAServer{gcas})
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewClient(clientDir)
	if err != nil {
		t.Fatal(err)
	}
	err = c.Close()
	if err != nil {
		t.Fatal(err)
	}
}
