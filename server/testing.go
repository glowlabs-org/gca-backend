package server

// testing.go contains a bunch of exported functions that are useful for
// testing, but are really only intended to be used for testing.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"time"

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

// BaseDir returns the base dir of the GCA server.
func (gcas *GCAServer) BaseDir() string {
	return gcas.baseDir
}

// CheckInvariants is a function which will ensure that the data on the server
// is self-consistent. If something is broken, it means the struct has corrupted
// and a panic is necessary to prevent grey goo from infecting the system.
//
// This function is primarily used during testing, but doesn't hurt to run
// occasionally in prod.
func (gcas *GCAServer) CheckInvariants() {
	// Lock the gcas for concurrency safety if needed
	gcas.mu.Lock()

	// Check if equipmentShortID has the same number of elements as equipment
	if len(gcas.equipment) != len(gcas.equipmentShortID) {
		gcas.mu.Unlock()
		panic("equipment and equipmentShortID maps have different sizes")
	}

	// Create a map to track unique PublicKeys
	pubKeys := make(map[glow.PublicKey]struct{})

	for shortID, auth := range gcas.equipment {
		// Check for unique PublicKey
		if _, exists := pubKeys[auth.PublicKey]; exists {
			gcas.mu.Unlock()
			panic("duplicate PublicKey found in equipment map")
		}
		pubKeys[auth.PublicKey] = struct{}{}

		// Check if equipmentShortID correctly maps to equipment
		if mappedShortID, exists := gcas.equipmentShortID[auth.PublicKey]; !exists || mappedShortID != shortID {
			gcas.mu.Unlock()
			panic("mismatch between equipment and equipmentShortID maps")
		}

		// Check that an impact rate element exists for this equipment.
		_, exists := gcas.equipmentImpactRate[shortID]
		if !exists {
			gcas.mu.Unlock()
			panic("the equipment impact rate was not established for this equipment")
		}
	}
	gcas.mu.Unlock()
}

// AuthorizeEquipment will submit an equipment authorization to the GCA server
// for the provided equipment.
func (gcas *GCAServer) AuthorizeEquipment(ea glow.EquipmentAuthorization, gcaPrivKey glow.PrivateKey) error {
	sb := ea.SigningBytes()
	ea.Signature = glow.Sign(sb, gcaPrivKey)
	j, err := json.Marshal(ea)
	if err != nil {
		return fmt.Errorf("unable to marshal equipment authorization: %v", err)
	}
	resp, err := http.Post(fmt.Sprintf("http://localhost:%v/api/v1/authorize-equipment", gcas.httpPort), "application/json", bytes.NewBuffer(j))
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("Expected status 200, but got %d. Response: %s", resp.StatusCode, string(bodyBytes))
	}
	err = resp.Body.Close()
	if err != nil {
		return fmt.Errorf("failed to close response: %v", err)
	}
	return nil
}

// SetupTestEnvironment will return a fully initialized gca server that is
// ready to be used.
func SetupTestEnvironment(testName string) (gcas *GCAServer, dir string, gcaPubKey glow.PublicKey, gcaPrivKey glow.PrivateKey, err error) {
	dir = glow.GenerateTestDir(testName)
	server, tempPrivKey, err := gcaServerWithTempKey(dir)
	if err != nil {
		return nil, "", glow.PublicKey{}, glow.PrivateKey{}, fmt.Errorf("unable to create gca server with temp key: %v", err)
	}
	// On slower hosts such as test runners, the UDP thread may not be reading messages soon enough,
	// so add a short delay here.
	time.Sleep(50 * time.Microsecond)
	gcaPubKey, gcaPrivKey, err = server.submitGCAKey(tempPrivKey)
	if err != nil {
		return nil, "", glow.PublicKey{}, glow.PrivateKey{}, fmt.Errorf("unable to submit gca priv key: %v", err)
	}
	return server, dir, gcaPubKey, gcaPrivKey, nil
}

