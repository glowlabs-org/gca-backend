package client

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/glowlabs-org/gca-backend/glow"
	"github.com/glowlabs-org/gca-backend/server"
)

// TODO: Need to make sure sending multiple reports at once is working.

// TODO: Need to make sure we are correctly handling cases where multiple
// monitoring reports are in the same bin. The strategy there is that the bin
// will always get the first report that appears in that bin. If 2 reports
// appear (or more), the extra report(s) will go into the next bin. We will
// bother with this code after we confirm that the monitoring equipment isn't
// already ensuring that we get exactly one reading per 300 seconds.

// updateMonitorFile is an apparatus that allows the monitor file to be changed
// during testing, simulating a new reading being taken.
func updateMonitorFile(dir string, newTimeslots []uint32, newReadings []uint64) error {
	if len(newTimeslots) != len(newReadings) {
		return fmt.Errorf("incorrect usage, timeslots and readings must have same len")
	}

	// Craft the new file.
	newFileDataStr := "timestamp,energy (mWh)"
	for i := 0; i < len(newTimeslots); i++ {
		newFileDataStr += "\n"
		newFileDataStr += fmt.Sprintf("%v", int64(newTimeslots[i]*300)+glow.GenesisTime)
		newFileDataStr += fmt.Sprintf(",%v", newReadings[i])
	}

	// Write the new file.
	path := filepath.Join(dir, EnergyFile)
	err := ioutil.WriteFile(path, []byte(newFileDataStr), 0644)
	if err != nil {
		return fmt.Errorf("unable to write the new monitor file: %v", err)
	}

	return nil
}

// TestPeriodicMonitoring is a simple test to make sure that the periodic
// monitoring is working.
func TestPeriodicMonitoring(t *testing.T) {
	client, gcas, _, err := FullClientTestEnvironment(t.Name())
	if err != nil {
		t.Fatal(err)
	}
	<-client.syncThread

	// Update the monitoring file so that there is data to read.
	err = updateMonitorFile(client.baseDir, []uint32{1, 5}, []uint64{500, 3000})
	if err != nil {
		t.Fatal(err)
	}

	// Sleep for long enough that the client will send messages to the
	// server based on the readings if everything is working right.
	time.Sleep(2 * sendReportTime)

	// Check whether the server got the reports.
	httpPort, _, _ := gcas.Ports()
	resp, err := http.Get(fmt.Sprintf("http://localhost:%v/api/v1/recent-reports?publicKey=%x", httpPort, client.pubkey))
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
		if i == 1 && report.PowerOutput != 499 {
			t.Fatal("server does not seem to have the report", report.PowerOutput)
		} else if i == 5 && report.PowerOutput != 2999 {
			t.Fatal("server does not seem to have expected report", report.PowerOutput)
		} else if i != 1 && i != 5 && report.PowerOutput != 0 {
			t.Fatal("server has reports we didn't send")
		}
	}

	// Update the monitoring file so that there are now server readings
	// that go back in time a bit. The logic of the reporting thread will
	// not send any readings that it picks up if they are old readings; it
	// assumes that it already sent all of the old readings.
	//
	// This means that we can retroactively add readings during testing to
	// test the sync function, as the only way those readings will get to
	// the server is if the sync function identifies that they are missing
	// and sends them.
	err = updateMonitorFile(client.baseDir, []uint32{1, 2, 5}, []uint64{500, 100, 3000})
	if err != nil {
		t.Fatal(err)
	}

	// Sleep for long enough that the client would send a message for
	// report '2' to the server if we are too late and its caught in the
	// main thread. We do this check because we want to make sure that we
	// are testing the actual sync functions and not picking up a false
	// positive because the normal reporting function picked up the number.
	time.Sleep(2 * sendReportTime)

	// Verify the server had the same reports as before.
	httpPort, _, _ = gcas.Ports()
	resp, err = http.Get(fmt.Sprintf("http://localhost:%v/api/v1/recent-reports?publicKey=%x", httpPort, client.pubkey))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatal("bad status")
	}
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		t.Fatal(err)
	}
	for i, report := range response.Reports {
		if i == 1 && report.PowerOutput != 499 {
			t.Fatal("server does not seem to have the report", report.PowerOutput)
		} else if i == 5 && report.PowerOutput != 2999 {
			t.Fatal("server does not seem to have expected report", report.PowerOutput)
		} else if i != 1 && i != 5 && report.PowerOutput != 0 {
			t.Fatal("server has reports we didn't send")
		}
	}

	// Give the server enough time to execute a sync. We only need to sleep
	// 20 cycles because the first sync check happens 20 ticks after
	// startup, and we haven't spent 20 ticks yet testing things.
	time.Sleep(25 * sendReportTime)

	// Verify the server had the same reports as before.
	httpPort, _, _ = gcas.Ports()
	resp, err = http.Get(fmt.Sprintf("http://localhost:%v/api/v1/recent-reports?publicKey=%x", httpPort, client.pubkey))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatal("bad status")
	}
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		t.Fatal(err)
	}
	for i, report := range response.Reports {
		if i == 1 && report.PowerOutput != 499 {
			t.Fatal("server does not seem to have the report", report.PowerOutput)
		} else if i == 5 && report.PowerOutput != 2999 {
			t.Fatal("server does not seem to have expected report", report.PowerOutput)
		} else if i == 2 && report.PowerOutput != 99 {
			t.Fatal("server does not seem to have expected report", report.PowerOutput)
		} else if i != 1 && i != 5 && i != 2 && report.PowerOutput != 0 {
			t.Fatal("server has reports we didn't send")
		}
	}
}
