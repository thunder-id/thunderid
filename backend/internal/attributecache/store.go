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

package attributecache

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// attributeCacheStoreInterface defines the interface for the attribute cache store.
type attributeCacheStoreInterface interface {
	// CreateAttributeCache creates a new attribute cache entry in the store.
	CreateAttributeCache(ctx context.Context, cache AttributeCache) error

	// GetAttributeCache retrieves an attribute cache entry by ID from the store.
	GetAttributeCache(ctx context.Context, id string) (AttributeCache, error)

	// ExtendAttributeCacheTTL extends the TTL of an attribute cache entry in the store.
	ExtendAttributeCacheTTL(ctx context.Context, id string, ttlSeconds int) error

	// DeleteAttributeCache deletes an attribute cache entry by ID from the store.
	DeleteAttributeCache(ctx context.Context, id string) error
}

// attributeCacheStore is the SQL implementation of attributeCacheStoreInterface.
type attributeCacheStore struct {
	store providers.RuntimeStoreProvider
}

// newAttributeCacheStore creates a new instance of attributeCacheStore.
func newAttributeCacheStore(store providers.RuntimeStoreProvider) attributeCacheStoreInterface {
	return &attributeCacheStore{
		store: store,
	}
}

// CreateAttributeCache creates a new attribute cache entry in the database.
func (s *attributeCacheStore) CreateAttributeCache(ctx context.Context, cache AttributeCache) error {
	data, err := json.Marshal(cache.Attributes)
	if err != nil {
		return fmt.Errorf("failed to marshal attributes: %w", err)
	}
	return s.store.Put(ctx, providers.NamespaceAttributeCache, cache.ID, data, cache.TTLSeconds)
}

// GetAttributeCache retrieves an attribute cache entry by ID from the database.
func (s *attributeCacheStore) GetAttributeCache(ctx context.Context, id string) (AttributeCache, error) {
	data, err := s.store.Get(ctx, providers.NamespaceAttributeCache, id)
	if err != nil {
		return AttributeCache{}, fmt.Errorf("failed to get attribute cache: %w", err)
	}
	if data == nil {
		return AttributeCache{}, errAttributeCacheNotFound
	}

	var attributes map[string]interface{}
	if err := json.Unmarshal(data, &attributes); err != nil {
		return AttributeCache{}, fmt.Errorf("failed to unmarshal attributes: %w", err)
	}

	return AttributeCache{
		ID:         id,
		Attributes: attributes,
	}, nil
}

// ExtendAttributeCacheTTL extends the TTL of an attribute cache entry in the database.
func (s *attributeCacheStore) ExtendAttributeCacheTTL(ctx context.Context, id string, ttlSeconds int) error {
	return s.store.ExtendTTL(ctx, providers.NamespaceAttributeCache, id, int64(ttlSeconds))
}

// DeleteAttributeCache deletes an attribute cache entry by ID from the database.
func (s *attributeCacheStore) DeleteAttributeCache(ctx context.Context, id string) error {
	return s.store.Delete(ctx, providers.NamespaceAttributeCache, id)
}
