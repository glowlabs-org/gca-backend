# This is a login script, and the tokens that it provides are only valid for 30
# minutes. Therefore any process will need to get new tokens every 30 minutes,
# and probably best practice is to get new tokens every 5 minutes.
# The script creates a 'token' file which can be used by other scripts
# If this file exists, it will be overwritten. The other scripts will
# fail if this token is expired, and this script can be called again to refresh it.
import requests
from requests.auth import HTTPBasicAuth

def login(username, password):
    """
    Log in to WattTime.

    Parameters:
        username (str): User name for the API.
        password (str): Password for the API.

    Returns:
        str: Valid token (or None if API call failed).
    """    
    login_url = 'https://api.watttime.org/login'
    rsp = requests.get(login_url, auth=HTTPBasicAuth(username, password))
    if rsp.status_code != 200:
        print(rsp.status_code, rsp.text)
        return None

    return rsp.json()['token']

if __name__ == "__main__":
    # Load username and password from files
    with open('username', 'r') as f:
        username = f.read().strip()
    with open('password', 'r') as f:
        password = f.read().strip()

    print(f'Login {username} using V3 api')
    tok = login(username, password)
    if tok is not None:
        with open('token', 'w') as f:
            f.write(tok)
        print('Login ok')
