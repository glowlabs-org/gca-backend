# This is a registration script, and only needs to be run once per user.
import requests

def register(username, password, email):
    register_url = 'https://api.watttime.org/register'
    params = {'username': username,
            'password': password,
            'email': email,
            'org': 'Glow International, Inc.'}
    rsp = requests.post(register_url, json=params)
    if rsp.status_code != 200:
        print(rsp.status_code, rsp.text)

if __name__ == "__main__":
    # Load username and password from files
    with open('username', 'r') as f:
        username = f.read().strip()
    with open('password', 'r') as f:
        password = f.read().strip()
    with open('email', 'r') as f:
        email = f.read().strip()

    print(f'Register {username} using V3 api')
    register(username, password, email)
