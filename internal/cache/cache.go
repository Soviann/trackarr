package cache

import (
	"container/list"
	"sync"
	"time"
)

const (
	TagTitles   = "titles"
	TagStats    = "stats"
	TagCalendar = "calendar"

	DefaultMaxEntries = 256
	DefaultTTL        = 5 * time.Minute
)

type entry struct {
	key       string
	value     any
	expiresAt time.Time
	tags      []string
	element   *list.Element
}

// Cache is a thread-safe, tag-indexed in-memory read cache with bounded
// capacity (evicting oldest on overflow) and TTL expiry.
type Cache struct {
	mu         sync.RWMutex
	maxEntries int
	ttl        time.Duration
	entries    map[string]*entry
	tags       map[string]map[string]struct{}
	order      *list.List
}

// New creates an initialized Cache. Non-positive maxEntries or ttl fallback
// to DefaultMaxEntries and DefaultTTL.
func New(maxEntries int, ttl time.Duration) *Cache {
	if maxEntries <= 0 {
		maxEntries = DefaultMaxEntries
	}
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	return &Cache{
		maxEntries: maxEntries,
		ttl:        ttl,
		entries:    make(map[string]*entry),
		tags:       make(map[string]map[string]struct{}),
		order:      list.New(),
	}
}

// Get returns the cached value for key if present and unexpired.
func (c *Cache) Get(key string) (any, bool) {
	c.mu.RLock()
	e, ok := c.entries[key]
	if !ok {
		c.mu.RUnlock()
		return nil, false
	}

	if time.Now().After(e.expiresAt) {
		c.mu.RUnlock()
		c.mu.Lock()
		if e2, ok2 := c.entries[key]; ok2 && time.Now().After(e2.expiresAt) {
			c.removeEntry(e2)
		}
		c.mu.Unlock()
		return nil, false
	}

	val := e.value
	c.mu.RUnlock()
	return val, true
}

// Set stores a key-value pair associated with the provided domain tags, using the default TTL.
func (c *Cache) Set(key string, value any, tags ...string) {
	c.SetWithTTL(key, value, c.ttl, tags...)
}

// SetWithTTL stores a key-value pair with a custom TTL and domain tags.
func (c *Cache) SetWithTTL(key string, value any, ttl time.Duration, tags ...string) {
	if ttl <= 0 {
		ttl = c.ttl
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if existing, ok := c.entries[key]; ok {
		c.removeEntry(existing)
	}

	for c.maxEntries > 0 && len(c.entries) >= c.maxEntries {
		oldestElem := c.order.Back()
		if oldestElem == nil {
			break
		}
		oldest := oldestElem.Value.(*entry)
		c.removeEntry(oldest)
	}

	e := &entry{
		key:       key,
		value:     value,
		expiresAt: time.Now().Add(ttl),
		tags:      tags,
	}
	elem := c.order.PushFront(e)
	e.element = elem
	c.entries[key] = e

	for _, tag := range tags {
		keys, ok := c.tags[tag]
		if !ok {
			keys = make(map[string]struct{})
			c.tags[tag] = keys
		}
		keys[key] = struct{}{}
	}
}

// Invalidate removes all cached entries associated with any of the specified domain tags.
func (c *Cache) Invalidate(tags ...string) {
	if len(tags) == 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	keysToEvict := make(map[string]struct{})
	for _, tag := range tags {
		if keys, ok := c.tags[tag]; ok {
			for k := range keys {
				keysToEvict[k] = struct{}{}
			}
		}
	}

	for k := range keysToEvict {
		if e, ok := c.entries[k]; ok {
			c.removeEntry(e)
		}
	}
}

// Clear removes all entries and tag associations from the cache.
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*entry)
	c.tags = make(map[string]map[string]struct{})
	c.order.Init()
}

// Len returns the current number of cached entries.
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// Keys returns all active cached keys.
func (c *Cache) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	res := make([]string, 0, len(c.entries))
	for k := range c.entries {
		res = append(res, k)
	}
	return res
}

func (c *Cache) removeEntry(e *entry) {
	delete(c.entries, e.key)
	if e.element != nil {
		c.order.Remove(e.element)
		e.element = nil
	}
	for _, tag := range e.tags {
		if keys, ok := c.tags[tag]; ok {
			delete(keys, e.key)
			if len(keys) == 0 {
				delete(c.tags, tag)
			}
		}
	}
}
