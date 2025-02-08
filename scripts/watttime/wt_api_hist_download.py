import requests
from requests.auth import HTTPBasicAuth
import sys
import json
from os import path
import os
import calendar
from wt_api_historical_data import load_credentials, get_token, get_historical_data

def fetch_and_save_historical_data(token, ba):
    """
    Fetch and save historical data for a given balancing authority.
    
    Parameters:
        token (str): WattTime API authorization token.
        ba (str): Abbreviation of the balancing authority.

    Returns:
        None
    """

    data_path = path.join("data", ba)
    if not os.path.exists(data_path):
        os.makedirs(data_path)

    for month in range(1, 13):
        _, last = calendar.monthrange(2024, month)
        # CAISO_NORTH_2022-05_MOER.json
        fname = f"{ba}_2024-{month:02}_MOER.json"
        # "2024-08-01T00:00:00Z"
        start = f"2024-{month:02}-01T00:00:00Z"
        end = f"2024-{month:02}-{last:02}T23:59:59Z"
        file_path = path.join(data_path, fname)
        if path.exists(file_path):
            pass
        else:
            print(f"generating {file_path}")
            dat = get_historical_data(token, ba, start, end)
            if dat is not None:
                with open(file_path, "w") as f:
                    json.dump(dat, f)
            else:
                sys.exit(f"error downloading data")
            

    print(f"Generated 2024 historical data for {ba} to the directory: {data_path}")

if __name__ == "__main__":
    # Command line: latitude longitude
    if len(sys.argv) < 2:
        print('No region on command line, using CAISO_NORTH')
        ba = "CAISO_NORTH"
    else:
        ba = sys.argv[1]

    # Load username and password
    username = load_credentials('username')
    password = load_credentials('password')
    
    # Get the token
    token = get_token(username, password)
    
    # Fetch and print region information
    print(f"region {ba}")

    fetch_and_save_historical_data(token, ba)
