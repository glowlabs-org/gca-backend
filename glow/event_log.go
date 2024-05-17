package glow

// This file contains a simple event logger, designed to collect a limited
// number of timestamped logs and output them to file or terminal.

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// LogEntry contains a logged string, and a list of time ordered timestamps.
type LogEntry struct {
	line    string      // The logged line
	updates []time.Time // Time ordered list of updates (time.Time)
}

// EventLogger keeps track logged events and manages their size in memory,
// allowing for duplicate entries.
type EventLogger struct {
	logs            map[string]*LogEntry // Map of log lines to entries
	logExpiry       time.Duration        // Duration to retain logs
	logMaxBytes     int                  // Total allowed size for logged strings
	logMaxLineBytes int                  // Maximum logged string size per line
	logSizeBytes    int                  // Current size of logged strings
	mu              sync.Mutex
}

// NewEventLogger creates an in-memory event logger with size limitations to protect the client
// from run-time failures.
func NewEventLogger(logExpiry time.Duration, logMaxBytes, logMaxLineBytes int) *EventLogger {
	// Require parameters that allow logs to be collected, and enforce some reasonable limits.
	if logExpiry == time.Duration(0) || logMaxBytes <= 0 || logMaxLineBytes <= 0 {
		fmt.Println("LogEntry log settings do not allow log collection.")
		return nil
	}
	if logMaxBytes >= 1e8 || logMaxLineBytes >= 1e3 {
		fmt.Printf("LogEntry log parameters are too high: max bytes is %v and max line bytes is %v", logMaxBytes, logMaxLineBytes)
		return nil
	}
	return &EventLogger{
		logs:            make(map[string]*LogEntry),
		logExpiry:       logExpiry,
		logMaxBytes:     logMaxBytes,
		logMaxLineBytes: logMaxLineBytes,
	}
}

// ExpireLogs expires all logs, removing them after they have no unexpired timestamps.
func (l *EventLogger) ExpireLogs(now time.Time) {
	expireTime := now.Add(-l.logExpiry)
	keysToDelete := []string{}
	l.mu.Lock()
	defer l.mu.Unlock()

	// Log entries past the expiry time are removed and not retained in any way.
	for _, entry := range l.logs {
		removeIdx := 0
		for _, ts := range entry.updates {
			if ts.Before(expireTime) {
				removeIdx++
			} else {
				break
			}
		}
		entry.updates = entry.updates[removeIdx:]
		if len(entry.updates) == 0 {
			keysToDelete = append(keysToDelete, entry.line)
		}
	}
	// Remove all unused log entries
	for _, key := range keysToDelete {
		delete(l.logs, key)
	}
}

// Add a timestamped event to the log, truncating and removing logs
// according to the EventLogger rules.
func (l *EventLogger) Printf(format string, a ...interface{}) {
	now := time.Now()
	l.ExpireLogs(now)
	l.mu.Lock()
	defer l.mu.Unlock()

	// Apply line truncation.
	key := fmt.Sprintf(format, a...)
	if len(key) > l.logMaxLineBytes {
		key = key[:l.logMaxLineBytes]

	}
	sizeRequired := 2 * len(key) // The size used is twice the string length
	if sizeRequired > l.logMaxBytes {
		return // We can't ever store this.
	}

	// Avoid log duplication.
	entry, found := l.logs[key]
	if found {
		entry.updates = append(entry.updates, now)
		return
	}

	// Remove the oldest logs first.
	var updateOrder []*LogEntry

	if sizeRequired+l.logSizeBytes > l.logMaxBytes {
		updateOrder = make([]*LogEntry, 0)
		for _, entry := range l.logs {
			updateOrder = append(updateOrder, entry)
		}
		sort.Slice(updateOrder, func(i, j int) bool {
			return updateOrder[i].updates[len(updateOrder[i].updates)-1].Before(updateOrder[j].updates[len(updateOrder[j].updates)-1]) // Ascending sort
		})
	}

	// While there is not enough space, remove logs oldest to newest.
	for sizeRequired+l.logSizeBytes > l.logMaxBytes {
		oldest := updateOrder[0]
		updateOrder = updateOrder[1:]
		l.logSizeBytes -= 2 * len(oldest.line)
		delete(l.logs, oldest.line)
	}

	// Add the entry
	l.logSizeBytes += sizeRequired
	nxt := &LogEntry{
		line:    key,
		updates: make([]time.Time, 0),
	}
	nxt.updates = append(nxt.updates, now)
	l.logs[key] = nxt
}

// DumpLogEntries expires the logs and returns a map of logs to timestamps, and a time
// ordered slice of logs.
func (l *EventLogger) DumpLogEntries() (map[string][]time.Time, []string) {
	l.ExpireLogs(time.Now())
	l.mu.Lock()
	defer l.mu.Unlock()
	retMap := make(map[string][]time.Time, 0)
	retSlice := make([]string, 0)
	type orderMap struct {
		line   string
		latest time.Time
	}
	updateOrder := []*orderMap{}
	for _, entry := range l.logs {
		// Create the timestamps map entry
		timestamps := make([]time.Time, 0)
		timestamps = append(timestamps, entry.updates...)
		retMap[entry.line] = timestamps

		// Add the last timestamp to the update order
		nxt := &orderMap{
			line:   entry.line,
			latest: entry.updates[len(entry.updates)-1],
		}
		updateOrder = append(updateOrder, nxt)
	}
	sort.Slice(updateOrder, func(i, j int) bool {
		return updateOrder[i].latest.Before(updateOrder[j].latest) // Ascending sort
	})
	for _, entry := range updateOrder {
		retSlice = append(retSlice, entry.line)
	}
	return retMap, retSlice
}
