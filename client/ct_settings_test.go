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
	ctContent := "2000\n4000\n" // this will set energy multipler to 2000, divider to 4000
	_, err = ctf.WriteString(ctContent)
	if err != nil {
		ctf.Close()
		t.Errorf("unable to write ct settings file: %v", err)
	}
	// start the client
	client, err := NewClient(clientDir)
	if err != nil {
		t.Errorf("unable to create client: %v", err)
	}
	defer client.Close()

	// Update the monitoring file so that the client submits data to the
	// server.
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
