package server

// TODO: We should probably have some sort of process that will periodically
// look for gaps in the equipment and call '/data' to get the missing bits.
// '/data' gives up to 32 days of historical data for a ba, and this server
// only tracks 2 weeks of data anyway.

// TODO: We need some way to associate a solar farm to a wallet address. We
// don't have that at the moment.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/glowlabs-org/gca-backend/glow"
)

// Define a struct to receive the information from the call to get
// a token from WattTime.
type WattTimeTokenResponse struct {
	Token string `json:"token"`
}

// Define a struct to receive the information from a call to get
// the balancing authority from WattTime
type BalancingAuthorityResponse struct {
	Abbrev string `json:"region"`
}

// WattTime API generic data point type.
type DataPoint struct {
	PointTime string  `json:"point_time"`
	Value     float64 `json:"value"`
}

type DataPointsJSON struct {
	Data []DataPoint `json:"data"`
	Meta struct {
		DataPointPeriodSeconds int    `json:"data_point_period_seconds"`
		Region                 string `json:"region"`
		SignalType             string `json:"signal_type"`
		Units                  string `json:"units"`
	} `json:"meta"`
}

// loadWattTimeCredentials is a helper function to load one of
// the watttime credential files from disk.
func loadWattTimeCredentials(filename string) (string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf("unable to read watttime credentials: %v", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// getBalancingAuthority makes an API call to watttime to get the ba that's associated
// with a specific location
func getBalancingAuthority(token string, latitude, longitude float64) (string, error) {
	client := &http.Client{}
	regionURL := "https://api.watttime.org/v3/region-from-loc"
	req, err := http.NewRequest("GET", regionURL, nil)
	if err != nil {
		return "", err
	}

	// Set the authorization header and query parameters
	req.Header.Set("Authorization", "Bearer "+token)
	q := req.URL.Query()
	q.Add("latitude", strconv.FormatFloat(latitude, 'f', 6, 64))
	q.Add("longitude", strconv.FormatFloat(longitude, 'f', 6, 64))
	q.Add("signal_type", "co2_moer")
	req.URL.RawQuery = q.Encode()

	// Make the API request
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Check for non-200 status code
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("watttime API request failed with status code: %d", resp.StatusCode)
	}

	// Parse the JSON response
	var baResponse BalancingAuthorityResponse
	if err := json.NewDecoder(resp.Body).Decode(&baResponse); err != nil {
		return "", err
	}

	return baResponse.Abbrev, nil
}

// getWattTimeIndex returns the MOER value for the provided lat+long at the
// curernt time.
func getWattTimeIndex(token string, latitude float64, longitude float64) (float64, int64, string, error) {
	// Since this code depends on external APIs, we return an arbitrary
	// value during testing.
	if testMode {
		return latitude + longitude + 200 + float64(time.Now().UnixNano()%250), glow.TimeslotToUnix(glow.CurrentTimeslot()), "", nil
	}

	// Get the region associated with these coordinates
	region, err := getBalancingAuthority(token, latitude, longitude)
	if err != nil {
		return 0, 0, "", fmt.Errorf("unable to get watttime region: %v", err)
	}

	// Create the base url
	client := &http.Client{}
	url := "https://api.watttime.org/v3/signal-index"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, 0, "", fmt.Errorf("unable to get watttime index: %v", err)
	}

	// Set the parameters
	req.Header.Set("Authorization", "Bearer "+token)
	q := req.URL.Query()
	q.Add("region", region)
	q.Add("signal_type", "co2_moer")
	req.URL.RawQuery = q.Encode()

	// Make the API request
	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, "", fmt.Errorf("unable to make watttime api request")
	}
	defer resp.Body.Close()
	// Check for non-200 status code
	if resp.StatusCode != http.StatusOK {
		return 0, 0, "", fmt.Errorf("watttime API request failed with status code: %d", resp.StatusCode)
	}

	// Parse the JSON response.
	var jr DataPointsJSON
	err = json.NewDecoder(resp.Body).Decode(&jr)
	if err != nil {
		return 0, 0, "", fmt.Errorf("unable to decode watttime historical data: %v", err)
	}
	SentinelizeHistoricalData(&jr)

	// WattTime API should have a single point in the response
	if len(jr.Data) != 1 {
		return 0, 0, "", fmt.Errorf("invalid api return: %v data items", len(jr.Data))
	}
	// Parse the string time.
	t, err := time.Parse("2006-01-02T15:04:05+00:00", jr.Data[0].PointTime)
	if err != nil {
		return 0, 0, "", fmt.Errorf("unable to parse watttime response time: %v", err)
	}
	moer := jr.Data[0].Value

	// Convert the moer to grams per megawatt hour. Moer is provided in
	// pounds per megawatt hour. We multiply by 453.59237 to get from
	// pounds per megawatt hour to grams per megawatt hour.
	moer *= 453.59237
	return moer, t.Unix(), region, nil
}

