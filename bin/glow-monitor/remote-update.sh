if [ $# -lt 2 ]; then
	echo "Usage: $0 [port] [subdomain]"
	exit 1
fi

# Function to retry command until it succeeds
retry_command() {
    local command=$1
    local retry_interval=8
    while true; do
        echo "Attempting to run command: $command"
        eval $command
        local status=$?
        if [ $status -eq 0 ]; then
            echo "Command succeeded: $command"
            break
        else
            echo "Command failed with status $status, retrying in $retry_interval seconds..."
            sleep $retry_interval
        fi
    done
}

retry_command "ssh-copy-id -p $1 halki@$2.napter.soracom.io"
retry_command "scp -P $1 glow-monitor halki@$2.napter.soracom.io:~"
retry_command "scp -P $1 monitor-sync.service halki@$2.napter.soracom.io:~"
retry_command "scp -P $1 monitor-sync.sh halki@$2.napter.soracom.io:~"
retry_command "scp -P $1 monitor-udp.service halki@$2.napter.soracom.io:~"
retry_command "scp -P $1 monitor-udp.sh halki@$2.napter.soracom.io:~"
sleep 1
retry_command "ssh -p $1 halki@$2.napter.soracom.io 'sudo systemctl stop glow_monitor.service'"
sleep 1
retry_command "ssh -p $1 halki@$2.napter.soracom.io 'sudo mv /home/halki/glow-monitor /usr/bin/glow-monitor'"
sleep 1
retry_command "ssh -p $1 halki@$2.napter.soracom.io 'sudo mv /home/halki/monitor-sync.sh /usr/bin/monitor-sync.sh'"
sleep 1
retry_command "ssh -p $1 halki@$2.napter.soracom.io 'sudo mv /home/halki/monitor-udp.sh /usr/bin/monitor-udp.sh'"
sleep 1
retry_command "ssh -p $1 halki@$2.napter.soracom.io 'sudo mv /home/halki/monitor-sync.service /etc/systemd/system/monitor-sync.service'"
sleep 1
retry_command "ssh -p $1 halki@$2.napter.soracom.io 'sudo mv /home/halki/monitor-udp.service /etc/systemd/system/monitor-udp.service'"
sleep 1
retry_command "ssh -p $1 halki@$2.napter.soracom.io 'sudo systemctl daemon-reload'"
sleep 1
retry_command "ssh -p $1 halki@$2.napter.soracom.io 'sudo systemctl enable monitor-sync.service'"
sleep 1
retry_command "ssh -p $1 halki@$2.napter.soracom.io 'sudo systemctl enable monitor-udp.service'"
sleep 1
retry_command "ssh -p $1 halki@$2.napter.soracom.io 'sudo systemctl start monitor-sync.service'"
sleep 1
retry_command "ssh -p $1 halki@$2.napter.soracom.io 'sudo systemctl start monitor-udp.service'"
sleep 1
echo "update complete"
