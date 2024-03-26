package main

// gca-admin is a cli program that allows GCAs to perform all of their basic
// admin functions. This includes initializing the GCA, initializing new GCA
// servers, and initializing new monitoring devices.
//
// This binary is meant to be usable in production by GCAs, and therefore
// intentionally blocks any actions that may be harmful to the GCA or have
// unintended consequences.

// When a new server is created, there is an assumption that the new server
// will have the tempPubKey of the GCA, which is created prior to the GCA
// receiving a laptop. The server will need the gca temp key added at the
// location '/home/user/gca-server/gcaTempPubKey.dat'. The temp key is stored
// locally at "~/.config/gca-data/gcaTempPubKey.dat'.

// TODO: We need to get the servers persisting the list of other servers so
// that they retain the list after a restart. This persistence can probably
// happen in authorized_servers.go

// TODO: Need to ensure that any instructions for the GCAs will have them
// double check that brand new keys were created when they call the new-gca
// command.

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/glowlabs-org/gca-backend/client"
	"github.com/glowlabs-org/gca-backend/glow"
	"github.com/glowlabs-org/gca-backend/server"
)

// help() displays a list of commands.
func help() {
	fmt.Print(`
new-gca 
	Generates brand new keys for the device. Should only
	be called once per new GCA.

authorize-server
	Authorizes a new GCA server.

init-equipment
	Creates the very first GCA equipment

new-equipment
	Creates a new GCA equipement

resubmit
	Submits an existing GCA equipment to the server
`)
}

// main contains a harness to execute various commands. On startup, it makes
// sure that all of the basic requirements are in place. For example, a GCA key
// is needed unless the 'new-gca' command is provided.
func main() {
	if len(os.Args) < 1 {
		fmt.Println("unrecognized usage")
		return
	}

	// Get the location of the gca directory.
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Unable to locate home dir:", err)
		return
	}
	gcaDir := filepath.Join(homeDir, ".config", "gca-data")
	// Ensure that the gca directory exists
	err = os.MkdirAll(gcaDir, 0700)
	if err != nil {
		fmt.Println("Unable to create gca directory:", err)
		return
	}
	gcaKeyPath := filepath.Join(gcaDir, "gcaKeys.dat")
	gcaTempKeyPath := filepath.Join(gcaDir, "gcaTempKeys.dat")
	gcaTempPubKeyPath := filepath.Join(gcaDir, "gcaTempPubKey.dat")
	serversPath := filepath.Join(gcaDir, "gcaServers.dat")
	shortIDPath := filepath.Join(gcaDir, "latestShortID.dat")
	clientsPath := filepath.Join(gcaDir, "clients")

	// Create the temp keys if they don't already exist. The idea behind
	// the temp keys is that the technician who is creating the GCA servers
	// and the GCA lockbooks needs a way to ensure that only the lockbook
	// can control the first GCA servers, but also that the technician
	// doesn't have access to the real GCA keys.
	_, err = os.Stat(gcaTempKeyPath)
	if os.IsNotExist(err) {
		// Create the temp keys.
		var data [64]byte
		var pubData [32]byte
		pubKey, privKey := glow.GenerateKeyPair()
		fmt.Println(privKey, len(privKey))
		copy(data[:32], pubKey[:])
		copy(data[32:], privKey[:])
		copy(pubData[:], pubKey[:])
		err = ioutil.WriteFile(gcaTempKeyPath, data[:], 0400)
		if err != nil {
			fmt.Println("Unable to write GCA temp keys:", err)
			return
		}
		err = ioutil.WriteFile(gcaTempPubKeyPath, pubData[:], 0400)
		if err != nil {
			fmt.Println("Unable to write GCA temp pub key:", err)
			return
		}
		fmt.Println("GCA temp keys automatically created.")
	}

	// Default information.
	if len(os.Args) == 1 {
		fmt.Println("GCA Admin Tool v0.1")
		help()
		return
	}
	// Check for a help command.
	if os.Args[1] == "help" {
		help()
		return
	}

	// Look for a new-gca command.
	if os.Args[1] == "new-gca" {
		newGCA(gcaKeyPath)
		return
	}

	// If the command is not 'new-gca', then the assumption is that the GCA
	// keys already exist and will need to be used in one of the next
	// commands. Load those keys.
	keyData, err := ioutil.ReadFile(gcaKeyPath)
	if err != nil {
		fmt.Println("Unable to load gca keys, may need to run 'new-gca':", err)
		return
	}
	var gcaPubKey glow.PublicKey
	var gcaPrivKey glow.PrivateKey
	copy(gcaPubKey[:], keyData[:32])
	copy(gcaPrivKey[:], keyData[32:])
	tempKeyData, err := ioutil.ReadFile(gcaTempKeyPath)
	if err != nil {
		fmt.Println("Unable to load gca temp keys, won't be able to authorize new servers:", err)
		return
	}
	var gcaTempPrivKey glow.PrivateKey
	copy(gcaTempPrivKey[:], tempKeyData[32:])

	// Load the list of servers for this GCA.
	serversData, err := ioutil.ReadFile(serversPath)
	if err != nil && !os.IsNotExist(err) {
		fmt.Println("unable to load list of GCA servers:", err)
		return
	}
	serversMap, err := client.UntrustedDeserializeGCAServerMap(serversData)
	if err != nil {
		fmt.Println("list of GCA servers appears correupt:", err)
		return
	}
	for pk, server := range serversMap {
		fmt.Printf("Loaded server %x: %v\n", pk, server)
	}

	// Check for authorize-server, a command that will use the gca temp key
	// to declare the real gca key to the server, and also authorize the
	// gca server and grab credentials for it.
	if os.Args[1] == "authorize-server" {
		authorizeServer(gcaPubKey, gcaTempPrivKey, serversPath, serversMap)
		return
	}

	// TODO: Go through each server and download the list of other servers.

	// Check if the user wants to create their very first equipement, a
	// process that has a few extra steps. We don't do this automatically
	// because we want to make sure that the intention is there. Otherwise
	// the user may accidentally create equipment with conflicting short
	// ids.
	if os.Args[1] == "init-equipment" {
		err := initEquipmentCmd(shortIDPath)
		if err != nil {
			fmt.Println("Unable to make first equipment:", err)
			return
		}
	}

	// Check if the user wants to authorize a new device.
	if os.Args[1] == "new-equipment" {
		err := newEquipmentCmd(gcaPubKey, gcaPrivKey, serversMap, shortIDPath, clientsPath)
		if err != nil {
			fmt.Println("Unable to create and authorize new equipment:", err)
			return
		}
		return
	}

	// Check if the user wants to resubmit an authorization
	if os.Args[1] == "resubmit" {
		err := resubmitCmd(gcaPubKey, gcaPrivKey, serversMap, clientsPath)
		if err != nil {
			fmt.Println("Unable to resubmit equipment:", err)
			return
		}
		return
	}
}

