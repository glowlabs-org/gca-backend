package server

import (
	"encoding/csv"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Define a struct to receive the information from the call to
// get historical sunlight information for a region.
type NASAResponse struct {
	Properties struct {
		Parameter map[string]map[string]float64 `json:"ALLSKY_SFC_SW_DWN"`
	} `json:"properties"`
}

// Define a struct to receive the information from the call to get
// a token from WattTime.
type WattTimeTokenResponse struct {
	Token string `json:"token"`
}

// Define a struct to receive the information from a call to get
// the balancing authority from WattTime
type BalancingAuthorityResponse struct {
	Abbrev string `json:"abbrev"`
}

// Define a struct that contains the response data for the call to
// the GeoStatsHandler.
type GeoStatsResponse struct {
	AverageSunlight           float64 `json:"average_sunlight"`
	AverageCarbonCertificates float64 `json:"average_carbon_certificates"`
}

// GeoStatsHandler will respond to a call to the /geo-stats api endpoint.
func GeoStatsHandler(w http.ResponseWriter, r *http.Request) {
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
	wtUsernamePath := filepath.Join("watttime_data", "username")
	wtPasswordPath := filepath.Join("watttime_data", "password"
	username := loadWattTimeCredentials(wtUsernamePath)
	password := loadWattTimeCredentials(wtPasswordPath)
	token, err := getWattTimeToken(username, password)
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
		http.Error(w, "Error in fetching balancing authority", http.StatusInternalServerError)
		return
	}

	// Get all of the historical data for this BA. It's a very expensive operation,
	// but only if the historical data is not cached locally already. Luckily, most
	// of the historical data is already cached locally.
	err = fetchAndSaveHistoricalBAData(token, ba)
	if err != nil {
		http.Error(w, "Error in fetching balancing authority", http.StatusInternalServerError)
		return
	}
	
	// Load the historical data from disk. The previous call to fetch the data saves
	// it to disk if the data is not already saved.
	baData, err := loadMOERData(ba)
	if err != nil {
		http.Error(w, "Error loading balancing authority historical data", http.StatusInternalServerError)
		return
	}

	// Calculate results
	averageSunlight, averageCarbonCredits, err := calculateCarbonCredits(nasaData)
	if err != nil {
		log.Println("Error in calculation:", err)
		http.Error(w, "Error in calculation", http.StatusInternalServerError)
		return
	}

	// Create response
	responseData := ResponseData{
		AverageSunlightPerDay: averageSunlight,
		AverageCarbonCredits:  averageCarbonCredits,
	}

	responseJSON, _ := json.Marshal(responseData)
	w.Header().Set("Content-Type", "application/json")
	w.Write(responseJSON)
}

func loadWattTimeCredentials(filename string) string {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatal("Error reading credentials file:", err)
	}
	return strings.TrimSpace(string(data))
}

func getWattTimeToken(username, password string) (string, error) {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", "https://api2.watttime.org/v2/login", nil)
	req.SetBasicAuth(username, password)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var tokenResponse TokenResponse
	json.NewDecoder(resp.Body).Decode(&tokenResponse)
	return tokenResponse.Token, nil
}

func fetchNASAData(latitude, longitude float64) (NASAData, error) {
	var data NASAData
	url := "https://power.larc.nasa.gov/api/temporal/hourly/point?parameters=ALLSKY_SFC_SW_DWN&community=RE&longitude=" +
		strconv.FormatFloat(longitude, 'f', -1, 64) + "&latitude=" +
		strconv.FormatFloat(latitude, 'f', -1, 64) + "&start=20220101&end=20221231&format=json"
	resp, err := http.Get(url)
	if err != nil {
		return data, err
	}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&data)
	return data, err
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
	var baResponse BAResponse
	if err := json.NewDecoder(resp.Body).Decode(&baResponse); err != nil {
		return "", err
	}

	return baResponse.Abbrev, nil
}

