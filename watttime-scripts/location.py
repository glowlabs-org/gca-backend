# Location of Kurt's House: 27.62425177166974, -80.40863545263362
import requests
from requests.auth import HTTPBasicAuth

# Load username and password from files
with open('username', 'r') as f:
    username = f.read().strip()
with open('password', 'r') as f:
    password = f.read().strip()

login_url = 'https://api2.watttime.org/v2/login'
token = requests.get(login_url, auth=HTTPBasicAuth(username, password)).json()['token']

region_url = 'https://api2.watttime.org/v2/ba-from-loc'
headers = {'Authorization': 'Bearer {}'.format(token)}
params = {'latitude': '27.62425', 'longitude': '-80.4086'}
rsp=requests.get(region_url, headers=headers, params=params)
print(rsp.text)
