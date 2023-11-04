import requests

# Load username and password from files
with open('username', 'r') as f:
    username = f.read().strip()
with open('password', 'r') as f:
    password = f.read().strip()

password_url = 'https://api2.watttime.org/v2/password/?username=${username}'
rsp = requests.get(password_url)
print(rsp.json())
