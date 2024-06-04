#!/bin/bash

# HOW TO USE:
#
# This is a preconfiguration script that is used to characterize the CT and
# make sure that the correct parity is used before a box goes live on the Glow
# protocol. For reason we don't full understand, some of the hardware reads the
# CTs backwards when it is installed. This script will check if that is the
# case for this box, and set the CT settings accordingly.
#
# STEP 1: Before running the script, connect the box and its CT to a power
# source in the correct orientation. Make sure the power is running so that the
# CT will be producing real readings.
#
# STEP 2: Run the script.
#
# STEP 3: The script will run for about 10 minutes. When it is done running, it
# will confirm with the user that the CT has now been disconnected. Do not say
# 'yes' to the script until the CT has actually been disconnected.
#
# STEP 4: The script will handle cleanup and the monitoring box should be good
# to go.

# Check that the correct parameters were provided.
if [ $# -lt 3 ]; then
        echo "Usage: $0 [equipment-ip] [ct-numerator] [ct-denominator]"
        exit 1
fi

# Function to check if the input is an integer
is_number() {
    if ! [[ $1 =~ ^[0-9]+$ ]]; then
        echo "Error: Input is not a valid number."
        exit 1
    fi
}
# Check that the ct values are integers
is_number $2
is_number $3

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

# First add the local ssh pubkey to the device so that all of these commands
# aren't asking for the password every time. Then change the password of the
# device so that it isn't just using the default password.
#
# The GCA is expected to have a custom password for the monitoring boxes.
retry_command "ssh-copy-id halki@$1"
sleep 1
retry_command "ssh halki@$1 \"echo 'halki:$(<.config/gca-data/clients/halki-password)' | sudo chpasswd\"" true
sleep 1

# Add the new halki-app firmware to the device so that the CT is reading the
# right values. The halki-app firmware cannot be updated unless the atm90e32
# service is stopped.
retry_command "ssh halki@$1 'sudo systemctl stop atm90e32.service'"
sleep 1
retry_command "scp .config/gca-data/clients/halki-app halki@$1:/home/halki"
sleep 4
retry_command "ssh halki@$1 'sudo mv /home/halki/halki-app /usr/bin/halki-app'"
sleep 4

# Get a confirmation from the user that the CT is set up and reading power.
echo "Initial firmware update is complete. Please set up the box to be reading from a power source of at least 50 watts."
echo "Type 'YES' when setup is complete:"
read confirmation
if [[ "${confirmation^^}" != "YES" ]]; then
        echo "Please run this script again when you are ready."
        exit 1
fi
echo "Proceeding with setup. This will take approximately 11 minutes."

# Clear the energy_data file and then start the sensor firmware. Clearing the
# file before starting the firmware ensures that any old readings (before the
# CT was confirmed to be set up correctly) get wiped. The test can't be
# performed if the atm90e32 service isn't running.
retry_command "ssh halki@$1 'echo \"timestamp,energy (mWh)\" | sudo tee /opt/halki/energy_data.csv > /dev/null'"
sleep 4
retry_command "ssh halki@$1 'sudo systemctl start atm90e32.service'"

# User has confirmed that setup is complete. We will now wait for 11 minutes to
# collect power readings. We need to wait for 10 minutes because the device
# only produces a reading every 5 minutes and we want at least two readings in
# the file. To cover the edge case where the atm90e32 service takes almost
# minute to get going, we wait an extra 55 seconds.
sleep 655

# There should be at least two data readings in the file. If the data is good,
# all energy readings will have the same sign, and all energy readings will
# have a value >1200. The next section of code goes through the file and
# validates the readings.

# Read the data skipping the header
data=$(ssh halki@$1 "tail -n +2 /opt/halki/energy_data.csv")

# Check that there are at least 2 energy readings.
line_count=$(echo "$data" | wc -l)
if [ "$line_count" -lt 2 ]; then
        echo "energy test failed, not enough energy readings collected"
        exit 1
fi

# Check on the data in the file. Check that every line has the same sign
# (either all positive or all negative), and also check that the power reading
# of every line is >1200.
first_sign=0
while IFS=, read -r timestamp energy; do
    # Convert the energy reading to its absolute value and an int.
    energy_abs=${energy#-}
    energy_int=${energy_abs%%.*}
    # Verify that the result is really an int.
    if ! [[ $energy_int =~ ^[0-9]+$ ]]; then
            echo "energy test failed, invalid energy reading: $energy"
            exit 1
    fi

    # Check if the absolute value of the energy is less than 1200 (1.2 watt hours in 5 minutes)
    if [ $energy_int -lt 1200 ]; then
        echo "energy test failed, energy signal is not strong enough"
        exit 1
    fi

    # Determine the sign of the current number.
    current_sign=$((${energy%%.*}>0?1:-1))

    # If this is the first number, set the sign. Otherwise, check that the sign
    # matches the first sign.
    if [ $first_sign -eq 0 ]; then
        first_sign=$current_sign
    else
        if [ $current_sign -ne $first_sign ]; then
            echo "energy test failed, energy signal is not consistent"
            exit 1
        fi
    fi
done <<< "$data"

# Get the final numerator based on the energy readings.
final_numerator=$((first_sign * $2))

# Create a file on the remote device with the ct-settings. The result should be
# a file in /opt/glow-monitor/ called 'ct-settings.txt' where the first line is
# the final numerator and the second line is the denomintor (provided as the
# third input to the script).
retry_command "ssh halki@$1 'echo $final_numerator | sudo tee /opt/glow-monitor/ct-settings.txt > /dev/null'"
sleep 1
retry_command "ssh halki@$1 'echo $3 | sudo tee -a /opt/glow-monitor/ct-settings.txt > /dev/null'"
sleep 1

# Setup is now complete, but we collected a bunch of false data in the
# energy_data.csv file. We need to make sure the user takes off the CT, and
# then we need to clear the energy_data.csv file to prevent bad data from
# making it into the device history after full setup is complete. Remember that
# the file can only be cleared correctly if the atm90e32 service is stopped.
retry_command "ssh halki@$1 'sudo systemctl stop atm90e32.service'"
sleep 4
retry_command "ssh halki@$1 'echo \"timestamp,energy (mWh)\" | sudo tee /opt/halki/energy_data.csv > /dev/null'"
sleep 4

echo "Data testing is complete. Please remove the CT and type 'DONE' when finished:"
read confirmation
if [[ "${confirmation^^}" != "DONE" ]]; then
        echo "Text does not equal 'DONE'. Assuming error. Please restart the script."
        exit 1
fi
retry_command "ssh halki@$1 'sudo systemctl start atm90e32.service'"
sleep 4
echo "CT configuration is complete. You may now proceed with equipment-setup."
