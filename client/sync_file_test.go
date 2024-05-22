package client

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestSyncFileCreateOnStartup(t *testing.T) {
	client, gcas, _, _ := FullClientTestEnvironment(t.Name())
	defer client.Close()
	defer gcas.Close()
	path := filepath.Join(client.staticBaseDir, LastSyncFile)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("file %s missing: %v", path, err)
	}
	storedTime, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		t.Errorf("could not parse timestamp from: %v", path)
		return
	}
	currentTime := time.Now().Unix()
	if storedTime != currentTime {
		t.Errorf("stored time %v different than current time %v", storedTime, currentTime)
	}
}

func TestSyncFileUpdateOnSync(t *testing.T) {
	client, gcas, _, _ := FullClientTestEnvironment(t.Name())
	defer client.Close()
	defer gcas.Close()
	path := filepath.Join(client.staticBaseDir, LastSyncFile)
	if err := os.WriteFile(path, []byte("12345"), 0644); err != nil {
		t.Errorf("error writing to %v: %v", path, err)
	}
	// Execute a sync with the server
	if s := client.threadedSyncWithServer(0); !s {
		t.Error("sync failed")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("file %s missing: %v", path, err)
	}
	storedTime, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		t.Errorf("could not parse timestamp from: %v", path)
		return
	}
	currentTime := time.Now().Unix()
	if storedTime != currentTime {
		t.Errorf("stored time %v different than current time %v", storedTime, currentTime)
	}
}

/*
func TestRequestRestartFileCreateAndRemove(t *testing.T) {
	// Create a client and a server to perform the test.
	client, gcas, _, _ := FullClientTestEnvironment(t.Name())
	defer client.Close()
	defer gcas.Close()

	path := filepath.Join(client.staticBaseDir, RequestRestartFile)

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

	// Wait for the delay period again to create a file
	time.Sleep(RequestRestartFileDelay + 10*time.Millisecond)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("does not exist when it should: %v", path)
	}

	// Sync again to remove the file
	if s := client.threadedSyncWithServer(0); !s {
		t.Error("sync failed")
	}
	time.Sleep(10 * time.Millisecond)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("exists when it should not: %v", path)
	}
}

func TestRequestRestartFileRemoveOnClose(t *testing.T) {
	// Create a client and a server to perform the test.
	client, gcas, _, _ := FullClientTestEnvironment(t.Name())
	defer gcas.Close()

	path := filepath.Join(client.staticBaseDir, RequestRestartFile)

	// Wait for the client to create a file
	time.Sleep(RequestRestartFileDelay + 10*time.Millisecond)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		client.Close()
		t.Errorf("does not exist when it should: %v", path)
	}

	// Stop the client, which should remove the file
	client.Close()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("exists when it should not: %v", path)
	}
}
*/
