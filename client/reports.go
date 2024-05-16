package client

// reports.go contains all of the code for sending reports to the server. It
// also contains the synchronization code which is responsible for migrating
// the client to a new primary server every few hours (chosen randomly), as
// well as code that will migrate the client to a new GCA if necessary.

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
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
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
	eqr.Signature = glow.Sign(sb, c.staticPrivKey)
	data := eqr.Serialize()
	location := fmt.Sprintf("%v:%v", gcas.Location, gcas.UdpPort)
	glow.SendUDPReport(data, location)
}

// staticReadEnergyFile will read the data from the energy file and return an array
// that contains all of the values.
func (c *Client) staticReadEnergyFile() ([]EnergyRecord, error) {
	// Open the CSV file. We do a quick conditional check because in prod
	// the energy file is located in an absolute location rather than being
	// part of the energy monitor directory, but in testing the location is
	// in the energy monitor directory so that multiple clients can use
	// different energy monitor files during testing.
	filePath := EnergyFile
	if EnergyFile[0] != '/' {
		filePath = path.Join(c.staticBaseDir, EnergyFile)
	}

	// Read the file all at once, to avoid any potential issues if it
	// is modified while being parsed.
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("unable to read monitoring file: %v", err)
	}

	// Iterate over the CSV records
	reader := csv.NewReader(strings.NewReader(string(data)))
	var records []EnergyRecord
	for {
		record, err := reader.Read()
		if err != nil {
			// Stop at EOF or on error. An error here means we
			// couldn't read a timestamp, which means we don't know
			// what timestamp to tell the server for having errors.
			// Whether it's an EOF or a real error, the best move
			// is to break.
			break
		}

		timestamp, err := strconv.ParseInt(record[0], 10, 64)
		if err != nil {
			// If the timestamp can't be read, we don't know what
			// timestamp to submit to the server as having an
			// error, we just ignore the read in this case.
			continue
		}
		timeslot, err := glow.UnixToTimeslot(timestamp)
		if err != nil {
			// If the timeslot can't be determined, we don't know
			// what timeslot to submit to the server as having an
			// error, we just ignore the read in this case.
			continue
		}

		// Parse the value. If the value is not parsed correctly, it's
		// probably because the firmware version is wrong or because
		// the file has a text error instead of a number.
		var energy uint64
		energyF64, err := strconv.ParseFloat(record[1], 64)
		if err != nil {
			// In the event of a parse error for the energy reading
			// for this timestamp, set the value to '3' to indicate
			// that there was a parse error.
			energy = 3
		} else if energyF64 > -24 && energyF64 < 24 {
			// If the energy value read successfully but it read
			// below an absolute value of 24, set the value to '2'
			// to indicate that there was a reading that
			// effectively counts as 0 power. We use '2' as the 'no
			// power' sentinel because '0' is the number that
			// appears if no report is submitted at all, and we
			// want to distinguish between no report submitted at
			// all and a blank report being submitted.
			//
			// Note that the sentinel value '1' is already reserve
			// to indicate that the gca server has banned a
			// timeslot.
			energy = 2
		} else {
			// NOTE: 'energy' might be a negative number, which will cast
			// as an underflowed uint64. To the best of my knowledge, there
			// is nothing wrong with that, but all downstream applications
			// will have to accept the numbers and know to convert them to
			// their respective negative values.
			//
			// The EnergyMultiplier needs to be applied in advance
			// of performing the underflow conversion, otherwise
			// the multiplier will be multiplying a giant uint64 by
			// 4 rather than multiplying a negative number by 4.
			energy = uint64(c.energyMultiplier * energyF64 / c.energyDivider)
		}

		// Append the data to the records slice
		records = append(records, EnergyRecord{
			Timeslot: timeslot,
			Energy:   energy,
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
	// Open a TCP connection and send our shortID as the request. The
	// server will respond with a bunch of information that will allow the
	// client to remain synchronized with the GCA server.
	location := fmt.Sprintf("%v:%v", gcas.Location, gcas.TcpPort)
	conn, err := net.Dial("tcp", location)
	if err != nil {
		return 0, [504]byte{}, glow.PublicKey{}, 0, nil, fmt.Errorf("unable to dial the gca server: %v", err)
	}
	defer conn.Close()
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], c.shortID)
	_, err = conn.Write(buf[:])
	if err != nil {
		return 0, [504]byte{}, glow.PublicKey{}, 0, nil, fmt.Errorf("unable to send reqeust to gca server: %v", err)
	}

	// Receive the response length, which is a two byte prefix to the
	// actual response. No safety checks are needed here, any possible
	// value is sane and will be processed correctly.
	var respLenBuf [2]byte
	_, err = io.ReadFull(conn, respLenBuf[:])
	if err != nil {
		return 0, [504]byte{}, glow.PublicKey{}, 0, nil, fmt.Errorf("unable to read the response length: %v", err)
	}
	respLen := binary.LittleEndian.Uint16(respLenBuf[:])

	// Receive the response.
	respBuf := make([]byte, respLen)
	n, err := io.ReadFull(conn, respBuf)
	if err != nil {
		return 0, [504]byte{}, glow.PublicKey{}, 0, nil, fmt.Errorf("unable to read response from gca server: %v", err)
	}
	if n != int(respLen) {
		return 0, [504]byte{}, glow.PublicKey{}, 0, nil, fmt.Errorf("server did not send enough data: %v", err)
	}

	// Verify the signature. No safety checks are needed here, as any
	// possible values will be parsed correctly for the signing time and
	// signature.
	signingTime := binary.LittleEndian.Uint64(respBuf[respLen-72:])
	now := uint64(time.Now().Unix())
	if now+24*3600 < signingTime || now-24*3600 > signingTime {
		return 0, [504]byte{}, glow.PublicKey{}, 0, nil, fmt.Errorf("received response from server that is out of bounds temporally: %v vs %v", now, signingTime)
	}
	var sig glow.Signature
	copy(sig[:], respBuf[respLen-64:])
	if !glow.Verify(gcasKey, respBuf[:respLen-64], sig) {
		return 0, [504]byte{}, glow.PublicKey{}, 0, nil, fmt.Errorf("received response from server with invalid signature: %v", err)
	}

	// Extract the constant length fields. No safety checks are needed, as
	// all values are binary values with no invariants.
	var equipmentKey glow.PublicKey
	copy(equipmentKey[:], respBuf[:32])
	timeslotOffset = binary.LittleEndian.Uint32(respBuf[32:36])
	copy(bitfield[:], respBuf[36:540])
	copy(newGCA[:], respBuf[540:572])
	newShortID = binary.LittleEndian.Uint32(respBuf[572:576])
	var newGCASignature glow.Signature
	copy(newGCASignature[:], respBuf[respLen-(64+72):respLen-72])

	// Ensure that the equipment key matches the client's public key. If
	// they do not match, the GCAServer has some other public key
	// associated with our shortID, and therefore this response is
	// meaningless.
	if equipmentKey != c.staticPubKey {
		return 0, [504]byte{}, glow.PublicKey{}, 0, nil, fmt.Errorf("equipment appears to have the wrong short id")
	}

	// Ensure that if there's a new GCA, the signature authorizing the GCA
	// migration for the device is valid and comes from the current GCA. No
	// safety checks are needed, as all values are binary values with no
	// invariants.
	var blank glow.PublicKey
	migrationBytes := append(equipmentKey[:], respBuf[540:respLen-(64+72)]...)
	newGCASigningBytes := append([]byte("EquipmentMigration"), migrationBytes...)
	if newGCA != blank && !glow.Verify(gcaKey, newGCASigningBytes, newGCASignature) {
		return 0, [504]byte{}, glow.PublicKey{}, 0, nil, fmt.Errorf("received new GCA from server with invalid signature: %v", err)
	}

	// Extract the variable length fields. Safety checks are needed for
	// each iteration to ensure that the respLen is long enough to hold the
	// next pieces of data. The respLen needs to be checked once for
	// reading all of the fields up to the locationLen, and then again for
	// reading all of the fields after the locationLen. Can't do it in one
	// go because we don't know the locationLen until after it is read.
	i := 576
	end := int(respLen) - (72 + 64)
	for i < end {
		// Check that there are enough bytes to read up to the
		// locationLen.
		if i+34 > end {
			return 0, [504]byte{}, glow.PublicKey{}, 0, nil, fmt.Errorf("unable to decode authorized servers due to length mismatches")
		}
		var as server.AuthorizedServer
		copy(as.PublicKey[:], respBuf[i:i+32])
		i += 32
		as.Banned = respBuf[i] != 0
		i += 1
		locationLen := int(respBuf[i])
		i += 1
		if i+locationLen+70 > end {
			return 0, [504]byte{}, glow.PublicKey{}, 0, nil, fmt.Errorf("unable to decode authorized servers due to length mismatches")
		}
		as.Location = string(respBuf[i : i+locationLen])
		i += locationLen
		as.HttpPort = binary.LittleEndian.Uint16(respBuf[i : i+2])
		i += 2
		as.TcpPort = binary.LittleEndian.Uint16(respBuf[i : i+2])
		i += 2
		as.UdpPort = binary.LittleEndian.Uint16(respBuf[i : i+2])
		i += 2
		copy(as.GCAAuthorization[:], respBuf[i:])
		i += 64
		gcaServers = append(gcaServers, as)
	}

	// Verify all of the signatures on the authorized servers. If there's a
	// new GCA, it's assumed that all of the new servers being presented
	// are related to the migration.
	for _, as := range gcaServers {
		sb := as.SigningBytes()
		var verify bool
		if newGCA != blank {
			verify = glow.Verify(newGCA, sb, as.GCAAuthorization)
		} else {
			verify = glow.Verify(gcaKey, sb, as.GCAAuthorization)
		}
		if !verify {
			return 0, [504]byte{}, glow.PublicKey{}, 0, nil, fmt.Errorf("received authorized server which has invalid authorization")
		}
	}

	return timeslotOffset, bitfield, newGCA, newShortID, gcaServers, nil
}