// Same as SetupTestEnvironment except that the GCA keys are already known.
func SetupTestEnvironmentKnownGCA(testName string, gcaPublicKey glow.PublicKey, gcaPrivateKey glow.PrivateKey) (gcas *GCAServer, dir string, err error) {
	dir = glow.GenerateTestDir(testName)
	server, tempPrivKey, err := gcaServerWithTempKey(dir)
	if err != nil {
		return nil, "", fmt.Errorf("unable to create gca server with temp key: %v", err)
	}
	// On slower hosts such as test runners, the UDP thread may not be reading messages soon enough,
	// so add a short delay here.
	time.Sleep(50 * time.Microsecond)
	err = server.submitKnownGCAKey(tempPrivKey, gcaPublicKey, gcaPrivateKey)
	if err != nil {
		return nil, "", fmt.Errorf("unable to submit gca priv key: %v", err)
	}
	return server, dir, nil
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
	pubKeyPath := filepath.Join(dir, "gcaTempPubKey.dat")
	if err := ioutil.WriteFile(pubKeyPath, tempPubKey[:], 0644); err != nil {
		return nil, glow.PrivateKey{}, fmt.Errorf("failed to write public key to file: %v", err)
	}

	// Create empty username and password files for watttime.
	if err := os.MkdirAll(filepath.Join(dir, "watttime_data"), 0755); err != nil {
		return nil, glow.PrivateKey{}, fmt.Errorf("failed to create server basedir: %v", err)
	}
	err = ioutil.WriteFile(filepath.Join(dir, "watttime_data", "username"), []byte("hi"), 0644)
	if err != nil {
		return nil, glow.PrivateKey{}, fmt.Errorf("failed to write watttime username: %v", err)
	}
	err = ioutil.WriteFile(filepath.Join(dir, "watttime_data", "password"), []byte("ih"), 0644)
	if err != nil {
		return nil, glow.PrivateKey{}, fmt.Errorf("failed to write watttime password: %v", err)
	}

	// Initialize and launch the GCAServer.
	gcas, err = NewGCAServer(dir)
	if err != nil {
		return nil, glow.PrivateKey{}, fmt.Errorf("failed to create gca server: %v", err)
	}
	return gcas, tempPrivKey, nil
}

// submitKnownGCAKey will submit the GCA key to the GCA server using a known
// GCA key.
func (gcas *GCAServer) submitKnownGCAKey(tempPrivKey glow.PrivateKey, publicKey glow.PublicKey, privateKey glow.PrivateKey) error {
	// Create a GCAKey object and populate it with the public key and signature.
	gr := GCARegistration{GCAKey: publicKey}
	signingBytes := gr.SigningBytes()
	gr.Signature = glow.Sign(signingBytes, tempPrivKey)
	payloadBytes, err := json.Marshal(gr)
	if err != nil {
		return fmt.Errorf("error marshaling payload: %v", err)
	}

	// Create a new HTTP request to submit the GCA key.
	req, err := http.NewRequest("POST", fmt.Sprintf("http://localhost:%v/api/v1/register-gca", gcas.httpPort), bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("error creating new http request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Send the HTTP request.
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	// Validate the server's response.
	if resp.StatusCode != http.StatusOK {
		// Read and log the full response body for debugging.
		respBodyBytes, readErr := ioutil.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("failed to read response body: %v", readErr)
		}
		return fmt.Errorf("received a non-200 status code: %d :: %s", resp.StatusCode, string(respBodyBytes))
	}

	// Decode the JSON response from the server.
	var grr GCARegistrationResponse
	err = json.NewDecoder(resp.Body).Decode(&grr)
	if err != nil {
		return fmt.Errorf("failed to decode json response: %v", err)
	}
	if grr.PublicKey != gcas.staticPublicKey {
		return fmt.Errorf("got wrong response from server: %v", grr)
	}
	return nil
}

// submitGCAKey takes in a GCAServer object and submits the 'real'
// GCA public key to it. It returns the private key corresponding to
// the GCA public key and any error encountered during the process.
func (gcas *GCAServer) submitGCAKey(tempPrivKey glow.PrivateKey) (gcaPubKey glow.PublicKey, gcaPrivKey glow.PrivateKey, err error) {
	publicKey, privateKey := glow.GenerateKeyPair()
	err = gcas.submitKnownGCAKey(tempPrivKey, publicKey, privateKey)
	if err != nil {
		return glow.PublicKey{}, glow.PrivateKey{}, fmt.Errorf("unable to submit gca key using newly generated keys: %v", err)
	}
	return publicKey, privateKey, nil
}
