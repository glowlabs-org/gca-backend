package glow

import (
	"testing"
	"time"
)

// Basic multithreaded RateLimiter test
func TestRateLimiter(t *testing.T) {

	maxcalls := 5
	maxdur := 100 * time.Millisecond
	testcalls := 19
	testint := 15 * time.Millisecond

	rl := NewRateLimiter(maxcalls, maxdur)

	ch := make(chan bool, 2*testcalls)

	start := time.Now()

	go simulateCalls(testcalls, testint, rl, ch)
	go simulateCalls(testcalls, testint, rl, ch)

	var called int
	var allowed int

	for i := 0; i < 2*testcalls; i++ {
		if <-ch {
			called++
			allowed++
		} else {
			called++
		}
	}

	t.Logf("allow %v logs in %v, requests %v allowed %v in %v", maxcalls, maxdur, called, allowed, time.Since(start))

	if allowed != 3*maxcalls {
		t.Errorf("Wrong count, wanted %v got %v", 3*maxcalls, allowed)
	}
}

// Simulate calls spaced out by an interval
func simulateCalls(n int, dur time.Duration, rl *RateLimiter, ch chan<- bool) {

	ticker := time.NewTicker(dur)
	defer ticker.Stop()

	count := 0

	for _ = range ticker.C {
		ch <- rl.Allow()
		count++
		if count == n {
			return
		}
	}
}
