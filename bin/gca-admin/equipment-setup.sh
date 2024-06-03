#!/bin/bash

# Check that the correct parameters were provided.
if [ $# -lt 2 ]; then
        echo "Usage: $0 [short-id] [equipment-ip]"
        exit 1
fi

# Function to check if the input is an integer
is_number() {
    if ! [[ $1 =~ ^[0-9]+$ ]]; then
        echo "Error: Input is not a valid number."
        exit 1
    fi
}
# Check that the short-id is an integer.
is_number $1

# Function to retry command until it succeeds
retry_command() {
    local command=$1
    local suppress="${2:-false}"
    local retry_interval=8
    local max_retries=20
    local retry_count=0
    while [ $retry_count -lt $max_retries ]; do
        if [ "$suppress" = false ]; then
            echo "Attempting to run command: $command"
        else
            echo "Attempting to run a sensitive command"
        fi
        eval $command
        local status=$?
        if [ $status -eq 0 ]; then
            echo "Command succeeded"
            return 0
        else
            echo "Command failed with status $status, retrying in $retry_interval seconds..."
            sleep $retry_interval
            ((retry_count++))
        fi
    done
    echo "Error: maximum retries reached for command. Script has failed, please try running it again."
    exit 1
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

# Wait for 15 seconds to give the glow-monitor service time to start up and
# error out. Then check whether the service is running. If not, report the
# error and quit.
sleep 15
monitor_status=$(ssh halki@$2 'sudo systemctl is-active glow_monitor.service')
if [ "$monitor_status" != "active" ]; then
    echo "Error: glow-monitor did not start, setup appears to have FAILED. Please run the script again"
    exit 1
fi

echo "Hardware setup is complete"
