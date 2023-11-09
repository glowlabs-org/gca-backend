package server

// This file contains testing related to the GCA temporary key and the
// authorization process. The major test is the integration test, which tries
// to walk the GCAServer through its entire lifecycle. This means that we set
// up a server where the pubkey of the temporary GCA key has already been saved
// to disk at the file "gca.tempkey". The test itself will have to create the
// temporary key and save the public key of the temporary key to that location.
//
// When the GCA server starts up, it will load and see the temporary key. Then
// the integration test will need to create a new key which represents the real
// GCA key, and the test will need to use the right endpoint to tell the
// GCAServer what the real GCA key is.
//
// Then the test needs to verify that all of the actions led to the desired
// result.
//
// The process of creating the temporary key and launching the GCAServer should
// be put into a separate function so that it can be used by all tests. The
// process of creating the real GCA key and using an endpoint to submit the
// real GCA key to the server should similarly be its own separate function so
// that other tests can use it when they need it.

import (
	"testing"

	"github.com/glowlabs-org/gca-backend/glow"
)

// TestGCAKeyLifecycle tests the full lifecycle of GCA keys,
// including temporary key creation, server startup, real key submission, and validation.
func TestGCAKeyLifecycle(t *testing.T) {
	// Generate a test directory.
	dir := glow.GenerateTestDir(t.Name())

	// Setup the test environment and launch the GCAServer.
	gcas, tempPrivKey, err := gcaServerWithTempKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer gcas.Close()

	// Check that the server lists the gca pubkey as unavailable.
	if gcas.gcaPubkeyAvailable {
		t.Fatal("gca pubkey should be set to unavailable before a pubkey has been submitted")
	}

	// Try submitting a public key to the server using the wrong priv key.
	badTempKey := tempPrivKey
	badTempKey[0]++
	_, err = gcas.submitGCAKey(badTempKey)
	if err == nil {
		t.Fatal("expected an error")
	}
	if gcas.gcaPubkeyAvailable {
		t.Fatal("gca pubkey should be set to unavailable before a pubkey has been submitted")
	}

	// Try submitting a public key to the server using the temp priv key.
	_, err = gcas.submitGCAKey(tempPrivKey)
	if err != nil {
		t.Fatal(err)
	}
	if !gcas.gcaPubkeyAvailable {
		t.Fatal("gca pubkey should be set to available after a pubkey has been submitted")
	}

	// Check that we get an error when trying to submit another gca key.
	_, err = gcas.submitGCAKey(tempPrivKey)
	if err == nil {
		t.Fatal("expecting an error")
	}
}
