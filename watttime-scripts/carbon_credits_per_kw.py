import csv
import os
import json
import zipfile
import requests
import sys
from requests.auth import HTTPBasicAuth
from os import path
from statistics import mean
from collections import defaultdict

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
    if path.exists(data_path):
        print(f"Data for {ba} already exists locally.")
    else:
        # Construct historical data URL and headers
        historical_url = 'https://api2.watttime.org/v2/historical'
        headers = {'Authorization': f'Bearer {token}'}
        params = {'ba': ba}
        
        # Fetch historical data
        rsp = requests.get(historical_url, headers=headers, params=params)
        
        # Create a directory for the balancing authority
        if not os.path.exists(data_path):
            os.mkdir(data_path)
        
        # Save the zip file
        zip_path = path.join("data", ba, f'{ba}_historical.zip')
        with open(zip_path, 'wb') as fp:
            fp.write(rsp.content)
        
        # Extract the zip file
        with zipfile.ZipFile(zip_path, 'r') as zip_ref:
            zip_ref.extractall(ba)
        
        print(f"Wrote and unzipped historical data for {ba} to the directory: {ba}")
        
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
    count_moer = 0

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
            # print("skipping due to lack of moer data")
            continue

        # Calculate the average MOER value for the hour
        average_moer = sum(moer_values) / len(moer_values)
        # Convert the MOER value from pounds to metric tons
        average_moer_metric_tons = average_moer / 2204.62
        # Convert the sun intensity from w/m2 to kW/m2 
        sun_intensity_kw = sun_intensity / 1000

        # Calculate the carbon credits for this hour and add it to the total
        total_kwh += sun_intensity_kw
        #print(sun_intensity_kw, average_moer_metric_tons, carbon_credits, total_carbon_credits)

        # Accumulate MOER for average calculation
        total_moer += average_moer_metric_tons
        count_moer += 1
        total_hours += 1

    # Calculate the average MOER value per megawatt hour
    average_moer_per_mwh = (total_moer / count_moer) if count_moer else 0
    total_carbon_credits = average_moer_per_mwh/1000*total_kwh
    
    # Print the results
    print()
    print(f"Average Sunlight Per Day: {total_kwh/total_hours*24}")
    print(f"Average Carbon Credits per MWh: {average_moer_per_mwh:.2f}")
    print(f"Total Carbon Credits for 1 kW of Solar Panels: {total_carbon_credits:.2f} metric tons CO2")
    return total_carbon_credits

if __name__ == "__main__":
    # Load API credentials
    username = load_credentials('username')
    password = load_credentials('password')

    # Fetch WattTime API token
    token = get_token(username, password)

    # Get latitude and longitude from the user
    latitude, longitude = prompt_for_coordinates()

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

