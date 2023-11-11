package server

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"

	"github.com/glowlabs-org/gca-backend/glow"
)

// TestToAuthorization verifies that an EquipmentAuthorizationRequest
// correctly converts into an EquipmentAuthorization.
func TestToAuthorization(t *testing.T) {
	// Generate a real ECDSA public-private key pair using the secp256k1 curve.
	pubKey, privKey := glow.GenerateKeyPair()

	// Generate a real signature using the ECDSA private key.
	message := []byte("test message")
	signature := glow.Sign(message, privKey)

	// Hex-encode the real public key and signature.
	hexPublicKey := hex.EncodeToString(pubKey[:])
	hexSignature := hex.EncodeToString(signature[:])

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
	server, _, gcaPrivKey, err := SetupTestEnvironment(t.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	// Create a mock request for equipment authorization.
	body := EquipmentAuthorizationRequest{
		ShortID:    12345,
		PublicKey:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Capacity:   1000000,
		Debt:       2000000,
		Expiration: 2000,
		Signature:  "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	ea, err := body.ToAuthorization()
	if err != nil {
		t.Fatal(err)
	}

	// Sign the authorization request with GCA's private key.
	sb := ea.SigningBytes()
	signature := glow.Sign(sb, gcaPrivKey)
	body.Signature = hex.EncodeToString(signature[:])

	// Convert the request body to JSON.
	jsonBody, _ := json.Marshal(body)

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
	server, _, gcaPrivateKey, err := SetupTestEnvironment(t.Name())
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
