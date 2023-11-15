package client

// reports.go contains all of the code for sending reports to the server.
//
// TODO: We will have to create a simlink between the client directory's
// 'energy_data.csv' and the '/opt/halki/energy_data.csv' file.

// TODO: Add concurrency testing. Since the client doesn't have APIs, the
// concurrency testing can be a bit less intensive than for when there's an API
// to bash.

// TODO: Need to add testing around how banned servers get handled.

// TODO: Need to pick a random offset that is preferred for sending new reports
// to the server, to avoid overwhelming the server DoS style.

// TODO: The test suite needs to have some optional randomization on the
// reports sending so that we can sometimes control the report not to send,
// simulating a UDP failure.

// Remaining for tomorrow:
//
// 1. Get multiple servers so the hardware can have them
// 2. Configure the first hardware files (probably by hand)
// 3. Contemplate writing code to grab new GCA servers from the existing one
// 4. Contemplate writing code to complete GCA migration
//
// TODO TODO TODO

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/csv"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/glowlabs-org/gca-backend/glow"
)

// Represents one row of data from the energy file.
type EnergyRecord struct {
	Timeslot uint32
	Energy   uint64
}

// Create an energy report from the provided energy record.
func (c *Client) staticSendReport(gcas GCAServer, er EnergyRecord) {
	eqr := glow.EquipmentReport{
		ShortID:     c.shortID,
		Timeslot:    er.Timeslot,
		PowerOutput: er.Energy,
	}
	sb := eqr.SigningBytes()
	eqr.Signature = glow.Sign(sb, c.privkey)
	data := eqr.Serialize()
	location := fmt.Sprintf("%v:%v", gcas.Location, gcas.UdpPort)
	glow.SendUDPReport(data, location)
}

// readEnergyFile will read the data from the energy file and return an array
// that contains all of the values.
func (c *Client) readEnergyFile() ([]EnergyRecord, error) {
	// Open the CSV file. We do a quick conditional switch because in prod
	// the energy file is actually located in an absolute location rather
	// than being part of the energy monitor directory.
	filePath := EnergyFile
	if EnergyFile[0] != '/' {
		filePath = path.Join(c.baseDir, EnergyFile)
	}
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("unable to open monitoring file: %v", err)
	}
	defer file.Close()

	// Iterate over the CSV records
	reader := csv.NewReader(file)
	var records []EnergyRecord
	for {
		record, err := reader.Read()
		if err != nil {
			break // Stop at EOF or on error
		}

		timestamp, err := strconv.ParseInt(record[0], 10, 64)
		if err != nil {
			continue // Skip records with invalid timestamps
		}
		timeslot, err := glow.UnixToTimeslot(timestamp)
		if err != nil {
			continue
		}
		energy, err := strconv.ParseUint(record[1], 10, 32)
		if err != nil {
			energy = 0
		}

		// Round the energy down so that we never over-estiamte the amount of
		// power that has been produced.
		energy = energy - 1

		// 0, 1, and 2 are reserved sentinel values, so we just skip this
		// reading if we are in that range.
		if energy < 3 {
			continue
		}

		// Append the data to the records slice
		records = append(records, EnergyRecord{
			Timeslot: timeslot,
			Energy:   uint64(energy),
		})
	}

	return records, nil
}

// c.staticServerReadings is a wrapper for the networking call that happens in
// threadedSyncServer, it allows us to isolate failures which indicate that a
// failover needs to happen.
func (c *Client) staticServerReadings(gcas GCAServer, gcasKey glow.PublicKey) (timeslotOffset uint32, bitfield [504]byte, err error) {
	// The first step is to make a connection to the server and download
	// the list of reports that it current has.
	location := fmt.Sprintf("%v:%v", gcas.Location, gcas.TcpPort)
	conn, err := net.Dial("tcp", location)
	if err != nil {
		return 0, [504]byte{}, fmt.Errorf("unable to dial the gca server")
	}
	defer conn.Close()

	// Send the request.
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], c.shortID)
	_, err = conn.Write(buf[:])
	if err != nil {
		return 0, [504]byte{}, fmt.Errorf("unable to send reqeust to gca server")
	}

	// Receive the response.
	var respBuf [32 + 4 + 504 + 8 + 64]byte
	_, err = io.ReadFull(conn, respBuf[:])
	if err != nil {
		return 0, [504]byte{}, fmt.Errorf("unable to read response from gca server")
	}

	// Verify the signature.
	signingTime := binary.BigEndian.Uint64(respBuf[32+4+504:])
	now := uint64(time.Now().Unix())
	if now+24*3600 < signingTime || now-24*3600 > signingTime {
		return 0, [504]byte{}, fmt.Errorf("received response from server that is out of bounds temporally")
	}
	var sig glow.Signature
	copy(sig[:], respBuf[32+4+504+8:])
	if !glow.Verify(gcasKey, respBuf[:32+4+504+8], sig) {
		return 0, [504]byte{}, fmt.Errorf("received response from server with invalid signature")
	}

	// Extract the timeslot offset and bitfield.
	copy(bitfield[:], respBuf[36:540])
	timeslotOffset = binary.BigEndian.Uint32(respBuf[32:36])
	return timeslotOffset, bitfield, nil
}

