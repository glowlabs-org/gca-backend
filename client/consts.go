package client

const (
	// The file that contains the keypair for the client, authorized by the GCA.
	ClientKeyfile = "client.keys"

	// The file that contains the public key of the GCA.
	GCAPubfile = "gca.pub"

	// GCAServerMapFile contains the list of servers that the GCA has online. It
	// will also contain banned servers.
	GCAServerMapFile = "gca-servers.dat"

	// HistoryFile contains all of the historic power readings for this solar
	// installation.
	HistoryFile = "history.dat"

	// EnergyFile is the file used by the monitoring equipment to write the total
	// amount of energy that was measured in each timeslot.
	EnergyFile = "energy_data.csv"

	// ShortIDFile contains the ShortID of the device, which is useful for
	// compressing communications with the GCA servers.
	ShortIDFile = "short-id.dat"
)
