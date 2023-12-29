//go:build test
// +build test

package client

import (
	"crypto/rand"
	"encoding/binary"
	"time"
)

const (
	// reports will get sent every 50 milliseconds during testing. 50
	// milliseconds was chosen because it is slow enough that our reports
	// testing can happen in real time.
	sendReportTime = 50 * time.Millisecond

	// EnergyFile is the file used by the monitoring equipment to write the total
	// amount of energy that was measured in each timeslot.
	EnergyFile = "energy_data.csv"

	// CTMultiplier is the multiplier that we use on the current
	// transformer to correctly normalize the readings from the current
	// transformer.
	EnergyMultiplier = 1000
)

// mimics the production version of randomTimeExtension()
func randomTimeExtension() time.Duration {
	var n int64
	// Read random bits into n and check for errors
	err := binary.Read(rand.Reader, binary.LittleEndian, &n)
	if err != nil {
		panic(err) // Handle the error according to your application's needs
	}
	// Ensure n is non-negative and within the desired range
	n = n % 4
	return time.Duration(n) * time.Millisecond
}
