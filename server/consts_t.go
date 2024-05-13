//go:build test
// +build test

package server

import (
	"time"
)

const (
	maxRecentReports        = 1e3
	maxRecentEquipmentAuths = 1e3
	serverIP                = "127.0.0.1"
	httpPort                = 0
	tcpPort                 = 0
	udpPort                 = 0
	defaultLogLevel         = DEBUG
	testMode                = true
	serverShutdownTime      = 5 * time.Second
	wattTimeFrequency       = 20 * time.Millisecond

	ReportMigrationFrequency        = 100 * time.Millisecond
	WattTimeWeekDataUpdateFrequency = 1000 * time.Millisecond
)
