#!/bin/bash

# Dumps event logs to either console or file.
# The console dump can be accessed via command journalctl -u glow_monitor.service
# The file is /opt/glow-monitor/status.txt

if [[ "$1" != "console" && "$1" != "file" ]]; then
    echo "usage: $0 console|file"
    exit 1
fi

if [[ -z "`pidof glow-monitor`" ]]; then
    echo "glow-monitor is not running"
    exit 1
fi

if [[ "$1" == "console" ]]; then
    set -x
    sudo kill -usr1 `pidof glow-monitor`
    set +x
else
    set -x
    sudo kill -usr2 `pidof glow-monitor`
    set +x
fi
