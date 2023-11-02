import pandas as pd
import geopandas as gpd
import matplotlib.pyplot as plt
from matplotlib.colors import LinearSegmentedColormap
from matplotlib.cm import ScalarMappable
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
colors = ['blue', 'green', 'yellow', 'orange', 'red']
cmap_name = 'custom_colormap'
n_bins = 50  # Increase this number to have more fine transitions between colors
custom_colormap = LinearSegmentedColormap.from_list(cmap_name, colors, N=n_bins)

# Calculate the range for 'Carbon Credits per Year per KW' within the bounded area
credit_values_within_bounds = gdf_clipped['Carbon Credits per Year per KW']
vmin = credit_values_within_bounds.min()
vmax = credit_values_within_bounds.max()

# Create a ScalarMappable to be used for legend
sm = ScalarMappable(cmap=custom_colormap, norm=plt.Normalize(vmin=vmin, vmax=vmax))
sm._A = []  # Dummy array for the ScalarMappable. Required for the next step.

# Plotting
fig, ax = plt.subplots(1, figsize=(15, 25), dpi=300)

# Plotting the clipped US states with a thinner border line
us_states.cx[continental_bbox['west']:continental_bbox['east'], 
             continental_bbox['south']:continental_bbox['north']].boundary.plot(ax=ax, linewidth=0.25, color="black")

# Plotting the real data points within the bounding box
gdf_clipped.plot(column='Carbon Credits per Year per KW', 
         cmap=custom_colormap,
         ax=ax, 
         marker='o',  # Circle markers
         markersize=10)  # Size of the markers

# Add color bar
# Create an axes on the right side of ax. The width of cax will be 5%
# of ax and the height will be matched to the bounding box of the continental US.
divider = make_axes_locatable(ax)
cax = divider.append_axes("right", size="5%", pad=0.1)

# Create a colorbar in the axes cax with the specified tick locations
cbar = plt.colorbar(sm, cax=cax)

cbar.set_label('Carbon Credits per Year per KW')

# Set the aspect of the color bar
cbar.ax.set_aspect(20)

# Show/save the plot
plt.savefig('heatmap_raw.png', bbox_inches='tight', dpi=200)
