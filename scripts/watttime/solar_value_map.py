############################################################
# Requirements / Instructions:
#
# - This script loads Balancing Authority (BA) polygons and maps them onto a
#   latitude/longitude grid. Each cell in that grid is labeled with a BA ID.
# - It then processes NASA solar data (hourly) for each mini-grid cell at a
#   coarser resolution (0.625 degrees). For each cell, the script:
#     1. Loads the NASA solar data from JSON (hourly).
#     2. Loads the BA's MOER data in 5-min increments for the entire year of 2024.
#     3. Aligns MOER data and solar data by UTC hour, weighting the average MOER by
#        the amount of sunlight each hour.
#     4. Converts MOER from lbs CO2/MWh to metric tons CO2/MWh (1 metric ton of CO2 = 1 carbon credit).
#     5. Produces a final "cc_per_mwh" (carbon credits per MWh) for each (lat, lon, BA).
# - The output is stored in "data/solar_values.csv".
#
# Leap-Year Note (2024):
# - 2024 is a leap year with 366 days. That means 366 * 24 hours = 8,784 hours total.
# - For 5-minute increments, 8,784 hours * 12 intervals per hour = 105,408 intervals.
#   However, NASA data often lumps full hours, so your final count might differ
#   depending on how data extends from Jan 1 00:00 to Dec 31 23:55. 
# - We are using 105,264 as the total number of 5-min intervals for the year. That
#   means we allow for 366 days * 24 hours * 12 intervals/hour = 105,264 intervals.
#
# If you switch to a non-leap year (e.g., 2025):
# - You would typically revert to 105,120 intervals = 365 days * 24 hours * 12 intervals/hour.
# - And you'd also make sure the NASA data fetching starts at 20250101 and ends at 20251231
#   (plus or minus any partial coverage).
#
# Historic instructions that still apply:
# - Code style should be reminiscent of David Vorick's polished code: straightforward logic,
#   explicit error handling, and composable functions.
# - Output must satisfy all existing requirements unless explicitly changed above.
#
############################################################

import geopandas as gpd
import pandas as pd
import numpy as np
import os
import json
import csv
from datetime import datetime, timezone

# Two singleton values to track the smallest and largest cc_per_mwh collected.
# This helps us understand the range of computed values over the entire run.
smallest_cc_per_mwh = 10000000
largest_cc_per_mwh = 0

