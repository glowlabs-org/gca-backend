//go:build !test
// +build !test

package server

import (
	"time"
)

const (
	maxRecentReports        = 100e3
	maxRecentEquipmentAuths = 1e3
	serverIP                = "0.0.0.0"
	httpPort                = ":35015"
	tcpPort                 = ":35030"
	udpPort                 = 35045
	defaultLogLevel         = WARN
	testMode                = false
	wattTimeFrequency       = 2 * time.Minute
	httpServerCtxTimeout    = 5 * time.Second

	ReportMigrationFrequency        = 1 * time.Hour
	WattTimeWeekDataUpdateFrequency = 24 * time.Hour
)
