package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/glowlabs-org/gca-backend/glow"
)

type EquipmentResponse struct {
	EquipmentList    map[string]uint32
	EquipmentDetails map[uint32]glow.EquipmentAuthorization
}

// EquipmentHandler returns a list of all equipment, the corresponding
// ShortIDs, and the stats for each piece.
//
// TODO: API endpoint is currently unsigned, and therefore unofficial. The
// signing step was skipped because we needed info in prod, we'll come back to
// it shortly.
func (gcas *GCAServer) EquipmentHandler(w http.ResponseWriter, r *http.Request) {
	// Restrict to GET calls.
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET method is allowed", http.StatusMethodNotAllowed)
		return
	}

	// Fill out the EquipmentResponse with data from the server.
	er := EquipmentResponse{
		EquipmentList:    make(map[string]uint32),
		EquipmentDetails: make(map[uint32]glow.EquipmentAuthorization),
	}
	gcas.mu.Lock()
	for k, v := range gcas.equipmentShortID {
		er.EquipmentList[fmt.Sprint(k)] = v
	}
	for k, v := range gcas.equipment {
		er.EquipmentDetails[k] = v
	}
	gcas.mu.Unlock()

	// Marshal the EquipmentResponse to JSON
	_, err := json.Marshal(er)
	fmt.Println(err)
	if err != nil {
		http.Error(w, "Failed to encode JSON response: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Write the response
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(er); err != nil {
		http.Error(w, "Failed to encode JSON response: "+err.Error(), http.StatusInternalServerError)
		return
	}
}
