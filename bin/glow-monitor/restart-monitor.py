# Monitors for a sync file, and issues a command to reboot if the sync time exceeds 24 hours.
# This script must be run with parameter "production" to enable it to issue a reboot command.

import time
import signal
import sys
import subprocess
from pathlib import Path
from datetime import datetime

_sync_file_path = Path.home() / "last-sync.txt"
_reboot_delay_sec = 86400 # 24 hours
_check_delay_sec = 7200   # 2 hours
_glow_monitor_kill_cmd = "sudo kill -usr2 `pidof glow-monitor`"
_reboot_cmd = "sudo reboot"
_production_mode = False

# signal_handler exits the script on a signal.
def signal_handler(sig, frame):
    sys.exit(0)


# update_timestamp writes the current timestamp to the sync file.
def update_timestamp():
    current_time = int(time.time())

    with open(_sync_file_path, 'w') as file:
        file.write(str(current_time))


# sync_threshold_handler handles processing when the threshold is exceeded.
# In production mode, this will issue a kill -usr2 to the glow monitor to
# create an event log dump file, and then will issue a reboot command.
# In testing mode, prints the commands it would run.
def testing_sync_threshold_handler():
    if _production_mode:
        cmd1 = _glow_monitor_kill_cmd
        cmd2 = _reboot_cmd
    else:
        cmd1 = f"echo '{_glow_monitor_kill_cmd}'"
        cmd2 = f"echo '{_reboot_cmd}'"
    
    print("sync threshold exceeded, issuing the following commands:")
    print(f"  {cmd1}")
    print(f"  {cmd2}")
    
    try:
        subprocess.run(cmd1, shell=True)
        subprocess.run(cmd2, shell=True)
    except subprocess.CalledProcessError as e:
        print(f"fatal error in subprocess call: {e.stderr}")
        sys.exit(1)


# check_timestamp opens the sync file and compares its timestamp to the current
# time. If they differ by a threshold, calls a command.
def check_timestamp():
    try:
        with open(_sync_file_path, 'r') as file:
            saved_ts = int(file.read().strip())

        saved_time = datetime.fromtimestamp(saved_ts)
        current_ts = int(time.time())
        current_time = datetime.fromtimestamp(current_ts)

        print(f"sync file time {saved_time} -- {current_time-saved_time} ago")

        if (current_ts - saved_ts) > _reboot_delay_sec:
            testing_sync_threshold_handler()

    except FileNotFoundError:
        print(f"fatal error: sync file {_sync_file_path} does not exist")
        sys.exit(1)
    except ValueError:
        print(f"fatal error: sync file {_sync_file_path} contents is not a valid timestamp")
        sys.exit(1)

if __name__ == "__main__":
    if len(sys.argv) > 1 and sys.argv[1] == "production":
        _production_mode = True
        print("\n*** WARNING ***\n")
        print(f"this service can reboot the system with command `{_reboot_cmd}`")
        if len(sys.argv) > 2:
            _check_delay_sec = int(sys.argv[2])
        if len(sys.argv) > 3:
            _reboot_delay_sec = int(sys.argv[3])
    else:
        print("testing mode: this service will print messages instead of rebooting")
        if len(sys.argv) > 1:
            _check_delay_sec = int(sys.argv[1])
        if len(sys.argv) > 2:
            _reboot_delay_sec = int(sys.argv[2])

    print(f"\nchecks sync file every {_check_delay_sec} seconds")
    print(f"reboots if sync is delayed for {_reboot_delay_sec} seconds")

    signal.signal(signal.SIGINT, signal_handler)
    signal.signal(signal.SIGTERM, signal_handler)

    # Update once at startup to ensure we don't have reboot loops.
    update_timestamp()

    try:
        while True:
            time.sleep(_check_delay_sec)
            check_timestamp()
    except SystemExit:
        pass
    except Exception as e:
        print(f"unexpected error {e}")
