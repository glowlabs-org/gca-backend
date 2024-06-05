import time
from datetime import datetime, timezone

GENESIS_TIME = 1700352000  # The genesis timestamp
TIMESLOT_DURATION = 5 * 60  # Timeslot duration in seconds (5 minutes)
TIMESLOTS_PER_WEEK = 2016  # Number of timeslots in a week

# Calculate and print out weekly information up to the current week.
def print_week_info(current_time):
	elapsed_time = current_time - GENESIS_TIME
	total_timeslots = elapsed_time // TIMESLOT_DURATION
	for week_number in range(total_timeslots // TIMESLOTS_PER_WEEK + 1):
		week_start_timeslot = week_number * TIMESLOTS_PER_WEEK
		week_start_time = GENESIS_TIME + week_start_timeslot * TIMESLOT_DURATION
		week_start_date = datetime.fromtimestamp(week_start_time, timezone.utc).date()
		print(f"Week Number: {week_number:2}, Starting Timeslot: {week_start_timeslot:5}, Starting Date: {week_start_date} UTC")

current_time = int(time.time())
current_time_utc = datetime.fromtimestamp(current_time, timezone.utc)
print(f"Current Time (UTC): {current_time_utc}")
print_week_info(current_time)
