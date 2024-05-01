package glow

// Rate limiter utility using a sliding window approach.

import (
	"sync"
	"time"
)

// Rate limiter, to allow a maximum number of requests for
// a time duration.
type RateLimiter struct {
	limit int           // Maximum number of requests allowed
	rate  time.Duration // Request rate for max requests
	reqs  []time.Time   // Request list
	mu    sync.Mutex
}

// Create a new RateLimiter.
func NewRateLimiter(limit int, rate time.Duration) *RateLimiter {
	return &RateLimiter{
		limit: limit,
		rate:  rate,
		reqs:  make([]time.Time, 0),
	}
}

// Ask the rate limiter if a request is allowed.
// Returns true if allowed, false otherwise.
func (r *RateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()

	// expire requests
	exp := now.Add(-r.rate)
	idx := -1
	for i, t := range r.reqs {
		if t.After(exp) {
			idx = i
			break
		}
	}

	if idx == -1 {
		r.reqs = r.reqs[:0] // Clear the list since none of the entries were after the expiry time
	} else {
		r.reqs = r.reqs[idx:] // Retain starting at first index after the expiry time
	}

	if len(r.reqs) < r.limit {
		r.reqs = append(r.reqs, now)
		return true
	}

	return false
}