// SentinalizeHistoricalData provides sentinal values into the JSON
// data as follows:
// 0 denotes no data found for this time slot
// 2 denotes 0 returned for this time slot
// and integer N below 24 is replaced with a decimal N - 0.0001
func SentinelizeHistoricalData(jr *DataPointsJSON) {
}

// getWattTimeHistoricalDataRaw returns uninterpreted WattTime historical data for a given
// region and time range.
func getWattTimeHistoricalDataRaw(token, region string, startTime, endTime int64) ([]byte, error) {
	// Convert the times to ISO 8601 UTC format
	startTimeT := time.Unix(startTime, 0).UTC()
	startTimeISO := startTimeT.Format("2006-01-02T15:04:05Z")
	endTimeT := time.Unix(endTime, 0).UTC()
	endTimeISO := endTimeT.Format("2006-01-02T15:04:05Z")

	// Create the base url
	client := &http.Client{}
	url := "https://api.watttime.org/v3/historical"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to get watttime index: %v", err)
	}

	// Set the parameters
	req.Header.Set("Authorization", "Bearer "+token)
	q := req.URL.Query()
	q.Add("region", region)
	q.Add("start", startTimeISO)
	q.Add("end", endTimeISO)
	q.Add("signal_type", "co2_moer")
	req.URL.RawQuery = q.Encode()

	// Make the API request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to make watttime api request")
	}
	defer resp.Body.Close()
	// Check for non-200 status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("watttime API request failed with status code: %d", resp.StatusCode)
	}
	rb, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("watttime API read failed: %v", err)
	}
	return rb, nil
}

// getWattTimeWeeklyData returns the MOER values for the provided lat+long from the
// provided time to 1 week later.
func getWattTimeWeeklyData(token string, latitude float64, longitude float64, startTime int64) ([]float64, []int64, error) {
	// Determine what data range can be requested. If the end time is
	// within 5 days of the current time, WattTime will be asked for the
	// full set of data that it has. In the WattTime API v3 the end time is
	// required and will be set to current time in this case.
	endTime := startTime + 604800 // Ideal end time, number of seconds in a week.
	if endTime+432000 > time.Now().Unix() {
		endTime = time.Now().Unix()
	}

	// During testing we return blank values, that way it doesn't interfere
	// with testing that's probing individual fields.
	if testMode {
		return nil, nil, nil
	}

	// Get the region and historical data
	region, err := getBalancingAuthority(token, latitude, longitude)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to get watttime region: %v", err)
	}
	raw, err := getWattTimeHistoricalDataRaw(token, region, startTime, endTime)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to get watttime historical data: %v", err)
	}
	br := bytes.NewReader(raw)

	// Parse the JSON response.
	var jr DataPointsJSON
	err = json.NewDecoder(br).Decode(&jr)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to decode watttime historical data: %v", err)
	}
	SentinelizeHistoricalData(&jr)

	// Build the return values.
	var moers []float64
	var dates []int64
	for _, ir := range jr.Data {
		// Parse the string time.
		t, err := time.Parse("2006-01-02T15:04:05Z", ir.PointTime)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to parse watttime response time: %v", err)
		}
		// Convert the moer to grams per megawatt hour. Moer is
		// provided in pounds per megawatt hour by the WattTime API. We
		// multiply by 453.59237 to get from pounds per megawatt hour
		// to grams per megawatt hour.
		moer := ir.Value * 453.59237
		moers = append(moers, moer)
		dates = append(dates, t.Unix())
	}
	return moers, dates, nil
}

