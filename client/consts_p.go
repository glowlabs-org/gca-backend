//go:build !test
// +build !test

package client

import (
	"crypto/rand"
	"encoding/binary"
	"time"
)

const (
	// sendReportTime is set to 270 seconds, which is 30 seconds shorter
	// than 5 minutes. Including the random time extension, you get to 274
	// seconds at worst, which means that the delay should never be so long
	// that an entire 5 minute window is missed.
	//
	// Even if it is missed, the code will pick up the two reports and send
	// them both at once.
	sendReportTime = 270 * time.Second

	// EnergyFile is the file used by the monitoring equipment to write the total
	// amount of energy that was measured in each timeslot.
	EnergyFile = "/opt/halki/energy_data.csv"

	// CTMultiplier is the multiplier that we use on the current
	// transformer to correctly normalize the readings from the current
	// transformer.
	EnergyMultiplier = -2000

	// UDPSleepSyncTime sets the amount of time that the system sleeps
	// between each UDP packet that gets sent. We sleep between packets
	// because the cell network can only handle at points less than 1 kbps
	// of traffic, and sending a ton of packets all at once during a sync
	// operation is all but guaranteed to get them dropped.
	UDPSleepSyncTime = time.Second

	// Indicate that this is not a testing build of the client.
	testMode = false
)

// randomTimeExtension returns a random amount of time between 0 seconds and 4
// seconds. The goal is to introduce drift into the tick timer, such that all
// of the devices reporting to the same server will naturally spread out over
// the 5 minute period with their reports rather than cluster up and submit
// everything during the first 3 seconds.
func randomTimeExtension() time.Duration {
	var n int64
	// Read random bits into n and check for errors
	err := binary.Read(rand.Reader, binary.LittleEndian, &n)
	if err != nil {
		panic(err) // Handle the error according to your application's needs
	}
	// Ensure n is non-negative and within the desired range
	n = n % 4000
	return time.Duration(n) * time.Millisecond
}
