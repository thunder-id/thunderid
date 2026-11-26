// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"context"

	"github.com/thunder-id/thunderid/internal/system/log"
)

// GroupedCacheKey identifies one entry inside a group.
//
// A group is a set of entries that are invalidated together. Sharing uses one group per resource
// and one entry per organization unit, because a single policy write changes the answer for every
// unit a resource reaches, and the set of those units is not known at write time.
type GroupedCacheKey struct {
	// Group is the set the entry belongs to, and the unit of invalidation.
	Group string
	// Key identifies the entry within its group.
	Key string
}

// ToString returns the string representation of the key, for logging.
func (key GroupedCacheKey) ToString() string {
	return key.Group + "/" + key.Key
}

// GroupedCacheInterface is a cache whose entries can be invalidated a whole group at a time.
//
// It is deliberately separate from CacheInterface rather than an option on it: a plain cache is
// invalidated by exact key, and giving every cache a group would let one mix grouped and plain
// entries under the same name. The two have different Redis layouts, so that mixture would be a
// WRONGTYPE error at runtime rather than a compile error.
//
// There is no Clear. Dropping everything is what group invalidation exists to replace, and on Redis
// it could only ever be a no-op or a dangerous scan.
type GroupedCacheInterface[T any] interface {
	GetName() string
	Set(ctx context.Context, key GroupedCacheKey, value T) error
	Get(ctx context.Context, key GroupedCacheKey) (T, bool)
	Delete(ctx context.Context, key GroupedCacheKey) error
	// DeleteGroup removes every entry in the group. Deleting a group that holds nothing succeeds.
	DeleteGroup(ctx context.Context, group string) error
	IsEnabled() bool
	GetStats() CacheStat
	CleanupExpired()
}

// GroupedCache is the enabled-aware wrapper the manager hands out, mirroring Cache for the plain
// caches: a failing cache degrades to a miss and is logged, never surfaced to the caller.
type GroupedCache[T any] struct {
	enabled   bool
	cacheName string
	cacheImpl GroupedCacheInterface[T]
}

// GetName returns the name of the cache.
func (c *GroupedCache[T]) GetName() string {
	return c.cacheName
}

// logger returns the cache's logger.
func (c *GroupedCache[T]) logger() *log.Logger {
	return log.GetLogger().With(log.String(log.LoggerKeyComponentName, "GroupedCache"),
		log.String("cacheName", c.cacheName))
}

// active reports whether there is a working implementation behind this cache.
func (c *GroupedCache[T]) active() bool {
	return c.enabled && c.cacheImpl != nil && c.cacheImpl.IsEnabled()
}

// Set stores a value in the cache.
func (c *GroupedCache[T]) Set(ctx context.Context, key GroupedCacheKey, value T) error {
	if c.active() {
		if err := c.cacheImpl.Set(ctx, key, value); err != nil {
			c.logger().Warn(ctx, "Failed to set value in the cache",
				log.String("key", key.ToString()), log.Error(err))
		}
	}
	return nil
}

// Get retrieves a value from the cache.
func (c *GroupedCache[T]) Get(ctx context.Context, key GroupedCacheKey) (T, bool) {
	if c.active() {
		if value, found := c.cacheImpl.Get(ctx, key); found {
			return value, true
		}
	}

	var zero T
	return zero, false
}

// Delete removes one entry from the cache.
func (c *GroupedCache[T]) Delete(ctx context.Context, key GroupedCacheKey) error {
	if c.active() {
		if err := c.cacheImpl.Delete(ctx, key); err != nil {
			c.logger().Warn(ctx, "Failed to delete value from the cache",
				log.String("key", key.ToString()), log.Error(err))
		}
	}
	return nil
}

// DeleteGroup removes every entry in the group.
//
// The error is returned rather than only logged, because this is how a caller invalidates after a
// write: a failure here leaves stale answers being served, which the caller may want to act on.
func (c *GroupedCache[T]) DeleteGroup(ctx context.Context, group string) error {
	if !c.active() {
		return nil
	}

	if err := c.cacheImpl.DeleteGroup(ctx, group); err != nil {
		c.logger().Warn(ctx, "Failed to delete the cache group",
			log.String("group", group), log.Error(err))
		return err
	}
	return nil
}

// IsEnabled returns whether the cache is enabled.
func (c *GroupedCache[T]) IsEnabled() bool {
	return c.enabled
}

// GetStats returns cache statistics.
func (c *GroupedCache[T]) GetStats() CacheStat {
	if c.enabled && c.cacheImpl != nil {
		return c.cacheImpl.GetStats()
	}
	return CacheStat{Enabled: false}
}

// CleanupExpired cleans up expired entries in the cache.
func (c *GroupedCache[T]) CleanupExpired() {
	if c.active() {
		c.cacheImpl.CleanupExpired()
	}
}
