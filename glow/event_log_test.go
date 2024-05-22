package glow

import (
	"testing"
	"time"
)

func TestEventLogBasic(t *testing.T) {
	l := NewEventLogger(time.Second, 1000, 100) // 2 logs of size 5 requires 20 bytes
	l.Printf("abcde")
	time.Sleep(5 * time.Millisecond)
	l.Printf("fghij")
	time.Sleep(5 * time.Millisecond)
	entries, order := l.DumpLogEntries()
	if len(entries) != 2 {
		t.Errorf("wrong size, expecting %v got %v", 2, len(entries))
	}
	if _, found := entries["abcde"]; !found {
		t.Errorf("missing log: %v", "abcde")
	}
	if _, found := entries["fghij"]; !found {
		t.Errorf("missing log: %v", "fghij")
	}
	if len(order) != 2 {
		t.Errorf("wrong size, expecting %v got %v", 2, len(order))
	}
	if order[0] != "abcde" {
		t.Errorf("out of order: %v", "abcde")
	}
	if order[1] != "fghij" {
		t.Errorf("out of order: %v", "fghij")
	}
}

func TestEventLogMaxSize(t *testing.T) {
	l := NewEventLogger(time.Second, 20, 100) // 2 logs of size 5 requires 20 bytes
	l.Printf("abcde")
	time.Sleep(5 * time.Millisecond)
	l.Printf("fghij")
	time.Sleep(5 * time.Millisecond)
	entries, _ := l.DumpLogEntries()
	if len(entries) != 2 {
		t.Fatalf("Expected 2, got %v", len(entries))
	}
	if _, found := entries["abcde"]; !found {
		t.Errorf("missing log: %v", "abcde")
	}
	if _, found := entries["fghij"]; !found {
		t.Errorf("missing log: %v", "fghij")
	}
	l.Printf("klmno")
	time.Sleep(5 * time.Millisecond)
	l.Printf("pqrst")
	time.Sleep(5 * time.Millisecond)
	entries, _ = l.DumpLogEntries()
	if len(entries) != 2 {
		t.Fatalf("Expected 2, got %v", len(entries))
	}
	if _, found := entries["klmno"]; !found {
		t.Errorf("missing log: %v", "klmno")
	}
	if _, found := entries["pqrst"]; !found {
		t.Errorf("missing log: %v", "pqrst")
	}
}

func TestEventLogExpiry(t *testing.T) {
	l := NewEventLogger(5*time.Millisecond, 1000, 100)
	l.Printf("abcde")
	l.Printf("fghij")
	entries, _ := l.DumpLogEntries()
	if len(entries) != 2 {
		t.Fatalf("Expected 2, got %v", len(entries))
	}
	if _, found := entries["abcde"]; !found {
		t.Errorf("missing log: %v", "abcde")
	}
	if _, found := entries["fghij"]; !found {
		t.Errorf("missing log: %v", "fghij")
	}
	time.Sleep(5 * time.Millisecond)
	l.Printf("klmno")
	l.Printf("pqrst")
	entries, _ = l.DumpLogEntries()
	if len(entries) != 2 {
		t.Fatalf("Expected 2, got %v", len(entries))
	}
	if _, found := entries["klmno"]; !found {
		t.Errorf("missing log: %v", "klmno")
	}
	if _, found := entries["pqrst"]; !found {
		t.Errorf("missing log: %v", "pqrst")
	}
}

func TestEventLogLineLengthLimit(t *testing.T) {
	l := NewEventLogger(5*time.Millisecond, 1000, 5)
	l.Printf("abcdefghij")
	entries, _ := l.DumpLogEntries()
	if _, found := entries["abcdefghij"]; found {
		t.Errorf("log was not truncated: %v", "abcdefghij")
	}
	if _, found := entries["abcde"]; !found {
		t.Errorf("missing log: %v", "abcde")
	}
}
