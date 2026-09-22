// Package ratelimit implements an in-memory fixed-window rate limiter.
//
// Limitation: state lives in the process. With several API replicas each
// replica enforces the limit independently, so the effective limit is
// multiplied by the replica count. That is acceptable for the abuse
// protection this template needs; a product that requires exact global
// limits should move the state to PostgreSQL or a shared store.
package ratelimit

import (
	"sync"
	"time"
)

// Limiter allows at most Requests per Window for each key.
type Limiter struct {
	requests int
	window   time.Duration
	now      func() time.Time

	mu      sync.Mutex
	buckets map[string]*bucket
	lastGC  time.Time
}

type bucket struct {
	windowStart time.Time
	count       int
}

// New builds a limiter allowing requests per window for each key.
func New(requests int, window time.Duration) *Limiter {
	return &Limiter{
		requests: requests,
		window:   window,
		now:      time.Now,
		buckets:  map[string]*bucket{},
	}
}

// Allow records one request for key and reports whether it is within the
// limit. When it is not, retryAfter tells how long until the window resets.
func (l *Limiter) Allow(key string) (allowed bool, retryAfter time.Duration) {
	now := l.now()

	l.mu.Lock()
	defer l.mu.Unlock()

	l.gc(now)

	b, ok := l.buckets[key]
	if !ok || now.Sub(b.windowStart) >= l.window {
		b = &bucket{windowStart: now}
		l.buckets[key] = b
	}
	if b.count >= l.requests {
		return false, b.windowStart.Add(l.window).Sub(now)
	}
	b.count++
	return true, 0
}

// gc drops expired buckets at most once per window so memory stays bounded
// by the number of distinct keys seen in the last window.
func (l *Limiter) gc(now time.Time) {
	if now.Sub(l.lastGC) < l.window {
		return
	}
	l.lastGC = now
	for key, b := range l.buckets {
		if now.Sub(b.windowStart) >= l.window {
			delete(l.buckets, key)
		}
	}
}
