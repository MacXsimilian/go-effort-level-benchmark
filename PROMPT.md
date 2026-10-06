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

## Execution and submission

Implement the specification by editing cache.go in the current workspace.
The file on disk is the submission, not your final message.

Preserve the package name, exported API signatures, and EvictReason values.
Use only the Go standard library.
Do not modify any other supplied file or add additional submission files.

You may inspect the supplied files and run local Go tools.
Local testing and revision are allowed within this attempt.

Do not retrieve previous attempts, saved conversations, repository history,
or external solutions. Do not launch additional agents or model clients.

Before finishing, format cache.go with gofmt and run:
go test -race -count=1 ./...

Do not change tests, skip tests, or modify the build/test environment to
influence the result. Implement the specified behavior generally, not only
the particular examples in the visible tests.

Only cache.go will be copied into a separate clean grading environment.

Finish with a brief status message. Do not repeat the source code in your
final response.