// fetchAndSaveHistoricalBAData fetches historical data for the given balancing authority and saves it locally.
func fetchAndSaveHistoricaBAlData(token, ba string) error {
	dataPath := filepath.Join("watttime_data", ba)
	if _, err := os.Stat(dataPath); !os.IsNotExist(err) {
		// Data already exists
		log.Println("Data for", ba, "already exists locally.")
		return nil
	}

	// Make directory for the BA
	if err := os.MkdirAll(dataPath, os.ModePerm); err != nil {
		return err
	}

	// Fetch historical data
	historicalURL := "https://api2.watttime.org/v2/historical"
	client := &http.Client{}
	req, err := http.NewRequest("GET", historicalURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	q := req.URL.Query()
	q.Add("ba", ba)
	req.URL.RawQuery = q.Encode()

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check for non-200 status code
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status code: %d", resp.StatusCode)
	}

	// Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Save the ZIP file
	zipPath := filepath.Join(dataPath, ba+"_historical.zip")
	if err := ioutil.WriteFile(zipPath, body, 0644); err != nil {
		return err
	}

	// Extract the ZIP file
	if err := extractZipFile(zipPath, dataPath); err != nil {
		return err
	}

	log.Println("Wrote and unzipped historical data for", ba, "to the directory:", dataPath)
	return nil
}

// extractZipFile extracts a ZIP file to the specified destination directory.
func extractZipFile(zipFile, destDir string) error {
	reader, err := zip.OpenReader(zipFile)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		path := filepath.Join(destDir, file.Name)
		if file.FileInfo().IsDir() {
			os.MkdirAll(path, os.ModePerm)
			continue
		}

		fileReader, err := file.Open()
		if err != nil {
			return err
		}
		defer fileReader.Close()

		targetFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return err
		}
		defer targetFile.Close()

		if _, err := io.Copy(targetFile, fileReader); err != nil {
			return err
		}
	}
	return nil
}

// MOERData represents the structure of MOER values.
type MOERData struct {
	Timestamp string
	MOER      float64
}

// loadMOERData loads MOER data from CSV files for the specified balancing authority.
func loadMOERData(ba string) (map[string]map[string][]float64, error) {
	folderPath := filepath.Join("watttime_data", ba)
	moerData := make(map[string]map[string][]float64)

	files, err := ioutil.ReadDir(folderPath)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".csv") {
			filePath := filepath.Join(folderPath, file.Name())
			fileData, err := readMOERCSV(filePath)
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

// readMOERCSV reads MOER values from a CSV file.
func readMOERCSV(filePath string) (map[string]map[string][]float64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	moerValues := make(map[string]map[string][]float64)

	_, err = reader.Read() // Skip header
	if err != nil {
		return nil, err
	}

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		timestamp, moerStr := record[0], record[1]
		moer, err := strconv.ParseFloat(moerStr, 64)
		if err != nil {
			continue
		}

		// Assuming the timestamp format is YYYY-MM-DDTHH
		parts := strings.Split(timestamp, "T")
		day := parts[0][5:] // Extract MM-DD
		hour := parts[1][:2]

		if _, ok := moerValues[day]; !ok {
			moerValues[day] = make(map[string][]float64)
		}
		moerValues[day][hour] = append(moerValues[day][hour], moer)
	}

	return moerValues, nil
}

// calculateGeoStats takes in the nasa data and ba data as input, and provides a
// computed average sunlight hours and average carbon impact.
func calculateCarbonCredits(nasaData NASAData, moerData map[string]map[string][]float64) (float64, float64, error) {
	totalKWh := 0.0
	totalHours := 0
	totalMOER := 0.0

	for dayData, sunIntensity := range nasaData.Properties.Parameter["ALLSKY_SFC_SW_DWN"] {
		if sunIntensity == 0 {
			continue
		}

		date, err := time.Parse("20060102", dayData)
		if err != nil {
			return 0, 0, err
		}

		day := fmt.Sprintf("%02d-%02d", date.Month(), date.Day())
		hour := strconv.Itoa(date.Hour())

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

	averageSunlightPerDay := totalKWh / float64(totalHours)
	averageMOERPerMWh := (totalMOER / totalKWh) * 1000

	return averageSunlightPerDay, averageMOERPerMWh, nil
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
