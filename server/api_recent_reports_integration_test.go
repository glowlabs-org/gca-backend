package server

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/glowlabs-org/gca-backend/glow"
)

// TestRecentReportsIntegration checks that the full flow for fetching recent equipment reports works as intended.
func TestRecentReportsIntegration(t *testing.T) {
	server, _, _, gcaPrivKey, err := SetupTestEnvironment(t.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	// Set up a piece of equipment and generate some reports for testing
	ea, equipmentKey, err := server.submitNewHardware(12345, gcaPrivKey)
	if err != nil {
		t.Fatal("Failed to submit new hardware: ", err)
	}

	// Generate some reports for the equipment (this part might vary based on how reports are generated in your system)
	err = server.sendEquipmentReportSpecific(ea, equipmentKey, 4, 45)
	if err != nil {
		t.Fatal("Failed to generate reports for testing: ", err)
	}

	// Perform a GET request to the recent-reports endpoint
	pubkey := hex.EncodeToString(ea.PublicKey[:])
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%v/api/v1/recent-reports?publicKey=%s", server.httpPort, pubkey))
	if err != nil {
		t.Fatalf("Failed to send GET request: %v", err)
	}
	defer resp.Body.Close()

	// Check if the HTTP status code is OK (200)
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		t.Fatalf("Expected status 200, but got %d. Response: %s", resp.StatusCode, string(bodyBytes))
	}

	// Decode the JSON response
	var response RecentReportsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	rj, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	t.Log("RecentReportsResponse size:", len(rj), "bytes")

	// Validate the response
	if len(response.Reports) != 4032 {
		t.Fatalf("Unexpected number of reports: got %d, want 4032", len(response.Reports))
	}
	var sig glow.Signature
	if response.Signature == sig {
		t.Fatal("Signature is missing in the response")
	}
	// Verify the signature.
	signBytes, err := json.Marshal(response.Reports)
	if err != nil {
		t.Fatal(err)
	}
	if !glow.Verify(server.staticPublicKey, signBytes, response.Signature) {
		t.Fatal("signature mismatch")
	}

	// Ensure that only the fourth report has a value in it
	for i, report := range response.Reports {
		if i == 4 && report.PowerOutput != 45 {
			t.Fatal("wrong")
		} else if i != 4 && report.PowerOutput != 0 {
			t.Fatal("bad power", i, report.PowerOutput)
		}
	}

	// Perform a GET request with an invalid public key
	invalidPublicKey := "invalidPublicKey"
	resp, err = http.Get(fmt.Sprintf("http://127.0.0.1:%v/api/v1/recent-reports?publicKey=%s", server.httpPort, invalidPublicKey))
	if err != nil {
		t.Fatalf("Failed to send GET request: %v", err)
	}
	defer resp.Body.Close()

	// Expect a bad request response
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid public key, but got %d", resp.StatusCode)
	}

	// Set up a piece of equipment without generating reports
	ea, _, err = server.submitNewHardware(12346, gcaPrivKey)
	if err != nil {
		t.Fatal("Failed to submit new hardware: ", err)
	}

	// Perform a GET request for the equipment with no reports
	pubkey = hex.EncodeToString(ea.PublicKey[:])
	resp, err = http.Get(fmt.Sprintf("http://127.0.0.1:%v/api/v1/recent-reports?publicKey=%s", server.httpPort, pubkey))
	if err != nil {
		t.Fatalf("Failed to send GET request: %v", err)
	}
	defer resp.Body.Close()
	// Expect no issues
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		t.Fatalf("Expected status 200, but got %d. Response: %s", resp.StatusCode, string(bodyBytes))
	}

	// Decode the JSON response
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	// Ensure only all reports are blank.
	for i, report := range response.Reports {
		if report.PowerOutput != 0 {
			t.Fatal("bad power", i, report.PowerOutput)
		}
	}
}
