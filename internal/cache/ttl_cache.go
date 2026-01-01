package cache

import (
	"sync"
	"time"
)

// Cache provides a minimal TTL cache interface for hot-path lookups.
type Cache[K comparable, V any] interface {
	Get(key K) (V, bool)
	Set(key K, value V, ttl time.Duration)
	Delete(key K)
}

type cacheEntry[V any] struct {
	value     V
	expiresAt time.Time
}

// TTLCache stores values in-memory with per-entry TTLs.
type TTLCache[K comparable, V any] struct {
	mu    sync.RWMutex
	items map[K]cacheEntry[V]
}

// NewTTLCache constructs a new TTLCache instance.
func NewTTLCache[K comparable, V any]() *TTLCache[K, V] {
	return &TTLCache[K, V]{items: make(map[K]cacheEntry[V])}
}

// Get returns a cached value if it exists and has not expired.
func (c *TTLCache[K, V]) Get(key K) (V, bool) {
	var zero V
	if c == nil {
		return zero, false
	}
	c.mu.RLock()
	entry, ok := c.items[key]
	c.mu.RUnlock()
	if !ok {
		return zero, false
	}
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		c.Delete(key)
		return zero, false
	}
	return entry.value, true
}

// Set stores a value with the provided TTL.
func (c *TTLCache[K, V]) Set(key K, value V, ttl time.Duration) {
	if c == nil {
		return
	}
	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}
	c.mu.Lock()
	c.items[key] = cacheEntry[V]{
		value:     value,
		expiresAt: expiresAt,
	}
	c.mu.Unlock()
}

// Delete removes a cached entry.
func (c *TTLCache[K, V]) Delete(key K) {
	if c == nil {
		return
	}
	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()
}

// NoopCache always returns cache misses and ignores writes.
type NoopCache[K comparable, V any] struct{}

// Get always returns a miss.
func (NoopCache[K, V]) Get(key K) (V, bool) {
	var zero V
	return zero, false
}

// Set is a no-op.
func (NoopCache[K, V]) Set(key K, value V, ttl time.Duration) {}

// Delete is a no-op.
func (NoopCache[K, V]) Delete(key K) {}
