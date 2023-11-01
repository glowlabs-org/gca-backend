import os
import csv
from collections import defaultdict
from statistics import mean

def choose_directory():
    """
    List all directories in the current folder and ask the user to choose one.

    Returns:
    - str: The path to the selected folder.
    """
    # Get a list of all entries in the current directory
    all_entries = os.listdir()

    # Filter only the directories
    directories = [entry for entry in all_entries if os.path.isdir(entry)]

    # List the directories and ask for user input
    print("Available directories:")
    for idx, directory in enumerate(directories):
        print(f"{idx+1}. {directory}")

    selected_idx = int(input("Enter the number corresponding to the directory you want to use: ")) - 1

    # Return the selected directory
    return directories[selected_idx]

def load_csv_files(folder_path):
    """
    Load all CSV files in a folder and return the MOER values organized by year and day.

    Args:
    - folder_path (str): The path to the folder containing the CSV files.

    Returns:
    - dict: A nested dictionary containing the MOER values organized by year and day.
    """
    data = defaultdict(lambda: defaultdict(list))
    for filename in os.listdir(folder_path):
        if filename.endswith('.csv'):
            filepath = os.path.join(folder_path, filename)
            with open(filepath, 'r') as csvfile:
                reader = csv.reader(csvfile)
                next(reader)
                for row in reader:
                    timestamp, moer = row[0], float(row[1])
                    year, rest = timestamp.split('-', 1)
                    day = timestamp.split('T')[0]
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

    for year, days in data.items():
        daily_low_avg = []
        daily_high_avg = []

        for day, moer_values in days.items():
            moer_values.sort()
            daily_low_avg.append(mean(moer_values[:30]))
            daily_high_avg.append(mean(moer_values[-120:]))
        
        yearly_low_avg[year] = mean(daily_low_avg)
        yearly_high_avg[year] = mean(daily_high_avg)

    return yearly_low_avg, yearly_high_avg

def main():
    """
    Main function to run the tasks.
    """
    # Let user choose the directory
    folder_path = choose_directory()

    # Load MOER values from CSV files
    data = load_csv_files(folder_path)

    # Calculate daily averages of the 64 lowest and 64 highest MOER values
    yearly_low_avg, yearly_high_avg = calculate_daily_averages(data)

    # Sort by year
    sorted_years = sorted(yearly_low_avg.keys())

    # Conversion factor
    conversion_factor = 5.38
    conversion_factor *= 365.25
    conversion_factor /= 1000
    conversion_factor /= 2204.62

    # Print results
    for year in sorted_years:
        low_avg = yearly_low_avg[year] * conversion_factor
        high_avg = yearly_high_avg[year] * conversion_factor
        print(year)
        print(f"Solar Only:              {low_avg:.3f} carbon credits per year, per kilowatt of solar ({yearly_low_avg[year]/2204.62:.3f})")
        print(f"Solar + Smart Batteries: {high_avg:.3f} carbon credits per year, per kilowatt of solar ({yearly_high_avg[year]/2204.62:.3f})")
        print()

if __name__ == "__main__":
    main()

