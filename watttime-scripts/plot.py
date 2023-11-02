import pandas as pd
import geopandas as gpd
import matplotlib.pyplot as plt

# Load the solar data from CSV file into a Pandas DataFrame
# Assuming the file is named 'carbon_credits_sweep.csv'
solar_data = pd.read_csv('carbon_credits_sweep.csv')

# Load the US states border GeoJSON into a GeoPandas GeoDataFrame
# Assuming the file is named 'us_state_outlines.json'
us_states = gpd.read_file('us_state_outlines.json')

def create_heatmap(solar_data, us_states):
    """
    Create a heatmap with US states overlayed.
    
    Parameters:
        solar_data (pd.DataFrame): DataFrame containing solar data
        us_states (gpd.GeoDataFrame): GeoDataFrame containing US state borders
    
    Returns:
        None
    """
    
    # Create a GeoDataFrame from the solar data
    gdf = gpd.GeoDataFrame(solar_data, 
                            geometry=gpd.points_from_xy(solar_data.Longitude, solar_data.Latitude))

    # Setting the Coordinate Reference System (CRS) for both GeoDataFrames
    us_states = us_states.to_crs(epsg=4326)
    gdf = gdf.set_crs(epsg=4326)

    # Plotting
    fig, ax = plt.subplots(1, figsize=(15, 25))

    # Plotting the US states
    us_states.boundary.plot(ax=ax, linewidth=1, color="black")
    
    # Plotting the heatmap
    gdf.plot(column='Carbon Credits per Year per KW', 
             cmap='coolwarm_r', 
             ax=ax, 
             legend=True, 
             legend_kwds={'label': "Carbon Credits per Year per KW"},
             markersize=10, 
             missing_kwds={"color": "lightgrey"})

    plt.title("Heatmap of Carbon Credits per KW per Year")
    plt.xlabel("Longitude")
    plt.ylabel("Latitude")
    plt.show()

# Call the function to create the heatmap
create_heatmap(solar_data, us_states)

