//go:build test
// +build test

package client

import (
	"time"
)

const (
	// reports will get sent every 50 milliseconds during testing. 50
	// milliseconds was chosen because it is slow enough that our reports
	// testing can happen in real time.
	sendReportTime = 50 * time.Millisecond
)
