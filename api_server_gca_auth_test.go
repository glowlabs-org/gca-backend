package main

// This file contains testing related to the GCA temporary key and the
// authorization process. The major test is the integration test, which tries
// to walk the GCAServer through its entire lifecycle. This means that we set
// up a server where the pubkey of the temporary GCA key has already been saved
// to disk at the file "gca.tempkey". The test itself will have to create the
// temporary key and save the public key of the temporary key to that location.
//
// When the GCA server starts up, it will load and see the temporary key. Then
// the integration test will need to create a new key which represents the real
// GCA key, and the test will need to use the right endpoint to tell the
// GCAServer what the real GCA key is.
//
// Then the test needs to verify that all of the actions led to the desired
// result.
//
// The process of creating the temporary key and launching the GCAServer should
// be put into a separate function so that it can be used by all tests. The
// process of creating the real GCA key and using an endpoint to submit the
// real GCA key to the server should similarly be its own separate function so
// that other tests can use it when they need it.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// gcaServerWithTempKey creates a temporary GCA key, saves its public key to a
// file, and launches the GCAServer.
//
// dir specifies the directory where the temporary public key will be stored.
// The function returns the created GCAServer instance and any errors that
// occur.
func gcaServerWithTempKey(dir string) (gcas *GCAServer, tempPrivKey PrivateKey, err error) {
	// Create the temp priv key, corresponding directory and file, and
	// write the public key to disk where the GCAServer will look for it at
	// startup.
	tempPubKey, tempPrivKey := GenerateKeyPair()
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return nil, PrivateKey{}, fmt.Errorf("unable to create gca dir: %v", err)
		}
	}
	pubKeyPath := filepath.Join(dir, "gca.tempkey")
	if err := ioutil.WriteFile(pubKeyPath, tempPubKey[:], 0644); err != nil {
		return nil, PrivateKey{}, fmt.Errorf("failed to write public key to file: %v", err)
	}

	// Initialize and launch the GCAServer.
	gcas = NewGCAServer(dir)
	return gcas, tempPrivKey, nil
}

// submitGCAKey takes in a GCAServer object and submits the 'real'
// GCA public key to it. It returns the private key corresponding to
// the GCA public key and any error encountered during the process.
func (gcas *GCAServer) submitGCAKey(tempPrivKey PrivateKey) (gcaPrivKey PrivateKey, err error) {
	// Generate the new key pair for the GCA. Assume GenerateKeyPair
	// is a function that returns a public key and a private key.
	publicKey, privateKey := GenerateKeyPair()

	// Create a GCAKey object. We'll populate this with the newly
	// generated public key and a corresponding signature.
	gk := GCAKey{PublicKey: publicKey}
	signingBytes := gk.SigningBytes()
	gk.Signature = Sign(signingBytes, tempPrivKey)

	// Create a new request payload
	reqPayload := RegisterGCARequest{
		GCAKey:    fmt.Sprintf("%x", gk.PublicKey),
		Signature: fmt.Sprintf("%x", gk.Signature),
	}
	payloadBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return PrivateKey{}, fmt.Errorf("error marshaling payload: %v", err)
	}

	// Create a new HTTP request
	req, err := http.NewRequest("POST", "http://localhost"+gcas.httpPort+"/api/v1/register-gca", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return PrivateKey{}, fmt.Errorf("error creating new http request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return PrivateKey{}, fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	// Read the response and validate it
	if resp.StatusCode != http.StatusOK {
		// Read the full response body for debugging
		respBodyBytes, readErr := ioutil.ReadAll(resp.Body)
		if readErr != nil {
			return PrivateKey{}, fmt.Errorf("failed to read response body: %v", readErr)
		}
		// Log or print the full response body
		return PrivateKey{}, fmt.Errorf("received a non-200 status code: %d :: %s", resp.StatusCode, string(respBodyBytes))
	}

	// Return the private key and no error since the function succeeded.
	return privateKey, nil
}

// TestGCAKeyLifecycle tests the full lifecycle of GCA keys,
// including temporary key creation, server startup, real key submission, and validation.
func TestGCAKeyLifecycle(t *testing.T) {
	// Generate a test directory.
	dir := generateTestDir(t.Name())

	// Setup the test environment and launch the GCAServer.
	gcas, tempPrivKey, err := gcaServerWithTempKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer gcas.Close()

	// Check that the server lists the gca pubkey as unavailable.
	if gcas.gcaPubkeyAvailable {
		t.Fatal("gca pubkey should be set to unavailable before a pubkey has been submitted")
	}

	// Try submitting a public key to the server using the wrong priv key.
	badTempKey := tempPrivKey
	badTempKey[0]++
	_, err = gcas.submitGCAKey(badTempKey)
	if err == nil {
		t.Fatal("expected an error")
	}
	if gcas.gcaPubkeyAvailable {
		t.Fatal("gca pubkey should be set to unavailable before a pubkey has been submitted")
	}

	// Try submitting a public key to the server using the temp priv key.
	_, err = gcas.submitGCAKey(tempPrivKey)
	if err != nil {
		t.Fatal(err)
	}
	if !gcas.gcaPubkeyAvailable {
		t.Fatal("gca pubkey should be set to available after a pubkey has been submitted")
	}

	// Check that we get an error when trying to submit another gca key.
	_, err = gcas.submitGCAKey(tempPrivKey)
	if err == nil {
		t.Fatal("expecting an error")
	}

}
