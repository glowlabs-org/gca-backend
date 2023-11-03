// The client package implements the code that runs on GCA monitoring
// equipment. The main purpose of this software is to regularly read the output
// of the monitoring sensors (hosted on-device and published to a specific
// file), sign that output, and submit the signed output as a 'report' to the
// GCA servers. TODO
//
// The client is expected to be running on a lightweight IoT device that is
// heavily bandwidth constrained. Most of the reports are submitted over UDP,
// so there needs to be another thread running which checks that the reports
// made it to the GCA server. TODO
//
// Because there's a non-trivial amount of money riding on the reports being
// published, the client needs to maintain a robust list of servers that can be
// used as failover servers in the event that the main GCA server goes down.
// There is therefore a background thread that routinely pings all of the known
// GCA servers to ask them for their list of backups. TODO
//
// Every 6 hours, the client needs to run a routine that detects whether the
// primary GCA server is still operational. If the GCA server is not
// operational, the client needs to failover to one of the backup servers. The
// client considers the primary server to be operational as long as the GCA has
// not issued a ban for the server, and as long as the server is responding to
// the TCP requests to check which reports were submitted successfully. The
// client checks whether a ban has been issued by asking all of the failover
// servers for a list of bans. TODO
//
// When the client fails over to a new server, it'll select a server randomly
// from the backups. That backup will become its new primary. TODO
//
// The GCA can optionally declare that a monitoring device is being migrated to
// a new GCA. The client will have to look for that signal from the GCA. If it
// receives that signal, it trusts the GCA and will move to the new GCA as its
// trusted GCA. When it moves to the new GCA, it will receive a new ShortID. TODO
package main

// TODO: Need to build the systemd services that will automatically restart the
// client if it turns off for some reason.
//
// TODO: Should probably have the ssh port open just in case.

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Step 1: Open the file that contains the keypair for the client/iot
	// + test this
	// Step 2: Open the file that contains all the historic data.
	// add a little header that tells us what time the first reading starts
	// use '0' in the power reading as the sentinal value indiacting that no data is available
	// 4 bytes per reading, leave blank spaces for no data, that way the file is random access for historics
	// Step 3: Kick off the background loop that checks for monitoring data and sends UDP reports
	// Step 4: Kick off the background loop that checks for reports that failed to submit, and checks if a failover is needed
	// Step 5: Kick off the background loop that checks for new failover servers and new banned servers
	// Step 6: Kick off the background loop that checks for migration orders

	// Create a new client, using the current directory as the basedir.
	c, err := NewClient(".")
	if err != nil {
		fmt.Println("unable to create client")
		return
	}

	// Wait for a shutdown signal from the OS.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	// Close the client.
	err = c.Close()
	if err != nil {
		fmt.Println("Issue during shutdown:", err)
	}
}
