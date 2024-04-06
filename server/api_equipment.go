package server

import (
	"encoding/json"
	"net/http"

	"github.com/glowlabs-org/gca-backend/glow"
)

type EquipmentResponse struct {
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
		EquipmentDetails: make(map[uint32]glow.EquipmentAuthorization),
	}
	gcas.mu.Lock()
	for k, v := range gcas.equipment {
		er.EquipmentDetails[k] = v
	}
	gcas.mu.Unlock()

	// Write the response
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(er); err != nil {
		http.Error(w, "Failed to encode JSON response: "+err.Error(), http.StatusInternalServerError)
		return
	}
}
