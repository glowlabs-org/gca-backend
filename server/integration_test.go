package server

// This file contains some helper methods that are applicable across the whole
// test suite. The functions in this file are useful for testing in general and
// not specific to testing any particular function of the GCA server. Most test
// suite helper functions can be found in the respective test file that tests
// the core component that the helper function relates to.

import (
	"fmt"

	"github.com/glowlabs-org/gca-backend/glow"
)

// setupTestEnvironment will return a fully initialized gca server that is
// ready to be used.
func setupTestEnvironment(testName string) (gcas *GCAServer, dir string, gcaPrivKey glow.PrivateKey, err error) {
	dir = glow.GenerateTestDir(testName)
	server, tempPrivKey, err := gcaServerWithTempKey(dir)
	if err != nil {
		return nil, "", glow.PrivateKey{}, fmt.Errorf("unable to create gca server with temp key: %v", err)
	}
	gcaPrivKey, err = server.submitGCAKey(tempPrivKey)
	if err != nil {
		return nil, "", glow.PrivateKey{}, fmt.Errorf("unable to submit gca priv key: %v", err)
	}
	return server, dir, gcaPrivKey, nil
}
