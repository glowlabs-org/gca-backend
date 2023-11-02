import pandas as pd
import geopandas as gpd
import matplotlib.pyplot as plt

# Load the solar data from CSV file into a Pandas DataFrame
solar_data = pd.read_csv('carbon_credits_sweep.csv')

# Load the US states border GeoJSON into a GeoPandas GeoDataFrame
us_states = gpd.read_file('us_state_outlines.json')

def create_heatmap(solar_data, us_states):
    """
    Create a heatmap with US states overlayed that only displays data within the continental US,
    and save it as a high-resolution PNG.

    Parameters:
        solar_data (pd.DataFrame): DataFrame containing solar data
        us_states (gpd.GeoDataFrame): GeoDataFrame containing US state borders

    Returns:
        None
    """

    # Define the bounding box for the continental US (approximate coordinates)
    bbox = {
        "minx": -124.848974,  # The westernmost point
        "maxx": -66.93457,    # The easternmost point
        "miny": 24.396308,    # The southernmost point
        "maxy": 49.384358     # The northernmost point
    }
    
    # Filter the solar data points that fall within the continental US
    solar_data = solar_data[
        (solar_data['Latitude'] >= bbox['miny']) &
        (solar_data['Latitude'] <= bbox['maxy']) &
        (solar_data['Longitude'] >= bbox['minx']) &
        (solar_data['Longitude'] <= bbox['maxx'])
    ]

    # Create a GeoDataFrame from the filtered solar data
    gdf = gpd.GeoDataFrame(solar_data, 
                            geometry=gpd.points_from_xy(solar_data.Longitude, solar_data.Latitude))

    # Setting the Coordinate Reference System (CRS) for both GeoDataFrames to WGS84
    us_states = us_states.to_crs(epsg=4326)
    gdf = gdf.set_crs(epsg=4326)

    # Plotting
    fig, ax = plt.subplots(1, figsize=(15, 25), dpi=300)
    
    # Define continental US bounding box
    continental_bbox = {'west': -125, 'east': -66.5, 'south': 24.4, 'north': 49.3843}
    
    # Clip the states' data to the bounding box of continental US
    us_states_clipped = us_states.cx[continental_bbox['west']:continental_bbox['east'], 
                                     continental_bbox['south']:continental_bbox['north']]
    
    # Plotting the clipped US states with a thinner border line
    us_states_clipped.boundary.plot(ax=ax, linewidth=0.2, color="black")
    
    # Plotting the heatmap
    gdf.plot(column='Carbon Credits per Year per KW', 
             cmap='coolwarm',  # Using the reversed gradient
             ax=ax, 
             legend=True, 
             legend_kwds={'label': "Carbon Credits per Year per KW"},
             markersize=0.5,  # Reduced marker size
             missing_kwds={"color": "lightgrey"})

    plt.title("Heatmap of Carbon Credits per KW per Year (Continental US)")
    plt.xlabel("Longitude")
    plt.ylabel("Latitude")

    # Save the plot as a high-resolution PNG
    plt.savefig("high_res_heatmap.png", dpi=800)  # dpi set to 800 for high resolution

    plt.show()

# Call the function to create and save the heatmap
create_heatmap(solar_data, us_states)

