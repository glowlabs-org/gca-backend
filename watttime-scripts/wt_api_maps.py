import requests
from requests.auth import HTTPBasicAuth
import os
import sys
import json

# New function to save the balancing authority maps
def save_ba_maps(token):
    if token is None:
         sys.exit(f"Login failed")
    
    maps_url = 'https://api.watttime.org/v3/maps'
    headers = {'Authorization': 'Bearer {}'.format(token)}
    params = {
        'signal_type': 'co2_moer',
    }
    response = requests.get(maps_url, headers=headers, params=params)
    
    if response.status_code != 200:
        sys.exit(f"Failed to get maps, status code: {response.status_code}")

    # Make sure the 'data' directory exists
    cur_dir = os.path.dirname(os.path.realpath('__file__'))
    data_dir = os.path.join(cur_dir, 'data')
    if not os.path.exists(data_dir):
        os.makedirs(data_dir)

    file_path = os.path.join(data_dir, 'ba_maps.json')
    
    with open(file_path, 'w') as fp:
        # Assuming the content is JSON formatted, directly use json.dump
        json.dump(response.json(), fp, indent=2)

    print(f"Balancing authority maps saved to {file_path}")

if __name__ == "__main__":
    # Assumes 'token' file exists. If it does not or token is expired,
    # call the wt_api_login.py script again.
    
    with open('token', 'r') as f:
        token = f.read().strip()

    save_ba_maps(token)
