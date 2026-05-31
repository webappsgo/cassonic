package cache

import (
	"sync"
	"testing"
	"time"
)

func TestSetGet(t *testing.T) {
	c := New[string, int](10, 0)

	c.Set("a", 1)
	got, ok := c.Get("a")
	if !ok {
		t.Fatal("Get: key not found after Set")
	}
	if got != 1 {
		t.Errorf("Get: got %d, want 1", got)
	}
}

func TestGetMissing(t *testing.T) {
	c := New[string, int](10, 0)

	got, ok := c.Get("missing")
	if ok {
		t.Errorf("Get missing key: got ok=true, want false")
	}
	if got != 0 {
		t.Errorf("Get missing key: got %d, want 0 (zero value)", got)
	}
}

func TestEvictionAtCapacity(t *testing.T) {
	c := New[int, int](3, 0)

	c.Set(1, 1)
	c.Set(2, 2)
	c.Set(3, 3)

	if c.Len() != 3 {
		t.Fatalf("Len: got %d, want 3", c.Len())
	}

	c.Set(4, 4)

	if c.Len() != 3 {
		t.Errorf("after eviction Len: got %d, want 3", c.Len())
	}

	_, ok := c.Get(1)
	if ok {
		t.Error("evicted key 1 should not be present")
	}

	_, ok = c.Get(4)
	if !ok {
		t.Error("newest key 4 should be present")
	}
}

func TestLRUOrder(t *testing.T) {
	c := New[int, int](3, 0)

	c.Set(1, 1)
	c.Set(2, 2)
	c.Set(3, 3)

	c.Get(1)

	c.Set(4, 4)

	_, ok := c.Get(2)
	if ok {
		t.Error("key 2 should have been evicted (LRU)")
	}
	_, ok = c.Get(1)
	if !ok {
		t.Error("key 1 should still be present (was accessed recently)")
	}
}

func TestTTLExpiry(t *testing.T) {
	c := New[string, int](10, 50*time.Millisecond)

	c.Set("x", 42)

	got, ok := c.Get("x")
	if !ok || got != 42 {
		t.Fatalf("Get before expiry: got (%d, %v), want (42, true)", got, ok)
	}

	time.Sleep(100 * time.Millisecond)

	_, ok = c.Get("x")
	if ok {
		t.Error("Get after TTL expiry: got ok=true, want false")
	}
}

func TestSetTTLOverride(t *testing.T) {
	c := New[string, int](10, time.Hour)

	c.SetTTL("short", 99, 50*time.Millisecond)

	got, ok := c.Get("short")
	if !ok || got != 99 {
		t.Fatalf("Get before expiry: got (%d, %v), want (99, true)", got, ok)
	}

	time.Sleep(100 * time.Millisecond)

	_, ok = c.Get("short")
	if ok {
		t.Error("key with short TTL should have expired")
	}
}

func TestNoTTL(t *testing.T) {
	c := New[string, int](10, 0)
	c.Set("permanent", 7)

	time.Sleep(10 * time.Millisecond)

	got, ok := c.Get("permanent")
	if !ok || got != 7 {
		t.Errorf("key with no TTL should not expire: got (%d, %v)", got, ok)
	}
}

func TestDelete(t *testing.T) {
	c := New[string, int](10, 0)

	c.Set("del", 100)
	c.Delete("del")

	_, ok := c.Get("del")
	if ok {
		t.Error("key should not be present after Delete")
	}
	if c.Len() != 0 {
		t.Errorf("Len after Delete: got %d, want 0", c.Len())
	}
}

func TestDeleteNonExistent(t *testing.T) {
	c := New[string, int](10, 0)
	c.Delete("nonexistent")
	if c.Len() != 0 {
		t.Errorf("Len after deleting nonexistent: got %d, want 0", c.Len())
	}
}

func TestClear(t *testing.T) {
	c := New[string, int](10, 0)

	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)

	c.Clear()

	if c.Len() != 0 {
		t.Errorf("Len after Clear: got %d, want 0", c.Len())
	}

	_, ok := c.Get("a")
	if ok {
		t.Error("key should not be present after Clear")
	}
}

func TestLen(t *testing.T) {
	c := New[int, int](10, 0)

	if c.Len() != 0 {
		t.Errorf("empty Len: got %d, want 0", c.Len())
	}
	c.Set(1, 1)
	if c.Len() != 1 {
		t.Errorf("Len after 1 Set: got %d, want 1", c.Len())
	}
	c.Set(2, 2)
	c.Set(3, 3)
	if c.Len() != 3 {
		t.Errorf("Len after 3 Sets: got %d, want 3", c.Len())
	}
}

func TestUpdate(t *testing.T) {
	c := New[string, int](10, 0)

	c.Set("k", 1)
	c.Set("k", 2)

	got, ok := c.Get("k")
	if !ok {
		t.Fatal("key missing after update")
	}
	if got != 2 {
		t.Errorf("Get after update: got %d, want 2", got)
	}
	if c.Len() != 1 {
		t.Errorf("Len after update: got %d, want 1", c.Len())
	}
}

func TestConcurrentAccess(t *testing.T) {
	t.Parallel()

	c := New[int, int](100, 0)
	var wg sync.WaitGroup
	const goroutines = 50
	const ops = 200

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < ops; j++ {
				key := (id*ops + j) % 50
				c.Set(key, j)
				c.Get(key)
			}
		}(i)
	}

	wg.Wait()
}

func TestConcurrentDeleteAndGet(t *testing.T) {
	t.Parallel()

	c := New[string, int](50, 0)
	for i := 0; i < 20; i++ {
		c.Set("key", i)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			c.Delete("key")
		}()
		go func(v int) {
			defer wg.Done()
			c.Set("key", v)
		}(i)
	}
	wg.Wait()
}

func TestMinCapacity(t *testing.T) {
	c := New[int, int](0, 0)

	c.Set(1, 1)
	c.Set(2, 2)

	if c.Len() > 1 {
		t.Errorf("capacity was clamped to 1, Len should be 1, got %d", c.Len())
	}
}

func TestEvict(t *testing.T) {
	c := New[string, int](10, 50*time.Millisecond)

	c.Set("a", 1)
	c.Set("b", 2)

	time.Sleep(100 * time.Millisecond)

	c.Set("c", 3)

	c.Evict()

	if c.Len() != 1 {
		t.Errorf("after Evict Len: got %d, want 1", c.Len())
	}
	_, ok := c.Get("c")
	if !ok {
		t.Error("non-expired key c should still be present after Evict")
	}
}
