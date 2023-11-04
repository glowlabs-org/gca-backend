# index.py gets the actual carbon data for an area
import requests
from requests.auth import HTTPBasicAuth

# Load username and password from files
with open('username', 'r') as f:
    username = f.read().strip()
with open('password', 'r') as f:
    password = f.read().strip()

login_url = 'https://api2.watttime.org/v2/login'
token = requests.get(login_url, auth=HTTPBasicAuth(username, password)).json()['token']

index_url = 'https://api2.watttime.org/index'
headers = {'Authorization': 'Bearer {}'.format(token)}
params = {'ba': 'CAISO_NORTH', 'style': 'moer'}
rsp=requests.get(index_url, headers=headers, params=params)
print(rsp.text)
