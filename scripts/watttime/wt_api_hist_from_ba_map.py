import sys
import os
import json
import requests
from requests.auth import HTTPBasicAuth
from wt_api_hist_download import fetch_and_save_historical_data

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

def get_token(username, password):
    login_url = 'https://api.watttime.org/login'
    response = requests.get(login_url, auth=HTTPBasicAuth(username, password))
    return response.json()['token']

def load_ba_regions():
    """
    Load the BA regions from the JSON file.

    Returns:
        list: List of BA regions and their details.
    """
    path = "data/ba_maps.json"
    if not os.path.exists(path):
        print("data/ba_maps.json not found")
        return None
    with open(path, 'r') as f:
        mapdat = json.load(f)
    print(f"loaded {len(mapdat['features'])} ba regions")
    return mapdat['features']

def list_regions_from_ba_maps(features):
    """
    List all regions from the BA maps.

    Parameters:
        features (list): List of features from the BA map data.
    """
    lmap = []
    for feat in features:
        coord = feat['geometry']['coordinates'][0][0][0]
        print(f"{feat['properties']['region']:21} {feat['properties']['region_full_name']} ({coord[1]} {coord[0]})") # print lat long coordinates from the polygon

def fetch_all_ba_data(token, features):
    """
    Fetch data for all BA regions.

    Parameters:
        token (str): The API token.
        features (list): List of BA regions.
    """
    for feat in features:
        ba_region = feat['properties']['region']
        print(f"Fetching data for {ba_region}...")
        fetch_and_save_historical_data(token, ba_region)

if __name__ == "__main__":
    # Load API credentials
    username = load_credentials('username')
    password = load_credentials('password')

    # Fetch WattTime API token
    token = get_token(username, password)

    # Load BA regions from the map file
    features = load_ba_regions()

    if features is None:
        sys.exit(1)

    # Command line: latitude longitude
    if len(sys.argv) < 2:
        # If no argument is provided, fetch data for all regions
        print("No arguments provided. Fetching data for all BA regions.")
        fetch_all_ba_data(token, features)
    elif sys.argv[1] == "list":
        # List all regions if "list" argument is provided
        list_regions_from_ba_maps(features)
    else:
        # Fetch data for the specific BA region provided
        ba = sys.argv[1]
        fetch_and_save_historical_data(token, ba)
