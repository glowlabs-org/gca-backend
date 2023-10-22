//go:build test
// +build test

package main

import (
	"sync/atomic"
	"time"
)

const (
	equipmentReportSize      = 80
	udpPort                  = 35030
	maxRecentReports         = 1e3
	maxRecentEquipmentAuths  = 1e3
	serverIP                 = "127.0.0.1"
	httpPort                 = ":35015"
	defaultLogLevel          = DEBUG
	reportMigrationFrequency = 100 * time.Millisecond
)

// This is a special variable only available in testing which allows the test
// to control what the current timeslot is, this makes it a lot easier to test
// timeslot related code.
var manualCurrentTimeslot = uint32(0)

// Sets the current time of the protocol to the provided value.
func setCurrentTimeslot(val uint32) {
	atomic.StoreUint32(&manualCurrentTimeslot, val)
}

// Returns the current time of the protocol, as measured in 5 minute increments
// from genesis. This function implies a genesis time.
func currentTimeslot() uint32 {
	return atomic.LoadUint32(&manualCurrentTimeslot)
}
