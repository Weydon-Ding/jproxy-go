package cache

import (
	"testing"
	"time"
)

func TestTTLCacheEvictsOldestEntryWhenCapacityIsReached(t *testing.T) {
	// Given
	c := NewTTLCache[string](time.Minute, 2)
	c.Set("first", "one")
	c.Set("second", "two")

	// When
	c.Set("third", "three")

	// Then
	if _, ok := c.Get("first"); ok {
		t.Fatal("first key still exists after capacity eviction")
	}
	if got, ok := c.Get("second"); !ok || got != "two" {
		t.Fatalf("second key = %q, %t; want two, true", got, ok)
	}
	if got, ok := c.Get("third"); !ok || got != "three" {
		t.Fatalf("third key = %q, %t; want three, true", got, ok)
	}
}

func TestTTLCacheReplacementDoesNotEvictAnotherEntry(t *testing.T) {
	// Given
	c := NewTTLCache[string](time.Minute, 2)
	c.Set("first", "one")
	c.Set("second", "two")

	// When
	c.Set("first", "updated")

	// Then
	if got, ok := c.Get("first"); !ok || got != "updated" {
		t.Fatalf("first key = %q, %t; want updated, true", got, ok)
	}
	if got, ok := c.Get("second"); !ok || got != "two" {
		t.Fatalf("second key = %q, %t; want two, true", got, ok)
	}
}

func TestTTLCacheRemovesExpiredEntriesBeforeEvictingLiveEntries(t *testing.T) {
	// Given
	c := NewTTLCache[string](time.Minute, 2)
	now := time.Now()
	c.m["expired"] = ttlItem[string]{value: "old", expiresAt: now.Add(-time.Minute)}
	c.m["live"] = ttlItem[string]{value: "current", expiresAt: now.Add(time.Minute)}
	c.order = []string{"expired", "live"}

	// When
	c.Set("new", "fresh")

	// Then
	if _, ok := c.Get("expired"); ok {
		t.Fatal("expired key still exists")
	}
	if got, ok := c.Get("live"); !ok || got != "current" {
		t.Fatalf("live key = %q, %t; want current, true", got, ok)
	}
	if got, ok := c.Get("new"); !ok || got != "fresh" {
		t.Fatalf("new key = %q, %t; want fresh, true", got, ok)
	}
}

func TestTTLCacheExpiresEntriesByTTL(t *testing.T) {
	// Given
	c := NewTTLCache[string](-time.Nanosecond, 2)
	c.Set("key", "value")

	// When
	got, ok := c.Get("key")

	// Then
	if ok || got != "" {
		t.Fatalf("expired key = %q, %t; want empty, false", got, ok)
	}
}

func TestTTLCacheDelete_removesOnlyNamedEntry_andMissingKeyIsNoOp(t *testing.T) {
	// Given
	c := NewTTLCache[string](time.Minute, 3)
	c.Set("first", "one")
	c.Set("second", "two")

	// When
	c.Delete("first")
	c.Delete("missing")

	// Then
	if _, ok := c.Get("first"); ok {
		t.Fatal("deleted key remains")
	}
	if got, ok := c.Get("second"); !ok || got != "two" {
		t.Fatalf("second key = %q, %t; want two, true", got, ok)
	}
}
