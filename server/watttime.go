package server

// TODO: We should probably have some sort of process that will periodically
// look for gaps in the equipment and call '/data' to get the missing bits.
// '/data' gives up to 32 days of historical data for a ba, and this server
// only tracks 2 weeks of data anyway.

// TODO: We need some way to associate a solar farm to a wallet address. We
// don't have that at the moment.

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/glowlabs-org/gca-backend/glow"
)

// loadWattTimeCredentials is a helper function to load one of
// the watttime credential files from disk.
func loadWattTimeCredentials(filename string) (string, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf("unable to read watttime credentials: %v", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// getBalancingAuthority makes an API call to watttime to get the ba that's associated
// with a specific location
func getBalancingAuthority(token string, latitude, longitude float64) (string, error) {
	client := &http.Client{}
	regionURL := "https://api2.watttime.org/v2/ba-from-loc"
	req, err := http.NewRequest("GET", regionURL, nil)
	if err != nil {
		return "", err
	}

	// Set the authorization header and query parameters
	req.Header.Set("Authorization", "Bearer "+token)
	q := req.URL.Query()
	q.Add("latitude", strconv.FormatFloat(latitude, 'f', 6, 64))
	q.Add("longitude", strconv.FormatFloat(longitude, 'f', 6, 64))
	req.URL.RawQuery = q.Encode()

	// Make the API request
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Check for non-200 status code
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed with status code: %d", resp.StatusCode)
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
func getWattTimeIndex(token string, latitude float64, longitude float64) (float64, int64, error) {
	// Since this code depends on external APIs, we return an arbitrary
	// value during testing.
	if testMode {
		return latitude + longitude + 200 + float64(time.Now().UnixNano()%250), glow.TimeslotToUnix(glow.CurrentTimeslot()), nil
	}

	// Create the base url
	client := &http.Client{}
	url := "https://api2.watttime.org/v2/index"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("unable to get index: %v", err)
	}

	// Set the parameters
	req.Header.Set("Authorization", "Bearer "+token)
	q := req.URL.Query()
	q.Add("latitude", strconv.FormatFloat(latitude, 'f', 6, 64))
	q.Add("longitude", strconv.FormatFloat(longitude, 'f', 6, 64))
	q.Add("style", "moer")
	req.URL.RawQuery = q.Encode()

	// Make the API request
	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("Unable to make api request")
	}
	defer resp.Body.Close()
	// Check for non-200 status code
	if resp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("API request failed with status code: %d", resp.StatusCode)
	}

	// Parse the JSON respose.
	type indexResponse struct {
		Point_time string
		Moer       string
	}
	var ir indexResponse
	err = json.NewDecoder(resp.Body).Decode(&ir)
	if err != nil {
		return 0, 0, fmt.Errorf("Unable to parse api response: %v", err)
	}
	// Parse the string time.
	t, err := time.Parse("2006-01-02T15:04:05Z", ir.Point_time)
	if err != nil {
		return 0, 0, fmt.Errorf("Unable to parse response time: %v", err)
	}
	// Parse the string float.
	moer, err := strconv.ParseFloat(ir.Moer, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("Unable to parse response moer: %v", err)
	}
	// Convert the moer to grams per megawatt hour. Moer is provided in
	// pounds per megawatt hour. We multiply by 453.59237 to get from
	// pounds per megawatt hour to grams per megawatt hour.
	moer *= 453.59237
	return moer, t.Unix(), nil
}

