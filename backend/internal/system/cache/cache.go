// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package cache provides a centralized cache management system for different cache implementations.
package cache

import (
	"context"

	"github.com/thunder-id/thunderid/internal/system/deployment"
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

// scoped returns the key an entry is held under: the key within the deployment the context acts for.
// A server can act for more than one deployment, each with its own rows under the same ids and
// handles, so a key on its own would hand one deployment's entry to another.
func scoped(ctx context.Context, key CacheKey) CacheKey {
	return CacheKey{Key: deployment.Resolve(ctx) + ":" + key.Key}
}

// Set stores a value in the cache.
func (c *Cache[T]) Set(ctx context.Context, key CacheKey, value T) error {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "Cache"),
		log.String("cacheName", c.cacheName))

	if c.IsEnabled() && c.cacheImpl.IsEnabled() {
		if err := c.cacheImpl.Set(ctx, scoped(ctx, key), value); err != nil {
			logger.Warn(ctx, "Failed to set value in the cache",
				log.String("key", key.ToString()), log.Error(err))
		}
	}

	return nil
}

// Get retrieves a value from the cache.
func (c *Cache[T]) Get(ctx context.Context, key CacheKey) (T, bool) {
	if c.IsEnabled() && c.cacheImpl.IsEnabled() {
		if value, found := c.cacheImpl.Get(ctx, scoped(ctx, key)); found {
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
		if err := c.cacheImpl.Delete(ctx, scoped(ctx, key)); err != nil {
			logger.Warn(ctx, "Failed to delete value from the cache",
				log.String("key", key.ToString()), log.Error(err))
		}
	}

	return nil
}

// Clear removes all entries in the cache.
func (c *Cache[T]) Clear(ctx context.Context) error {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "Cache"),
		log.String("cacheName", c.cacheName))

	if c.IsEnabled() && c.cacheImpl.IsEnabled() {
		logger.Debug(ctx, "Clearing all entries in the cache")

		if err := c.cacheImpl.Clear(ctx); err != nil {
			logger.Warn(ctx, "Failed to clear the cache", log.Error(err))
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
