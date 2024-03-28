# index.py gets the actual carbon data for an area
import requests
from requests.auth import HTTPBasicAuth
import sys
import json

def current_moer_index(token, region):
    """
    Fetch the current moer index (1-100 percentile)

    Parameters:
        token (str): The authorization token for the API.
        region (str): Region, e.g CAISO_NORTH, FPL.

    Returns:
        json: Response from WattTime (or None if API call failed)
    """    
    index_url = 'https://api.watttime.org/v3/signal-index'
    headers = {'Authorization': 'Bearer {}'.format(token)}
    params = {
        'region': region,
        'signal_type': 'co2_moer'
    }
    rsp = requests.get(index_url, headers=headers, params=params)
    if rsp.status_code == 200:
        return rsp.json()
    else:
        return None

if __name__ == "__main__":
    # Assumes 'token' file exists. If it does not or token is expired,
    # call the wt_api_login.py script again.
    
    with open('token', 'r') as f:
        token = f.read().strip()

    if len(sys.argv) < 2:
        print('No region on command line, using CAISO_NORTH')
        region = 'CAISO_NORTH'
    else:
        region = sys.argv[1]

    print(f'Current moer index for region: {region} using V3 API')

    res = current_moer_index(token, region)
    if res is not None:
        print(json.dumps(res, indent=2))
    else:
        print ('API failed, you probably need to log in again.')
