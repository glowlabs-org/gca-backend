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
    login_url = 'https://api2.watttime.org/v2/login'
    response = requests.get(login_url, auth=HTTPBasicAuth(username, password))
    return response.json()['token']

def list_regions_from_ba_maps(token):
    path = "data/ba_maps.json"
    if not os.path.exists(path):
        print("data/ba_maps.json not found")
        return
    with open(path, 'r') as f:
        mapdat = json.load(f)
    print(f"loaded {len(mapdat['features'])} ba regions")
    lmap = []
    for feat in mapdat['features']:
        coord = feat['geometry']['coordinates'][0][0][0]
        print(f"{feat['properties']['abbrev']:21} {feat['properties']['name']} ({coord[1]} {coord[0]})") # print lat long coordinates from the polygon

if __name__ == "__main__":
    # Load API credentials
    username = load_credentials('username')
    password = load_credentials('password')

    # Fetch WattTime API token
    token = get_token(username, password)

    # Command line: latitude longitude
    if len(sys.argv) < 2:
        print("usage: [ba region] or list")
    elif sys.argv[1] == "list":
        list_regions_from_ba_maps(token)
    else: 
        ba = sys.argv[1]
        fetch_and_save_historical_data(token, ba)
