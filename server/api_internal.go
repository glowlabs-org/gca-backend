package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"time"
)

// InternalWattTimeHistoricalHandler is an Internal API to test the WattTime historical query.
// For a latitude and longitude, a start time, and a duration, returns historical energy data
// for this range. Time format is yyyy-mm-ddThh:mm:ssZ.
func (gcas *GCAServer) InternalWattTimeHistoricalHandler(w http.ResponseWriter, r *http.Request) {
	if !gcas.allowIntApis {
		http.Error(w, "Not implemented in production", http.StatusNotImplemented)
		return
	}
	// Only allow GET calls.
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET method is allowed", http.StatusMethodNotAllowed)
		return
	}
	// Parse the input latitude and longitude.
	query := r.URL.Query()
	latitude, errLat := strconv.ParseFloat(query.Get("latitude"), 64)
	longitude, errLong := strconv.ParseFloat(query.Get("longitude"), 64)
	if errLat != nil || errLong != nil {
		http.Error(w, "Invalid coordinate query parameters", http.StatusBadRequest)
		return
	}
	// Parse the start time and duration
	start, startErr := time.Parse("2006-01-02T15:04:05Z", query.Get("start"))
	dur, durErr := time.ParseDuration(query.Get("dur"))
	if startErr != nil || durErr != nil {
		http.Error(w, "Invalid time query parameters", http.StatusBadRequest)
		return
	}

	// Load WattTime credentials and then get the auth token.
	wtUsernamePath := filepath.Join(gcas.baseDir, "watttime_data", "username")
	wtPasswordPath := filepath.Join(gcas.baseDir, "watttime_data", "password")
	username, err := loadWattTimeCredentials(wtUsernamePath)
	if err != nil {
		http.Error(w, "Error in loading watttime username", http.StatusInternalServerError)
		return
	}
	password, err := loadWattTimeCredentials(wtPasswordPath)
	if err != nil {
		http.Error(w, "Error in loading watttime password", http.StatusInternalServerError)
		return
	}
	token, err := staticGetWattTimeToken(username, password)
	if err != nil {
		http.Error(w, "Error in fetching watttime token", http.StatusInternalServerError)
		gcas.logger.Error("watttime fetch token error:", err)
		return
	}
	// Get the region and historical data
	region, err := getBalancingAuthority(token, latitude, longitude)
	if err != nil {
		http.Error(w, fmt.Sprintf("unable to get watttime region: %v", err), http.StatusInternalServerError)
		return
	}
	endTime := start.Add(dur)
	raw, err := getWattTimeHistoricalDataRaw(token, region, start.Unix(), endTime.Unix())
	if err != nil {
		http.Error(w, fmt.Sprintf("unable to get watttime historical data: %v", err), http.StatusInternalServerError)
		return
	}
	//br := bytes.NewReader(raw)
	/*	// Parse the JSON response
		type jsonResponse struct {
			Data []DataPoint `json:"data"`
			Meta struct{}    `json:"meta"`
		}
		var jr jsonResponse
		err = json.NewDecoder(br).Decode(&jr)
		if err != nil {
			http.Error(w, fmt.Sprintf("unable to parse watttime api data: %v", err), http.StatusInternalServerError)
			return
		}*/
	w.Header().Set("Content-Type", "application/json")
	w.Write(raw)
}

// InternalWattTimeSignalIndexHandler is an internal API to test the WattTime signal index query.
// For a latitude and longitude, returns a time point and an energy value.
func (gcas *GCAServer) InternalWattTimeSignalIndexHandler(w http.ResponseWriter, r *http.Request) {
	if !gcas.allowIntApis {
		http.Error(w, "Not implemented in production", http.StatusNotImplemented)
		return
	}
	// Only allow GET calls.
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET method is allowed", http.StatusMethodNotAllowed)
		return
	}
	// Parse the input coordinates.
	query := r.URL.Query()
	latitude, errLat := strconv.ParseFloat(query.Get("latitude"), 64)
	longitude, errLong := strconv.ParseFloat(query.Get("longitude"), 64)
	if errLat != nil || errLong != nil {
		http.Error(w, "Invalid query parameters", http.StatusBadRequest)
		return
	}
	// Load WattTime credentials and then get the auth token.
	wtUsernamePath := filepath.Join(gcas.baseDir, "watttime_data", "username")
	wtPasswordPath := filepath.Join(gcas.baseDir, "watttime_data", "password")
	username, err := loadWattTimeCredentials(wtUsernamePath)
	if err != nil {
		http.Error(w, "Error in loading watttime username", http.StatusInternalServerError)
		return
	}
	password, err := loadWattTimeCredentials(wtPasswordPath)
	if err != nil {
		http.Error(w, "Error in loading watttime password", http.StatusInternalServerError)
		return
	}
	token, err := staticGetWattTimeToken(username, password)
	if err != nil {
		http.Error(w, "Error in fetching watttime token", http.StatusInternalServerError)
		gcas.logger.Error("watttime fetch token error:", err)
		return
	}
	moer, epoch, region, err := getWattTimeIndex(token, latitude, longitude)
	if err != nil {
		http.Error(w, "Error in fetching watttime index", http.StatusInternalServerError)
		gcas.logger.Error("watttime fetch watttime index error:", err)
		return
	}
	type jsonResponse struct {
		Data struct {
			Time string  `json:"time"`
			Moer float64 `json:"moer"`
		} `json:"data"`
		Meta struct {
			Latitude   float64 `json:"latitude"`
			Longitude  float64 `json:"longitude"`
			Region     string  `json:"region"`
			SignalType string  `json:"signal_type"`
			Units      string  `json:"units"`
		} `json:"meta"`
	}
	t := time.Unix(epoch, 0)
	jr := jsonResponse{}
	jr.Data.Time = t.UTC().Format("2006-01-02 15:04:05 MST")
	jr.Data.Moer = moer
	jr.Meta.Latitude = latitude
	jr.Meta.Longitude = longitude
	jr.Meta.Region = region
	jr.Meta.SignalType = "co2_moer"
	jr.Meta.Units = "grams_co2_per_mwh"
	res, err := json.Marshal(jr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(res)
}
