import os
import json
import requests
import time
from concurrent.futures import ThreadPoolExecutor

# Function to fetch data from NASA API
def fetch_nasa_data(latitude, longitude):
    url = "https://power.larc.nasa.gov/api/temporal/hourly/point"
    params = {
        "parameters": "ALLSKY_SFC_SW_DWN",
        "community": "RE",
        "longitude": longitude,
        "latitude": latitude,
        "start": "20220101",
        "end": "20221231",
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

# Worker function to be run by each thread with retry mechanism
def fetch_and_save(latitude, longitude):
    file_path = f"data/nasa/{latitude}/{longitude}.json"
    # Check if the file already exists
    if os.path.isfile(file_path):
        print(f"Data for latitude {latitude} and longitude {longitude} already exists.")
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

# Main function to spawn threads and save data
def main():
    latitudes = [round(lat, 1) for lat in frange(24, 50, 0.2)]
    longitudes = [round(lon, 1) for lon in frange(-125, -66, 0.2)]
    
    with ThreadPoolExecutor(max_workers=9) as executor:  # Adjust the number of workers as needed
        for latitude in latitudes:
            for longitude in longitudes:
                executor.submit(fetch_and_save, latitude, longitude)

# Helper function to generate a range of floating point numbers
def frange(start, stop, step):
    while start < stop:
        yield start
        start += step

if __name__ == "__main__":
    main()

