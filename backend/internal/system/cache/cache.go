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

// Package cache provides a centralized cache management system for different cache implementations.
package cache

import (
	"context"

	"github.com/thunder-id/thunderid/internal/system/log"
)

// CacheInterface defines the common interface for cache operations.
type CacheInterface[T any] interface {
	GetName() string
	Set(ctx context.Context, key CacheKey, value T) error
	Get(ctx context.Context, key CacheKey) (T, bool)
	Delete(ctx context.Context, key CacheKey) error
	Clear(ctx context.Context) error
	IsEnabled() bool
	GetStats() CacheStat
	CleanupExpired()
}

// Cache implements the CacheInterface for individual caches.
type Cache[T any] struct {
	enabled   bool
	cacheName string
	cacheImpl CacheInterface[T]
}

// GetName returns the name of the cache.
func (c *Cache[T]) GetName() string {
	return c.cacheName
}

// Set stores a value in the cache.
func (c *Cache[T]) Set(ctx context.Context, key CacheKey, value T) error {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "Cache"),
		log.String("cacheName", c.cacheName))

	if c.IsEnabled() && c.cacheImpl.IsEnabled() {
		if err := c.cacheImpl.Set(ctx, key, value); err != nil {
			logger.Warn("Failed to set value in the cache", log.String("key", key.ToString()), log.Error(err))
		}
	}

	return nil
}

// Get retrieves a value from the cache.
func (c *Cache[T]) Get(ctx context.Context, key CacheKey) (T, bool) {
	if c.IsEnabled() && c.cacheImpl.IsEnabled() {
		if value, found := c.cacheImpl.Get(ctx, key); found {
			return value, true
		}
	}

	var zero T
	return zero, false
}

// Delete removes a value from the cache.
func (c *Cache[T]) Delete(ctx context.Context, key CacheKey) error {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "Cache"),
		log.String("cacheName", c.cacheName))

	if c.IsEnabled() && c.cacheImpl.IsEnabled() {
		if err := c.cacheImpl.Delete(ctx, key); err != nil {
			logger.Warn("Failed to delete value from the cache", log.String("key", key.ToString()), log.Error(err))
		}
	}

	return nil
}

// Clear removes all entries in the cache.
func (c *Cache[T]) Clear(ctx context.Context) error {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "Cache"),
		log.String("cacheName", c.cacheName))

	if c.IsEnabled() && c.cacheImpl.IsEnabled() {
		logger.Debug("Clearing all entries in the cache")

		if err := c.cacheImpl.Clear(ctx); err != nil {
			logger.Warn("Failed to clear the cache", log.Error(err))
		}
	}

	return nil
}

// IsEnabled returns whether the cache is enabled.
func (c *Cache[T]) IsEnabled() bool {
	return c.enabled
}

// GetStats returns cache statistics.
func (c *Cache[T]) GetStats() CacheStat {
	if c.IsEnabled() && c.cacheImpl != nil {
		return c.cacheImpl.GetStats()
	}
	return CacheStat{Enabled: false}
}

// CleanupExpired cleans up expired entries in the cache.
func (c *Cache[T]) CleanupExpired() {
	if c.IsEnabled() && c.cacheImpl.IsEnabled() {
		c.cacheImpl.CleanupExpired()
	}
}
