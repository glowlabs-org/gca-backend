# This is a request to get recent historical data for a location.
# This request only provides 32 days of data. Use historical.py to
# get more data than that.
import requests
import json
from requests.auth import HTTPBasicAuth

# Load username and password from files
with open('username', 'r') as f:
    username = f.read().strip()
with open('password', 'r') as f:
    password = f.read().strip()

login_url = 'https://api2.watttime.org/v2/login'
token = requests.get(login_url, auth=HTTPBasicAuth(username, password)).json()['token']

data_url = 'https://api2.watttime.org/v2/data'
headers = {'Authorization': 'Bearer {}'.format(token)}
params = {'ba': 'CAISO_NORTH', 
          'starttime': '2023-09-16T20:30:00-0800', 
          'endtime': '2023-10-13T20:45:00-0800'}
rsp = requests.get(data_url, headers=headers, params=params)
for data in datas:
    print(data['value'])
