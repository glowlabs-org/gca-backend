package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Define a struct that contains the response data for the call to
// the GeoStatsHandler.
type GeoStatsResponse struct {
	AverageSunlight           float64 `json:"average_sunlight"`
	AverageCarbonCertificates float64 `json:"average_carbon_certificates"`
}

// GeoStatsHandler will respond to a call to the /geo-stats api endpoint.
func (gcas *GCAServer) GeoStatsHandler(w http.ResponseWriter, r *http.Request) {
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
		return
	}

	// Fetch NASA data for this coordinate.
	nasaData, err := fetchNASAData(latitude, longitude)
	if err != nil {
		http.Error(w, "Error in fetching nasa data", http.StatusInternalServerError)
		return
	}

	// Fetch the balancing authority for this coordinate.
	ba, err := getBalancingAuthority(token, latitude, longitude)
	if err != nil {
		fmt.Println("Error fetching balancing authority:", err)
		http.Error(w, "Error in fetching balancing authority", http.StatusInternalServerError)
		return
	}

	// Get all of the historical data for this BA. It's a very expensive operation,
	// but only if the historical data is not cached locally already. Luckily, most
	// of the historical data is already cached locally.
	err = gcas.fetchAndSaveHistoricalBAData(token, ba)
	if err != nil {
		fmt.Println("Error fetching balancing authority:", err)
		http.Error(w, "Error in fetching balancing authority", http.StatusInternalServerError)
		return
	}

	// Load the historical data from disk. The previous call to fetch the data saves
	// it to disk if the data is not already saved.
	baData, err := gcas.loadMOERData(ba)
	if err != nil {
		http.Error(w, "Error loading balancing authority historical data", http.StatusInternalServerError)
		return
	}

	// Calculate results
	averageSunlight, averageCarbonCredits, err := calculateGeoStats(nasaData, baData)
	if err != nil {
		log.Println("Error in calculation:", err)
		http.Error(w, "Error in calculation", http.StatusInternalServerError)
		return
	}

	// Create response
	responseData := GeoStatsResponse{
		AverageSunlight:           averageSunlight,
		AverageCarbonCertificates: averageCarbonCredits,
	}

	if err := json.NewEncoder(w).Encode(responseData); err != nil {
		http.Error(w, "Failed to encode JSON response", http.StatusInternalServerError)
		return
	}
}

// fetchNASAData is a helper function to download the historical sunlight data
// for a given geographical coordinate from NASA.
func fetchNASAData(latitude, longitude float64) (map[string]float64, error) {
	type Parameter struct {
		AllSkySfcSwDwn map[string]float64 `json:"ALLSKY_SFC_SW_DWN"`
	}
	type Properties struct {
		Parameter Parameter `json:"parameter"`
	}
	type Response struct {
		Type       string     `json:"type"`
		Properties Properties `json:"properties"`
	}

	baseURL := "https://power.larc.nasa.gov/api/temporal/hourly/point"
	// Create a map for the query parameters
	params := url.Values{}
	params.Add("parameters", "ALLSKY_SFC_SW_DWN")
	params.Add("community", "RE")
	params.Add("longitude", strconv.FormatFloat(longitude, 'f', -1, 64))
	params.Add("latitude", strconv.FormatFloat(latitude, 'f', -1, 64))
	params.Add("start", strconv.Itoa(WattTimeYear)+"0101")
	params.Add("end", strconv.Itoa(WattTimeYear)+"1231")
	params.Add("format", "json")
	// Construct the final URL with encoded query parameters
	finalURL := baseURL + "?" + params.Encode()

	// Now make the request
	resp, err := http.Get(finalURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var data Response
	err = json.NewDecoder(resp.Body).Decode(&data)
	return data.Properties.Parameter.AllSkySfcSwDwn, err
}

// fetchAndSaveHistoricalBAData fetches historical data for the given balancing
// authority and saves it locally. WattTime allows querying historical data
// for up to 32 days, so we will create 12 data files for each month of the year.
func (gcas *GCAServer) fetchAndSaveHistoricalBAData(token, ba string) error {
	dataPath := filepath.Join(gcas.baseDir, "watttime_data", ba)
	if err := os.MkdirAll(dataPath, os.ModePerm); err != nil {
		return err
	}
	// Make a list of files we need to load for each month's data. Since WattTime's API
	// allows a maximum of 32 days per query, we will query by month.
	type Info struct {
		name  string
		start time.Time
		end   time.Time
	}
	needed := make([]Info, 0)
	for month := 1; month <= 12; month++ {
		year := WattTimeYear
		start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC) // First second of the month
		// Calculate the last second of the month by by using the last second
		// of the "0th day" of the next month. The 0th day is calculated as the last day
		// of the previous month, so this calculation will give the result we need.
		end := time.Date(year, time.Month(month+1), 0, 23, 59, 59, 0, time.UTC)
		fname := fmt.Sprintf("%s_%d-%02d_MOER.json", ba, year, month)
		filePath := filepath.Join(dataPath, fname)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			needed = append(needed, Info{filePath, start, end})
		}
	}
	for _, f := range needed {
		raw, err := getWattTimeHistoricalDataRaw(token, ba, f.start.Unix(), f.end.Unix())
		if err != nil {
			return err
		}
		if err = os.WriteFile(f.name, raw, 0644); err != nil {
			return err
		}
		gcas.logger.Info("wrote historical data file: ", f.name)
	}
	return nil
}

