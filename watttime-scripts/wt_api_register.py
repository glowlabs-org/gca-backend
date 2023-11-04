# This is a registration script, and only needs to be run once per user.
import requests

# Load username and password from files
with open('username', 'r') as f:
    username = f.read().strip()
with open('password', 'r') as f:
    password = f.read().strip()

register_url = 'https://api2.watttime.org/v2/register'
params = {'username': username,
         'password': password,
         'email': 'david@${username}.org',
         'org': 'Glow International, Inc.'}
rsp = requests.post(register_url, json=params)
print(rsp.text)
