// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package serverconfig

import (
	"context"
	"encoding/json"
	"time"

	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const serverConfigCacheName = "ServerConfigByNameCache"

// serverConfigCacheTTL is how long a node keeps a section it read, unless cache.properties sets a TTL for
// the cache. A write made on another node reaches this one within it.
const serverConfigCacheTTL = 30 * time.Second

// cachedBackStore wraps a serverConfigStoreInterface with an in-memory cache for GetServerConfig.
// Writes invalidate the affected cache entry.
type cachedBackStore struct {
	configCache cache.CacheInterface[*ServerConfig]
	inner       serverConfigStoreInterface
}

// newCachedBackStore wraps an existing serverConfigStoreInterface with caching.
func newCachedBackStore(inner serverConfigStoreInterface,
	configCache cache.CacheInterface[*ServerConfig]) serverConfigStoreInterface {
	return &cachedBackStore{
		configCache: configCache,
		inner:       inner,
	}
}

func (c *cachedBackStore) GetServerConfig(ctx context.Context, name ConfigName) (storeLayers, error) {
	key := cache.CacheKey{Key: string(name)}
	if cached, ok := c.configCache.Get(ctx, key); ok {
		return storeLayers{Writable: cached.Value}, nil
	}

	layers, err := c.inner.GetServerConfig(ctx, name)
	if err != nil {
		return storeLayers{}, err
	}
	// Cache the result even when the section is unset (Value == nil) so repeated reads of an absent
	// section are served from the cache; writes invalidate the key.
	c.cacheServerConfig(ctx, ServerConfig{Name: name, Value: layers.Writable})
	return layers, nil
}

func (c *cachedBackStore) UpsertServerConfig(ctx context.Context, cfg ServerConfig) error {
	if err := c.inner.UpsertServerConfig(ctx, cfg); err != nil {
		return err
	}
	c.invalidateServerConfig(ctx, cfg.Name)
	return nil
}

// ReplaceServerConfigIf writes through to the database, and drops the cached layer whether or not it
// wrote: a write that lost read a stale layer, so the next read goes to the database.
func (c *cachedBackStore) ReplaceServerConfigIf(ctx context.Context, cfg ServerConfig,
	expected json.RawMessage) (bool, error) {
	written, err := c.inner.ReplaceServerConfigIf(ctx, cfg, expected)
	c.invalidateServerConfig(ctx, cfg.Name)
	return written, err
}

// --- Cache helpers ---

func (c *cachedBackStore) cacheServerConfig(ctx context.Context, cfg ServerConfig) {
	if cfg.Name == "" {
		return
	}
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ServerConfigCachedBackStore"))
	if err := c.configCache.Set(ctx, cache.CacheKey{Key: string(cfg.Name)}, &cfg); err != nil {
		logger.Error(ctx, "Failed to cache server config", log.String("name", string(cfg.Name)), log.Error(err))
	}
}

func (c *cachedBackStore) invalidateServerConfig(ctx context.Context, name ConfigName) {
	if name == "" {
		return
	}
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ServerConfigCachedBackStore"))
	if err := c.configCache.Delete(ctx, cache.CacheKey{Key: string(name)}); err != nil {
		logger.Error(ctx, "Failed to invalidate server config cache",
			log.String("name", string(name)), log.Error(err))
	}
}
