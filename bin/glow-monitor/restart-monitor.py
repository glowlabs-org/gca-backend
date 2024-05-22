# Monitors for a sync file, and issues a command to reboot if the sync time exceeds 24 hours.

import time
import signal
import sys
import os
from pathlib import Path
from datetime import datetime

_path = Path.home() / "last-sync.txt"

_reboot_delay_sec = 86400 # 24 hours
_check_delay_sec = 7200   # 2 hours
_reboot_cmd = "echo rebooting!"
_prod_reboot_cmd = "sudo reboot"

def signal_handler(sig, frame):
    sys.exit(0)

def update_timestamp():
    current_time = int(time.time())

    with open(_path, 'w') as file:
        file.write(str(current_time))

def check_timestamp():
    try:
        with open(_path, 'r') as file:
            saved_ts = int(file.read().strip())

        saved_time = datetime.fromtimestamp(saved_ts)

        current_time = time.time()

        current_ts = int(current_time)

        print(f"last sync {current_ts-saved_ts} ago")

        if (current_ts - saved_ts) > _reboot_delay_sec:
            os.system(_reboot_cmd)
            sys.exit(0)

    except FileNotFoundError:
        print(f"Error: The file {_path} does not exist.")
    except ValueError:
        print("Error: The content of 'time.txt' is not a valid integer.")

if __name__ == "__main__":
    if len(sys.argv) > 1 and sys.argv[1] == "production":
        _reboot_cmd = _prod_reboot_cmd
        print("starting in production mode, this service will reboot the system")
    else:
        print("starting in testing mode, this service will print a message instead of rebooting")
        if len(sys.argv) > 1:
            _check_delay_sec = int(sys.argv[1])
        if len(sys.argv) > 2:
            _reboot_delay_sec = int(sys.argv[2])

    signal.signal(signal.SIGINT, signal_handler)
    signal.signal(signal.SIGTERM, signal_handler)

    update_timestamp()

    try:
        while True:
            time.sleep(_check_delay_sec)
            check_timestamp()
    except SystemExit as e:
        pass
    except Exception as e:
        print(f"Unexpected error: {e}")