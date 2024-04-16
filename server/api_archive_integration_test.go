package server

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glowlabs-org/gca-backend/glow"
)

func TestApiArchiveIntegration(t *testing.T) {
	// Create a populated test environment and start a new server.
	gcas, dir, err := ServerTestEnvironment(t.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer gcas.Close()

	fmap := map[string]bool{
		"server.keys":                  false,
		"gcaTempPubKey.dat":            false,
		"gcaPubKey.dat":                false,
		"equipment-authorizations.dat": false,
		"allDeviceStats.dat":           false,
		"equipment-reports.dat":        false,
	}

	// Load all the generated data
	dmap := make(map[string][]byte)

	for f, _ := range fmap {
		path := filepath.Join(dir, f)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		dmap[f] = data
	}

	// Post the archive request
	resp, err := http.Post(fmt.Sprintf("http://localhost:%v/api/v1/archive", gcas.httpPort), "", nil)
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

	rdat := bytes.NewReader(body)

	rzip, err := zip.NewReader(rdat, int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}

	for _, zipf := range rzip.File {
		// Make sure there is not and extra file in the zip contents.
		if _, ok := fmap[zipf.Name]; ok {
			fmap[zipf.Name] = true
		} else {
			t.Fatal(fmt.Errorf("%v from zipfile is not valid", zipf.Name))
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

		if zipf.Name == "server.keys" {
			if len(data) != 96 || len(data) != len(dmap[zipf.Name]) {
				t.Fatal(fmt.Errorf("%v from zipfile wrong length", zipf.Name))
			}
			if !bytes.Equal(data[:32], dmap[zipf.Name][:32]) {
				t.Fatal(fmt.Errorf("file %v data does not match original file", zipf.Name))
			}
			zbuf := make([]byte, 64)
			if !bytes.Equal(data[32:], zbuf) {
				t.Fatal(fmt.Errorf("file %v zip contained private data contents", zipf.Name))
			}

		} else {
			if !bytes.Equal(data, dmap[zipf.Name]) {
				t.Fatal(fmt.Errorf("file %v data does not match original file", zipf.Name))
			}
		}
	}

	for f, found := range fmap {
		if !found {
			t.Fatal(fmt.Errorf("%v not found in zipfile", f))
		}
	}
}

// TODO: To avoid cut and paste, should consolidate some of these helper routines.

// Create a server test environment with populated files.
// Returns the test environment directory.
func ServerTestEnvironment(name string) (*GCAServer, string, error) {
	gcas, dirname, _, gcaPrivKey, err := SetupTestEnvironment(name)
	if err != nil {
		return nil, "", err
	}
	// This test is going to be messing with time, therefore defer a reset
	// of the time.
	defer glow.SetCurrentTimeslot(0)

	// Generate a keypair for a device.
	authPub, authPriv := glow.GenerateKeyPair()
	auth := glow.EquipmentAuthorization{ShortID: 0, PublicKey: authPub}
	sb := auth.SigningBytes()
	sig := glow.Sign(sb, gcaPrivKey)
	auth.Signature = sig
	_, err = gcas.saveEquipment(auth)
	if err != nil {
		return nil, "", err
	}

	// Submit reports for slots 0, 2, and 4.
	err = gcas.sendEquipmentReportSpecific(auth, authPriv, 0, 50)
	if err != nil {
		return nil, "", err
	}
	err = gcas.sendEquipmentReportSpecific(auth, authPriv, 2, 50)
	if err != nil {
		return nil, "", err
	}
	err = gcas.sendEquipmentReportSpecific(auth, authPriv, 4, 50)
	if err != nil {
		return nil, "", err
	}

	// Submit reports for slots 4031, 4030, and 4028. For these reports to
	// be accepted, time must be shifted. This will also trigger a report
	// migration.
	glow.SetCurrentTimeslot(4000)
	time.Sleep(450 * time.Millisecond) // Manually set to 450ms because it would NDF sometimes at 250ms.
	err = gcas.sendEquipmentReportSpecific(auth, authPriv, 4031, 50)
	if err != nil {
		return nil, "", err
	}
	err = gcas.sendEquipmentReportSpecific(auth, authPriv, 4030, 50)
	if err != nil {
		return nil, "", err
	}
	err = gcas.sendEquipmentReportSpecific(auth, authPriv, 4028, 50)
	if err != nil {
		return nil, "", err
	}

	return gcas, dirname, nil
}
