import requests
import json
from requests.auth import HTTPBasicAuth

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
    url = "https://power.larc.nasa.gov/api/temporal/daily/point"
    params = {
        "parameters": "ALLSKY_SFC_SW_DWN",
        "community": "RE",
        "longitude": longitude,
        "latitude": latitude,
        "start": "20200101",
        "end": "20221231",
        "format": "json"
    }
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
    sunlight_data = data["properties"]["parameter"]["ALLSKY_SFC_SW_DWN"].values()
    filtered_data = [x for x in sunlight_data if x != -999.0]
    average_sunlight = sum(filtered_data) / len(filtered_data) if filtered_data else 0
    return average_sunlight

# New function to load credentials from a file
def load_credentials(filename):
    """
    Load credentials from a given file.

    Parameters:
        filename (str): The name of the file containing the credentials.

    Returns:
        str: The credentials read from the file.
    """
    with open(filename, 'r') as f:
        return f.read().strip()

# New function to get the WattTime API token
def get_token(username, password):
    """
    Fetch the authorization token from WattTime API.

    Parameters:
        username (str): The username for the API.
        password (str): The password for the API.

    Returns:
        str: The authorization token.
    """
    login_url = 'https://api2.watttime.org/v2/login'
    response = requests.get(login_url, auth=HTTPBasicAuth(username, password))
    return response.json()['token']

# New function to fetch the balancing authority
def get_balancing_authority(token, latitude, longitude):
    """
    Fetch the balancing authority based on latitude and longitude.

    Parameters:
        token (str): The authorization token for the WattTime API.
        latitude (float): The latitude coordinate.
        longitude (float): The longitude coordinate.

    Returns:
        str: The balancing authority.
    """
    region_url = 'https://api2.watttime.org/v2/ba-from-loc'
    headers = {'Authorization': 'Bearer {}'.format(token)}
    params = {'latitude': latitude, 'longitude': longitude}
    response = requests.get(region_url, headers=headers, params=params)
    return response.json()['abbrev']

if __name__ == "__main__":
    # Load WattTime API credentials
    username = load_credentials('username')
    password = load_credentials('password')

    # Get the WattTime API token
    token = get_token(username, password)

    # Prompt user for coordinates
    latitude, longitude = prompt_for_coordinates()

    # Fetch NASA data
    data = fetch_nasa_data(latitude, longitude)
  
    # Calculate the average annual sunlight
    average_sunlight = calculate_average_sunlight(data)

    # Fetch and print the balancing authority
    balancing_authority = get_balancing_authority(token, latitude, longitude)

    print(f"The average annual sunlight for the coordinates {latitude}, {longitude} is {average_sunlight} kW-hr/m^2/day.")
    print(f"The balancing authority for the coordinates {latitude}, {longitude} is {balancing_authority}.")

