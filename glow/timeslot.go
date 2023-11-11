//go:build !test
// +build !test

package glow

import (
	"time"
)

const (
	GenesisTime = 1697414400
)

// Returns the current time of the protocol, as measured in 5 minute increments
// from genesis. This function implies a genesis time.
func CurrentTimeslot() uint32 {
	// TODO: Update for real network launch.
	genesisTime := int64(GenesisTime) // Monday Oct 16, 0:00:00 UTC
	now := time.Now().Unix()
	if now < genesisTime {
		panic("system clock appears to be incorrect")
	}
	secondsSinceGenesis := now - genesisTime
	return uint32(secondsSinceGenesis / 300)
}
