package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
	"time"
)

func TestAuthorizeEquipmentEndpoint(t *testing.T) {
	server := NewGCAServer()
	defer server.Close() // Ensure resources are cleaned up after the test.

	// Create a GCA key pair
	gcaPubKey, gcaPrivKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate GCA key pair: %v", err)
	}
	server.LoadGCAPubkey(gcaPubKey)

	time.Sleep(100 * time.Millisecond) // Give the server a bit of time to start.

	// Create a mock request
	body := EquipmentAuthorizationRequest{
		ShortID:    12345,
		PublicKey:  "abcd1234",
		Capacity:   1000000,
		Debt:       2000000,
		Expiration: 2000,
		Signature:  "efgh5678",
	}

	// Sign the authorization
	data := []byte(fmt.Sprintf("%d", body.ShortID))
	data = append(data, []byte(body.PublicKey)...)
	data = append(data, []byte(fmt.Sprintf("%d", body.Capacity))...)
	data = append(data, []byte(fmt.Sprintf("%d", body.Debt))...)
	data = append(data, []byte(fmt.Sprintf("%d", body.Expiration))...)
	signature := ed25519.Sign(gcaPrivKey, data)
	body.Signature = hex.EncodeToString(signature)

	// Convert request to JSON
	jsonBody, _ := json.Marshal(body)
	resp, err := http.Post("http://localhost:35015/api/v1/authorize-equipment", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Check for successful response
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		t.Fatalf("Expected status 200, but got %d. Response: %s", resp.StatusCode, string(bodyBytes))
	}

	var response map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if status, exists := response["status"]; !exists || status != "success" {
		t.Fatalf("Unexpected response: %v", response)
	}
}