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
	defer glow.SetCurrentTimeslot(0)

	l2 := "invalid energy value in energy file: random error here"

	// Send a report to the server
	err = updateMonitorFile(client.staticBaseDir, []uint32{1, 5, 10}, []uint64{500, 3000, 5000})
	if err != nil {
		gcas.Close()
		t.Fatal(err)
	}
	time.Sleep(2 * sendReportTime)
	dump := client.DumpEventLogs()

	if strings.Contains(dump, l2) {
		t.Errorf("logs missing: %v", l2)
	}

	// Add a report using the magic number that creates a bad record.
	err = updateMonitorFile(client.staticBaseDir, []uint32{20, 30}, []uint64{20, 34404})
	if err != nil {
		gcas.Close()
		t.Fatal(err)
	}
	time.Sleep(2 * sendReportTime)
	dump = client.DumpEventLogs()
	if !strings.Contains(dump, l2) {
		t.Errorf("logs missing: %v", l2)
	}
}
