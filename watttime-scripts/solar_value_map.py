import geopandas as gpd
import os
import csv
import json
from collections import defaultdict
from shapely.geometry import Point

# Function to load all BA history into memory
def load_all_ba_history():
    ba_histories = {}
    base_folder_path = os.path.join("data")
    for ba_folder in os.listdir(base_folder_path):
        ba_folder_path = os.path.join(base_folder_path, ba_folder)
        if os.path.isdir(ba_folder_path):
            ba_histories[ba_folder] = defaultdict(lambda: defaultdict(lambda: defaultdict(list)))
            for filename in os.listdir(ba_folder_path):
                if filename.endswith('.csv'):
                    filepath = os.path.join(ba_folder_path, filename)
                    with open(filepath, 'r') as csvfile:
                        reader = csv.reader(csvfile)
                        next(reader)  # Skip the header
                        for row in reader:
                            timestamp, moer = row[0], float(row[1])
                            year, rest = timestamp.split('-', 1)
                            day, time = rest.split('T')
                            hour = time.split(':')[0]
                            ba_histories[ba_folder][year][day][hour].append(moer)
    return ba_histories

# determines what BA is responsible for a specific coordinate.
# Will return 'None' if there is no BA data for that coordinate.
def get_ba_by_coords(gdf, latitude, longitude):
    # Create a Shapely Point from the latitude and longitude
    point = Point(longitude, latitude)  # Note: Point takes (longitude, latitude)

    # Use the .contains method of the geometry to check if the point is within any of the BAs
    for _, row in gdf.iterrows():
        if row['geometry'].contains(point):
            return row['abbrev']
    
    return None  # If no BA contains the point

# Loads all of the history that we have for a specific ba
def load_ba_history(ba):
    return ba_histories.get(ba)

# loads solar data from disk
def load_solar_history(latitude, longitude):
    """
    Load solar data from disk for the given latitude and longitude coordinates.
    
    Parameters:
    - latitude (float): Latitude of the location
    - longitude (float): Longitude of the location
    
    Returns:
    - data (dict): The solar data for the given coordinates, or None if the file does not exist.
    """
    file_path = f"data/nasa/{latitude}/{longitude}.json"
    
    # Check if the file exists
    if os.path.isfile(file_path):
        with open(file_path, 'r') as file:
            data = json.load(file)
            return data
    else:
        print(f"No data found for latitude {latitude} and longitude {longitude}.")
        return None

# save a row to the csv
def save_to_csv(row, csvfile):
    writer = csv.writer(csvfile)
    writer.writerow(row)
 
# Helper function to generate a range of floating point numbers
def frange(start, stop, step):
    while start < stop:
        yield start
        start += step

# Open the file that has all of the BA map data.
file_path = os.path.join('data', 'ba_maps.json')
gdf = gpd.read_file(file_path)

# Load all of the ba histories
ba_histories = load_all_ba_history()

# Set up a range of coordinates to loop over.
latitudes = [round(lat, 1) for lat in frange(24, 50, 0.2)]
longitudes = [round(lon, 1) for lon in frange(-125, -66, 0.2)]
csvfile = open("data/solar_values.csv", 'a', newline='')
for latitude in latitudes:
    csvfile.flush()
    for longitude in longitudes:
        # Load the solar data for this coordinate.
        solar_history = load_solar_history(latitude, longitude)

        # We will produce a 4x4 grid for each solar data point. This
        # is because BAs need higher resolution than solar map, and
        # getting BA data is a lot cheaper than getting solar data.
        for lat in frange(latitude, latitude+0.2, 0.04):
            for lon in frange(longitude, longitude+0.2, 0.04):
                # Get the ba for this coordinate
                ba = get_ba_by_coords(gdf, lat, lon)
                if ba is None:
                    continue
                # Load the history for the ba.
                ba_history = ba_histories.get(ba)
                if ba_history is None:
                    continue
                
                # Process the data to determine the total carbon credits for this coordinate.
                total_kwh = 0
                total_moer = 0
                total_hours = 0
                for day_data, sun_intensity in solar_history['properties']['parameter']['ALLSKY_SFC_SW_DWN'].items():
                    # Skip if there is no sunlight data
                    if sun_intensity is None:
                        continue

                    # Extract the hour from the timestamp (last two characters of the 'hour_end' key)
                    hour_f = f"{day_data[-2:]}"
                    day_f = f"{day_data[4:6]}-{day_data[6:8]}"
                    year_f = f"{day_data[:4]}"
                    moer_values = ba_history[year_f][day_f][hour_f]

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
                save_to_csv([lat, lon, total_carbon_credits], csvfile)
