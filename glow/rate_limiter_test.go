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

// Configuration for a single test run, specifying rate limiter
// parameters, number of threads and requests, and duration of the
// test.
type TestConfig struct {
	limit    int
	rate     time.Duration
	threads  int           // Number of threads (requestors) to start.
	requests int           // Requests to make per thread.
	duration time.Duration // Maximum duration of the test.
}

// Test a matrix of test cases. The production limit should be the
// middle point of the test cases. Corner case of interest is the single
// limit case, as well as high limit, high thread count.
func TestRateLimiterParallel(t *testing.T) {
	limits := []int{1, 3, 10}
	rate_ms := 24
	threads := []int{1, 10, 20, 50, 100}
	durs_ms := []int{15, 24, 32, 48, 60}

	for _, i := range limits {
		for _, j := range threads {
			for _, k := range durs_ms {
				tc := TestConfig{
					limit:    i,
					rate:     time.Duration(rate_ms) * time.Millisecond,
					threads:  j,
					requests: i, // Each thread sends the limit.
					duration: time.Duration(k) * time.Millisecond,
				}
				t.Run(fmt.Sprintf("%v per %v threads: %v reqs: %v dur: %v", tc.limit, tc.rate, tc.threads, tc.requests, tc.duration), func(t *testing.T) {
					t.Parallel()
					if err := tc.runTest(); err != nil {
						t.Errorf("Test failed: %v", err)
					}
				})
			}
		}
	}
}

// Creates threads, and makes rate limiter calls in each thread,
// with some random spacing within the test duration.
func (tc TestConfig) runTest() error {
	var wg sync.WaitGroup
	var allowed atomic.Int32
	var denied atomic.Int32

	rl := NewRateLimiter(tc.limit, tc.rate)
	segment := tc.duration / time.Duration(tc.limit+1) // Leave some space at the end for timing.
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
			cutTab := make([]time.Time, tc.requests)
			for j := 0; j < tc.requests; j++ {
				cutTab[j] = start.Add(time.Duration(durs[j] * float64(time.Second)))
			}

			for j := 0; j < tc.requests; j++ {
				// Sleep until the next time. All the times should fall
				// within the max duration.
				time.Sleep(time.Until(cutTab[j]))

				if rl.Allow() {
					allowed.Add(1)
				} else {
					denied.Add(1)
				}
			}
		}()
	}

	wg.Wait()

	// Ensure the test completes within the specified duration.
	testDur := time.Since(start)
	if testDur > tc.duration {
		return fmt.Errorf("test %v duration %v exceeded %v", tc, testDur, tc.duration)
	}

	// Ensure that correct number of requests were made.
	if allowed.Load()+denied.Load() != int32(tc.threads*tc.requests) {
		return fmt.Errorf("test %v incorrect requests %v", tc, allowed.Load()+denied.Load())
	}

	// Calculate the number of rate limit intervals.
	periods := int(testDur / tc.rate)
	if testDur%tc.rate != 0 {
		periods++
	}

	// Verify rate limit.
	if int(allowed.Load()) > periods*tc.limit {
		return fmt.Errorf("test %v failed with %v allowed for %v rate periods", tc, allowed.Load(), periods)
	}

	return nil
}
