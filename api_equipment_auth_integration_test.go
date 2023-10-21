package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

// TestAuthorizeEquipmentIntegration checks that the full flow for
// authorizing new equipment works as intended.
func TestAuthorizeEquipmentIntegration(t *testing.T) {
	// Generate test directory and GCA keys
	// The GCA keys are cryptographic keys needed by the GCA server.
	dir := generateTestDir(t.Name())
	gcaPrivKey, err := generateGCATestKeys(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Create a new instance of the GCA Server
	server := NewGCAServer(dir)

	// Create the http request that will authorize new equipment.
	body := EquipmentAuthorizationRequest{
		ShortID:    12345,                                                                // A unique identifier for the equipment
		PublicKey:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", // Public key of the equipment for secure communication
		Capacity:   1000000,                                                              // Storage capacity
		Debt:       2000000,                                                              // Current debt value
		Expiration: 2000,                                                                 // Expiry time for the equipment
		Signature:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	ea, err := body.ToAuthorization()
	if err != nil {
		t.Fatal(err)
	}
	// Sign the authorization request with GCA's private key.
	sb := ea.SigningBytes()
	signature, err := crypto.Sign(sb, gcaPrivKey)
	if err != nil {
		t.Fatal(err)
	}
	body.Signature = hex.EncodeToString(signature)
	// Convert the request body to JSON format.
	jsonBody, _ := json.Marshal(body)
	// Perform an HTTP POST request to the authorize-equipment endpoint.
	resp, err := http.Post("http://localhost"+httpPort+"/api/v1/authorize-equipment", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	// Check if the HTTP status code is OK (200).
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
	if status, exists := response["status"]; !exists || status != "success" {
		t.Fatalf("Unexpected response: %v", response)
	}
	// Close the response body to prevent resource leaks.
	resp.Body.Close()

	// Verify that the server now sees the equipment.
	if len(server.equipment) != 1 {
		t.Fatal("server did not receive equipment")
	}

	// Restart the server and verify that the equipment persists after reboot.
	server.Close()
	server = NewGCAServer(dir)
	if len(server.equipment) != 1 {
		t.Fatal("server did not receive equipment")
	}

	// Send a duplicate request. The server should ignore the request.
	resp, err = http.Post("http://localhost"+httpPort+"/api/v1/authorize-equipment", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	// Check if the HTTP status code is OK (200).
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		t.Fatalf("Expected status 200, but got %d. Response: %s", resp.StatusCode, string(bodyBytes))
	}
	if len(server.equipment) != 1 {
		t.Fatal("server did not receive equipment")
	}
	resp.Body.Close()

	// Send a new request, this time with the same ShortID. The server should add the ShortID to the banlist.
	body = EquipmentAuthorizationRequest{
		ShortID:    12345,                                                                // A unique identifier for the equipment
		PublicKey:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", // Public key of the equipment for secure communication
		Capacity:   1000000,                                                              // Storage capacity
		Debt:       2400000,                                                              // Current debt value
		Expiration: 2000,                                                                 // Expiry time for the equipment
		Signature:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	ea, err = body.ToAuthorization()
	if err != nil {
		t.Fatal(err)
	}
	// Sign the authorization request with GCA's private key.
	sb = ea.SigningBytes()
	signature, err = crypto.Sign(sb, gcaPrivKey)
	if err != nil {
		t.Fatal(err)
	}
	body.Signature = hex.EncodeToString(signature)
	// Convert the request body to JSON format.
	jsonBody, _ = json.Marshal(body)
	// Perform an HTTP POST request to the authorize-equipment endpoint.
	resp, err = http.Post("http://localhost"+httpPort+"/api/v1/authorize-equipment", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	// Close the response body to prevent resource leaks.
	resp.Body.Close()

	// The equipment should be gone now, because it should have been banned.
	if len(server.equipment) == 1 {
		t.Fatal("server did not ban equipment")
	}
	if len(server.equipmentBans) != 1 {
		t.Fatal("bad")
	}

	// Restart the server again, make sure that the equipment is still banned.
	server.Close()
	server = NewGCAServer(dir)
	if len(server.equipment) == 1 {
		t.Fatal("server did not ban equipment")
	}
	if len(server.equipmentBans) != 1 {
		t.Fatal("bad")
	}

	// Send a new request, this time with a new ShortID.
	body = EquipmentAuthorizationRequest{
		ShortID:    12346,                                                                // A unique identifier for the equipment
		PublicKey:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", // Public key of the equipment for secure communication
		Capacity:   1000000,                                                              // Storage capacity
		Debt:       2400000,                                                              // Current debt value
		Expiration: 2000,                                                                 // Expiry time for the equipment
		Signature:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	ea, err = body.ToAuthorization()
	if err != nil {
		t.Fatal(err)
	}
	// Sign the authorization request with GCA's private key.
	sb = ea.SigningBytes()
	signature, err = crypto.Sign(sb, gcaPrivKey)
	if err != nil {
		t.Fatal(err)
	}
	body.Signature = hex.EncodeToString(signature)
	// Convert the request body to JSON format.
	jsonBody, _ = json.Marshal(body)
	// Perform an HTTP POST request to the authorize-equipment endpoint.
	resp, err = http.Post("http://localhost"+httpPort+"/api/v1/authorize-equipment", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	// Close the response body to prevent resource leaks.
	resp.Body.Close()
	if len(server.equipment) != 1 {
		t.Fatal("bad")
	}
	if len(server.equipmentBans) != 1 {
		t.Fatal("bad")
	}

	// Restart the server again, make sure the state is maintained.
	server.Close()
	server = NewGCAServer(dir)
	defer server.Close()
	if len(server.equipment) != 1 {
		t.Fatal("bad")
	}
	if len(server.equipmentBans) != 1 {
		t.Fatal("bad")
	}
}