// threadedSyncWithServer will select a random server from the pool of servers
// and then perform a sync operation. The sync operation includes downloading
// the latest list of servers that have been authorized by the GCA, checking
// for new migration orders from the GCA, and checking if the server is missing
// any power reports.
//
// The client is assuming that the various GCA servers are synchronizing power
// reports that they receive from each other.
//
// This function returns true if the sync appears to have been successful,
// false otherwise.
func (c *Client) threadedSyncWithServer(latestReading uint32) bool {
	// Grab the state we need from the client under the safety of a mutex.
	c.mu.Lock()
	gcas := c.gcaServers[c.primaryServer]
	gcasKey := c.primaryServer
	gcaKey := c.gcaPubKey
	c.mu.Unlock()

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
	// attempt. We don't persist any data about how often servers are
	// available, because we assume that unavailable servers will
	// eventually be banned by the GCA. The client code is a little bit
	// dumber in the hopes that the GCA is making an effort to take care of
	// it.
	//
	// The Glow protocol expects the GCA's to be paying attention to their
	// servers and their clients and properly caring for them.
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
			return false
		}

		// Sleep for a tick.
		if !c.tg.Sleep(sendReportTime) {
			return false
		}

		// Pick a new random primary server. The steps are:
		//
		// 1. Pull all the servers into an array.
		// 2. Randomly shuffle the array.
		// 3. Select the first non-banned server.
		c.mu.Lock()
		servers := make([]glow.PublicKey, 0, len(c.gcaServers))
		for server := range c.gcaServers {
			servers = append(servers, server)
		}
		for i := range servers {
			j, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
			if err != nil {
				continue
			}
			servers[i], servers[j.Int64()] = servers[j.Int64()], servers[i]
		}
		found := false
		for _, server := range servers {
			_, exists := failedServers[server]
			if exists || c.gcaServers[server].Banned {
				continue
			}
			found = true
			c.primaryServer = server
			break
		}
		if !found {
			// All servers have either failed or are banned,
			// therefore this sync operation does not need to
			// continue.
			return false
		}
		gcas = c.gcaServers[c.primaryServer]
		gcasKey = c.primaryServer
		c.mu.Unlock()

		// Try performing the sync operation with the chosen server.
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
	blank := glow.PublicKey{}
	c.mu.Lock()
	if newGCA != c.gcaPubKey && newGCA != blank {
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
		err := os.WriteFile(filepath.Join(c.staticBaseDir, GCAPubKeyFile), newGCA[:], 0644)
		if err != nil {
			panic(err)
		}
		var shortIDBytes [4]byte
		binary.LittleEndian.PutUint32(shortIDBytes[:], newShortID)
		err = os.WriteFile(filepath.Join(c.staticBaseDir, ShortIDFile), shortIDBytes[:], 0644)
		if err != nil {
			panic(err)
		}
		// Add new information.
		newServers := make(map[glow.PublicKey]GCAServer)
		for _, s := range gcaServers {
			_, exists := newServers[s.PublicKey]
			if !exists || s.Banned {
				newServers[s.PublicKey] = GCAServer{
					Banned:   s.Banned,
					Location: s.Location,
					HttpPort: s.HttpPort,
					TcpPort:  s.TcpPort,
					UdpPort:  s.UdpPort,
				}
			}
		}
		raw, err := SerializeGCAServerMap(newServers)
		if err != nil {
			panic(err)
		}
		err = os.WriteFile(filepath.Join(c.staticBaseDir, GCAServerMapFile), raw[:], 0644)
		if err != nil {
			panic(err)
		}

		// Need to update the gca pubkey + file, the shortID + file,
		// and the list of GCA servers needs to be wiped clean.
		c.gcaPubKey = newGCA
		c.shortID = newShortID
		c.gcaServers = newServers
	} else {
		// Update the server map based on the new list of gca servers.
		// Specifcially we want to look for new servers, as well as
		// servers that are now banned. This code runs conditionally
		// because other code handles updating the gcaServers if
		// there's a gca migration.
		for _, s := range gcaServers {
			_, exists := c.gcaServers[s.PublicKey]
			if !exists || s.Banned {
				c.gcaServers[s.PublicKey] = GCAServer{
					Banned:   s.Banned,
					Location: s.Location,
					HttpPort: s.HttpPort,
					TcpPort:  s.TcpPort,
					UdpPort:  s.UdpPort,
				}
			}
		}
	}
	raw, err := SerializeGCAServerMap(c.gcaServers)
	if err != nil {
		panic(err)
	}
	err = os.WriteFile(filepath.Join(c.staticBaseDir, GCAServerMapFile), raw[:], 0644)
	if err != nil {
		panic(err)
	}
	c.mu.Unlock()

	// Scan through the bitfield and submit any reports that the device has
	// but the GCA server does not.
	lastIndex := latestReading - timeslotOffset
	for i := uint32(0); i <= lastIndex && int(i)/8 < len(bitfield); i++ {
		if bitfield[i/8]&(1<<(i%8)) == 0 {
			// Server is missing this index, check locally to see
			// if we have it, and send it if we do. The server will
			// ignore any power output below 2, so we ignore those
			// values as well as errors.
			powerOutput, err := c.staticLoadReading(i + timeslotOffset)
			if err != nil || powerOutput < 2 {
				continue
			}
			// Because we now handle negative numbers, the uint32
			// needs to be cast to an int32 before being upscaled
			// to a uint64.
			record := EnergyRecord{
				Timeslot: i + timeslotOffset,
				Energy:   uint64(int32(powerOutput)),
			}
			c.staticSendReport(gcas, record)
			time.Sleep(UDPSleepSyncTime)
		}
	}

	return true
}

