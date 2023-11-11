//go:build test
// +build test

package glow

import (
	"sync/atomic"
	"time"
)

var (
	GenesisTime = time.Now().Unix()
)

// This is a special variable only available in testing which allows the test
// to control what the current timeslot is, this makes it a lot easier to test
// timeslot related code.
var ManualCurrentTimeslot = uint32(0)

// Sets the current time of the protocol to the provided value.
func SetCurrentTimeslot(val uint32) {
	atomic.StoreUint32(&ManualCurrentTimeslot, val)
}

// Returns the current time of the protocol, as measured in 5 minute increments
// from genesis. This function implies a genesis time.
func CurrentTimeslot() uint32 {
	return atomic.LoadUint32(&ManualCurrentTimeslot)
}
