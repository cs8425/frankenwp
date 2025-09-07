package cache

import (
	"container/list"
	"weak"

	"github.com/puzpuzpuz/xsync"
)

type LRUCache[K comparable, V any] struct {
	capacity int
	mu       xsync.RBMutex
	ll       *list.List
	cache    map[K]*list.Element

	// TODO: use concurrent map?
	// cache *xsync.MapOf[K, *list.Element]
}

type entry[K comparable, V any] struct {
	key   K
	value weak.Pointer[V] // Store weak pointer to the actual value
}

func NewLRUCache[K comparable, V any](capacity int) *LRUCache[K, V] {
	return &LRUCache[K, V]{
		capacity: capacity,
		cache:    make(map[K]*list.Element),
		ll:       list.New(),
	}
}

func (c *LRUCache[K, V]) Size() int {
	tk := c.mu.RLock()
	defer c.mu.RUnlock(tk)
	return len(c.cache)
}

func (c *LRUCache[K, V]) Get(key K) (*V, bool) {
	tk := c.mu.RLock()
	defer c.mu.RUnlock(tk)
	return c.get(key, true)
}

func (c *LRUCache[K, V]) Peek(key K) (*V, bool) {
	tk := c.mu.RLock()
	defer c.mu.RUnlock(tk)
	return c.get(key, false)
}

func (c *LRUCache[K, V]) get(key K, touch bool) (*V, bool) {
	if elem, ok := c.cache[key]; ok {
		if touch {
			c.ll.MoveToFront(elem)
		}
		valEntry := elem.Value.(*entry[K, V])

		// Attempt to get the strong pointer from the weak pointer
		if strongVal := valEntry.value.Value(); strongVal != nil {
			return strongVal, true
		} else {
			// Object has been garbage collected, remove from cache
			c.removeElement(elem)
			return nil, false
		}
	}
	return nil, false
}

func (c *LRUCache[K, V]) Put(key K, value V) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.put(key, value)
}

func (c *LRUCache[K, V]) put(key K, value V) bool {
	if elem, ok := c.cache[key]; ok {
		c.ll.MoveToFront(elem)
		valEntry := elem.Value.(*entry[K, V])
		valEntry.value = weak.Make(&value) // Update weak pointer
		return true
	}

	if c.ll.Len() >= c.capacity {
		c.removeOldest()
	}
	newEntry := &entry[K, V]{key: key, value: weak.Make(&value)}
	elem := c.ll.PushFront(newEntry)
	c.cache[key] = elem
	return false
}

func (c *LRUCache[K, V]) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.cache[key]; ok {
		c.removeElement(elem)
	}
}

func (c *LRUCache[K, V]) removeElement(e *list.Element) {
	c.ll.Remove(e)
	delete(c.cache, e.Value.(*entry[K, V]).key)
}

func (c *LRUCache[K, V]) removeOldest() {
	if c.ll.Len() == 0 {
		return
	}
	elem := c.ll.Back()
	c.removeElement(elem)
}

// LoadOrCompute returns the existing value for the key if present.
// Otherwise, it computes the value using the provided function and returns the computed value.
// The loaded result is true if the value was loaded, false if stored.
func (c *LRUCache[K, V]) LoadOrCompute(key K, valueFn func() V) (actual V, loaded bool) {
	tk := c.mu.RLock()
	val, ok := c.get(key, true)
	if ok {
		c.mu.RUnlock(tk)
		return *val, true
	}
	c.mu.RUnlock(tk)

	// upgrade lock
	c.mu.Lock()
	defer c.mu.Unlock()

	// check again if someone already set value between we release read lock and  get write lock
	val, ok = c.get(key, true)
	if ok {
		c.mu.Unlock()
		return *val, true
	}

	// still no value, call compute function
	newVal := valueFn()
	c.put(key, newVal)
	return newVal, false
}

// Range calls f sequentially for each key and value present in the
// map. If f returns false, range stops the iteration.
//
// Range does not necessarily correspond to any consistent snapshot
// of the Map's contents: no key will be visited more than once, but
// if the value for any key is stored or deleted concurrently, Range
// may reflect any mapping for that key from any point during the
// Range call.
//
// Should NOT modify the map while iterating it.
func (c *LRUCache[K, V]) Range(f func(key K, value V) bool) {
	tk := c.mu.RLock()
	defer c.mu.RUnlock(tk)
	for k := range c.cache {
		v, _ := c.get(k, false)
		if !f(k, *v) {
			return
		}
	}
}
