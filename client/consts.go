package client

const (
	// The file that contains the authorization for the client, signed by
	// the GCA.
	AuthorizationFile = "authorization.dat"

	// The file that contains the keypair for the client, authorized by the GCA.
	ClientKeyFile = "clientKeys.dat"

	// The file that contains the public key of the GCA.
	GCAPubKeyFile = "gcaPubKey.dat"

	// GCAServerMapFile contains the list of servers that the GCA has online. It
	// will also contain banned servers.
	GCAServerMapFile = "gcaServers.dat"

	// HistoryFile contains all of the historic power readings for this solar
	// installation.
	HistoryFile = "history.dat"

	// ShortIDFile contains the ShortID of the device, which is useful for
	// compressing communications with the GCA servers.
	ShortIDFile = "shortID.dat"

	// CTSettingsFile contains the current transformer multiplier.
	CTSettingsFile = "ct-settings.txt"

	// LastSyncFile is a file that contains the last successful sync time.
	LastSyncFile = "last-sync.txt"
)
