package cache

import (
	"math/rand"
	"reflect"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ---- test helpers (do not edit) ----

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newClock() *fakeClock { return &fakeClock{t: time.Unix(1_000_000, 0)} }
func (f *fakeClock) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.t
}
func (f *fakeClock) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.t = f.t.Add(d)
}

type event struct {
	Key    string
	Val    int
	Reason EvictReason
}

type recorder struct {
	mu sync.Mutex
	ev []event
}

func (r *recorder) fn(k string, v int, why EvictReason) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ev = append(r.ev, event{k, v, why})
}

// take returns the recorded events (sorted by key) and clears them.
func (r *recorder) take() []event {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.ev
	r.ev = nil
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

func mustGet(t *testing.T, c *Cache[string, int], k string, want int) {
	t.Helper()
	v, ok := c.Get(k)
	if !ok || v != want {
		t.Fatalf("Get(%q) = (%d, %v), want (%d, true)", k, v, ok, want)
	}
}

func mustMiss(t *testing.T, c *Cache[string, int], k string) {
	t.Helper()
	if v, ok := c.Get(k); ok || v != 0 {
		t.Fatalf("Get(%q) = (%d, %v), want (0, false)", k, v, ok)
	}
}

func wantEvents(t *testing.T, r *recorder, want ...event) {
	t.Helper()
	got := r.take()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("events = %+v, want %+v", got, want)
	}
}

// ---- tests ----

func TestBasicSetGet(t *testing.T) {
	clk := newClock()
	c := New[string, int](3, time.Minute, clk.Now, nil)
	mustMiss(t, c, "a")
	c.Set("a", 1)
	c.Set("b", 2)
	mustGet(t, c, "a", 1)
	mustGet(t, c, "b", 2)
	if c.Len() != 2 {
		t.Fatalf("Len = %d, want 2", c.Len())
	}
}

func TestConstructorPanics(t *testing.T) {
	for name, f := range map[string]func(){
		"zero capacity":     func() { New[string, int](0, time.Second, nil, nil) },
		"negative capacity": func() { New[string, int](-1, time.Second, nil, nil) },
		"zero ttl":          func() { New[string, int](1, 0, nil, nil) },
		"negative ttl":      func() { New[string, int](1, -time.Second, nil, nil) },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: expected panic", name)
				}
			}()
			f()
		}()
	}
}

func TestNilClockUsesRealTime(t *testing.T) {
	c := New[string, int](2, time.Hour, nil, nil)
	c.Set("a", 1)
	mustGet(t, c, "a", 1)
}

func TestLRUEvictionOrder(t *testing.T) {
	clk := newClock()
	rec := &recorder{}
	c := New[string, int](2, time.Hour, clk.Now, rec.fn)
	c.Set("a", 1)
	c.Set("b", 2)
	mustGet(t, c, "a", 1) // a is now most recently used
	c.Set("c", 3)         // must evict b
	wantEvents(t, rec, event{"b", 2, EvictCapacity})
	mustMiss(t, c, "b")
	mustGet(t, c, "a", 1)
	mustGet(t, c, "c", 3)
}

func TestSetExistingKeyUpdatesWithoutEviction(t *testing.T) {
	clk := newClock()
	rec := &recorder{}
	c := New[string, int](2, time.Hour, clk.Now, rec.fn)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("a", 10) // update, not insert: no eviction, a becomes MRU
	wantEvents(t, rec)
	if c.Len() != 2 {
		t.Fatalf("Len = %d, want 2", c.Len())
	}
	c.Set("c", 3) // evicts b (LRU), not a
	wantEvents(t, rec, event{"b", 2, EvictCapacity})
	mustGet(t, c, "a", 10)
}

func TestExpiryBoundaryIsInclusive(t *testing.T) {
	clk := newClock()
	rec := &recorder{}
	c := New[string, int](2, 10*time.Second, clk.Now, rec.fn)
	c.Set("a", 1)
	clk.Advance(10*time.Second - time.Nanosecond)
	mustGet(t, c, "a", 1)
	clk.Advance(time.Nanosecond) // exactly ttl elapsed => expired
	mustMiss(t, c, "a")
	wantEvents(t, rec, event{"a", 1, EvictExpired})
	if c.Len() != 0 {
		t.Fatalf("Len = %d, want 0", c.Len())
	}
}

func TestGetDoesNotExtendTTL(t *testing.T) {
	clk := newClock()
	c := New[string, int](2, 10*time.Second, clk.Now, nil)
	c.Set("a", 1)
	clk.Advance(6 * time.Second)
	mustGet(t, c, "a", 1)
	clk.Advance(5 * time.Second) // 11s since Set
	mustMiss(t, c, "a")
}

func TestSetResetsTTL(t *testing.T) {
	clk := newClock()
	c := New[string, int](2, 10*time.Second, clk.Now, nil)
	c.Set("a", 1)
	clk.Advance(6 * time.Second)
	c.Set("a", 2)
	clk.Advance(6 * time.Second) // 12s since first Set, 6s since second
	mustGet(t, c, "a", 2)
}

