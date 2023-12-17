#!/bin/bash

# This file is meant to be used by technicians only, not by GCAs. It will
# generate a new temp key for the GCA and then transfer the temp pub key to the
# remote GCA servers.

go build # build the latest version of the admin tools
./gca-admin # run the tools so that the gca temp keys get created if they don't exist yet
scp ~/.config/gca-data/gcaTempPubKey.dat $1:/home/user/gca-server/gcaTempPubKey.dat # upload the temp keys to the server
