import time
import random

def manage_file(file_path):
    # Open and read the file
    with open(file_path, 'r') as file:
        lines = file.readlines()

    # Keep only the most recent 9 lines if there are more than 9 lines
    if len(lines) > 9:
        lines = lines[-9:]

    # Generate a random number
    rand_number = random.randint(1000, 2000000)

    # Get the current unix timestamp % 300
    new_unix_timestamp = int(time.time()) % 300

    # Add the new line
    lines.append(f"{new_unix_timestamp},{rand_number}\n")

    # Overwrite the file with the new lines
    with open(file_path, 'w') as file:
        file.writelines(lines)

def main():
    while True:
        manage_file("/opt/halki/energy_data.csv")
        time.sleep(300)  # Wait for 5 minutes
