#!/bin/bash

# Check that the correct parameters were provided.
if [ $# -lt 2 ]; then
	echo "Usage: $0 [short-id] [equipment-ip]"
	exit 1
fi

# Function to check if the input is an integer
is_number() {
    if ! [[ $1 =~ ^-?[0-9]+$ ]]; then
        echo "Error: Input is not a valid number."
        exit 1
    fi
}
# Check that the short-id is an integer.
is_number $1

# Function to retry command until it succeeds.
retry_command() {
    local command=$1
    local retry_interval=8
    local max_retries=8
    while [ $retry_count -lt $max_retries ]; do
        echo "Attempting to run command: $command"
        eval $command
        local status=$?
        if [ $status -eq 0 ]; then
            echo "Command succeeded: $command"
            break
        else
            echo "Command failed with status $status, retrying in $retry_interval seconds..."
            sleep $retry_interval
	    ((retry_count++))
        fi
    done
}

# Copy over all of the custom files for the device.
retry_command "scp .config/gca-data/clients/client_$1/* halki@$2:/home/halki/"
sleep 4
retry_command "ssh halki@$2 'sudo systemctl stop glow_monitor.service'"
sleep 1
retry_command "scp .config/gca-data/clients/glow-monitor halki@$2:/home/halki"
sleep 4
retry_command "scp .config/gca-data/clients/monitor-sync.service halki@$2:/home/halki"
sleep 4
retry_command "scp .config/gca-data/clients/monitor-sync.sh halki@$2:/home/halki"
sleep 4
retry_command "scp .config/gca-data/clients/monitor-udp.service halki@$2:/home/halki"
sleep 4
retry_command "scp .config/gca-data/clients/monitor-udp.sh halki@$2:/home/halki"
sleep 4

# Move all of the files that were copied over to their intended destination.
retry_command "ssh halki@$2 'sudo mv /home/halki/glow-monitor /usr/bin/glow-monitor'"
sleep 4
retry_command "ssh halki@$2 'sudo mv /home/halki/monitor-sync.sh /usr/bin/monitor-sync.sh'"
sleep 4
retry_command "ssh halki@$2 'sudo mv /home/halki/monitor-udp.sh /usr/bin/monitor-udp.sh'"
sleep 4
retry_command "ssh halki@$2 'sudo mv /home/halki/monitor-sync.service /etc/systemd/system/monitor-sync.service'"
sleep 4
retry_command "ssh halki@$2 'sudo mv /home/halki/monitor-udp.service /etc/systemd/system/monitor-udp.service'"
sleep 4
retry_command "ssh halki@$2 'sudo mv /home/halki/* /opt/glow-monitor/'"
sleep 4

# Set up the systemd services that keep the devices stable.
retry_command "ssh halki@$2 'sudo systemctl daemon-reload'"
sleep 4
retry_command "ssh halki@$2 'sudo systemctl start glow_monitor.service'"
sleep 1
retry_command "ssh halki@$2 'sudo systemctl enable monitor-sync.service'"
sleep 1
retry_command "ssh halki@$2 'sudo systemctl enable monitor-udp.service'"
sleep 1
retry_command "ssh halki@$2 'sudo systemctl start monitor-sync.service'"
sleep 1
retry_command "ssh halki@$2 'sudo systemctl start monitor-udp.service'"
sleep 1

echo "Hardware setup is complete"
