################################################################################
# Requirements:
# The user wants to:
# 1. Authenticate with the WattTime API using credentials stored in files.
# 2. Fetch data from the WattTime endpoint that provides balancing authority maps.
# 3. Extract each WattTime region and corresponding country from the response.
# 4. Write the data into a CSV file that maps each region to its country.
#
# Assumptions:
# - The username and password are stored in files named 'username' and 'password'
#   respectively, each containing only the credential (no extra whitespace or lines).
# - The script should authenticate with the WattTime API and use the returned token
#   to fetch balancing authority (BA) maps from the /v3/maps endpoint.
# - The response from /v3/maps includes a structure from which we can extract
#   properties about each BA, including a region code (e.g., abbrev) and country.
# - The exact structure returned by /v3/maps is assumed to contain features with
#   "properties" that include fields like "ba_name", "ba_abbrev" (region code),
#   and "country". If the structure differs, adjustments will be needed.
#
# Output:
# - A CSV file named 'regions.csv' with the following fields:
#   region_code,country
# - Each row corresponds to one WattTime region and its associated country.
#
# Approach:
# 1. Load credentials from files.
# 2. Get a token from the WattTime API.
# 3. Call the /v3/maps endpoint to retrieve BA maps data.
# 4. Parse the returned JSON to extract region_code and country for each BA.
# 5. Write the extracted data into 'regions.csv'.
#
# Error handling:
# - If authentication or data fetch fails, the script will exit with an error message.
# - If no matching properties are found, the CSV will be empty or contain only headers.
#
################################################################################

import requests
from requests.auth import HTTPBasicAuth
import os
import sys
import json
import csv

# Load credentials from file
def load_credentials(filename):
    # Attempts to read a single line from the specified file and return it stripped
    with open(filename, 'r') as f:
        return f.read().strip()

# Request an authentication token from WattTime
def get_token(username, password):
    login_url = 'https://api.watttime.org/login'
    response = requests.get(login_url, auth=HTTPBasicAuth(username, password))
    if response.status_code == 200:
        return response.json()['token']
    else:
        sys.exit(f"Failed to get token, status code: {response.status_code}")

# Fetch balancing authority data from WattTime using the token
def fetch_ba_data(token):
    maps_url = 'https://api.watttime.org/v3/maps'
    headers = {'Authorization': f'Bearer {token}'}
    params = {'signal_type': 'co2_moer'}
    
    response = requests.get(maps_url, headers=headers, params=params)
    if response.status_code != 200:
        sys.exit(f"Failed to get maps, status code: {response.status_code}")
    return response.json()

# Extract region_code and country from the BA data
def extract_regions(ba_data):
    # The data is assumed to have a structure like:
    # {
    #   "features": [
    #       {
    #         "properties": {
    #             "ba_name": "...",
    #             "ba_abbrev": "...",
    #             "country": "..."
    #         },
    #         ...
    #       }, ...
    #   ]
    # }
    regions = []
    features = ba_data.get('features', [])
    for feature in features:
        props = feature.get('properties', {})
        region_code = props.get('ba_abbrev')
        country = props.get('country')
        if region_code and country:
            regions.append((region_code, country))
    return regions

# Write the extracted regions to a CSV file
def write_csv(regions, filename='data/regions.csv'):
    # Ensure output directory exists
    with open(filename, 'w', newline='') as f:
        writer = csv.writer(f)
        writer.writerow(['region_code', 'country'])
        for region_code, country in regions:
            writer.writerow([region_code, country])
    print(f"CSV with WattTime regions saved to {filename}")

if __name__ == "__main__":
    username = load_credentials('username')
    password = load_credentials('password')
    token = get_token(username, password)
    ba_data = fetch_ba_data(token)
    regions = extract_regions(ba_data)
    write_csv(regions)
