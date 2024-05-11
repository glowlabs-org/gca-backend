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

// TestAuthorizedServers writes an integration test to make sure the authorized
// servers endpoints are working.
func TestAuthorizeServers(t *testing.T) {
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
	resp, err := http.Post(fmt.Sprintf("http://127.0.0.1:%v/api/v1/authorize-equipment", server.httpPort), "application/json", bytes.NewBuffer(jsonBody))
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
