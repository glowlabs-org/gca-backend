//go:build !test
// +build !test

package client

import (
	"time"
)

const (
	// reports will get sent every minute during prod.
	sendReportTime = 60 * time.Second

	// EnergyFile is the file used by the monitoring equipment to write the total
	// amount of energy that was measured in each timeslot.
	EnergyFile = "/opt/halki/energy_data.csv"
)
