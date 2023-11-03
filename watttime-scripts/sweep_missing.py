import csv
import os
import time
import threading
from concurrent.futures import ThreadPoolExecutor, as_completed
import combination

# You can adjust the number of workers based on your requirements
NUM_THREADS = 4

def load_existing_points(filename):
    """
    Load all existing points from the CSV file into a dictionary.
    """
    existing_points = {}
    with open(filename, 'r', newline='') as csvfile:
        reader = csv.reader(csvfile)
        next(reader, None)  # Skip the header
        for row in reader:
            if row:  # Ensure the row is not empty
                lat, lon = float(row[0]), float(row[1])
                existing_points[(lat, lon)] = None
    return existing_points

def generate_points_to_scan(filename, offsets):
    """
    Generate points to scan by creating new points around existing points in the CSV file.
    """
    existing_points = load_existing_points(filename)
    points_to_scan = {}
    for (lat, lon) in existing_points.keys():
        for lat_offset in offsets:
            for lon_offset in offsets:
                new_lat = lat + lat_offset
                new_lon = lon + lon_offset
                if new_lat > 40:
                    points_to_scan[(new_lat, new_lon)] = None
    # Deduplicate points
    points_to_scan = {point: None for point in points_to_scan if point not in existing_points}
    return points_to_scan
    
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
    
def process_coordinate(lat, lon, username, password, filename):
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
    
    token = combination.get_token(username, password)
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

def get_token_and_scan_points(points_to_scan, filename):
    """
    Get the API token and scan the points in the dictionary.
    """
    # Load API credentials and get the WattTime token
    username = combination.load_credentials('username')
    password = combination.load_credentials('password')

    # Start thread pool to process coordinates
    with ThreadPoolExecutor(max_workers=NUM_THREADS) as executor:
        future_to_coordinate = {
            executor.submit(process_coordinate, lat, lon, username, password, filename): (lat, lon) 
            for (lat, lon) in points_to_scan
        }

        for future in as_completed(future_to_coordinate):
            lat, lon = future_to_coordinate[future]
            try:
                # Attempt to get the result, which will re-raise any exceptions
                future.result()
            except Exception as exc:
                print(f'Coordinate ({lat}, {lon}) generated an exception: {exc}')

def main():
    """
    Main function to orchestrate generating new points to scan and processing them with threading.
    """
    # Define the offsets to calculate new points
    offsets = [-0.5, -0.25, 0.25, 0.5]

    # Initialize CSV file
    filename = 'carbon_credits_sweep.csv'
    if not os.path.exists(filename):
        with open(filename, 'w', newline='') as csvfile:
            writer = csv.writer(csvfile)
            writer.writerow(["Latitude", "Longitude", "Carbon Credits per Year per KW", "Carbon Credits per Year per KW with Batteries"])
    
    # Generate points to scan
    points_to_scan = generate_points_to_scan(filename, offsets)
    print(f"Number of new, deduplicated points to check: {len(points_to_scan)}")
    print(f"Generated {len(points_to_scan)} new points to scan.")

    # Scan the points
    get_token_and_scan_points(points_to_scan, filename)

if __name__ == "__main__":
    main()

