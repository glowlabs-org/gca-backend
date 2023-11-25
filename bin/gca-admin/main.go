package main

// gca-admin is a cli program that allows GCAs to perform all of their basic
// admin functions. This includes things like creating new devices, submitting
// the devices to the gca servers, and potentially even things like configuring
// a new set of servers.
//
// NOTE: Most of these functions are only intended to be used for testing
// purposes. This binary is capable of setting up servers both with temporary
// GCA keys and final GCA keys, but in production technicians should only ever
// be using this binary to create servers with temporary keys.

// TODO: We need to get the servers persisting the list of other servers so
// that they retain the list after a restart. This persistence can probably
// happen in authorized_servers.go

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

// main contains a harness to execute various commands. On startup, it makes
// sure that all of the basic requirements are in place. For example, a GCA key
// is needed unless the 'new-gca' command is provided.
func main() {
	// Default information.
	if len(os.Args) == 1 {
		fmt.Println("GCA Admin Tool v0.1")
		return
	}
	if len(os.Args) < 1 {
		fmt.Println("unrecognized usage of program")
		return
	}

	// The 'new-gca' command is for setting up a brand new GCA,
	// which mainly involved generating keys.
	if os.Args[1] == "new-gca" {
		fmt.Println("not implemented")
		return
	}

	// If the command is not 'new-gca', then the assumption is that the GCA
	// keys already exist and are available locally. These keys are going
	// to be part of all the other actions.
	keypath := filepath.Join("data", "gcaKeys.dat")
	keyData, err := ioutil.ReadFile(keypath)
	if err != nil {
		fmt.Println("unable to load gca keys:", err)
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
	serversMap, err := client.DeserializeGCAServerMap(serversData)
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

// fristEquipmentCmd creates the shortID file and submits the first equipment
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
	err = ioutil.WriteFile(shortIDPath, data[:], 0644)
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
