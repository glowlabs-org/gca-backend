import pandas as pd
import geopandas as gpd
import matplotlib.pyplot as plt
from matplotlib.colors import LinearSegmentedColormap

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

# Plotting the real data points
gdf.plot(column='Carbon Credits per Year per KW', 
         cmap=custom_colormap,
         ax=ax, 
         legend=True, 
         legend_kwds={'label': "Carbon Credits per Year per KW"},
         marker='o',  # Circle markers
         markersize=5)  # Size of the markers
         
# Show/save the plot
plt.savefig('heatmap_raw.png', bbox_inches='tight', dpi=200)

