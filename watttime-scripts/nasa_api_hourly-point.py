# This is a request to get recent historical data for a location.
# This request only provides 32 days of data.
import requests
import json
import sys
from datetime import datetime, timedelta

def nasa_hourly(latitude, longitude, start, end):
    """
    Fetch hourly NASA data for a given latitude and longitude.
    
    Parameters:
        latitude (float): The latitude coordinate.
        longitude (float): The longitude coordinate.
        start (str): Start day in YYYYMMDD format.
        end (str): End day in YYYYMMDD format.
        
    Returns:
        json: Response from the NASA API (or None)
    """
    # Define the API endpoint and parameters
    url = "https://power.larc.nasa.gov/api/temporal/hourly/point"
    params = {
        "parameters": "ALLSKY_SFC_SW_DWN",
        "community": "RE",
        "longitude": longitude,
        "latitude": latitude,
        "start": start,
        "end": end,
        "format": "json"
    }
    
    # Perform the API request and parse the JSON response
    rsp = requests.get(url, params=params)
    
    if rsp.status_code == 200:
        return rsp.json()
    else:
        print(rsp.status_code, rsp.text)
        return None

if __name__ == "__main__":
    if len(sys.argv) < 3:
        print('No coordinates on command line, using Apple HQ location')
        latitude = 37.335
        longitude = -122.009
    else:
        latitude = sys.argv[1]
        longitude = sys.argv[2]

    if len(sys.argv) < 5:
        print('No start and end time on command line, using one year ago')
        ts1 = datetime.utcnow() - timedelta(days=365)
        ts2 = ts1
        start = ts1.strftime("%Y%m%d")
        end = ts2.strftime("%Y%m%d")
    else:
        start = sys.argv[3]
        end = sys.argv[4]

    print(f'NASA Power hourly latitude: {latitude} longitude: {longitude} start: {start} end: {end}')

    res = nasa_hourly(latitude, longitude, start, end)
    if res is not None:
        print(json.dumps(res, indent=2))
    else:
        print ('API failed.')
