#!/bin/bash

# This file is meant to be used by technicians only, not by GCAs. It will
# generate a new temp key for the GCA and then transfer the temp pub key to the
# remote GCA servers.

# Call this script using `./server-setup.sh [ip-address]`

# Build the basic tooling.
./gca-admin # run the tools so that the gca temp keys get created if they don't exist yet

# Create a 'user' on the server.
ssh root@$1 'apt update'
sleep 4
ssh root@$1 'apt upgrade'
sleep 4
ssh root@$1 'apt install tmux'
sleep 4
ssh root@$1 'adduser user --gecos ""'
sleep 4

# Get our pubkey to the user
ssh-copy-id user@$1
sleep 4

# Create a gca-server directory
ssh user@$1 'mkdir -p /home/user/gca-server'
sleep 4

# Transfer a bunch of critical files over.
scp ~/.config/gca-data/gcaTempPubKey.dat user@$1:/home/user/gca-server/gcaTempPubKey.dat
sleep 4
scp -r ~/.config/gca-data/watttime_data user@$1:/home/user/gca-server/watttime_data
sleep 4
scp -r ~/.config/gca-data/gca-server user@$1:/home/user/gca-server/gca-server
sleep 4

# Add execution permissions to gca-server
ssh user@$1 'chmod +x /home/user/gca-server/gca-server'
