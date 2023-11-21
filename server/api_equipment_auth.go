package server

// This file contains an endpoint which allows the GCA to authorize a new piece
// of equipment. That equipment will then be able to submit power recording
// requests to the server.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/glowlabs-org/gca-backend/glow"
)

// AuthorizeEquipmentHandler handles the authorization requests for equipment.
// This function serves as the HTTP handler for equipment authorization.
func (gca *GCAServer) AuthorizeEquipmentHandler(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is supported.", http.StatusMethodNotAllowed)
		gca.logger.Warn("Received non-POST request for equipment authorization.")
		return
	}

	// Decode the JSON request body into the authorization.
	var request glow.EquipmentAuthorization
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		gca.logger.Error("Failed to decode request body: ", err)
		return
	}

	// Validate and process the request
	isNew, err := gca.managedAuthorizeEquipment(request)
	if err != nil {
		http.Error(w, fmt.Sprint("Failed to authorize equipment:", err), http.StatusInternalServerError)
		gca.logger.Error("Failed to authorize equipment: ", err)
		return
	}

	// Now that the equipment has been verified, send the authorization to
	// all other known servers. This code running on every GCA server will
	// result in a total of n^2 messages being sent, but that's okay
	// because new equipment is a pretty big deal and we want to be sure
	// that every GCA server recognizes every piece of equipment.
	//
	// If a particular GCA server is offline at the time that a device is
	// authorized, that GCA server is going to miss the device, so standard
	// sychronization calls still need to be in place. But this is a good
	// starting point.
	if !isNew {
		// Don't send equipment to the other servers unless this
		// equipment is new to us.
		return
	}
	gca.gcaServers.mu.Lock()
	ass := make([]AuthorizedServer, len(gca.gcaServers.servers))
	copy(ass, gca.gcaServers.servers)
	gca.gcaServers.mu.Unlock()
	for _, as := range ass {
		jsonBody, _ := json.Marshal(request)
		resp, err := http.Post(fmt.Sprintf("http://%v:%v/api/v1/authorize-equipment", as.Location, as.HttpPort), "application/json", bytes.NewBuffer(jsonBody))
		if err != nil {
			gca.logger.Infof("unable to send http request to submit new hardware: %v", err)
		}
		resp.Body.Close()
	}

	// Send a success response
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	gca.logger.Info("Successfully authorized equipment.")
}

// managedAuthorizeEquipment performs the actual authorization based on the client request.
// This function is responsible for the actual logic of authorizing the equipment.
func (gcas *GCAServer) managedAuthorizeEquipment(auth glow.EquipmentAuthorization) (bool, error) {
	gcas.mu.Lock()
	defer gcas.mu.Unlock()
	if !gcas.gcaPubkeyAvailable {
		return false, fmt.Errorf("this gca server has not yet been initialized by the GCA")
	}

	err := gcas.verifyEquipmentAuthorization(auth)
	if err != nil {
		gcas.logger.Warn("Received bad equipment authorization signature", auth)
		return false, fmt.Errorf("unable to verify authorization: %v", err)
	}
	isNew, err := gcas.saveEquipment(auth)
	if err != nil {
		gcas.logger.Warn("Unable to save equipment:", auth)
		return false, fmt.Errorf("unable to save equipment: %v", err)
	}
	return isNew, nil
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
