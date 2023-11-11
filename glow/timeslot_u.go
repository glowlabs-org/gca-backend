package glow

import (
	"fmt"
)

// UnixToTimeslot converts a unix time to the current timeslot.
func UnixToTimeslot(time int64) (uint32, error) {
	if time < GenesisTime {
		return 0, fmt.Errorf("not a valid timeslot")
	}
	return uint32(time-GenesisTime) / 300, nil
}