// launchSendReports will create the infinite loop that sends reports to the
// server.
func (c *Client) launchSendReports() {
	// Right at startup, we save all of the existing records. We don't
	// bother sending them because we assume we already sent them, and if
	// we haven't already sent them, the periodic synchronization will fix
	// everything up.
	latestRecord := uint32(0)
	records, err := c.staticReadEnergyFile()
	// We'll no-op if there's an error. One error that gets caught is if
	// the monitoring equipment saves a duplicate reading. The monitoring
	// equipment shouldn't have this issue, because it should be doing
	// everything using Unix timestamps and therefore be immune to timezone
	// and daylight savings related issues, but if the hardware clock
	// messes up somehow we would rather ignore the error.
	if err == nil {
		for _, record := range records {
			// Downcasting a uint64 to a uint32 is a pretty
			// questionable choice, in hindsight I'm not sure why I
			// thought that was okay. But it's not something worth
			// fixing at the moment, so we just have to be careful
			// in the rest of the code to make sure that we upscale
			// it again properly when the value is loaded.
			//
			// Since we now generate negative numbers by
			// underflowing the reading, this means that the uint32
			// needs to be cast to an int32 before being recast to
			// a uint64.
			//
			// Downcasting an underflowed uint64 to a uint32
			// results in an underflow of the same value in the
			// uint32, so nothing special is needed here.
			err := c.staticSaveReading(record.Timeslot, uint32(record.Energy))
			if err != nil {
				continue
			}
			if record.Timeslot > latestRecord {
				latestRecord = record.Timeslot
			}
		}
	}
	c.tg.Launch(func() {
		c.threadedSendReports(latestRecord)
	})
}

