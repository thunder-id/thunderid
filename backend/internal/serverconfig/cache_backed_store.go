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

package serverconfig

import (
	"context"

	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const serverConfigCacheName = "ServerConfigByNameCache"

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
