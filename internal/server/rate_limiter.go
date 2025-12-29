package server

import (
	"sync"
	"time"
)

type rateLimiter struct {
	limit  int
	window time.Duration
	mu     sync.Mutex
	items  map[string]*rateLimitEntry
}

type rateLimitEntry struct {
	windowStart time.Time
	count       int
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		limit:  limit,
		window: window,
		items:  make(map[string]*rateLimitEntry),
	}
}

func (r *rateLimiter) Allow(key string) bool {
	if key == "" {
		return false
	}

	now := time.Now().UTC()
	r.mu.Lock()
	defer r.mu.Unlock()

	entry := r.items[key]
	if entry == nil || now.Sub(entry.windowStart) > r.window {
		entry = &rateLimitEntry{windowStart: now}
		r.items[key] = entry
	}

	if entry.count >= r.limit {
		return false
	}

	entry.count++
	return true
}
