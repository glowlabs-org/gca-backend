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
// that they retain the list after a restart.

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

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

	// TODO: Go through each server and download the list of other servers.

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

// newEquipmentCmd will create a new device and submit it to the remote server.
func newEquipmentCmd(gcaPubKey glow.PublicKey, gcaPrivKey glow.PrivateKey, serversMap map[glow.PublicKey]client.GCAServer) error {
	// TODO: We should probably track what shortIDs have been used via a
	// file. Might be good to have the servers tell us what shortIDs have
	// already been authorized. There are other options too, such as
	// bucketing and using randomization to minimize the chance that we
	// repeat values.
	return nil
}
