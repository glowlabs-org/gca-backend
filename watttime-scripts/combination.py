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
    """
    Prompt the user for latitude and longitude coordinates.

    Returns:
        tuple: (latitude, longitude)
    """
    latitude = float(input("Please enter the latitude: "))
    longitude = float(input("Please enter the longitude: "))
    return latitude, longitude

def fetch_nasa_data(latitude, longitude):
    """
    Fetch data from NASA's POWER API.

    Parameters:
        latitude (float): Latitude coordinate.
        longitude (float): Longitude coordinate.

    Returns:
        dict: NASA POWER API response as a dictionary.
    """
    # Construct API endpoint and parameters
    url = "https://power.larc.nasa.gov/api/temporal/daily/point"
    params = {
        "parameters": "ALLSKY_SFC_SW_DWN",
        "community": "RE",
        "longitude": longitude,
        "latitude": latitude,
        "start": "20200101",
        "end": "20221231",
        "format": "json"
    }
    # Make request and return parsed response
    response = requests.get(url, params=params)
    return json.loads(response.text)

def calculate_average_sunlight(data):
    """
    Calculate average annual sunlight based on NASA POWER API data.

    Parameters:
        data (dict): NASA POWER API response as a dictionary.

    Returns:
        float: Average annual sunlight in kW-hr/m^2/day.
    """
    # Extract relevant data and calculate average, ignoring invalid values
    sunlight_data = data["properties"]["parameter"]["ALLSKY_SFC_SW_DWN"].values()
    filtered_data = [x for x in sunlight_data if x != -999.0]
    return sum(filtered_data) / len(filtered_data) if filtered_data else 0

def load_credentials(filename):
    """
    Load API credentials from a file.

    Parameters:
        filename (str): File containing API credentials.

    Returns:
        str: API credentials as a string.
    """
    with open(filename, 'r') as f:
        return f.read().strip()

def get_token(username, password):
    """
    Fetch an authorization token from the WattTime API.

    Parameters:
        username (str): WattTime API username.
        password (str): WattTime API password.

    Returns:
        str: WattTime API authorization token.
    """
    login_url = 'https://api2.watttime.org/v2/login'
    response = requests.get(login_url, auth=HTTPBasicAuth(username, password))
    return response.json()['token']

def get_balancing_authority(token, latitude, longitude):
    """
    Fetch the balancing authority based on latitude and longitude.

    Parameters:
        token (str): WattTime API authorization token.
        latitude (float): Latitude coordinate.
        longitude (float): Longitude coordinate.

    Returns:
        str: Abbreviation of the balancing authority or None if location not supported.
    """
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
        return None
    else:
        print(f"Unexpected error: {response.content}")
        sys.exit("An unexpected error occurred while fetching the balancing authority.")

def update_gitignore(folder_name):
    """
    Update the .gitignore file with the name of a new folder.

    Parameters:
        folder_name (str): Name of the new folder to add to .gitignore.

    Returns:
        None
    """
    # Open the .gitignore file for appending
    # If the file doesn't exist, it will be created
    with open('.gitignore', 'a+') as f:
        # Ensure the folder name is not already in the file
        f.seek(0)
        if folder_name not in f.read().splitlines():
            # Append the folder name to .gitignore file
            f.write(f"\n{folder_name}")
            print(f"Added {folder_name} to .gitignore")

def fetch_and_save_historical_data(token, ba):
    """
    Fetch and save historical data for a given balancing authority.
    
    Parameters:
        token (str): WattTime API authorization token.
        ba (str): Abbreviation of the balancing authority.

    Returns:
        None
    """
    if path.exists(ba):
        print(f"Data for {ba} already exists locally.")
    else:
        # Construct historical data URL and headers
        historical_url = 'https://api2.watttime.org/v2/historical'
        headers = {'Authorization': f'Bearer {token}'}
        params = {'ba': ba}
        
        # Fetch historical data
        rsp = requests.get(historical_url, headers=headers, params=params)
        
        # Create a directory for the balancing authority
        if not os.path.exists(ba):
            os.mkdir(ba)
            update_gitignore(ba)
        
        # Save the zip file
        zip_path = path.join(ba, f'{ba}_historical.zip')
        with open(zip_path, 'wb') as fp:
            fp.write(rsp.content)
        
        # Extract the zip file
        with zipfile.ZipFile(zip_path, 'r') as zip_ref:
            zip_ref.extractall(ba)
        
        print(f"Wrote and unzipped historical data for {ba} to the directory: {ba}")
        
