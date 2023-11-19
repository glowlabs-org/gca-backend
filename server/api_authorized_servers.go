package server

// TODO: Write tests for this endpoint, integrate this endpoint into the
// client, write

import (
	"encoding/json"
	"net/http"

	"github.com/glowlabs-org/gca-backend/glow"
)

// This struct is used for the GET request. It doesn't need to be authenticated
// because the servers are all already signed by the GCA.
type AuthorizedServersResponse struct {
	AuthorizedServers []AuthorizedServer `json:"AuthorizedServers"`
}

// The GET server just returns the set of authorized servers, including banned
// ones.
func (s *GCAServer) AuthorizedServersHandler(w http.ResponseWriter, r *http.Request) {
	// Accept POST requests.
	if r.Method == http.MethodPost {
		s.AuthorizedServersHandlerPOST(w, r)
		return
	}

	// And also accept GET requests.
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET method is supported.", http.StatusMethodNotAllowed)
		s.logger.Warn("Received non-GET request for recent reports.")
		return
	}

	// Fetch the authorized servers and generate a signature
	asr := AuthorizedServersResponse{
		AuthorizedServers: s.AuthorizedServers(),
	}

	// Send the response as JSON with a status code of OK
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(asr); err != nil {
		http.Error(w, "Failed to encode JSON response", http.StatusInternalServerError)
		s.logger.Error("Failed to encode JSON response:", err)
		return
	}
}

// The POST handler receives a single server to be added to the server
// collection.
func (s *GCAServer) AuthorizedServersHandlerPOST(w http.ResponseWriter, r *http.Request) {
	// Decode the JSON request body.
	var server AuthorizedServer
	if err := json.NewDecoder(r.Body).Decode(&server); err != nil {
		http.Error(w, "Failed to decode JSON request", http.StatusInternalServerError)
		s.logger.Error("Failed to decode JSON request:", err)
		return
	}

	// Validate the signature.
	sb := server.SigningBytes()
	if !glow.Verify(s.gcaPubkey, sb, server.GCAAuthorization) {
		http.Error(w, "Invalid signature!", http.StatusInternalServerError)
		s.logger.Error("Invalid signature!")
		return
	}

	// Verify that the new server isn't conflicting with existing servers.
	// The GCA isn't allowed to update the information (like location and
	// ports) for the server. Any updates will be ignored. The server would
	// instead have to be banned and a new public key would need to be
	// created.
	s.gcaServers.mu.Lock()
	defer s.gcaServers.mu.Unlock()
	for i := 0; i < len(s.gcaServers.servers); i++ {
		if s.gcaServers.servers[i].PublicKey == server.PublicKey {
			if s.gcaServers.servers[i].Banned {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]string{"status": "success"})
				s.logger.Info("received authorization for server that is banned")
				return
			}
			if !server.Banned {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(map[string]string{"status": "success"})
				s.logger.Info("received authorization for server that already exists")
				return
			}
			s.gcaServers.servers[i] = server
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "success"})
			s.logger.Info("received authorization to ban server")
			return
		}
	}

	// If we made it here, it's a new server. The loop would have caught
	// any other instances for this server and it would have exited the
	// function.
	s.gcaServers.servers = append(s.gcaServers.servers, server)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	s.logger.Info("received authorization for new server")
}
