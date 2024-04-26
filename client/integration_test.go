package client

// The client package is the package that doesn't have any other dependencies,
// and therefore it's the best place to perform integration tests.
// reports_test.go also has some pretty comprehensive testing.

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glowlabs-org/gca-backend/glow"
	"github.com/glowlabs-org/gca-backend/server"
)

// TestEquipmentHistory checks that the server is correctly collecting
// equipment history, saving the equipment history, and then reloading the
// equipment history.
func TestEquipmentHistory(t *testing.T) {
	// Create a client and a server to perform the test.
	client, gcas, _, err := FullClientTestEnvironment(t.Name())
	gcasDir := gcas.BaseDir()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err := client.Close()
		if err != nil {
			t.Error(err)
		}
	}()
	// This test manipulates the time, so we need to defer a function to
	// reset the time when we are done.
	defer func() {
		glow.SetCurrentTimeslot(0)
	}()

	// Update the monitoring file so that the client submits data to the
	// server.
	err = updateMonitorFile(client.staticBaseDir, []uint32{1, 5}, []uint64{500, 3000})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * sendReportTime)

	// Check that the server got the report.
	httpPort, _, _ := gcas.Ports()
	resp, err := http.Get(fmt.Sprintf("http://localhost:%v/api/v1/recent-reports?publicKey=%x", httpPort, client.staticPubKey))
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
		if i == 1 && report.PowerOutput != 500 {
			t.Error("server does not seem to have the report", report.PowerOutput)
		} else if i == 5 && report.PowerOutput != 3000 {
			t.Error("server does not seem to have expected report", report.PowerOutput)
		} else if i != 1 && i != 5 && report.PowerOutput != 0 {
			t.Error("server has reports we didn't send")
		}
	}

	// Check that the report is available in the history.
	resp, err = http.Get(fmt.Sprintf("http://localhost:%v/api/v1/all-device-stats?timeslot_offset=0", httpPort))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatal("bad status")
	}
	var histResp server.AllDeviceStats
	err = json.NewDecoder(resp.Body).Decode(&histResp)
	if err != nil {
		t.Fatal(err)
	}
	if len(histResp.Devices) != 1 {
		t.Fatal("expected to see one device listed in the device history")
	}
	if histResp.TimeslotOffset != 0 {
		t.Fatal(histResp.TimeslotOffset)
	}
	for i, output := range histResp.Devices[0].PowerOutputs {
		if i == 1 && output != 500 {
			t.Fatal("bad:", i, output)
		} else if i == 5 && output != 3000 {
			t.Fatal("bad")
		} else if i != 1 && i != 5 && output != 0 {
			t.Fatal("bad")
		}
	}

	// Restart the server and check that the histResp is still intact.
	err = gcas.Close()
	if err != nil {
		t.Fatal(err)
	}
	gcas, err = server.NewGCAServer(gcasDir)
	if err != nil {
		t.Fatal(err)
	}
	httpPort, _, _ = gcas.Ports()
	resp, err = http.Get(fmt.Sprintf("http://localhost:%v/api/v1/all-device-stats?timeslot_offset=0", httpPort))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatal("bad status")
	}
	err = json.NewDecoder(resp.Body).Decode(&histResp)
	if err != nil {
		t.Fatal(err)
	}
	if histResp.TimeslotOffset != 0 {
		t.Fatal(histResp.TimeslotOffset)
	}
	if len(histResp.Devices) != 1 {
		t.Fatal("expected to see one device listed in the device history")
	}
	for i, output := range histResp.Devices[0].PowerOutputs {
		if i == 1 && output != 500 {
			t.Fatal("bad")
		} else if i == 5 && output != 3000 {
			t.Fatal("bad")
		} else if i != 1 && i != 5 && output != 0 {
			t.Fatal("bad")
		}
	}

	// Advance time far enough that the history should be saved to disk.
	// Ensure that after this advance, the report is still available in
	// history.
	glow.SetCurrentTimeslot(3500)
	time.Sleep(2 * server.ReportMigrationFrequency)
	resp, err = http.Get(fmt.Sprintf("http://localhost:%v/api/v1/all-device-stats?timeslot_offset=0", httpPort))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		t.Fatal("bad status:", string(body))
	}
	err = json.NewDecoder(resp.Body).Decode(&histResp)
	if err != nil {
		t.Fatal(err)
	}
	if len(histResp.Devices) != 1 {
		t.Fatal("expected to see one device listed in the device history")
	}
	if histResp.TimeslotOffset != 0 {
		t.Fatal(histResp.TimeslotOffset)
	}
	for i, output := range histResp.Devices[0].PowerOutputs {
		if i == 1 && output != 500 {
			t.Fatal("bad")
		} else if i == 5 && output != 3000 {
			t.Fatal("bad")
		} else if i != 1 && i != 5 && output != 0 {
			t.Fatal("bad")
		}
	}

	// Restart the server and check that the report is still available in
	// the history.
	err = gcas.Close()
	if err != nil {
		t.Fatal(err)
	}
	gcas, err = server.NewGCAServer(gcasDir)
	if err != nil {
		t.Fatal(err)
	}
	httpPort, tcpPort, udpPort := gcas.Ports()
	// Update the ports that the client has for this server because the
	// client is actually now out of date. We can't control what ports get
	// assigned during testing so we are just stuck with these invasive
	// sort of fixes.
	client.mu.Lock()
	cSrv := client.gcaServers[gcas.PublicKey()]
	cSrv.TcpPort = tcpPort
	cSrv.UdpPort = udpPort
	client.gcaServers[gcas.PublicKey()] = cSrv
	client.mu.Unlock()
	resp, err = http.Get(fmt.Sprintf("http://localhost:%v/api/v1/all-device-stats?timeslot_offset=0", httpPort))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatal("bad status")
	}
	err = json.NewDecoder(resp.Body).Decode(&histResp)
	if err != nil {
		t.Fatal(err)
	}
	if histResp.TimeslotOffset != 0 {
		t.Fatal(histResp.TimeslotOffset)
	}
	if len(histResp.Devices) != 1 {
		t.Fatal("expected to see one device listed in the device history")
	}
	for i, output := range histResp.Devices[0].PowerOutputs {
		if i == 1 && output != 500 {
			t.Fatal("bad")
		} else if i == 5 && output != 3000 {
			t.Fatal("bad")
		} else if i != 1 && i != 5 && output != 0 {
			t.Fatal("bad")
		}
	}

	// Add some new reports in a new history. There is some reasonably
	// complex code for saving and loading multiple histories, so we want
	// to make sure that everything still works when multiple histories
	// have been saved. The clock has been moved forward, so these reports
	// should be going into the second history.
	err = updateMonitorFile(client.staticBaseDir, []uint32{2016 + 1501, 2016 + 1505}, []uint64{400, 2000})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * sendReportTime)
	// Check the recent reports
	resp, err = http.Get(fmt.Sprintf("http://localhost:%v/api/v1/recent-reports?publicKey=%x", httpPort, client.staticPubKey))
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
		if i == 1501 && report.PowerOutput != 400 {
			t.Error("server does not seem to have the report", report.PowerOutput)
		} else if i == 1505 && report.PowerOutput != 2000 {
			t.Error("server does not seem to have expected report", report.PowerOutput)
		} else if i != 1501 && i != 1505 && report.PowerOutput != 0 {
			t.Error("server has reports we didn't send")
		}
	}
	// Check that the old reports are still available in the old history.
	resp, err = http.Get(fmt.Sprintf("http://localhost:%v/api/v1/all-device-stats?timeslot_offset=0", httpPort))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		t.Fatal("bad status:", string(body))
	}
	err = json.NewDecoder(resp.Body).Decode(&histResp)
	if err != nil {
		t.Fatal(err)
	}
	if histResp.TimeslotOffset != 0 {
		t.Fatal(histResp.TimeslotOffset)
	}
	if len(histResp.Devices) != 1 {
		t.Fatal("expected to see one device listed in the device history")
	}
	for i, output := range histResp.Devices[0].PowerOutputs {
		if i == 1 && output != 500 {
			t.Fatal("bad")
		} else if i == 5 && output != 3000 {
			t.Fatal("bad")
		} else if i != 1 && i != 5 && output != 0 {
			t.Fatal("bad")
		}
	}
	// Check that the new reports are available in the new history.
	resp, err = http.Get(fmt.Sprintf("http://localhost:%v/api/v1/all-device-stats?timeslot_offset=2016", httpPort))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		t.Fatal("bad status:", string(body))
	}
	err = json.NewDecoder(resp.Body).Decode(&histResp)
	if err != nil {
		t.Fatal(err)
	}
	if histResp.TimeslotOffset != 2016 {
		t.Fatal(histResp.TimeslotOffset)
	}
	if len(histResp.Devices) != 1 {
		t.Fatal("expected to see one device listed in the device history")
	}
	for i, output := range histResp.Devices[0].PowerOutputs {
		if i == 1501 && output != 400 {
			t.Fatal("bad")
		} else if i == 1505 && output != 2000 {
			t.Fatal("bad")
		} else if i != 1501 && i != 1505 && output != 0 {
			t.Fatal("bad")
		}
	}

	// Advance time far enough that the new history gets saved to disk as
	// well, then check that all the reports are still available.
	glow.SetCurrentTimeslot(3500 + 2016)
	time.Sleep(2 * server.ReportMigrationFrequency)
	resp, err = http.Get(fmt.Sprintf("http://localhost:%v/api/v1/all-device-stats?timeslot_offset=0", httpPort))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		t.Fatal("bad status:", string(body))
	}
	err = json.NewDecoder(resp.Body).Decode(&histResp)
	if err != nil {
		t.Fatal(err)
	}
	if histResp.TimeslotOffset != 0 {
		t.Fatal(histResp.TimeslotOffset)
	}
	if len(histResp.Devices) != 1 {
		t.Fatal("expected to see one device listed in the device history")
	}
	for i, output := range histResp.Devices[0].PowerOutputs {
		if i == 1 && output != 500 {
			t.Fatal("bad")
		} else if i == 5 && output != 3000 {
			t.Fatal("bad")
		} else if i != 1 && i != 5 && output != 0 {
			t.Fatal("bad")
		}
	}
	resp, err = http.Get(fmt.Sprintf("http://localhost:%v/api/v1/all-device-stats?timeslot_offset=2016", httpPort))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		t.Fatal("bad status:", string(body))
	}
	err = json.NewDecoder(resp.Body).Decode(&histResp)
	if err != nil {
		t.Fatal(err)
	}
	if len(histResp.Devices) != 1 {
		t.Fatal("expected to see one device listed in the device history")
	}
	if histResp.TimeslotOffset != 2016 {
		t.Fatal(histResp.TimeslotOffset)
	}
	for i, output := range histResp.Devices[0].PowerOutputs {
		if i == 1501 && output != 400 {
			t.Error("bad")
		} else if i == 1505 && output != 2000 {
			t.Error("bad")
		} else if i != 1501 && i != 1505 && output != 0 {
			t.Error("bad:", i, output)
		}
	}

	// Restart the server and check that both histories are still
	// available.
	err = gcas.Close()
	if err != nil {
		t.Fatal(err)
	}
	gcas, err = server.NewGCAServer(gcasDir)
	if err != nil {
		t.Fatal(err)
	}
	httpPort, tcpPort, udpPort = gcas.Ports()
	// Update the ports that the client has for this server because the
	// client is actually now out of date. We can't control what ports get
	// assigned during testing so we are just stuck with these invasive
	// sort of fixes.
	client.mu.Lock()
	cSrv = client.gcaServers[gcas.PublicKey()]
	cSrv.TcpPort = tcpPort
	cSrv.UdpPort = udpPort
	client.gcaServers[gcas.PublicKey()] = cSrv
	client.mu.Unlock()
	resp, err = http.Get(fmt.Sprintf("http://localhost:%v/api/v1/all-device-stats?timeslot_offset=0", httpPort))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatal("bad status")
	}
	err = json.NewDecoder(resp.Body).Decode(&histResp)
	if err != nil {
		t.Fatal(err)
	}
	if histResp.TimeslotOffset != 0 {
		t.Fatal(histResp.TimeslotOffset)
	}
	if len(histResp.Devices) != 1 {
		t.Fatal("expected to see one device listed in the device history")
	}
	for i, output := range histResp.Devices[0].PowerOutputs {
		if i == 1 && output != 500 {
			t.Fatal("bad")
		} else if i == 5 && output != 3000 {
			t.Fatal("bad")
		} else if i != 1 && i != 5 && output != 0 {
			t.Fatal("bad")
		}
	}
	resp, err = http.Get(fmt.Sprintf("http://localhost:%v/api/v1/all-device-stats?timeslot_offset=2016", httpPort))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		t.Fatal("bad status:", string(body))
	}
	err = json.NewDecoder(resp.Body).Decode(&histResp)
	if err != nil {
		t.Fatal(err)
	}
	if len(histResp.Devices) != 1 {
		t.Fatal("expected to see one device listed in the device history")
	}
	if histResp.TimeslotOffset != 2016 {
		t.Fatal(histResp.TimeslotOffset)
	}
	for i, output := range histResp.Devices[0].PowerOutputs {
		if i == 1501 && output != 400 {
			t.Error("bad")
		} else if i == 1505 && output != 2000 {
			t.Error("bad")
		} else if i != 1501 && i != 1505 && output != 0 {
			t.Error("bad:", i, output)
		}
	}

	// One more round of adding new reports in a new history.
	err = updateMonitorFile(client.staticBaseDir, []uint32{2*2016 + 1501, 2*2016 + 1505}, []uint64{300, 1000})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * sendReportTime)
	// Check the recent reports
	resp, err = http.Get(fmt.Sprintf("http://localhost:%v/api/v1/recent-reports?publicKey=%x", httpPort, client.staticPubKey))
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
		if i == 1501 && report.PowerOutput != 300 {
			t.Error("server does not seem to have the report", report.PowerOutput)
		} else if i == 1505 && report.PowerOutput != 1000 {
			t.Error("server does not seem to have expected report", report.PowerOutput)
		} else if i != 1501 && i != 1505 && report.PowerOutput != 0 {
			t.Error("server has reports we didn't send")
		}
	}
	resp, err = http.Get(fmt.Sprintf("http://localhost:%v/api/v1/all-device-stats?timeslot_offset=%d", httpPort, 2016*2))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		t.Fatal("bad status:", string(body))
	}
	err = json.NewDecoder(resp.Body).Decode(&histResp)
	if err != nil {
		t.Fatal(err)
	}
	if len(histResp.Devices) != 1 {
		t.Fatal("expected to see one device listed in the device history")
	}
	if histResp.TimeslotOffset != 2016*2 {
		t.Fatal(histResp.TimeslotOffset)
	}
	for i, output := range histResp.Devices[0].PowerOutputs {
		if i == 1501 && output != 300 {
			t.Error("bad")
		} else if i == 1505 && output != 1000 {
			t.Error("bad")
		} else if i != 1501 && i != 1505 && output != 0 {
			t.Error("bad:", i, output)
		}
	}
	// Bury the new reports into history.
	glow.SetCurrentTimeslot(3500 + 2*2016)
	time.Sleep(2 * server.ReportMigrationFrequency)

	// Restart the server and check that all the history is there.
	err = gcas.Close()
	if err != nil {
		t.Fatal(err)
	}
	gcas, err = server.NewGCAServer(gcasDir)
	if err != nil {
		t.Fatal(err)
	}
	httpPort, tcpPort, udpPort = gcas.Ports()
	// Update the ports that the client has for this server because the
	// client is actually now out of date. We can't control what ports get
	// assigned during testing so we are just stuck with these invasive
	// sort of fixes.
	client.mu.Lock()
	cSrv = client.gcaServers[gcas.PublicKey()]
	cSrv.TcpPort = tcpPort
	cSrv.UdpPort = udpPort
	client.gcaServers[gcas.PublicKey()] = cSrv
	client.mu.Unlock()
	resp, err = http.Get(fmt.Sprintf("http://localhost:%v/api/v1/all-device-stats?timeslot_offset=0", httpPort))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatal("bad status")
	}
	err = json.NewDecoder(resp.Body).Decode(&histResp)
	if err != nil {
		t.Fatal(err)
	}
	if histResp.TimeslotOffset != 0 {
		t.Fatal(histResp.TimeslotOffset)
	}
	if len(histResp.Devices) != 1 {
		t.Fatal("expected to see one device listed in the device history")
	}
	for i, output := range histResp.Devices[0].PowerOutputs {
		if i == 1 && output != 500 {
			t.Fatal("bad")
		} else if i == 5 && output != 3000 {
			t.Fatal("bad")
		} else if i != 1 && i != 5 && output != 0 {
			t.Fatal("bad")
		}
	}
	resp, err = http.Get(fmt.Sprintf("http://localhost:%v/api/v1/all-device-stats?timeslot_offset=2016", httpPort))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		t.Fatal("bad status:", string(body))
	}
	err = json.NewDecoder(resp.Body).Decode(&histResp)
	if err != nil {
		t.Fatal(err)
	}
	if len(histResp.Devices) != 1 {
		t.Fatal("expected to see one device listed in the device history")
	}
	if histResp.TimeslotOffset != 2016 {
		t.Fatal(histResp.TimeslotOffset)
	}
	for i, output := range histResp.Devices[0].PowerOutputs {
		if i == 1501 && output != 400 {
			t.Error("bad")
		} else if i == 1505 && output != 2000 {
			t.Error("bad")
		} else if i != 1501 && i != 1505 && output != 0 {
			t.Error("bad:", i, output)
		}
	}
	resp, err = http.Get(fmt.Sprintf("http://localhost:%v/api/v1/all-device-stats?timeslot_offset=%d", httpPort, 2016*2))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		t.Fatal("bad status:", string(body))
	}
	err = json.NewDecoder(resp.Body).Decode(&histResp)
	if err != nil {
		t.Fatal(err)
	}
	if len(histResp.Devices) != 1 {
		t.Fatal("expected to see one device listed in the device history")
	}
	if histResp.TimeslotOffset != 2016*2 {
		t.Fatal(histResp.TimeslotOffset)
	}
	for i, output := range histResp.Devices[0].PowerOutputs {
		if i == 1501 && output != 300 {
			t.Error("bad")
		} else if i == 1505 && output != 1000 {
			t.Error("bad")
		} else if i != 1501 && i != 1505 && output != 0 {
			t.Error("bad:", i, output)
		}
	}

	// Sanity test of the event logging.
	path := filepath.Join(client.staticBaseDir, "event.log")
	os.WriteFile(path, []byte(client.Log.DumpLog()), 0644)
}
