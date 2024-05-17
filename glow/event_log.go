package glow

// This file contains a simple event logger, designed to collect a limited
// number of timestamped logs and output them to file or terminal.

import (
	"container/list"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// LogEntry contains a timestamp and logged message.
type LogEntry struct {
	ts   time.Time
	line string
}

// EventStats keys track of an event type key and timestamp for when the events were received.
type EventStats map[string]*list.List // List contains time.Time

// EventLogger keeps track of a list of LogEntries, manages the size of logged lines in memory,
// and allows the collection of keyed event statistics.
type EventLogger struct {
	logs            *list.List    // List of log entries (contains LogEntry)
	logExpiry       time.Duration // Duration to retain logs
	logMaxBytes     uint          // Total allowed size for logged strings
	logMaxLineBytes uint          // Maximum logged string size per line
	logSizeBytes    uint          // Current size of logged strings
	stats           EventStats    // Per event type statistics counter
	mu              sync.Mutex
}

// NewEventLogger creates an in-memory event logger with size limitations to protect the client
// from run-time failures.
//
// In general, logging is intended for error conditions, or unusual conditions, while event statistic
// are intended for normally occurring conditions.
func NewEventLogger(logExpiry time.Duration, logMaxBytes, logMaxLineBytes uint) *EventLogger {
	// Require parameters that allow logs to be collected, and enforce some reasonable limits.
	if logExpiry == time.Duration(0) || logMaxBytes == 0 || logMaxLineBytes == 0 {
		fmt.Println("LogEntry log settings do not allow log collection.")
		return nil
	}
	if logMaxBytes == 1e8 || logMaxLineBytes >= 1e3 {
		fmt.Printf("LogEntry log parameters are too high: max bytes is %v and max line bytes is %v", logMaxBytes, logMaxLineBytes)
		return nil
	}
	return &EventLogger{
		logs:            list.New(),
		logExpiry:       logExpiry,
		logMaxBytes:     logMaxBytes,
		logMaxLineBytes: logMaxLineBytes,
		stats:           make(EventStats),
	}
}

// ExpireEvents removes events older than the logExpiry duration.
func (l *EventLogger) ExpireEvents(now time.Time) {
	expireTime := now.Add(-l.logExpiry)
	l.mu.Lock()
	defer l.mu.Unlock()

	// Events past the expiry time are removed and not retained in any way.
	for key, eventList := range l.stats {
		for e := eventList.Front(); e != nil; {
			eventTime := e.Value.(time.Time)
			nxt := e.Next()
			if eventTime.Before(expireTime) {
				eventList.Remove(e)
			} else {
				break
			}
			e = nxt
		}
		// Remove the key if the list is empty.
		if eventList.Len() == 0 {
			delete(l.stats, key)
		}
	}
}

// LogEvent increments the counter for the keyed event, and
// updates the most recent event time.
func (l *EventLogger) UpdateEventStats(key string) {
	now := time.Now()
	l.ExpireEvents(now)
	l.mu.Lock()
	defer l.mu.Unlock()

	if ev, found := l.stats[key]; found {
		ev.PushBack(now)
	} else {
		l.stats[key] = list.New()
		l.stats[key].PushBack(now)
	}
}

// ExpireLogs removes all logs older than the logExpiry duration.
func (l *EventLogger) ExpireLogs(now time.Time) {
	expireTime := now.Add(-l.logExpiry)
	l.mu.Lock()
	defer l.mu.Unlock()

	// Logs past the expiry time are removed and not retained in any way.
	for i := l.logs.Front(); i != nil; {
		log := i.Value.(LogEntry)
		nxt := i.Next()
		if log.ts.Before(expireTime) {
			l.logs.Remove(i)
		} else {
			break
		}
		i = nxt
	}
}

// Add a timestamped event to the log, truncating and removing logs
// according to the EventLogger rules.
func (l *EventLogger) Printf(format string, a ...interface{}) {
	now := time.Now()
	l.ExpireLogs(now)
	nxt := LogEntry{
		ts:   now,
		line: fmt.Sprintf(format, a...),
	}

	// Apply line truncation.
	if len(nxt.line) > int(l.logMaxLineBytes) {
		nxt.line = nxt.line[:l.logMaxLineBytes]

	}
	l.mu.Lock()
	defer l.mu.Unlock()

	// Estimate the size of the event, and limit the size.
	sz := uint(len(nxt.line))
	if sz > l.logMaxBytes {
		return // We can't ever store this.
	}

	// While there is not enough space, remove logs oldest to newest.
	for sz+l.logSizeBytes > l.logMaxBytes {
		i := l.logs.Front()
		l.logs.Remove(i)
		ev := i.Value.(LogEntry)
		l.logSizeBytes -= uint(len(ev.line))
	}
	l.logMaxBytes += sz
	l.logs.PushBack(nxt)
}

// DumpLogEntries concatenates the logs into a formatted string separated by newlines.
func (l *EventLogger) DumpLogEntries() string {
	l.ExpireLogs(time.Now())
	var sb strings.Builder
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := l.logs.Front(); i != nil; i = i.Next() {
		ev := i.Value.(LogEntry)
		sb.WriteString(ev.ts.UTC().Format(time.RFC3339))
		sb.WriteString(" ")
		sb.WriteString(ev.line)
		sb.WriteString("\n")
	}
	return sb.String()
}

// GetLogEntries returns the current log events.
func (l *EventLogger) GetLogEntries() []LogEntry {
	ret := make([]LogEntry, 0)
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := l.logs.Front(); i != nil; i = i.Next() {
		ret = append(ret, i.Value.(LogEntry))
	}
	return ret
}

// DumpLogEntries concatenates the logs into a formatted string separated by newlines.
func (l *EventLogger) DumpEventStats() string {
	l.ExpireEvents(time.Now())
	var sb strings.Builder
	l.mu.Lock()
	defer l.mu.Unlock()
	for k, evl := range l.stats {
		sb.WriteString(k + ":\n")
		sb.WriteString("  call count: " + strconv.Itoa(evl.Len()) + "\n")
		latest := evl.Back().Value.(time.Time)
		sb.WriteString("  last call:  " + latest.UTC().Format(time.RFC3339) + "\n")
	}
	return sb.String()
}

// GetEventStats returns the current event stats.
func (l *EventLogger) GetEventStats() EventStats {
	ret := make(EventStats)
	l.mu.Lock()
	defer l.mu.Unlock()
	for k, v := range l.stats {
		ret[k] = v
	}
	return ret
}
