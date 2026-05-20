/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package cache

import (
	"container/heap"
	"container/list"
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// lfuHeapItem represents an item in the LFU heap.
type lfuHeapItem struct {
	key         CacheKey
	accessCount int64
	lastAccess  time.Time
	index       int // Index in the heap
}

// lfuHeap implements heap.Interface for LFU eviction.
type lfuHeap []*lfuHeapItem

func (h lfuHeap) Len() int { return len(h) }

func (h lfuHeap) Less(i, j int) bool {
	// Primary: fewer accesses come first
	if h[i].accessCount != h[j].accessCount {
		return h[i].accessCount < h[j].accessCount
	}
	// Tie-breaker: earlier access time comes first
	return h[i].lastAccess.Before(h[j].lastAccess)
}

func (h lfuHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *lfuHeap) Push(x any) {
	n := len(*h)
	item := x.(*lfuHeapItem)
	item.index = n
	*h = append(*h, item)
}

func (h *lfuHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // for safety
	*h = old[0 : n-1]
	return item
}

// inMemoryCacheEntry represents an entry in the in-memory cache with additional metadata.
type inMemoryCacheEntry[T any] struct {
	*CacheEntry[T]
	listElement *list.Element
	heapItem    *lfuHeapItem
	lastAccess  time.Time
	accessCount int64
}

// inMemoryCache implements the CacheInterface for an in-memory cache.
type inMemoryCache[T any] struct {
	enabled        bool
	name           string
	cache          map[CacheKey]*inMemoryCacheEntry[T]
	accessOrder    *list.List
	lfuHeap        *lfuHeap
	mu             sync.RWMutex
	size           int
	ttl            time.Duration
	evictionPolicy evictionPolicy
	hitCount       atomic.Int64
	missCount      atomic.Int64
	evictCount     atomic.Int64
}

// getEvictionPolicy retrieves the eviction policy from the cache configuration.
func getEvictionPolicy(cacheConfig config.CacheConfig, cacheProperty config.CacheProperty) evictionPolicy {
	evictionPolicy := cacheProperty.EvictionPolicy
	if evictionPolicy == "" {
		evictionPolicy = cacheConfig.EvictionPolicy
	}
	if evictionPolicy == "" {
		return evictionPolicyLRU
	}

	switch evictionPolicy {
	case string(evictionPolicyLRU):
		return evictionPolicyLRU
	case string(evictionPolicyLFU):
		return evictionPolicyLFU
	default:
		log.GetLogger().Warn("Unknown eviction policy, defaulting to LRU")
		return evictionPolicyLRU
	}
}

// getCacheTTL retrieves the cache TTL as a Duration from the cache configuration.
func getCacheTTL(cacheConfig config.CacheConfig, cacheProperty config.CacheProperty) time.Duration {
	ttl := cacheProperty.TTL
	if ttl <= 0 {
		ttl = cacheConfig.TTL
	}
	return time.Duration(ttl) * time.Second
}

// getCacheSize retrieves the cache size from the cache configuration.
func getCacheSize(cacheConfig config.CacheConfig, cacheProperty config.CacheProperty) int {
	size := cacheProperty.Size
	if size <= 0 {
		size = cacheConfig.Size
	}
	return size
}

// newInMemoryCache creates a new instance of InMemoryCache.
func newInMemoryCache[T any](name string, enabled bool,
	cacheConfig config.CacheConfig, cacheProperty config.CacheProperty) CacheInterface[T] {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "InMemoryCache"),
		log.String("name", name))

	if !enabled {
		logger.Warn("In-memory cache is disabled, returning empty cache")
		return &inMemoryCache[T]{
			name:    name,
			enabled: false,
		}
	}

	ttl := getCacheTTL(cacheConfig, cacheProperty)
	size := getCacheSize(cacheConfig, cacheProperty)
	evictionPolicy := getEvictionPolicy(cacheConfig, cacheProperty)

	logger.Debug("Initializing In-memory cache", log.String("evictionPolicy", string(evictionPolicy)),
		log.Int("size", size), log.Any("ttl", ttl))

	lfuHeapInstance := &lfuHeap{}
	heap.Init(lfuHeapInstance)

	return &inMemoryCache[T]{
		enabled:        true,
		name:           name,
		cache:          make(map[CacheKey]*inMemoryCacheEntry[T]),
		accessOrder:    list.New(),
		lfuHeap:        lfuHeapInstance,
		size:           size,
		ttl:            ttl,
		evictionPolicy: evictionPolicy,
	}
}

