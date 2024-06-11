#!/bin/bash

# Define the path for the timestamp file and log file
TIMESTAMP_FILE="/dev/shm/last-udp.txt"
touch $TIMESTAMP_FILE

# Function to check the age of the timestamp
check_timestamp() {
    current_time=$(date +%s)
    last_timestamp=$(cat "$TIMESTAMP_FILE")
    time_diff=$((current_time - last_timestamp))

    # Restart the glow_monitor service if the timestamp is older than 900
    # seconds. This means that for some reason, the glow-monitor binary hasn't
    # attempted to send a udp packet in at least 900 seconds, but the
    # glow-monitor binary should be attempting this every 5 minutes.
    if [ "$time_diff" -gt 900 ]; then
        # Restart the glow_monitor service
        echo "restarting glow_monitor.service because the udp file is 15 minutes old"
        echo $current_time > $TIMESTAMP_FILE
	echo $(date) >> /dev/shm/glow-monitor-reset.txt
        systemctl restart glow_monitor.service
    fi
}

# Set up a loop to check the timestamp every 4 minutes.
echo "starting $TIMESTAMP_FILE monitor"
while true; do
    check_timestamp
    sleep 240 # Sleep for 4 minutes (240 seconds)
done
