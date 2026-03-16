package cache

import (
	"sync"
	"time"
)

// Cache provides thread-safe in-memory caching with TTL
type Cache struct {
	mu    sync.RWMutex
	items map[string]cacheItem
}

type cacheItem struct {
	value     string
	expiresAt time.Time
}

// New creates a new cache instance
func New() *Cache {
	return &Cache{
		items: make(map[string]cacheItem),
	}
}

// Get retrieves a value from the cache
// Returns the value and true if found and not expired, empty string and false otherwise
func (c *Cache) Get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, ok := c.items[key]
	if !ok {
		return "", false
	}

	if time.Now().After(item.expiresAt) {
		return "", false
	}

	return item.value, true
}

// Set stores a value in the cache with the given TTL
func (c *Cache) Set(key, value string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.items) > 50 {
		c.cleanExpired()
	}

	c.items[key] = cacheItem{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
}

// CleanExpired removes all expired entries from the cache
func (c *Cache) CleanExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cleanExpired()
}

// cleanExpired removes expired entries (must be called with lock held)
func (c *Cache) cleanExpired() {
	now := time.Now()
	for k, v := range c.items {
		if now.After(v.expiresAt) {
			delete(c.items, k)
		}
	}
}

// IsStale returns true if the key is missing or expired
func (c *Cache) IsStale(key string, ttl time.Duration) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, ok := c.items[key]
	if !ok {
		return true
	}

	return time.Now().After(item.expiresAt)
}

// Delete removes a key from the cache
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

// DeleteByPrefix removes all keys with the given prefix
func (c *Cache) DeleteByPrefix(prefix string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key := range c.items {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			delete(c.items, key)
		}
	}
}

// Clear removes all items from the cache
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]cacheItem)
}

// Common TTL constants
const (
	GitTTL      = 2 * time.Second
	ProcessTTL  = 2 * time.Second
	ConfigTTL   = 10 * time.Second
	WorktreeTTL      = 5 * time.Minute // Worktree status rarely changes
	SpotifyTTL       = 3 * time.Second // Fast enough for track changes
	SupabaseTTL      = 3 * time.Second // Local stack status can change
	VercelDeployTTL  = 10 * time.Second
	VercelBuildTTL   = 3 * time.Second  // Faster polling during transient states
	VercelProjectTTL = 5 * time.Minute  // Project linkage rarely changes
	VercelTeamTTL    = 5 * time.Minute  // Team/user context rarely changes
)