// Set adds or updates an entry in the cache.
func (c *inMemoryCache[T]) Set(_ context.Context, key CacheKey, value T) error {
	if !c.enabled {
		return nil
	}

	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "InMemoryCache"),
		log.String("name", c.GetName()))

	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	expiryTime := now.Add(c.ttl)

	// Update existing entry if an entry exists
	if existingEntry, exists := c.cache[key]; exists {
		existingEntry.Value = value
		existingEntry.ExpiryTime = expiryTime
		existingEntry.lastAccess = now
		existingEntry.accessCount++
		c.accessOrder.MoveToFront(existingEntry.listElement)

		// Update the heap item for LFU eviction
		if c.evictionPolicy == evictionPolicyLFU && existingEntry.heapItem != nil {
			existingEntry.heapItem.accessCount = existingEntry.accessCount
			existingEntry.heapItem.lastAccess = existingEntry.lastAccess
			heap.Fix(c.lfuHeap, existingEntry.heapItem.index)
		}
		return nil
	}

	// Create new entry
	cacheEntry := &CacheEntry[T]{
		Value:      value,
		ExpiryTime: expiryTime,
	}

	listElement := c.accessOrder.PushFront(key)

	// Create heap item for LFU eviction
	var heapItem *lfuHeapItem
	if c.evictionPolicy == evictionPolicyLFU {
		heapItem = &lfuHeapItem{
			key:         key,
			accessCount: 1,
			lastAccess:  now,
		}
		heap.Push(c.lfuHeap, heapItem)
	}

	inMemoryCacheEntry := &inMemoryCacheEntry[T]{
		CacheEntry:  cacheEntry,
		listElement: listElement,
		heapItem:    heapItem,
		lastAccess:  now,
		accessCount: 1,
	}
	c.cache[key] = inMemoryCacheEntry

	logger.Debug("Cache entry set", log.String("key", key.ToString()))

	// Check if there's a requirement to evict an entry
	if len(c.cache) > c.size {
		logger.Debug("Cache size exceeded, evicting an entry")
		c.evict()
	}

	return nil
}

// Get retrieves a value from the cache.
func (c *inMemoryCache[T]) Get(_ context.Context, key CacheKey) (T, bool) {
	if !c.enabled {
		var zero T
		return zero, false
	}

	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "InMemoryCache"),
		log.String("name", c.GetName()))

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.cache[key]
	if !exists {
		c.missCount.Add(1)
		var zero T
		return zero, false
	}

	// Check if the entry has expired
	if time.Now().After(entry.ExpiryTime) {
		c.deleteEntry(key, entry)
		c.missCount.Add(1)
		var zero T
		return zero, false
	}

	// Update access order for LRU/LFU
	entry.lastAccess = time.Now()
	entry.accessCount++
	c.accessOrder.MoveToFront(entry.listElement)
	c.hitCount.Add(1)

	// Update the heap item for LFU eviction
	if c.evictionPolicy == evictionPolicyLFU && entry.heapItem != nil {
		entry.heapItem.accessCount = entry.accessCount
		entry.heapItem.lastAccess = entry.lastAccess
		heap.Fix(c.lfuHeap, entry.heapItem.index)
	}

	logger.Debug("Cache hit", log.String("key", key.ToString()))
	return entry.Value, true
}

// Delete removes an entry from the cache.
func (c *inMemoryCache[T]) Delete(_ context.Context, key CacheKey) error {
	if !c.enabled {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, exists := c.cache[key]; exists {
		c.deleteEntry(key, entry)
	}

	return nil
}

// Clear removes all entries from the cache.
func (c *inMemoryCache[T]) Clear(_ context.Context) error {
	if !c.enabled {
		return nil
	}

	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "InMemoryCache"),
		log.String("name", c.GetName()))

	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[CacheKey]*inMemoryCacheEntry[T])
	c.accessOrder.Init()
	c.lfuHeap = &lfuHeap{}
	heap.Init(c.lfuHeap)
	c.hitCount.Store(0)
	c.missCount.Store(0)
	c.evictCount.Store(0)

	logger.Debug("Cleared all entries in the cache")
	return nil
}

