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

package idp

import (
	"context"
	"errors"

	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const cacheBackedIDPStoreLoggerComponentName = "CacheBackedIDPStore"

// cacheBackedIDPStore wraps any idpStoreInterface with two in-memory caches.
type cacheBackedIDPStore struct {
	idpByIDCache     cache.CacheInterface[*IDPDTO]
	idpByIssuerCache cache.CacheInterface[*IDPDTO]
	inner            idpStoreInterface
}

// newCacheBackedIDPStore creates a new cache-backed IDP store wrapping the provided inner store.
func newCacheBackedIDPStore(
	idpByIDCache cache.CacheInterface[*IDPDTO],
	idpByIssuerCache cache.CacheInterface[*IDPDTO],
	inner idpStoreInterface,
) idpStoreInterface {
	return &cacheBackedIDPStore{
		idpByIDCache:     idpByIDCache,
		idpByIssuerCache: idpByIssuerCache,
		inner:            inner,
	}
}

// CreateIdentityProvider delegates to the inner store and caches the created IDP.
func (s *cacheBackedIDPStore) CreateIdentityProvider(ctx context.Context, idp IDPDTO) error {
	if err := s.inner.CreateIdentityProvider(ctx, idp); err != nil {
		return err
	}
	s.cacheIDP(ctx, &idp)
	return nil
}

// GetIdentityProviderList delegates to the inner store without caching.
func (s *cacheBackedIDPStore) GetIdentityProviderList(ctx context.Context) ([]BasicIDPDTO, error) {
	return s.inner.GetIdentityProviderList(ctx)
}

// GetIdentityProviderListCount delegates to the inner store without caching.
func (s *cacheBackedIDPStore) GetIdentityProviderListCount(ctx context.Context) (int, error) {
	return s.inner.GetIdentityProviderListCount(ctx)
}

// GetIdentityProvider retrieves an IDP by ID, using the ID cache on a hit.
func (s *cacheBackedIDPStore) GetIdentityProvider(ctx context.Context, idpID string) (*IDPDTO, error) {
	cacheKey := cache.CacheKey{Key: idpID}
	if cached, ok := s.idpByIDCache.Get(ctx, cacheKey); ok {
		return cached, nil
	}

	idp, err := s.inner.GetIdentityProvider(ctx, idpID)
	if err != nil || idp == nil {
		return idp, err
	}
	s.cacheIDP(ctx, idp)
	return idp, nil
}

// GetIdentityProviderByName delegates to inner store and populates the ID and issuer caches with the result.
func (s *cacheBackedIDPStore) GetIdentityProviderByName(ctx context.Context, idpName string) (*IDPDTO, error) {
	idp, err := s.inner.GetIdentityProviderByName(ctx, idpName)
	if err != nil || idp == nil {
		return idp, err
	}
	s.cacheIDP(ctx, idp)
	return idp, nil
}

// GetIdentityProviderByIssuer retrieves an IDP by its issuer property, using the issuer cache on a hit.
// A nil cached value means the absence of an IDP for that issuer was previously recorded.
func (s *cacheBackedIDPStore) GetIdentityProviderByIssuer(ctx context.Context, issuer string) (*IDPDTO, error) {
	cacheKey := cache.CacheKey{Key: issuer}
	if cached, ok := s.idpByIssuerCache.Get(ctx, cacheKey); ok {
		if cached == nil {
			return nil, ErrIDPNotFound
		}
		return cached, nil
	}

	idp, err := s.inner.GetIdentityProviderByIssuer(ctx, issuer)
	if err != nil {
		if errors.Is(err, ErrIDPNotFound) {
			_ = s.idpByIssuerCache.Set(ctx, cacheKey, nil)
		}
		return nil, err
	}
	if idp == nil {
		_ = s.idpByIssuerCache.Set(ctx, cacheKey, nil)
		return nil, ErrIDPNotFound
	}
	s.cacheIDP(ctx, idp)
	return idp, nil
}

// UpdateIdentityProvider fetches the old IDP to capture its issuer, delegates the update, then
// invalidates old cache entries and caches the new state.
func (s *cacheBackedIDPStore) UpdateIdentityProvider(ctx context.Context, idp *IDPDTO) error {
	oldIDP, err := s.inner.GetIdentityProvider(ctx, idp.ID)
	if err != nil {
		return err
	}

	var oldIssuer string
	if oldIDP != nil {
		oldIssuer = GetPropertyValue(oldIDP.Properties, PropIssuer)
	}

	if err := s.inner.UpdateIdentityProvider(ctx, idp); err != nil {
		return err
	}

	s.invalidateIDP(ctx, idp.ID, oldIssuer)
	s.cacheIDP(ctx, idp)
	return nil
}

// DeleteIdentityProvider fetches the IDP to get its issuer before delegating deletion,
// then invalidates the relevant cache entries.
func (s *cacheBackedIDPStore) DeleteIdentityProvider(ctx context.Context, id string) error {
	existing, err := s.inner.GetIdentityProvider(ctx, id)
	if err != nil && !errors.Is(err, ErrIDPNotFound) {
		return err
	}

	var issuer string
	if existing != nil {
		issuer = GetPropertyValue(existing.Properties, PropIssuer)
	}

	if err := s.inner.DeleteIdentityProvider(ctx, id); err != nil {
		return err
	}

	s.invalidateIDP(ctx, id, issuer)
	return nil
}

// cacheIDP stores the IDP in both the ID cache and (when an issuer is present) the issuer cache.
func (s *cacheBackedIDPStore) cacheIDP(ctx context.Context, idp *IDPDTO) {
	if idp == nil {
		return
	}
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, cacheBackedIDPStoreLoggerComponentName))

	if idp.ID != "" {
		idKey := cache.CacheKey{Key: idp.ID}
		if err := s.idpByIDCache.Set(ctx, idKey, idp); err != nil {
			logger.Error("Failed to cache IDP by ID", log.Error(err), log.String("idpID", idp.ID))
		}
	}

	issuer := GetPropertyValue(idp.Properties, PropIssuer)
	if issuer != "" {
		issuerKey := cache.CacheKey{Key: issuer}
		if err := s.idpByIssuerCache.Set(ctx, issuerKey, idp); err != nil {
			logger.Error("Failed to cache IDP by issuer", log.Error(err), log.String("issuer", issuer))
		}
	}
}

// invalidateIDP removes the IDP from both the ID cache and the issuer cache.
func (s *cacheBackedIDPStore) invalidateIDP(ctx context.Context, id, issuer string) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, cacheBackedIDPStoreLoggerComponentName))

	if id != "" {
		idKey := cache.CacheKey{Key: id}
		if err := s.idpByIDCache.Delete(ctx, idKey); err != nil {
			logger.Error("Failed to invalidate IDP cache by ID", log.Error(err), log.String("idpID", id))
		}
	}

	if issuer != "" {
		issuerKey := cache.CacheKey{Key: issuer}
		if err := s.idpByIssuerCache.Delete(ctx, issuerKey); err != nil {
			logger.Error("Failed to invalidate IDP cache by issuer", log.Error(err), log.String("issuer", issuer))
		}
	}
}
