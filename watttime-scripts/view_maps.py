import os
import geopandas as gpd
import matplotlib.pyplot as plt

def plot_boundaries():
    cur_dir = os.path.dirname(os.path.realpath('__file__'))
    file_path = os.path.join(cur_dir, 'data', 'ba_maps.json')
    
    # Load GeoJSON data into a GeoDataFrame
    gdf = gpd.read_file(file_path)

    # Plotting
    fig, ax = plt.subplots(figsize=(15, 15))  # You can adjust the size as needed
    gdf.boundary.plot(ax=ax, edgecolor='blue', linewidth=1)
    ax.set_title('Balancing Authority Boundaries')
    plt.show()

# Run the function
plot_boundaries()

