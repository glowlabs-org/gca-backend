This folder contains all of the code required to generate a heatmap of the
world that showcases the emissions of the Glow protocol. There are a couple of
steps required to get the heatmap built.

The highres versions of the scripts require 30 GB of available RAM to complete.

1. Download the WattTime balancing authority map. You can do this by running `python wt_api_maps.py`. This script should complete in a minute or two.

2. Download the nasa data. You can do this by running `python heatmap-nasa-grid.py`. This process will take around 24 hours to complete. You may need to run the script multiple times to collect datapoints that are missed on the first pass. Subsequent runs of the script will be significantly faster, as they will only collect missing datapoints.

3. Download all of the historical data for WattTime. You can do this by running `python wt_api_hist_from_ba_map.py`. This script may take several hours to run, and may download 20+ GB of data.

4. Build the `solar_values` file. This is an in-between file that computes the solar impact for each datapoint. This script can take multiple hours to run and will keep the CPU at 100%. It requires 14 GB of available RAM to run. Run `python solar_value_map.py`

5. For the final step, run `python plot.py`, which will create multiple maps of increasing resolution from the data in the csv. The whole script should finish in a few minutes.
