// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/thunder-id/thunderid/internal/system/log"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// groupedRedisCache stores each group as one Redis hash, with one field per entry.
//
// The layout is what makes group invalidation a single command: dropping the hash drops every
// entry in the group at once, where a flat key layout would need a SCAN across the keyspace.
type groupedRedisCache[T any] struct {
	enabled   bool
	name      string
	client    *redis.Client
	ttl       time.Duration
	keyPrefix string
	hitCount  int64
	missCount int64
}

// newGroupedRedisCache creates a Redis-backed cache that supports group invalidation.
func newGroupedRedisCache[T any](name string, enabled bool, client *redis.Client, keyPrefix string,
	cacheConfig engineconfig.CacheConfig, cacheProperty engineconfig.CacheProperty,
) GroupedCacheInterface[T] {
	// Cache infrastructure logging has no request scope, so context.Background() is used.
	ctx := context.Background()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "GroupedRedisCache"),
		log.String("name", name))

	if !enabled {
		logger.Warn(ctx, "Redis cache is disabled, returning empty cache")
		return &groupedRedisCache[T]{name: name, enabled: false}
	}

	ttl := getCacheTTL(cacheConfig, cacheProperty)
	logger.Debug(ctx, "Initializing grouped Redis cache", log.Any("ttl", ttl),
		log.String("keyPrefix", keyPrefix))

	return &groupedRedisCache[T]{
		enabled:   true,
		name:      name,
		client:    client,
		ttl:       ttl,
		keyPrefix: keyPrefix,
	}
}

// buildGroupKey constructs the Redis key holding one group's hash.
//
// The ":g:" segment keeps group keys in a namespace of their own. A plain cache writes strings at
// "{prefix}:{name}:{key}", so without the segment a group could land on a key some other cache
// already holds as a string, and every command against it would fail with WRONGTYPE.
func (c *groupedRedisCache[T]) buildGroupKey(group string) string {
	return c.keyPrefix + ":" + c.name + ":g:" + group
}

// logger returns the cache's logger.
func (c *groupedRedisCache[T]) logger() *log.Logger {
	return log.GetLogger().With(log.String(log.LoggerKeyComponentName, "GroupedRedisCache"),
		log.String("name", c.name))
}

// Set stores a value as a field of its group's hash.
//
// The field TTL and the key TTL are written with the value in one transaction. HSET clears any TTL
// the field already had, so HEXPIRE has to accompany every write rather than be set once. The key
// TTL is what keeps the hash eligible for volatile-* eviction, and is never shorter than the
// fields it holds.
func (c *groupedRedisCache[T]) Set(ctx context.Context, key GroupedCacheKey, value T) error {
	if !c.enabled {
		return nil
	}

	data, err := json.Marshal(value)
	if err != nil {
		c.logger().Warn(ctx, "Failed to marshal value for Redis cache", log.Error(err))
		return err
	}

	groupKey := c.buildGroupKey(key.Group)
	if _, err := c.client.TxPipelined(ctx, func(p redis.Pipeliner) error {
		p.HSet(ctx, groupKey, key.Key, data)
		// A TTL of zero means "no expiry" everywhere else in this package, but to Redis it means
		// delete now: HEXPIRE would drop the field just written and EXPIRE the whole group with
		// it. Sending neither leaves the entry with no expiry, which is what a cache configured
		// without a TTL asks for, and what the plain Redis cache already does.
		if c.ttl > 0 {
			p.HExpire(ctx, groupKey, c.ttl, key.Key)
			p.Expire(ctx, groupKey, c.ttl)
		}
		return nil
	}); err != nil {
		c.logger().Warn(ctx, "Failed to set value in Redis cache",
			log.String("key", key.ToString()), log.Error(err))
		return err
	}

	return nil
}

// Get retrieves a value from its group's hash.
func (c *groupedRedisCache[T]) Get(ctx context.Context, key GroupedCacheKey) (T, bool) {
	var zero T
	if !c.enabled {
		return zero, false
	}

	data, err := c.client.HGet(ctx, c.buildGroupKey(key.Group), key.Key).Bytes()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			c.logger().Warn(ctx, "Failed to get value from Redis cache",
				log.String("key", key.ToString()), log.Error(err))
		}
		atomic.AddInt64(&c.missCount, 1)
		return zero, false
	}

	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		c.logger().Warn(ctx, "Failed to unmarshal value from Redis cache", log.Error(err))
		atomic.AddInt64(&c.missCount, 1)
		return zero, false
	}

	atomic.AddInt64(&c.hitCount, 1)
	return value, true
}

// Delete removes one field from its group's hash.
func (c *groupedRedisCache[T]) Delete(ctx context.Context, key GroupedCacheKey) error {
	if !c.enabled {
		return nil
	}

	if err := c.client.HDel(ctx, c.buildGroupKey(key.Group), key.Key).Err(); err != nil {
		c.logger().Warn(ctx, "Failed to delete value from Redis cache",
			log.String("key", key.ToString()), log.Error(err))
		return err
	}
	return nil
}

// DeleteGroup removes the group's whole hash.
//
// Unlink rather than Del: the hash can hold an entry per organization unit a resource reaches, and
// freeing that on the main thread would block Redis for as long as it takes.
func (c *groupedRedisCache[T]) DeleteGroup(ctx context.Context, group string) error {
	if !c.enabled {
		return nil
	}

	if err := c.client.Unlink(ctx, c.buildGroupKey(group)).Err(); err != nil {
		c.logger().Warn(ctx, "Failed to delete the cache group from Redis",
			log.String("group", group), log.Error(err))
		return err
	}
	return nil
}

// IsEnabled returns whether the cache is enabled.
func (c *groupedRedisCache[T]) IsEnabled() bool {
	return c.enabled
}

// GetName returns the name of the cache.
func (c *groupedRedisCache[T]) GetName() string {
	return c.name
}

// CleanupExpired is a no-op for Redis, which expires entries itself.
func (c *groupedRedisCache[T]) CleanupExpired() {}

// GetStats returns cache statistics.
func (c *groupedRedisCache[T]) GetStats() CacheStat {
	if !c.enabled {
		return CacheStat{Enabled: false}
	}

	hits := atomic.LoadInt64(&c.hitCount)
	misses := atomic.LoadInt64(&c.missCount)
	totalOps := hits + misses
	var hitRate float64
	if totalOps > 0 {
		hitRate = float64(hits) / float64(totalOps)
	}

	return CacheStat{
		Enabled:   true,
		HitCount:  hits,
		MissCount: misses,
		HitRate:   hitRate,
	}
}
