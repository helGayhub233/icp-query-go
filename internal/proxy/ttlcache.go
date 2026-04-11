package proxy

import (
	"sync"
	"time"
)

// TTLCache is a generic thread-safe cache with per-entry TTL.
type TTLCache[K comparable, V any] struct {
	mu    sync.RWMutex
	items map[K]ttlItem[V]
}

type ttlItem[V any] struct {
	value     V
	expiresAt time.Time
}

// NewTTLCache creates a new TTLCache.
func NewTTLCache[K comparable, V any]() *TTLCache[K, V] {
	return &TTLCache[K, V]{
		items: make(map[K]ttlItem[V]),
	}
}

// Set stores a value with the given TTL.
func (c *TTLCache[K, V]) Set(key K, value V, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = ttlItem[V]{value: value, expiresAt: time.Now().Add(ttl)}
}

// Get retrieves a value if it hasn't expired.
func (c *TTLCache[K, V]) Get(key K) (V, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, ok := c.items[key]
	if !ok || time.Now().After(item.expiresAt) {
		var zero V
		return zero, false
	}
	return item.value, true
}

// Delete removes an entry.
func (c *TTLCache[K, V]) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

// Len returns the number of non-expired entries.
func (c *TTLCache[K, V]) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now()
	count := 0
	for _, item := range c.items {
		if now.Before(item.expiresAt) {
			count++
		}
	}
	return count
}

// Keys returns all non-expired keys.
func (c *TTLCache[K, V]) Keys() []K {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now()
	var keys []K
	for k, item := range c.items {
		if now.Before(item.expiresAt) {
			keys = append(keys, k)
		}
	}
	return keys
}

// Clean removes all expired entries. Returns the number of entries removed.
func (c *TTLCache[K, V]) Clean() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	removed := 0
	for k, item := range c.items {
		if now.After(item.expiresAt) {
			delete(c.items, k)
			removed++
		}
	}
	return removed
}

// StartCleanupGoroutine periodically removes expired entries.
// Returns a stop channel. Close it to stop the cleanup goroutine.
func (c *TTLCache[K, V]) StartCleanupGoroutine(interval time.Duration) chan struct{} {
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				c.Clean()
			case <-stop:
				return
			}
		}
	}()
	return stop
}
