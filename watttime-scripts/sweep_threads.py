import csv
import os
import combination
import time
from concurrent.futures import ThreadPoolExecutor, as_completed

# You can adjust the number of workers based on your requirements
NUM_THREADS = 12

def save_to_csv(row, filename):
    """
    Append a row of data to a CSV file.
    
    Parameters:
        row (list): Data row to append.
        filename (str): Name of the CSV file.
        
    Returns:
        None
    """
    with open(filename, 'a', newline='') as csvfile:
        writer = csv.writer(csvfile)
        writer.writerow(row)

def get_last_coordinates(filename):
    """
    Get the last latitude and longitude from the CSV file.
    
    Parameters:
        filename (str): Name of the CSV file.
    
    Returns:
        tuple: Last latitude and longitude in the CSV file, or None if the file is empty or not found.
    """
    try:
        with open(filename, 'r', newline='') as csvfile:
            reader = csv.reader(csvfile)
            # Skip the header row
            next(reader, None)
            last_row = None
            for row in reader:
                last_row = row
            if last_row:
                return float(last_row[0]), float(last_row[1])
    except FileNotFoundError:
        # If the file is not found, it's likely the first run.
        return None
    return None

def process_coordinate(lat, lon, token, filename):
    """
    Process a single coordinate point.

    Parameters:
        lat (float): Latitude of the point.
        lon (float): Longitude of the point.
        token (str): API token for data access.
        filename (str): CSV file to save the data.

    Returns:
        None
    """
    print(f"Attempting for Latitude: {lat}, Longitude: {lon}")  # Print current lat and long being attempted
    
    try:
        # Fetch NASA data and calculate average sunlight
        nasa_data = combination.fetch_nasa_data(lat, lon)
        avg_sunlight = combination.calculate_average_sunlight(nasa_data)
        
        # Fetch balancing authority
        ba = combination.get_balancing_authority(token, lat, lon)
        
        # Only proceed if the location is supported
        if ba is not None:
            # Fetch and save historical data for the balancing authority
            combination.fetch_and_save_historical_data(token, ba)
            
            # Calculate carbon credits
            low_avg, high_avg = combination.calculate_carbon_credits(ba, avg_sunlight)
            
            # Save to CSV
            save_to_csv([lat, lon, low_avg, high_avg], filename)
            
            print(f"Saved data for Latitude: {lat}, Longitude: {lon}")
        else:
            print(f"Skipping unsupported location at Latitude: {lat}, Longitude: {lon}")
        
    except Exception as e:
        error_message = str(e)
        # Check for 403 Forbidden error
        if '403 Forbidden' in error_message:
            print(f"Received a 403 Forbidden error. Invalid coordinate at Latitude: {lat}, Longitude: {lon}. Skipping this coordinate.")
        else:
            print(f"An unexpected error occurred for Latitude: {lat}, Longitude: {lon}. Error message: {error_message}")

def generate_coordinates(lat_min, lat_max, lon_min, lon_max, granularity):
    """
    Generate a list of coordinates based on defined boundaries and granularity.

    Parameters:
        lat_min (float): Minimum latitude.
        lat_max (float): Maximum latitude.
        lon_min (float): Minimum longitude.
        lon_max (float): Maximum longitude.
        granularity (float): Increment for latitude and longitude.

    Yields:
        tuple: Each latitude and longitude combination within the specified range.
    """
    lat = lat_min
    while lat <= lat_max:
        lon = lon_min
        while lon <= lon_max:
            yield lat, lon
            lon += granularity
        lat += granularity

def main():
    """
    Main function to perform a multi-threaded sweep over the continental US to calculate the expected
    carbon credits at each point and save them to a CSV file.
    
    Returns:
        None
    """
    # Define the continental US boundaries and granularity
    lat_min, lat_max = 24.396308, 49.384358
    lon_min, lon_max = -125.000000, -66.934570
    granularity = 0.5
    
    # Initialize CSV file
    filename = 'carbon_credits_sweep.csv'
    if not os.path.exists(filename):
        with open(filename, 'w', newline='') as csvfile:
            writer = csv.writer(csvfile)
            writer.writerow(["Latitude", "Longitude", "Carbon Credits per Year per KW", "Carbon Credits per Year per KW with Batteries"])
    
    # Load API credentials and get the WattTime token
    username = combination.load_credentials('username')
    password = combination.load_credentials('password')
    token = combination.get_token(username, password)
    
    # Get last coordinates from the CSV to resume from
    last_coordinates = get_last_coordinates(filename)
    if last_coordinates:
        lat_min, _ = last_coordinates

    # Generate all coordinates to be processed
    coordinates = list(generate_coordinates(lat_min, lat_max, lon_min, lon_max, granularity))

    # Start thread pool
    with ThreadPoolExecutor(max_workers=NUM_THREADS) as executor:
        # Schedule the processing of each coordinate using the executor
        # Each thread will handle the processing of a single coordinate
        future_to_coordinate = {executor.submit(process_coordinate, lat, lon, token, filename): (lat, lon) for lat, lon in coordinates}
        
        # Iterate over the completed futures
        for future in as_completed(future_to_coordinate):
            lat, lon = future_to_coordinate[future]
            try:
                # Attempt to get the result, which will also re-raise any exceptions caught during processing
                future.result()
            except Exception as exc:
                print(f'Coordinate ({lat}, {lon}) generated an exception: {exc}')

if __name__ == "__main__":
    main()

