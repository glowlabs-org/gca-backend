package server

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	if err := json.NewEncoder(w).Encode(asr); err != nil {
		http.Error(w, "Failed to encode JSON response", http.StatusInternalServerError)
		s.logger.Error("Failed to encode JSON response:", err)
		return
	}
}

// The POST handler receives a single server to be added to the server
// collection. After the new server information is received, it will send
// details of that server to all of the other servers.
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
	for i := 0; i < len(s.gcaServers.servers); i++ {
		if s.gcaServers.servers[i].PublicKey == server.PublicKey {
			if s.gcaServers.servers[i].Banned {
				s.gcaServers.mu.Unlock()
				json.NewEncoder(w).Encode(map[string]string{"status": "success"})
				s.logger.Info("received authorization for server that is banned")
				return
			}
			if !server.Banned {
				s.gcaServers.mu.Unlock()
				json.NewEncoder(w).Encode(map[string]string{"status": "success"})
				s.logger.Info("received authorization for server that already exists")
				return
			}
			s.gcaServers.servers[i] = server
			s.gcaServers.mu.Unlock()
			json.NewEncoder(w).Encode(map[string]string{"status": "success"})
			s.logger.Info("received authorization to ban server")
			return
		}
	}
	// If we made it here, it's a new server. The loop would have caught
	// any other instances for this server and it would have exited the
	// function.
	s.gcaServers.servers = append(s.gcaServers.servers, server)
	// Create a list of all the servers that we need to call to submit this
	// new server to. This list that we send the new server to actually
	// includes the server itself, and that is intentional because we want
	// to make sure that the server's own list of viable servers includes
	// itself.
	ass := make([]AuthorizedServer, len(s.gcaServers.servers))
	copy(ass, s.gcaServers.servers)
	s.gcaServers.mu.Unlock()

	// Create a list of all the authorized devices that we want to send to
	// the new server. This is potentially a decent amount of traffic, but
	// onboarding a new server is pretty rare and we want to make sure they
	// know everything. Note that this happens under its own mutex.
	s.mu.Lock()
	auths := make([]glow.EquipmentAuthorization, 0)
	for _, e := range s.equipment {
		auths = append(auths, e)
	}
	s.mu.Unlock()

	// Send the new server to all of the other GCA servers we know about.
	// This means that there's sort of a quadradic DoS here, where each new
	// server that gets submitted will be sent to all of the other servers,
	// and then they will also send it to all of the other servers,
	// resulting in n^2 total messages sent and received. But it'll stop
	// there because once a server has received the message once, it'll
	// have it and it won't send it again.
	//
	// It's important to ensure that this code runs after the duplicate
	// check runs, otherwise you can get infinite loops of servers sending
	// authorizations to each other.
	for _, as := range ass {
		// Don't tell banned servers anything.
		if as.Banned {
			continue
		}

		// Send the new authorization to the other servers.
		j, err := json.Marshal(server)
		if err != nil {
			continue
		}
		resp, err := http.Post(fmt.Sprintf("http://%v:%v/api/v1/authorized-servers", as.Location, as.HttpPort), "application/json", bytes.NewBuffer(j))
		if err != nil {
			s.logger.Infof("Failed to send request to server: %v", err)
			continue
		}
		// We don't check any errors because if there is an error,
		// there's not much we can do about it. Instead we fire and
		// forget. This does mean that propagation of the new server is
		// not guaranteed and that we will need the sync loop to be
		// grabbing the full list of servers every once in a while
		// anyway.
		resp.Body.Close()
	}

	// Send the new server the full list of authorized equipment.
	for _, a := range auths {
		j, err := json.Marshal(a)
		if err != nil {
			continue
		}
		resp, err := http.Post(fmt.Sprintf("http://%v:%v/api/v1/authorize-equipment", server.Location, server.HttpPort), "application/json", bytes.NewBuffer(j))
		if err != nil {
			s.logger.Infof("Failed to send request to server: %v", err)
			continue
		}
		resp.Body.Close()
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	s.logger.Info("received authorization for new server")
}
