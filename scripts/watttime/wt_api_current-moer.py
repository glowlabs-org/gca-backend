# index.py gets the actual carbon data for an area
import requests
import json
from requests.auth import HTTPBasicAuth

# Load username and password from files
with open('username', 'r') as f:
    username = f.read().strip()
with open('password', 'r') as f:
    password = f.read().strip()

login_url = 'https://api.watttime.org/login'
token = requests.get(login_url, auth=HTTPBasicAuth(username, password)).json()['token']

index_url = 'https://api.watttime.org/v3/signal-index'
headers = {'Authorization': 'Bearer {}'.format(token)}
params = {'region': 'CAISO_NORTH', 'signal_type': 'co2_moer'}
rsp=requests.get(index_url, headers=headers, params=params)
if rsp.status_code == 200:
    print(json.dumps(rsp.json(), indent=2))
else:
    print(rsp.text)