// IsEnabled returns whether the cache is enabled.
func (c *inMemoryCache[T]) IsEnabled() bool {
	return c.enabled
}

// GetName returns the name of the cache.
func (c *inMemoryCache[T]) GetName() string {
	return c.name
}

// GetStats returns cache statistics.
func (c *inMemoryCache[T]) GetStats() CacheStat {
	if !c.enabled {
		return CacheStat{Enabled: false}
	}

	c.mu.RLock()
	size := len(c.cache)
	c.mu.RUnlock()

	hits := c.hitCount.Load()
	misses := c.missCount.Load()
	totalOps := hits + misses
	var hitRate float64
	if totalOps > 0 {
		hitRate = float64(hits) / float64(totalOps)
	}

	return CacheStat{
		Enabled:    true,
		Size:       size,
		MaxSize:    c.size,
		HitCount:   hits,
		MissCount:  misses,
		HitRate:    hitRate,
		EvictCount: c.evictCount.Load(),
	}
}

// evict removes an entry based on the eviction policy.
func (c *inMemoryCache[T]) evict() {
	if c.evictionPolicy == evictionPolicyLFU {
		c.evictLeastFrequent()
	} else {
		c.evictOldest()
	}
}

// evictOldest removes the oldest entry from the cache (LRU eviction).
func (c *inMemoryCache[T]) evictOldest() {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "InMemoryCache"),
		log.String("name", c.GetName()))

	if c.accessOrder.Len() == 0 {
		return
	}

	// Get the least recently used item
	oldest := c.accessOrder.Back()
	if oldest != nil {
		key := oldest.Value.(CacheKey)
		if entry, exists := c.cache[key]; exists {
			c.deleteEntry(key, entry)
			c.evictCount.Add(1)
			logger.Debug("Cache entry evicted", log.String("key", key.ToString()))
		}
	}
}

// evictLeastFrequent removes the least frequently used entry from the cache (LFU eviction).
func (c *inMemoryCache[T]) evictLeastFrequent() {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "InMemoryCache"),
		log.String("name", c.GetName()))

	if c.lfuHeap.Len() == 0 {
		return
	}

	// Get the least frequently used item from the heap
	leastFrequentItem := heap.Pop(c.lfuHeap).(*lfuHeapItem)

	if entry, exists := c.cache[leastFrequentItem.key]; exists {
		c.deleteEntry(leastFrequentItem.key, entry)
		c.evictCount.Add(1)
		logger.Debug("Cache entry evicted (LFU)", log.String("key", leastFrequentItem.key.ToString()),
			log.Any("accessCount", leastFrequentItem.accessCount))
	}
}

// deleteEntry removes an entry from both the map and the access order list.
func (c *inMemoryCache[T]) deleteEntry(key CacheKey, entry *inMemoryCacheEntry[T]) {
	delete(c.cache, key)
	c.accessOrder.Remove(entry.listElement)

	// Remove from heap if using LFU eviction
	if c.evictionPolicy == evictionPolicyLFU && entry.heapItem != nil && entry.heapItem.index >= 0 {
		heap.Remove(c.lfuHeap, entry.heapItem.index)
	}
}

// CleanupExpired removes all expired entries from the cache.
func (c *inMemoryCache[T]) CleanupExpired() {
	if !c.enabled {
		return
	}

	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "InMemoryCache"),
		log.String("name", c.GetName()))
	logger.Debug("Cleaning up expired entries from the cache")

	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	cleaned := 0
	for key, entry := range c.cache {
		if now.After(entry.ExpiryTime) {
			c.deleteEntry(key, entry)
			cleaned++
		}
	}

	if logger.IsDebugEnabled() {
		if cleaned > 0 {
			logger.Debug("Expired cache entries cleaned", log.Int("count", cleaned))
		} else {
			logger.Debug("No expired entries found in the cache")
		}
	}
}
