//go:build !test
// +build !test

package server

import (
	"time"
)

const (
	equipmentReportSize      = 80
	maxRecentReports         = 100e3
	maxRecentEquipmentAuths  = 1e3
	serverIP                 = "0.0.0.0"
	httpPort                 = ":35015"
	udpPort                  = 35030
	tcpPort                  = ":35045"
	defaultLogLevel          = WARN
	reportMigrationFrequency = 1 * time.Hour
)