// threadedSendReports will periodically check for new readings in the energy
// file, and if a new reading is found, a report will be created and sent to
// the primary server. The report is sent over UDP, and no short-term attempt
// is made to confirm that the report reached its destination. There is a
// separate synchronization process that occurs at much larger intervals, which
// will re-send any reports that failed to be delivered on their first attempt.
func (c *Client) threadedSendReports(latestRecord uint32) {
	// Infinite loop to send reports. We start ticks at 30 so that the
	// catchup function will run about 2.5 hours after boot. We don't want
	// to run it immediately after boot because if some process causes the
	// device to restart frequently, the bandwidth costs of doing frequent
	// sync operations could be quite expensive.
	//
	// The client is expected to be operating on a mobile network, which is
	// why we use UDP at all. Some research suggested that packet delivery
	// rate over UDP is much greater than 99%, but packets will nearly
	// always arrive out of order.
	//
	// The client only ever sends one packet at a time (less than 200
	// bytes), so ordering is not a concern. A 99% delivery rate is quite
	// good, and means the background sync loop is not going to incur much
	// overhead from packet loss.
	//
	// The infrequent sync loop is done using TCP, which is more expensive,
	// but it also requires transferring more than one packet and we would
	// rather let TCP handle the ordering than implement that ourselves.
	ticks := 30
	syncStatus := uint64(1)
	for {
		// Check if the server has shut down.
		if c.tg.IsStopped() {
			return
		}

		// Grab the gca server for use when sending the report.
		c.mu.Lock()
		gcas := c.gcaServers[c.primaryServer]
		c.mu.Unlock()

		// Read the energy file. No-op if there's an error. Can't
		// continue because we still want to sleep.
		records, err := c.staticReadEnergyFile()
		if err == nil {
			for _, record := range records {
				// We try saving the reading first, which can
				// produce an error. The main error that we are
				// looking for is a double report error, which
				// means the same timeslot has multiple
				// different energy readings. That's a problem
				// that will cause the timeslot to get banned,
				// so we don't send the report if that happens.
				//
				// Even if the error is not a double report
				// error, we don't want to send the server a
				// reading that we aren't able to persist
				// ourselves, because that could indicate other
				// issues such as faulty hardware, and
				// therefore the readings may not be accurate.
				err := c.staticSaveReading(record.Timeslot, uint32(record.Energy))
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

		// Sleep before checking the file again. The sleep includes
		// both a standard time and a random extra amount of time. The
		// randomization helps multiple devices to spread out over a
		// larger interval when sending information to the server,
		// hopefully preventing the server from being in a situation
		// where all of the hardware is sending it data in the same 3
		// seconds followed by 5 minutes of absolutely no activity.
		if !c.tg.Sleep(sendReportTime + randomTimeExtension()) {
			return
		}

		// Once every 60 iterations a server sync is performed. That's
		// just under 5 hours between each sync operation.
		ticks++
		if ticks >= 60 || atomic.LoadUint64(&syncStatus) == 0 {
			ticks = 0
			c.tg.Launch(func() {
				success := c.threadedSyncWithServer(latestRecord)
				if success {
					atomic.StoreUint64(&syncStatus, 1)
				} else {
					atomic.StoreUint64(&syncStatus, 0)
				}
			})
		}
	}
}