def load_ba_histories():
    """
    Load BA histories from JSON files into a dictionary. Each BA has an array of
    MOER values (lbs CO2 per MWh) for every 5-minute interval of the year 2024.
    
    For 2024, we have a leap year with 366 days. That implies:
        366 days * 24 hours/day * 12 intervals/hour = 105,264 intervals
    
    If you switch to a different year (like 2025), you'd typically revert to 365 days:
        365 days * 24 hours/day * 12 intervals/hour = 105,120 intervals
    
    Returns:
        dict: A dictionary mapping BA names to NumPy arrays of MOER values (lbs CO2 per MWh).
    """
    # Print a heads-up
    print("Starting to load BA histories for a leap year (2024)")

    ba_histories = {}  # Dictionary to hold BA name -> numpy array
    base_folder_path = os.path.join("data")

    # This is the critical variable if you want to handle a different year:
    # For 2024 (leap year), we choose 105,264. For a non-leap year, 105,120.
    num_intervals = 105264  # 366 days * 24 hours * 12 intervals/hour

    for ba_folder in os.listdir(base_folder_path):
        # Skip the 'nasa' folder because that's where solar data is stored,
        # not MOER data from WattTime or other sources.
        if ba_folder == 'nasa':
            continue

        ba_folder_path = os.path.join(base_folder_path, ba_folder)
        if os.path.isdir(ba_folder_path):
            print(f"Loading data for BA: {ba_folder}")

            # Create an empty array for the MOER data
            ba_array = np.zeros(num_intervals, dtype=np.float32)
            total_value = 0.0
            num_values = 0

            # Collect all JSON file paths for this BA
            ba_filepaths = [
                os.path.join(ba_folder_path, filename)
                for filename in os.listdir(ba_folder_path)
                if filename.endswith('.json')
            ]

            # Process each JSON file to fill the ba_array
            for filepath in ba_filepaths:
                with open(filepath, 'r') as jsonfile:
                    data = json.load(jsonfile)
                    # Typically, 'data' is a dictionary with 'data' -> list of entries
                    # Each entry has 'point_time' (string) and 'value' (float).
                    for entry in data['data']:
                        timestamp = entry['point_time']
                        value = float(entry['value'])

                        # Parse timestamp to get a Python datetime in UTC
                        # WattTime data is typically provided in UTC, but let's handle
                        # the case where the time zone might be missing or different.
                        try:
                            dt = datetime.strptime(timestamp, '%Y-%m-%dT%H:%M:%S%z')
                        except ValueError:
                            dt = datetime.strptime(timestamp, '%Y-%m-%dT%H:%M:%S')
                            # If there's truly no timezone info, we might forcibly set it to UTC:
                            dt = dt.replace(tzinfo=timezone.utc)

                        # Calculate how many minutes have elapsed since the start of the year
                        year_start = datetime(dt.year, 1, 1, tzinfo=timezone.utc)
                        delta = dt - year_start
                        total_minutes = int(delta.total_seconds() // 60)
                        index = total_minutes // 5  # Each interval is 5 minutes

                        # If index is out of range (e.g., due to an extra day or mismatch),
                        # the code warns and skips writing that data.
                        if 0 <= index < num_intervals:
                            ba_array[index] = value
                            total_value += value
                            num_values += 1
                        else:
                            print(f"Warning: timestamp {timestamp} out of bounds in file {os.path.basename(filepath)}")

            # Compute an average MOER just for logging or debugging
            average_value = total_value / num_values if num_values > 0 else 0.0
            print(f"Finished loading BA {ba_folder}, average MOER (lbs/MWh): {average_value:.4f}")

            # Store the array in the dictionary
            ba_histories[ba_folder] = ba_array

    return ba_histories

def load_solar_history(lat, lon):
    """
    Load solar data for the given NASA grid point (latitude, longitude).
    The NASA data is stored in subfolders by latitude, then JSON files by longitude.
    This data is hourly and includes irradiance (W/m^2) for each hour of 2024 (a leap year).

    If you switch to 2025 or any other year, you'll need to adjust your NASA data retrieval
    script to fetch that year's data. The logic below doesn't change, but the data references
    (start/end dates) do.
    
    Args:
        lat (float): The latitude coordinate.
        lon (float): The longitude coordinate.

    Returns:
        dict or None: Parsed solar JSON data if found, otherwise None.
    """
    base_dir = os.path.join('data', 'nasa')

    # NASA data directory organization:
    # data/nasa/<latitude>/<longitude>.json
    #
    # We try integer variants and float variants because sometimes lat, lon might be stored
    # as strings like "35" or "35.0", etc.

    lat_variants = {str(lat)}
    if float(lat).is_integer():
        lat_variants.add(str(int(lat)))
        lat_variants.add(f"{int(lat)}.0")

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

    # If no file is found that matches the lat/lon combination, return None
    return None

def compute_glow_strength(ba_name, solar_history, ba_histories):
    """
    Computes the 'cc_per_mwh' (carbon credits per MWh) for a given BA and solar history.

    Steps:
    1. Looks up the BA's MOER data (lbs CO2 / MWh) at 5-min intervals for 2024 (a leap year).
    2. Iterates over the NASA solar irradiance data, which is in hourly increments.
    3. For each hour, gather the 12 five-minute intervals (5 * 12 = 60 min) from the MOER array.
    4. Average those 12 intervals to get an hourly MOER value, converting from lbs to metric tons
       by dividing by 2204.62.
    5. Weight that value by the hour's solar intensity (W/m^2, converted to kW/m^2 by dividing by 1000).
    6. Accumulate totals to find an overall weighted average across the entire year.
    7. Return that average in metric tons CO2 per MWh. This also equals carbon credits per MWh if
       we define 1 carbon credit = 1 metric ton CO2 avoided.

    Args:
        ba_name (str): Name of the Balancing Authority.
        solar_history (dict): Dictionary loaded from NASA containing hourly solar data for the year.
        ba_histories (dict): Maps BA name -> array of MOER data (lbs CO2 per MWh).

    Returns:
        float or None: The computed cc_per_mwh (CO2 metric tons / MWh) or None if insufficient data.
    """
    ba_array = ba_histories.get(ba_name)
    if ba_array is None:
        return None

    # We'll accumulate sums to compute a weighted average
    total_kwh = 0.0    # Sum of all "sun intensity" across the year for weighting
    total_moer = 0.0   # Sum of (MOER * sun intensity)

    # The NASA data is typically in a structure like:
    # solar_history['properties']['parameter']['ALLSKY_SFC_SW_DWN']['YYYYMMDDHH'] = float irradiance
    # Each key is an hour of 2024, e.g. "2024010100" for Jan 1, 00:00 UTC.
    allsky_sfc_sw_dwn = solar_history['properties']['parameter'].get('ALLSKY_SFC_SW_DWN', {})

    for day_hour, sun_intensity in allsky_sfc_sw_dwn.items():
        if sun_intensity is None:
            continue

        # Parse the string "YYYYMMDDHH"
        # Example: "2024022912" => 2024-02-29 12:00 UTC (leap day)
        try:
            dt = datetime.strptime(day_hour, '%Y%m%d%H')
        except ValueError:
            print(f"Invalid date format in solar data: {day_hour}")
            continue

        # Convert to UTC-based datetime
        dt = dt.replace(tzinfo=timezone.utc)
        year_start = datetime(dt.year, 1, 1, tzinfo=timezone.utc)
        delta = dt - year_start
        total_minutes = int(delta.total_seconds() // 60)
        index = total_minutes // 5  # For each hour, we look at 12 intervals (5-min each)

        # Gather 12 intervals for this hour: index, index+1, ..., index+11
        indices = range(index, index + 12)
        indices = [i for i in indices if 0 <= i < len(ba_array)]
        moer_values = ba_array[indices]

        if len(moer_values) == 0:
            continue

        # Average MOER in lbs CO2 / MWh
        avg_moer_lbs = np.mean(moer_values)
        # Convert lbs to metric tons
        avg_moer_metric_tons = avg_moer_lbs / 2204.62

        # NASA's solar intensity is in W/m^2. Convert W to kW: / 1000
        # We'll treat sun_intensity_kw as a weighting factor for each hour.
        sun_intensity_kw = sun_intensity / 1000.0

        # Accumulate
        total_kwh += sun_intensity_kw
        total_moer += avg_moer_metric_tons * sun_intensity_kw

    # If there's no solar data or everything is zero, we get a division by zero
    if total_kwh == 0.0:
        return None

    # Weighted average in metric tons CO2 per kWh
    avg_moer_per_kwh = total_moer / total_kwh
    # Convert kWh to MWh by multiplying by 1000
    avg_moer_per_mwh = avg_moer_per_kwh * 1000.0
    cc_per_mwh = avg_moer_per_mwh

    # Update global smallest and largest
    global smallest_cc_per_mwh, largest_cc_per_mwh
    if cc_per_mwh < smallest_cc_per_mwh:
        smallest_cc_per_mwh = cc_per_mwh
        print("new smallest cc_per_mwh", cc_per_mwh)
    if cc_per_mwh > largest_cc_per_mwh:
        largest_cc_per_mwh = cc_per_mwh
        print("new largest cc_per_mwh", cc_per_mwh)

    return cc_per_mwh

def main():
    """
    Main entry point of the script. High-level steps:
    1. Creates a fine-resolution grid (-90 to 90 lat, -180 to 180 lon) with 0.0390625° steps.
    2. Loads BA polygons (ba_maps.json) and maps each grid cell to a BA ID.
    3. Builds a coarser NASA grid (0.625°) and, for each cell, identifies which BAs are present.
    4. Loads NASA solar data for that cell (year 2024) and loads BA MOER data in 5-min increments.
    5. Calls compute_glow_strength(...) to get cc_per_mwh for each BA in that mini-grid.
    6. Writes output rows (lat, lon, cc_per_mwh, BA) to data/solar_values.csv.

    Leap Year Handling:
    - For 2024, we store 105,264 intervals in each BA's MOER array (366 days).
    - If switching to 2025 or another non-leap year, you'd revert to 105,120 intervals and re-fetch
      NASA data for that year's start/end.
    """
    print("Loading BA grid...")

    # 1. Generate the fine grid of points
    resolution = 0.0390625  # Fine resolution for BA data
    latitudes = np.arange(-90, 90 + resolution, resolution)
    longitudes = np.arange(-180, 180 + resolution, resolution)
    num_lats = len(latitudes)
    num_lons = len(longitudes)
    total_points = num_lats * num_lons
    print(f"Total number of points (fine grid): {total_points}")

    # Create empty ba_array: holds the BA ID (int) for each (lat_idx, lon_idx)
    ba_array = np.zeros((num_lats, num_lons), dtype=np.int16)

    # 2. Read BA polygons
    file_path = os.path.join('data', 'ba_maps.json')
    ba_gdf = gpd.read_file(file_path)
    # Ensure polygons are in EPSG:4326 (lat/lon)
    ba_gdf = ba_gdf.to_crs(epsg=4326)

    # 3. Define chunking to reduce memory usage in the spatial join
    num_chunks = 4
    chunk_size = num_lats // num_chunks

    # Mappings from BA region name to integer ID and back
    ba_to_id = {}
    id_to_ba = {}

    num_points_with_ba = 0

    # Process latitudes in chunks
    for chunk_idx in range(num_chunks):
        print(f"Processing chunk {chunk_idx+1} of {num_chunks}")

        lat_start_idx = chunk_idx * chunk_size
        if chunk_idx == num_chunks - 1:
            lat_end_idx = num_lats
        else:
            lat_end_idx = (chunk_idx + 1) * chunk_size

        lat_chunk = latitudes[lat_start_idx:lat_end_idx]

        # Create meshgrid for this chunk
        lon_grid, lat_grid = np.meshgrid(longitudes, lat_chunk)

        # Flatten
        lat_flat = lat_grid.ravel()
        lon_flat = lon_grid.ravel()
        points = gpd.points_from_xy(lon_flat, lat_flat)

        # Build a GeoDataFrame
        points_gdf = gpd.GeoDataFrame(geometry=points, crs="EPSG:4326")

        # Spatial join: which BA polygon does each point fall within?
        joined = gpd.sjoin(
            points_gdf,
            ba_gdf[['region', 'geometry']],
            how='left',
            predicate='within'
        )

        ba_regions_array = joined['region'].values
        unique_bas_in_chunk = pd.unique(ba_regions_array[pd.notna(ba_regions_array)])

        # Assign new IDs for newly encountered BAs
        for ba in unique_bas_in_chunk:
            if ba not in ba_to_id:
                ba_id = len(ba_to_id) + 1  # BA IDs start from 1
                ba_to_id[ba] = ba_id
                id_to_ba[ba_id] = ba

        # Map each BA region name to its ID. If region is NaN, store 0.
        ba_region_ids = np.array(
            [ba_to_id.get(ba, 0) if pd.notna(ba) else 0 for ba in ba_regions_array],
            dtype=np.int16
        )

        # Convert lat/lon in this chunk to array indices
        local_lat_indices = ((joined.geometry.y.values - lat_chunk[0]) / resolution).round().astype(int)
        lon_indices = ((joined.geometry.x.values + 180) / resolution).round().astype(int)

        # Global lat index in the full array
        lat_indices = lat_start_idx + local_lat_indices

        # Clip indices to avoid out-of-bounds
        lat_indices = np.clip(lat_indices, 0, num_lats - 1)
        lon_indices = np.clip(lon_indices, 0, num_lons - 1)

        # Assign BA IDs to the ba_array
        ba_array[lat_indices, lon_indices] = ba_region_ids

        # Count how many points in this chunk had a BA
        num_points_with_ba += np.count_nonzero(ba_region_ids)

    print(f"Total number of points in the fine array: {ba_array.size}")
    print(f"Number of points contained within a BA: {num_points_with_ba}")

    # 4. Now proceed to process the mini-grids at NASA resolution (0.625 degrees)
    print("Processing mini-grids at NASA resolution...")

    nasa_resolution = 0.625
    nasa_latitudes = np.arange(-90, 90, nasa_resolution)
    nasa_longitudes = np.arange(-180, 180, nasa_resolution)

    # Load BA histories for the entire year (now sized for leap year 2024)
    ba_histories = load_ba_histories()

    # Open the CSV file for writing results
    with open("data/solar_values.csv", 'w', newline='') as csvfile:
        writer = csv.writer(csvfile)
        # Write header (csv has columns: latitude, longitude, cc_per_mwh, BA)
        writer.writerow(['latitude', 'longitude', 'cc_per_mwh', 'BA'])

        num_mini_grids_with_ba = 0
        total_mini_grids = 0

        for lat in nasa_latitudes:
            print(f"Processing mini-grids for latitude {lat}")
            for lon in nasa_longitudes:
                total_mini_grids += 1

                # For each NASA cell, find the bounding indices in the fine grid.
                lat_start_idx = int(round((lat + 90) / resolution))
                lat_end_idx = int(round((lat + nasa_resolution + 90) / resolution))
                lon_start_idx = int(round((lon + 180) / resolution))
                lon_end_idx = int(round((lon + nasa_resolution + 180) / resolution))

                # Clip to avoid going out of range
                lat_start_idx = max(0, lat_start_idx)
                lat_end_idx = min(num_lats, lat_end_idx)
                lon_start_idx = max(0, lon_start_idx)
                lon_end_idx = min(num_lons, lon_end_idx)

                # Extract the BA IDs in this mini-grid
                mini_grid_bas = ba_array[lat_start_idx:lat_end_idx, lon_start_idx:lon_end_idx]

                # Check which unique BA IDs appear
                unique_ba_ids = np.unique(mini_grid_bas)
                unique_ba_ids = unique_ba_ids[unique_ba_ids != 0]  # Exclude 0 (no BA)

                if unique_ba_ids.size == 0:
                    continue  # Skip if no BA is present

                num_mini_grids_with_ba += 1

                # Load NASA solar data once for this (lat, lon)
                solar_history = load_solar_history(lat, lon)
                if solar_history is None:
                    print(f"Could not load solar history for lat {lat}, lon {lon}")
                    continue  # We skip if solar data is missing

                # We'll cache the cc_per_mwh for each BA in this mini-grid
                ba_cache = {}

                # Compute cc_per_mwh for each BA in the mini-grid
                for ba_id in unique_ba_ids:
                    ba_region = id_to_ba[ba_id]
                    if ba_region in ba_cache:
                        # Already computed for this BA_region; skip
                        continue
                    cc_per_mwh = compute_glow_strength(ba_region, solar_history, ba_histories)
                    if cc_per_mwh is not None:
                        ba_cache[ba_region] = cc_per_mwh
                    else:
                        print(f"Could not compute cc_per_mwh for BA {ba_region}")

                # Write a row to the CSV for each point in this mini-grid
                mini_grid_latitudes = latitudes[lat_start_idx:lat_end_idx]
                mini_grid_longitudes = longitudes[lon_start_idx:lon_end_idx]
                lon_grid, lat_grid = np.meshgrid(mini_grid_longitudes, mini_grid_latitudes)
                lat_flat = lat_grid.ravel()
                lon_flat = lon_grid.ravel()
                ba_ids_flat = mini_grid_bas.ravel()

                for idx in range(len(ba_ids_flat)):
                    ba_id = ba_ids_flat[idx]
                    if ba_id == 0:
                        continue
                    ba_region = id_to_ba[ba_id]
                    cc_per_mwh = ba_cache.get(ba_region)
                    if cc_per_mwh is None:
                        # Means we couldn't compute a value for that BA
                        continue
                    lat_point = lat_flat[idx]
                    lon_point = lon_flat[idx]
                    writer.writerow([lat_point, lon_point, cc_per_mwh, ba_region])

                # Print statement per mini-grid
                ba_list_str = ', '.join(ba_cache.keys())
                print(f"Mini-grid at ({lat}, {lon}) processed with BAs: {ba_list_str}")

                # Flush after each mini-grid to reduce data loss on crash
                csvfile.flush()

        print(f"Total number of mini-grids: {total_mini_grids}")
        print(f"Number of mini-grids with at least one point in a BA: {num_mini_grids_with_ba}")
        print("Final smallest cc_per_mwh:", smallest_cc_per_mwh)
        print("Final largest cc_per_mwh:", largest_cc_per_mwh)

if __name__ == '__main__':
    main()
