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

// TestAuthorizeEquipmentEndpoint is a test function that verifies the functionality
// of the Authorize Equipment Endpoint in the GCA Server.
func TestAuthorizeEquipmentEndpoint(t *testing.T) {
	server, _, _, gcaPrivKey, err := SetupTestEnvironment(t.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	// Create a mock request for equipment authorization.
	ea := glow.EquipmentAuthorization{
		ShortID:    12345,
		Capacity:   1000000,
		Debt:       2000000,
		Expiration: 2000,
	}

	// Sign the authorization request with GCA's private key.
	sb := ea.SigningBytes()
	ea.Signature = glow.Sign(sb, gcaPrivKey)

	// Convert the request body to JSON.
	jsonBody, _ := json.Marshal(ea)

	// Perform an HTTP POST request.
	resp, err := http.Post(fmt.Sprintf("http://localhost:%v/api/v1/authorize-equipment", server.httpPort), "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Check the HTTP status code.
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		t.Fatalf("Expected status 200, but got %d. Response: %s", resp.StatusCode, string(bodyBytes))
	}

	// Decode the JSON response.
	var response map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Validate the server's response.
	if status, exists := response["status"]; !exists || status != "success" {
		t.Fatalf("Unexpected response: %v", response)
	}
}

// loadEquipmentAuths is responsible for populating the equipment map
// using the provided array of EquipmentAuths.
func (gcas *GCAServer) loadEquipmentAuth(ea glow.EquipmentAuthorization) {
	// Add the equipment's public key to the equipment map using its ShortID as the key
	gcas.equipment[ea.ShortID] = ea
	gcas.equipmentReports[ea.ShortID] = new([4032]glow.EquipmentReport)
	gcas.addRecentEquipmentAuth(ea)
}

// Perform an integration test for the equipment authorizations.
func TestVerifyEquipmentAuthorization(t *testing.T) {
	server, _, _, gcaPrivateKey, err := SetupTestEnvironment(t.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	// Create and sign a valid EquipmentAuthorization
	ea := glow.EquipmentAuthorization{
		ShortID:    1,
		PublicKey:  [32]byte{1},
		Capacity:   100,
		Debt:       0,
		Expiration: 1000,
	}
	signingBytes := ea.SigningBytes()
	ea.Signature = glow.Sign(signingBytes, gcaPrivateKey)

	// Test case 1: Valid EquipmentAuthorization should pass verification
	if err := server.verifyEquipmentAuthorization(ea); err != nil {
		t.Errorf("Failed to verify a valid EquipmentAuthorization: %v", err)
	}

	// Create and sign an invalid EquipmentAuthorization
	eaInvalid := glow.EquipmentAuthorization{
		ShortID:    2,
		PublicKey:  [32]byte{2},
		Capacity:   200,
		Debt:       50,
		Expiration: 2000,
	}
	eaInvalidBytes := eaInvalid.SigningBytes()
	ea.Signature = glow.Sign(eaInvalidBytes, gcaPrivateKey)

	// Tamper with the EquipmentAuthorization to make it invalid
	eaInvalid.Debt = 100

	// Test case 2: Invalid EquipmentAuthorization should fail verification
	if err := server.verifyEquipmentAuthorization(eaInvalid); err == nil {
		t.Errorf("Verified an invalid EquipmentAuthorization without error")
	}
}
