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

// Since Go does not support constant lists from mutable types, provide a
// list of files which can be shared here. If new public files are
// added to this server, this function should be modified to return them.
// The order matters in this, because we want to archive files before other
// files that depend on them.
func PublicFiles() [5]string {
	return [5]string{"gcaTempPubKey.dat", "gcaPubKey.dat", "allDeviceStats.dat", "equipment-authorizations.dat", "equipment-reports.dat"}
}
