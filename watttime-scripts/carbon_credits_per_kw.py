import csv
import os
import json
import requests
import sys
from requests.auth import HTTPBasicAuth
from os import path
from statistics import mean
from collections import defaultdict
from wt_api_hist_download import fetch_and_save_historical_data

def prompt_for_coordinates():
    latitude = float(input("Please enter the latitude: "))
    longitude = float(input("Please enter the longitude: "))
    return latitude, longitude

def fetch_nasa_data(latitude, longitude):
    # Construct API endpoint and parameters
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
    # Make request and return parsed response
    response = requests.get(url, params=params)
    return json.loads(response.text)

def load_credentials(filename):
    with open(filename, 'r') as f:
        return f.read().strip()

def get_token(username, password):
    login_url = 'https://api2.watttime.org/v2/login'
    response = requests.get(login_url, auth=HTTPBasicAuth(username, password))
    return response.json()['token']

def get_balancing_authority(token, latitude, longitude):
    # Define the URL and headers for the API request
    region_url = 'https://api2.watttime.org/v2/ba-from-loc'
    headers = {'Authorization': 'Bearer {}'.format(token)}
    params = {'latitude': latitude, 'longitude': longitude}

    # Make the API request
    response = requests.get(region_url, headers=headers, params=params)

    # Check if the API call was successful
    if response.status_code == 200:
        return response.json()['abbrev']
    elif response.status_code == 404:  # Location not supported
        print("Got 404")
        return None
    elif response.status_code == 403:  # Location not supported
        print("Got 403")
        return None
    else:
        print(f"Unexpected error: {response.content}")
        sys.exit("An unexpected error occurred while fetching the balancing authority.")
        
def load_csv_files(ba):
    """
    Load CSV files in a folder with a specific prefix and return the MOER values organized by year and hour.
    
    Args:
        folder_path (str): The path to the folder containing the CSV files.
    
    Returns:
        dict: A nested dictionary containing the MOER values organized by year and hour.
    """
    folder_path = os.path.join("data", ba)
    data = defaultdict(lambda: defaultdict(lambda: defaultdict(list)))
    prefix = f"{ba}_2022"
    for filename in os.listdir(folder_path):
        if filename.startswith(prefix) and filename.endswith('.csv'):
            filepath = os.path.join(folder_path, filename)
            with open(filepath, 'r') as csvfile:
                reader = csv.reader(csvfile)
                next(reader)  # Skip the header
                for row in reader:
                    timestamp, moer = row[0], float(row[1])
                    year, rest = timestamp.split('-', 1)
                    day, time = rest.split('T')
                    hour = time.split(':')[0]
                    data[year][day][hour].append(moer)
    return data
    
def calculate_carbon_credits(nasa_data, moer_data):
    total_kwh = 0
    total_hours = 0
    total_moer = 0

    # Iterate over each day in the NASA data
    for day_data, sun_intensity in nasa_data['properties']['parameter']['ALLSKY_SFC_SW_DWN'].items():
        # Skip if there is no sunlight data
        if sun_intensity is None:
            print("skipping sun intensity due to lack of data")
            continue

        # Extract the hour from the timestamp (last two characters of the 'hour_end' key)
        hour_f = f"{day_data[-2:]}"
        day_f = f"{day_data[4:6]}-{day_data[6:8]}"
        year_f = f"{day_data[:4]}"
        moer_values = moer_data[year_f][day_f][hour_f]

        # Skip if there is no MOER data for the hour
        if not moer_values:
            continue

        # Calculate the average MOER value for the hour
        average_moer = sum(moer_values) / len(moer_values)
        # Convert the MOER value from pounds to metric tons
        average_moer_metric_tons = average_moer / 2204.62
        # Convert the sun intensity from w/m2 to kW/m2 
        sun_intensity_kw = sun_intensity / 1000
        # Calculate the carbon credits for this hour and add it to the total
        total_kwh += sun_intensity_kw

        # Accumulate MOER for average calculation
        total_moer += average_moer_metric_tons * sun_intensity_kw
        total_hours += 1

    # Calculate the average MOER value per megawatt hour
    average_moer_per_mwh = (total_moer / total_kwh)
    avg_hour = total_kwh / total_hours
    total_carbon_credits = average_moer_per_mwh/1000*avg_hour*8766
    
    # Print the results
    print()
    print(f"No Batteries:")
    print(f"Average Sunlight Per Day: {avg_hour*24}")
    print(f"Average Carbon Credits per MWh: {average_moer_per_mwh:.2f}")
    print(f"Total Carbon Credits for 1 kW of Solar Panels: {total_carbon_credits:.2f} metric tons CO2")
    
def calculate_carbon_credits_b(nasa_data, moer_data):
    total_kwh = 0
    total_hours = 0
    total_moer = 0
    count_moer = 0

    # Iterate over the moer data.
    for year in moer_data.items():
        for day in year[1].items():
            for hour in day[1].items():
                avg_hr = sum(hour[1]) / len(hour[1]) if len(hour[1]) else 0
                avg_hr_tons = avg_hr / 2204.62
                total_moer += avg_hr_tons
                count_moer += 1

    for day_data, sun_intensity in nasa_data['properties']['parameter']['ALLSKY_SFC_SW_DWN'].items():
        if sun_intensity is None:
            continue
        sun_intensity_kw = sun_intensity / 1000
        total_kwh += sun_intensity_kw
        total_hours += 1

    # Calculate the average MOER value per megawatt hour
    average_moer_per_mwh = (total_moer / count_moer)
    average_sunlight_per_hour = total_kwh / total_hours
    kwh_per_year = average_sunlight_per_hour * 8766
    total_carbon_credits = average_moer_per_mwh/1000*kwh_per_year
    
    # Print the results
    print()
    print(f"Naive Battery Strategy:")
    print(f"Average Sunlight Per Day: {average_sunlight_per_hour*24}")
    print(f"Average Carbon Credits per MWh: {average_moer_per_mwh:.2f}")
    print(f"Total Carbon Credits for 1 kW of Solar Panels: {total_carbon_credits:.2f} metric tons CO2")

if __name__ == "__main__":
    # Command line: latitude longitude
    if len(sys.argv) < 3:
        print('No coordinates on command line, using Coit Tower (CAISO_NORTH)')
        latitude = 37.803
        longitude = -122.406
    else:
        latitude = sys.argv[1]
        longitude = sys.argv[2]
        
    # Load API credentials
    username = load_credentials('username')
    password = load_credentials('password')

    # Fetch WattTime API token
    token = get_token(username, password)

    # Fetch balancing authority
    ba = get_balancing_authority(token, latitude, longitude)
    
    # Check if the balancing authority is available for the given location
    if ba is None:
        sys.exit("Location not supported")

    # Fetch and save historical data for the balancing authority
    fetch_and_save_historical_data(token, ba)
    
    # Fetch NASA data and calculate average sunlight
    nasa_data = fetch_nasa_data(latitude, longitude)
    moer_data = load_csv_files(ba)
    combo_print = calculate_carbon_credits(nasa_data, moer_data)
    combo_print = calculate_carbon_credits_b(nasa_data, moer_data)
