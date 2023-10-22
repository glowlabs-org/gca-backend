//go:build !test
// +build !test

package main

import (
	"time"
)

const (
	equipmentReportSize      = 80
	udpPort                  = 35030
	maxRecentReports         = 100e3
	maxRecentEquipmentAuths  = 1e3
	serverIP                 = "0.0.0.0"
	httpPort                 = ":35015"
	defaultLogLevel          = WARN
	reportMigrationFrequency = 1 * time.Hour
)

// Returns the current time of the protocol, as measured in 5 minute increments
// from genesis. This function implies a genesis time.
func currentTimeslot() uint32 {
	// TODO: Update for real network launch.
	genesisTime := 1697414400 // Monday Oct 16, 0:00:00 UTC
	now := time.Now().Unix()
	if now < genesisTime {
		panic("system clock appears to be incorrect")
	}
	secondsSinceGenesis := now - genesisTime
	return uint32(secondsSinceGenesis / 300)
}
