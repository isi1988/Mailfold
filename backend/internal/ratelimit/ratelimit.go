// Package ratelimit provides a small, dependency-free per-key rate limiter used
// to throttle sensitive endpoints (primarily login) against brute-force abuse.
package ratelimit

import (
	"sync"
	"time"
)

// entry tracks how many requests a key has made within the current window.
type entry struct {
	count       int       // requests seen so far in the active window
	windowStart time.Time // when the active window began
}

// Limiter is a per-key fixed-window rate limiter, safe for concurrent use.
//
// Each key (typically a client IP) may make at most max requests within each
// window. A fixed window is deliberately chosen over a token bucket or sliding
// log: for a single admin backend it is more than adequate, trivial to reason
// about, and cheap in both memory and CPU.
type Limiter struct {
	max    int
	window time.Duration
	now    func() time.Time // injectable clock, overridden in tests

	mu      sync.Mutex
	entries map[string]*entry
}

// New creates a Limiter allowing max requests per window per key. A max of 0 or
// less disables limiting entirely (Allow always permits the request).
func New(max int, window time.Duration) *Limiter {
	return &Limiter{
		max:     max,
		window:  window,
		now:     time.Now,
		entries: make(map[string]*entry),
	}
}

// Allow records a request for key and reports whether it is permitted. When the
// request is denied it also returns how long the caller should wait before the
// current window resets (suitable for a Retry-After header).
func (l *Limiter) Allow(key string) (bool, time.Duration) {
	if l.max <= 0 {
		return true, 0
	}
	now := l.now()

	l.mu.Lock()
	defer l.mu.Unlock()

	e, ok := l.entries[key]
	if !ok || now.Sub(e.windowStart) >= l.window {
		// First request from this key, or the previous window has elapsed: start
		// a fresh window.
		l.entries[key] = &entry{count: 1, windowStart: now}
		return true, 0
	}
	if e.count >= l.max {
		// Budget exhausted for the current window; report the remaining wait.
		return false, l.window - now.Sub(e.windowStart)
	}
	e.count++
	return true, 0
}

// GC removes entries whose window has fully elapsed, bounding memory use over
// time. It is safe to call periodically from a background goroutine.
func (l *Limiter) GC() {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	for key, e := range l.entries {
		if now.Sub(e.windowStart) >= l.window {
			delete(l.entries, key)
		}
	}
}
