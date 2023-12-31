package client

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/glowlabs-org/gca-backend/glow"
)

// The object used to store GCA servers on disk. Once a GCAServer is banned, it
// can never become un-banned.
type GCAServer struct {
	Banned   bool
	Location string
	HttpPort uint16
	TcpPort  uint16
	UdpPort  uint16
}

// This is a function for creating the server map and saving it to disk.
func SerializeGCAServerMap(gcaMap map[glow.PublicKey]GCAServer) ([]byte, error) {
	var buffer bytes.Buffer

	for key, server := range gcaMap {
		// Serialize the key (PublicKey)
		if _, err := buffer.Write(key[:]); err != nil {
			return nil, fmt.Errorf("unable to write key to buffer: %v", err)
		}

		// Serialize the 'Banned' bool as a single byte
		if server.Banned {
			buffer.WriteByte(1)
		} else {
			buffer.WriteByte(0)
		}

		// Serialize the length of 'Location' string as a uint16
		if len(server.Location) > 0xFFFF { // the max value of uint16
			return nil, fmt.Errorf("location string is too long")
		}
		locationLength := uint16(len(server.Location))
		if err := binary.Write(&buffer, binary.LittleEndian, locationLength); err != nil {
			return nil, fmt.Errorf("unable to write location length to buffer: %v", err)
		}

		// Serialize the 'Location' string
		if _, err := buffer.WriteString(server.Location); err != nil {
			return nil, fmt.Errorf("unable to write location to buffer: %v", err)
		}

		// Serialize the ports (HttpPort, TcpPort, UdpPort) as uint16
		if err := binary.Write(&buffer, binary.LittleEndian, server.HttpPort); err != nil {
			return nil, fmt.Errorf("unable to write HttpPort to buffer: %v", err)
		}
		if err := binary.Write(&buffer, binary.LittleEndian, server.TcpPort); err != nil {
			return nil, fmt.Errorf("unable to write TcpPort to buffer: %v", err)
		}
		if err := binary.Write(&buffer, binary.LittleEndian, server.UdpPort); err != nil {
			return nil, fmt.Errorf("unable to write UdpPort to buffer: %v", err)
		}
	}

	return buffer.Bytes(), nil
}

// UntrustedDeserializeGCAServerMap will deserialize an untrusted array into a
// GCA server map.
func UntrustedDeserializeGCAServerMap(data []byte) (map[glow.PublicKey]GCAServer, error) {
	gcaMap := make(map[glow.PublicKey]GCAServer)
	reader := bytes.NewReader(data)

	for reader.Len() > 0 {
		// Deserialize the PublicKey
		if reader.Len() < 32 {
			return nil, fmt.Errorf("not enough data to read PublicKey, only have %v bytes", reader.Len())
		}
		var key glow.PublicKey
		if err := binary.Read(reader, binary.LittleEndian, &key); err != nil {
			return nil, fmt.Errorf("error reading PublicKey: %v", err)
		}

		// Deserialize the 'Banned' bool
		if reader.Len() < 1 {
			return nil, fmt.Errorf("not enough data to read Banned flag")
		}
		bannedByte, err := reader.ReadByte()
		if err != nil {
			return nil, fmt.Errorf("error reading Banned flag: %v", err)
		}
		banned := bannedByte != 0

		// Deserialize the length of 'Location' string
		var locationLength uint16
		if reader.Len() < 2 {
			return nil, fmt.Errorf("not enough data to read Location length")
		}
		if err := binary.Read(reader, binary.LittleEndian, &locationLength); err != nil {
			return nil, fmt.Errorf("error reading Location length: %v", err)
		}

		// Deserialize the 'Location' string
		location := make([]byte, locationLength)
		if reader.Len() < int(locationLength) {
			return nil, fmt.Errorf("not enough data to read Location string")
		}
		if _, err := reader.Read(location); err != nil {
			return nil, fmt.Errorf("error reading Location string: %v", err)
		}

		// Deserialize the ports (HttpPort, TcpPort, UdpPort)
		var httpPort, tcpPort, udpPort uint16
		if reader.Len() < 6 { // 2 bytes each for httpPort, tcpPort, udpPort
			return nil, fmt.Errorf("not enough data to read ports")
		}
		if err := binary.Read(reader, binary.LittleEndian, &httpPort); err != nil {
			return nil, fmt.Errorf("error reading HttpPort: %v", err)
		}
		if err := binary.Read(reader, binary.LittleEndian, &tcpPort); err != nil {
			return nil, fmt.Errorf("error reading TcpPort: %v", err)
		}
		if err := binary.Read(reader, binary.LittleEndian, &udpPort); err != nil {
			return nil, fmt.Errorf("error reading UdpPort: %v", err)
		}

		// Add the deserialized key-value pair to the map
		gcaMap[key] = GCAServer{
			Banned:   banned,
			Location: string(location),
			HttpPort: httpPort,
			TcpPort:  tcpPort,
			UdpPort:  udpPort,
		}
	}

	return gcaMap, nil
}
