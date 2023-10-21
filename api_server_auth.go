package main

// Server auth has two endpoints. When the GCA server is first set up, it has
// no idea who its GCA is, as the GCA hasn't even generated keys yet. So we
// need one endpoint for the GCA to tell the server what the GCA pubkey is for
// that server. In the response to this endpoint, the GCA server will tell the
// GCA client what its own pubkey is. This is a TOFU sort of trust mechanism,
// the two entities are essentially swapping keypairs and both will remember
// those keys permanently into the future.
//
// The second endpoint is a signature from the GCA that the server is valid.
// This is a timestamped signature which expires after 4 weeks, on Sunday at
// midnight (right as Sunday becomes Monday). The GCA client is essentially
// signing the pubkey of the GCA server and asserting that the pubkey is still
// valid.
//
// All keys are ECDSA keys using the secp256k1 curve.
