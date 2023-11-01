import requests
import json

# Fetch NASA POWER data for a specific latitude and longitude
def fetch_nasa_power_data(lat, lon, start_date, end_date):
    """
    Fetch solar insolation data from NASA POWER API for given latitude, longitude, and date range.
    
    Parameters:
        lat (float): Latitude of the location
        lon (float): Longitude of the location
        start_date (str): Start date in YYYY-MM-DD format
        end_date (str): End date in YYYY-MM-DD format
    
    Returns:
        dict: Dictionary of daily solar insolation values in kWh/m²/day
    """
    # NASA POWER API endpoint
    url = "https://power.larc.nasa.gov/api/temporal/daily/point?parameters=SI&community=RE&longitude={}&latitude={}&start={}&end={}&format=json".format(lon, lat, start_date, end_date)
    
    # Print the API request URL for debugging
    print(f"Making API request to: {url}")
    
    try:
        # Make the API request
        response = requests.get(url)
        
        # Check if the request was successful
        if response.status_code == 200:
            # Parse the JSON response
            data = json.loads(response.text)
            
            # Check if 'SI' data exists in the response
            if 'SI' in data['properties']['parameter']:
                # Extract daily insolation values
                insolation_values = data['properties']['parameter']['SI']
                return insolation_values
            else:
                print(f"Error: 'SI' data not found in API response. Full response: {data}")
                return None
        else:
            print(f"Error: Received status code {response.status_code}. Full response: {response.text}")
            return None
    
    except Exception as e:
        print(f"An exception occurred: {e}")
        return None

# Calculate the total insolation for a year
def calculate_yearly_insolation(lat, lon, year):
    """
    Calculate the total insolation received in a year for a given latitude and longitude.
    
    Parameters:
        lat (float): Latitude of the location
        lon (float): Longitude of the location
        year (int): The year for which to calculate insolation
    
    Returns:
        float: Total yearly insolation in kWh/m²
    """
    # Define the start and end date for the year
    start_date = "{}-01-01".format(year)
    end_date = "{}-12-31".format(year)
    
    # Fetch daily insolation values
    daily_insolation = fetch_nasa_power_data(lat, lon, start_date, end_date)
    
    if daily_insolation is not None:
        # Calculate the total yearly insolation
        total_insolation = sum(daily_insolation.values())
        return total_insolation
    else:
        print("Error: Could not fetch daily insolation data.")
        return None

# Example usage
lat = 40.7128  # Latitude for New York, NY
lon = -74.0060  # Longitude for New York, NY
year = 2022  # Year of interest

total_insolation = calculate_yearly_insolation(lat, lon, year)

if total_insolation is not None:
    print(f"Total yearly insolation for the location is {total_insolation} kWh/m²")

