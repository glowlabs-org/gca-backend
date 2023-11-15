package client

const (
	// The file that contains the keypair for the client, authorized by the GCA.
	ClientKeyfile = "client.keys"

	// The file that contains the public key of the GCA.
	GCAPubfile = "gca.pubkey"

	// GCAServerMapFile contains the list of servers that the GCA has online. It
	// will also contain banned servers.
	GCAServerMapFile = "gca-servers.dat"

	// HistoryFile contains all of the historic power readings for this solar
	// installation.
	HistoryFile = "history.dat"

	// ShortIDFile contains the ShortID of the device, which is useful for
	// compressing communications with the GCA servers.
	ShortIDFile = "short-id.dat"
)
