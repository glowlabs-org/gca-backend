# Helper to return device stats information from a GCA server.
import requests
import json
import argparse
from datetime import datetime, timezone
from equipment import fetch_equipment

GENESIS_TIME = 1700352000  # The genesis timestamp
TIMESLOT_DURATION = 5 * 60  # Timeslot duration in seconds (5 minutes)
TIMESLOTS_PER_WEEK = 2016  # Number of timeslots in a week

# Get the raw device stats from the server for a specific week.
def fetch_device_stats(server, port, week):
	url = f"http://{server}:{port}/api/v1/all-device-stats"
	params = {"timeslot_offset": week*TIMESLOTS_PER_WEEK}
	res = requests.get(url, params=params)
	if res.status_code != 200:
		print(f"Error: Failed to fetch data: {res.status_code} - {res.text}")
		return None
	try:
		data = res.json()
		return data
	except json.JSONDecodeError:
		print("Error: Failed to decode JSON response")
		return None

# Return a map of devices keyed by ShortID.
def process_device_stats(dev_data, equip_data):
	# Make a PublicKey to ShortID map
	kmap = {}
	for _, entry in equip_data.items():
		kmap[entry["PublicKey"]] = entry["ShortID"]
	ret = {}
	for entry in dev_data["Devices"]:
		if "Signature" in entry:
			del entry["Signature"]
		if "PublicKey" in entry and isinstance(entry["PublicKey"], list):
			entry["PublicKey"] = "".join(format(byte, "02x") for byte in entry["PublicKey"])
		id = kmap[entry["PublicKey"]]
		entry["ShortID"] = id
		ret[str(id)] = entry
	
	return ret

def timeslot_to_datetime(s):
	total_seconds = GENESIS_TIME + s * TIMESLOT_DURATION
	dt = datetime.utcfromtimestamp(total_seconds)
	formatted_datetime = dt.strftime('%m/%d %H:%M')
	return formatted_datetime

# Emit an entry, including a list of power outputs and impact rates, to allow visualizing the device stats.
def emit_entry(entry, week):
	print(f"{entry['ShortID']}:")
	print(f"  PublicKey: {entry['PublicKey']}")
	print(f"  Week: {week} Starting Timeslot: {week*2016} {datetime.fromtimestamp(GENESIS_TIME+week*TIMESLOTS_PER_WEEK*TIMESLOT_DURATION, timezone.utc)}")
	print("  PowerOutputs by Timeslot")
	data_list = entry["PowerOutputs"]
	prs = "\n".join([f"  {timeslot_to_datetime(week*TIMESLOTS_PER_WEEK+i)}: " + " ".join(map(lambda x: str(int(x)), data_list[i:i+16])) for i in range(0, len(data_list), 16)])
	print(prs)
	print("  ImpactRates by Timeslot")
	data_list = entry["ImpactRates"]
	prs = "\n".join([f"  {timeslot_to_datetime(week*TIMESLOTS_PER_WEEK+i)}: " + " ".join(map(lambda x: str(int(x)), data_list[i:i+16])) for i in range(0, len(data_list), 16)])
	print(prs)

if __name__ == "__main__":
	parser = argparse.ArgumentParser(description="Fetch device stats from server")
	parser.add_argument("-s", "--server", type=str, required=True, help="Server address")
	parser.add_argument("-w", "--week", type=int, required=True, help="Week")
	parser.add_argument("-p", "--port", type=int, default=35015, help="Server port (default: 35015)")
	parser.add_argument("-i", "--sid", type=str, default=None, help="Short ID (default: ALL)")
	args = parser.parse_args()
	
	# Get a map of all equipment on this server.
	equip = fetch_equipment(args.server, args.port)
	if equip is None:
		exit(1)

	# Get raw decid stats
	data = fetch_device_stats(args.server, args.port, args.week)
	if data is None:
		exit(1)

	data = process_device_stats(data, equip)
	
	if args.sid is None:
		for key in sorted(data.keys(), key=int):
			entry = data[key]
			emit_entry(entry, args.week)
	else:
		emit_entry(data[args.sid], args.week)