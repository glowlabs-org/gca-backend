# Helper to return equipment information from a GCA server.
import requests
import json
import argparse

# Drops the signature array, converts the public key to a hex string,
# and returns a map of shortIDs to details.
def process_equipment(data):
	equipment_details = data.get("EquipmentDetails", {})
	for key, details in equipment_details.items():
		if "Signature" in details:
			del details["Signature"]
		if "PublicKey" in details and isinstance(details["PublicKey"], list):
			details["PublicKey"] = "".join(format(byte, "02x") for byte in details["PublicKey"])
	return equipment_details

# Gets the equipment list from the server. Returns a map with ShortID key.
def fetch_equipment(server, port):
	url = f"http://{server}:{port}/api/v1/equipment"
	res = requests.get(url)
	if res.status_code != 200:
		print(f"Error: Failed to fetch data: {res.status_code} - {res.text}")
		return None
	try:
		data = res.json()
		data = process_equipment(data)
		return data
	except json.JSONDecodeError:
		print("Error: Failed to decode JSON response")
		return None

def emit_entry(entry):
	print(f"{entry['ShortID']}:")
	print(f"  Lat & Long: {entry['Latitude']}, {entry['Longitude']} Cap: {entry['Capacity']} Debt: {entry['Debt']} Exp: {entry['Expiration']} Init: {entry['Initialization']}")
	print(f"  PublicKey: {entry['PublicKey']}")

if __name__ == "__main__":
	parser = argparse.ArgumentParser(description="Fetch equipment from server")
	parser.add_argument("-s", "--server", type=str, required=True, help="Server address")
	parser.add_argument("-p", "--port", type=int, default=35015, help="Server port (default: 35015)")
	parser.add_argument("-i", "--sid", type=str, default=None, help="Short ID (default: ALL)")
	args = parser.parse_args()

	data = fetch_equipment(args.server, args.port)
	if data is None:
		exit(1)

	if args.sid is None:
		for key in sorted(data.keys(), key=int):
			entry = data[key]
			emit_entry(entry)
	else:
		emit_entry(data[args.sid])
