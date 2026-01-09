package server

import (
	"sync"
	"time"

	publicinvoicedomain "github.com/smallbiznis/valora/internal/publicinvoice/domain"
)

type paymentMethodsCache struct {
	ttl   time.Duration
	mu    sync.RWMutex
	items map[string]paymentMethodsCacheEntry
}

type paymentMethodsCacheEntry struct {
	expiresAt time.Time
	methods   []publicinvoicedomain.PublicPaymentMethod
}

func newPaymentMethodsCache(ttl time.Duration) *paymentMethodsCache {
	return &paymentMethodsCache{
		ttl:   ttl,
		items: make(map[string]paymentMethodsCacheEntry),
	}
}

func (c *paymentMethodsCache) Get(key string) ([]publicinvoicedomain.PublicPaymentMethod, bool) {
	if c == nil || key == "" {
		return nil, false
	}
	c.mu.RLock()
	entry, ok := c.items[key]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if time.Now().UTC().After(entry.expiresAt) {
		c.mu.Lock()
		delete(c.items, key)
		c.mu.Unlock()
		return nil, false
	}
	methods := append([]publicinvoicedomain.PublicPaymentMethod(nil), entry.methods...)
	return methods, true
}

func (c *paymentMethodsCache) Set(key string, methods []publicinvoicedomain.PublicPaymentMethod) {
	if c == nil || key == "" {
		return
	}
	cloned := append([]publicinvoicedomain.PublicPaymentMethod(nil), methods...)
	c.mu.Lock()
	c.items[key] = paymentMethodsCacheEntry{
		expiresAt: time.Now().UTC().Add(c.ttl),
		methods:   cloned,
	}
	c.mu.Unlock()
}
