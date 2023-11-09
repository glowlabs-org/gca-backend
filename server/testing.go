package server

// testing.go contains a bunch of exported functions that are useful for
// testing, but are really only intended to be used for testing.

import (
	"github.com/glowlabs-org/gca-backend/glow"
)

// Ports returns the ports that each of the listeners for this server are listening on.
func (gcas *GCAServer) Ports() (httpPort uint16, tcpPort uint16, udpPort uint16) {
	return gcas.httpPort, gcas.tcpPort, gcas.udpPort
}

// PublicKey returns the public key of this GCA server.
func (gcas *GCAServer) PublicKey() glow.PublicKey {
	return gcas.staticPublicKey
}
