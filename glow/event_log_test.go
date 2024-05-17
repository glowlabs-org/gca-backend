package glow

import (
	"testing"
)

func TestEventLogDefaults(t *testing.T) {
}

/*
func TestEventLogDefaults(t *testing.T) {
	l := NewEventLog(100 * time.Millisecond, 1024, })

	l.Printf("alpha")
	l.Printf("bravo")
	if len(l.GetEventLogs()) != 2 {
		t.Fatalf("Expected 2, got %v", len(l.GetEventLogs()))
	}
	s := l.DumpLog()
	if !strings.Contains(s, "alpha") || !strings.Contains(s, "bravo") {
		t.Fatalf("Bad content")
	}
	time.Sleep(100 * time.Millisecond)
	l.Printf("charlie")
	l.Printf("delta")
	if len(l.GetEventLogs()) != 2 {
		t.Fatalf("Expected 2, got %v", len(l.GetEventLogs()))
	}
	s = l.DumpLog()
	if !strings.Contains(s, "charlie") || !strings.Contains(s, "delta") {
		t.Fatalf("Bad content")
	}
}

/*
func TestEventLogMaxSize(t *testing.T) {
	l := NewEventLog() // Default does not store anything.
	l.Printf("12345")
	l.Printf("67890")
	if len(l.GetEventLogs()) != 0 {
		t.Fatalf("Expected 0, got %v", len(l.GetEventLogs()))
	}
	l.Options.LimitBytes = 2*8 + 2*5 // Just enough for two events.
	l.Printf("12345")
	l.Printf("67890")
	if len(l.GetEventLogs()) != 2 {
		t.Fatalf("Expected 2, got %v", len(l.GetEventLogs()))
	}
	s := l.DumpLog()
	if !strings.Contains(s, "12345") || !strings.Contains(s, "67890") {
		t.Fatalf("Bad content")
	}
	l.Printf("54321")
	l.Printf("09876")
	if len(l.GetEventLogs()) != 2 {
		t.Fatalf("Expected 2, got %v", len(l.GetEventLogs()))
	}
	s = l.DumpLog()
	if !strings.Contains(s, "54321") || !strings.Contains(s, "09876") {
		t.Fatalf("Bad content")
	}
}
*/
