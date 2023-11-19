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

// SigningBytes generates the byte slice for signing an AuthorizedServer.
func (as *AuthorizedServer) SigningBytes() []byte {
	// Calculate the lengths, excluding the signature.
	locationLength := len(as.Location)
	totalLength := 32 + 1 + 2 + locationLength + 2 + 2 + 2
	data := make([]byte, totalLength)

	// Serialize the fields, except the signature.
	copy(data[0:32], as.PublicKey[:])
	if as.Banned {
		data[32] = 1
	} else {
		data[32] = 0
	}
	binary.LittleEndian.PutUint16(data[33:35], uint16(locationLength))
	copy(data[35:35+locationLength], []byte(as.Location))
	binary.LittleEndian.PutUint16(data[35+locationLength:], as.HttpPort)
	binary.LittleEndian.PutUint16(data[37+locationLength:], as.TcpPort)
	binary.LittleEndian.PutUint16(data[39+locationLength:], as.UdpPort)
	return append([]byte("AuthorizedServer"), data...)
}
