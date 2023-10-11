package main

import (
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

// ToAuthorization converts an EquipmentAuthorizationRequest to an EquipmentAuthorization.
// It decodes the hex-encoded PublicKey and Signature.
func (req *EquipmentAuthorizationRequest) ToAuthorization() (EquipmentAuthorization, error) {
	decodedPublicKey, err := hex.DecodeString(req.PublicKey)
	if err != nil {
		return EquipmentAuthorization{}, err
	}

	decodedSignature, err := hex.DecodeString(req.Signature)
	if err != nil {
		return EquipmentAuthorization{}, err
	}

	return EquipmentAuthorization{
		ShortID:    req.ShortID,
		PublicKey:  decodedPublicKey,
		Capacity:   req.Capacity,
		Debt:       req.Debt,
		Expiration: req.Expiration,
		Signature:  decodedSignature,
	}, nil
}

// AuthorizeEquipmentHandler handles the authorization requests for equipment.
// This function serves as the HTTP handler for equipment authorization.
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
// This function is responsible for the actual logic of authorizing the equipment.
func (gcas *GCAServer) authorizeEquipment(req EquipmentAuthorizationRequest) error {
	// Parse and verify the authorization
	auth, err := req.ToAuthorization()
	if err != nil {
		gcas.logger.Warn("Received bad equipment authorization", req)
		return fmt.Errorf("unable to convert to normal authorization: %v", err)
	}
	err = gcas.verifyEquipmentAuthorization(auth)
	if err != nil {
		gcas.logger.Warn("Received bad equipment authorization signature", req)
		return fmt.Errorf("unable to verify authorization: %v", err)
	}

	// TODO: Check banlist here. Abort if on banlist.

	// TODO: Need to handle duplicates here. If there is a duplicate, we have
	// to add this auth to the banlist along with the dual proofs. Duplicate
	// means two *different* auths that are both signed.

	// Add the equipment to the server.
	gcas.equipment[auth.ShortID] = auth
	return nil
}
