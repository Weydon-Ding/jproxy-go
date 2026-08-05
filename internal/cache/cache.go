package cache

import (
	"sync"
	"time"
)

type ttlItem[T any] struct {
	value     T
	expiresAt time.Time
}

type TTLCache[T any] struct {
	mu         sync.RWMutex
	ttl        time.Duration
	maxEntries int
	m          map[string]ttlItem[T]
	order      []string
}

func NewTTLCache[T any](ttl time.Duration, maxEntries int) *TTLCache[T] {
	if maxEntries < 1 {
		maxEntries = 1
	}
	return &TTLCache[T]{ttl: ttl, maxEntries: maxEntries, m: make(map[string]ttlItem[T])}
}

func (c *TTLCache[T]) Get(key string) (T, bool) {
	c.mu.RLock()
	item, ok := c.m[key]
	c.mu.RUnlock()
	var zero T
	if !ok {
		return zero, false
	}
	if time.Now().After(item.expiresAt) {
		c.mu.Lock()
		c.deleteLocked(key)
		c.mu.Unlock()
		return zero, false
	}
	return item.value, true
}

func (c *TTLCache[T]) Set(key string, val T) {
	c.mu.Lock()
	now := time.Now()
	if _, ok := c.m[key]; !ok {
		c.removeExpiredLocked(now)
		for len(c.m) >= c.maxEntries {
			c.evictOldestLocked()
		}
		c.order = append(c.order, key)
	}
	c.m[key] = ttlItem[T]{value: val, expiresAt: now.Add(c.ttl)}
	c.mu.Unlock()
}

func (c *TTLCache[T]) Clear() {
	c.mu.Lock()
	c.m = make(map[string]ttlItem[T])
	c.order = nil
	c.mu.Unlock()
}

// Delete removes one named entry. Deleting a missing entry is intentionally a no-op.
func (c *TTLCache[T]) Delete(key string) {
	c.mu.Lock()
	c.deleteLocked(key)
	c.mu.Unlock()
}

func (c *TTLCache[T]) removeExpiredLocked(now time.Time) {
	kept := c.order[:0]
	for _, key := range c.order {
		item, ok := c.m[key]
		if !ok {
			continue
		}
		if now.After(item.expiresAt) {
			delete(c.m, key)
			continue
		}
		kept = append(kept, key)
	}
	c.order = kept
}

func (c *TTLCache[T]) evictOldestLocked() {
	for len(c.order) > 0 {
		key := c.order[0]
		c.order = c.order[1:]
		if _, ok := c.m[key]; ok {
			delete(c.m, key)
			return
		}
	}
}

func (c *TTLCache[T]) deleteLocked(key string) {
	delete(c.m, key)
	for i, existing := range c.order {
		if existing == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			return
		}
	}
}
