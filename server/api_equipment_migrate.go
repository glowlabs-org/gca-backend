package server

// This file contains an endpoint which allows the GCA to authorize a new piece
// of equipment. That equipment will then be able to submit power recording
// requests to the server.

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/glowlabs-org/gca-backend/glow"
)

// EquipmentMigrationRequest contains the fields that have an order from the
// GCA to migrate equipment.
//
// Note: the authorized servers need to be individually validated by the new
// gca, but the struct as a whole needs to be validated by the current GCA.
type EquipmentMigration struct {
	Equipment  glow.PublicKey     // Which equipment is being migrated
	NewGCA     glow.PublicKey     // What GCA it's being migrated to
	NewShortID uint32             // What the new ShortID is for the equipment
	NewServers []AuthorizedServer // A list of servers for the new GCA (signed by the new GCA)
	Signature  glow.Signature     // A signature from the current GCA
}

// Serialize will return a set of bytes that can be sent over the wire.
func (em EquipmentMigration) Serialize() []byte {
	result := make([]byte, 68)
	copy(result, em.Equipment[:])
	copy(result[32:], em.NewGCA[:])
	binary.LittleEndian.PutUint32(result[64:], em.NewShortID)
	// Get the serialization of all the authorized servers.
	for _, as := range em.NewServers {
		result = append(result, as.Serialize()...)
	}
	result = append(result, em.Signature[:]...)
	return result
}

// SigningBytes returns the data that should be signed by the
// EquipmentMigration.
func (em EquipmentMigration) SigningBytes() []byte {
	// The SigningBytes are just the serialized bytes minus the signature
	// plus a special prefix.
	result := em.Serialize()
	return append([]byte("EquipmentMigration"), result[:len(result)-64]...)
}

// EquipmentMigrateHandler handles an API request from the GCA to move a piece
// of equipment to a new GCA. Typically all equipment will be moved at once,
// but the requests need to be made one at a time.
func (gca *GCAServer) EquipmentMigrateHandler(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is supported.", http.StatusMethodNotAllowed)
		gca.logger.Warn("Received non-POST request for equipment migration.")
		return
	}

	// Decode the JSON request body into the authorization.
	var request EquipmentMigration
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		gca.logger.Error("Failed to decode request body: ", err)
		return
	}

	// Validate and process the request
	err := gca.managedValidateMigration(request)
	if err != nil {
		http.Error(w, fmt.Sprint("Failed to authorize equipment:", err), http.StatusInternalServerError)
		gca.logger.Error("Failed to authorize equipment: ", err)
		return
	}

	// Now that the migration request has been validated, add it to the
	// equipment migration list.
	gca.mu.Lock()
	gca.equipmentMigrations[request.Equipment] = request
	gca.mu.Unlock()

	// TODO: Need to persist the list of migrations, and write tests to
	// ensure that the persistence is functioning.

	// Send a success response
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	gca.logger.Info("Successfully authorized equipment.")
}

// managedValidateMigration will check that the EquipmentMigration is valid.
// The whole struct needs to be signed by the GCA for this server, and each of
// the AuthorizedServers in the struct need to be signed by the new GCA that
// the equipment is being migrated to.
func (gcas *GCAServer) managedValidateMigration(em EquipmentMigration) error {
	// Check the signature on the entire migration.
	sb := em.SigningBytes()
	if !glow.Verify(gcas.gcaPubkey, sb, em.Signature) {
		return fmt.Errorf("invalid signature on equipment migration")
	}

	// Check the signature on each authorized server. Remember that the
	// signatures on these needs to be from the new GCA.
	for _, as := range em.NewServers {
		sb := as.SigningBytes()
		if !glow.Verify(em.NewGCA, sb, as.GCAAuthorization) {
			return fmt.Errorf("invalid signature on authorized server within equipment migration")
		}
	}

	// Everything checks out.
	return nil
}
