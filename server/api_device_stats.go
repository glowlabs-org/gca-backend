package server

// api_device_stats.go will grab the statistics for a device that a GCA needs
// to make on-chain reports about the production values of the equipment being
// tracked by the server.
//
// TODO: These values are only going to be reliable for the GCA if the servers
// are syncing with each other. So we need to get synchronization working ASAP.
//
// TODO: Definitely need to persist the results from WattTime. Maybe we can get
// away with only persisting weekly calls to check the historical values. That
// means things might come in soft throughout the week and then we get a big
// bump at the end of the week. And we can always iterate on that later :)
//
// TODO: Definitely need to implement the call that will allow us to grab
// historical data from WattTime. We'll probably do that within 20 minutes of
// startup, and then every 18 hours afterwards.
//
// TODO: Need to make sure that the impact rates get rotated at the same time
// as the equipment reports. I think they already do... but should confirm with
// a test.

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"

	"github.com/glowlabs-org/gca-backend/glow"
)

// DeviceStats contains the statistics for one device.
type DeviceStats struct {
	PublicKey    glow.PublicKey
	PowerOutputs [2016]uint64
	ImpactRates  [2016]float64
}

// AllDeviceStats contains aggregate weekly statistics
type AllDeviceStats struct {
	Devices        []DeviceStats // A set of data for each device
	TimeslotOffset uint32        // Establish what week is covered by the data
	Signature      glow.Signature
}

// SigningBytes returns the set of bytes that should be signed by the GCA
// server to authenticate the response.
func (ads AllDeviceStats) SigningBytes() []byte {
	// Add the length prefix
	b := make([]byte, 4+len(ads.Devices)*(32+8*2*2016)+4)
	i := 0
	binary.BigEndian.PutUint32(b, uint32(len(ads.Devices)))
	i += 4

	// Add all the device data.
	for x := 0; x < len(ads.Devices); x++ {
		copy(b[i:], ads.Devices[x].PublicKey[:])
		i += 32
		for _, po := range ads.Devices[x].PowerOutputs {
			binary.BigEndian.PutUint64(b[i:], po)
			i += 8
		}
		for _, ir := range ads.Devices[x].ImpactRates {
			binary.BigEndian.PutUint64(b[i:], math.Float64bits(ir))
			i += 8
		}
	}

	// Add the timeslot offset.
	binary.BigEndian.PutUint32(b[i:], ads.TimeslotOffset)
	i += 4
	if i != len(b) {
		panic("SigningBytes gone wrong")
	}
	b = append([]byte("AllDeviceStats"), b...)
	return b
}

// Serialize will convert an AllDeviceStats to a byte slice.
func (ads AllDeviceStats) Serialize() []byte {
	// Calculate the byte slice size
	size := 4 + len(ads.Devices)*(32+8*2*2016) + 4 + 64 // Additional 64 for Signature
	b := make([]byte, size)
	i := 0

	// Number of devices
	binary.BigEndian.PutUint32(b[i:], uint32(len(ads.Devices)))
	i += 4

	// Device data
	for _, device := range ads.Devices {
		copy(b[i:], device.PublicKey[:])
		i += 32
		for _, po := range device.PowerOutputs {
			binary.BigEndian.PutUint64(b[i:], po)
			i += 8
		}
		for _, ir := range device.ImpactRates {
			binary.BigEndian.PutUint64(b[i:], math.Float64bits(ir))
			i += 8
		}
	}

	// Timeslot offset
	binary.BigEndian.PutUint32(b[i:], ads.TimeslotOffset)
	i += 4

	// Signature
	copy(b[i:], ads.Signature[:])

	return b
}

