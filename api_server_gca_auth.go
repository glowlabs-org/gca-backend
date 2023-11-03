package main

// This file provides an endpoint that can only be used one time by the GCA.
// The GCA will call this endpoint to report its public key, and then this
// server essentially becomes property of the GCA at that point.
//
// The server comes pre-installed with a temporary key that exists on the
// lockbook prior to the GCA generating their own keys. This temporary key is
// what is used to sign the message which relays the real key.
//
// This awkward double-key step is used because we don't want to leave the GCA
// server vulnerable to some rando taking ownership of it, but we also don't
// want to generate the GCA key until the lockbook is in the hands of the GCA,
// and we also don't want the GCA to have to worry about configuring the
// server.
//
// The setup that we have right now allows a technician to set up both the
// lockbook and the GCA server without ever having possession of the GCA keys,
// while allowing the GCA to generate their own keys the first time they log
// into their lockbook.
//
// This endpoint is a POST request.

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"

	"github.com/glowlabs-org/gca-backend/glow"
)

// RegisterGCARequest defines the inputs that need to be collected to secure
// the GCA key. The GCA key is a 32 byte public key. The signature is a 64 byte
// signature. They are presented using hex encodings.
type RegisterGCARequest struct {
	GCAKey    string `json:"GCAKey"`    // The 32-byte GCA public key, hex encoded
	Signature string `json:"Signature"` // The 64-byte signature, hex encoded
}

// GCAKey struct represents the GCA's public key and a signature to verify it.
type GCAKey struct {
	PublicKey glow.PublicKey
	Signature glow.Signature
}

// ToGCAKey converts a RegisterGCARequest to a GCAKey struct.
// It decodes the hex-encoded GCAKey and Signature.
func (req *RegisterGCARequest) ToGCAKey() (GCAKey, error) {
	if len(req.GCAKey) != 64 {
		return GCAKey{}, errors.New("GCAKey is of wrong length")
	}
	decodedGCAKey, err := hex.DecodeString(req.GCAKey)
	if err != nil {
		return GCAKey{}, err
	}

	if len(req.Signature) != 128 {
		return GCAKey{}, fmt.Errorf("signature is of wrong length: %v", len(req.Signature))
	}
	decodedSignature, err := hex.DecodeString(req.Signature)
	if err != nil {
		return GCAKey{}, err
	}

	gk := GCAKey{}
	copy(gk.PublicKey[:], decodedGCAKey)
	copy(gk.Signature[:], decodedSignature)

	return gk, nil
}

// SigningBytes generates the byte slice used for signing or verifying the GCAKey.
//
// This method excludes the Signature field from the byte slice and adds
// a "GCAKey" prefix. The returned byte slice is intended for the signing operation.
func (gk *GCAKey) SigningBytes() []byte {
	// Initialize the prefix string and convert it to a byte slice
	prefix := "GCAKey"
	prefixBytes := []byte(prefix)

	// Initialize a byte slice with sufficient length to hold the serialized PublicKey
	// and the length of prefixBytes for the "GCAKey" prefix.
	data := make([]byte, len(prefixBytes)+32) // 32 bytes for PublicKey

	// Copy the prefix and the public key into the byte slice.
	copy(data[0:len(prefixBytes)], prefixBytes)
	copy(data[len(prefixBytes):len(prefixBytes)+32], gk.PublicKey[:])

	return data
}

// RegisterGCAHandler handles the GCA registration requests.
// This function serves as the HTTP handler for GCA registration.
func (s *GCAServer) RegisterGCAHandler(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is supported.", http.StatusMethodNotAllowed)
		s.logger.Warn("Received non-POST request for GCA registration.")
		return
	}

	// Decode the JSON request body into RegisterGCARequest struct
	var request RegisterGCARequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		s.logger.Error("Failed to decode request body:", err)
		return
	}

	// Validate and process the request
	if err := s.registerGCA(request); err != nil {
		http.Error(w, fmt.Sprint("Failed to register GCA:", err), http.StatusInternalServerError)
		s.logger.Error("Failed to register GCA:", err)
		return
	}

	// Create a JSON object with the hex-encoded public key of the GCA server
	hexPublicKey := hex.EncodeToString(s.staticPublicKey[:])
	response := map[string]string{"ServerPublicKey": hexPublicKey}

	// Send the response as JSON with a status code of OK
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		// Handle the error if JSON encoding fails
		http.Error(w, "Failed to encode JSON response", http.StatusInternalServerError)
		s.logger.Error("Failed to encode JSON response:", err)
		return
	}

	// Log the successful registration
	s.logger.Info("Successfully registered GCA.")
}

// registerGCA performs the actual registration based on the client request.
// This function is responsible for the actual logic of registering the GCA.
func (s *GCAServer) registerGCA(req RegisterGCARequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.gcaPubkeyAvailable {
		return fmt.Errorf("a GCA key has already been registered")
	}

	// Parse and verify the GCA key
	gk, err := req.ToGCAKey()
	if err != nil {
		s.logger.Warn("Received bad GCA registration:", req)
		return fmt.Errorf("unable to convert to GCA key: %v", err)
	}
	err = s.verifyGCAKey(gk)
	if err != nil {
		s.logger.Warn("Received bad GCA key signature:", req)
		return fmt.Errorf("unable to verify GCA key: %v", err)
	}
	err = s.saveGCAKey(gk)
	if err != nil {
		s.logger.Warn("Unable to save GCA key:", gk)
		return fmt.Errorf("unable to save GCA key: %v", err)
	}
	return nil
}

// verifyGCAKey verifies the signature on a GCAKey object.
//
// It uses the public key of some authority (let's assume it's available in
// the server struct as someAuthorityPubkey) to verify the signature.
// The method returns an error if the verification fails.
func (gcas *GCAServer) verifyGCAKey(gk GCAKey) error {
	// Generate the byte slice intended for the signing operation
	signingBytes := gk.SigningBytes()

	// Perform the signature verification. Assume Verify is a function that exists
	// to verify the signature.
	isValid := glow.Verify(gcas.gcaTempKey, signingBytes, gk.Signature)

	// Check if the signature is valid
	if !isValid {
		return errors.New("invalid signature on GCAKey")
	}
	return nil
}

// saveGCAKey saves the GCA key to a file on disk.
//
// This function takes the GCA key, serialized as a byte array,
// and writes it to a file. The path for the file is determined by
// concatenating the server's base directory with "gca.pubkey".
func (server *GCAServer) saveGCAKey(gk GCAKey) error {
	// Determine the file path for the public key.
	// This should reside in the same directory as when reading it,
	// usually specified in server.baseDir.
	pubkeyPath := filepath.Join(server.baseDir, "gca.pubkey")

	// Serialize the GCA public key from the GCAKey struct.
	// In this case, we only need to save the public key, not the signature.
	serializedPubkey := gk.PublicKey[:]

	// Write the serialized public key to a file.
	// ioutil.WriteFile creates the file if it doesn't exist,
	// or truncates it before writing if it does.
	err := ioutil.WriteFile(pubkeyPath, serializedPubkey, 0644)
	if err != nil {
		return fmt.Errorf("unable to write public key to file: %v", err)
	}

	server.gcaPubkey = gk.PublicKey
	server.gcaPubkeyAvailable = true
	return nil
}
