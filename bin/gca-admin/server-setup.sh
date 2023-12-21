#!/bin/bash

# This file is meant to be used by technicians only, not by GCAs. It will
# generate a new temp key for the GCA and then transfer the temp pub key to the
# remote GCA servers.

# Call this script using `./server-setup.sh [ip-address]`

# Build the basic tooling.
go build # build the latest version of the admin tools
./gca-admin # run the tools so that the gca temp keys get created if they don't exist yet

# Create a 'user' on the server.
ssh root@$1 'apt update'
ssh root@$1 'apt upgrade'
ssh root@$1 'apt install tmux'
ssh root@$1 'adduser user --gecos ""'

# Get our pubkey to the user
ssh-copy-id user@$1

# Create a gca-server directory
ssh user@$1 'mkdir -p /home/user/gca-server'

# Transfer a bunch of critical files over.
scp ~/.config/gca-data/gcaTempPubKey.dat user@$1:/home/user/gca-server/gcaTempPubKey.dat
scp -r ~/.config/gca-data/watttime_data user@$1:/home/user/gca-server/watttime_data
scp -r ~/.config/gca-data/gca-server user@$1:/home/user/gca-server/gca-server

# Add execution permissions to gca-server
ssh user@$1 'chmod +x /home/user/gca-server/gca-server'
