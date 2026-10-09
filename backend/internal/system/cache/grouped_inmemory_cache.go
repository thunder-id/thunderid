// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"context"
	"strconv"
	"sync"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// groupedInMemoryCache adds group invalidation to the ordinary in-memory cache.
//
// It wraps inMemoryCache rather than reimplementing it, so eviction, TTL, sizing and statistics
// stay in one place and behave identically to every other in-memory cache. All this layer adds is
// an index from a group to the keys stored under it.
type groupedInMemoryCache[T any] struct {
	inner *inMemoryCache[T]
	mu    sync.Mutex
	// groups maps a group to the flat keys stored under it. It records membership only; the values
	// themselves live in the wrapped cache.
	groups map[string]map[CacheKey]struct{}
}

// newGroupedInMemoryCache creates an in-memory cache that supports group invalidation.
func newGroupedInMemoryCache[T any](name string, enabled bool,
	cacheConfig engineconfig.CacheConfig, cacheProperty engineconfig.CacheProperty,
) GroupedCacheInterface[T] {
	// newInMemoryCache always returns *inMemoryCache, enabled or not. The concrete type is kept so
	// this layer can probe the cache without disturbing hit counts or eviction order.
	inner, _ := newInMemoryCache[T](name, enabled, cacheConfig, cacheProperty).(*inMemoryCache[T])
	return &groupedInMemoryCache[T]{
		inner:  inner,
		groups: make(map[string]map[CacheKey]struct{}),
	}
}

// flatten turns a grouped key into the key the wrapped cache stores it under.
//
// The wrapped cache has one flat keyspace, so the group and the entry key have to be squashed into
// one string, and that string has to name exactly one pair. Joining them with a separator does
// not: with a plain colon, group "a:b" entry "c" and group "a" entry "b:c" both read "a:b:c", so
// one would overwrite the other and invalidating either group would take both.
//
// The length of the group is written first, which makes the encoding injective whatever the two
// contain: the reader knows where the group ends without searching for a delimiter. Redis needs
// none of this, because there the group is the key and the entry is a hash field, which are
// already separate namespaces; this keeps the two backends answering alike.
func flatten(key GroupedCacheKey) CacheKey {
	return CacheKey{Key: strconv.Itoa(len(key.Group)) + ":" + key.Group + key.Key}
}

// Set stores a value and records it as a member of its group.
//
// The lock covers the stored value and the index together. Publishing the value first would leave
// a window in which the entry exists but nothing names it, and a DeleteGroup landing in that
// window would take the group's other entries while leaving this one behind, to be served until
// its TTL ran out.
func (c *groupedInMemoryCache[T]) Set(ctx context.Context, key GroupedCacheKey, value T) error {
	if !c.inner.IsEnabled() {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.inner.Set(ctx, flatten(key), value); err != nil {
		return err
	}

	members, ok := c.groups[key.Group]
	if !ok {
		members = make(map[CacheKey]struct{})
		c.groups[key.Group] = members
	}
	members[flatten(key)] = struct{}{}
	return nil
}

// Get retrieves a value.
func (c *groupedInMemoryCache[T]) Get(ctx context.Context, key GroupedCacheKey) (T, bool) {
	return c.inner.Get(ctx, flatten(key))
}

// Delete removes one entry.
func (c *groupedInMemoryCache[T]) Delete(ctx context.Context, key GroupedCacheKey) error {
	if !c.inner.IsEnabled() {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.inner.Delete(ctx, flatten(key)); err != nil {
		return err
	}

	c.forget(key.Group, flatten(key))
	return nil
}

// DeleteGroup removes every entry recorded under the group.
//
// The index can name keys the wrapped cache has already evicted or expired; deleting those is a
// no-op, which is why the index does not have to be kept in step with eviction.
func (c *groupedInMemoryCache[T]) DeleteGroup(ctx context.Context, group string) error {
	if !c.inner.IsEnabled() {
		return nil
	}

	// Held across the deletions as well as the index read: releasing in between would let a Set
	// that has already chosen its value slip its entry in behind the invalidation.
	c.mu.Lock()
	defer c.mu.Unlock()

	members := c.groups[group]
	delete(c.groups, group)

	for key := range members {
		if err := c.inner.Delete(ctx, key); err != nil {
			return err
		}
	}
	return nil
}

// forget drops one key from its group's index, and the group itself once it is empty. The caller
// holds the lock.
func (c *groupedInMemoryCache[T]) forget(group string, key CacheKey) {
	members, ok := c.groups[group]
	if !ok {
		return
	}
	delete(members, key)
	if len(members) == 0 {
		delete(c.groups, group)
	}
}

// IsEnabled returns whether the cache is enabled.
func (c *groupedInMemoryCache[T]) IsEnabled() bool {
	return c.inner.IsEnabled()
}

// GetName returns the name of the cache.
func (c *groupedInMemoryCache[T]) GetName() string {
	return c.inner.GetName()
}

// GetStats returns cache statistics.
func (c *groupedInMemoryCache[T]) GetStats() CacheStat {
	return c.inner.GetStats()
}

// CleanupExpired removes expired entries, and the index entries naming them.
func (c *groupedInMemoryCache[T]) CleanupExpired() {
	c.inner.CleanupExpired()

	// The wrapped cache has dropped the expired entries, so any key the index still names that is
	// no longer there can go too. Without this the index would grow for the lifetime of a group
	// that is never invalidated.
	c.mu.Lock()
	defer c.mu.Unlock()
	for group, members := range c.groups {
		for key := range members {
			if !c.inner.contains(key) {
				delete(members, key)
			}
		}
		if len(members) == 0 {
			delete(c.groups, group)
		}
	}
}
