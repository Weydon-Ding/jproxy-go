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
	mu  sync.RWMutex
	ttl time.Duration
	m   map[string]ttlItem[T]
}

func NewTTLCache[T any](ttl time.Duration) *TTLCache[T] {
	return &TTLCache[T]{ttl: ttl, m: make(map[string]ttlItem[T])}
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
		delete(c.m, key)
		c.mu.Unlock()
		return zero, false
	}
	return item.value, true
}

func (c *TTLCache[T]) Set(key string, val T) {
	c.mu.Lock()
	c.m[key] = ttlItem[T]{value: val, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

func (c *TTLCache[T]) Clear() {
	c.mu.Lock()
	c.m = make(map[string]ttlItem[T])
	c.mu.Unlock()
}