// threadedCollectImpactData will periodically (every 2 minutes) query WattTime
// for the latest impact data for each device being tracked by the server. The
// WattTime period is 5 minutes, so we'll be grabbing the same datapoint pretty
// regularly.
func (gcas *GCAServer) threadedCollectImpactData(username, password string) {
	// Infinite loop to keep fetching data from WattTime.
	for {
		if !gcas.tg.Sleep(wattTimeFrequency) {
			return
		}

		err := gcas.managedGetWattTimeIndexData(username, password)
		if err != nil {
			gcas.logger.Errorf("unable to complete watttime data update: %v", err)
			continue
		}
	}
}

// managedGettWattTimeIndexData performs a single round of grabbing the current
// index data from WattTime for every device.
func (gcas *GCAServer) managedGetWattTimeIndexData(username, password string) error {
	// Get a new auth token. They expire relatively quickly so it's better
	// to get a new token every time this function is called.
	token, err := staticGetWattTimeToken(username, password)
	if err != nil {
		return fmt.Errorf("unable to get watttime token: %v", err)
	}

	// Grab a list of devices to loop over. We grab the list with a mutex
	// so that we don't have to hold a lock while doing network operations
	// for each device.
	gcas.mu.Lock()
	var devices []uint32
	var lats []float64
	var longs []float64
	for shortID, e := range gcas.equipment {
		devices = append(devices, shortID)
		lats = append(lats, e.Latitude)
		longs = append(longs, e.Longitude)
	}
	gcas.mu.Unlock()

	// Loop over the devices.
	for i, shortID := range devices {
		if !testMode {
			// We don't want to hit the WattTime ratelimits, so we
			// sleep a bit before making a request to ensure that
			// we don't go too far.
			time.Sleep(250 * time.Millisecond)
		}
		// Fetch the current results for this device.
		moer, date, _, err := getWattTimeIndex(token, lats[i], longs[i])
		if err != nil {
			gcas.logger.Errorf("unable to get watttime data: %v", err)
			gcas.logger.Errorf("watttime lat: %v, long: %v", lats[i], longs[i])
			continue
		}
		timeslot, err := glow.UnixToTimeslot(date)
		if err != nil {
			gcas.logger.Errorf("watttime returned data at an invalid timeslot: %v", err)
			gcas.logger.Errorf("watttime time: %v, genesis time: %v", date, glow.GenesisTime)
			continue
		}

		// Update the struct which tracks the moer times. If the clock
		// goes backwards in time for some reason, this can panic, so
		// we have to double check the bounds.
		//
		// We also have to check that we aren't fetching data for a
		// timeslot that is well beyond the current
		// equipmentReportsOffset, which can happen if the server
		// hasn't been online in a few weeks.
		gcas.mu.Lock()
		impactIndex := timeslot - gcas.equipmentReportsOffset
		if timeslot >= gcas.equipmentReportsOffset && impactIndex < 4032 {
			gcas.equipmentImpactRate[shortID][impactIndex] = moer
		}
		gcas.mu.Unlock()
	}
	return nil
}

