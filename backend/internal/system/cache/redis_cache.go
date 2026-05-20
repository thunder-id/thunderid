/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// redisCache implements the internalCacheInterface backed by Redis.
type redisCache[T any] struct {
	enabled   bool
	name      string
	client    *redis.Client
	ttl       time.Duration
	keyPrefix string
	hitCount  int64
	missCount int64
}

// newRedisCache creates a new instance of redisCache.
func newRedisCache[T any](name string, enabled bool, client *redis.Client, keyPrefix string,
	cacheConfig config.CacheConfig, cacheProperty config.CacheProperty) CacheInterface[T] {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "RedisCache"),
		log.String("name", name))

	if !enabled {
		logger.Warn("Redis cache is disabled, returning empty cache")
		return &redisCache[T]{
			name:    name,
			enabled: false,
		}
	}

	ttl := getCacheTTL(cacheConfig, cacheProperty)

	logger.Debug("Initializing Redis cache", log.Any("ttl", ttl),
		log.String("keyPrefix", keyPrefix))

	return &redisCache[T]{
		enabled:   true,
		name:      name,
		client:    client,
		ttl:       ttl,
		keyPrefix: keyPrefix,
	}
}

// buildKey constructs the full Redis key with prefix and cache name.
func (c *redisCache[T]) buildKey(key CacheKey) string {
	return c.keyPrefix + ":" + c.name + ":" + key.Key
}

// Set stores a value in Redis.
func (c *redisCache[T]) Set(ctx context.Context, key CacheKey, value T) error {
	if !c.enabled {
		return nil
	}

	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "RedisCache"),
		log.String("name", c.name))

	data, err := json.Marshal(value)
	if err != nil {
		logger.Warn("Failed to marshal value for Redis cache", log.Error(err))
		return err
	}

	fullKey := c.buildKey(key)

	if err := c.client.Set(ctx, fullKey, data, c.ttl).Err(); err != nil {
		logger.Warn("Failed to set value in Redis cache", log.Error(err))
		return err
	}

	logger.Debug("Cache entry set in Redis", log.String("key", key.ToString()))
	return nil
}

// Get retrieves a value from Redis.
func (c *redisCache[T]) Get(ctx context.Context, key CacheKey) (T, bool) {
	var zero T
	if !c.enabled {
		return zero, false
	}

	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "RedisCache"),
		log.String("name", c.name))

	fullKey := c.buildKey(key)

	data, err := c.client.Get(ctx, fullKey).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			atomic.AddInt64(&c.missCount, 1)
			return zero, false
		}
		logger.Warn("Failed to get value from Redis cache", log.Error(err))
		atomic.AddInt64(&c.missCount, 1)
		return zero, false
	}

	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		logger.Warn("Failed to unmarshal value from Redis cache", log.Error(err))
		atomic.AddInt64(&c.missCount, 1)
		return zero, false
	}

	atomic.AddInt64(&c.hitCount, 1)
	logger.Debug("Cache hit from Redis", log.String("key", key.ToString()))
	return value, true
}

// Delete removes a value from Redis.
func (c *redisCache[T]) Delete(ctx context.Context, key CacheKey) error {
	if !c.enabled {
		return nil
	}

	fullKey := c.buildKey(key)

	if err := c.client.Del(ctx, fullKey).Err(); err != nil {
		log.GetLogger().Warn("Failed to delete value from Redis cache",
			log.Error(err))
		return err
	}

	return nil
}

// Clear is a no-op for Redis as TTL-based expiry is handled natively by Redis.
func (c *redisCache[T]) Clear(_ context.Context) error { return nil }

// IsEnabled returns whether the cache is enabled.
func (c *redisCache[T]) IsEnabled() bool {
	return c.enabled
}

// GetName returns the name of the cache.
func (c *redisCache[T]) GetName() string {
	return c.name
}

// CleanupExpired is a no-op for Redis as TTL-based expiry is handled natively by Redis.
func (c *redisCache[T]) CleanupExpired() {}

// GetStats returns cache statistics.
func (c *redisCache[T]) GetStats() CacheStat {
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
