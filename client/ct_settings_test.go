package client

// The client package is the package that doesn't have any other dependencies,
// and therefore it's the best place to perform integration tests.
// reports_test.go also has some pretty comprehensive testing.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"testing"
	"time"

	"github.com/glowlabs-org/gca-backend/glow"
	"github.com/glowlabs-org/gca-backend/server"
)

// CISettingsTest creates an override settings file, and checks that
// the client sends the correct energy report.
func TestCISettings(t *testing.T) {
	name := t.Name()
	gcas, _, gcaPubkey, gcaPrivKey, err := server.SetupTestEnvironment(name + "_server1")
	if err != nil {
		t.Errorf("unable to set up the test environment for a server: %v", err)
	}
	defer gcas.Close()
	clientDir := glow.GenerateTestDir(name + "_client1")
	err = SetupTestEnvironment(clientDir, gcaPubkey, gcaPrivKey, []*server.GCAServer{gcas})
	if err != nil {
		t.Errorf("unable to set up the client test environment: %v", err)
	}
	// create a ct settings file
	ctPath := path.Join(clientDir, CTSettingsFile)
	ctf, err := os.Create(ctPath)
	if err != nil {
		t.Errorf("could not open ct settings file: %v", err)
	}
	ctContent := "-2000\n-4000\n" // this will set energy value to 1/2.
	_, err = ctf.WriteString(ctContent)
	if err != nil {
		ctf.Close()
		t.Errorf("unable to write ct settings file: %v", err)
	}
	ctf.Close()
	// start the client
	client, err := NewClient(clientDir)
	if err != nil {
		t.Errorf("unable to create client: %v", err)
	}
	defer client.Close()

	// Update the monitoring file so that the client submits data to the server.
	err = updateMonitorFile(client.staticBaseDir, []uint32{1, 5}, []uint64{500, 3000})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * sendReportTime)

	// Check that the server got the report.
	httpPort, _, _ := gcas.Ports()
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%v/api/v1/recent-reports?publicKey=%x", httpPort, client.staticPubKey))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatal("bad status")
	}
	var response server.RecentReportsResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		t.Fatal(err)
	}
	for i, report := range response.Reports {
		if i == 1 && report.PowerOutput != 250 {
			t.Error("server does not seem to have the report", report.PowerOutput)
		} else if i == 5 && report.PowerOutput != 1500 {
			t.Error("server does not seem to have expected report", report.PowerOutput)
		} else if i != 1 && i != 5 && report.PowerOutput != 0 {
			t.Error("server has reports we didn't send")
		}
	}
}

func TestCISettingsMalformed(t *testing.T) {
	name := t.Name()
	gcas, _, gcaPubkey, gcaPrivKey, err := server.SetupTestEnvironment(name + "_server1")
	if err != nil {
		t.Errorf("unable to set up the test environment for a server: %v", err)
	}
	defer gcas.Close()
	clientDir := glow.GenerateTestDir(name + "_client1")
	err = SetupTestEnvironment(clientDir, gcaPubkey, gcaPrivKey, []*server.GCAServer{gcas})
	if err != nil {
		t.Errorf("unable to set up the client test environment: %v", err)
	}
	// create a malformed ct settings file
	ctPath := path.Join(clientDir, CTSettingsFile)
	ctf, err := os.Create(ctPath)
	if err != nil {
		t.Errorf("could not open ct settings file: %v", err)
	}
	ctContent := "-2000\nwhoops\n"
	_, err = ctf.WriteString(ctContent)
	if err != nil {
		ctf.Close()
		t.Errorf("unable to write ct settings file: %v", err)
	}
	ctf.Close()
	// try to start the client
	client, err := NewClient(clientDir)
	if err == nil {
		client.Close()
		t.Errorf("client read the malformed ct settings file")
	}
}

func TestCISettingsMissingValue(t *testing.T) {
	name := t.Name()
	gcas, _, gcaPubkey, gcaPrivKey, err := server.SetupTestEnvironment(name + "_server1")
	if err != nil {
		t.Errorf("unable to set up the test environment for a server: %v", err)
	}
	defer gcas.Close()
	clientDir := glow.GenerateTestDir(name + "_client1")
	err = SetupTestEnvironment(clientDir, gcaPubkey, gcaPrivKey, []*server.GCAServer{gcas})
	if err != nil {
		t.Errorf("unable to set up the client test environment: %v", err)
	}
	// create a malformed ct settings file
	ctPath := path.Join(clientDir, CTSettingsFile)
	ctf, err := os.Create(ctPath)
	if err != nil {
		t.Errorf("could not open ct settings file: %v", err)
	}
	ctContent := "-2000\n"
	_, err = ctf.WriteString(ctContent)
	if err != nil {
		ctf.Close()
		t.Errorf("unable to write ct settings file: %v", err)
	}
	ctf.Close()
	// try to start the client
	client, err := NewClient(clientDir)
	if err == nil {
		client.Close()
		t.Errorf("client read the missing value ct settings file")
	}
}

