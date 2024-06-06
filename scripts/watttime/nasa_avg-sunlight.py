import requests
import json
import sys

def prompt_for_coordinates():
    """
    Prompt the user for latitude and longitude coordinates.
    
    Returns:
        latitude (float): The latitude input by the user.
        longitude (float): The longitude input by the user.
    """
    latitude = float(input("Please enter the latitude: "))
    longitude = float(input("Please enter the longitude: "))
    return latitude, longitude

def fetch_nasa_data(latitude, longitude):
    """
    Fetch NASA data for a given latitude and longitude.
    
    Parameters:
        latitude (float): The latitude coordinate.
        longitude (float): The longitude coordinate.
        
    Returns:
        data (dict): The JSON response from the NASA API parsed into a dictionary.
    """
    # Define the API endpoint and parameters
    url = "https://power.larc.nasa.gov/api/temporal/daily/point"
    params = {
        "parameters": "ALLSKY_SFC_SW_DWN",
        "community": "RE",
        "longitude": longitude,
        "latitude": latitude,
        "start": "20200101",
        "end": "20231231",
        "format": "json"
    }

    print(f"Using time range {params['start']} to {params['end']}")
    
    # Perform the API request and parse the JSON response
    response = requests.get(url, params=params)
    data = json.loads(response.text)
    
    return data

def calculate_average_sunlight(data):
    """
    Calculate the average annual sunlight from the NASA data.
    
    Parameters:
        data (dict): The parsed NASA API response.
        
    Returns:
        average_sunlight (float): The average annual sunlight in kW-hr/m^2/day.
    """
    # Extract the sunlight data points
    sunlight_data = data["properties"]["parameter"]["ALLSKY_SFC_SW_DWN"].values()
    
    # Filter out any fill values
    filtered_data = [x for x in sunlight_data if x != -999.0]
    
    # Calculate the average sunlight
    average_sunlight = sum(filtered_data) / len(filtered_data) if filtered_data else 0
    
    return average_sunlight

if __name__ == "__main__":
    # Command line: latitude longitude
    if len(sys.argv) < 3:
        print('No coordinates on command line, using Coit Tower (CAISO_NORTH)')
        latitude = 37.803
        longitude = -122.406
    else:
        latitude = sys.argv[1]
        longitude = sys.argv[2]
        
    # Step 2: Fetch NASA data
    data = fetch_nasa_data(latitude, longitude)
    
    # Step 3: Calculate the average annual sunlight
    average_sunlight = calculate_average_sunlight(data)
    
    # Step 4: Display the result
    print(f"The average annual sunlight for the coordinates {latitude}, {longitude} is {average_sunlight} kW-hr/m^2/day.")
