package server

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strconv"
	"time"
)

// Internal API to test the WattTime signal index query. For a latitude and longitude, returns a time point.
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
	// Parse the input latitude and longitude
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
	moer, epoch, err := getWattTimeIndex(token, latitude, longitude)
	if err != nil {
		http.Error(w, "Error in fetching watttime index", http.StatusInternalServerError)
		gcas.logger.Error("watttime fetch watttime index error:", err)
		return
	}
	type jsonResponse struct {
		Time string  `json:"time"`
		Moer float64 `json:"moer"`
	}
	t := time.Unix(epoch, 0)
	jr := jsonResponse{
		Time: t.Local().Format("2006-01-02 15:04:05 MST"),
		Moer: moer,
	}
	res, err := json.Marshal(jr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(res)
}
