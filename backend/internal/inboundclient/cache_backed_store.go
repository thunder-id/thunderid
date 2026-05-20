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

// Package inboundclient provides the inbound client persistence and service layer.
package inboundclient

import (
	"context"

	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const (
	inboundClientCacheName = "InboundClientByEntityIDCache"
	oauthProfileCacheName  = "OAuthProfileByEntityIDCache"
)

// cachedBackStore wraps an inboundClientStoreInterface with an in-memory cache for
// GetInboundClientByEntityID and GetOAuthProfileByEntityID. Writes invalidate/refresh cache
// entries.
type cachedBackStore struct {
	inboundClientCache cache.CacheInterface[*inboundmodel.InboundClient]
	oauthProfileCache  cache.CacheInterface[*inboundmodel.OAuthProfile]
	inner              inboundClientStoreInterface
}

// newCachedBackStore wraps an existing inboundClientStoreInterface with caching.
func newCachedBackStore(inner inboundClientStoreInterface,
	inboundClientCache cache.CacheInterface[*inboundmodel.InboundClient],
	oauthProfileCache cache.CacheInterface[*inboundmodel.OAuthProfile]) inboundClientStoreInterface {
	return &cachedBackStore{
		inboundClientCache: inboundClientCache,
		oauthProfileCache:  oauthProfileCache,
		inner:              inner,
	}
}

func (c *cachedBackStore) CreateInboundClient(ctx context.Context, client inboundmodel.InboundClient) error {
	if err := c.inner.CreateInboundClient(ctx, client); err != nil {
		return err
	}
	c.cacheInboundClient(ctx, &client)
	return nil
}

func (c *cachedBackStore) CreateOAuthProfile(ctx context.Context, entityID string,
	oauthProfile *inboundmodel.OAuthProfile) error {
	return c.inner.CreateOAuthProfile(ctx, entityID, oauthProfile)
}

func (c *cachedBackStore) GetInboundClientByEntityID(ctx context.Context, entityID string) (
	*inboundmodel.InboundClient, error) {
	key := cache.CacheKey{Key: entityID}
	if cached, ok := c.inboundClientCache.Get(ctx, key); ok {
		return cached, nil
	}

	client, err := c.inner.GetInboundClientByEntityID(ctx, entityID)
	if err != nil || client == nil {
		return client, err
	}
	c.cacheInboundClient(ctx, client)
	return client, nil
}

func (c *cachedBackStore) GetOAuthProfileByEntityID(ctx context.Context, entityID string) (
	*inboundmodel.OAuthProfile, error) {
	key := cache.CacheKey{Key: entityID}
	if cached, ok := c.oauthProfileCache.Get(ctx, key); ok {
		return cached, nil
	}

	profile, err := c.inner.GetOAuthProfileByEntityID(ctx, entityID)
	if err != nil || profile == nil {
		return profile, err
	}
	c.cacheOAuthProfile(ctx, entityID, profile)
	return profile, nil
}

func (c *cachedBackStore) GetInboundClientList(ctx context.Context, limit int) ([]inboundmodel.InboundClient, error) {
	return c.inner.GetInboundClientList(ctx, limit)
}

func (c *cachedBackStore) GetTotalInboundClientCount(ctx context.Context) (int, error) {
	return c.inner.GetTotalInboundClientCount(ctx)
}

func (c *cachedBackStore) UpdateInboundClient(ctx context.Context, client inboundmodel.InboundClient) error {
	if err := c.inner.UpdateInboundClient(ctx, client); err != nil {
		return err
	}
	c.invalidateInboundClient(ctx, client.ID)
	c.cacheInboundClient(ctx, &client)
	return nil
}

func (c *cachedBackStore) UpdateOAuthProfile(ctx context.Context, entityID string,
	oauthProfile *inboundmodel.OAuthProfile) error {
	if err := c.inner.UpdateOAuthProfile(ctx, entityID, oauthProfile); err != nil {
		return err
	}
	c.invalidateOAuthProfile(ctx, entityID)
	return nil
}

func (c *cachedBackStore) DeleteInboundClient(ctx context.Context, entityID string) error {
	if err := c.inner.DeleteInboundClient(ctx, entityID); err != nil {
		return err
	}
	c.invalidateInboundClient(ctx, entityID)
	c.invalidateOAuthProfile(ctx, entityID)
	return nil
}

func (c *cachedBackStore) DeleteOAuthProfile(ctx context.Context, entityID string) error {
	if err := c.inner.DeleteOAuthProfile(ctx, entityID); err != nil {
		return err
	}
	c.invalidateOAuthProfile(ctx, entityID)
	return nil
}

func (c *cachedBackStore) InboundClientExists(ctx context.Context, entityID string) (bool, error) {
	return c.inner.InboundClientExists(ctx, entityID)
}

func (c *cachedBackStore) IsDeclarative(ctx context.Context, entityID string) bool {
	return c.inner.IsDeclarative(ctx, entityID)
}

// --- Cache helpers ---

func (c *cachedBackStore) cacheInboundClient(ctx context.Context, client *inboundmodel.InboundClient) {
	if client == nil || client.ID == "" {
		return
	}
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "InboundClientCachedBackStore"))
	if err := c.inboundClientCache.Set(ctx, cache.CacheKey{Key: client.ID}, client); err != nil {
		logger.Error("Failed to cache inbound client", log.String("entityID", client.ID), log.Error(err))
	}
}

func (c *cachedBackStore) cacheOAuthProfile(ctx context.Context, entityID string, profile *inboundmodel.OAuthProfile) {
	if profile == nil || entityID == "" {
		return
	}
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "InboundClientCachedBackStore"))
	if err := c.oauthProfileCache.Set(ctx, cache.CacheKey{Key: entityID}, profile); err != nil {
		logger.Error("Failed to cache OAuth profile", log.String("entityID", entityID), log.Error(err))
	}
}

func (c *cachedBackStore) invalidateInboundClient(ctx context.Context, entityID string) {
	if entityID == "" {
		return
	}
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "InboundClientCachedBackStore"))
	if err := c.inboundClientCache.Delete(ctx, cache.CacheKey{Key: entityID}); err != nil {
		logger.Error("Failed to invalidate inbound client cache", log.String("entityID", entityID), log.Error(err))
	}
}

func (c *cachedBackStore) invalidateOAuthProfile(ctx context.Context, entityID string) {
	if entityID == "" {
		return
	}
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "InboundClientCachedBackStore"))
	if err := c.oauthProfileCache.Delete(ctx, cache.CacheKey{Key: entityID}); err != nil {
		logger.Error("Failed to invalidate OAuth profile cache", log.String("entityID", entityID), log.Error(err))
	}
}
