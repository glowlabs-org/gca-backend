if [ $# -lt 2 ]; then
	echo "Usage: $0 [short-id] [equipment-ip]"
	exit 1
fi

ssh-copy-id halki@$2
sleep 4
scp .config/gca-data/clients/client_$1/* halki@$2:/home/halki/
sleep 4
ssh halki@$2 'sudo systemctl stop glow_monitor.service'
sleep 1
ssh halki@$2 'sudo systemctl stop atm90e32.service'
sleep 1
scp .config/gca-data/clients/glow-monitor halki@$2:/home/halki
sleep 4
scp .config/gca-data/clients/halki-app halki@$2:/home/halki
sleep 4
scp .config/gca-data/clients/monitor-sync.service halki@$2:/home/halki
sleep 4
scp .config/gca-data/clients/monitor-sync.sh halki@$2:/home/halki
sleep 4
scp .config/gca-data/clients/monitor-udp.service halki@$2:/home/halki
sleep 4
scp .config/gca-data/clients/monitor-udp.sh halki@$2:/home/halki

ssh halki@$2 'sudo mv /home/halki/glow-monitor /usr/bin/glow-monitor'
sleep 4
ssh halki@$2 'sudo mv /home/halki/halki-app /usr/bin/halki-app'
sleep 4
ssh halki@$2 'sudo mv /home/halki/monitor-sync.sh /usr/bin/monitor-sync.sh'
sleep 4
ssh halki@$2 'sudo mv /home/halki/monitor-udp.sh /usr/bin/monitor-udp.sh'
sleep 4
ssh halki@$2 'sudo mv /home/halki/monitor-sync.service /etc/systemd/system/monitor-sync.service'
sleep 4
ssh halki@$2 'sudo mv /home/halki/monitor-udp.service /etc/systemd/system/monitor-udp.service'
sleep 4
ssh halki@$2 'sudo mv /home/halki/* /opt/glow-monitor/'
sleep 4
ssh halki@$2 'sudo systemctl daemon-reload'
sleep 4
ssh halki@$2 'sudo systemctl start glow_monitor.service'
sleep 1
ssh halki@$2 'sudo systemctl start atm90e32.service'
sleep 1
ssh halki@$2 'sudo systemctl enable monitor-sync.service'
sleep 1
ssh halki@$2 'sudo systemctl enable monitor-udp.service'
sleep 1
ssh halki@$2 'sudo systemctl start monitor-sync.service'
sleep 1
ssh halki@$2 'sudo systemctl start monitor-udp.service'
sleep 1

echo "Hardware setup is complete"
