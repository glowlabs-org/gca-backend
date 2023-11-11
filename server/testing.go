package server

// testing.go contains a bunch of exported functions that are useful for
// testing, but are really only intended to be used for testing.

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/glowlabs-org/gca-backend/glow"
)

// Ports returns the ports that each of the listeners for this server are listening on.
func (gcas *GCAServer) Ports() (httpPort uint16, tcpPort uint16, udpPort uint16) {
	return gcas.httpPort, gcas.tcpPort, gcas.udpPort
}

// PublicKey returns the public key of this GCA server.
func (gcas *GCAServer) PublicKey() glow.PublicKey {
	return gcas.staticPublicKey
}

// GCAPublicKey returns the public key of the GCA.
func (gcas *GCAServer) GCAPublicKey() glow.PublicKey {
	return gcas.gcaPubkey
}

// CheckInvariants is a function which will ensure that the data on the server
// is self-consistent. If something is broken, it means the struct has corrupted
// and a panic is necessary to prevent grey goo from infecting the system.
//
// This function is primarily used during testing, but doesn't hurt to run
// occasionally in prod.
func (server *GCAServer) CheckInvariants() {
	// Lock the server for concurrency safety if needed
	server.mu.Lock()
	defer server.mu.Unlock()

	// Check if equipmentShortID has the same number of elements as equipment
	if len(server.equipment) != len(server.equipmentShortID) {
		panic("equipment and equipmentShortID maps have different sizes")
	}

	// Create a map to track unique PublicKeys
	pubKeys := make(map[glow.PublicKey]struct{})

	for shortID, auth := range server.equipment {
		// Check for unique PublicKey
		if _, exists := pubKeys[auth.PublicKey]; exists {
			panic("duplicate PublicKey found in equipment map")
		}
		pubKeys[auth.PublicKey] = struct{}{}

		// Check if equipmentShortID correctly maps to equipment
		if mappedShortID, exists := server.equipmentShortID[auth.PublicKey]; !exists || mappedShortID != shortID {
			panic("mismatch between equipment and equipmentShortID maps")
		}
	}
}

// SetupTestEnvironment will return a fully initialized gca server that is
// ready to be used.
func SetupTestEnvironment(testName string) (gcas *GCAServer, dir string, gcaPrivKey glow.PrivateKey, err error) {
	dir = glow.GenerateTestDir(testName)
	server, tempPrivKey, err := gcaServerWithTempKey(dir)
	if err != nil {
		return nil, "", glow.PrivateKey{}, fmt.Errorf("unable to create gca server with temp key: %v", err)
	}
	gcaPrivKey, err = server.submitGCAKey(tempPrivKey)
	if err != nil {
		return nil, "", glow.PrivateKey{}, fmt.Errorf("unable to submit gca priv key: %v", err)
	}
	return server, dir, gcaPrivKey, nil
}

// gcaServerWithTempKey creates a temporary GCA key, saves its public key to a
// file, and launches the GCAServer.
//
// dir specifies the directory where the temporary public key will be stored.
// The function returns the created GCAServer instance and any errors that
// occur.
func gcaServerWithTempKey(dir string) (gcas *GCAServer, tempPrivKey glow.PrivateKey, err error) {
	// Create the temp priv key, corresponding directory and file, and
	// write the public key to disk where the GCAServer will look for it at
	// startup.
	tempPubKey, tempPrivKey := glow.GenerateKeyPair()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return nil, glow.PrivateKey{}, fmt.Errorf("unable to create gca dir: %v", err)
		}
	}
	pubKeyPath := filepath.Join(dir, "gca.tempkey")
	if err := ioutil.WriteFile(pubKeyPath, tempPubKey[:], 0644); err != nil {
		return nil, glow.PrivateKey{}, fmt.Errorf("failed to write public key to file: %v", err)
	}

	// Initialize and launch the GCAServer.
	gcas, err = NewGCAServer(dir)
	if err != nil {
		return nil, glow.PrivateKey{}, fmt.Errorf("failed to create gca server: %v", err)
	}
	return gcas, tempPrivKey, nil
}

// submitGCAKey takes in a GCAServer object and submits the 'real'
// GCA public key to it. It returns the private key corresponding to
// the GCA public key and any error encountered during the process.
func (gcas *GCAServer) submitGCAKey(tempPrivKey glow.PrivateKey) (gcaPrivKey glow.PrivateKey, err error) {
	// Generate the new key pair for the GCA.
	publicKey, privateKey := glow.GenerateKeyPair()

	// Create a GCAKey object and populate it with the public key and signature.
	gk := GCAKey{PublicKey: publicKey}
	signingBytes := gk.SigningBytes()
	gk.Signature = glow.Sign(signingBytes, tempPrivKey)

	// Create a new request payload for the server.
	reqPayload := RegisterGCARequest{
		GCAKey:    fmt.Sprintf("%x", gk.PublicKey),
		Signature: fmt.Sprintf("%x", gk.Signature),
	}
	payloadBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return glow.PrivateKey{}, fmt.Errorf("error marshaling payload: %v", err)
	}

	// Create a new HTTP request to submit the GCA key.
	req, err := http.NewRequest("POST", fmt.Sprintf("http://localhost:%v/api/v1/register-gca", gcas.httpPort), bytes.NewBuffer(payloadBytes))
	if err != nil {
		return glow.PrivateKey{}, fmt.Errorf("error creating new http request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Send the HTTP request.
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return glow.PrivateKey{}, fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	// Validate the server's response.
	if resp.StatusCode != http.StatusOK {
		// Read and log the full response body for debugging.
		respBodyBytes, readErr := ioutil.ReadAll(resp.Body)
		if readErr != nil {
			return glow.PrivateKey{}, fmt.Errorf("failed to read response body: %v", readErr)
		}
		return glow.PrivateKey{}, fmt.Errorf("received a non-200 status code: %d :: %s", resp.StatusCode, string(respBodyBytes))
	}

	// Decode the JSON response from the server.
	var jsonResponse map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&jsonResponse); err != nil {
		return glow.PrivateKey{}, fmt.Errorf("failed to decode json response: %v", err)
	}

	// Extract the server's public key from the JSON response.
	serverPubKeyHex, ok := jsonResponse["ServerPublicKey"]
	if !ok {
		return glow.PrivateKey{}, fmt.Errorf("ServerPublicKey not found in response")
	}

	// Verify that the server's public key matches the expected public key.
	expectedServerPubKeyHex := hex.EncodeToString(gcas.staticPublicKey[:])
	if serverPubKeyHex != expectedServerPubKeyHex {
		return glow.PrivateKey{}, fmt.Errorf("mismatch in server public key: expected %s, got %s", expectedServerPubKeyHex, serverPubKeyHex)
	}

	// Return the GCA private key if everything is successful.
	return privateKey, nil
}
