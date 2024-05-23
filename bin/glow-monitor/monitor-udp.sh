#!/bin/bash

# Define the path for the timestamp file and log file
TIMESTAMP_FILE="/dev/shm/last-udp.txt"

# Ensure the timestamp file exists and has the current timestamp
touch "$TIMESTAMP_FILE"
echo "$(date +%s)" > "$TIMESTAMP_FILE"

# Function to update timestamp
update_timestamp() {
    echo "$(date +%s)" > "$TIMESTAMP_FILE"
}

# Function to check the age of the timestamp
check_timestamp() {
    current_time=$(date +%s)
    last_timestamp=$(cat "$TIMESTAMP_FILE")
    time_diff=$((current_time - last_timestamp))

    # If the timestamp is older than 15 minutes (900 seconds)
    if [ "$time_diff" -gt 900 ]; then
        # Dump logs
        pid=$(pidof glow-monitor)
        if [ -n "$pid" ]; then
            kill -USR1 "$pid"
        fi

        # Update timestamp before rebooting
        update_timestamp

        # Reboot system
        echo "rebooting system as $TIMESTAMP_FILE was not updated"
        /sbin/reboot
    fi
}

# Set up a loop to check the timestamp every 4 minutes
echo "starting $TIMESTAMP_FILE monitor"
while true; do
    check_timestamp
    sleep 240 # Sleep for 4 minutes (240 seconds)
done