// newGCA will create keys for a new GCA. It will refuse to generate new keys
// if a file already exists containing GCA keys.
func newGCA(gcaKeyPath string) {
	// Check that gca keys don't already exist.
	_, err := os.Stat(gcaKeyPath)
	if err == nil {
		fmt.Println("The GCA keys already exist")
		return
	}
	if !os.IsNotExist(err) {
		fmt.Println("Unexpected error:", err)
		return
	}

	// Create the keys.
	var data [64]byte
	pubKey, privKey := glow.GenerateKeyPair()
	copy(data[:32], pubKey[:])
	copy(data[32:], privKey[:])
	err = ioutil.WriteFile(gcaKeyPath, data[:], 0400)
	if err != nil {
		fmt.Println("Unable to write GCA keys:", err)
		return
	}
	fmt.Println("GCA keys successfully written!")
}

// authorizeServer will authorize a new server.
func authorizeServer(gcaPubKey glow.PublicKey, gcaTempPrivKey glow.PrivateKey, serversPath string, serversMap map[glow.PublicKey]client.GCAServer) {
	if len(os.Args) != 4 {
		fmt.Println("Usage: ./gca-admin authorize-server [server-location] [port]")
		return
	}

	gr := server.GCARegistration{
		GCAKey: gcaPubKey,
	}
	sb := gr.SigningBytes()
	gr.Signature = glow.Sign(sb, gcaTempPrivKey)
	payload, err := json.Marshal(gr)
	if err != nil {
		fmt.Println("Unable to create GCARegistration:", err)
		return
	}
	req, err := http.NewRequest("POST", fmt.Sprintf("http://%v:%v/api/v1/register-gca", os.Args[2], os.Args[3]), bytes.NewBuffer(payload))
	if err != nil {
		fmt.Println("Unable to create GCARegistration request:", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	c := &http.Client{}
	resp, err := c.Do(req)
	if err != nil {
		fmt.Println("Unable to submit GCARegistration request:", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respBodyBytes, readErr := ioutil.ReadAll(resp.Body)
		if readErr != nil {
			fmt.Println("failed to read response body:", err)
			return
		}
		fmt.Printf("Received a non-200 status code: %d :: %s\n", resp.StatusCode, string(respBodyBytes))
		return
	}

	// Decode the response from the server.
	var grr server.GCARegistrationResponse
	err = json.NewDecoder(resp.Body).Decode(&grr)
	if err != nil {
		fmt.Println("Unable to decode response:", err)
		return
	}

	// Save the new server.
	srv := client.GCAServer{
		Banned:   false,
		Location: os.Args[2],
		HttpPort: grr.HttpPort,
		TcpPort:  grr.TcpPort,
		UdpPort:  grr.UdpPort,
	}
	serversMap[grr.PublicKey] = srv
	serversData, err := client.SerializeGCAServerMap(serversMap)
	if err != nil {
		fmt.Println("Unable to serialize server map:", err)
		return
	}
	err = ioutil.WriteFile(serversPath, serversData, 0644)
	if err != nil {
		fmt.Println("Unable to write server map:", err)
		return
	}
	fmt.Println("Successfully initialized new server.")
	return
}

// initEquipmentCmd creates the shortID file so that equipment can be created
// in the future.
func initEquipmentCmd(shortIDPath string) error {
	// Check whether the shortID file already exists, return an error if
	// so.
	_, err := os.Stat(shortIDPath)
	if err == nil {
		return fmt.Errorf("it appears that the first equipment already exists")
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("unexpected error when opening short id file: %v", err)
	}

	// Write out the file.
	var data [4]byte
	binary.LittleEndian.PutUint32(data[:], 1)
	err = ioutil.WriteFile(shortIDPath, data[:], 0600)
	if err != nil {
		return fmt.Errorf("unable to write the short id file: %v", err)
	}
	return nil
}

// newEquipmentCmd will create a new device and submit it to the remote server.
func newEquipmentCmd(gcaPubKey glow.PublicKey, gcaPrivKey glow.PrivateKey, serversMap map[glow.PublicKey]client.GCAServer, shortIDPath string, clientsPath string) error {
	// Check that there are servers, since this command makes network
	// calls.
	if len(serversMap) == 0 {
		return fmt.Errorf("no servers provided")
	}

	var latitude, longitude float64
	var capacity, debt int
	var expiration, initYear, initMonth, initDay int
	var protocolFee int

	fmt.Print("Enter latitude (3 decimals): ")
	if _, err := fmt.Scanln(&latitude); err != nil {
		fmt.Println("Error reading latitude:", err)
		os.Exit(1)
	}
	fmt.Print("Enter longitude (3 decimals): ")
	if _, err := fmt.Scanln(&longitude); err != nil {
		fmt.Println("Error reading longitude:", err)
		os.Exit(1)
	}
	fmt.Print("Enter capacity (integer number of watts): ")
	if _, err := fmt.Scanln(&capacity); err != nil {
		fmt.Println("Error reading capacity:", err)
		os.Exit(1)
	}
	fmt.Print("Enter debt (integer number of kilograms of CO2): ")
	if _, err := fmt.Scanln(&debt); err != nil {
		fmt.Println("Error reading debt:", err)
		os.Exit(1)
	}
	fmt.Print("Enter lifetime (integer number of years, typically 10): ")
	if _, err := fmt.Scanln(&expiration); err != nil {
		fmt.Println("Error reading expiration:", err)
		os.Exit(1)
	}
	fmt.Print("Enter protocol fee (integer number of cents): ")
	if _, err := fmt.Scanln(&protocolFee); err != nil {
		fmt.Println("Error reading protocol fee:", err)
		os.Exit(1)
	}
	fmt.Print("Enter initialization year (integer): ")
	if _, err := fmt.Scanln(&initYear); err != nil {
		fmt.Println("Error reading initialization year:", err)
		os.Exit(1)
	}
	fmt.Print("Enter initialization month (integer): ")
	if _, err := fmt.Scanln(&initMonth); err != nil {
		fmt.Println("Error reading initialization month:", err)
		os.Exit(1)
	}
	fmt.Print("Enter initialization day (integer): ")
	if _, err := fmt.Scanln(&initDay); err != nil {
		fmt.Println("Error reading initialization day:", err)
		os.Exit(1)
	}

	// Printing the entered data
	fmt.Printf("\nYou entered the following data:\n")
	fmt.Printf("Latitude: %.3f\n", latitude)
	fmt.Printf("Longitude: %.3f\n", longitude)
	fmt.Printf("Capacity: %.3f kw\n", float64(capacity)/1000)
	fmt.Printf("Debt: %.3f metric tons of CO2\n", float64(debt)/1000)
	fmt.Printf("Lifetime: %d years\n", expiration)
	fmt.Printf("Protocol Fee: $%.2f\n", float64(protocolFee)/100)
	fmt.Printf("Initialization Date: %d-%d-%d\n", initYear, initMonth, initDay)

	// Asking for confirmation
	var confirmation string
	fmt.Print("\nIs this information correct? (yes/no): ")
	if _, err := fmt.Scanln(&confirmation); err != nil {
		fmt.Println("Error reading confirmation:", err)
		os.Exit(1)
	}

	if confirmation != "yes" {
		fmt.Println("Data entry aborted.")
		os.Exit(0)
	}

	fmt.Println("Data confirmed. Proceeding...")

	// Get the unix timestamp of the initialization date.
	initDate := time.Date(initYear, time.Month(initMonth), initDay, 0, 0, 0, 0, time.UTC)
	initUnix := initDate.Unix()
	initTimeslot, err := glow.UnixToTimeslot(initUnix)
	if err != nil {
		fmt.Println("Invalid init time:", err)
		os.Exit(1)
	}
	// Perform other unit conversions.
	secondsPerGlowYear := int64(3600 * 24 * 7 * 52)
	expirationUnix := initUnix + int64(expiration)*secondsPerGlowYear
	finalCapacity := uint64(capacity) * 1000 / 12 // watts to milliwatthours per 5 minutes
	finalDebt := uint64(debt) * 1000              // kilograms to grams
	finalExpiration, err := glow.UnixToTimeslot(expirationUnix)
	if err != nil {
		return fmt.Errorf("expiration date for solar farm is out of bounds")
	}

	// Load the latest shortid.
	shortIDData, err := ioutil.ReadFile(shortIDPath)
	if err != nil {
		return fmt.Errorf("unable to read shortID file: %v", err)
	}
	if len(shortIDData) != 4 {
		return fmt.Errorf("shortID file should have 4 bytes")
	}
	nextShortID := binary.LittleEndian.Uint32(shortIDData)

	// Update the latest shortid to the next value, so that multiple
	// equipment doesn't get created with the same ShortID.
	binary.LittleEndian.PutUint32(shortIDData, nextShortID+1)
	err = ioutil.WriteFile(shortIDPath, shortIDData, 0644)
	if err != nil {
		return fmt.Errorf("unable to write updated ShortID file: %v", err)
	}

	// Create a directory for all of the equipment data.
	clientName := "client_" + strconv.Itoa(int(nextShortID))
	dir := filepath.Join(clientsPath, clientName)
	err = os.MkdirAll(dir, 0744)
	if err != nil {
		return fmt.Errorf("unable to create client directory: %v", err)
	}
	fmt.Println()
	fmt.Println("Creating equipment", nextShortID)

	// Create keys for the client
	var keyData [64]byte
	keyPath := filepath.Join(dir, client.ClientKeyFile)
	clientPubKey, clientPrivKey := glow.GenerateKeyPair()
	copy(keyData[:32], clientPubKey[:])
	copy(keyData[32:], clientPrivKey[:])
	err = ioutil.WriteFile(keyPath, keyData[:], 0644)
	if err != nil {
		return fmt.Errorf("unable to write client keys: %v", err)
	}

	// Give the client the list of authorized servers
	serversPath := filepath.Join(dir, client.GCAServerMapFile)
	serversData, err := client.SerializeGCAServerMap(serversMap)
	if err != nil {
		return fmt.Errorf("unable to serialize client server map: %v", err)
	}
	err = ioutil.WriteFile(serversPath, serversData, 0644)
	if err != nil {
		return fmt.Errorf("unable to write client server map: %v", err)
	}

	// Give the client the gca pubkey
	gcaPubKeyPath := filepath.Join(dir, client.GCAPubKeyFile)
	err = ioutil.WriteFile(gcaPubKeyPath, gcaPubKey[:], 0644)
	if err != nil {
		return fmt.Errorf("unable to write gca pubkey file: %v", err)
	}

	// Give the client its short id
	clientShortIDPath := filepath.Join(dir, client.ShortIDFile)
	binary.LittleEndian.PutUint32(shortIDData, nextShortID)
	err = ioutil.WriteFile(clientShortIDPath, shortIDData, 0644)
	if err != nil {
		return fmt.Errorf("unable to write updated ShortID file: %v", err)
	}

	// Initialize the history file for the client. We set the history file
	// to be the current timeslot minus three days. We substract three days
	// because these files are being transferred to a new device which may
	// have a different clock. 3 days is quite generous but it's also
	// pretty inexpensive.
	clientHistoryPath := filepath.Join(dir, client.HistoryFile)
	currentTimeslot := glow.CurrentTimeslot()
	var historyData [4]byte
	binary.LittleEndian.PutUint32(historyData[:], currentTimeslot-3*288)
	err = ioutil.WriteFile(clientHistoryPath, historyData[:], 0644)
	if err != nil {
		return fmt.Errorf("unable to write history file header: %v", err)
	}

	// Create the authorization.
	ea := glow.EquipmentAuthorization{
		ShortID:   nextShortID,
		PublicKey: clientPubKey,

		Latitude:  latitude,
		Longitude: longitude,

		Capacity:   finalCapacity,
		Debt:       finalDebt,
		Expiration: finalExpiration,

		Initialization: initTimeslot,
		ProtocolFee:    uint64(protocolFee),
	}
	sb := ea.SigningBytes()
	ea.Signature = glow.Sign(sb, gcaPrivKey)
	j, err := json.Marshal(ea)
	if err != nil {
		return fmt.Errorf("unable to marshal equipment authorization: %v", err)
	}

	// Save the authorization to disk so that it can be resubmitted in the future.
	clientAuthorizationPath := filepath.Join(dir, client.AuthorizationFile)
	err = ioutil.WriteFile(clientAuthorizationPath, j, 0644)
	if err != nil {
		return fmt.Errorf("unable to write authorization file to disk: %v", err)
	}

	// Submit the authorization to all servers.
	fmt.Println()
	for _, server := range serversMap {
		url := fmt.Sprintf("http://%v:%v/api/v1/authorize-equipment", server.Location, server.HttpPort)
		resp, err := http.Post(url, "application/json", bytes.NewBuffer(j))
		if err != nil || resp.StatusCode != http.StatusOK || resp.Body.Close() != nil {
			fmt.Printf("Had difficulties submitting auth to %v: %v\n", server.Location, err)
		} else {
			fmt.Println("Equipment successfully authorized on", server.Location)
		}
	}
	return nil
}

// resubmitCmd is used when the admin wants to resbumit an authorized solar
// farm to the GCA servers.
func resubmitCmd(gcaPubKey glow.PublicKey, gcaPrivKey glow.PrivateKey, serversMap map[glow.PublicKey]client.GCAServer, clientsPath string) error {
	// Check that there are servers, since this command makes network
	// calls.
	if len(serversMap) == 0 {
		return fmt.Errorf("no servers provided")
	}

	// Ensure that a shortID was provided.
	if len(os.Args) != 3 {
		return fmt.Errorf("Usage: ./gca-admin resubmit [shortID]")
	}

	// Convert the provided shortID to a uint16
	num, err := strconv.ParseUint(os.Args[2], 10, 16)
	if err != nil {
		return fmt.Errorf("Provided shortID must be an integer between 0 and 65000")
	}
	shortID := uint16(num)

	// Load the client authorization from disk.
	clientName := "client_" + strconv.Itoa(int(shortID))
	dir := filepath.Join(clientsPath, clientName)
	filePath := filepath.Join(dir, client.AuthorizationFile)
	b, err := ioutil.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("Unable to load the path for this ShortID")
	}
	fmt.Println("The following data was loaded:")
	fmt.Println(string(b))

	// Asking for confirmation
	var confirmation string
	fmt.Print("\nIs this information correct? (yes/no): ")
	if _, err := fmt.Scanln(&confirmation); err != nil {
		fmt.Println("Error reading confirmation:", err)
		os.Exit(1)
	}

	if confirmation != "yes" {
		fmt.Println("Data entry aborted.")
		os.Exit(0)
	}
	fmt.Println("Data confirmed. Proceeding...")

	for _, server := range serversMap {
		url := fmt.Sprintf("http://%v:%v/api/v1/authorize-equipment", server.Location, server.HttpPort)
		resp, err := http.Post(url, "application/json", bytes.NewBuffer(b))
		if err != nil || resp.StatusCode != http.StatusOK || resp.Body.Close() != nil {
			fmt.Printf("Had difficulties submitting auth to %v: %v\n", server.Location, err)
		} else {
			fmt.Println("Equipment successfully authorized on", server.Location)
		}
	}
	return nil
}
