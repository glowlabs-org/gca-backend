# This is a request to get recent historical data for a location.
# This request only provides 32 days of data.
import requests
import json
import sys
from requests.auth import HTTPBasicAuth
from datetime import datetime, timedelta

def get_historical_data(token, region, tsstart, tsend):
    """
    Fetch historical .

    Parameters:
        token (str): The authorization token for the API.
        region (str): Region, e.g CAISO_NORTH, FPL.
        start (str): Start time in ISO8601 format
        end (str): End time in ISO8601 format

    Returns:
        json: Response from WattTime (or None if API call failed)
    """
    url = 'https://api.watttime.org/v3/historical'
    headers = {'Authorization': f'Bearer {token}'}
    params = {
        'start': start,
        'end': end,
        'region': region,
        'signal_type': 'co2_moer'
    }
    rsp = requests.get(url, headers=headers, params=params)
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

    if len(sys.argv) < 4:
        print('No timestamps on command line, using last hour')
        ts2 = datetime.utcnow()
        ts1 = ts2 - timedelta(hours=1)
        start = ts1.strftime("%Y-%m-%dT%H:%M:%SZ")
        end = ts2.strftime("%Y-%m-%dT%H:%M:%SZ")
    else:
        start = sys.argv[2]
        end = sys.argv[3]

    print(f'Historical data for region: {region} start: {start} end: {end} using V3 API')

    res = get_historical_data(token, region, start, end)
    if res is not None:
        print(json.dumps(res, indent=2))
    else:
        print ('API failed, you probably need to log in again.')
