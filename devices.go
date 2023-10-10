package main

import (
	"crypto/ed25519"
)

// Device represents a known device with its ShortID and corresponding public key.
type Device struct {
	ShortID uint32
	Key     ed25519.PublicKey
}

// loadDeviceKeys populates the deviceKeys map using the provided array of Devices.
func (gca *GCAServer) loadDeviceKeys(devices []Device) {
	for _, device := range devices {
		gca.deviceKeys[device.ShortID] = device.Key
	}
}
