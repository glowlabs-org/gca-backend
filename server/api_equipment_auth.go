package server

// This file contains an endpoint which allows the GCA to authorize a new piece
// of equipment. That equipment will then be able to submit power recording
// requests to the server.

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/glowlabs-org/gca-backend/glow"
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
func (req *EquipmentAuthorizationRequest) ToAuthorization() (glow.EquipmentAuthorization, error) {
	if len(req.PublicKey) != 64 {
		return glow.EquipmentAuthorization{}, errors.New("public key is wrong length")
	}
	decodedPublicKey, err := hex.DecodeString(req.PublicKey)
	if err != nil {
		return glow.EquipmentAuthorization{}, err
	}

	if len(req.Signature) != 128 {
		return glow.EquipmentAuthorization{}, fmt.Errorf("signature is wrong length: %v", len(req.Signature))
	}
	decodedSignature, err := hex.DecodeString(req.Signature)
	if err != nil {
		return glow.EquipmentAuthorization{}, err
	}

	ea := glow.EquipmentAuthorization{
		ShortID:    req.ShortID,
		Capacity:   req.Capacity,
		Debt:       req.Debt,
		Expiration: req.Expiration,
	}
	copy(ea.PublicKey[:], decodedPublicKey)
	copy(ea.Signature[:], decodedSignature)

	return ea, nil
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
	if err := gca.managedAuthorizeEquipment(request); err != nil {
		http.Error(w, fmt.Sprint("Failed to authorize equipment:", err), http.StatusInternalServerError)
		gca.logger.Error("Failed to authorize equipment: ", err)
		return
	}

	// Send a success response
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	gca.logger.Info("Successfully authorized equipment.")
}

// managedAuthorizeEquipment performs the actual authorization based on the client request.
// This function is responsible for the actual logic of authorizing the equipment.
func (gcas *GCAServer) managedAuthorizeEquipment(req EquipmentAuthorizationRequest) error {
	gcas.mu.Lock()
	defer gcas.mu.Unlock()
	if !gcas.gcaPubkeyAvailable {
		return fmt.Errorf("this gca server has not yet been initialized by the GCA")
	}

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
	err = gcas.saveEquipment(auth)
	if err != nil {
		gcas.logger.Warn("Unable to save equipment:", auth)
		return fmt.Errorf("unable to save equipment: %v", err)
	}
	return nil
}

// verifyEquipmentAuthorization checks the validity of the signature on an EquipmentAuthorization.
//
// It uses the public key of the Grid Control Authority (gcaPubkey) to verify the signature.
// The method returns an error if the verification fails.
func (gcas *GCAServer) verifyEquipmentAuthorization(ea glow.EquipmentAuthorization) error {
	signingBytes := ea.SigningBytes()
	isValid := glow.Verify(gcas.gcaPubkey, signingBytes, ea.Signature)
	if !isValid {
		return errors.New("invalid signature on EquipmentAuthorization")
	}
	return nil
}
