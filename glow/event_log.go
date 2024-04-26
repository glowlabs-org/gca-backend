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

type event struct {
	ts   time.Time
	line string
}

type EventLogOptions struct {
	Expiry   time.Duration // Older logs will be removed, 0 means never expire
	MaxCount int           // Limit the number of logs to collect, 0 means unlimited
}

type EventLog struct {
	Options EventLogOptions
	Logs    *list.List
	mu      sync.Mutex
}

// Create a new event log.
// The default behavior is to expire in 48 hours, and keep 5000 logs.
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

	old := now.Add(-l.Options.Expiry)

	l.mu.Lock()
	defer l.mu.Unlock()

	for i := l.Logs.Front(); i != nil; {
		ev := i.Value.(event)
		if !ev.ts.Before(old) {
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
		ts: time.Now(),
	}
	nxt.line = fmt.Sprintf(format, a...)

	l.mu.Lock()
	defer l.mu.Unlock()

	// If max count is exceeded, make space.
	if l.Options.MaxCount > 0 && l.Logs.Len() == l.Options.MaxCount {
		l.Logs.Remove(l.Logs.Front())
	}

	l.Logs.PushBack(nxt)
}

// Concatenate the logs into a string separated by newlines. Implements
// the Stringer interface.
func (l *EventLog) String() string {
	var builder strings.Builder

	l.mu.Lock()
	defer l.mu.Unlock()

	for i := l.Logs.Front(); i != nil; i = i.Next() {
		ev := i.Value.(event)

		builder.WriteString(ev.ts.UTC().Format("2006-01-02 15:04:05 UTC"))
		builder.WriteString(" ")
		builder.WriteString(ev.line)
		builder.WriteString("\n")
	}

	return builder.String()
}
