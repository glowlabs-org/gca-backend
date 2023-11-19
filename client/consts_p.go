//go:build !test
// +build !test

package client

import (
	"crypto/rand"
	"encoding/binary"
	"time"
)

const (
	// sendReportTime is set to 285 seconds, which is 15 seconds shorter
	// than 5 minutes. Including the random time extension, you get to 289
	// seconds at worst, which means that the delay should never be so long
	// that an entire 5 minute window is missed.
	//
	// Even if it is missed, the code will pick up the two reports and send
	// them both at once.
	sendReportTime = 285 * time.Second

	// EnergyFile is the file used by the monitoring equipment to write the total
	// amount of energy that was measured in each timeslot.
	EnergyFile = "/opt/halki/energy_data.csv"
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
