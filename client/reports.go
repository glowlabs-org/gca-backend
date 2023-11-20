package client

// reports.go contains all of the code for sending reports to the server.

// TODO: Add concurrency testing. Since the client doesn't have APIs, the
// concurrency testing can be a bit less intensive than for when there's an API
// to bash.

// TODO: Need to add testing around how banned servers get handled.

// TODO: The test suite needs to have some optional randomization on the
// reports sending so that we can sometimes control the report not to send,
// simulating a UDP failure.

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/csv"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"net"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"time"

	"github.com/glowlabs-org/gca-backend/glow"
	"github.com/glowlabs-org/gca-backend/server"
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
		} else {
			// Round the energy down so that we never over-estiamte the amount of
			// power that has been produced.
			energy = energy - 1
		}

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

// staticServerSync is a wrapper for the networking call that happens in
// threadedSyncServer, it allows us to isolate failures which indicate that a
// failover needs to happen.
//
// The server sync grabs a bitfield indicating what reports are missing, a
// public key that states which GCA owns the device, and a list of gcaServers
// (including banned servers) associated with the GCA. In most cases, the GCA
// will be the same as the current GCA owner of the device. In the event that a
// GCA retires or otherwise needs to shuffle a device away, the GCA may be
// updated.
//
// When reading the newGCA field, we need to make sure that this new GCA is
// authorized by the current GCA. The message as a whole is authorized by the
// GCA server, but that's not the same as the GCA, and any GCA transitions (and
// also any GCA server lists) need to be authorized by the GCA.
func (c *Client) staticServerSync(gcas GCAServer, gcasKey glow.PublicKey, gcaKey glow.PublicKey) (timeslotOffset uint32, bitfield [504]byte, newGCA glow.PublicKey, newShortID uint32, gcaServers []server.AuthorizedServer, err error) {
	// The first step is to make a connection to the server and download
	// the list of reports that it current has.
	location := fmt.Sprintf("%v:%v", gcas.Location, gcas.TcpPort)
	conn, err := net.Dial("tcp", location)
	if err != nil {
		return 0, [504]byte{}, glow.PublicKey{}, 0, nil, fmt.Errorf("unable to dial the gca server: %v", err)
	}
	defer conn.Close()

	// Send the request.
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], c.shortID)
	_, err = conn.Write(buf[:])
	if err != nil {
		return 0, [504]byte{}, glow.PublicKey{}, 0, nil, fmt.Errorf("unable to send reqeust to gca server: %v", err)
	}

	// Receive the response length
	var respLenBuf [2]byte
	_, err = io.ReadFull(conn, respLenBuf[:])
	if err != nil {
		return 0, [504]byte{}, glow.PublicKey{}, 0, nil, fmt.Errorf("unable to read the response length: %v", err)
	}
	respLen := binary.BigEndian.Uint16(respLenBuf[:])

	// Receive the response.
	respBuf := make([]byte, respLen)
	n, err := io.ReadFull(conn, respBuf)
	if err != nil {
		return 0, [504]byte{}, glow.PublicKey{}, 0, nil, fmt.Errorf("unable to read response from gca server: %v", err)
	}
	if n != int(respLen) {
		return 0, [504]byte{}, glow.PublicKey{}, 0, nil, fmt.Errorf("server did not send enough data: %v", err)
	}

	// Verify the signature.
	signingTime := binary.BigEndian.Uint64(respBuf[respLen-72:])
	now := uint64(time.Now().Unix())
	if now+24*3600 < signingTime || now-24*3600 > signingTime {
		return 0, [504]byte{}, glow.PublicKey{}, 0, nil, fmt.Errorf("received response from server that is out of bounds temporally: %v vs %v", now, signingTime)
	}
	var sig glow.Signature
	copy(sig[:], respBuf[respLen-64:])
	if !glow.Verify(gcasKey, respBuf[:respLen-64], sig) {
		return 0, [504]byte{}, glow.PublicKey{}, 0, nil, fmt.Errorf("received response from server with invalid signature: %v", err)
	}

	// Extract the constant length fields.
	var equipmentKey glow.PublicKey
	copy(equipmentKey[:], respBuf[:32])
	timeslotOffset = binary.BigEndian.Uint32(respBuf[32:36])
	copy(bitfield[:], respBuf[36:540])
	copy(newGCA[:], respBuf[540:572])
	newShortID = binary.BigEndian.Uint32(respBuf[572:576])
	var newGCASignature glow.Signature
	copy(newGCASignature[:], respBuf[576:640])

	// Ensure that if there's a new GCA, that the signature authorizing the
	// GCA migration for the device is valid and comes from the current
	// GCA.
	var blank glow.PublicKey
	newGCASigningBytes := append([]byte("gca_migration_key"), respBuf[540:576]...) // Add the new GCA and the new shortID to the verification
	if newGCA != blank && !glow.Verify(gcaKey, newGCASigningBytes, newGCASignature) {
		return 0, [504]byte{}, glow.PublicKey{}, 0, nil, fmt.Errorf("received new GCA from server with invalid signature: %v", err)
	}

	// Ensure that the equipment key matches the client's public key.
	if equipmentKey != c.pubkey {
		return 0, [504]byte{}, glow.PublicKey{}, 0, nil, fmt.Errorf("equipment appears to have the wrong short id")
	}

	// Extract the variable length fields.
	i := 640
	for i < int(respLen)-72 {
		var as server.AuthorizedServer
		copy(as.PublicKey[:], respBuf[i:i+32])
		i += 32
		as.Banned = respBuf[i] != 0
		i += 1
		locationLen := int(respBuf[i])
		i += 1
		as.Location = string(respBuf[i : i+locationLen])
		i += locationLen
		as.HttpPort = binary.BigEndian.Uint16(respBuf[i : i+2])
		i += 2
		as.TcpPort = binary.BigEndian.Uint16(respBuf[i : i+2])
		i += 2
		as.UdpPort = binary.BigEndian.Uint16(respBuf[i : i+2])
		i += 2
		copy(as.GCAAuthorization[:], respBuf[i:])
		i += 64
		gcaServers = append(gcaServers, as)
	}

	// Verify all of the signatures on the authorized servers.
	for _, as := range gcaServers {
		sb := as.SigningBytes()
		if !glow.Verify(gcaKey, sb, as.GCAAuthorization) {
			return 0, [504]byte{}, glow.PublicKey{}, 0, nil, fmt.Errorf("received authorized server which has invalid authorization")
		}
	}

	return timeslotOffset, bitfield, newGCA, newShortID, gcaServers, nil
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
	gcaKey := c.gcaPubkey
	c.serverMu.Unlock()

	// The sync loop starts out by randomly selecting a new server. We do
	// this rather than stick with the current server because we want to
	// know if the current server has gone rogue and gotten banned. A rogue
	// server doesn't have to report itself as banned, and if the client
	// isn't shuffling between available servers, it'll never see news from
	// the GCA that the rogue server doesn't want to expose. Therefore we
	// ensure that the client is always bouncing between servers so that it
	// has good exposure to news from the GCA.
	//
	// The relevant news items are server bannings, new servers, and GCA
	// migrations.

	// Create a map to track servers that have already failed, so
	// that we don't try the same server twice in the same sync
	// attempt. We don't persist this because the server may come
	// back later.
	failedServers := make(map[glow.PublicKey]struct{})
	// Try up to 5 times to grab a random server.
	var timeslotOffset uint32
	var bitfield [504]byte
	var newGCA glow.PublicKey
	var newShortID uint32
	var gcaServers []server.AuthorizedServer
	for i := 0; i < 6; i++ {
		if i == 5 {
			// Give up entirely after 5 attempts. We use
			// this control structure to exit the loop so
			// that we can have relevant bits of logic
			// after the loop which assume that a
			// connection was made successfully.
			return
		}

		// Sleep for a tick.
		select {
		case <-c.closeChan:
			return
		case <-time.After(sendReportTime):
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
			_, exists := failedServers[server]
			if exists || c.gcaServers[server].Banned {
				continue
			}
			c.primaryServer = server
			break
		}
		gcas = c.gcaServers[c.primaryServer]
		gcasKey = c.primaryServer
		c.serverMu.Unlock()

		// Retry grabbing the bitfield.
		var err error
		timeslotOffset, bitfield, newGCA, newShortID, gcaServers, err = c.staticServerSync(gcas, gcasKey, gcaKey)
		if err == nil {
			break
		}
		failedServers[gcasKey] = struct{}{}
	}

	// Check whether the GCA has changed. If the GCA has changed, the
	// device will need to update its servers and gcaPubkey and shortId.
	// This will include updating all of the relevant persist files. The
	// history does not need to be wiped.
	c.serverMu.Lock()
	blank := glow.PublicKey{}
	if newGCA != c.gcaPubkey && newGCA != blank {
		// Before updating the internal state, the on-disk files need
		// to be updated. A new GCA is a big deal, and a failure here
		// could render the device useless. We don't have any ACID
		// frameworks in place, so we have to take a risk with all of
		// the files and hope that they all update atomically.
		// Practical experience tells me that this is a very small
		// risk, because the action happens infrequently, and it
		// happens after the device has been running for a while, and
		// it should complete in under 100ms. The device would have to
		// fail within that 100ms window.
		//
		// If failure does happen, the consequence will be a bricked
		// device that needs its SD card replaced. This isn't a
		// terrible consequence. In the event of an error we panic and
		// hope it doesn't happen again.
		err := ioutil.WriteFile(filepath.Join(c.baseDir, GCAPubfile), newGCA[:], 0644)
		if err != nil {
			panic(err)
		}
		var shortIDBytes [4]byte
		binary.LittleEndian.PutUint32(shortIDBytes[:], newShortID)
		err = ioutil.WriteFile(filepath.Join(c.baseDir, ShortIDFile), shortIDBytes[:], 0644)
		if err != nil {
			panic(err)
		}
		newServers := make(map[glow.PublicKey]GCAServer)
		for _, s := range gcaServers {
			newServers[s.PublicKey] = GCAServer{
				Banned:   s.Banned,
				Location: s.Location,
				HttpPort: s.HttpPort,
				TcpPort:  s.TcpPort,
				UdpPort:  s.UdpPort,
			}
		}
		raw, err := SerializeGCAServerMap(newServers)
		if err != nil {
			panic(err)
		}
		err = ioutil.WriteFile(filepath.Join(c.baseDir, GCAServerMapFile), raw[:], 0644)
		if err != nil {
			panic(err)
		}

		// Need to update the gca pubkey + file, the shortID + file,
		// and the list of GCA servers needs to be wiped clean.
		c.gcaPubkey = newGCA
		c.shortID = newShortID
		c.gcaServers = newServers
	}
	// Update the server map based on the new list of gca servers.
	// Specifcially we want to look for new servers, as well as servers
	// that are now banned.
	//
	// This code runs whether or not there's a new GCA, and it needs to run
	// even if there's a new GCA because the update in the newGCA code does
	// not properly handle server bans.
	for _, s := range gcaServers {
		_, exists := c.gcaServers[s.PublicKey]
		if !exists && s.Banned {
			c.gcaServers[s.PublicKey] = GCAServer{
				Banned:   s.Banned,
				Location: s.Location,
				HttpPort: s.HttpPort,
				TcpPort:  s.TcpPort,
				UdpPort:  s.UdpPort,
			}
		}
	}
	c.serverMu.Unlock()

	// Scan through the bitfield and submit any reports that the device has
	// but the GCA server does not.
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

	// Infinite loop to send reports. We start ticks at 30 so that the
	// catchup function will run about 2.5 hours after boot. We don't want
	// to run it immediately after boot because if we get stuck in a short
	// boot loop scenario, we don't want to blow all of our bandwidth doing
	// sync operations.
	ticks := 30
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
		case <-time.After(sendReportTime + randomTimeExtension()):
		}

		ticks++
		if ticks >= 60 {
			ticks = 0
			go c.threadedSyncWithServer(latestRecord)
		}
	}
}
