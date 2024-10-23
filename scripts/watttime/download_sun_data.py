import os
import json
import requests
import time
from concurrent.futures import ThreadPoolExecutor
from datetime import datetime
import math

META_params = {
    "parameters": "ALLSKY_SFC_SW_DWN",
    "community": "RE",
    "start": "20230101",
    "end": "20231231",
}

# Function to fetch data from NASA API
def fetch_nasa_data(latitude, longitude):
    url = "https://power.larc.nasa.gov/api/temporal/hourly/point"
    params = {
        "parameters": META_params["parameters"],
        "community": META_params["community"],
        "longitude": longitude,
        "latitude": latitude,
        "start": META_params["start"],
        "end": META_params["end"],
        "format": "json"
    }
    response = requests.get(url, params=params)

    # Check if the response was successful
    if response.status_code == 200:
        try:
            data = response.json()
            # Further checks can be added to validate response contents
            return data
        except json.JSONDecodeError:
            raise ValueError("Invalid JSON received")
    else:
        raise Exception(f"Error fetching data: {response.status_code}, {response.text}")

# Function to save data to disk
def save_to_disk(data, latitude, longitude):
    folder_path = f"data/nasa/{latitude}"
    if not os.path.exists(folder_path):
        os.makedirs(folder_path)
    file_path = os.path.join(folder_path, f"{longitude}.json")
    with open(file_path, 'w') as f:
        json.dump(data, f)

# Helper function to extract parameter data
def get_parameter_data(data):
    try:
        return data['properties']['parameter']
    except KeyError:
        return None

# Helper function to compare parameter data
def parameter_data_equal(p1, p2):
    return json.dumps(p1, sort_keys=True) == json.dumps(p2, sort_keys=True)

# Function to try to derive data from neighbors
def try_to_derive_data(latitude, longitude, step_size):
    # Find the 4 cardinal neighbors
    neighbors = []
    offsets = [(-step_size, 0), (step_size, 0), (0, -step_size), (0, step_size)]  # N, S, W, E
    for dy, dx in offsets:
        neighbor_lat = round(latitude + dy, 6)
        neighbor_lon = round(longitude + dx, 6)
        neighbor_file = f"data/nasa/{neighbor_lat}/{neighbor_lon}.json"
        if os.path.isfile(neighbor_file):
            neighbors.append(neighbor_file)
        else:
            # Neighbor data does not exist
            return False
    # If all 4 neighbors exist, check if their data is the same
    neighbor_params = []
    for neighbor_file in neighbors:
        with open(neighbor_file, 'r') as f:
            data = json.load(f)
            params = get_parameter_data(data)
            if params is None:
                return False  # Cannot extract parameter data
            neighbor_params.append(params)
    # Check if all neighbor parameter data are the same
    first_param = neighbor_params[0]
    if all(parameter_data_equal(first_param, nd) for nd in neighbor_params[1:]):
        # Data is the same, save it
        # Copy the data from one of the neighbors
        with open(neighbors[0], 'r') as f:
            data_to_save = json.load(f)
        save_to_disk(data_to_save, latitude, longitude)
        return True
    else:
        # Data is not the same
        return False

# Worker function to be run by each thread with retry mechanism
def fetch_and_save(latitude, longitude, step_size):
    file_path = f"data/nasa/{latitude}/{longitude}.json"
    # Check if the file already exists
    if os.path.isfile(file_path):
        print(f"Data for latitude {latitude} and longitude {longitude} already exists.")
        return

    # Try to derive data from neighbors
    derived = try_to_derive_data(latitude, longitude, step_size)
    if derived:
        print(f"Derived data for latitude {latitude} and longitude {longitude} from neighbors.")
        return

    max_retries = 30  # Set the number of retries
    attempt = 0

    while attempt < max_retries:
        try:
            print(f"Fetching data for latitude {latitude} and longitude {longitude}")
            data = fetch_nasa_data(latitude, longitude)
            save_to_disk(data, latitude, longitude)
            print(f"Data saved for latitude {latitude} and longitude {longitude}")
            break  # If save was successful, break out of the loop
        except Exception as e:
            print(f"Attempt {attempt + 1} failed: Error fetching or saving data for latitude {latitude} and longitude {longitude}: {e}")
            attempt += 1
            if attempt < max_retries:
                print(f"Retrying after 15 seconds...")
                time.sleep(15)
            else:
                print(f"Max retries reached for latitude {latitude} and longitude {longitude}. Giving up.")

# Helper function to generate grid points
def generate_grid(start, stop, step):
    num_steps = int(round((stop - start) / step))
    points = []
    for i in range(num_steps + 1):
        value = start + i * step
        if value <= stop:
            points.append(round(value, 6))
        else:
            break
    return points

# Main function to spawn threads and save data
def main():
    initial_step_size = 0.2 * 16  # 0.2 * 16 = 3.2
    num_passes = int(math.log2(16)) + 1  # 5 passes
    start_lat, stop_lat = 24, 50
    start_lon, stop_lon = -125, -66

    with ThreadPoolExecutor(max_workers=9) as executor:
        for pass_num in range(num_passes):
            step_size = initial_step_size / (2 ** pass_num)
            print(f"Pass {pass_num + 1}/{num_passes}: step size {step_size}")
            latitudes = generate_grid(start_lat, stop_lat, step_size)
            longitudes = generate_grid(start_lon, stop_lon, step_size)
            tasks = []
            for latitude in latitudes:
                for longitude in longitudes:
                    executor.submit(fetch_and_save, latitude, longitude, step_size)

if __name__ == "__main__":
    # Downloads solar energy data from NASA Power. Output to data/nasa.
    # Does not depend on any other script.
    print(f"Downloading to data/nasa start {META_params['start']} end {META_params['end']}")

    metadata = {
        "generation_time": datetime.utcnow().strftime("%Y-%m-%dT%H:%M:%SZ"),
        "params": META_params,
    }

    path = "data/nasa"
    if not os.path.exists(path):
        os.makedirs(path)
    path = os.path.join(path, "nasa_meta.json")
    if not os.path.isfile(path):
        with open(path, 'w') as f:
            json.dump(metadata, f, indent=2)

    main()