// getWattTimeData returns the MOER values for the provided lat+long from the
// provided time to 1 week later.
func getWattTimeData(token string, latitude float64, longitude float64, startTime int64) ([]float64, []int64, error) {
	// Determine what data range can be requested. If the end time is
	// within 5 days of the current time, WattTime will be asked for the
	// full set of data that it has. In the WattTime API, omitting the end
	// time will result in all data being collected up to the present.
	idealEndTime := startTime + 604800 // number of seconds in a week
	useEndTime := true
	if idealEndTime+432000 > time.Now().Unix() {
		useEndTime = false
	}

	// During testing we return blank values, that way it doesn't interfere
	// with testing that's probing individual fields.
	if testMode {
		return nil, nil, nil
	}

	// Convert the times to ISO 8601.
	startTimeT := time.Unix(startTime, 0)
	startTimeISO := startTimeT.Format("2006-01-02T15:04:05Z")
	endTimeT := time.Unix(idealEndTime, 0)
	endTimeISO := endTimeT.Format("2006-01-02T15:04:05Z")

	// Create the base url
	client := &http.Client{}
	url := "https://api2.watttime.org/v2/data"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to get index: %v", err)
	}

	// Set the parameters
	req.Header.Set("Authorization", "Bearer "+token)
	q := req.URL.Query()
	q.Add("latitude", strconv.FormatFloat(latitude, 'f', 6, 64))
	q.Add("longitude", strconv.FormatFloat(longitude, 'f', 6, 64))
	q.Add("starttime", startTimeISO)
	if useEndTime {
		q.Add("endtime", endTimeISO)
	}
	req.URL.RawQuery = q.Encode()

	// Make the API request
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("Unable to make api request")
	}
	defer resp.Body.Close()
	// Check for non-200 status code
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("API request failed with status code: %d", resp.StatusCode)
	}

	// Parse the JSON respose.
	type indexResponse struct {
		Point_time string
		Value      float64
	}
	var irs []indexResponse
	err = json.NewDecoder(resp.Body).Decode(&irs)
	if err != nil {
		return nil, nil, fmt.Errorf("Unable to parse api response: %v", err)
	}

	// Build the return values.
	var moers []float64
	var dates []int64
	for _, ir := range irs {
		// Parse the string time.
		t, err := time.Parse("2006-01-02T15:04:05Z", ir.Point_time)
		if err != nil {
			return nil, nil, fmt.Errorf("Unable to parse response time: %v", err)
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
		// Soft sleep before collecting data. We sleep before instead
		// of after so that any errors can just 'continue' to the next
		// iteration of the loop and the sleep will happen.
		select {
		case <-gcas.quit:
			return
		case <-time.After(wattTimeFrequency):
		}

		err := gcas.managedGetWattTimeIndexData(username, password)
		if err != nil {
			gcas.logger.Errorf("unable to complete WattTime data update: %v", err)
			continue
		}
	}
}

// managedGettWattTimeIndexData performs a single round of grabbing the current
// index data from WattTime for every device.
func (gcas *GCAServer) managedGetWattTimeIndexData(username, password string) error {
	// Get a new auth token. They expire relatively quickly so it's better
	// to get a new token every time this function is called.
	token, err := getWattTimeToken(username, password)
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
		moer, date, err := getWattTimeIndex(token, lats[i], longs[i])
		if err != nil {
			gcas.logger.Errorf("unable to get watttime data: %v", err)
			continue
		}
		timeslot, err := glow.UnixToTimeslot(date)
		if err != nil {
			gcas.logger.Errorf("watttime returned data at an invalid timeslot: %v", err)
			gcas.logger.Errorf("time: %v, genesis time: %v", date, glow.GenesisTime)
			continue
		}

		// Update the struct which tracks the moer times. If the clock
		// goes backwards in time for some reason, this can panic, so
		// we have to double check the bounds.
		gcas.mu.Lock()
		impactIndex := timeslot - gcas.equipmentReportsOffset
		if timeslot >= gcas.equipmentReportsOffset {
			gcas.equipmentImpactRate[shortID][impactIndex] = moer
		}
		gcas.mu.Unlock()
	}
	return nil
}

// managedGetWattTimeWeekData will grab all of the data for the latest week and
// fill out the impact rates as much as possible.
func (gcas *GCAServer) managedGetWattTimeWeekData(username, password string) error {
	// Get a new auth token. They expire relatively quickly so it's better
	// to get a new token every time this function is called.
	token, err := getWattTimeToken(username, password)
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
		moers, dates, err := getWattTimeData(token, lats[i], longs[i], startTime)
		if err != nil {
			gcas.logger.Errorf("unable to get watttime data: %v", err)
			continue
		}

		// Integrate all the data in one loop with the lock held.
		gcas.mu.Lock()
		for i, date := range dates {
			timeslot, err := glow.UnixToTimeslot(date)
			if err != nil {
				gcas.logger.Errorf("watttime returned data at an invalid timeslot: %v", err)
				gcas.logger.Errorf("time: %v, genesis time: %v", date, glow.GenesisTime)
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

// getWattTimeToken makes an API call to WattTime to authenticate and retrieve an access token.
func getWattTimeToken(username, password string) (string, error) {
	// Don't hit the watttime api during testing.
	if testMode {
		return "fake-token", nil
	}

	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://api2.watttime.org/v2/login", nil)
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
			return "", fmt.Errorf("authentication failed: invalid credentials")
		}
		return "", fmt.Errorf("API request failed with status code: %d", resp.StatusCode)
	}

	var tokenResponse WattTimeTokenResponse
	err = json.NewDecoder(resp.Body).Decode(&tokenResponse)
	if err != nil {
		return "", err
	}

	return tokenResponse.Token, nil
}
