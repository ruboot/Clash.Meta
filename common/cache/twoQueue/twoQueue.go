package twoQ

import (
	"github.com/Dreamacro/clash/common/cache/lru"
	"github.com/samber/lo"
	"sync"
	"time"
)

const (
	// Default2QRecentRatio is the ratio of the twoQueue cache dedicated
	// to recently added entries that have only been accessed once.
	Default2QRecentRatio = 0.25

	// Default2QGhostEntries is the default ratio of ghost
	// entries kept to track entries recently evicted
	Default2QGhostEntries = 0.50
)

type TwoQueueCache[K comparable, V any] struct {
	size       int
	recentSize int

	recent      *lru.LruCache[K, V]
	frequent    *lru.LruCache[K, V]
	recentEvict *lru.LruCache[K, struct{}]
	mu          sync.RWMutex
}

// New2Q creates a new TwoQueueCache using the default
// values for the parameters.
func New[K comparable, V any](size int) (*TwoQueueCache[K, V], error) {
	return New2QParams[K, V](size, Default2QRecentRatio, Default2QGhostEntries), nil
}

// New2QParams creates a new TwoQueueCache using the provided
// parameter values.
func New2QParams[K comparable, V any](size int, recentRatio, ghostRatio float64) *TwoQueueCache[K, V] {
	if size <= 0 {
		return nil
	}
	if recentRatio < 0.0 || recentRatio > 1.0 {
		return nil
	}
	if ghostRatio < 0.0 || ghostRatio > 1.0 {
		return nil
	}

	// Determine the sub-sizes
	recentSize := int(float64(size) * recentRatio)
	evictSize := int(float64(size) * ghostRatio)

	// Initialize the cache
	return &TwoQueueCache[K, V]{
		size:        size,
		recentSize:  recentSize,
		recent:      lru.New[K, V](lru.WithSize[K, V](size)),
		frequent:    lru.New[K, V](lru.WithSize[K, V](size)),
		recentEvict: lru.New[K, struct{}](lru.WithSize[K, struct{}](evictSize)),
	}
}

// Get returns any representation of a cached response and a bool
// set to true if the key was found.
func (c *TwoQueueCache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if this is a frequent value
	if val, ok := c.frequent.Get(key); ok {
		return val, ok
	}

	// If the value is contained in recent, then we
	// promote it to frequent
	if val, ok := c.recent.Peek(key); ok {
		c.recent.Delete(key)
		c.frequent.Set(key, val)
		return val, ok
	}

	return lo.Empty[V](), false
}

// Set stores any representation of a response for a given key.
func (c *TwoQueueCache[K, V]) Set(key K, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if the value is frequently used already,
	// and just update the value
	if c.frequent.Exist(key) {
		c.frequent.Set(key, value)
		return
	}

	// Check if the value is recently used, and promote
	// the value into the frequent list
	if c.recent.Exist(key) {
		c.recent.Delete(key)
		c.frequent.Set(key, value)
		return
	}

	// If the value was recently evicted, add it to the
	// frequently used list
	if c.recentEvict.Exist(key) {
		c.ensureSpace(true)
		c.recentEvict.Delete(key)
		c.frequent.Set(key, value)
		return
	}

	// Add to the recently seen list
	c.ensureSpace(false)
	c.recent.Set(key, value)
}

// ensureSpace is used to ensure we have space in the cache
func (c *TwoQueueCache[K, V]) ensureSpace(recentEvict bool) {
	// If we have space, nothing to do
	recentLen := c.recent.Len()
	freqLen := c.frequent.Len()
	if recentLen+freqLen < c.size {
		return
	}

	// If the recent buffer is larger than
	// the target, evict from there
	if recentLen > 0 && (recentLen > c.recentSize || (recentLen == c.recentSize && !recentEvict)) {
		k := c.recent.DeleteOldest()
		c.recentEvict.Set(k, struct{}{})
		return
	}

	// Remove from the frequent list otherwise
	c.frequent.DeleteOldest()
}

// SetWithExpire stores any representation of a response for a given key and given expires.
// The expires time will round to second.
func (c *TwoQueueCache[K, V]) SetWithExpire(key K, value V, expires time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.frequent.Exist(key) {
		c.frequent.Set(key, value)
		return
	}

	c.recent.SetWithExpire(key, value, expires)
}

// Remove removes the provided key from the cache.
func (c *TwoQueueCache[K, V]) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.frequent.Delete(key)
	c.recent.Delete(key)
	c.recentEvict.Delete(key)
}

// Clear is used to completely clear the cache.
func (c *TwoQueueCache[K, V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.recent.Clear()
	c.frequent.Clear()
	c.recentEvict.Clear()
}
