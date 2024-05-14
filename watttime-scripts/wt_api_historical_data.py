# This is a request to get recent historical data for a location.
# This request only provides 32 days of data. Use historical.py to
# get more data than that.
import requests
import json
import sys
from requests.auth import HTTPBasicAuth

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

# Load historical data (max 32 days)
# /v2/data call transitioned to /v3/historical
# returns json or None
def get_historical_data(token, region, start, end):
    data_url = 'https://api.watttime.org/v3/historical'
    headers = {'Authorization': f'Bearer {token}'}
    params = {'region': region, 
            'start': start, 
            'end': end,
            'signal_type': 'co2_moer'}
    rsp = requests.get(data_url, headers=headers, params=params)
    if rsp.status_code == 200:
        return rsp.json()
    else:
        print(rsp.text)
        return None

if __name__ == "__main__":
    # Command line: latitude longitude
    if len(sys.argv) < 2:
        print('No region found on command line using CAISO_NORTH')
        region = "CAISO_NORTH"
    else:
        region = sys.argv[1]

    if len(sys.argv) < 3:
        print('No start and end dates on command line, using Aug 2023')
        start = "2023-08-01T00:00:00Z"
        end = "2023-08-31T23:59:59Z"
    else:
        start = sys.argv[2]
        end = sys.argv[3]

    print(f"region {region}")
    print(f"start {start}")
    print(f"end {end}")

    # Load username and password
    username = load_credentials('username')
    password = load_credentials('password')
    
    # Get the token
    token = get_token(username, password)

    j = get_historical_data(token, region, start, end)
    if j is not None:
        print(json.dumps(j, indent=2))