// MOERData represents the structure of MOER values.
type MOERData struct {
	Timestamp string
	MOER      float64
}

// loadMOERData loads MOER data from CSV files for the specified balancing authority.
func (gcas *GCAServer) loadMOERData(ba string) (map[string]map[string][]float64, error) {
	dataPath := filepath.Join(gcas.baseDir, "watttime_data", ba)
	moerData := make(map[string]map[string][]float64)

	entries, err := os.ReadDir(dataPath)
	files := make([]fs.FileInfo, 0, len(entries))
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		fi, err := entry.Info()
		if err != nil {
			return nil, err
		}
		files = append(files, fi)
	}
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".json") {
			filePath := filepath.Join(dataPath, file.Name())
			fileData, err := readMOERJson(filePath)
			if err != nil {
				return nil, err
			}

			for day, hours := range fileData {
				if _, ok := moerData[day]; !ok {
					moerData[day] = make(map[string][]float64)
				}
				for hour, values := range hours {
					moerData[day][hour] = append(moerData[day][hour], values...)
				}
			}
		}
	}

	return moerData, nil
}

// readMOERJson reads MOER values from a Json formatted file.
func readMOERJson(filePath string) (map[string]map[string][]float64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var jd DataPointsJSON
	err = json.NewDecoder(file).Decode(&jd)
	if err != nil {
		return nil, err
	}

	// Convert the data to moer map
	moerValues := make(map[string]map[string][]float64)
	for _, dp := range jd.Data {
		moer := dp.Value
		// Assuming the timestamp format is YYYY-MM-DDTHH
		parts := strings.Split(dp.PointTime, "T")
		year := parts[0][:4] // Extract YYYY
		day := parts[0][5:]  // Extract MM-DD
		hour := parts[1][:2] // Extract TT
		if year != strconv.Itoa(WattTimeYear) {
			continue
		}
		if _, ok := moerValues[day]; !ok {
			moerValues[day] = make(map[string][]float64)
		}
		moerValues[day][hour] = append(moerValues[day][hour], moer)
	}

	return moerValues, nil
}

// calculateGeoStats takes in the nasa data and ba data as input, and provides
// a computed average sunlight hours and average carbon impact.
func calculateGeoStats(nasaData map[string]float64, moerData map[string]map[string][]float64) (float64, float64, error) {
	totalKWh := 0.0
	totalHours := 0
	totalMOER := 0.0

	for dayData, sunIntensity := range nasaData {
		day := fmt.Sprintf("%s-%s", dayData[4:6], dayData[6:8])
		hour := dayData[8:10]

		moerValues, exists := moerData[day][hour]
		if !exists || len(moerValues) == 0 {
			continue
		}

		averageMOER := calculateAverage(moerValues)
		averageMOERMetricTons := averageMOER / 2204.62
		sunIntensityKW := sunIntensity / 1000

		totalKWh += sunIntensityKW
		totalMOER += averageMOERMetricTons * sunIntensityKW
		totalHours++
	}

	avgSun := 24 * totalKWh / float64(totalHours)
	averageMOER := (totalMOER / totalKWh)

	return avgSun, averageMOER, nil
}

// calculateAverage computes the average of a slice of float64.
func calculateAverage(nums []float64) float64 {
	total := 0.0
	for _, num := range nums {
		total += num
	}
	if len(nums) == 0 {
		return 0
	}
	return total / float64(len(nums))
}
