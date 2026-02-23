package plugin

import (
	"sync"
	"time"
)

// cacheEntry holds a cached value with an expiration time.
type cacheEntry struct {
	data      interface{}
	expiresAt time.Time
}

// Cache provides an in-memory key-value cache with TTL-based expiration.
type Cache struct {
	mu  sync.RWMutex
	ttl time.Duration
	m   map[string]cacheEntry
	stop chan struct{}
}

// NewCache creates a new Cache with the given TTL and starts eviction.
func NewCache(ttl time.Duration) *Cache {
	c := &Cache{
		ttl:  ttl,
		m:    make(map[string]cacheEntry),
		stop: make(chan struct{}),
	}
	go c.evictLoop()
	return c
}

// Get retrieves a value from the cache if it exists and hasn't expired.
func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.m[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.data, true
}

// Set stores a value in the cache with the configured TTL.
func (c *Cache) Set(key string, data interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[key] = cacheEntry{
		data:      data,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// evictLoop periodically removes expired entries.
func (c *Cache) evictLoop() {
	ticker := time.NewTicker(c.ttl / 2)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.evict()
		case <-c.stop:
			return
		}
	}
}

// evict removes all expired entries.
func (c *Cache) evict() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for k, v := range c.m {
		if now.After(v.expiresAt) {
			delete(c.m, k)
		}
	}
}

// Close stops the eviction goroutine and clears the cache.
func (c *Cache) Close() {
	close(c.stop)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m = make(map[string]cacheEntry)
}
