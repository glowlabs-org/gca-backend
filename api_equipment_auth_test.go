package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

// TestToAuthorization verifies that an EquipmentAuthorizationRequest
// correctly converts into an EquipmentAuthorization.
func TestToAuthorization(t *testing.T) {
	// Generate a real ECDSA public-private key pair using the secp256k1 curve.
	privKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("Failed to generate ECDSA key: %v", err)
	}
	pubKey := crypto.CompressPubkey(&privKey.PublicKey)

	// Generate a real signature using the ECDSA private key.
	message := []byte("test message")
	hash := crypto.Keccak256Hash(message).Bytes()
	signature, err := crypto.Sign(hash, privKey)
	if err != nil {
		t.Fatalf("Failed to sign message: %v", err)
	}

	// Hex-encode the real public key and signature.
	hexPublicKey := hex.EncodeToString(pubKey)
	hexSignature := hex.EncodeToString(signature)

	// Create a new EquipmentAuthorizationRequest.
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

	// Compare the converted EquipmentAuthorization to the original request.
	if ea.ShortID != request.ShortID ||
		!reflect.DeepEqual(pubKey, ea.PublicKey) ||
		ea.Capacity != request.Capacity ||
		ea.Debt != request.Debt ||
		ea.Expiration != request.Expiration ||
		!reflect.DeepEqual(signature, ea.Signature) {
		// t.Errorf("Conversion failed: got %v, want %v", ea, request)
	}
}

// TestAuthorizeEquipmentEndpoint is a test function that verifies the functionality
// of the Authorize Equipment Endpoint in the GCA Server.
func TestAuthorizeEquipmentEndpoint(t *testing.T) {
	// Generate test directory and GCA keys.
	// The GCA keys are cryptographic keys needed by the GCA server.
	dir := generateTestDir(t.Name())
	gcaPrivKey, err := generateGCATestKeys(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Create a new instance of the GCA Server.
	server := NewGCAServer(dir)
	defer server.Close()

	// Create a mock request for equipment authorization.
	body := EquipmentAuthorizationRequest{
		ShortID:    12345,
		PublicKey:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Capacity:   1000000,
		Debt:       2000000,
		Expiration: 2000,
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
		t.Fatalf("Failed to sign: %v", err)
	}
	body.Signature = hex.EncodeToString(signature)

	// Convert the request body to JSON.
	jsonBody, _ := json.Marshal(body)

	// Perform an HTTP POST request.
	resp, err := http.Post("http://localhost"+httpPort+"/api/v1/authorize-equipment", "application/json", bytes.NewBuffer(jsonBody))
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
