//go:build !test
// +build !test

package glow

import (
	"time"
)

const (
	// GenesisTime determines what counts as 'week 0' for the Glow
	// protocol. It has been set to Sunday, November 19th, 2023 at 0:00:00
	// UTC.
	GenesisTime = 1700352000
)

// Returns the current time of the protocol, as measured in 5 minute increments
// from genesis. This function implies a genesis time.
func CurrentTimeslot() uint32 {
	genesisTime := int64(GenesisTime)
	now := time.Now().Unix()
	if now < genesisTime {
		panic("system clock appears to be incorrect")
	}
	secondsSinceGenesis := now - genesisTime
	return uint32(secondsSinceGenesis / 300)
}
