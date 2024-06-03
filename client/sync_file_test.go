package client

import (
	"math"
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
	if math.Abs(float64(storedTime)-float64(currentTime)) > 3 {
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
	if math.Abs(float64(storedTime)-float64(currentTime)) > 3 {
		t.Errorf("stored time %v different than current time %v", storedTime, currentTime)
	}
}

func TestReportFileCreateOnStartup(t *testing.T) {
	client, gcas, _, _ := FullClientTestEnvironment(t.Name())
	defer client.Close()
	defer gcas.Close()
	path := filepath.Join(client.staticBaseDir, LastReportFile)
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
	if math.Abs(float64(storedTime)-float64(currentTime)) > 3 {
		t.Errorf("stored time %v different than current time %v", storedTime, currentTime)
	}
}

func TestReportFileUpdateOnReport(t *testing.T) {
	client, gcas, _, _ := FullClientTestEnvironment(t.Name())
	defer client.Close()
	defer gcas.Close()
	path := filepath.Join(client.staticBaseDir, LastReportFile)
	if err := os.WriteFile(path, []byte("12345"), 0644); err != nil {
		t.Errorf("error writing to %v: %v", path, err)
	}
	// Send a report to the server
	err := updateMonitorFile(client.staticBaseDir, []uint32{1, 5, 10}, []uint64{500, 3000, 5000})
	if err != nil {
		gcas.Close()
		t.Fatal(err)
	}
	time.Sleep(2 * sendReportTime)
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
	if math.Abs(float64(storedTime)-float64(currentTime)) > 3 {
		t.Errorf("stored time %v different than current time %v", storedTime, currentTime)
	}
}