func TestFullCachePrefersEvictingExpiredOverLRU(t *testing.T) {
	clk := newClock()
	rec := &recorder{}
	c := New[string, int](3, 10*time.Second, clk.Now, rec.fn)
	c.Set("a", 1) // t=0, expires t=10
	clk.Advance(6 * time.Second)
	c.Set("b", 2) // t=6, expires t=16
	c.Set("c", 3)
	mustGet(t, c, "a", 1) // a is MRU but will expire first
	clk.Advance(5 * time.Second) // t=11: a expired, b and c alive
	c.Set("d", 4)                // full => drop expired a, NOT live LRU b
	wantEvents(t, rec, event{"a", 1, EvictExpired})
	mustGet(t, c, "b", 2)
	mustGet(t, c, "c", 3)
	mustGet(t, c, "d", 4)
}

func TestFullCacheDropsAllExpiredThenLRUIfStillFull(t *testing.T) {
	clk := newClock()
	rec := &recorder{}
	c := New[string, int](3, 10*time.Second, clk.Now, rec.fn)
	c.Set("a", 1)
	c.Set("b", 2)
	clk.Advance(6 * time.Second)
	c.Set("c", 3)
	clk.Advance(5 * time.Second) // a, b expired; c alive
	c.Set("d", 4)
	c.Set("e", 5) // room exists now (c, d, e) => no capacity eviction
	wantEvents(t, rec, event{"a", 1, EvictExpired}, event{"b", 2, EvictExpired})
	c.Set("f", 6) // full of live entries => LRU (c) evicted
	wantEvents(t, rec, event{"c", 3, EvictCapacity})
}

func TestLenIgnoresAndPurgesExpired(t *testing.T) {
	clk := newClock()
	rec := &recorder{}
	c := New[string, int](5, 10*time.Second, clk.Now, rec.fn)
	c.Set("a", 1)
	clk.Advance(6 * time.Second)
	c.Set("b", 2)
	clk.Advance(5 * time.Second)
	if n := c.Len(); n != 1 {
		t.Fatalf("Len = %d, want 1", n)
	}
	wantEvents(t, rec, event{"a", 1, EvictExpired})
}

func TestDelete(t *testing.T) {
	clk := newClock()
	rec := &recorder{}
	c := New[string, int](3, 10*time.Second, clk.Now, rec.fn)
	c.Set("a", 1)
	c.Set("b", 2)
	if !c.Delete("a") {
		t.Fatal("Delete(a) = false, want true")
	}
	if c.Delete("a") {
		t.Fatal("second Delete(a) = true, want false")
	}
	wantEvents(t, rec) // explicit Delete never calls the callback
	clk.Advance(10 * time.Second)
	if c.Delete("b") { // b already expired: not a live entry
		t.Fatal("Delete(expired b) = true, want false")
	}
	wantEvents(t, rec, event{"b", 2, EvictExpired})
	if c.Len() != 0 {
		t.Fatalf("Len = %d, want 0", c.Len())
	}
}

func TestCallbackMayCallBackIntoCache(t *testing.T) {
	clk := newClock()
	var c *Cache[string, int]
	lens := []int{}
	c = New[string, int](1, time.Hour, clk.Now, func(k string, v int, why EvictReason) {
		lens = append(lens, c.Len()) // would deadlock if lock is held
		c.Get("zzz")
	})
	done := make(chan struct{})
	go func() {
		c.Set("a", 1)
		c.Set("b", 2) // evicts a, callback re-enters the cache
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("deadlock: callback must run after the lock is released")
	}
	// The callback runs after the operation completes, so b is already stored.
	if !reflect.DeepEqual(lens, []int{1}) {
		t.Fatalf("Len observed in callback = %v, want [1]", lens)
	}
}

func TestCapacityNeverExceededRandomOps(t *testing.T) {
	clk := newClock()
	c := New[int, int](10, 5*time.Second, clk.Now, nil)
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 20000; i++ {
		k := rng.Intn(40)
		switch rng.Intn(5) {
		case 0, 1:
			c.Set(k, i)
		case 2:
			c.Get(k)
		case 3:
			c.Delete(k)
		case 4:
			clk.Advance(time.Duration(rng.Intn(1500)) * time.Millisecond)
		}
		if n := c.Len(); n > 10 || n < 0 {
			t.Fatalf("step %d: Len = %d, out of range", i, n)
		}
	}
}

func TestConcurrentUse(t *testing.T) {
	var evictions int64
	c := New[int, int](50, 50*time.Millisecond, nil, func(k, v int, why EvictReason) {
		atomic.AddInt64(&evictions, 1)
	})
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(seed int64) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(seed))
			for i := 0; i < 2000; i++ {
				k := rng.Intn(100)
				switch rng.Intn(4) {
				case 0:
					c.Set(k, i)
				case 1:
					c.Get(k)
				case 2:
					c.Delete(k)
				case 3:
					c.Len()
				}
			}
		}(int64(g))
	}
	wg.Wait()
	if n := c.Len(); n > 50 {
		t.Fatalf("Len = %d, exceeds capacity 50", n)
	}
}
