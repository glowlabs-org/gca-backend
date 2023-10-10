package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
)

type EquipmentAuthorizationRequest struct {
	ShortID    uint32 `json:"ShortID"`
	PublicKey  string `json:"Public Key"`
	Capacity   uint64 `json:"Capacity"`
	Debt       uint64 `json:"Debt"`
	Expiration uint32 `json:"Expiration"`
	Signature  string `json:"Signature"`
}

func (gca *GCAServer) AuthorizeEquipmentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is supported.", http.StatusMethodNotAllowed)
		return
	}

	// Parse the request body into EquipmentAuthorizationRequest
	var request EquipmentAuthorizationRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate and process the authorization request
	if err := gca.authorizeEquipment(request); err != nil {
		http.Error(w, fmt.Sprintf("Failed to authorize equipment: %v", err), http.StatusInternalServerError)
		return
	}

	// Send success response
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func (gca *GCAServer) authorizeEquipment(req EquipmentAuthorizationRequest) error {
	// Convert hex-encoded public key and signature to bytes
	pubKeyBytes, err := hex.DecodeString(req.PublicKey)
	if err != nil {
		return fmt.Errorf("failed to decode public key: %v", err)
	}

	// Create a data buffer from the provided parameters for signature verification
	data := []byte(fmt.Sprintf("%d", req.ShortID))
	data = append(data, []byte(req.PublicKey)...)
	data = append(data, []byte(fmt.Sprintf("%d", req.Capacity))...)
	data = append(data, []byte(fmt.Sprintf("%d", req.Debt))...)
	data = append(data, []byte(fmt.Sprintf("%d", req.Expiration))...)

	signatureBytes, err := hex.DecodeString(req.Signature)
	if err != nil {
		return fmt.Errorf("Invalid signature format")
	}

	// Verify the signature
	if !ed25519.Verify(gca.gcaPubkey, data, signatureBytes) {
		return fmt.Errorf("Invalid signature")
	}

	// Check for duplicate authorization with different public keys
	existingKey, exists := gca.deviceKeys[req.ShortID]
	if exists && string(existingKey) != string(pubKeyBytes) {
		// Mark both as banned. For simplicity, we will just remove them.
		// You can implement more complex banning logic if required.
		delete(gca.deviceKeys, req.ShortID)
	} else {
		gca.deviceKeys[req.ShortID] = pubKeyBytes
	}

	return nil
}

func (gca *GCAServer) startAPI() {
	gca.mux.HandleFunc("/api/v1/authorize-equipment", gca.AuthorizeEquipmentHandler)
	go func() {
		fmt.Println("Starting HTTP server on port 35015...")
		if err := gca.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Could not start HTTP server: %v", err)
		}
	}()
}
