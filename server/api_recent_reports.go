package server

// TODO: The code here does not align with standards.

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/glowlabs-org/gca-backend/glow"
)

// RecentReportsResponse defines the output containing the equipment reports and a signature.
type RecentReportsResponse struct {
	Reports        [4032]glow.EquipmentReport `json:"Reports"`        // Array of equipment reports
	TimeslotOffset uint32                     `json:"TimeslotOffset"` // The timeslot offset of the first report
	Signature      glow.Signature             `json:"Signature"`      // Signature of the GCA server
}

// RecentReportsHandler handles requests for fetching the most recent equipment reports.
func (s *GCAServer) RecentReportsHandler(w http.ResponseWriter, r *http.Request) {
	// Only accept GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET method is supported.", http.StatusMethodNotAllowed)
		s.logger.Warn("Received non-GET request for recent reports.")
		return
	}

	// Retrieve the public key from query parameters
	publicKeyHex := r.URL.Query().Get("publicKey")
	if publicKeyHex == "" {
		http.Error(w, "Public key is required as a query parameter", http.StatusBadRequest)
		s.logger.Warn("Public key not provided in query parameters")
		return
	}
	publicKeyBytes, err := hex.DecodeString(publicKeyHex)
	if err != nil {
		http.Error(w, "Invalid public key format", http.StatusBadRequest)
		s.logger.Error("Failed to decode public key:", err)
		return
	}
	var publicKey glow.PublicKey
	if len(publicKeyBytes) != len(publicKey) {
		http.Error(w, "Invalid public key length", http.StatusBadRequest)
		s.logger.Warn("Invalid public key length")
		return
	}
	copy(publicKey[:], publicKeyBytes)

	// Fetch the equipment reports and generate a signature
	response, err := s.getRecentReportsWithSignature(publicKey)
	if err != nil {
		http.Error(w, fmt.Sprint("Failed to fetch equipment reports:", err), http.StatusInternalServerError)
		s.logger.Error("Failed to fetch equipment reports:", err)
		return
	}

	// Send the response as JSON with a status code of OK
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode JSON response", http.StatusInternalServerError)
		s.logger.Error("Failed to encode JSON response:", err)
		return
	}
}

// getRecentReportsWithSignature fetches the 4032 most recent equipment reports and signs the response.
func (s *GCAServer) getRecentReportsWithSignature(publicKey glow.PublicKey) (RecentReportsResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Convert the public key to a ShortID for lookup
	shortID, exists := s.equipmentShortID[publicKey]
	if !exists {
		return RecentReportsResponse{}, fmt.Errorf("equipment not found")
	}
	reports, exists := s.equipmentReports[shortID]
	if !exists {
		return RecentReportsResponse{}, fmt.Errorf("no reports found for the provided public key")
	}

	// Serialize the reports for signing
	reportsBytes, err := json.Marshal(reports)
	if err != nil {
		return RecentReportsResponse{}, fmt.Errorf("failed to serialize reports: %v", err)
	}

	// Sign the serialized reports
	sig := glow.Sign(reportsBytes, s.staticPrivateKey)
	return RecentReportsResponse{
		Reports:   *reports,
		Signature: sig,
	}, nil
}
