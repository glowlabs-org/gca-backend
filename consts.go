//go:build !test
// +build !test

package main

const (
	equipmentReportSize = 80
	udpPort             = 35030
	maxRecentReports    = 100e3
	serverIP            = "0.0.0.0"
	httpPort            = ":35015"
	defaultLogLevel     = WARN
)
