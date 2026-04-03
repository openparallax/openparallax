// Package shield implements the multi-tier security evaluation pipeline.
package shield

import (
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter.
type RateLimiter struct {
	rate     int
	tokens   int
	lastFill time.Time
	mu       sync.Mutex
}

// NewRateLimiter creates a rate limiter that allows rate actions per minute.
func NewRateLimiter(rate int) *RateLimiter {
	return &RateLimiter{
		rate:     rate,
		tokens:   rate,
		lastFill: time.Now(),
	}
}

// Allow returns true if the action is within the rate limit.
// Refills tokens based on elapsed time since last check.
func (r *RateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(r.lastFill)
	refill := int(elapsed.Minutes() * float64(r.rate))
	if refill > 0 {
		r.tokens += refill
		if r.tokens > r.rate {
			r.tokens = r.rate
		}
		r.lastFill = now
	}

	if r.tokens <= 0 {
		return false
	}
	r.tokens--
	return true
}
