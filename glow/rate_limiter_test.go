package glow

import (
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Test configuration.
type TestConfig struct {
	limit    int
	rate     time.Duration
	threads  int           // Number of threads to start.
	requests int           // Requests to make per thread.
	duration time.Duration // Maximum duration of the test.
}

func TestRateLimiter(t *testing.T) {

	limits := []int{1, 3, 10}
	rate_ms := 24
	threads := []int{1, 10, 20, 50}
	durs_ms := []int{15, 24, 32, 48, 60}

	ch := make(chan TestConfig)

	var wg sync.WaitGroup

	// Paralellize the tests to reduce to a managable time.
	parallel := 3
	for i := 0; i < parallel; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for config := range ch { // Finished when empty and closed.

				if err := config.runTest(); err != nil {
					t.Errorf("Test failed: %v", err)
				}
			}
		}()
	}

	for _, i := range limits {
		for _, j := range threads {
			for _, k := range durs_ms {
				ch <- TestConfig{
					limit:    i,
					rate:     time.Duration(rate_ms) * time.Millisecond,
					threads:  j,
					requests: i, // Each thread sends the limit.
					duration: time.Duration(k) * time.Millisecond,
				}
			}
		}
	}

	close(ch) // Signal the workers.
	wg.Wait()
}

// Creates threads, and makes rate limiter calls in each thread,
// with some random spacing within the test duration.
func (tc TestConfig) runTest() error {
	var wg sync.WaitGroup
	var allowed atomic.Int32
	var denied atomic.Int32

	rl := NewRateLimiter(tc.limit, tc.rate)
	segment := tc.duration / time.Duration(tc.limit+1) // Ensure API is not called at the end
	start := time.Now()

	for i := 0; i < tc.threads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Choose a point within each segment
			durs := make([]float64, tc.requests)
			for j := 0; j < tc.requests; j++ {
				durs[j] = (float64(j) + rand.Float64()) * segment.Seconds()
			}

			sort.Float64s(durs) // Ensure increasing order

			// Order by time
			cuttab := make([]time.Time, tc.requests)
			for j := 0; j < tc.requests; j++ {
				cuttab[j] = start.Add(time.Duration(durs[j] * float64(time.Second)))
			}

			for j := 0; j < tc.requests; j++ {
				// Sleep until the next time. All the times should fall
				// within the max duration.
				time.Sleep(time.Until(cuttab[j]))

				if rl.Allow() {
					allowed.Add(1)
				} else {
					denied.Add(1)
				}
			}
		}()
	}

	wg.Wait()

	test_dur := time.Since(start)
	if test_dur > tc.duration {
		return fmt.Errorf("test %v duration %v exceeded %v", tc, test_dur, tc.duration)
	}

	if allowed.Load()+denied.Load() != int32(tc.threads*tc.requests) {
		return fmt.Errorf("test %v incorrect requests %v", tc, allowed.Load()+denied.Load())
	}

	// Calculate the number of rate limit intervals.
	periods := int(test_dur / tc.rate)
	if test_dur%tc.rate != 0 {
		periods++
	}

	if int(allowed.Load()) > periods*tc.limit {
		return fmt.Errorf("test %v failed with %v allowed for %v rate periods", tc, allowed.Load(), periods)
	}

	return nil
}

/*

type callResponse struct {
	ReqTime      time.Time
	ResponseCode int
}

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

// Simulate calls spaced out by an interval. Sends calls spaced out over a
// duration. Returns the time each call was made, and the error response returned.
func timedCaller(n int, dur time.Duration, rl *RateLimiter, ch chan<- callResponse) {

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
*/
