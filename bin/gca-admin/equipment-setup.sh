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

ssh halki@$2 'sudo mv /home/halki/glow-monitor /usr/bin/glow-monitor'
sleep 4
ssh halki@$2 'sudo mv /home/halki/halki-app /usr/bin/halki-app'
sleep 4
ssh halki@$2 'sudo mv /home/halki/* /opt/glow-monitor/'
sleep 4
ssh halki@$2 'sudo systemctl start glow_monitor.service'
sleep 1
ssh halki@$2 'sudo systemctl start atm90e32.service'
sleep 1
