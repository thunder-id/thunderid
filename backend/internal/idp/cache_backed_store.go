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

// cacheablePropertyKeys lists the property keys whose lookup results are cached.
// Adding a key here means every distinct value for that key gets a cache entry —
// keep the list small to avoid unbounded cache growth.
var cacheablePropertyKeys = map[string]bool{
	PropIssuer: true,
}

// cacheBackedIDPStore wraps any idpStoreInterface with two in-memory caches.
type cacheBackedIDPStore struct {
	idpByIDCache       cache.CacheInterface[*IDPDTO]
	idpByPropertyCache cache.CacheInterface[[]IDPDTO]
	inner              idpStoreInterface
}

// newCacheBackedIDPStore creates a new cache-backed IDP store wrapping the provided inner store.
func newCacheBackedIDPStore(
	idpByIDCache cache.CacheInterface[*IDPDTO],
	idpByPropertyCache cache.CacheInterface[[]IDPDTO],
	inner idpStoreInterface,
) idpStoreInterface {
	return &cacheBackedIDPStore{
		idpByIDCache:       idpByIDCache,
		idpByPropertyCache: idpByPropertyCache,
		inner:              inner,
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

// GetIdentityProviderByName delegates to inner store and populates the ID cache with the result.
func (s *cacheBackedIDPStore) GetIdentityProviderByName(ctx context.Context, idpName string) (*IDPDTO, error) {
	idp, err := s.inner.GetIdentityProviderByName(ctx, idpName)
	if err != nil || idp == nil {
		return idp, err
	}
	s.cacheIDP(ctx, idp)
	return idp, nil
}

// GetIdentityProvidersByProperty retrieves IDPs by property, using the property cache on a hit.
// Only property keys listed in cacheablePropertyKeys are cached; all others bypass the cache.
func (s *cacheBackedIDPStore) GetIdentityProvidersByProperty(ctx context.Context,
	propertyKey, propertyValue string) ([]IDPDTO, error) {
	if !cacheablePropertyKeys[propertyKey] {
		return s.inner.GetIdentityProvidersByProperty(ctx, propertyKey, propertyValue)
	}

	cacheKey := cache.CacheKey{Key: propertyKey + ":" + propertyValue}
	if cached, ok := s.idpByPropertyCache.Get(ctx, cacheKey); ok {
		if cached == nil {
			return nil, ErrIDPNotFound
		}
		return cached, nil
	}

	idps, err := s.inner.GetIdentityProvidersByProperty(ctx, propertyKey, propertyValue)
	if err != nil {
		if errors.Is(err, ErrIDPNotFound) {
			_ = s.idpByPropertyCache.Set(ctx, cacheKey, nil)
		}
		return nil, err
	}
	_ = s.idpByPropertyCache.Set(ctx, cacheKey, idps)
	return idps, nil
}

// UpdateIdentityProvider fetches the old IDP to capture its properties, delegates the update, then
// invalidates old cache entries and caches the new state.
func (s *cacheBackedIDPStore) UpdateIdentityProvider(ctx context.Context, idp *IDPDTO) error {
	oldIDP, err := s.inner.GetIdentityProvider(ctx, idp.ID)
	if err != nil {
		return err
	}

	if err := s.inner.UpdateIdentityProvider(ctx, idp); err != nil {
		return err
	}

	s.invalidateIDP(ctx, oldIDP)
	s.cacheIDP(ctx, idp)
	return nil
}

// DeleteIdentityProvider fetches the IDP to get its properties before delegating deletion,
// then invalidates the relevant cache entries.
func (s *cacheBackedIDPStore) DeleteIdentityProvider(ctx context.Context, id string) error {
	existing, err := s.inner.GetIdentityProvider(ctx, id)
	if err != nil && !errors.Is(err, ErrIDPNotFound) {
		return err
	}

	if err := s.inner.DeleteIdentityProvider(ctx, id); err != nil {
		return err
	}

	s.invalidateIDP(ctx, existing)
	return nil
}

// cacheIDP stores the IDP in the ID cache only.
// Property cache is populated lazily on read (GetIdentityProvidersByProperty).
func (s *cacheBackedIDPStore) cacheIDP(ctx context.Context, idp *IDPDTO) {
	if idp == nil || idp.ID == "" {
		return
	}
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, cacheBackedIDPStoreLoggerComponentName))

	idKey := cache.CacheKey{Key: idp.ID}
	if err := s.idpByIDCache.Set(ctx, idKey, idp); err != nil {
		logger.Error(ctx, "Failed to cache IDP by ID", log.Error(err), log.String("idpID", idp.ID))
	}
}

// invalidateIDP removes the IDP from ID cache and invalidates property cache entries.
func (s *cacheBackedIDPStore) invalidateIDP(ctx context.Context, idp *IDPDTO) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, cacheBackedIDPStoreLoggerComponentName))
	if idp == nil {
		return
	}
	if idp.ID != "" {
		idKey := cache.CacheKey{Key: idp.ID}
		if err := s.idpByIDCache.Delete(ctx, idKey); err != nil {
			logger.Error(ctx, "Failed to invalidate IDP cache by ID",
				log.Error(err), log.String("idpID", idp.ID))
		}
	}
	for _, prop := range idp.Properties {
		if !cacheablePropertyKeys[prop.GetName()] {
			continue
		}
		val, err := prop.GetValue()
		if err != nil || val == "" {
			continue
		}
		propKey := cache.CacheKey{Key: prop.GetName() + ":" + val}
		if err := s.idpByPropertyCache.Delete(ctx, propKey); err != nil {
			logger.Error(ctx, "Failed to invalidate IDP property cache", log.Error(err))
		}
	}
}
