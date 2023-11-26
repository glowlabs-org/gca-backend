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
	// value.
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
	return moer, t.Unix(), nil
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

		err := gcas.managedGetWattTimeData(username, password)
		if err != nil {
			gcas.logger.Errorf("unable to complete WattTime data update: %v", err)
			continue
		}
	}
}

// managedGettWattTimeData performs a single round of grabbing the current
// index data from WattTime for every device.
func (gcas *GCAServer) managedGetWattTimeData(username, password string) error {
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
		// Fetch the current results for this device.
		moer, date, err := getWattTimeIndex(token, lats[i], longs[i])
		if err != nil {
			gcas.logger.Errorf("unable to get watttime data: %v", err)
			continue
		}
		if !testMode {
			// We don't want to hit the WattTime ratelimits, so we
			// sleep a bit after making a request to ensure that we
			// don't go too far.
			time.Sleep(250 * time.Millisecond)
		}
		timeslot, err := glow.UnixToTimeslot(date)
		if err != nil {
			gcas.logger.Errorf("watttime returned data at an invalid timeslot: %v", err)
			gcas.logger.Errorf("time: %v, genesis time: %v", date, glow.GenesisTime)
			continue
		}

		// Update the struct which tracks the moer times. If the clock
		// goes backwards in time for some reason, this can panic, so
		// we have to double check.
		save := true
		gcas.mu.Lock()
		impactIndex := timeslot - gcas.equipmentReportsOffset
		if moer == 0 || (timeslot > gcas.equipmentReportsOffset && gcas.equipmentImpactRate[shortID][impactIndex] == moer) {
			save = false
		}
		gcas.equipmentImpactRate[shortID][impactIndex] = moer
		gcas.mu.Unlock()

		if save {
			// TODO: persist this reading to disk. Thinking about
			// this a little more, we should save the readings to
			// disk one-by-one so that there's a logged history of
			// readings that have been made. Which device, what
			// time, what reading.
			//
			// When we load the readings later, I don't think the
			// offset will have updated so we can fill out the
			// things and still get the correct historical values.
			//
			// We should probably just have the server refuse to
			// start if it has been offline for more than 7 days.
			// You have to hard-reset it and let it resync with
			// it's peers.
		}
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