// managedGetWattTimeWeekData will grab all of the data for the latest week and
// fill out the impact rates as much as possible.
func (gcas *GCAServer) managedGetWattTimeWeekData(username, password string) error {
	// Disable this during testing, as the testing does not have WattTime access.
	if testMode {
		return nil
	}

	// Get a new auth token. They expire relatively quickly so it's better
	// to get a new token every time this function is called.
	token, err := staticGetWattTimeToken(username, password)
	if err != nil {
		return fmt.Errorf("unable to get watttime token: %v", err)
	}

	// Grab a list of devices to loop over. We grab the list with a mutex
	// so that we don't have to hold a lock while doing network operations
	// for each device.
	gcas.mu.Lock()
	var devices []uint32
	var lats []float64
	var longs []float64
	for shortID, e := range gcas.equipment {
		devices = append(devices, shortID)
		lats = append(lats, e.Latitude)
		longs = append(longs, e.Longitude)
	}
	startTime := glow.TimeslotToUnix(gcas.equipmentReportsOffset)
	gcas.mu.Unlock()

	// Loop over the devices.
	for i, shortID := range devices {
		if !testMode {
			// We don't want to hit the WattTime ratelimits, so we
			// sleep a bit before making a request to ensure that
			// we don't go too far.
			time.Sleep(250 * time.Millisecond)
		}
		// Fetch the current results for this device.
		moers, dates, err := getWattTimeWeeklyData(token, lats[i], longs[i], startTime)
		if err != nil {
			gcas.logger.Errorf("unable to get watttime data: %v", err)
			gcas.logger.Errorf("watttime lat: %v, long: %v, startTime: %v", lats[i], longs[i], startTime)
			continue
		}

		// Integrate all the data in one loop with the lock held.
		gcas.mu.Lock()
		for i, date := range dates {
			// There's an edge case where the 'ero' is more than 2
			// weeks in the past, which means there will be a lot
			// more data than what we can process, so the loop has
			// to be aborted early.
			if i >= 4032 {
				break
			}

			timeslot, err := glow.UnixToTimeslot(date)
			if err != nil {
				gcas.logger.Errorf("watttime returned data at an invalid timeslot: %v", err)
				gcas.logger.Errorf("watttime time: %v, genesis time: %v", date, glow.GenesisTime)
				continue
			}

			// Update the struct which tracks the moer times. If the clock
			// goes backwards in time for some reason, this can panic, so
			// we have to double check.
			impactIndex := timeslot - gcas.equipmentReportsOffset
			if timeslot >= gcas.equipmentReportsOffset {
				gcas.equipmentImpactRate[shortID][impactIndex] = moers[i]
			}
		}
		gcas.mu.Unlock()

	}
	return nil
}

// staticGetWattTimeToken makes an API call to WattTime to authenticate and
// retrieve an access token.
func staticGetWattTimeToken(username, password string) (string, error) {
	// Don't hit the watttime api during testing.
	if testMode {
		return "fake-token", nil
	}

	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://api.watttime.org/login", nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(username, password)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Check for non-200 status code and handle specific errors
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == 403 {
			return "", fmt.Errorf("watttime authentication failed: invalid credentials")
		}
		return "", fmt.Errorf("watttime API request failed with status code: %d", resp.StatusCode)
	}

	var tokenResponse WattTimeTokenResponse
	err = json.NewDecoder(resp.Body).Decode(&tokenResponse)
	if err != nil {
		return "", err
	}

	return tokenResponse.Token, nil
}

// threadedGetWattTimeWeekData wakes up periodically and refreshes the weekly
// WattTime data.
func (gcas *GCAServer) threadedGetWattTimeWeekData(username, password string) {
	for {
		if !gcas.tg.Sleep(WattTimeWeekDataUpdateFrequency) {
			return
		}

		// This API is called during startup, so it is safe to sleep before calling it here. The
		// intention is to call it periodically, most likely once per day.
		err := gcas.managedGetWattTimeWeekData(username, password)
		if err != nil {
			gcas.logger.Errorf("threaded call unable to get watttime data for the most recent week: %v", err)
		}
	}
}
