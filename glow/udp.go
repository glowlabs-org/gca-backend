package glow

import (
	"fmt"
	"net"
)

// SendUDPReport simulates sending a report to the server via UDP.
// The server should be listening on the given IP and port.
func SendUDPReport(report []byte, location string) error {
	conn, err := net.Dial("udp", location)
	if err != nil {
		return err
	}
	defer conn.Close()

	fmt.Println("*** sending SendUDPReport to", location)
	_, err = conn.Write(report)
	return err
}
