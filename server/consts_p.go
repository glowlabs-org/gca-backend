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
	httpPort                = 35015
	tcpPort                 = 35030
	udpPort                 = 35045
	defaultLogLevel         = WARN
	testMode                = false
	serverShutdownTime      = 90 * time.Second
	wattTimeFrequency       = 2 * time.Minute

	ReportMigrationFrequency        = 1 * time.Hour
	WattTimeWeekDataUpdateFrequency = 24 * time.Hour

	apiArchiveLimit = 3
	apiArchiveRate  = 3 * time.Second
)
