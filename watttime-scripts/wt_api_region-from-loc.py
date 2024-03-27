import requests
from requests.auth import HTTPBasicAuth
import sys
import json

# Function to fetch region information based on coordinates
def get_region_info(token, latitude, longitude):
    """
    Fetch the region information based on latitude and longitude.

    Parameters:
        token (str): The authorization token for the API.
        latitude (str): The latitude coordinate.
        longitude (str): The longitude coordinate.

    Returns:
        json: Response from WattTime (or None if API call failed)
    """
    region_url = 'https://api.watttime.org/v3/region-from-loc'
    headers = {'Authorization': f'Bearer {token}'}
    params = {
        'latitude': latitude,
        'longitude': longitude,
        'signal_type': 'co2_moer'
    }
    rsp = requests.get(region_url, headers=headers, params=params)
    if rsp.status_code == 200:
        return rsp.json()
    else:
        return None    

if __name__ == "__main__":
    # Assumes 'token' file exists. If it does not or token is expired,
    # call the wt_api_login.py script again.
    
    with open('token', 'r') as f:
        token = f.read().strip()
    
    if len(sys.argv) < 3:
        print('No coordinates on command line, using Apple HQ location (CAISO_NORTH)')
        latitude = 37.335
        longitude = -122.009
    else:
        latitude = sys.argv[1]
        longitude = sys.argv[2]

    print(f'Region from latitude: {latitude} longitude: {longitude} using V3 API')

    # Fetch and print region information
    res = get_region_info(token, latitude, longitude)

    if res is not None:
        print(json.dumps(res, indent=2))
    else:
        print ('API failed, you probably need to log in again.')

