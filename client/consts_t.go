//go:build test
// +build test

package client

import (
	"time"
)

const (
	// reports will get sent every 15 milliseconds during testing.
	sendReportTime = time.Millisecond * 15
)
