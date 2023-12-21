if [ $# -lt 2 ]; then
	echo "Usage: ./equipment-setup.sh [shortID] [ip]"
	exit 1
fi

ssh-keygen -f "/home/user/.ssh/known_hosts" -R $2
ssh-copy-id halki@$2
scp .config/gca-data/clients/client_$1/* halki@$2:/home/halki/
scp .config/gca-data/clients/glow-monitor halki@$2:/home/halki

ssh halki@$2 'sudo mkdir /opt/glow-monitor'
ssh halki@$2 'sudo mv /home/halki/glow-monitor /usr/bin/glow-monitor'
ssh halki@$2 'sudo mv /home/halki/* /opt/glow-monitor/'
