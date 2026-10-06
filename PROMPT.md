# Task prompt (give this exact text to every model / effort level)

Implement the Go package in `cache.go` so that every test in `cache_test.go`
passes with `go test -race -count=1 ./...`. Do not edit `cache_test.go`.
Use only the Go standard library. Go 1.21+, generics allowed.

## Specification

A fixed-capacity, TTL-aware, thread-safe LRU cache:

    func New[K comparable, V any](capacity int, ttl time.Duration,
        now func() time.Time,
        onEvict func(key K, value V, reason EvictReason)) *Cache[K, V]
    func (c *Cache[K, V]) Set(key K, value V)
    func (c *Cache[K, V]) Get(key K) (V, bool)
    func (c *Cache[K, V]) Delete(key K) bool
    func (c *Cache[K, V]) Len() int

Rules:

1. `New` panics if capacity <= 0 or ttl <= 0. A nil `now` means `time.Now`.
   A nil `onEvict` means no callback.
2. An entry is expired once `now >= setTime + ttl` (exactly ttl elapsed = expired).
3. `Set` on a new key inserts it as most recently used. `Set` on an existing
   key updates the value, resets its TTL, marks it most recently used, and
   never evicts anything.
4. `Get` on a live entry marks it most recently used but does NOT extend its TTL.
   `Get` on an expired entry removes it, reports a miss, and fires
   `onEvict` with `EvictExpired`.
5. When `Set` adds a new key and the cache is full, first remove ALL expired
   entries (callback `EvictExpired` each). If it is still full, evict the
   single least-recently-used entry (callback `EvictCapacity`).
6. `Len` returns the number of live entries. It removes any expired entries
   it finds (callback `EvictExpired`).
7. `Delete` removes the key. It returns true only if a live entry was removed.
   If the entry was already expired it is removed with an `EvictExpired`
   callback and `Delete` returns false. Deleting a live entry never fires
   the callback.
8. The cache is safe for concurrent use.
9. `onEvict` must be called AFTER the operation has completed and the internal
   lock has been released, so the callback may safely call back into the cache.

Return only the complete contents of `cache.go`.
