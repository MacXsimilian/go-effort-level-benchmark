package cache

import "time"

// EvictReason says why an entry left the cache without an explicit Delete.
type EvictReason int

const (
	EvictCapacity EvictReason = iota + 1 // removed as least-recently-used to make room
	EvictExpired                         // removed because its TTL elapsed
)

// Cache is a fixed-capacity, TTL-aware, thread-safe LRU cache.
//
// TODO: implement. See PROMPT.md for the full specification.
type Cache[K comparable, V any] struct {
	// your fields here
}

// New creates a Cache. See PROMPT.md for the rules.
func New[K comparable, V any](capacity int, ttl time.Duration, now func() time.Time, onEvict func(key K, value V, reason EvictReason)) *Cache[K, V] {
	panic("not implemented")
}

// Set inserts or updates a key.
func (c *Cache[K, V]) Set(key K, value V) { panic("not implemented") }

// Get returns the value for key if present and not expired.
func (c *Cache[K, V]) Get(key K) (V, bool) { panic("not implemented") }

// Delete removes key. It reports whether a live (non-expired) entry was removed.
func (c *Cache[K, V]) Delete(key K) bool { panic("not implemented") }

// Len returns the number of live (non-expired) entries.
func (c *Cache[K, V]) Len() int { panic("not implemented") }
