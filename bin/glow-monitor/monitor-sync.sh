#!/bin/bash

# Define the path for the timestamp file.
TIMESTAMP_FILE="/opt/glow-monitor/last-sync.txt"
touch $TIMESTAMP_FILE

reset_modem() {
    # Define the GPIO pin
    GPIO_PIN=12

    # Export the GPIO pin if it hasn't been exported already
    if [ ! -d "/sys/class/gpio/gpio$GPIO_PIN" ]; then
      echo "$GPIO_PIN" > /sys/class/gpio/export
    fi

    # Set the GPIO pin direction to output
    echo "out" > /sys/class/gpio/gpio$GPIO_PIN/direction

    # Drive the GPIO pin low to reset the modem
    echo "0" > /sys/class/gpio/gpio$GPIO_PIN/value

    # Wait for 3 seconds
    sleep 3

    # Release the GPIO pin (set it back to high)
    echo "1" > /sys/class/gpio/gpio$GPIO_PIN/value

    # Optionally, set the GPIO direction back to input (clean up)
    echo "in" > /sys/class/gpio/gpio$GPIO_PIN/direction

    # Unexport the GPIO pin (optional cleanup)
    echo "$GPIO_PIN" > /sys/class/gpio/unexport

    echo "Modem reset complete."
}

# Write a function that will reboot the system if the timestamp is more than 24
# hours old.
check_timestamp() {
    # See how long it has been since the last successful sync.
    current_time=$(date +%s)
    last_timestamp=$(cat "$TIMESTAMP_FILE")
    time_diff=$((current_time - last_timestamp))

    # See how long the system has been up.
    uptime_seconds=$(awk '{print int($1)}' /proc/uptime)

    # Reboot the system if the timestamp of the last successful sync is more
    # than 24 hours old, and also the system has had more than 10 hours of
    # uptime. We check that the system has had 10 hours of uptime in case this
    # service got restarted at some point while the system was operating. 10
    # hours of system uptime is enough for 2 sync operations, if that much time
    # has passed without a successful sync, it means the latest reboot probably
    # didn't work and another reboot should be attempted.
    if [ "$uptime_seconds" -gt 36000 ] && [ "$time_diff" -gt 86400 ]; then
        # Send a command to the glow-monitor service to dump its logs.
        pid=$(pidof glow-monitor)
        if [ -n "$pid" ]; then
            kill -USR1 "$pid"
        fi

        # Reboot system. This starts by power cycling the USBs.
        echo "powering off usb devices to power cycle the usb bus"
        echo 0 > /sys/devices/platform/soc/3f980000.usb/buspower
        sleep 10
        echo "rebooting the system because there has not been a successful sync in the past 24 hours"
        echo $(date) >> /opt/glow-monitor/reboots.txt
        reset_modem
        reboot
    fi
}

# Set up a loop to check the timestamp every 2 hours
echo "starting $TIMESTAMP_FILE monitor"
while true; do
    check_timestamp
    sleep 7200 # Sleep for 2 hours (7200 seconds)
done
