package client

import (
	"bytes"
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

// updateMonitorFile is an apparatus that allows the monitor file to be changed
// during testing, simulating a new reading being taken.
func updateMonitorFile(dir string, newTimeslots []uint32, newReadings []uint64) error {
	if len(newTimeslots) != len(newReadings) {
		return fmt.Errorf("incorrect usage, timeslots and readings must have same len")
	}

	// Craft the new file.
	newFileDataStr := "timestamp,energy (mWh)"
	for i := 0; i < len(newTimeslots); i++ {
		if newReadings[i] != 34404 {
			newFileDataStr += fmt.Sprintf("\n%v,%v", int64(newTimeslots[i]*300)+glow.GenesisTime, newReadings[i])
		} else {
			newFileDataStr += fmt.Sprintf("\n%v,random error here", int64(newTimeslots[i]*300)+glow.GenesisTime)
		}
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
	<-client.started
	defer func() {
		err := client.Close()
		if err != nil {
			t.Error(err)
		}
	}()

	// Update the monitoring file so that there is data to read.
	err = updateMonitorFile(client.staticBaseDir, []uint32{1, 5}, []uint64{500, 3000})
	if err != nil {
		t.Fatal(err)
	}

	// Sleep for long enough that the client will send messages to the
	// server based on the readings if everything is working right.
	time.Sleep(2 * sendReportTime)

	// Check whether the server got the reports.
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
	err = updateMonitorFile(client.staticBaseDir, []uint32{1, 2, 5, 6}, []uint64{500, 100, 3000, 34404})
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
		if i == 1 && report.PowerOutput != 499 {
			t.Fatal("server does not seem to have the report", report.PowerOutput)
		} else if i == 5 && report.PowerOutput != 2999 {
			t.Fatal("server does not seem to have expected report", report.PowerOutput)
		} else if i != 1 && i != 5 && report.PowerOutput != 0 {
			t.Fatal("server has reports we didn't send")
		}
	}

	// Give the server enough time to execute a sync. The server needs
	// about 20 cycles to execute a sync.
	time.Sleep(25 * sendReportTime)

	// Verify the server had the same reports as before.
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

// TestAddingServers has a goal of spinning up multiple servers for a hardware
// device, and then adding them one at a time and ensuring that the hardware
// device is able to gracefully transition from one to another.
func TestAddingServers(t *testing.T) {
	// Set up the first server.
	gcas1, _, gcaPubKey, gcaPrivKey, err := server.SetupTestEnvironment(t.Name() + "_server1")
	if err != nil {
		t.Fatal(err)
	}
	// Set up the second server.
	gcas2, _, err := server.SetupTestEnvironmentKnownGCA(t.Name()+"_server2", gcaPubKey, gcaPrivKey)
	if err != nil {
		t.Fatal(err)
	}

	// Create a client that can submit reports to either server.
	clientDir := glow.GenerateTestDir(t.Name() + "_client1")
	err = SetupTestEnvironment(clientDir, gcaPubKey, gcaPrivKey, []*server.GCAServer{gcas1, gcas2})
	if err != nil {
		t.Fatal(err)
	}

	// Create the client and immediately close it, which will ensure that
	// even having 2 servers at all is useful.
	c, err := NewClient(clientDir)
	if err != nil {
		t.Fatal(err)
	}
	// Ensure that the client is running properly.
	<-c.started

	// Update the monitoring file for the client so that the client has
	// stuff to report.
	err = updateMonitorFile(c.staticBaseDir, []uint32{1, 5}, []uint64{500, 3000})
	if err != nil {
		t.Fatal(err)
	}

	// Give the client some time to tick.
	time.Sleep(2 * sendReportTime)

	// Ensure that at least one of the servers got a report.
	httpPort1, _, _ := gcas1.Ports()
	resp, err := http.Get(fmt.Sprintf("http://localhost:%v/api/v1/recent-reports?publicKey=%x", httpPort1, c.staticPubKey))
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
	gcas1HasReports := true
	for i, report := range response.Reports {
		if i == 1 && report.PowerOutput != 499 {
			gcas1HasReports = false
		} else if i == 5 && report.PowerOutput != 2999 {
			gcas1HasReports = false
		} else if i != 1 && i != 5 && report.PowerOutput != 0 {
			gcas1HasReports = false
		}
	}
	httpPort2, _, _ := gcas2.Ports()
	resp, err = http.Get(fmt.Sprintf("http://localhost:%v/api/v1/recent-reports?publicKey=%x", httpPort2, c.staticPubKey))
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
	gcas2HasReports := true
	for i, report := range response.Reports {
		if i == 1 && report.PowerOutput != 499 {
			gcas2HasReports = false
		} else if i == 5 && report.PowerOutput != 2999 {
			gcas2HasReports = false
		} else if i != 1 && i != 5 && report.PowerOutput != 0 {
			gcas2HasReports = false
		}
	}
	if !gcas1HasReports && !gcas2HasReports {
		t.Fatal("The client does not seem to be correctly submitting reports to one of the servers")
	}

	// Shut down gcas1, then update the monitoring file. The client should
	// failover to gcas2 and continue submitting reports. Half the time,
	// the client will actually already be on client2, so this test may
	// occasionally have a false positive pass if things are broken. To be
	// certain, the test should be run at least 3 times, and to be really
	// certain 20 times is a better number.
	//
	// We have to sleep for a large number of ticks because the failover
	// code only runs once every 60 ticks, but the client starts with the
	// tick counter at 30.
	err = gcas1.Close()
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(35 * sendReportTime)

	// The client should have successfully failed over at this point, even
	// though it has nothing to report. Let's give it something to report,
	// and then see if the report lands on gcas2.
	err = updateMonitorFile(c.staticBaseDir, []uint32{2, 6}, []uint64{550, 3500})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * sendReportTime)
	resp, err = http.Get(fmt.Sprintf("http://localhost:%v/api/v1/recent-reports?publicKey=%x", httpPort2, c.staticPubKey))
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
		if i == 2 || i == 5 {
			// Reports 2 and 5 may or may not have been sent.
			continue
		}
		if i == 1 && report.PowerOutput != 499 {
			t.Fatal("expected power report")
		} else if i == 5 && report.PowerOutput != 2999 {
			t.Fatal("expected power report", report.PowerOutput)
		} else if i == 6 && report.PowerOutput != 3499 {
			t.Fatal("expected power report")
		} else if i != 1 && i != 5 && i != 2 && i != 6 && report.PowerOutput != 0 {
			t.Fatal("expected no power report")
		}
	}

	// Check that the all-device-stats endpoint is listing out the client
	// and the corresponding reports.
	resp, err = http.Get(fmt.Sprintf("http://localhost:%v/api/v1/all-device-stats?timeslot_offset=0", httpPort2))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		t.Fatal("bad status:", string(body), err)
	}
	var statsResp server.AllDeviceStats
	err = json.NewDecoder(resp.Body).Decode(&statsResp)
	if err != nil {
		t.Fatal(err)
	}
	if len(statsResp.Devices) != 1 {
		t.Fatal("expecting 1 device in the stats:", len(statsResp.Devices))
	}
	// Check that there are non-zero values in the output.
	isData := false
	for _, ir := range statsResp.Devices[0].ImpactRates {
		if ir != 0 {
			isData = true
		}
	}
	if !isData {
		t.Fatal("Expecting at least some IR values to have accrued, but there is nothing")
	}
	// Verify the signature.
	sb := statsResp.SigningBytes()
	if !glow.Verify(gcas2.PublicKey(), sb, statsResp.Signature) {
		t.Fatal("signature mismatch on the AllDeviceStats object")
	}

	// Try restarting the client, make sure it can still submit reports to
	// gcas2. It will need to potentially go through a sync to find gcas2
	// as a viable option for submitting reports.
	err = c.Close()
	if err != nil {
		t.Fatal(err)
	}
	c, err = NewClient(clientDir)
	if err != nil {
		t.Fatal(err)
	}
	<-c.started

	// Sleep long enough to let a sync happen.
	time.Sleep(35 * sendReportTime)

	// Update the monitor file so that the client has data to send to gcas2.
	err = updateMonitorFile(c.staticBaseDir, []uint32{3, 4, 7}, []uint64{55, 59, 1200})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * sendReportTime)
	resp, err = http.Get(fmt.Sprintf("http://localhost:%v/api/v1/recent-reports?publicKey=%x", httpPort2, c.staticPubKey))
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
			t.Fatal("expected power report")
		} else if i == 2 && report.PowerOutput != 549 {
			t.Fatal("expected power report")
		} else if i == 5 && report.PowerOutput != 2999 {
			t.Fatal("expected power report")
		} else if i == 6 && report.PowerOutput != 3499 {
			t.Fatal("expected power report")
		} else if i == 7 && report.PowerOutput != 1199 {
			t.Fatal("expected power report")
		} else if (i < 1 || i > 8) && report.PowerOutput != 0 {
			t.Fatal("expected no power report")
		}
	}

	// Bring up gcas3, submit it as a new server to gcas2, give the client
	// time to sync and see that gcas3 exists, then shut down gcas2 and see
	// if the client is able to properly failover to gcas3.
	gcas3, _, err := server.SetupTestEnvironmentKnownGCA(t.Name()+"_server3", gcaPubKey, gcaPrivKey)
	if err != nil {
		t.Fatal(err)
	}
	// Submit gcas3 to gcas2 so that the client, when it syncs, will see
	// the new server.
	httpPort3, tcpPort3, udpPort3 := gcas3.Ports()
	as := server.AuthorizedServer{
		PublicKey: gcas3.PublicKey(),
		Banned:    false,
		Location:  "localhost",
		HttpPort:  httpPort3,
		TcpPort:   tcpPort3,
		UdpPort:   udpPort3,
	}
	sb = as.SigningBytes()
	sig := glow.Sign(sb, gcaPrivKey)
	as.GCAAuthorization = sig
	// Create the http request for gcas2
	requestBody, err := json.Marshal(as)
	if err != nil {
		t.Fatal(err)
	}
	resp, err = http.Post(fmt.Sprintf("http://localhost:%v/api/v1/authorized-servers", httpPort2), "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatal("bad status code:", resp.StatusCode)
	}
	err = resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Close the client, re-open the client, then wait for a sync
	// operation. Client will have to sync with gcas2 because that's the
	// only server it knows that is online.
	err = c.Close()
	if err != nil {
		t.Fatal(err)
	}
	c, err = NewClient(clientDir)
	if err != nil {
		t.Fatal(err)
	}
	<-c.started
	time.Sleep(35 * sendReportTime)

	// Client should now have gcas3 as a failover server. Close both the
	// client and gcas2. Then bring the client back up, the only server it
	// will know is gcas3. Add some new data, and see if that new data
	// makes it to gcas3.
	err = gcas2.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = c.Close()
	if err != nil {
		t.Fatal(err)
	}
	c, err = NewClient(clientDir)
	if err != nil {
		t.Fatal(err)
	}
	<-c.started
	time.Sleep(35 * sendReportTime)

	// Add some new data that can be reported.
	err = updateMonitorFile(c.staticBaseDir, []uint32{8}, []uint64{1800})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * sendReportTime)
	resp, err = http.Get(fmt.Sprintf("http://localhost:%v/api/v1/recent-reports?publicKey=%x", httpPort3, c.staticPubKey))
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
			t.Error("expected power report")
		} else if i == 2 && report.PowerOutput != 549 {
			t.Error("expected power report")
		} else if i == 3 && report.PowerOutput != 54 {
			t.Error("expected power report")
		} else if i == 4 && report.PowerOutput != 58 {
			t.Error("expected power report")
		} else if i == 5 && report.PowerOutput != 2999 {
			t.Error("expected power report")
		} else if i == 6 && report.PowerOutput != 3499 {
			t.Error("expected power report")
		} else if i == 7 && report.PowerOutput != 1199 {
			t.Error("expected power report")
		} else if i == 8 && report.PowerOutput != 1799 {
			t.Error("expected power report")
		} else if (i < 1 || i > 8) && report.PowerOutput != 0 {
			t.Error("expected no power report")
		}
	}

	// Create a new GCA with a new GCA server, and execute a migration of
	// the client from the old GCA to the new GCA.
	gcasA, _, ngcaPubKey, ngcaPrivKey, err := server.SetupTestEnvironment(t.Name() + "_serverA")
	if err != nil {
		t.Fatal(err)
	}
	httpPortA, tcpPortA, udpPortA := gcasA.Ports()

	// Create an authorization for the client on the new GCA.
	shortID := uint32(135) // Use a different short id for the new GCA to make sure the migration is correct.
	ea := glow.EquipmentAuthorization{
		ShortID:    shortID,
		PublicKey:  c.staticPubKey,
		Latitude:   38,
		Longitude:  -100,
		Capacity:   12341234,
		Debt:       11223344,
		Expiration: 100e6 + glow.CurrentTimeslot(),
	}
	sb = ea.SigningBytes()
	sig = glow.Sign(sb, ngcaPrivKey)
	ea.Signature = sig
	jsonEA, err := json.Marshal(ea)
	if err != nil {
		t.Fatal("unable to marshal the auth request")
	}
	resp, err = http.Post(fmt.Sprintf("http://localhost:%v/api/v1/authorize-equipment", httpPortA), "application/json", bytes.NewBuffer(jsonEA))
	if err != nil {
		t.Fatal("unable to authorize device on GCA server:", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatal("got non-OK status code when authorizing gca client")
	}
	err = resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Create and submit the migration object.
	as = server.AuthorizedServer{
		PublicKey: gcasA.PublicKey(),
		Banned:    false,
		Location:  "localhost",
		HttpPort:  httpPortA,
		TcpPort:   tcpPortA,
		UdpPort:   udpPortA,
	}
	sb = as.SigningBytes()
	as.GCAAuthorization = glow.Sign(sb, ngcaPrivKey)
	em := server.EquipmentMigration{
		Equipment:  c.staticPubKey,
		NewGCA:     ngcaPubKey,
		NewShortID: 135,
		NewServers: []server.AuthorizedServer{as},
	}
	sb = em.SigningBytes()
	em.Signature = glow.Sign(sb, gcaPrivKey)
	jsonEM, err := json.Marshal(em)
	if err != nil {
		t.Fatal(err)
	}
	resp, err = http.Post(fmt.Sprintf("http://localhost:%v/api/v1/equipment-migrate", httpPort3), "application/json", bytes.NewBuffer(jsonEM))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatal("got bad code")
	}
	err = resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Get the client to perform a sync and see if it picks up the
	// migration.
	err = c.Close()
	if err != nil {
		t.Fatal(err)
	}
	c, err = NewClient(clientDir)
	if err != nil {
		t.Fatal(err)
	}
	<-c.started
	time.Sleep(35 * sendReportTime)

	// At this point, the client should have picked up the migration, but
	// we now need to trigger an actual sync with the new GCA server.
	err = c.Close()
	if err != nil {
		t.Fatal(err)
	}
	c, err = NewClient(clientDir)
	if err != nil {
		t.Fatal(err)
	}
	<-c.started
	time.Sleep(35 * sendReportTime)

	// Update the monitor file as well for good measure.
	err = updateMonitorFile(c.staticBaseDir, []uint32{9}, []uint64{800})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * sendReportTime)
	resp, err = http.Get(fmt.Sprintf("http://localhost:%v/api/v1/recent-reports?publicKey=%x", httpPortA, c.staticPubKey))
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
			t.Error("expected power report")
		} else if i == 2 && report.PowerOutput != 549 {
			t.Error("expected power report")
		} else if i == 3 && report.PowerOutput != 54 {
			t.Error("expected power report")
		} else if i == 4 && report.PowerOutput != 58 {
			t.Error("expected power report")
		} else if i == 5 && report.PowerOutput != 2999 {
			t.Error("expected power report")
		} else if i == 6 && report.PowerOutput != 3499 {
			t.Error("expected power report")
		} else if i == 7 && report.PowerOutput != 1199 {
			t.Error("expected power report")
		} else if i == 8 && report.PowerOutput != 1799 {
			t.Error("expected power report")
		} else if i == 9 && report.PowerOutput != 799 {
			t.Error("expected power report")
		} else if (i < 1 || i > 9) && report.PowerOutput != 0 {
			t.Error("expected no power report")
		}
	}

	// Check the old GCA, which should not be receiving reports anymore.
	resp, err = http.Get(fmt.Sprintf("http://localhost:%v/api/v1/recent-reports?publicKey=%x", httpPort3, c.staticPubKey))
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
			t.Error("expected power report")
		} else if i == 2 && report.PowerOutput != 549 {
			t.Error("expected power report")
		} else if i == 3 && report.PowerOutput != 54 {
			t.Error("expected power report")
		} else if i == 4 && report.PowerOutput != 58 {
			t.Error("expected power report")
		} else if i == 5 && report.PowerOutput != 2999 {
			t.Error("expected power report")
		} else if i == 6 && report.PowerOutput != 3499 {
			t.Error("expected power report")
		} else if i == 7 && report.PowerOutput != 1199 {
			t.Error("expected power report")
		} else if i == 8 && report.PowerOutput != 1799 {
			t.Error("expected power report")
		} else if (i < 1 || i > 8) && report.PowerOutput != 0 {
			t.Error("expected no power report")
		}
	}

	// TODO: We have to test banning servers. That one is going to take a
	// while. Maybe 30 seconds even. Perhaps after launch.
}
