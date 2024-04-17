package glow

// Rate limiter utility using a sliding window approach.

import (
	"sync"
	"time"
)

type RateLimiter struct {
	limit int           // Maximum number of requests allowed
	rate  time.Duration // Request rate for max requests
	reqs  []time.Time   // Request list
	mu    sync.Mutex
}

// Creates a new RateLimiter with limit requests per rate allowed.
func NewRateLimiter(limit int, rate time.Duration) *RateLimiter {
	return &RateLimiter{
		limit: limit,
		rate:  rate,
		reqs:  make([]time.Time, 0),
	}
}

// Test the rate limiter to decide if a call is allowed.
// Returns true if allowed, false otherwise.
func (r *RateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	cur := time.Now()

	// expire requests
	exp := cur.Add(-r.rate)
	idx := 0
	for i, t := range r.reqs {
		if t.After(exp) {
			idx = i
			break
		}
	}
	r.reqs = r.reqs[idx:]

	if len(r.reqs) < r.limit {
		r.reqs = append(r.reqs, cur)
		return true
	}

	return false
}
