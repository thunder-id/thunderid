// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/deployment"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

func scopedCacheFor(t *testing.T) *Cache[string] {
	t.Helper()
	cfg := &config.Config{
		Server: engineconfig.ServerConfig{Identifier: "configured"},
		Cache:  engineconfig.CacheConfig{Size: 10, TTL: 60, EvictionPolicy: "LRU"},
	}
	config.ResetServerRuntime()
	require.NoError(t, config.InitializeServerRuntime("", cfg))
	t.Cleanup(config.ResetServerRuntime)
	return &Cache[string]{
		enabled:   true,
		cacheName: "scoped",
		cacheImpl: newInMemoryCache[string]("scoped", true, cfg.Cache, engineconfig.CacheProperty{}),
	}
}

// Two deployments a server acts for can hold entries under the same key, such as each one's root
// organization unit's handle, and neither reads, nor removes, the other's.
func TestACacheKeepsEachDeploymentsEntriesApart(t *testing.T) {
	c := scopedCacheFor(t)
	acme := deployment.WithID(context.Background(), "acme")
	globex := deployment.WithID(context.Background(), "globex")
	key := CacheKey{Key: "default"}

	require.NoError(t, c.Set(acme, key, "acme root"))
	_, found := c.Get(globex, key)
	assert.False(t, found, "one deployment read another's entry")

	require.NoError(t, c.Set(globex, key, "globex root"))
	value, found := c.Get(acme, key)
	assert.True(t, found)
	assert.Equal(t, "acme root", value)

	require.NoError(t, c.Delete(globex, key))
	value, found = c.Get(acme, key)
	assert.True(t, found, "removing one deployment's entry removed another's")
	assert.Equal(t, "acme root", value)
	_, found = c.Get(globex, key)
	assert.False(t, found)
}

// Work outside a request acts for the configured deployment, so it shares entries with the requests
// that name that deployment.
func TestACacheWithoutARequestUsesTheConfiguredDeployment(t *testing.T) {
	c := scopedCacheFor(t)
	key := CacheKey{Key: "k"}

	require.NoError(t, c.Set(context.Background(), key, "v"))
	value, found := c.Get(deployment.WithID(context.Background(), "configured"), key)
	assert.True(t, found)
	assert.Equal(t, "v", value)
	_, found = c.Get(deployment.WithID(context.Background(), "acme"), key)
	assert.False(t, found)
}
