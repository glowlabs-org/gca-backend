package server

// TODO: We should probably have some sort of process that will periodically
// look for gaps in the equipment and call '/data' to get the missing bits.
// '/data' gives up to 32 days of historical data for a ba, and this server
// only tracks 2 weeks of data anyway.

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
//
// TODO: Have this function return a proper error.
func loadWattTimeCredentials(filename string) string {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		panic("unable to load watttime credentials")
	}
	return strings.TrimSpace(string(data))
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
		return latitude + longitude + 1.5, time.Now().Unix(), nil
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
		point_time string
		moer       float64
	}
	var ir indexResponse
	err = json.NewDecoder(resp.Body).Decode(&ir)
	if err != nil {
		return 0, 0, fmt.Errorf("Unable to parse api response: %v", err)
	}
	// Parse the string time.
	t, err := time.Parse(ir.point_time, ir.point_time)
	if err != nil {
		return 0, 0, fmt.Errorf("Unable to parse response time: %v", err)
	}
	return ir.moer, t.Unix(), nil
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
		case <-time.After(2 * time.Minute):
			// TODO: Need to swap this to a const
		}

		// Get a new auth token. They expire relatively quickly so we
		// have to keep refreshing them.
		token, err := getWattTimeToken(username, password)
		if err != nil {
			gcas.logger.Errorf("unable to get watttime token: %v", err)
			continue
		}

		// Grab a list of devices to loop over. We grab the list with a
		// mutex so that we don't have to hold a lock while doing
		// network operations for each device.
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
			moer, time, err := getWattTimeIndex(token, lats[i], longs[i])
			if err != nil {
				gcas.logger.Errorf("unable to get watttime data: %v", err)
				continue
			}
			timeslot, err := glow.UnixToTimeslot(time)
			if err != nil {
				gcas.logger.Errorf("watttime returned data at an invalid timeslot: %v", err)
				continue
			}

			// Update the struct which tracks the moer times.
			save := true
			gcas.mu.Lock()
			impactIndex := timeslot - gcas.equipmentReportsOffset
			if moer == 0 || gcas.equipmentImpactRate[shortID][impactIndex] == moer {
				save = false
			}
			gcas.equipmentImpactRate[shortID][impactIndex] = moer
			gcas.mu.Unlock()

			if save {
				// TODO: persist this reading to disk.
			}
		}
	}
}
