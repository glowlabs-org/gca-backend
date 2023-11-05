import requests
from requests.auth import HTTPBasicAuth
import os
import sys
import json

# Your existing functions
def load_credentials(filename):
    with open(filename, 'r') as f:
        return f.read().strip()

def get_token(username, password):
    login_url = 'https://api2.watttime.org/v2/login'
    response = requests.get(login_url, auth=HTTPBasicAuth(username, password))
    if response.status_code == 200:
        return response.json()['token']
    else:
        sys.exit(f"Failed to get token, status code: {response.status_code}")

# New function to save the balancing authority maps
def save_ba_maps():
    # Replace 'your_credentials_file.txt' with the actual path to your credentials file
    username = load_credentials('username')
    password = load_credentials('password')
    token = get_token(username, password)
    
    maps_url = 'https://api2.watttime.org/v2/maps'
    headers = {'Authorization': 'Bearer {}'.format(token)}
    
    response = requests.get(maps_url, headers=headers)
    
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

# Run the function
save_ba_maps()

