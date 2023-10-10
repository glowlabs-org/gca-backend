// This Go package serves as the main entry point for our API server.
package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
)

// EquipmentAuthorizationRequest is a struct that maps the JSON request payload.
type EquipmentAuthorizationRequest struct {
	ShortID    uint32 `json:"ShortID"`
	PublicKey  string `json:"Public Key"`
	Capacity   uint64 `json:"Capacity"`
	Debt       uint64 `json:"Debt"`
	Expiration uint32 `json:"Expiration"`
	Signature  string `json:"Signature"`
}

// AuthorizeEquipmentHandler handles the authorization requests for equipment.
func (gca *GCAServer) AuthorizeEquipmentHandler(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is supported.", http.StatusMethodNotAllowed)
		gca.logger.Warn("Received non-POST request for equipment authorization.")
		return
	}

	// Decode the JSON request body into EquipmentAuthorizationRequest struct
	var request EquipmentAuthorizationRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		gca.logger.Error("Failed to decode request body: ", err)
		return
	}

	// Validate and process the request
	if err := gca.authorizeEquipment(request); err != nil {
		http.Error(w, "Failed to authorize equipment.", http.StatusInternalServerError)
		gca.logger.Error("Failed to authorize equipment: ", err)
		return
	}

	// Send a success response
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	gca.logger.Info("Successfully authorized equipment.")
}

// authorizeEquipment performs the actual authorization based on the client request.
func (gca *GCAServer) authorizeEquipment(req EquipmentAuthorizationRequest) error {
	// Decode the hex-encoded public key
	pubKeyBytes, err := hex.DecodeString(req.PublicKey)
	if err != nil {
		gca.logger.Error("Failed to decode public key: ", err)
		return err
	}

	// Create a data buffer for signature verification
	data := []byte(fmt.Sprintf("%d", req.ShortID))
	data = append(data, []byte(req.PublicKey)...)
	data = append(data, []byte(fmt.Sprintf("%d", req.Capacity))...)
	data = append(data, []byte(fmt.Sprintf("%d", req.Debt))...)
	data = append(data, []byte(fmt.Sprintf("%d", req.Expiration))...)

	// Decode the hex-encoded signature
	signatureBytes, err := hex.DecodeString(req.Signature)
	if err != nil {
		gca.logger.Error("Invalid signature format.")
		return err
	}

	// Verify the signature
	if !ed25519.Verify(gca.gcaPubkey, data, signatureBytes) {
		gca.logger.Warn("Invalid signature for equipment authorization.")
		return fmt.Errorf("Invalid signature")
	}

	// Check for duplicate authorizations
	existingKey, exists := gca.deviceKeys[req.ShortID]
	if exists && string(existingKey) != string(pubKeyBytes) {
		delete(gca.deviceKeys, req.ShortID)
		gca.logger.Warn("Duplicate authorization detected, removing.")
	} else {
		gca.deviceKeys[req.ShortID] = pubKeyBytes
		gca.logger.Info("Added new device for authorization.")
	}

	return nil
}

// launchAPI sets up the HTTP API endpoints and starts the HTTP server.
func (gca *GCAServer) launchAPI() {
	gca.mux.HandleFunc("/api/v1/authorize-equipment", gca.AuthorizeEquipmentHandler)
	go func() {
		gca.logger.Info("Starting HTTP server on port 35015...")
		if err := gca.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			gca.logger.Fatal("Could not start HTTP server: ", err)
		}
	}()
}
