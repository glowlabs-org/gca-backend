package main

import (
	"crypto/ed25519"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

const (
	maxBufferSize    = 80
	port             = 35030
	maxRecentReports = 100e3
)

// EquipmentReport defines the structure of the report received from equipment.
type EquipmentReport struct {
	ShortID     uint32
	Timeslot    uint32
	PowerOutput uint64
	Signature   [64]byte
}

// Device represents a known device with its ShortID and corresponding public key.
type Device struct {
	ShortID uint32
	Key     ed25519.PublicKey
}

// GCAServer is the main server structure.
type GCAServer struct {
	deviceKeys    map[uint32]ed25519.PublicKey
	recentReports []EquipmentReport
	conn          *net.UDPConn
	quit          chan bool
}

// NewGCAServer creates and initializes a new GCAServer instance and loads device keys.
func NewGCAServer() *GCAServer {
	server := &GCAServer{
		deviceKeys:    make(map[uint32]ed25519.PublicKey),
		recentReports: make([]EquipmentReport, 0, maxRecentReports),
		quit:          make(chan bool),
	}
	devices := make([]Device, 0)
	server.loadDeviceKeys(devices)
	go server.startUDPServer()
	return server
}

// Close method to close the UDP connection.
func (gca *GCAServer) Close() {
	close(gca.quit) // Signal the quit channel.
	// Wait for the listening loop to exit before closing the connection.
	<-time.After(100 * time.Millisecond) // A small delay to allow the loop to exit. Adjust as necessary.
	if gca.conn != nil {
		gca.conn.Close()
	}
}

// loadDeviceKeys populates the deviceKeys map using the provided array of Devices.
func (gca *GCAServer) loadDeviceKeys(devices []Device) {
	for _, device := range devices {
		gca.deviceKeys[device.ShortID] = device.Key
	}
}

// parseReport decodes the raw data into an EquipmentReport and verifies its signature.
// It checks the ShortID in the report against known device keys and ensures the signature is valid.
func (gca *GCAServer) parseReport(data []byte) (*EquipmentReport, error) {
	report := &EquipmentReport{}

	if len(data) != 80 { // 4 bytes for ShortID + 4 bytes for Timeslot + 8 bytes for PowerOutput + 64 bytes for Signature
		return nil, fmt.Errorf("unexpected data length: got %d bytes, expected 80 bytes", len(data))
	}

	// Parse ShortID
	report.ShortID = binary.BigEndian.Uint32(data[0:4])
	// Parse Timeslot
	report.Timeslot = binary.BigEndian.Uint32(data[4:8])
	// Parse PowerOutput
	report.PowerOutput = binary.BigEndian.Uint64(data[8:16])
	// Copy Signature
	copy(report.Signature[:], data[16:80])

	// Look up the device's public key.
	pubKey, ok := gca.deviceKeys[report.ShortID]
	if !ok {
		return nil, fmt.Errorf("unknown device ID: %d", report.ShortID)
	}

	// Verify the signature using the device's public key.
	if !ed25519.Verify(pubKey, data[:16], report.Signature[:]) {
		return nil, errors.New("signature verification failed")
	}

	return report, nil
}

// handleEquipmentReport processes the received raw data.
// It parses the report, logs the details if successful, and stores the report in recentReports.
func (gca *GCAServer) handleEquipmentReport(data []byte) {
	report, err := gca.parseReport(data)
	if err != nil {
		println(err)
		fmt.Println("Failed to process report:", err)
		return
	}
	println("success")

	fmt.Printf("Received Report:\nShortID: %d\nTimeslot: %d\nPowerOutput: %d\nSignature: %x\n",
		report.ShortID, report.Timeslot, report.PowerOutput, report.Signature)

	// Append the report to recentReports.
	gca.recentReports = append(gca.recentReports, *report)

	// If the length of recentReports exceeds maxRecentReports, reallocate and keep only the 50% latest reports.
	if len(gca.recentReports) > maxRecentReports {
		halfIndex := len(gca.recentReports) / 2
		copy(gca.recentReports[:], gca.recentReports[halfIndex:])
		gca.recentReports = gca.recentReports[:halfIndex]
	}
}

// startUDPServer starts the UDP server to listen for incoming reports.
func (gca *GCAServer) startUDPServer() {
	addr := net.UDPAddr{
		Port: port,
		IP:   net.ParseIP("0.0.0.0"),
	}

	var err error
	gca.conn, err = net.ListenUDP("udp", &addr) // Initialize the conn field here.
	if err != nil {
		fmt.Println("Error starting UDP server:", err)
		os.Exit(1)
	}
	defer gca.conn.Close()

	fmt.Printf("Listening on UDP port %d...\n", port)

	buffer := make([]byte, maxBufferSize)
	for {
		select {
		case <-gca.quit:
			return
		default:
			// do nothing
		}

		n, _, err := gca.conn.ReadFromUDP(buffer)
		if err != nil {
			select {
			case <-gca.quit:
				// If it's not a closed connection error, print the error
				if !strings.Contains(err.Error(), "use of closed network connection") {
					fmt.Println("Error reading from UDP connection:", err)
				}
				return
			default:
				fmt.Println("Error reading from UDP connection:", err)
			}
			continue
		}

		if n != maxBufferSize {
			fmt.Println("Received message of invalid length")
			continue
		}

		go gca.handleEquipmentReport(buffer[:n])
	}
}
