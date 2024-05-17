package client

import (
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

	// Send a report to the server, using the magic number that creates a bad record
	err = updateMonitorFile(client.staticBaseDir, []uint32{1, 5, 10}, []uint64{500, 3000, 34404})
	if err != nil {
		gcas.Close()
		t.Fatal(err)
	}
	time.Sleep(2 * sendReportTime)

	t.Log(client.DumpEventLogs())
}
