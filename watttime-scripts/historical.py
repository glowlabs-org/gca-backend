import os
from os import path
import zipfile
import requests
from requests.auth import HTTPBasicAuth

def load_from_file(file_name):
    """
    Load content from a file.
    
    Parameters:
        file_name (str): The name of the file to read from.
    
    Returns:
        str: The stripped content read from the file.
    """
    with open(file_name, 'r') as f:
        return f.read().strip()

def update_gitignore(folder_name):
    """
    Update .gitignore to include the new folder if it's not already present.
    
    Parameters:
        folder_name (str): The name of the folder to ignore.
    """
    gitignore_path = '.gitignore'
    if not path.exists(gitignore_path):
        with open(gitignore_path, 'w') as f:
            f.write(folder_name + '\n')
    else:
        with open(gitignore_path, 'r+') as f:
            lines = f.readlines()
            if folder_name not in lines:
                f.write(folder_name + '\n')

# Get the Balancing Authority (ba) from the user
ba = input("Please enter the Balancing Authority (e.g., FPL): ")

# Check if data for the given 'ba' already exists locally
if path.exists(ba):
    print(f"Data for {ba} already exists locally.")
else:
    # Construct the login URL and get the token
    login_url = 'https://api2.watttime.org/v2/login'
    token = requests.get(login_url, auth=HTTPBasicAuth(username, password)).json()['token']

    # Construct the historical URL and the headers
    historical_url = 'https://api2.watttime.org/v2/historical'
    headers = {'Authorization': f'Bearer {token}'}
    params = {'ba': ba}

    # Make the request to get the historical data
    rsp = requests.get(historical_url, headers=headers, params=params)

    # Create a new directory for the 'ba' and save the zip file there
    if not os.path.exists(ba):
        os.mkdir(ba)
    zip_path = path.join(ba, f'{ba}_historical.zip')
    with open(zip_path, 'wb') as fp:
        fp.write(rsp.content)

    # Extract the zip file in the directory
    with zipfile.ZipFile(zip_path, 'r') as zip_ref:
        zip_ref.extractall(ba)

    # Update the .gitignore file
    update_gitignore(ba)

    # Notify the user that the data has been written and unzipped
    print(f'Wrote and unzipped historical data for {ba} to the directory: {ba}')

