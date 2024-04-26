package glow

import (
	"strings"
	"testing"
	"time"
)

func TestEventLogExpiry(t *testing.T) {
	l := NewEventLog(EventLogOptions{Expiry: 100 * time.Millisecond})

	l.Printf("alpha")
	l.Printf("bravo")

	if l.Logs.Len() != 2 {
		t.Fatalf("Expected 2, got %v", l.Logs.Len())
	}

	s := l.DumpLog()
	if !strings.Contains(s, "alpha") || !strings.Contains(s, "bravo") {
		t.Fatalf("Bad content")
	}
	time.Sleep(100 * time.Millisecond)

	l.Printf("charlie")
	l.Printf("delta")

	if l.Logs.Len() != 2 {
		t.Fatalf("Expected 2, got %v", l.Logs.Len())
	}

	s = l.DumpLog()
	if !strings.Contains(s, "charlie") || !strings.Contains(s, "delta") {
		t.Fatalf("Bad content")
	}
}

func TestEventLogMaxCount(t *testing.T) {
	l := NewEventLog(EventLogOptions{MaxCount: 2})

	l.Printf("alpha")
	l.Printf("bravo")

	if l.Logs.Len() != 2 {
		t.Fatalf("Expected 2, got %v", l.Logs.Len())
	}

	s := l.DumpLog()
	if !strings.Contains(s, "alpha") || !strings.Contains(s, "bravo") {
		t.Fatalf("Bad content")
	}

	l.Printf("charlie")
	l.Printf("delta")

	if l.Logs.Len() != 2 {
		t.Fatalf("Expected 2, got %v", l.Logs.Len())
	}

	s = l.DumpLog()
	if !strings.Contains(s, "charlie") || !strings.Contains(s, "delta") {
		t.Fatalf("Bad content")
	}
}
