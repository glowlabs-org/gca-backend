package client

import (
	"strings"
	"testing"
	"time"

	"github.com/glowlabs-org/gca-backend/glow"
)

func TestEventLogIntegration(t *testing.T) {
	client, gcas, _, err := FullClientTestEnvironment(t.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	defer gcas.Close()
	defer glow.SetCurrentTimeslot(0)

	// send a report to the server
	err = updateMonitorFile(client.staticBaseDir, []uint32{1, 5}, []uint64{500, 3000})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * sendReportTime)

	// Search for some key strings in the event log dump
	str := client.DumpStatus()
	keys := []string{"udp send ShortID 1 Timeslot 1 PowerOutput 500", "udp send ShortID 1 Timeslot 5 PowerOutput 3000"}
	for _, k := range keys {
		if c := strings.Count(str, k); c != 1 {
			t.Fatalf("event string occurred %v times, should be 1: %v", c, k)
		}
	}
}
