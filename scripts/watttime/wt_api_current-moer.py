# index.py gets the actual carbon data for an area
import requests
import json
from requests.auth import HTTPBasicAuth
from datetime import datetime, timedelta

# Get the current time
now = datetime.utcnow()
# Calculate the start time by subtracting 5 minutes
first_date = now - timedelta(minutes=5)
# Format the dates as specified by the API
date_start = first_date.strftime('%Y-%m-%dT%H:%M+00:00')
date_end = now.strftime('%Y-%m-%dT%H:%M+00:00')

# Load username and password from files
with open('username', 'r') as f:
    username = f.read().strip()
with open('password', 'r') as f:
    password = f.read().strip()

login_url = 'https://api.watttime.org/login'
token = requests.get(login_url, auth=HTTPBasicAuth(username, password)).json()['token']

index_url = 'https://api.watttime.org/v3/historical'
headers = {'Authorization': 'Bearer {}'.format(token)}
params = {'region': 'FPL', 'start': date_start, 'end': date_end, 'signal_type': 'co2_moer'}
rsp=requests.get(index_url, headers=headers, params=params)
if rsp.status_code == 200:
    print(json.dumps(rsp.json(), indent=2))
else:
    print(rsp.text)

