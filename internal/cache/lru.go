// Package cache implements a TTL-aware LRU cache, safe for concurrent use.
package cache

import (
	"container/list"
	"sync"
	"time"
)

type entry struct {
	key       string
	value     []byte
	expiresAt time.Time
}

// Cache is a fixed-capacity, least-recently-used cache where each entry also
// expires after its own TTL. Reads and writes are safe for concurrent use.
type Cache struct {
	mu       sync.Mutex
	capacity int
	ll       *list.List // front = most recently used, back = least
	items    map[string]*list.Element
}

// New creates a Cache holding at most capacity entries.
func New(capacity int) *Cache {
	if capacity <= 0 {
		capacity = 1
	}
	return &Cache{
		capacity: capacity,
		ll:       list.New(),
		items:    make(map[string]*list.Element),
	}
}

// Get returns the value stored for key and its remaining TTL. ok is false if
// the key is absent or has expired; an expired entry is evicted on access.
func (c *Cache) Get(key string) (value []byte, remaining time.Duration, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	el, found := c.items[key]
	if !found {
		return nil, 0, false
	}
	e := el.Value.(*entry)
	remaining = time.Until(e.expiresAt)
	if remaining <= 0 {
		c.removeElement(el)
		return nil, 0, false
	}
	c.ll.MoveToFront(el)
	return e.value, remaining, true
}

// Put inserts or updates key with value, expiring after ttl. If the cache is
// over capacity afterward, the least-recently-used entry is evicted.
func (c *Cache) Put(key string, value []byte, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	expiresAt := time.Now().Add(ttl)
	if el, found := c.items[key]; found {
		e := el.Value.(*entry)
		e.value = value
		e.expiresAt = expiresAt
		c.ll.MoveToFront(el)
		return
	}

	el := c.ll.PushFront(&entry{key: key, value: value, expiresAt: expiresAt})
	c.items[key] = el

	if c.ll.Len() > c.capacity {
		c.removeElement(c.ll.Back())
	}
}

// removeElement unlinks el from both the list and the map. Callers must hold c.mu.
func (c *Cache) removeElement(el *list.Element) {
	c.ll.Remove(el)
	delete(c.items, el.Value.(*entry).key)
}

// Len reports the number of entries currently stored.
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ll.Len()
}
