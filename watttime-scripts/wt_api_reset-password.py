import requests

def reset_password(username):
    password_url = f'https://api.watttime.org/password/?username={username}'
    rsp = requests.get(password_url)
    print(rsp.status_code, rsp.text)

if __name__ == "__main__":
    # Load username from file
    with open('username', 'r') as f:
        username = f.read().strip()

    print(f'Reset password for {username} using V3 api')
    reset_password(username)