// syncWithServer will make a request to the server to figure out which reports
// did not successfully get received by the server, and it will re-send those
// reports.
//
// If the function fails to complete a successful sync operation with the
// server, it will attempt to migrate to a new server.
func (c *Client) threadedSyncWithServer(latestReading uint32) {
	// Grab the state we need from the client under the safety of a mutex.
	c.serverMu.Lock()
	gcas := c.gcaServers[c.primaryServer]
	gcasKey := c.primaryServer
	c.serverMu.Unlock()

	// Perform the network call
	timeslotOffset, bitfield, err := c.staticServerReadings(gcas, gcasKey)
	if err != nil {
		// Try up to 5 times to get a successful interaction with a
		// server. Wait 10 ticks between each attempt.
		for i := 0; i < 6; i++ {
			if i == 5 {
				// Give up entirely after 3 attempts. We use
				// this control structure to exit the loop so
				// that we can have relevant bits of logic
				// after the loop which assume that a
				// connection was made successfully.
				return
			}

			// Sleep for 2 ticks.
			select {
			case <-c.closeChan:
				return
			case <-time.After(2 * sendReportTime):
			}

			// Pick a new random primary server. The steps are:
			//
			// 1. Pull all the servers into an array.
			// 2. Randomly shuffle the array.
			// 3. Select the first non-banned server.
			c.serverMu.Lock()
			servers := make([]glow.PublicKey, 0, len(c.gcaServers))
			for server, _ := range c.gcaServers {
				servers = append(servers, server)
			}
			for i := range servers {
				j, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
				if err != nil {
					continue
				}
				servers[i], servers[j.Int64()] = servers[j.Int64()], servers[i]
			}
			for _, server := range servers {
				if !c.gcaServers[server].Banned {
					c.primaryServer = server
					break
				}
			}
			gcas = c.gcaServers[c.primaryServer]
			gcasKey = c.primaryServer
			c.serverMu.Unlock()

			// Retry grabbing the bitfield.
			timeslotOffset, bitfield, err = c.staticServerReadings(gcas, gcasKey)
			if err == nil {
				break
			}
		}
	}

	// Scan through the bitfield
	// productive.
	lastIndex := latestReading - timeslotOffset
	for i := uint32(0); i < lastIndex && int(i)/8 < len(bitfield); i++ {
		if bitfield[i/8]&(1<<(i%8)) == 0 {
			// Server is missing this index, check locally to see
			// if we have it, and send it if we do. The server will
			// ignore any power output below 2, so we ignore those
			// values as well as errors.
			powerOutput, err := c.staticLoadReading(i + timeslotOffset)
			if err != nil || powerOutput < 2 {
				continue
			}
			record := EnergyRecord{
				Timeslot: i + timeslotOffset,
				Energy:   uint64(powerOutput),
			}
			c.staticSendReport(gcas, record)
		}
	}
}

// threadedSendReports will wake up every minute, check whether there's a new
// report available, and if so it'll send a report for the corresponding
// timeslot.
//
// TODO: Need to pick an arbitrary point within the 5 minute period to send
// data to the server so that the devices are spread out nicely.
func (c *Client) threadedSendReports() {
	// Right at startup, we save all of the existing records. We don't
	// bother sending them because we assume we already sent them, and if
	// we haven't already sent them, the periodic synchronization will fix
	// it up.
	latestRecord := uint32(0)
	records, err := c.readEnergyFile()
	// We'll no-op if there's an error. One error that gets caught is if
	// the monitoring equipment saves a duplicate reading. The monitoring
	// equipment shouldn't have this issue, because it should be doing
	// everything using Unix timestamps and therefore be immune to timezone
	// and daylight savings related issues, but if the hardware clock
	// messes up somehow we would rather ignore the error.
	if err == nil {
		for _, record := range records {
			err := c.saveReading(record.Timeslot, uint32(record.Energy))
			if err != nil {
				continue
			}
			if record.Timeslot > latestRecord {
				latestRecord = record.Timeslot
			}
		}
	}

	// Infinite loop to send reports. We start ticks at 270 so that the
	// catchup function will run about 20 minutes after boot. We don't want
	// to run it immediately after boot because if we get stuck in a short
	// boot loop scenario, we don't want to blow all of our bandwidth doing
	// sync operations.
	ticks := 270
	close(c.syncThread)
	for {
		// Quit if the closeChan was closed.
		select {
		case <-c.closeChan:
			return
		default:
		}

		// Grab the gca server for use when sending the report.
		c.serverMu.Lock()
		gcas := c.gcaServers[c.primaryServer]
		c.serverMu.Unlock()

		// Read the energy file. No-op if there's an error. Can't
		// continue because we still want to sleep.
		records, err := c.readEnergyFile()
		if err == nil {
			for _, record := range records {
				// We try saving the reading first, which can
				// produce an error. The main error that we are
				// looking for is a double report error, which
				// means the same timeslot has multiple
				// different energy readings. That's a problem
				// that will cause the timeslot to get banned,
				// so we don't send the report if that happens.
				err := c.saveReading(record.Timeslot, uint32(record.Energy))
				if err != nil {
					continue
				}
				if record.Timeslot > latestRecord {
					c.staticSendReport(gcas, record)
				}
			}
			// The above loop doesn't update the latestRecord
			// because if there are multiple new records we want to
			// send all of them, and then update the latest record
			// after all outstanding readings have been sent.
			for _, record := range records {
				if record.Timeslot > latestRecord {
					latestRecord = record.Timeslot
				}
			}
		}

		// Sleep for a minute before checking again.
		select {
		case <-c.closeChan:
			return
		case <-time.After(sendReportTime):
		}

		ticks++
		if ticks >= 300 {
			ticks = 0
			go c.threadedSyncWithServer(latestRecord)
		}
	}
}
