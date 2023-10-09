package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"os"
)

const (
	maxBufferSize = 80
	port          = 35030
)

// EquipmentReport defines the structure of the received message.
type EquipmentReport struct {
	ShortID     uint32
	Timeslot    uint32
	PowerOutput uint64
	Signature   [64]byte
}

func main() {
	// Start the UDP listener in its own goroutine.
	go func() {
		err := startUDPServer()
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}
	}()

	select {} // This keeps the main routine running indefinitely to keep the program alive.
}

func startUDPServer() error {
	// Define the UDP address (any available address on port 35030).
	addr := net.UDPAddr{
		Port: port,
		IP:   net.ParseIP("0.0.0.0"),
	}

	// Start listening on the defined UDP address.
	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		return fmt.Errorf("Error starting UDP server: %v", err)
	}
	defer conn.Close()

	fmt.Printf("Listening on UDP port %d...\n", port)

	buffer := make([]byte, maxBufferSize)
	for {
		// Read data from the UDP connection into the buffer.
		n, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Println("Error reading from UDP connection:", err)
			continue
		}

		// Check if received data length matches the expected length.
		if n != maxBufferSize {
			fmt.Println("Received message of invalid length")
			continue
		}

		// Start a new goroutine to handle the report, allowing for simultaneous processing of incoming data.
		go handleEquipmentReport(buffer[:n])
	}

	return nil
}

func parseReport(data []byte) (*EquipmentReport, error) {
	// Create a new bytes reader from the provided data.
	reader := bytes.NewReader(data)
	report := &EquipmentReport{}

	// Parse the individual fields from the data using the Big Endian format.
	if err := binary.Read(reader, binary.BigEndian, &report.ShortID); err != nil {
		return nil, err
	}
	if err := binary.Read(reader, binary.BigEndian, &report.Timeslot); err != nil {
		return nil, err
	}
	if err := binary.Read(reader, binary.BigEndian, &report.PowerOutput); err != nil {
		return nil, err
	}
	copy(report.Signature[:], data[16:])

	return report, nil
}

func handleEquipmentReport(data []byte) {
	// Parse the raw data into a structured report.
	report, err := parseReport(data)
	if err != nil {
		fmt.Println("Failed to parse report:", err)
		return
	}

	// Display the parsed report.
	fmt.Printf("Received Report:\nShortID: %d\nTimeslot: %d\nPowerOutput: %d\nSignature: %x\n",
		report.ShortID, report.Timeslot, report.PowerOutput, report.Signature)
}

