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
- The images should not include any legends, color bars, or additional features for passes below 7.
- The pixels should be colored according to the data values, which range from 0 to 5.
- The color gradient should transition through the colors: 'lime', 'yellow', 'orange', 'red', 'purple', 'black'.
- For passes 7 and above, a color bar with the unit 'carbon credits per year, per kilowatt' should be added at the top.
    - Both the tick labels (scale numbers) and the label should be below the color bar.
    - The color bar should not extend to the full width of the image, leaving a border around it.
- The font size of the color bar label should scale appropriately with the resolution to maintain the same relative size.
- Make the heatmaps look as professional as possible.

"""

import numpy as np
import matplotlib.pyplot as plt
from matplotlib.colors import LinearSegmentedColormap, Normalize
import csv

# Define the colors for the custom colormap
#colors = ['#AAFFFF', '#66FFFF', '#00FFFF', '#00FF00', '#BBFF00', '#FFFF00', '#FFBB00', '#FF9900', '#FF6600', '#FF0000', '#DD0022', '#BB0055', '#990077', '#880088', '#660066', '#440044', '#220022', '#000000']
colors = ['#AAFFFF', '#66FFFF', '#00FFFF', '#00FFAA', '#00FF77', '#00FF00', '#99FF00', '#CCFF00', '#D7FF00', '#FFFF00', '#FFD700', '#FFCC00', '#FFAA00', '#FF8800', '#FF0000', '#BB0055', '#880088', '#440044', '#000000']


cmap_name = 'custom_colormap'
n_bins = 2400  # Number of bins in the colormap
custom_colormap = LinearSegmentedColormap.from_list(cmap_name, colors, N=n_bins)

# Value range for normalization
vmin = 0.0
vmax = 4.015
norm = Normalize(vmin=vmin, vmax=vmax)

epsilon = 1e-7  # Small tolerance for floating point comparison
largest = 0

csv_file_path = 'data/solar_values.csv'  # Path to your CSV file

# Base values for font size scaling
base_pass_num = 7
base_font_size = 12
base_resolution = 10.0 / (2 ** (base_pass_num - 1))

for pass_num in range(7, 10):
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
                except Exception:
                    continue

        # Create the figure and axis with margins
        fig, ax = plt.subplots(figsize=(num_longitudes / 100, num_latitudes / 100), dpi=100)
        fig.subplots_adjust(left=0.05, right=0.95, bottom=0.05)

        # Display the data array as an image
        # Flip the data array vertically to align with latitude increasing from bottom to top
        img = ax.imshow(np.flipud(data_array), cmap=custom_colormap, norm=norm,
                        extent=[-180, 180, -90, 90], interpolation='none')

        # Remove axes for a clean image
        ax.axis('off')

        # Add a color bar with units only for passes 7 and above
        if pass_num >= 7:
            # Calculate font size scaling factor
            scaling_factor = base_resolution / resolution
            font_size = base_font_size * scaling_factor

            # Create a colorbar axis above the main axis
            cbar_width = 0.8  # Fraction of the figure width
            cbar_height = 0.02  # Height of the colorbar
            cbar_left = (1 - cbar_width) / 2  # Center the colorbar horizontally
            cbar_bottom = 0.9  # Position of the bottom of the colorbar
            cax = fig.add_axes([cbar_left, cbar_bottom, cbar_width, cbar_height])

            # Create the color bar in the cax axis
            cbar = fig.colorbar(img, cax=cax, orientation='horizontal')

            # Move ticks and label below the color bar
            cbar.ax.xaxis.set_ticks_position('bottom')
            cbar.ax.xaxis.set_label_position('bottom')

            # Set the label with increased spacing
            cbar.set_label('Glow Strength', fontsize=font_size, labelpad=font_size * 0.8)

            # Adjust tick label font sizes
            cbar.ax.tick_params(labelsize=font_size * 0.8)

            # Remove colorbar outline for a cleaner look
            cbar.outline.set_visible(False)

            # Adjust the spacing between the color bar and the heatmap
            fig.subplots_adjust(top=cbar_bottom - 0.05)

        # Save the image with padding to add borders
        output_path = f'data/heatmap_{pass_num}.png'
        plt.savefig(output_path, bbox_inches='tight', pad_inches=0.1)
        plt.close(fig)
        print(f"Pass {pass_num} completed, image saved to {output_path}")

    except MemoryError:
        print(f"MemoryError occurred at pass {pass_num}. Stopping script.")
        break
    except Exception as e:
        print(f"An error occurred at pass {pass_num}: {e}")
        break

print(f"Largest Value: {largest}")
