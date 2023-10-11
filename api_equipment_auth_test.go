package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"
)

// TestToAuthorization verifies that an EquipmentAuthorizationRequest
// correctly converts into an EquipmentAuthorizaiton.
func TestToAuthorization(t *testing.T) {
	// Generate a real public-private key pair.
	pubKey, privKey := generateTestKeys()
	// Generate a real signature.
	message := []byte("test message")
	realSignature := ed25519.Sign(privKey, message)

	// Hex-encode the real public key and signature.
	hexPublicKey := hex.EncodeToString(pubKey)
	hexSignature := hex.EncodeToString(realSignature)

	request := EquipmentAuthorizationRequest{
		ShortID:    1,
		PublicKey:  hexPublicKey,
		Capacity:   100,
		Debt:       50,
		Expiration: 50000,
		Signature:  hexSignature,
	}
	ea, err := request.ToAuthorization()

	if err != nil {
		t.Errorf("ToAuthorization returned error: %v", err)
		return
	}

	if ea.ShortID != request.ShortID ||
		!reflect.DeepEqual(pubKey, ea.PublicKey) ||
		ea.Capacity != request.Capacity ||
		ea.Debt != request.Debt ||
		ea.Expiration != request.Expiration ||
		!reflect.DeepEqual(realSignature, ea.Signature) {
		t.Errorf("Conversion failed: got %v, want %v", ea, request)
	}
}

// TestAuthorizeEquipmentEndpoint is a test function that verifies the functionality
// of the Authorize Equipment Endpoint in the GCA Server.
func TestAuthorizeEquipmentEndpoint(t *testing.T) {
	// Generate test directory and GCA keys
	// The GCA keys are cryptographic keys needed by the GCA server.
	dir := generateTestDir(t.Name())
	gcaPrivKey, err := generateGCATestKeys(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Create a new instance of the GCA Server
	// This is the server that will handle the authorization request.
	server := NewGCAServer(dir)
	// Make sure to close the server after the test is complete to free up resources.
	defer server.Close()

	// Create a mock request for equipment authorization.
	// This mimics what a real request might look like.
	body := EquipmentAuthorizationRequest{
		ShortID:    12345,                                                              // A unique identifier for the equipment
		PublicKey:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", // Public key of the equipment for secure communication
		Capacity:   1000000,                                                            // Storage capacity
		Debt:       2000000,                                                            // Current debt value
		Expiration: 2000,                                                               // Expiry time for the equipment
		Signature:  "",                                                                 // Placeholder for the cryptographic signature
	}
	ea, err := body.ToAuthorization()
	if err != nil {
		t.Fatal(err)
	}

	// Sign the authorization request with GCA's private key.
	raw := ea.Serialize()
	signature := ed25519.Sign(gcaPrivKey, raw)
	body.Signature = hex.EncodeToString(signature)

	// Convert the request body to JSON format.
	// This is necessary for HTTP communication.
	jsonBody, _ := json.Marshal(body)

	// Perform an HTTP POST request to the authorize-equipment endpoint.
	resp, err := http.Post("http://localhost"+httpPort+"/api/v1/authorize-equipment", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	// Close the response body to prevent resource leaks.
	defer resp.Body.Close()

	// Check if the HTTP status code is OK (200).
	// Any other code means something went wrong.
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		t.Fatalf("Expected status 200, but got %d. Response: %s", resp.StatusCode, string(bodyBytes))
	}

	// Decode the JSON response from the server.
	var response map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Validate the server's response.
	// In a successful case, it should return a "status" key with a value of "success".
	if status, exists := response["status"]; !exists || status != "success" {
		t.Fatalf("Unexpected response: %v", response)
	}
}
