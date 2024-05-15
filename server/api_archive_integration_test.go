package server

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glowlabs-org/gca-backend/glow"
)

// This test creates a running server, populated with some report data, then
// gets an archive and compares to the original file data.
func TestApiArchive(t *testing.T) {
	// Create a populated test environment and start a new server.
	gcas, dir, err := ServerWithArchiveFiles(t.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer gcas.Close()
	// If the time was changed, we need to reset it after closing the server.
	defer glow.SetCurrentTimeslot(0)

	time.Sleep(200 * time.Millisecond) // timing delay to ensure that the server has completed any file changes

	fileMap := map[string]bool{}
	dataMap := map[string][]byte{}

	// Add the public files to the test.
	for _, name := range PublicFiles {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		fileMap[name] = false
		dataMap[name] = data
	}

	// Add the public key file, and verify that
	// server.pubkey contains the first 32 bytes from server.keys
	// This test verifies this value directly.
	const pkf = "server.pubkey"
	fileMap[pkf] = false
	path := filepath.Join(dir, "server.keys")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	dataMap[pkf] = data[:32]

	// Readme file.
	const rmf = "README"
	fileMap[rmf] = false
	dataMap[rmf] = []byte(ReadmeContents)

	// Post the archive request
	resp, err := http.Get(fmt.Sprintf("http://localhost:%v/api/v1/archive", gcas.httpPort))
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatal(fmt.Errorf("expected status 200, but got %d: %s", resp.StatusCode, string(body)))
	}

	// Read back all the zip.
	reader := bytes.NewReader(body)
	rzip, err := zip.NewReader(reader, int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	for _, zipf := range rzip.File {
		// Make sure there is not an extra file in the zip contents,
		// and keep track of the ones we find.
		if _, ok := fileMap[zipf.Name]; ok {
			fileMap[zipf.Name] = true
		} else {
			t.Fatal(fmt.Errorf("%v from zipfile is not a valid file", zipf.Name))
		}

		// Check each file against the original data.
		f, err := zipf.Open()
		if err != nil {
			t.Fatal(fmt.Errorf("%v from zipfile could not be opened: %v", zipf.Name, err))
		}
		defer f.Close()
		data, err := io.ReadAll(f)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, dataMap[zipf.Name]) {
			t.Fatal(fmt.Errorf("file %v data does not match original file", zipf.Name))
		}
	}

	// Verify all files were in the zip.
	for f, found := range fileMap {
		if !found {
			t.Fatal(fmt.Errorf("expected %v not found in zipfile", f))
		}
	}
}

// Tests the rate limited API under multithreaded access.
func TestApiArchiveRateLimiterMultithreaded(t *testing.T) {
	// Create a populated test environment and start a new server.
	gcas, _, err := ServerWithArchiveFiles(t.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer gcas.Close()
	// If the time was changed, we need to reset it after closing the server.
	defer glow.SetCurrentTimeslot(0)

	time.Sleep(apiArchiveRate) // Ensure that the rate limiter has cleared

	var allowed atomic.Int32
	var denied atomic.Int32
	var wg sync.WaitGroup

	// Choose a multiple of the api duration
	maxDur := time.Duration(3) * apiArchiveRate

	// Ensure the API is not called at the end of the duration
	segment := maxDur / time.Duration(apiArchiveLimit+1)

	start := time.Now()

	const threads = 10

	for i := 0; i < threads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Randomly choose a point within each segment
			durs := make([]float64, apiArchiveLimit)
			for j := 0; j < apiArchiveLimit; j++ {
				durs[j] = (float64(j) + rand.Float64()) * segment.Seconds()
			}

			slices.Sort(durs) // Make sure the times are in increasing order

			cuttab := make([]time.Time, apiArchiveLimit)
			for j := 0; j < apiArchiveLimit; j++ {
				cuttab[j] = start.Add(time.Duration(durs[j] * float64(time.Second)))
			}

			for j := 0; j < apiArchiveLimit; j++ {
				// Sleep until the next cut time.
				time.Sleep(time.Until(cuttab[j]))

				resp, err := http.Get(fmt.Sprintf("http://localhost:%v/api/v1/archive", gcas.httpPort))
				if err != nil {
					t.Fatal(err)
				}
				switch resp.StatusCode {
				case http.StatusOK:
					allowed.Add(1)
				case http.StatusTooManyRequests:
					denied.Add(1)
				default:
					t.Fatalf("Archive API call returned an invalid response %v", resp)
				}
			}
		}()
	}
	wg.Wait()

	testDur := time.Since(start)

	if testDur > time.Duration(3)*apiArchiveRate {
		t.Errorf("duration %v exceeded %v", testDur, time.Duration(3)*apiArchiveRate)
	}

	if allowed.Load()+denied.Load() != int32(threads*apiArchiveLimit) {
		t.Errorf("incorrect requests %v, expected %v", allowed.Load()+denied.Load(), threads*apiArchiveLimit)
	}

	// Calculate the number of rate limit intervals.
	periods := int(testDur / apiArchiveRate)
	if testDur%apiArchiveRate != 0 {
		periods++
	}

	if int(allowed.Load()) > periods*apiArchiveLimit {
		t.Errorf("%v allowed, expected %v", allowed.Load(), periods*apiArchiveLimit)
	}
}

// Create a server environment with populated archive files.
// Returns a running GCA server, and the test environment directory name.
func ServerWithArchiveFiles(name string) (*GCAServer, string, error) {
	gcas, dirname, _, gcaPrivKey, err := SetupTestEnvironment(name)
	if err != nil {
		return nil, "", err
	}

	// Submit new hardware via API
	auth, authPriv, err := gcas.submitNewHardware(0, gcaPrivKey)
	if err != nil {
		return nil, "", err
	}

	// Submit reports via API for slots 0, 2, and 4.
	err = gcas.staticSendEquipmentReportSpecific(auth, authPriv, 0, 50)
	if err != nil {
		return nil, "", err
	}
	err = gcas.staticSendEquipmentReportSpecific(auth, authPriv, 2, 50)
	if err != nil {
		return nil, "", err
	}
	err = gcas.staticSendEquipmentReportSpecific(auth, authPriv, 4, 50)
	if err != nil {
		return nil, "", err
	}

	// Submit reports via API for slots 4031, 4030, and 4028. For these reports to
	// be accepted, time must be shifted. This will also trigger a report
	// migration.
	glow.SetCurrentTimeslot(4000)
	time.Sleep(450 * time.Millisecond) // Manually set to 450ms because it would NDF sometimes at 250ms.
	err = gcas.staticSendEquipmentReportSpecific(auth, authPriv, 4031, 50)
	if err != nil {
		return nil, "", err
	}
	err = gcas.staticSendEquipmentReportSpecific(auth, authPriv, 4030, 50)
	if err != nil {
		return nil, "", err
	}
	err = gcas.staticSendEquipmentReportSpecific(auth, authPriv, 4028, 50)
	if err != nil {
		return nil, "", err
	}

	return gcas, dirname, nil
}
