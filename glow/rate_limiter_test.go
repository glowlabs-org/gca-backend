package glow

import (
	"testing"
	"time"
)

func TestRateLimiterSingle(t *testing.T) {
	rl := NewRateLimiter(1, 50*time.Millisecond)

	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	count := 0
	allowed := 0
	for _ = range ticker.C {
		if rl.Allow() {
			allowed++
		}
		count++
		if count == 4 {
			break
		}
	}

	if allowed != 2 {
		t.Errorf("Single test failed, wanted %v got %v", 2, allowed)
	}
}

// Basic multithreaded RateLimiter test
func TestRateLimiterMultithreaded(t *testing.T) {

	maxcalls := 5
	maxdur := 100 * time.Millisecond
	testcalls := 19
	testint := 15 * time.Millisecond

	rl := NewRateLimiter(maxcalls, maxdur)

	ch := make(chan bool, 2*testcalls)

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