// DeserializeStreamAllDeviceStats will reverse a call to
// AllDeviceStats.Serialize(). This call is unique because it will count the
// number of bytes that it deserializes from the input array. The assumption is
// that the input array contains multiple AllDeviceStats objects listed one
// after another, and returning the size of each one allows them to be decoded
// consecutively more easily.
func DeserializeStreamAllDeviceStats(b []byte) (AllDeviceStats, int, error) {
	if len(b) < 4 {
		return AllDeviceStats{}, 0, fmt.Errorf("byte slice too short")
	}

	// Number of devices
	numDevices := binary.BigEndian.Uint32(b[:4])
	i := 4

	devices := make([]DeviceStats, numDevices)
	for x := 0; x < int(numDevices); x++ {
		if i+32 > len(b) {
			return AllDeviceStats{}, 0, fmt.Errorf("byte slice too short for public key")
		}
		copy(devices[x].PublicKey[:], b[i:i+32])
		i += 32

		for j := 0; j < 2016; j++ {
			if i+8 > len(b) {
				return AllDeviceStats{}, 0, fmt.Errorf("byte slice too short for power output")
			}
			devices[x].PowerOutputs[j] = binary.BigEndian.Uint64(b[i : i+8])
			i += 8
		}
		for j := 0; j < 2016; j++ {
			if i+8 > len(b) {
				return AllDeviceStats{}, 0, fmt.Errorf("byte slice too short for impact rate")
			}
			devices[x].ImpactRates[j] = math.Float64frombits(binary.BigEndian.Uint64(b[i : i+8]))
			i += 8
		}
	}

	if i+4 > len(b) {
		return AllDeviceStats{}, 0, fmt.Errorf("byte slice too short for timeslot offset")
	}
	timeslotOffset := binary.BigEndian.Uint32(b[i : i+4])
	i += 4

	if i+64 > len(b) {
		return AllDeviceStats{}, 0, fmt.Errorf("byte slice too short for signature")
	}
	var signature glow.Signature
	copy(signature[:], b[i:i+64])
	i += 64

	return AllDeviceStats{
		Devices:        devices,
		TimeslotOffset: timeslotOffset,
		Signature:      signature,
	}, i, nil
}

// AllDeviceStatsHandler will return the statistics on all of the devices for
// the requested week.
func (s *GCAServer) AllDeviceStatsHandler(w http.ResponseWriter, r *http.Request) {
	// Only accept GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET method is supported.", http.StatusMethodNotAllowed)
		s.logger.Warn("Received non-GET request for recent reports.")
		return
	}

	// Retrieve the desired week from the query.
	tsoStr := r.URL.Query().Get("timeslot_offset")
	if tsoStr == "" {
		http.Error(w, "timeslot_offset is a required query parameter", http.StatusBadRequest)
		return
	}
	tsoU64, err := strconv.ParseUint(tsoStr, 10, 32)
	if err != nil {
		http.Error(w, "invalid timeslot_offset format", http.StatusBadRequest)
		return
	}
	tso := uint32(tsoU64)
	if tso%2016 != 0 {
		http.Error(w, "invalid timeslot_offset, must be a multiple of 2016", http.StatusBadRequest)
		return
	}

	// Check whether we need to return all of the current values, or if we
	// need to return
	var stats AllDeviceStats
	s.mu.Lock()
	if tso < s.equipmentReportsOffset {
		relativeTSO := tso - s.equipmentHistoryOffset
		stats = s.equipmentStatsHistory[relativeTSO/2016]
		s.mu.Unlock()
	} else {
		stats, err = s.buildDeviceStats(tso)
		s.mu.Unlock()
		if err != nil {
			http.Error(w, "unable to build stats for the provided timeslot: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Send the response as JSON with a status code of OK
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		http.Error(w, "Failed to encode JSON response", http.StatusInternalServerError)
		return
	}
}

// managedBuildDeviceStats will build a DeviceStats object for the provided
// timeslot offset.
func (s *GCAServer) buildDeviceStats(timeslotOffset uint32) (ads AllDeviceStats, err error) {
	// Check that timeslotOffset is in a range where the ads can be built.
	if timeslotOffset%2016 != 0 {
		return ads, fmt.Errorf("timeslotOffset must be a multiple of 2016")
	}
	if timeslotOffset < s.equipmentReportsOffset {
		return ads, fmt.Errorf("timeslotOffset must not predate the current equipment offset")
	}
	var x int
	if timeslotOffset == s.equipmentReportsOffset+2016 {
		x = 2016
	}
	if timeslotOffset > s.equipmentReportsOffset+2016 {
		return ads, fmt.Errorf("timeslotOffset must not be in the future")
	}

	// Build the ads.
	for shortID, reports := range s.equipmentReports {
		var ds DeviceStats
		ds.PublicKey = s.equipment[shortID].PublicKey
		for i := 0; i < 2016; i++ {
			ds.PowerOutputs[i] = reports[x+i].PowerOutput
		}
		copy(ds.ImpactRates[:], s.equipmentImpactRate[shortID][x:])
		ads.Devices = append(ads.Devices, ds)
	}

	// Set the timeslot offset and add a signature.
	ads.TimeslotOffset = timeslotOffset
	sb := ads.SigningBytes()
	ads.Signature = glow.Sign(sb, s.staticPrivateKey)
	return ads, nil
}
