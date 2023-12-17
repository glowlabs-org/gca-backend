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

	"github.com/glowlabs-org/gca-backend/client"
	"github.com/glowlabs-org/gca-backend/glow"
)

// help() displays a list of commands.
func help() {
	fmt.Print(`
new-gca 
	Generates brand new keys for the device. Should only
	be called once per new GCA.

new-server
	Authorizes a new GCA server.
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

	// Create the temp keys if they don't already exist. The idea behind
	// the temp keys is that the technician who is creating the GCA servers
	// and the GCA lockbooks needs a way to ensure that only the lockbook
	// can control the first GCA servers, but also that the technician
	// doesn't have access to the real GCA keys.
	_, err := os.Stat(gcaTempKeyPath)
	if os.IsNotExist(err) {
		// Create the temp keys.
		var data [96]byte
		var pubData [32]byte
		pubKey, privKey := glow.GenerateKeyPair()
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
		fmt.Println("Unable to load gca keys:", err)
		return
	}
	var gcaPubKey glow.PublicKey
	var gcaPrivKey glow.PrivateKey
	copy(gcaPubKey[:], keyData[:32])
	copy(gcaPrivKey[:], keyData[32:])

	// TODO: We are going to need a command along the lines of
	// 'new-server'.

	// Load the list of servers for this GCA.
	serversPath := filepath.Join("data", "gcaServers.dat")
	serversData, err := ioutil.ReadFile(serversPath)
	if err != nil {
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

	// TODO: Go through each server and download the list of other servers.

	// Check if the user wants to create their very first equipement, a
	// process that has a few extra steps. We don't do this automatically
	// because we want to make sure that the intention is there. Otherwise
	// the user may accidentally create equipment with conflicting short
	// ids.
	if os.Args[1] == "first-equipment" {
		err := firstEquipmentCmd(gcaPubKey, gcaPrivKey, serversMap)
		if err != nil {
			fmt.Println("Unable to make first equipment:", err)
			return
		}
	}

	// Check if the user wants to authorize a new device.
	if os.Args[1] == "new-equipment" {
		err := newEquipmentCmd(gcaPubKey, gcaPrivKey, serversMap)
		if err != nil {
			fmt.Println("Unable to create and authorize new equipment:", err)
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
	var data [96]byte
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

// firstEquipmentCmd creates the shortID file and submits the first equipment
// to the servers.
func firstEquipmentCmd(gcaPubKey glow.PublicKey, gcaPrivKey glow.PrivateKey, serversMap map[glow.PublicKey]client.GCAServer) error {
	// Check whether the shortID file already exists, return an error if
	// so.
	shortIDPath := filepath.Join("data", "latestShortID.dat")
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
	err = ioutil.WriteFile(shortIDPath, data[:], 0400)
	if err != nil {
		return fmt.Errorf("unable to write the short id file: %v", err)
	}

	return newEquipmentCmd(gcaPubKey, gcaPrivKey, serversMap)
}

// newEquipmentCmd will create a new device and submit it to the remote server.
func newEquipmentCmd(gcaPubKey glow.PublicKey, gcaPrivKey glow.PrivateKey, serversMap map[glow.PublicKey]client.GCAServer) error {
	// Check that there are servers, since this command makes network
	// calls.
	if len(serversMap) == 0 {
		return fmt.Errorf("no servers provided")
	}

	// The user should have provided: latitude, longitude, capacity, debt,
	// and an expiration. Validate all of the inputs.
	if len(os.Args) != 7 {
		fmt.Println()
		fmt.Println("Usage: ./gca-admin new-equipment [latitude] [longitude] [capacity] [debt] [expiration]")
		fmt.Println("\tPlease provide latitdue as a floating point value")
		fmt.Println("\tPlease provide longitude as a floating point value")
		fmt.Println("\tPlease provide capacity as an integer number of watts")
		fmt.Println("\tPlease provide debt as an integer number of kilograms of CO2")
		fmt.Println("\tPlease provide 'expiration' as an integer number of years")
		fmt.Println()
		return fmt.Errorf("command was not called correctly")
	}
	fmt.Println()
	latitude, err := strconv.ParseFloat(os.Args[2], 64)
	if err != nil {
		return fmt.Errorf("unable to parse latitude: %v", err)
	}
	fmt.Printf("latitude: %.3f\n", latitude)
	longitude, err := strconv.ParseFloat(os.Args[3], 64)
	if err != nil {
		return fmt.Errorf("unable to parse longitude: %v", err)
	}
	fmt.Printf("longitude: %.3f\n", longitude)
	capacity, err := strconv.Atoi(os.Args[4])
	if err != nil {
		return fmt.Errorf("unable to parse capacity: %v", err)
	}
	fmt.Printf("capacity: %.3f kilowatts\n", float64(capacity)/1e6)
	debt, err := strconv.Atoi(os.Args[5])
	if err != nil {
		return fmt.Errorf("unable to parse debt: %v", err)
	}
	fmt.Printf("debt: %.3f metric tons of CO2\n", float64(debt)/1e6)
	expiration, err := strconv.Atoi(os.Args[6])
	if err != nil {
		return fmt.Errorf("unable to parse expiration: %v", err)
	}
	fmt.Printf("expiration: %d years\n", expiration)

	// Load the latest shortid.
	shortIDPath := filepath.Join("data", "latestShortID.dat")
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
	dir := filepath.Join("data", clientName)
	err = os.MkdirAll(dir, 0744)
	if err != nil {
		return fmt.Errorf("unable to create client directory: %v", err)
	}
	fmt.Println()
	fmt.Println("Creating equipment", nextShortID)

	// Create keys for the client
	var keyData [96]byte
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

	// Create the authorization and submit it to all of the servers.
	ea := glow.EquipmentAuthorization{
		ShortID:    nextShortID,
		PublicKey:  clientPubKey,
		Latitude:   latitude,
		Longitude:  longitude,
		Capacity:   uint64(capacity),
		Debt:       uint64(debt),
		Expiration: uint32(expiration),
	}
	sb := ea.SigningBytes()
	ea.Signature = glow.Sign(sb, gcaPrivKey)
	j, err := json.Marshal(ea)
	if err != nil {
		return fmt.Errorf("unable to marshal equipment authorization: %v", err)
	}
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
