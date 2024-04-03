import requests
from requests.auth import HTTPBasicAuth
import sys
from datetime import datetime
import json
import zipfile
from os import path
import os

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
    login_url = 'https://api2.watttime.org/v2/login'
    response = requests.get(login_url, auth=HTTPBasicAuth(username, password))
    return response.json()['token']

def get_balancing_authority(token, latitude, longitude):
    # Define the URL and headers for the API request
    region_url = 'https://api2.watttime.org/v2/ba-from-loc'
    headers = {'Authorization': 'Bearer {}'.format(token)}
    params = {'latitude': latitude, 'longitude': longitude}

    # Make the API request
    response = requests.get(region_url, headers=headers, params=params)

    # Check if the API call was successful
    if response.status_code == 200:
        return response.json()['abbrev']
    elif response.status_code == 404:  # Location not supported
        print("Got 404")
        return None
    elif response.status_code == 403:  # Location not supported
        print("Got 403")
        return None
    else:
        print(f"Unexpected error: {response.content}")
        sys.exit("An unexpected error occurred while fetching the balancing authority.")

def fetch_and_save_historical_data(token, ba):
    """
    Fetch and save historical data for a given balancing authority.
    
    Parameters:
        token (str): WattTime API authorization token.
        ba (str): Abbreviation of the balancing authority.

    Returns:
        None
    """
    data_path = path.join("data", ba)
    if path.exists(data_path):
        print(f"Data for {ba} already exists locally.")
    else:
        # Construct historical data URL and headers
        historical_url = 'https://api2.watttime.org/v2/historical'
        headers = {'Authorization': f'Bearer {token}'}
        params = {'ba': ba}
        
        # Fetch historical data
        rsp = requests.get(historical_url, headers=headers, params=params)
        
        # Create a directory for the balancing authority
        if not os.path.exists(data_path):
            os.mkdir(data_path)

        metadata = {
            "generation_time": datetime.utcnow().strftime("%Y-%m-%dT%H:%M:%SZ"),
        }

        metapath = os.path.join(data_path, "meta.json")
        with open(metapath, 'w') as f:
            json.dump(metadata, f, indent=2)

        # Save the zip file
        zip_path = path.join("data", ba, f'{ba}_historical.zip')
        with open(zip_path, 'wb') as fp:
            fp.write(rsp.content)
        
        # Extract the zip file
        with zipfile.ZipFile(zip_path, 'r') as zip_ref:
            zip_ref.extractall(data_path)
        
        print(f"Wrote and unzipped historical data for {ba} to the directory: {data_path}")

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
    ba = get_balancing_authority(token, latitude, longitude)

    fetch_and_save_historical_data(token, ba)
