"""
Requirements:

- The script reads a large CSV file containing data points with latitude, longitude, and value.
- The data corresponds to a map of the world but is sparse (missing in areas like oceans).
- We need to create 8 passes, each producing a heatmap image 'heatmap_n.png', where n is the pass number.
- Each pass doubles the resolution of the previous pass, starting from 10 degrees in pass 1.
    - Pass 1 resolution: 10 degrees
    - Pass 2 resolution: 5 degrees
    - Pass 3 resolution: 2.5 degrees
    - Pass 4 resolution: 1.25 degrees
    - Pass 5 resolution: 0.625 degrees
    - Pass 6 resolution: 0.3125 degrees
    - Pass 7 resolution: 0.15625 degrees
    - Pass 8 resolution: 0.078125 degrees
- For each pass, the script scans the CSV file line by line and only includes data points that align with the current resolution grid.
    - A data point is included in pass n if both (latitude + 90) and (longitude + 180) are multiples of the current resolution (within a small epsilon).
- The script should not load the entire CSV into memory but process it line by line to handle large files.
- Each heatmap should have exactly one pixel per non-sparse data point.
- The images cover the entire world, from -90 to 90 latitude, and -180 to 180 longitude.
- The images should be saved as 'data/heatmap_n.png' where n is the pass number.
- The images should not include any legends, color bars, or additional features.
- The pixels should be colored according to the data values, which range from 0 to 5.
- The color gradient should transition through the colors: 'lime', 'yellow', 'orange', 'red', 'purple', 'black'.
- The script should continue generating images until it runs out of memory and crashes (due to increasing resolution).

"""

import numpy as np
import matplotlib.pyplot as plt
from matplotlib.colors import LinearSegmentedColormap, Normalize
import csv
import math

# Define the colors for the custom colormap
#colors = ['#E0FFE0', 'lime', 'yellow', 'gold', 'orange', 'darkorange', 'red', 'purple', 'black']
#colors = ['#C0FFC0', '#77FF77', 'greenyellow', 'yellow', 'gold', 'orange', 'darkorange', 'red', 'purple', 'black']
colors = ['#00FF00', '#55FF00', '#99FF00', '#FFFF00', '#FFAA00', '#FF7700', '#FF3300', '#FF0000', '#AA00AA', '#000000']
cmap_name = 'custom_colormap'
n_bins = 600  # Number of bins in the colormap
custom_colormap = LinearSegmentedColormap.from_list(cmap_name, colors, N=n_bins)

# Value range for normalization
vmin = 0.0
vmax = 3.4
norm = Normalize(vmin=vmin, vmax=vmax)

epsilon = 1e-7  # Small tolerance for floating point comparison
largest = 0

csv_file_path = 'data/solar_values.csv'  # Path to your CSV file

for pass_num in range(1, 10):
    print(f"Starting pass {pass_num}")
    try:
        # Calculate the resolution for the current pass
        resolution = 10.0 / (2 ** (pass_num - 1))
        print(f"Resolution for pass {pass_num}: {resolution} degrees")

        # Calculate the number of grid cells
        num_longitudes = int(360 / resolution) + 1  # From -180 to 180
        num_latitudes = int(180 / resolution) + 1   # From -90 to 90
        print(f"Grid size for pass {pass_num}: {num_latitudes} x {num_longitudes}")

        # Initialize the data array with NaNs
        data_array = np.full((num_latitudes, num_longitudes), np.nan)

        # Open the CSV file and process line by line
        with open(csv_file_path, 'r') as csvfile:
            csv_reader = csv.reader(csvfile)
            for row in csv_reader:
                try:
                    lat = float(row[0])
                    lon = float(row[1])
                    value = float(row[2])

                    # Check if the data point aligns with the current grid resolution
                    lat_mod = (lat + 90) % resolution
                    lon_mod = (lon + 180) % resolution

                    if lat_mod < epsilon or abs(lat_mod - resolution) < epsilon:
                        if lon_mod < epsilon or abs(lon_mod - resolution) < epsilon:
                            # Calculate the indices
                            idx_lat = int(round((lat + 90) / resolution))
                            idx_lon = int(round((lon + 180) / resolution))

                            # Ensure indices are within array bounds
                            if 0 <= idx_lat < num_latitudes and 0 <= idx_lon < num_longitudes:
                                data_array[idx_lat, idx_lon] = value
                                if value > largest:
                                    largest = value
                except Exception as e:
                    # print(f"Error processing row {row}: {e}")
                    continue

        # Create the figure and axis
        fig, ax = plt.subplots(figsize=(num_longitudes / 100, num_latitudes / 100), dpi=100)

        # Display the data array as an image
        # Flip the data array vertically to align with latitude increasing from bottom to top
        img = ax.imshow(np.flipud(data_array), cmap=custom_colormap, norm=norm,
                        extent=[-180, 180, -90, 90], interpolation='none')

        # Remove axes for a clean image
        ax.axis('off')

        # Save the image
        output_path = f'data/heatmap_{pass_num}.png'
        plt.savefig(output_path, bbox_inches='tight', pad_inches=0)
        plt.close(fig)
        print(f"Pass {pass_num} completed, image saved to {output_path}")

    except MemoryError:
        print(f"MemoryError occurred at pass {pass_num}. Stopping script.")
        break
    except Exception as e:
        print(f"An error occurred at pass {pass_num}: {e}")
        break

print(f"Largest Value: {largest}")
