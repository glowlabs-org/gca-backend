import os
import sys
import json
import threading
import time
import math
import requests
from requests.auth import HTTPBasicAuth

# Constants
TOTAL_PASSES = 5  # Adjust this constant for the number of passes
DATA_FILE = 'data/ba-grid.jsonl'
USERNAME_FILE = 'username'
PASSWORD_FILE = 'password'
THREAD_COUNT = 8
API_RATE_LIMIT = 1  # seconds between API calls per thread

# Function to load credentials from a file
def load_credentials(filename):
    with open(filename, 'r') as f:
        return f.read().strip()

# Function to get token for WattTime API
def get_token(username, password):
    login_url = 'https://api.watttime.org/login'
    response = requests.get(login_url, auth=HTTPBasicAuth(username, password))
    return response.json()['token']

# Function to fetch region information based on coordinates
def get_region_info(token, latitude, longitude):
    region_url = 'https://api.watttime.org/v3/region-from-loc'
    headers = {'Authorization': f'Bearer {token}'}
    params = {'latitude': latitude, 'longitude': longitude, 'signal_type': 'co2_moer'}
    response = requests.get(region_url, headers=headers, params=params)
    return response.json()

# Thread worker function
def worker(thread_id, token, points_queue, data_lock, file_lock, existing_points):
    while True:
        with data_lock:
            if not points_queue:
                break
            point = points_queue.pop()
        latitude, longitude = point
        key = f"{latitude}_{longitude}"
        if key in existing_points:
            continue
        try:
            region_info = get_region_info(token, latitude, longitude)
            ba = region_info.get('abbrev', 'UNKNOWN')
            data = {'latitude': latitude, 'longitude': longitude, 'ba': ba}
            # Append to file
            with file_lock:
                with open(DATA_FILE, 'a') as f:
                    f.write(json.dumps(data) + '\n')
                existing_points.add(key)
        except Exception as e:
            print(f"Error fetching data for {latitude}, {longitude}: {e}")
        time.sleep(API_RATE_LIMIT)

# Main script execution
def main():
    # Ensure data directory exists
    os.makedirs(os.path.dirname(DATA_FILE), exist_ok=True)

    # Load existing data
    existing_points = set()
    if os.path.exists(DATA_FILE):
        with open(DATA_FILE, 'r') as f:
            for line in f:
                data = json.loads(line.strip())
                key = f"{data['latitude']}_{data['longitude']}"
                existing_points.add(key)

    # Load username and password
    username = load_credentials(USERNAME_FILE)
    password = load_credentials(PASSWORD_FILE)

    # Get the token
    token = get_token(username, password)

    data_lock = threading.Lock()
    file_lock = threading.Lock()

    for pass_num in range(TOTAL_PASSES):
        grid_size = 10 * (2 ** pass_num)
        lat_step = 180 / grid_size
        lon_step = 360 / grid_size
        latitudes = [ -90 + i * lat_step for i in range(grid_size + 1) ]
        longitudes = [ -180 + i * lon_step for i in range(grid_size + 1) ]

        points_queue = []

        # Assemble list of points for this pass
        for lat in latitudes:
            for lon in longitudes:
                # Round coordinates to avoid floating point issues
                lat_rounded = round(lat, 6)
                lon_rounded = round(lon, 6)
                key = f"{lat_rounded}_{lon_rounded}"
                if key not in existing_points:
                    points_queue.append((lat_rounded, lon_rounded))

        total_points = len(points_queue)
        print(f"Pass {pass_num + 1}/{TOTAL_PASSES}: {total_points} new points to process.")

        # Start threads
        threads = []
        for i in range(THREAD_COUNT):
            thread = threading.Thread(target=worker, args=(i, token, points_queue, data_lock, file_lock, existing_points))
            thread.start()
            threads.append(thread)

        # Wait for all threads to finish
        for thread in threads:
            thread.join()

        print(f"Pass {pass_num + 1} completed.")

if __name__ == "__main__":
    main()
