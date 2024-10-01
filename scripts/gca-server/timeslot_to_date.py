import sys
import time
from datetime import datetime, timezone

GENESIS_TIME = 1700352000  # The genesis timestamp
TIMESLOT_DURATION = 5 * 60  # Timeslot duration in seconds (5 minutes)
TIMESLOTS_PER_WEEK = 2016  # Number of timeslots in a week

# Convert timeslot to date and week.
def timeslot_to_date(tslot):
	t = GENESIS_TIME + TIMESLOT_DURATION*tslot
	d = datetime.fromtimestamp(t, timezone.utc)
	w = tslot // TIMESLOTS_PER_WEEK
	return d, w

if __name__ == "__main__":
	if len(sys.argv) < 2:
		print(f"usage: {sys.argv[0]} <timeslot>")
		exit(1)
	try:
		tslot = int(sys.argv[1])
		if tslot < 0:
			print("timeslot must not be negative")
			exit(1)
	except:
		print("invalid timeslot entered")
		exit(1)

	d, w = timeslot_to_date(tslot)
	print(f"timeslot {tslot} is week {w} and date {d}")
