//go:build test
// +build test

package server

import (
	"time"
)

const (
	equipmentReportSize      = 80
	tcpPort                  = ":0"
	udpPort                  = 0
	maxRecentReports         = 1e3
	maxRecentEquipmentAuths  = 1e3
	serverIP                 = "127.0.0.1"
	httpPort                 = ":0"
	defaultLogLevel          = DEBUG
	reportMigrationFrequency = 100 * time.Millisecond
)