def load_csv_files(folder_path):
    """
    Load all CSV files in a folder and return the MOER values organized by year and day.
    
    Args:
        folder_path (str): The path to the folder containing the CSV files.
    
    Returns:
        dict: A nested dictionary containing the MOER values organized by year and day.
    """
    data = defaultdict(lambda: defaultdict(list))
    for filename in os.listdir(folder_path):
        if filename.endswith('.csv'):
            filepath = os.path.join(folder_path, filename)
            with open(filepath, 'r') as csvfile:
                reader = csv.reader(csvfile)
                next(reader)
                for row in reader:
                    timestamp, moer = row[0], float(row[1])
                    year, _ = timestamp.split('-', 1)
                    day = timestamp.split('T')[0]
                    data[year][day].append(moer)
    return data

def calculate_carbon_credits(folder_path, avg_sunlight):
    """
    Calculate the expected carbon credits for the year 2022 based on the average sunlight hours.
    
    Args:
        folder_path (str): The path to the folder containing the CSV files.
        avg_sunlight (float): The average annual sunlight in hours.
    
    Returns:
        tuple: A tuple containing carbon credits per year, per kilowatt for solar only and solar+smart batteries.
    """
    data = load_csv_files(folder_path)
    
    if '2022' not in data:
        print("No data for 2022 is available.")
        return None
    
    daily_low_avg = []
    daily_high_avg = []
    
    for day, moer_values in data['2022'].items():
        moer_values.sort()
        daily_low_avg.append(mean(moer_values[:30]))
        daily_high_avg.append(mean(moer_values[-120:]))
    
    yearly_low_avg = mean(daily_low_avg)
    yearly_high_avg = mean(daily_high_avg)
    
    conversion_factor = avg_sunlight  # hours of sunlight per day
    conversion_factor *= 365.25  # days per year
    conversion_factor /= 1000  # kilowatt-hours per megawatt-hour
    conversion_factor /= 2204.62  # pounds per metric ton
    
    low_avg = yearly_low_avg * conversion_factor
    high_avg = yearly_high_avg * conversion_factor
    
    return low_avg, high_avg

if __name__ == "__main__":
    # Load API credentials
    username = load_credentials('username')
    password = load_credentials('password')

    # Fetch WattTime API token
    token = get_token(username, password)

    # Get latitude and longitude from the user
    latitude, longitude = prompt_for_coordinates()
    
    # Fetch NASA data and calculate average sunlight
    nasa_data = fetch_nasa_data(latitude, longitude)
    avg_sunlight = calculate_average_sunlight(nasa_data)

    # Fetch balancing authority
    ba = get_balancing_authority(token, latitude, longitude)
    
    # Check if the balancing authority is available for the given location
    if ba is None:
        sys.exit("Location not supported")

    # Fetch and save historical data for the balancing authority
    fetch_and_save_historical_data(token, ba)
    
    # Compute carbon credit averages using the historical data for the ba
    low_avg, high_avg = calculate_carbon_credits(ba, avg_sunlight)
    
    print(f"\nExpected Carbon Credits for 2022 in {ba} with avg sunlight of {avg_sunlight} hours:")
    print(f"Solar Only:              {low_avg:.3f} carbon credits per year, per kilowatt of solar")
    print(f"Solar + Smart Batteries: {high_avg:.3f} carbon credits per year, per kilowatt of solar")

