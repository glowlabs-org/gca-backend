import requests

def load_credentials(filename):
    """
    Load user credentials and other details from a file.

    Parameters:
        filename (str): The name of the file containing the credentials.

    Returns:
        dict: A dictionary containing all the loaded credentials and details.
    """
    creds = {}
    with open(filename, 'r') as f:
        lines = f.readlines()

    creds['full_name'] = lines[0].strip()
    creds['email'] = lines[1].strip()
    creds['affiliation'] = lines[2].strip()
    creds['api_key'] = lines[3].strip()

    return creds

def get_user_coordinates():
    """
    Prompt the user for latitude and longitude coordinates.

    Returns:
        tuple: A tuple containing latitude and longitude as strings.
    """
    coords = input("Enter latitude and longitude separated by a comma (e.g. 30.294012, -97.640115): ")
    latitude, longitude = map(str.strip, coords.split(","))
    return latitude, longitude

def fetch_solar_data(creds, latitude, longitude):
    """
    Fetch solar data from the API based on user credentials and coordinates.

    Parameters:
        creds (dict): A dictionary containing user credentials.
        latitude (str): Latitude coordinate.
        longitude (str): Longitude coordinate.

    Returns:
        str: The API response as a text string.
    """
    # Include API key in the URL
    url = f"http://developer.nrel.gov/api/nsrdb/v2/solar/psm3-2-2-download.json?api_key={creds['api_key']}"

    # Format payload as a URL-encoded string
    payload = f"names=2021&leap_day=false&interval=60&utc=false&full_name=Honored%2BUser&email=honored.user%40gmail.com&affiliation=NREL&mailing_list=true&reason=Academic&attributes=dhi%2Cdni%2Cwind_speed%2Cair_temperature&wkt=MULTIPOINT(-106.22%2032.9741%2C-106.18%2032.9741%2C-106.1%2032.9741)"
    
    #"api_key={creds['api_key']}&attributes=ghi&years=2022&utc=true&leap_day=true&interval=15&email={creds['email']}&location_ids=POINT({longitude} {latitude})"
    
    headers = {
        'content-type': "application/x-www-form-urlencoded",
        'cache-control': "no-cache"
    }
    
    # Print the full query URL and payload for debugging
    print("Full Query URL:", url)
    print("Payload:", payload)

    # Make the API request
    response = requests.request("POST", url, data=payload, headers=headers)

    return response.text

def main():
    """
    Main function to execute the entire flow.
    """
    creds = load_credentials('nsrdb_data.dat')
    latitude, longitude = get_user_coordinates()
    result = fetch_solar_data(creds, latitude, longitude)
    print("API Response:")
    print(result)

if __name__ == "__main__":
    main()

