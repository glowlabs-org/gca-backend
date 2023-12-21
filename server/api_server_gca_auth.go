package server

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
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"

	"github.com/glowlabs-org/gca-backend/glow"
)

// GCARegistration defines a request from the GCA to register itself on a
// server. The GCAKey is the GCA's real public key. The signature is signed by
// the GCA temp key which was installed on the machine before the GCA received
// their lockbook.
type GCARegistration struct {
	GCAKey    glow.PublicKey
	Signature glow.Signature
}

// GCARegistrationResponse defines the response that the server writes after a
// successful GCA registration.
type GCARegistrationResponse struct {
	PublicKey glow.PublicKey
	HttpPort  uint16
	TcpPort   uint16
	UdpPort   uint16
}

// SigningBytes generates the byte slice used for signing or verifying the GCAKey.
//
// This method excludes the Signature field from the byte slice and adds
// a "GCAKey" prefix. The returned byte slice is intended for the signing operation.
func (gr *GCARegistration) SigningBytes() []byte {
	prefix := "GCARegistration"
	prefixBytes := []byte(prefix)

	data := make([]byte, len(prefixBytes)+32)
	copy(data[0:len(prefixBytes)], prefixBytes)
	copy(data[len(prefixBytes):len(prefixBytes)+32], gr.GCAKey[:])
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
	var request GCARegistration
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		s.logger.Error("Failed to decode request body:", err)
		return
	}

	// Validate and process the request
	if err := s.registerGCA(request); err != nil {
		http.Error(w, fmt.Sprint("Failed to register GCA:", err), http.StatusInternalServerError)
		s.logger.Error("Failed to register GCA:", err)
		return
	}

	// Send the response as JSON with a status code of OK
	resp := GCARegistrationResponse{
		PublicKey: s.staticPublicKey,
		HttpPort:  s.httpPort,
		TcpPort:   s.tcpPort,
		UdpPort:   s.udpPort,
	}
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
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
func (gcas *GCAServer) registerGCA(gr GCARegistration) error {
	gcas.mu.Lock()
	defer gcas.mu.Unlock()
	if gcas.gcaPubkeyAvailable {
		return fmt.Errorf("a GCA key has already been registered")
	}

	// Parse and verify the GCA key
	sb := gr.SigningBytes()
	isValid := glow.Verify(gcas.gcaTempKey, sb, gr.Signature)
	if !isValid {
		gcas.logger.Warn("Received bad GCA registration:", gr)
		return errors.New("invalid signature on GCAKey")
	}
	err := gcas.saveGCAKey(gr)
	if err != nil {
		gcas.logger.Warn("Unable to save GCA key:", gr)
		return fmt.Errorf("unable to save GCA key: %v", err)
	}
	return nil
}

// verifyGCAKey verifies the signature on a GCAKey object.
//
// It uses the public key of some authority (let's assume it's available in
// the server struct as someAuthorityPubkey) to verify the signature.
// The method returns an error if the verification fails.
func (gcas *GCAServer) verifyGCAKey(gr GCARegistration) error {
	// Generate the byte slice intended for the signing operation
	signingBytes := gr.SigningBytes()

	// Perform the signature verification. Assume Verify is a function that exists
	// to verify the signature.
	isValid := glow.Verify(gcas.gcaTempKey, signingBytes, gr.Signature)

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
// concatenating the server's base directory with "gcaPubKey.dat"
func (server *GCAServer) saveGCAKey(gr GCARegistration) error {
	// Determine the file path for the public key.
	// This should reside in the same directory as when reading it,
	// usually specified in server.baseDir.
	pubkeyPath := filepath.Join(server.baseDir, "gcaPubKey.dat")

	// Write the serialized public key to a file.
	// ioutil.WriteFile creates the file if it doesn't exist,
	// or truncates it before writing if it does.
	err := ioutil.WriteFile(pubkeyPath, gr.GCAKey[:], 0644)
	if err != nil {
		return fmt.Errorf("unable to write public key to file: %v", err)
	}

	server.gcaPubkey = gr.GCAKey
	server.gcaPubkeyAvailable = true
	return nil
}
