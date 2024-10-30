import os
import json
import requests
import time
from concurrent.futures import ThreadPoolExecutor
import numpy as np
import geopandas as gpd
import pandas as pd

# Path to the data directory
DATA_DIR = 'data/nasa'

# Meta parameters for NASA API
META_params = {
    "parameters": "ALLSKY_SFC_SW_DWN",
    "community": "RE",
    "start": "20200101",
    "end": "20231231",
}

# Function to save data to disk
def save_to_disk(data, latitude, longitude):
    folder_path = os.path.join(DATA_DIR, f"{latitude}")
    if not os.path.exists(folder_path):
        os.makedirs(folder_path)
    file_path = os.path.join(folder_path, f"{longitude}.json")
    with open(file_path, 'w') as f:
        json.dump(data, f)

# Function to fetch data from NASA API and save to disk
def fetch_and_save_data(latitude, longitude):
    time.sleep(1)  # Adjust sleep time as needed to respect API rate limits
    url = "https://power.larc.nasa.gov/api/temporal/hourly/point"
    params = {
        "parameters": META_params["parameters"],
        "community": META_params["community"],
        "longitude": longitude,
        "latitude": latitude,
        "start": META_params["start"],
        "end": META_params["end"],
        "format": "json"
    }

    try:
        response = requests.get(url, params=params)
        response.raise_for_status()
        data = response.json()
        # Save the data to disk
        save_to_disk(data, latitude, longitude)
        print(f"Data fetched and saved for latitude {latitude}, longitude {longitude}")
    except Exception as e:
        print(f"Error fetching data for latitude {latitude}, longitude {longitude}: {e}")

def main():
    # Load BA polygons (ba_maps.json)
    file_path = os.path.join('data', 'ba_maps.json')
    ba_gdf = gpd.read_file(file_path)
    ba_gdf = ba_gdf.to_crs(epsg=4326)

    # Generate the grid of points with increased resolution (0.0390625 degrees)
    resolution = 0.0390625  # Resolution to match the main script
    latitudes = np.arange(-90, 90 + resolution, resolution)
    longitudes = np.arange(-180, 180 + resolution, resolution)
    num_lats = len(latitudes)
    num_lons = len(longitudes)
    total_points = num_lats * num_lons
    print(f"Total number of points: {total_points}")

    # Create empty ba_array
    ba_array = np.zeros((num_lats, num_lons), dtype=np.int16)

    # Define number of chunks
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

        # Create a GeoDataFrame of points
        points_gdf = gpd.GeoDataFrame(geometry=points, crs="EPSG:4326")

        # Perform spatial join
        joined = gpd.sjoin(points_gdf, ba_gdf[['region', 'geometry']], how='left', predicate='within')

        # Update mapping from BA regions to integer IDs
        ba_regions_array = joined['region'].values
        unique_bas_in_chunk = pd.unique(ba_regions_array[pd.notna(ba_regions_array)])

        for ba in unique_bas_in_chunk:
            if ba not in ba_to_id:
                ba_id = len(ba_to_id) + 1  # Start from 1
                ba_to_id[ba] = ba_id
                id_to_ba[ba_id] = ba

        # Map BA regions to IDs
        ba_region_ids = np.array([ba_to_id.get(ba, 0) if pd.notna(ba) else 0 for ba in ba_regions_array], dtype=np.int16)

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

    print(f"Number of points contained within a BA: {num_points_with_ba}")

    # Now proceed to process the mini-grids
    print("Processing mini-grids...")

    # Define the NASA grid resolution (0.625 degrees)
    nasa_resolution = 0.625  # NASA data resolution remains the same
    nasa_latitudes = np.arange(-90, 90, nasa_resolution)
    nasa_longitudes = np.arange(-180, 180, nasa_resolution)

    # List to store NASA grid coordinates to fetch
    coords_to_fetch = []

    total_mini_grids = 0
    num_mini_grids_with_ba = 0

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

            # Check if any BA ID is non-zero (meaning the point is within a BA)
            if np.any(mini_grid_bas):
                num_mini_grids_with_ba += 1
                coords_to_fetch.append((lat, lon))

    print(f"Total number of mini-grids: {total_mini_grids}")
    print(f"Number of mini-grids with at least one point in a BA: {num_mini_grids_with_ba}")
    print(f"Total coordinates to fetch from NASA API: {len(coords_to_fetch)}")

    # Ensure data directories exist
    if not os.path.exists(DATA_DIR):
        os.makedirs(DATA_DIR)

    # Load existing data points
    existing_coords = set()
    for root, dirs, files in os.walk(DATA_DIR):
        for file in files:
            if file.endswith('.json'):
                try:
                    lon = float(file[:-5])  # Remove '.json'
                    lat = float(os.path.basename(root))
                    existing_coords.add((lat, lon))
                except ValueError:
                    continue  # Skip files with unexpected names

    # Filter out existing data points
    coords_to_fetch = [coord for coord in coords_to_fetch if coord not in existing_coords]

    print(f"Coordinates to fetch after removing existing data: {len(coords_to_fetch)}")

    # Fetch data from NASA API using multithreading
    max_workers = 9  # Number of threads

    if coords_to_fetch:
        with ThreadPoolExecutor(max_workers=max_workers) as executor:
            futures = []
            for lat, lon in coords_to_fetch:
                futures.append(executor.submit(fetch_and_save_data, lat, lon))
            for future in futures:
                future.result()

    print("\nData collection complete.")

if __name__ == "__main__":
    print(f"Downloading solar data from {META_params['start']} to {META_params['end']}")
    main()
