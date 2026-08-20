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

	"github.com/thunder-id/thunderid/internal/system/deployment"
)

// tenantScopedCache wraps a cache so every keyed operation is namespaced by the request's deployment
// id. Stores partition their rows by deployment id; without this, a cache keyed only by resource
// id/name would serve one tenant's value to another in token (multi-tenant) mode. In server mode the
// deployment id is a single constant, so the namespace is a stable no-op prefix and behavior is
// unchanged. Clear is intentionally not namespaced (it clears every tenant's entries): over-clearing
// is safe, and per-tenant invalidation would require key enumeration the backing caches do not offer.
type tenantScopedCache[T any] struct {
	inner CacheInterface[T]
}

// scopeKey namespaces a key by the request's deployment id. An empty deployment id (no tenant in a
// token-mode background context) leaves the key unscoped.
func (c *tenantScopedCache[T]) scopeKey(ctx context.Context, key CacheKey) CacheKey {
	id := deployment.ResolveDefault(ctx)
	if id == "" {
		return key
	}
	return CacheKey{Key: id + "|" + key.Key}
}

func (c *tenantScopedCache[T]) Set(ctx context.Context, key CacheKey, value T) error {
	return c.inner.Set(ctx, c.scopeKey(ctx, key), value)
}

func (c *tenantScopedCache[T]) Get(ctx context.Context, key CacheKey) (T, bool) {
	return c.inner.Get(ctx, c.scopeKey(ctx, key))
}

func (c *tenantScopedCache[T]) Delete(ctx context.Context, key CacheKey) error {
	return c.inner.Delete(ctx, c.scopeKey(ctx, key))
}

func (c *tenantScopedCache[T]) Clear(ctx context.Context) error { return c.inner.Clear(ctx) }
func (c *tenantScopedCache[T]) GetName() string                 { return c.inner.GetName() }
func (c *tenantScopedCache[T]) IsEnabled() bool                 { return c.inner.IsEnabled() }
func (c *tenantScopedCache[T]) GetStats() CacheStat             { return c.inner.GetStats() }
func (c *tenantScopedCache[T]) CleanupExpired()                 { c.inner.CleanupExpired() }
