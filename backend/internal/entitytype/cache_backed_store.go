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

package entitytype

import (
	"context"
	"errors"

	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// cachedBackedEntityTypeStore wraps a entityTypeStoreInterface with in-memory caching
// for individual schema lookups by ID and Name. Cache keys are namespaced by category so the
// same name in user vs agent categories never collide.
type cachedBackedEntityTypeStore struct {
	schemaByIDCache   cache.CacheInterface[*EntityType]
	schemaByNameCache cache.CacheInterface[*EntityType]
	store             entityTypeStoreInterface
	logger            *log.Logger
}

// newCachedBackedEntityTypeStore creates a cache-backed wrapper around the given store.
func newCachedBackedEntityTypeStore(
	store entityTypeStoreInterface,
	entityTypeByIDCache cache.CacheInterface[*EntityType],
	entityTypeByNameCache cache.CacheInterface[*EntityType],
) entityTypeStoreInterface {
	return &cachedBackedEntityTypeStore{
		schemaByIDCache:   entityTypeByIDCache,
		schemaByNameCache: entityTypeByNameCache,
		store:             store,
		logger: log.GetLogger().With(
			log.String(log.LoggerKeyComponentName, "CacheBackedEntityTypeStore")),
	}
}

func cacheKeyForID(category TypeCategory, schemaID string) cache.CacheKey {
	return cache.CacheKey{Key: string(category) + ":" + schemaID}
}

func cacheKeyForName(category TypeCategory, name string) cache.CacheKey {
	return cache.CacheKey{Key: string(category) + ":" + name}
}

// GetEntityTypeByID retrieves an entity type by ID, checking cache first.
func (s *cachedBackedEntityTypeStore) GetEntityTypeByID(ctx context.Context, category TypeCategory,
	schemaID string) (EntityType, error) {
	cacheKey := cacheKeyForID(category, schemaID)
	if cached, ok := s.schemaByIDCache.Get(ctx, cacheKey); ok {
		return *cached, nil
	}

	schema, err := s.store.GetEntityTypeByID(ctx, category, schemaID)
	if err != nil {
		return schema, err
	}

	s.cacheEntityType(ctx, &schema)

	return schema, nil
}

// GetEntityTypeByName retrieves an entity type by name, checking cache first.
func (s *cachedBackedEntityTypeStore) GetEntityTypeByName(ctx context.Context, category TypeCategory,
	name string) (EntityType, error) {
	cacheKey := cacheKeyForName(category, name)
	if cached, ok := s.schemaByNameCache.Get(ctx, cacheKey); ok {
		return *cached, nil
	}

	schema, err := s.store.GetEntityTypeByName(ctx, category, name)
	if err != nil {
		return schema, err
	}

	s.cacheEntityType(ctx, &schema)

	return schema, nil
}

// CreateEntityType creates an entity type and populates the cache.
func (s *cachedBackedEntityTypeStore) CreateEntityType(ctx context.Context, entityType EntityType) error {
	if err := s.store.CreateEntityType(ctx, entityType); err != nil {
		return err
	}

	s.cacheEntityType(ctx, &entityType)

	return nil
}

// UpdateEntityTypeByID updates an entity type, invalidates old cache entries, and caches the new state.
func (s *cachedBackedEntityTypeStore) UpdateEntityTypeByID(
	ctx context.Context, category TypeCategory, schemaID string, entityType EntityType,
) error {
	existingCacheKey := cacheKeyForID(category, schemaID)
	existing, ok := s.schemaByIDCache.Get(ctx, existingCacheKey)
	if !ok {
		existingSchema, err := s.store.GetEntityTypeByID(ctx, category, schemaID)
		if err == nil {
			existing = &existingSchema
		}
	}

	if err := s.store.UpdateEntityTypeByID(ctx, category, schemaID, entityType); err != nil {
		return err
	}

	if existing != nil {
		s.invalidateEntityTypeCache(ctx, existing.Category, existing.ID, existing.Name)
	}

	s.cacheEntityType(ctx, &entityType)

	return nil
}

// DeleteEntityTypeByID deletes an entity type and invalidates its cache entries.
func (s *cachedBackedEntityTypeStore) DeleteEntityTypeByID(ctx context.Context, category TypeCategory,
	schemaID string) error {
	cacheKey := cacheKeyForID(category, schemaID)
	existing, ok := s.schemaByIDCache.Get(ctx, cacheKey)
	if !ok {
		existingSchema, err := s.store.GetEntityTypeByID(ctx, category, schemaID)
		if err != nil {
			if errors.Is(err, ErrEntityTypeNotFound) {
				return nil
			}
			return err
		}
		existing = &existingSchema
	}

	if err := s.store.DeleteEntityTypeByID(ctx, category, schemaID); err != nil {
		return err
	}

	if existing != nil {
		s.invalidateEntityTypeCache(ctx, existing.Category, existing.ID, existing.Name)
	}

	return nil
}

// GetEntityTypeListCount delegates to the underlying store.
func (s *cachedBackedEntityTypeStore) GetEntityTypeListCount(ctx context.Context,
	category TypeCategory) (int, error) {
	return s.store.GetEntityTypeListCount(ctx, category)
}

// GetEntityTypeList delegates to the underlying store.
func (s *cachedBackedEntityTypeStore) GetEntityTypeList(
	ctx context.Context, category TypeCategory, limit, offset int,
) ([]EntityTypeListItem, error) {
	return s.store.GetEntityTypeList(ctx, category, limit, offset)
}

// GetEntityTypeListByOUIDs delegates to the underlying store.
func (s *cachedBackedEntityTypeStore) GetEntityTypeListByOUIDs(
	ctx context.Context, category TypeCategory, ouIDs []string, limit, offset int,
) ([]EntityTypeListItem, error) {
	return s.store.GetEntityTypeListByOUIDs(ctx, category, ouIDs, limit, offset)
}

// GetEntityTypeListCountByOUIDs delegates to the underlying store.
func (s *cachedBackedEntityTypeStore) GetEntityTypeListCountByOUIDs(
	ctx context.Context, category TypeCategory, ouIDs []string,
) (int, error) {
	return s.store.GetEntityTypeListCountByOUIDs(ctx, category, ouIDs)
}

// IsEntityTypeDeclarative delegates to the underlying store.
func (s *cachedBackedEntityTypeStore) IsEntityTypeDeclarative(category TypeCategory, schemaID string) bool {
	return s.store.IsEntityTypeDeclarative(category, schemaID)
}

// GetDisplayAttributesByNames delegates to the underlying store.
func (s *cachedBackedEntityTypeStore) GetDisplayAttributesByNames(
	ctx context.Context, category TypeCategory, names []string,
) (map[string]string, error) {
	return s.store.GetDisplayAttributesByNames(ctx, category, names)
}

// cacheEntityType populates both ID and Name caches for the given schema.
func (s *cachedBackedEntityTypeStore) cacheEntityType(ctx context.Context, schema *EntityType) {
	if schema == nil || schema.Category == "" {
		return
	}

	if schema.ID != "" {
		key := cacheKeyForID(schema.Category, schema.ID)
		if err := s.schemaByIDCache.Set(ctx, key, schema); err != nil {
			s.logger.Error("Failed to cache entity type by ID",
				log.String("schemaID", schema.ID), log.Error(err))
		}
	}

	if schema.Name != "" {
		key := cacheKeyForName(schema.Category, schema.Name)
		if err := s.schemaByNameCache.Set(ctx, key, schema); err != nil {
			s.logger.Error("Failed to cache entity type by name",
				log.String("schemaName", schema.Name), log.Error(err))
		}
	}
}

// invalidateEntityTypeCache removes entries from both ID and Name caches.
func (s *cachedBackedEntityTypeStore) invalidateEntityTypeCache(ctx context.Context,
	category TypeCategory, schemaID, schemaName string) {
	if schemaID != "" {
		key := cacheKeyForID(category, schemaID)
		if err := s.schemaByIDCache.Delete(ctx, key); err != nil {
			s.logger.Error("Failed to invalidate entity type cache by ID",
				log.String("schemaID", schemaID), log.Error(err))
		}
	}

	if schemaName != "" {
		key := cacheKeyForName(category, schemaName)
		if err := s.schemaByNameCache.Delete(ctx, key); err != nil {
			s.logger.Error("Failed to invalidate entity type cache by name",
				log.String("schemaName", schemaName), log.Error(err))
		}
	}
}
