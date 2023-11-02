import pandas as pd
import numpy as np
import geopandas as gpd
from scipy.spatial import KDTree
import matplotlib.pyplot as plt
from matplotlib.colors import LinearSegmentedColormap

# Load the CSV file into a DataFrame
df = pd.read_csv('carbon_credits_sweep.csv')

# Drop rows where the 'Carbon Credits per Year per KW' column is missing
df = df.dropna(subset=['Carbon Credits per Year per KW'])

# Load GeoJSON map
us_states = gpd.read_file('us_state_outlines.json')

# Extracting relevant coordinates and values
coordinates = df[['Latitude', 'Longitude']].values
values = df['Carbon Credits per Year per KW'].values

# Create a KDTree for quick nearest-neighbor lookup
tree = KDTree(coordinates)

# Generate a high-resolution grid for plotting
# The following limits should be set according to the actual data range you're interested in
lat_range = np.arange(24.4, 49.3843, 0.125)  # Adjust step for desired resolution
lon_range = np.arange(-125, -66.5, 0.125)    # Adjust step for desired resolution

# Prepare a list to store the interpolated points
interpolated_data = []

# Iterate over the grid
for lat in lat_range:
    for lon in lon_range:
        # Find the distance to and the indices of the nearest four neighbors
        distances, indices = tree.query([(lat, lon)], k=4)
        
        # Retrieve the values of the nearest neighbors
        neighbor_values = values[indices[0]]
        
        # Check if the closest neighbor is more than 1 unit distance away
        if distances[0][0] > 0.75:
            avg_value = None
        else:
            # Compute weights as the inverse of distance squared, avoiding division by zero
            weights = 1 / np.maximum(distances[0], 0.0001) ** 2
            
            # Compute the weighted average, ignoring NaNs
            # This is done by multiplying the neighbor values by their weights, summing them, and then dividing by the sum of weights
            if np.all(np.isnan(neighbor_values)):
                avg_value = np.nan
            else:
                weighted_sum = np.nansum(neighbor_values * weights)
                sum_of_weights = np.nansum(weights)
                avg_value = weighted_sum / sum_of_weights
        
        # Append the result to the list
        interpolated_data.append({
            'Latitude': lat, 
            'Longitude': lon, 
            'Average Value': avg_value
        })

# Create a DataFrame from the list of dictionaries
interpolated = pd.DataFrame(interpolated_data)

# Convert to GeoDataFrame for plotting
gdf = gpd.GeoDataFrame(
    interpolated, geometry=gpd.points_from_xy(interpolated.Longitude, interpolated.Latitude)
)

# Plotting
fig, ax = plt.subplots(1, figsize=(15, 25), dpi=300)

# Define continental US bounding box
continental_bbox = {'west': -125, 'east': -66.5, 'south': 24.4, 'north': 49.3843}

# Clip the states' data to the bounding box of continental US
us_states_clipped = us_states.cx[continental_bbox['west']:continental_bbox['east'], 
                                 continental_bbox['south']:continental_bbox['north']]

# Plotting the clipped US states with a thinner border line
us_states_clipped.boundary.plot(ax=ax, linewidth=0.25, color="black")

# Define the colors for the custom colormap (blue -> green -> yellow -> orange -> red)
colors = ['blue', 'green', 'yellow', 'orange', 'red']
# Create a colormap object based on the colors
cmap_name = 'custom_colormap'
n_bins = 300  # Increase this number to have more fine transitions between colors
custom_colormap = LinearSegmentedColormap.from_list(cmap_name, colors, N=n_bins)

# Plotting the heatmap with square markers
gdf.plot(column='Average Value', 
         cmap=custom_colormap,
         ax=ax, 
         legend=False, 
         legend_kwds={'label': "Average Carbon Credits per Year per KW"},
         marker='s',  # Square markers
         markersize=0.01,  # Size of the markers
         missing_kwds={"color": "white"})  # White color for missing data
         
# Get the bounds of the axes for the continental US after plotting
bounds = ax.get_position().bounds
left, bottom, width, height = bounds

# Create a new axes for the colorbar.
# The left position is determined by the width of the plotting axes plus a small offset.
# The bottom position is the same as the plotting axes.
# The width of the colorbar is a fraction of the plotting axes width.
# The height is the same as the plotting axes.
cbar_ax = fig.add_axes([left + width + 0.01, bottom, 0.02, height])

# Plotting the colorbar in the new axes with the height matched to the plotting axes
sm = plt.cm.ScalarMappable(cmap=custom_colormap, norm=plt.Normalize(vmin=gdf['Average Value'].min(), vmax=gdf['Average Value'].max()))
sm._A = []  # This is a workaround for Matplotlib's ScalarMappable which expects an array-like _A attribute
cbar = fig.colorbar(sm, cax=cbar_ax)

# Show/save the plot
plt.savefig('heatmap.png', bbox_inches='tight', dpi=200)

