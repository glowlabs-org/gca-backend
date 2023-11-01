import csv
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
    with open(filename, 'w', newline='') as csvfile:
        writer = csv.writer(csvfile)
        writer.writerow(["Latitude", "Longitude", "Carbon Credits per Year per KW", "Carbon Credits per Year per KW with Batteries"])

    # Load API credentials and get the WattTime token
    username = combination.load_credentials('username')
    password = combination.load_credentials('password')
    token = combination.get_token(username, password)
    
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
                print(f"An error occurred for Latitude: {lat}, Longitude: {lon}. Error message: {e}")
            
            lon += granularity
        lat += granularity

if __name__ == "__main__":
    main()

