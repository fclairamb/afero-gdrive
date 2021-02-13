// Package cache allows to
package cache

import (
	"strings"
	"sync"
)

type item struct {
	value interface{}
}

// Cache management
type Cache struct {
	mutex sync.RWMutex
	items map[string]*item
}

// NewCache creates a new cache instance
func NewCache() *Cache {
	return &Cache{
		items: make(map[string]*item),
	}
}

// Set sets a value in the cache
func (c *Cache) Set(key string, value interface{}) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.items[key] = &item{value: value}
}

// Get gets a value from the cache
func (c *Cache) Get(key string) (interface{}, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if item, found := c.items[key]; found {
		return item.value, found
	}

	return nil, false
}

// GetValue gets a value without specifying if it existed in the cache
func (c *Cache) GetValue(key string) interface{} {
	v, _ := c.Get(key)
	return v
}

// Delete deletes a cache value
func (c *Cache) Delete(key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.items, key)
}

// CleanupByPrefix deletes all cache values with a given key prefix
func (c *Cache) CleanupByPrefix(prefix string) int {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	count := 0

	for k := range c.items {
		if strings.HasPrefix(k, prefix) {
			delete(c.items, k)
			count++
		}
	}

	return count
}

// CleanupEverything resets the cache
func (c *Cache) CleanupEverything() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.items = make(map[string]*item)
}
