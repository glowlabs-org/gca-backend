package server

const (
	equipmentReportSize = 80
)

const (
	// The name of the file that will contain the history of AllDeviceStats
	// objects.
	AllDeviceStatsHistoryFile = "allDeviceStats.dat"

	// MaxCapacityBuffer determines how much a solar farm is allowed to
	// exceed its capacity by in a 5 minute period without the report being
	// banned. There's in implied division by 100, so 135 implies 135%.
	MaxCapacityBuffer = 135
)

var (
	// Change order of the public files: gca public key, equipment authorization, equipment reports, all device statistics.
	// Files should be archived in reverse order.
	PublicFiles = []string{"allDeviceStats.dat", "equipment-reports.dat", "equipment-authorizations.dat", "gcaPubKey.dat", "gcaTempPubKey.dat"}
)
