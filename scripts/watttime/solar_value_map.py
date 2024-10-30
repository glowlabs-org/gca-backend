import geopandas as gpd
import pandas as pd
import numpy as np
import os
import json
import csv
from datetime import datetime, timezone

def load_ba_histories():
    print("Starting to load BA histories")

    ba_histories = {}  # Dictionary to hold BA name -> numpy array
    base_folder_path = os.path.join("data")

    # Number of 5-minute intervals in a non-leap year
    num_intervals = 105120  # (365 days * 24 hours * 12 intervals per hour)

    for ba_folder in os.listdir(base_folder_path):
        if ba_folder == 'nasa':
            continue
        ba_folder_path = os.path.join(base_folder_path, ba_folder)
        if os.path.isdir(ba_folder_path):
            print(f"Loading data for BA: {ba_folder}")

            ba_array = np.zeros(num_intervals, dtype=np.float32)
            total_value = 0.0
            num_values = 0

            # Collect all JSON file paths for the BA
            ba_filepaths = [os.path.join(ba_folder_path, filename)
                            for filename in os.listdir(ba_folder_path)
                            if filename.endswith('.json')]

            # Process each file
            for filepath in ba_filepaths:
                with open(filepath, 'r') as jsonfile:
                    data = json.load(jsonfile)
                    for entry in data['data']:
                        timestamp = entry['point_time']  # e.g., '2023-01-01T00:00:00+00:00'
                        value = float(entry['value'])

                        # Parse timestamp to get the index into the array
                        try:
                            # Parse the timestamp with timezone information
                            dt = datetime.strptime(timestamp, '%Y-%m-%dT%H:%M:%S%z')
                        except ValueError:
                            # Handle timestamps without timezone
                            dt = datetime.strptime(timestamp, '%Y-%m-%dT%H:%M:%S')

                        # Calculate the index for the array
                        year_start = datetime(dt.year, 1, 1, tzinfo=timezone.utc)
                        delta = dt - year_start
                        total_minutes = int(delta.total_seconds() // 60)
                        index = total_minutes // 5  # Each interval is 5 minutes

                        # Check index bounds
                        if 0 <= index < num_intervals:
                            ba_array[index] = value
                            total_value += value
                            num_values += 1
                        else:
                            print(f"Warning: timestamp {timestamp} out of bounds in file {os.path.basename(filepath)}")

            # Compute average value
            average_value = total_value / num_values if num_values > 0 else 0.0
            print(f"Finished loading BA {ba_folder}, average value: {average_value:.4f}")

            # Store the array in the dictionary
            ba_histories[ba_folder] = ba_array

    return ba_histories

def load_solar_history(lat, lon):
    """
    Load solar data for the given NASA grid point.
    Handles integer vs float folder/file names.

    Returns:
        solar_history (dict) or None if not found.
    """
    base_dir = os.path.join('data', 'nasa')

    # Possible folder names for latitude
    lat_variants = {str(lat)}
    if float(lat).is_integer():
        lat_variants.add(str(int(lat)))
        lat_variants.add(f"{int(lat)}.0")

    # Possible file names for longitude
    lon_variants = {str(lon) + '.json'}
    if float(lon).is_integer():
        lon_variants.add(str(int(lon)) + '.json')
        lon_variants.add(f"{int(lon)}.0.json")

    for lat_folder in lat_variants:
        lat_folder_path = os.path.join(base_dir, lat_folder)
        if not os.path.isdir(lat_folder_path):
            continue
        for lon_file in lon_variants:
            file_path = os.path.join(lat_folder_path, lon_file)
            if os.path.isfile(file_path):
                try:
                    with open(file_path, 'r') as file:
                        data = json.load(file)
                        return data
                except json.JSONDecodeError as e:
                    print(f"Error decoding JSON from file: {file_path}. Error: {e}")
                except Exception as e:
                    print(f"An unexpected error occurred while reading file: {file_path}. Error: {e}")
    # If not found
    return None

def compute_total_carbon_credits(ba_name, solar_history, ba_histories):
    """
    Computes the total_carbon_credits value for a given BA and solar history.

    Returns:
        total_carbon_credits (float) or None if computation cannot be performed.
    """
    ba_array = ba_histories.get(ba_name)
    if ba_array is None:
        return None

    total_kwh = 0.0
    total_moer = 0.0
    total_hours = 0

    allsky_sfc_sw_dwn = solar_history['properties']['parameter'].get('ALLSKY_SFC_SW_DWN', {})
    for day_hour, sun_intensity in allsky_sfc_sw_dwn.items():
        if sun_intensity is None:
            continue

        # day_hour format is 'YYYYMMDDHH'
        try:
            dt = datetime.strptime(day_hour, '%Y%m%d%H')
        except ValueError:
            print(f"Invalid date format in solar data: {day_hour}")
            continue

        # Get the index in BA history array
        year_start = datetime(dt.year, 1, 1, tzinfo=timezone.utc)
        dt = dt.replace(tzinfo=timezone.utc)
        delta = dt - year_start
        total_minutes = int(delta.total_seconds() // 60)
        index = total_minutes // 5  # Each interval is 5 minutes

        # Get the MOER values for the 12 intervals corresponding to this hour
        indices = range(index, index + 12)
        # Ensure indices are within bounds
        indices = [i for i in indices if 0 <= i < len(ba_array)]
        moer_values = ba_array[indices]
        if len(moer_values) == 0:
            continue

        # Compute average MOER value for the hour
        average_moer = np.mean(moer_values)
        # Convert MOER from pounds to metric tons
        average_moer_metric_tons = average_moer / 2204.62
        # Convert sun intensity from W/m^2 to kW/m^2
        sun_intensity_kw = sun_intensity / 1000

        # Accumulate totals
        total_kwh += sun_intensity_kw
        total_moer += average_moer_metric_tons * sun_intensity_kw
        total_hours += 1

    # Check if we have valid data to avoid division by zero
    if total_kwh == 0 or total_hours == 0:
        return None

    # Calculate the average MOER value per megawatt hour
    average_moer_per_mwh = (total_moer / total_kwh)
    avg_hour = total_kwh / total_hours
    total_carbon_credits = average_moer_per_mwh / 1000 * avg_hour * 8766

    return total_carbon_credits

def main():
    print("loading BA grid")

    # 1. Generate the grid of points
    resolution = 0.0390625  # Updated resolution for BA data
    latitudes = np.arange(-90, 90 + resolution, resolution)
    longitudes = np.arange(-180, 180 + resolution, resolution)
    num_lats = len(latitudes)
    num_lons = len(longitudes)
    total_points = num_lats * num_lons
    print(f"Total number of points: {total_points}")

    # Create empty ba_array
    ba_array = np.zeros((num_lats, num_lons), dtype=np.int16)

    # 2. Read in the BA polygons
    file_path = os.path.join('data', 'ba_maps.json')
    ba_gdf = gpd.read_file(file_path)
    ba_gdf = ba_gdf.to_crs(epsg=4326)

    # 3. Define number of chunks
    num_chunks = 4
    chunk_size = num_lats // num_chunks

    # Initialize mapping from BA regions to integer IDs
    ba_to_id = {}
    id_to_ba = {}

    # For counting total points with BA
    num_points_with_ba = 0

    # Process each chunk
    for chunk_idx in range(num_chunks):
        print(f"Processing chunk {chunk_idx+1} of {num_chunks}")

        lat_start_idx = chunk_idx * chunk_size
        if chunk_idx == num_chunks - 1:
            lat_end_idx = num_lats  # Include remainder in last chunk
        else:
            lat_end_idx = (chunk_idx + 1) * chunk_size

        # Get the latitudes for this chunk
        lat_chunk = latitudes[lat_start_idx:lat_end_idx]

        # Create meshgrid for this chunk
        lon_grid, lat_grid = np.meshgrid(longitudes, lat_chunk)

        # Flatten the grids and create points
        lat_flat = lat_grid.ravel()
        lon_flat = lon_grid.ravel()
        points = gpd.points_from_xy(lon_flat, lat_flat)

        # 4. Create a GeoDataFrame of points
        points_gdf = gpd.GeoDataFrame(geometry=points, crs="EPSG:4326")

        # 5. Perform spatial join
        joined = gpd.sjoin(points_gdf, ba_gdf[['region', 'geometry']], how='left', predicate='within')

        # 6. Update mapping from BA regions to integer IDs
        ba_regions_array = joined['region'].values
        unique_bas_in_chunk = pd.unique(ba_regions_array[pd.notna(ba_regions_array)])

        for ba in unique_bas_in_chunk:
            if ba not in ba_to_id:
                ba_id = len(ba_to_id) + 1  # Start from 1
                ba_to_id[ba] = ba_id
                id_to_ba[ba_id] = ba

        # Map BA regions to IDs
        ba_region_ids = np.array([ba_to_id.get(ba, 0) if pd.notna(ba) else 0 for ba in ba_regions_array], dtype=np.int16)

        # Map latitudes and longitudes to indices
        # Compute local indices
        local_lat_indices = ((joined.geometry.y.values - lat_chunk[0]) / resolution).round().astype(int)
        lon_indices = ((joined.geometry.x.values + 180) / resolution).round().astype(int)

        # Global indices in ba_array
        lat_indices = lat_start_idx + local_lat_indices

        # Ensure indices are within bounds
        lat_indices = np.clip(lat_indices, 0, num_lats - 1)
        lon_indices = np.clip(lon_indices, 0, num_lons - 1)

        # Assign BA IDs to the array
        ba_array[lat_indices, lon_indices] = ba_region_ids

        # Update num_points_with_ba
        num_points_with_ba += np.count_nonzero(ba_region_ids)

    # 7. Print counts
    print(f"Total number of points in the array: {ba_array.size}")
    print(f"Number of points contained within a BA: {num_points_with_ba}")

    # Now proceed to process the mini-grids
    print("Processing mini-grids...")

    # Define the NASA grid resolution (0.625 degrees)
    nasa_resolution = 0.625  # No change here
    nasa_latitudes = np.arange(-90, 90, nasa_resolution)
    nasa_longitudes = np.arange(-180, 180, nasa_resolution)

    ba_histories = load_ba_histories()

    # Open the CSV file for writing
    with open("data/solar_values.csv", 'w', newline='') as csvfile:
        writer = csv.writer(csvfile)
        # Write header
        writer.writerow(['latitude', 'longitude', 'total_carbon_credits', 'BA'])

        num_mini_grids_with_ba = 0
        total_mini_grids = 0

        for lat in nasa_latitudes:
            print(f"Processing mini-grids for latitude {lat}")
            for lon in nasa_longitudes:
                total_mini_grids += 1

                # Determine the range of indices for this mini-grid
                lat_start_idx = int(round((lat + 90) / resolution))
                lat_end_idx = int(round((lat + nasa_resolution + 90) / resolution))
                lon_start_idx = int(round((lon + 180) / resolution))
                lon_end_idx = int(round((lon + nasa_resolution + 180) / resolution))

                # Ensure indices are within bounds
                lat_start_idx = max(0, lat_start_idx)
                lat_end_idx = min(num_lats, lat_end_idx)
                lon_start_idx = max(0, lon_start_idx)
                lon_end_idx = min(num_lons, lon_end_idx)

                # Extract the BA IDs for the mini-grid
                mini_grid_bas = ba_array[lat_start_idx:lat_end_idx, lon_start_idx:lon_end_idx]

                # Find unique BA IDs in the mini-grid
                unique_ba_ids = np.unique(mini_grid_bas)
                unique_ba_ids = unique_ba_ids[unique_ba_ids != 0]  # Exclude zero (no BA)

                if unique_ba_ids.size == 0:
                    continue  # No BA in this mini-grid

                num_mini_grids_with_ba += 1
                ba_cache = {}  # Cache for total_carbon_credits per BA in this mini-grid
                solar_history = None  # Cache solar history

                # Load solar history once per mini-grid
                solar_history = load_solar_history(lat, lon)
                if solar_history is None:
                    print(f"Could not load solar history for lat {lat}, lon {lon}")
                    continue  # Cannot proceed without solar data

                for ba_id in unique_ba_ids:
                    ba_region = id_to_ba[ba_id]
                    if ba_region in ba_cache:
                        total_carbon_credits = ba_cache[ba_region]
                    else:
                        total_carbon_credits = compute_total_carbon_credits(ba_region, solar_history, ba_histories)
                        if total_carbon_credits is None:
                            print(f"Could not compute total_carbon_credits for BA {ba_region}")
                            continue
                        ba_cache[ba_region] = total_carbon_credits

                # Now loop over each datapoint in the mini-grid
                mini_grid_latitudes = latitudes[lat_start_idx:lat_end_idx]
                mini_grid_longitudes = longitudes[lon_start_idx:lon_end_idx]
                lon_grid, lat_grid = np.meshgrid(mini_grid_longitudes, mini_grid_latitudes)
                lat_flat = lat_grid.ravel()
                lon_flat = lon_grid.ravel()
                ba_ids_flat = mini_grid_bas.ravel()

                for idx in range(len(ba_ids_flat)):
                    ba_id = ba_ids_flat[idx]
                    if ba_id == 0:
                        continue  # No BA for this point
                    ba_region = id_to_ba[ba_id]
                    total_carbon_credits = ba_cache.get(ba_region)
                    if total_carbon_credits is None:
                        continue  # total_carbon_credits could not be computed
                    lat_point = lat_flat[idx]
                    lon_point = lon_flat[idx]
                    writer.writerow([lat_point, lon_point, total_carbon_credits, ba_region])

                # Print statement per mini-grid
                print(f"Mini-grid at ({lat}, {lon}) processed with BAs: {', '.join(ba_cache.keys())}")

                # Flush the CSV after each mini-grid
                csvfile.flush()

        print(f"Total number of mini-grids: {total_mini_grids}")
        print(f"Number of mini-grids with at least one point in a BA: {num_mini_grids_with_ba}")

if __name__ == '__main__':
    main()
