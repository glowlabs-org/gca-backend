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
	// transformer. The reported energy value is first multiplied, then divided by
	// these values.
	EnergyMultiplierDefault = 1000
	EnergyDividerDefault    = 1000

	// UDPSleepSyncTime sets the amount of time that the system sleeps
	// between each UDP packet that gets sent. We sleep between packets
	// because the cell network can only handle at points less than 1 kbps
	// of traffic, and sending a ton of packets all at once during a sync
	// operation is all but guaranteed to get them dropped.
	UDPSleepSyncTime = time.Millisecond

	// Indicate that this is a testing build of the client.
	testMode = true
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
