import pandas as pd
import geopandas as gpd
import matplotlib.pyplot as plt
from matplotlib.colors import LinearSegmentedColormap
from mpl_toolkits.axes_grid1 import make_axes_locatable

# Load the CSV file into a DataFrame
df = pd.read_csv('carbon_credits_sweep.csv')

# Drop rows where the 'Carbon Credits per Year per KW' column is missing
df = df.dropna(subset=['Carbon Credits per Year per KW'])

# Convert to GeoDataFrame for plotting the real data points
gdf = gpd.GeoDataFrame(
    df, geometry=gpd.points_from_xy(df.Longitude, df.Latitude)
)

# Load GeoJSON map
us_states = gpd.read_file('us_state_outlines.json')

# Define continental US bounding box
continental_bbox = {'west': -125, 'east': -66.5, 'south': 24.4, 'north': 49.3843}

# Clip the points to the bounding box of continental US
gdf_clipped = gdf.cx[continental_bbox['west']:continental_bbox['east'], 
                     continental_bbox['south']:continental_bbox['north']]

# Define the colors for the custom colormap (blue -> green -> yellow -> orange -> red)
colors = ['lime', 'yellow', 'orange', 'red', 'purple', 'black']
cmap_name = 'custom_colormap'
n_bins = 300  # Increase this number to have more fine transitions between colors
custom_colormap = LinearSegmentedColormap.from_list(cmap_name, colors, N=n_bins)

# Manually define the range for 'Carbon Credits per Year per KW' for normalization
vmin = 0  # Start of the colorbar range
vmax = 2  # End of the colorbar range

# Create a ScalarMappable with the new vmin and vmax
sm = plt.cm.ScalarMappable(cmap=custom_colormap, norm=plt.Normalize(vmin=vmin, vmax=vmax))
sm.set_array([])  # Set the array for the ScalarMappable to an empty list.

# Plotting
fig, ax = plt.subplots(1, figsize=(15, 25), dpi=300)

# Plotting the clipped US states with a thinner border line
us_states.cx[continental_bbox['west']:continental_bbox['east'], 
             continental_bbox['south']:continental_bbox['north']].boundary.plot(ax=ax, linewidth=0.25, color="black")

# Plotting the real data points within the bounding box
# The `vmin` and `vmax` in the `plot` function will now set the scale for the color map.
gdf_clipped.plot(column='Carbon Credits per Year per KW', 
         cmap=custom_colormap,
         ax=ax, 
         marker='o',
         markersize=5.25,  # Size of the markers
         vmin=vmin,  # Minimum value for colormap scaling
         vmax=vmax)  # Maximum value for colormap scaling

# Add color bar
divider = make_axes_locatable(ax)
cax = divider.append_axes("right", size="5%", pad=0.1)

# Create a colorbar in the axes cax
cbar = fig.colorbar(sm, cax=cax)

cbar.set_label('Carbon Credits per Year per KW')

# Set the aspect of the color bar
cbar.ax.set_aspect(20)

# Show/save the plot
plt.savefig('heatmap_raw.png', bbox_inches='tight', dpi=400)

