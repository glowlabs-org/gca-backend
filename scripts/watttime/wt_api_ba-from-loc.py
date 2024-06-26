import requests
from requests.auth import HTTPBasicAuth
import sys

# Function to load credentials from a file
def load_credentials(filename):
    """
    Load credentials from a given file.

    Parameters:
        filename (str): The name of the file containing the credential.

    Returns:
        str: The credential read from the file.
    """
    with open(filename, 'r') as f:
        return f.read().strip()

# Function to get token for WattTime API
def get_token(username, password):
    """
    Fetch the authorization token from WattTime API.

    Parameters:
        username (str): The username for the API.
        password (str): The password for the API.

    Returns:
        str: The authorization token.
    """
    login_url = 'https://api.watttime.org/login'
    response = requests.get(login_url, auth=HTTPBasicAuth(username, password))
    return response.json()['token']

# Function to fetch region information based on coordinates
def get_region_info(token, latitude, longitude):
    """
    Fetch the region information based on latitude and longitude.

    Parameters:
        token (str): The authorization token for the API.
        latitude (str): The latitude coordinate.
        longitude (str): The longitude coordinate.

    Returns:
        str: The API response as a string.
    """
    region_url = 'https://api.watttime.org/v3/region-from-loc'
    headers = {'Authorization': 'Bearer {}'.format(token)}
    params = {'latitude': latitude, 'longitude': longitude, 'signal_type': 'co2_moer'}
    response = requests.get(region_url, headers=headers, params=params)
    return response.text

if __name__ == "__main__":
    # Command line: latitude longitude
    if len(sys.argv) < 3:
        print('No coordinates on command line, using Coit Tower (CAISO_NORTH)')
        latitude = 37.803
        longitude = -122.406
    else:
        latitude = sys.argv[1]
        longitude = sys.argv[2]

    # Load username and password
    username = load_credentials('username')
    password = load_credentials('password')
    
    # Get the token
    token = get_token(username, password)
    
    # Fetch and print region information
    print(f"latitude {latitude} longitude {longitude}")
    region_info = get_region_info(token, latitude, longitude)
    print(region_info)

