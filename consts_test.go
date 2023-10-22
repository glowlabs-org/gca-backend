//go:build test
// +build test

package main

import (
	"time"
)

const (
	equipmentReportSize     = 80
	udpPort                 = 35030
	maxRecentReports        = 1e3
	maxRecentEquipmentAuths = 1e3
	serverIP                = "127.0.0.1"
	httpPort                = ":35015"
	defaultLogLevel         = DEBUG
)

// Returns the current time of the protocol, as measured in 5 minute increments
// from genesis. This function implies a genesis time.
func currentTimeslot() uint32 {
	genesisTime := time.Now().Unix()
	now := time.Now().Unix()
	if now < genesisTime {
		panic("system clock appears to be incorrect")
	}
	secondsSinceGenesis := now - genesisTime
	return uint32(secondsSinceGenesis / 300)
}