func TestCISettingsFileNotFound(t *testing.T) {
	name := t.Name()
	gcas, _, gcaPubkey, gcaPrivKey, err := server.SetupTestEnvironment(name + "_server1")
	if err != nil {
		t.Errorf("unable to set up the test environment for a server: %v", err)
	}
	defer gcas.Close()
	clientDir := glow.GenerateTestDir(name + "_client1")
	err = SetupTestEnvironment(clientDir, gcaPubkey, gcaPrivKey, []*server.GCAServer{gcas})
	if err != nil {
		t.Errorf("unable to set up the client test environment: %v", err)
	}
	// Ensure the CT settings file does not exist
	ctPath := path.Join(clientDir, CTSettingsFile)
	os.Remove(ctPath)

	// start the client
	client, err := NewClient(clientDir)
	if err != nil {
		t.Errorf("unable to create client: %v", err)
	}
	defer client.Close()

	if client.energyMultiplier != EnergyMultiplierDefault || client.energyDivider != EnergyDividerDefault {
		t.Errorf("default settings not applied: got multiplier %v and divider %v, expected %v and %v",
			client.energyMultiplier, client.energyDivider, EnergyMultiplierDefault, EnergyDividerDefault)
	}
}

func TestCISettingsEmptyFile(t *testing.T) {
	name := t.Name()
	gcas, _, gcaPubkey, gcaPrivKey, err := server.SetupTestEnvironment(name + "_server1")
	if err != nil {
		t.Errorf("unable to set up the test environment for a server: %v", err)
	}
	defer gcas.Close()
	clientDir := glow.GenerateTestDir(name + "_client1")
	err = SetupTestEnvironment(clientDir, gcaPubkey, gcaPrivKey, []*server.GCAServer{gcas})
	if err != nil {
		t.Errorf("unable to set up the client test environment: %v", err)
	}
	// create an empty ct settings file
	ctPath := path.Join(clientDir, CTSettingsFile)
	ctf, err := os.Create(ctPath)
	if err != nil {
		t.Errorf("could not open ct settings file: %v", err)
	}
	ctf.Close()

	// try to start the client
	client, err := NewClient(clientDir)
	if err == nil {
		client.Close()
		t.Errorf("client read the empty ct settings file")
	}
}

func TestCISettingsNonNumericValues(t *testing.T) {
	name := t.Name()
	gcas, _, gcaPubkey, gcaPrivKey, err := server.SetupTestEnvironment(name + "_server1")
	if err != nil {
		t.Errorf("unable to set up the test environment for a server: %v", err)
	}
	defer gcas.Close()
	clientDir := glow.GenerateTestDir(name + "_client1")
	err = SetupTestEnvironment(clientDir, gcaPubkey, gcaPrivKey, []*server.GCAServer{gcas})
	if err != nil {
		t.Errorf("unable to set up the client test environment: %v", err)
	}
	// create a ct settings file with non-numeric values
	ctPath := path.Join(clientDir, CTSettingsFile)
	ctf, err := os.Create(ctPath)
	if err != nil {
		t.Errorf("could not open ct settings file: %v", err)
	}
	ctContent := "not-a-number\nanother-non-number\n"
	_, err = ctf.WriteString(ctContent)
	if err != nil {
		ctf.Close()
		t.Errorf("unable to write ct settings file: %v", err)
	}
	ctf.Close()

	// try to start the client
	client, err := NewClient(clientDir)
	if err == nil {
		client.Close()
		t.Errorf("client read the non-numeric ct settings file")
	}
}

func TestCISettingsLargeValues(t *testing.T) {
	name := t.Name()
	gcas, _, gcaPubkey, gcaPrivKey, err := server.SetupTestEnvironment(name + "_server1")
	if err != nil {
		t.Errorf("unable to set up the test environment for a server: %v", err)
	}
	defer gcas.Close()
	clientDir := glow.GenerateTestDir(name + "_client1")
	err = SetupTestEnvironment(clientDir, gcaPubkey, gcaPrivKey, []*server.GCAServer{gcas})
	if err != nil {
		t.Errorf("unable to set up the client test environment: %v", err)
	}
	// create a ct settings file with large numeric values
	ctPath := path.Join(clientDir, CTSettingsFile)
	ctf, err := os.Create(ctPath)
	if err != nil {
		t.Errorf("could not open ct settings file: %v", err)
	}
	ctContent := "1e10\n1e10\n"
	_, err = ctf.WriteString(ctContent)
	if err != nil {
		ctf.Close()
		t.Errorf("unable to write ct settings file: %v", err)
	}
	ctf.Close()

	// start the client
	client, err := NewClient(clientDir)
	if err != nil {
		t.Errorf("unable to create client: %v", err)
	}
	defer client.Close()

	if client.energyMultiplier != 1e10 || client.energyDivider != 1e10 {
		t.Errorf("unexpected large values: got multiplier %v and divider %v, expected %v and %v",
			client.energyMultiplier, client.energyDivider, 1e10, 1e10)
	}
}
