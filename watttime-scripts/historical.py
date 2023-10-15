from os import path
import requests
from requests.auth import HTTPBasicAuth

# Load username and password from files
with open('username', 'r') as f:
    username = f.read().strip()
with open('password', 'r') as f:
    password = f.read().strip()

login_url = 'https://api2.watttime.org/v2/login'
token = requests.get(login_url, auth=HTTPBasicAuth(username, password)).json()['token']

historical_url = 'https://api2.watttime.org/v2/historical'
headers = {'Authorization': 'Bearer {}'.format(token)}
ba = 'CAISO_NORTH'
params = {'ba': ba}
rsp = requests.get(historical_url, headers=headers, params=params)
cur_dir = path.dirname(path.realpath('__file__'))
file_path = path.join(cur_dir, '{}_historical.zip'.format(ba))
with open(file_path, 'wb') as fp:
    fp.write(rsp.content)

print('Wrote historical data for {} to {}'.format(ba, file_path))
