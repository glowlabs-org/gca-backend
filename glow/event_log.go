package glow

// This file contains a simple event logger, designed to collect a limited
// number of timestamped logs and dump them to file or terminal.

import (
	"container/list"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Event.
type event struct {
	ts   time.Time
	line string
}

// Options for the event log.
type EventLogOptions struct {
	Expiry     time.Duration // Older events will be removed. Default is no expiry.
	LimitBytes int           // Limit the size of events to collect. Default is not to collect events.
}

// Event log.
type EventLog struct {
	Options   EventLogOptions
	Logs      *list.List // List of events
	sizeBytes int        // Current size of events
	mu        sync.Mutex
}

// Create a new event log.
// The default behavior is to never expire, and keep unlimited logs.
func NewEventLog(opts ...EventLogOptions) *EventLog {
	var elo EventLogOptions
	if len(opts) == 0 {
		elo = EventLogOptions{}
	} else {
		elo = opts[0]
	}
	return &EventLog{
		Options: elo,
		Logs:    list.New(),
	}
}

// Expire logs.
func (l *EventLog) Expire(now time.Time) {
	if l.Options.Expiry == 0 {
		return // Never expire.
	}
	expireTime := now.Add(-l.Options.Expiry)
	l.mu.Lock()
	defer l.mu.Unlock()

	// Remove from the list until we get to a
	// log which is not expired.
	for i := l.Logs.Front(); i != nil; {
		ev := i.Value.(event)
		if !ev.ts.Before(expireTime) {
			break
		}
		nxt := i.Next()
		l.Logs.Remove(i)
		i = nxt
	}
}

// Add a timestamped event to the log.
func (l *EventLog) Printf(format string, a ...interface{}) {
	now := time.Now()
	l.Expire(now)
	nxt := event{
		ts:   time.Now(),
		line: fmt.Sprintf(format, a...),
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	// Estimate the size of the event, and limit the size.
	sz := 64 + len(nxt.line)
	if sz > l.Options.LimitBytes {
		return // We can't ever store this.
	}

	// While there is not enough space, remove logs oldest to newest.
	for sz+l.sizeBytes > l.Options.LimitBytes {
		i := l.Logs.Front()
		l.Logs.Remove(i)
		ev := i.Value.(event)
		l.sizeBytes -= 64 + len(ev.line)
	}
	l.sizeBytes += sz
	l.Logs.PushBack(nxt)
}

// Concatenate the logs into a string separated by newlines.
func (l *EventLog) DumpLog() string {
	var builder strings.Builder
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := l.Logs.Front(); i != nil; i = i.Next() {
		ev := i.Value.(event)
		builder.WriteString(ev.ts.UTC().Format(time.RFC3339))
		builder.WriteString(" ")
		builder.WriteString(ev.line)
		builder.WriteString("\n")
	}
	return builder.String()
}
