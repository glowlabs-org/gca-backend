# historic-averages.py computes a very rough estimate for how
# many carbon credits a solar panel would produce per MWh of
# electricity generated.

import os
import csv
from collections import defaultdict
from statistics import mean

def load_csv_files(folder_path):
    """
    Load all CSV files in a folder and return the MOER values organized by year and day.

    Args:
    - folder_path (str): The path to the folder containing the CSV files.

    Returns:
    - dict: A nested dictionary containing the MOER values organized by year and day.
    """
    # Initialize a nested dictionary to hold MOER values organized by year and day
    data = defaultdict(lambda: defaultdict(list))

    # Iterate through each CSV file in the folder
    for filename in os.listdir(folder_path):
        if filename.endswith('.csv'):
            filepath = os.path.join(folder_path, filename)
            
            # Open and read the CSV file
            with open(filepath, 'r') as csvfile:
                reader = csv.reader(csvfile)
                next(reader)  # Skip the header row

                for row in reader:
                    timestamp, moer = row[0], float(row[1])
                    year, rest = timestamp.split('-', 1)
                    day = timestamp.split('T')[0]

                    # Organize MOER values by year and day
                    data[year][day].append(moer)
                    
    return data

def calculate_daily_averages(data):
    """
    Calculate the average of the 64 lowest and 64 highest MOER values for each day.

    Args:
    - data (dict): A nested dictionary containing the MOER values organized by year and day.

    Returns:
    - dict: A dictionary containing the average of the 64 lowest and highest MOER values for each year.
    """
    yearly_low_avg = {}
    yearly_high_avg = {}

    # Iterate through each year
    for year, days in data.items():
        daily_low_avg = []
        daily_high_avg = []

        # Iterate through each day
        for day, moer_values in days.items():
            moer_values.sort()
            
            # Calculate the average of the 64 lowest MOER values for the day
            daily_low_avg.append(mean(moer_values[:30]))
            
            # Calculate the average of the 64 highest MOER values for the day
            daily_high_avg.append(mean(moer_values[-120:]))
        
        # Calculate the average of the daily low averages for the year
        yearly_low_avg[year] = mean(daily_low_avg)
        
        # Calculate the average of the daily high averages for the year
        yearly_high_avg[year] = mean(daily_high_avg)

    return yearly_low_avg, yearly_high_avg

def main():
    """
    Main function to run the tasks.
    """
    # Load MOER values from CSV files
    data = load_csv_files('CAISO_NORTH')

    # Calculate daily averages of the 64 lowest and 64 highest MOER values
    yearly_low_avg, yearly_high_avg = calculate_daily_averages(data)

    # Sort by year
    sorted_years = sorted(yearly_low_avg.keys())

    # Conversion factor
    conversion_factor = 5.38 # peak sunlight hours per year, California
    conversion_factor *= 365.25 # days per year
    conversion_factor /= 1000 # kilowatt hours per megawatt hour
    conversion_factor /= 2204.62 # pounds per metric ton of co2

    # Print the results in a sorted manner
    for year in sorted_years:
        low_avg = yearly_low_avg[year] * conversion_factor
        high_avg = yearly_high_avg[year] * conversion_factor
        print(year)
        print(f"Solar Only:              {low_avg:.3f} carbon credits per year, per kilowatt of solar")
        print(f"Solar + Smart Batteries: {high_avg:.3f} carbon credits per year, per kilowatt of solar")
        print()

if __name__ == "__main__":
    main()
