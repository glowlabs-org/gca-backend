package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/glowlabs-org/gca-backend/glow"
)

// submitNewHardware will create a new piece of hardware and submit it to the
// GCA server.
func (gcas *GCAServer) submitNewHardware(shortID uint32, gcaPrivKey glow.PrivateKey) (ea glow.EquipmentAuthorization, equipmentKey glow.PrivateKey, err error) {
	// Verify that the shortID is free. Even if the shortID is not free,
	// we'll still make the web request because the caller may want the
	// request to go through.
	gcas.mu.Lock()
	_, shortIDAlreadyUsed := gcas.equipment[shortID]
	gcas.mu.Unlock()

	// Create a keypair for the equipment, then create the equipment
	// request body.
	pubkey, equipmentKey := glow.GenerateKeyPair()
	ea = glow.EquipmentAuthorization{
		ShortID:    shortID,
		PublicKey:  pubkey,
		Capacity:   15400300,
		Debt:       11223344,
		Expiration: 100e6 + glow.CurrentTimeslot(), // ensure the hardware won't be invalid for a while, but leave enough room for tests to intentionally expire the hardware
	}
	sb := ea.SigningBytes()
	ea.Signature = glow.Sign(sb, gcaPrivKey)

	// Convert the request to json and post it.
	jsonBody, _ := json.Marshal(ea)
	resp, err := http.Post(fmt.Sprintf("http://127.0.0.1:%v/api/v1/authorize-equipment", gcas.httpPort), "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return glow.EquipmentAuthorization{}, glow.PrivateKey{}, fmt.Errorf("unable to send http request to submit new hardware: %v", err)
	}

	// Verify the response.
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		return glow.EquipmentAuthorization{}, glow.PrivateKey{}, fmt.Errorf("expected status 200, but got %d: %s", resp.StatusCode, string(bodyBytes))
	}
	var response map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return glow.EquipmentAuthorization{}, glow.PrivateKey{}, fmt.Errorf("Failed to decode response: %v", err)
	}
	if status, exists := response["status"]; !exists || status != "success" {
		return glow.EquipmentAuthorization{}, glow.PrivateKey{}, fmt.Errorf("Unexpected response: %v", response)
	}
	resp.Body.Close()

	// Verify that the server sees the new equipment.
	if shortIDAlreadyUsed {
		return glow.EquipmentAuthorization{}, glow.PrivateKey{}, fmt.Errorf("shortID already in use")
	}
	gcas.mu.Lock()
	_, exists := gcas.equipment[shortID]
	gcas.mu.Unlock()
	if !exists {
		return glow.EquipmentAuthorization{}, glow.PrivateKey{}, fmt.Errorf("equipment does not appear to have been added to server correctly")
	}
	gcas.CheckInvariants()
	return ea, equipmentKey, nil
}

// TestAuthorizeEquipmentIntegration checks that the full flow for
// authorizing new equipment works as intended.
func TestAuthorizeEquipmentIntegration(t *testing.T) {
	server, dir, _, gcaPrivKey, err := SetupTestEnvironment(t.Name())
	if err != nil {
		t.Fatal(err)
	}

	// Create the http request that will authorize new equipment.
	ea := glow.EquipmentAuthorization{
		ShortID:    12345,   // A unique identifier for the equipment
		Capacity:   1000000, // Storage capacity
		Debt:       2000000, // Current debt value
		Expiration: 2000,    // Expiry time for the equipment
	}

	// Sign the authorization request with GCA's private key.
	sb := ea.SigningBytes()
	ea.Signature = glow.Sign(sb, gcaPrivKey)

	// Convert the request body to JSON format.
	jsonBody, _ := json.Marshal(ea)
	// Perform an HTTP POST request to the authorize-equipment endpoint.
	resp, err := http.Post(fmt.Sprintf("http://127.0.0.1:%v/api/v1/authorize-equipment", server.httpPort), "application/json", bytes.NewBuffer(jsonBody))
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
	server, err = NewGCAServer(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(server.equipment) != 1 {
		t.Fatal("server did not receive equipment")
	}

	// Send a duplicate request. The server should ignore the request.
	resp, err = http.Post(fmt.Sprintf("http://127.0.0.1:%v/api/v1/authorize-equipment", server.httpPort), "application/json", bytes.NewBuffer(jsonBody))
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
	ea = glow.EquipmentAuthorization{
		ShortID:    12345,   // A unique identifier for the equipment
		Capacity:   1000000, // Storage capacity
		Debt:       2400000, // Current debt value
		Expiration: 2000,    // Expiry time for the equipment
	}
	// Sign the authorization request with GCA's private key.
	sb = ea.SigningBytes()
	ea.Signature = glow.Sign(sb, gcaPrivKey)
	// Convert the request body to JSON format.
	jsonBody, _ = json.Marshal(ea)
	// Perform an HTTP POST request to the authorize-equipment endpoint.
	resp, err = http.Post(fmt.Sprintf("http://127.0.0.1:%v/api/v1/authorize-equipment", server.httpPort), "application/json", bytes.NewBuffer(jsonBody))
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
	server, err = NewGCAServer(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(server.equipment) == 1 {
		t.Fatal("server did not ban equipment")
	}
	if len(server.equipmentBans) != 1 {
		t.Fatal("bad")
	}

	// Send a new request, this time with a new ShortID.
	ea = glow.EquipmentAuthorization{
		ShortID:    12346,   // A unique identifier for the equipment
		Capacity:   1000000, // Storage capacity
		Debt:       2400000, // Current debt value
		Expiration: 2000,    // Expiry time for the equipment
	}
	// Sign the authorization request with GCA's private key.
	sb = ea.SigningBytes()
	ea.Signature = glow.Sign(sb, gcaPrivKey)
	// Convert the request body to JSON format.
	jsonBody, _ = json.Marshal(ea)
	// Perform an HTTP POST request to the authorize-equipment endpoint.
	resp, err = http.Post(fmt.Sprintf("http://127.0.0.1:%v/api/v1/authorize-equipment", server.httpPort), "application/json", bytes.NewBuffer(jsonBody))
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
	server.CheckInvariants()
	server.Close()
	server, err = NewGCAServer(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	if len(server.equipment) != 1 {
		t.Fatal("bad")
	}
	if len(server.equipmentBans) != 1 {
		t.Fatal("bad")
	}

	// Test the hardware function quickly.
	_, _, err = server.submitNewHardware(1024, gcaPrivKey)
	if err != nil {
		t.Fatal(err)
	}
	server.CheckInvariants()
}
