import csv
import os
import combination

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

def main():
    """
    Main function to perform a sweep over the continental US to calculate the expected carbon credits
    at each point and save them to a CSV file.
    
    Returns:
        None
    """
    # Define the continental US boundaries and granularity
    lat_min, lat_max = 24.396308, 49.384358
    lon_min, lon_max = -125.000000, -66.934570
    granularity = 0.5
    
    # Initialize CSV file
    filename = 'carbon_credits_sweep.csv'
    # Only write the header if the file doesn't already exist
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
        lat_min, lon_min = last_coordinates
    
    # Loop through the grid
    lat = lat_min
    while lat <= lat_max:
        lon = lon_min
        while lon <= lon_max:
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
                # Check for 403 Forbidden error
                if '403 Forbidden' in str(e):
                    print("Received a 403 Forbidden error. Skipping this coordinate.")
                else:
                    print(f"An unexpected error occurred for Latitude: {lat}, Longitude: {lon}. Error message: {e}")
            
            lon += granularity
        lat += granularity

if __name__ == "__main__":
    main()

