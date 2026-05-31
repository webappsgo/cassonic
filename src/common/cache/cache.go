package cache

import (
	"sync"
	"time"
)

// node is a doubly-linked list node used for LRU ordering.
type node[K comparable, V any] struct {
	key   K
	entry Entry[V]
	prev  *node[K, V]
	next  *node[K, V]
}

// Entry holds a cached value and its expiry time.
type Entry[V any] struct {
	Value     V
	ExpiresAt time.Time
}

// IsExpired returns true if the entry has passed its TTL.
// Entries with a zero ExpiresAt never expire.
func (e Entry[V]) IsExpired() bool {
	if e.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(e.ExpiresAt)
}

// Cache is a thread-safe generic LRU cache with optional TTL support.
type Cache[K comparable, V any] struct {
	mu       sync.Mutex
	capacity int
	ttl      time.Duration
	items    map[K]*node[K, V]
	head     *node[K, V]
	tail     *node[K, V]
}

// New creates a new Cache with the given capacity and default TTL.
// A ttl of 0 means entries never expire.
func New[K comparable, V any](capacity int, ttl time.Duration) *Cache[K, V] {
	if capacity < 1 {
		capacity = 1
	}
	c := &Cache[K, V]{
		capacity: capacity,
		ttl:      ttl,
		items:    make(map[K]*node[K, V], capacity),
	}
	c.head = &node[K, V]{}
	c.tail = &node[K, V]{}
	c.head.next = c.tail
	c.tail.prev = c.head
	return c
}

// Set adds or updates a value in the cache using the default TTL.
func (c *Cache[K, V]) Set(key K, value V) {
	c.SetTTL(key, value, c.ttl)
}

// SetTTL adds or updates a value with a custom TTL that overrides the default.
// A ttl of 0 means the entry never expires.
func (c *Cache[K, V]) SetTTL(key K, value V, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}

	if n, ok := c.items[key]; ok {
		n.entry = Entry[V]{Value: value, ExpiresAt: expiresAt}
		c.moveToFront(n)
		return
	}

	n := &node[K, V]{
		key:   key,
		entry: Entry[V]{Value: value, ExpiresAt: expiresAt},
	}
	c.items[key] = n
	c.pushFront(n)

	if len(c.items) > c.capacity {
		c.removeLRU()
	}
}

// Get retrieves a value from the cache.
// Returns the zero value and false if the key is not found or the entry has expired.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	n, ok := c.items[key]
	if !ok {
		var zero V
		return zero, false
	}

	if n.entry.IsExpired() {
		c.removeNode(n)
		delete(c.items, key)
		var zero V
		return zero, false
	}

	c.moveToFront(n)
	return n.entry.Value, true
}

// Delete removes a key from the cache.
func (c *Cache[K, V]) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if n, ok := c.items[key]; ok {
		c.removeNode(n)
		delete(c.items, key)
	}
}

// Clear removes all entries from the cache.
func (c *Cache[K, V]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[K]*node[K, V], c.capacity)
	c.head.next = c.tail
	c.tail.prev = c.head
}

// Len returns the number of items currently in the cache.
func (c *Cache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.items)
}

// Evict removes all expired entries from the cache.
func (c *Cache[K, V]) Evict() {
	c.mu.Lock()
	defer c.mu.Unlock()

	cur := c.head.next
	for cur != c.tail {
		next := cur.next
		if cur.entry.IsExpired() {
			c.removeNode(cur)
			delete(c.items, cur.key)
		}
		cur = next
	}
}

// pushFront inserts a node immediately after the sentinel head.
func (c *Cache[K, V]) pushFront(n *node[K, V]) {
	n.prev = c.head
	n.next = c.head.next
	c.head.next.prev = n
	c.head.next = n
}

// moveToFront moves an existing node to the front (most-recently-used position).
func (c *Cache[K, V]) moveToFront(n *node[K, V]) {
	c.removeNode(n)
	c.pushFront(n)
}

// removeNode unlinks a node from the doubly-linked list.
func (c *Cache[K, V]) removeNode(n *node[K, V]) {
	n.prev.next = n.next
	n.next.prev = n.prev
}

// removeLRU evicts the least-recently-used node (the one before the sentinel tail).
func (c *Cache[K, V]) removeLRU() {
	lru := c.tail.prev
	if lru == c.head {
		return
	}
	c.removeNode(lru)
	delete(c.items, lru.key)
}
