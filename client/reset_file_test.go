package client

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestResetFileCreateAndRemove(t *testing.T) {
	// Create a client and a server to perform the test.
	client, gcas, _, _ := FullClientTestEnvironment(t.Name())
	defer client.Close()
	defer gcas.Close()

	path := filepath.Join(client.staticBaseDir, RequestResetFile)

	// Settle to give the thread time to process
	time.Sleep(10 * time.Millisecond)

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("exists when it should not: %v", path)
	}

	// Execute a sync with the server and make sure the file is still not there.
	if s := client.threadedSyncWithServer(0); !s {
		t.Error("sync failed")
	}

	time.Sleep(10 * time.Millisecond)

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("exists when it should not: %v", path)
	}

	time.Sleep(RequestResetDelay + 10*time.Millisecond)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("does not exist when it should: %v", path)
	}

	if s := client.threadedSyncWithServer(0); !s {
		t.Error("sync failed")
	}

	time.Sleep(10 * time.Millisecond)

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("exists when it should not: %v", path)
	}

	// Wait again for the file to be written
	time.Sleep(RequestResetDelay + 10*time.Millisecond)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("does not exist when it should: %v", path)
	}
}

func TestResetRemoveOnClose(t *testing.T) {
	// Create a client and a server to perform the test.
	client, gcas, _, _ := FullClientTestEnvironment(t.Name())
	defer gcas.Close()

	path := filepath.Join(client.staticBaseDir, RequestResetFile)

	// Wait for the client to create a file
	time.Sleep(RequestResetDelay + 10*time.Millisecond)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		client.Close()
		t.Errorf("does not exist when it should: %v", path)
	}

	// Stop the client
	client.Close()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("exists when it should not: %v", path)
	}
}
