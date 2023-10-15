# This is a login script, and the tokens that it provides are only valid for 30
# minutes. Therefore any process will need to get new tokens every 30 minutes,
# and probably best practice is to get new tokens every 5 minutes.
import requests
from requests.auth import HTTPBasicAuth

# Load username and password from files
with open('username', 'r') as f:
    username = f.read().strip()
with open('password', 'r') as f:
    password = f.read().strip()

login_url = 'https://api2.watttime.org/v2/login'
rsp = requests.get(login_url, auth=HTTPBasicAuth(username, password))
print(rsp.json())
