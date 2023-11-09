package client

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/glowlabs-org/gca-backend/glow"
)

// The object used to store GCA servers on disk.
type GCAServer struct {
	Banned   bool
	Location string
	HttpPort uint16
	TcpPort  uint16
	UdpPort  uint16
}

func SerializeGCAServerMap(gcaMap map[glow.PublicKey]GCAServer) ([]byte, error) {
	var buffer bytes.Buffer

	for key, server := range gcaMap {
		// Serialize the key (PublicKey)
		if _, err := buffer.Write(key[:]); err != nil {
			return nil, err
		}

		// Serialize the 'Banned' bool as a single byte
		if server.Banned {
			buffer.WriteByte(1)
		} else {
			buffer.WriteByte(0)
		}

		// Serialize the length of 'Location' string as a uint16
		if len(server.Location) > 0xFFFF { // the max value of uint16
			return nil, errors.New("location string is too long")
		}
		locationLength := uint16(len(server.Location))
		if err := binary.Write(&buffer, binary.LittleEndian, locationLength); err != nil {
			return nil, err
		}

		// Serialize the 'Location' string
		if _, err := buffer.WriteString(server.Location); err != nil {
			return nil, err
		}

		// Serialize the ports (HttpPort, TcpPort, UdpPort) as uint16
		if err := binary.Write(&buffer, binary.LittleEndian, server.HttpPort); err != nil {
			return nil, err
		}
		if err := binary.Write(&buffer, binary.LittleEndian, server.TcpPort); err != nil {
			return nil, err
		}
		if err := binary.Write(&buffer, binary.LittleEndian, server.UdpPort); err != nil {
			return nil, err
		}
	}

	return buffer.Bytes(), nil
}

func DeserializeGCAServerMap(data []byte) (map[glow.PublicKey]GCAServer, error) {
	gcaMap := make(map[glow.PublicKey]GCAServer)
	reader := bytes.NewReader(data)

	for reader.Len() > 0 {
		// Deserialize the PublicKey
		var key glow.PublicKey
		if err := binary.Read(reader, binary.LittleEndian, &key); err != nil {
			return nil, err
		}

		// Deserialize the 'Banned' bool
		bannedByte, err := reader.ReadByte()
		if err != nil {
			return nil, err
		}
		banned := bannedByte != 0

		// Deserialize the length of 'Location' string
		var locationLength uint16
		if err := binary.Read(reader, binary.LittleEndian, &locationLength); err != nil {
			return nil, err
		}

		// Deserialize the 'Location' string
		location := make([]byte, locationLength)
		if _, err := reader.Read(location); err != nil {
			return nil, err
		}

		// Deserialize the ports (HttpPort, TcpPort, UdpPort)
		var httpPort, tcpPort, udpPort uint16
		if err := binary.Read(reader, binary.LittleEndian, &httpPort); err != nil {
			return nil, err
		}
		if err := binary.Read(reader, binary.LittleEndian, &tcpPort); err != nil {
			return nil, err
		}
		if err := binary.Read(reader, binary.LittleEndian, &udpPort); err != nil {
			return nil, err
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
