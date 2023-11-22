package server

import (
	"encoding/binary"
	"sync"

	"github.com/glowlabs-org/gca-backend/glow"
)

// AuthorizedServer tracks an authorized GCA server, complete with a signature
// from the GCA.
type AuthorizedServer struct {
	PublicKey        glow.PublicKey
	Banned           bool
	Location         string
	HttpPort         uint16
	TcpPort          uint16
	UdpPort          uint16
	GCAAuthorization glow.Signature
}

// OtherServers contains a mutex-protected list of servers that are known to be
// in use by the GCA.
type AuthorizedServers struct {
	servers []AuthorizedServer

	mu sync.Mutex
}

// Returns the list of servers that have been authorized and/or banned by the
// GCA. Makes a copy to avoid race condition issues.
func (gcas *GCAServer) AuthorizedServers() []AuthorizedServer {
	gcas.gcaServers.mu.Lock()
	defer gcas.gcaServers.mu.Unlock()
	as := make([]AuthorizedServer, len(gcas.gcaServers.servers))
	copy(as, gcas.gcaServers.servers)
	return as
}

// Create the serialization for the AuthorizedServer.
func (as *AuthorizedServer) Serialize() []byte {
	// Calculate the length.
	locationLength := len(as.Location)
	totalLength := 104 + locationLength
	data := make([]byte, totalLength)

	// Serialize the fields, except the signature.
	copy(data[0:32], as.PublicKey[:])
	if as.Banned {
		data[32] = 1
	} else {
		data[32] = 0
	}
	data[33] = byte(locationLength)
	copy(data[34:], []byte(as.Location))
	binary.BigEndian.PutUint16(data[34+locationLength:], as.HttpPort)
	binary.BigEndian.PutUint16(data[36+locationLength:], as.TcpPort)
	binary.BigEndian.PutUint16(data[38+locationLength:], as.UdpPort)
	copy(data[40+locationLength:], as.GCAAuthorization[:])
	return data
}

// SigningBytes generates the byte slice for signing an AuthorizedServer.
func (as *AuthorizedServer) SigningBytes() []byte {
	data := as.Serialize()
	return append([]byte("AuthorizedServer"), data[:len(data)-64]...)
}
