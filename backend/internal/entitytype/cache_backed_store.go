// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entitytype

import (
	"context"
	"errors"

	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// cachedBackedEntityTypeStore wraps a entityTypeStoreInterface with in-memory caching
// for individual schema lookups by ID and handle. Cache keys are namespaced by category so the
// same handle in user vs agent categories never collide.
type cachedBackedEntityTypeStore struct {
	schemaByIDCache     cache.CacheInterface[*EntityType]
	schemaByHandleCache cache.CacheInterface[*EntityType]
	store               entityTypeStoreInterface
	logger              *log.Logger
}

// newCachedBackedEntityTypeStore creates a cache-backed wrapper around the given store.
func newCachedBackedEntityTypeStore(
	store entityTypeStoreInterface,
	entityTypeByIDCache cache.CacheInterface[*EntityType],
	entityTypeByHandleCache cache.CacheInterface[*EntityType],
) entityTypeStoreInterface {
	return &cachedBackedEntityTypeStore{
		schemaByIDCache:     entityTypeByIDCache,
		schemaByHandleCache: entityTypeByHandleCache,
		store:               store,
		logger: log.GetLogger().With(
			log.String(log.LoggerKeyComponentName, "CacheBackedEntityTypeStore")),
	}
}

func cacheKeyForID(category TypeCategory, schemaID string) cache.CacheKey {
	return cache.CacheKey{Key: string(category) + ":" + schemaID}
}

func cacheKeyForHandle(category TypeCategory, handle string) cache.CacheKey {
	return cache.CacheKey{Key: string(category) + ":" + handle}
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

// GetEntityTypeByHandle retrieves an entity type by handle, checking cache first.
func (s *cachedBackedEntityTypeStore) GetEntityTypeByHandle(ctx context.Context, category TypeCategory,
	handle string) (EntityType, error) {
	cacheKey := cacheKeyForHandle(category, handle)
	if cached, ok := s.schemaByHandleCache.Get(ctx, cacheKey); ok {
		return *cached, nil
	}

	schema, err := s.store.GetEntityTypeByHandle(ctx, category, handle)
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

// UpdateEntityTypeByID updates an entity type, invalidates its cache entries, and caches the new state.
func (s *cachedBackedEntityTypeStore) UpdateEntityTypeByID(
	ctx context.Context, category TypeCategory, schemaID string, entityType EntityType,
) error {
	if err := s.store.UpdateEntityTypeByID(ctx, category, schemaID, entityType); err != nil {
		return err
	}

	s.invalidateEntityTypeCache(ctx, category, schemaID, entityType.Handle)
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
		s.invalidateEntityTypeCache(ctx, existing.Category, existing.ID, existing.Handle)
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

// GetDisplayAttributesByHandles delegates to the underlying store.
func (s *cachedBackedEntityTypeStore) GetDisplayAttributesByHandles(
	ctx context.Context, category TypeCategory, handles []string,
) (map[string]string, error) {
	return s.store.GetDisplayAttributesByHandles(ctx, category, handles)
}

// cacheEntityType populates both ID and handle caches for the given schema.
func (s *cachedBackedEntityTypeStore) cacheEntityType(ctx context.Context, schema *EntityType) {
	if schema == nil || schema.Category == "" {
		return
	}

	if schema.ID != "" {
		key := cacheKeyForID(schema.Category, schema.ID)
		if err := s.schemaByIDCache.Set(ctx, key, schema); err != nil {
			s.logger.Error(ctx, "Failed to cache entity type by ID",
				log.String("schemaID", schema.ID), log.Error(err))
		}
	}

	if schema.Handle != "" {
		key := cacheKeyForHandle(schema.Category, schema.Handle)
		if err := s.schemaByHandleCache.Set(ctx, key, schema); err != nil {
			s.logger.Error(ctx, "Failed to cache entity type by handle",
				log.String("handle", schema.Handle), log.Error(err))
		}
	}
}

// invalidateEntityTypeCache removes entries from both ID and handle caches.
func (s *cachedBackedEntityTypeStore) invalidateEntityTypeCache(ctx context.Context,
	category TypeCategory, schemaID, handle string) {
	if schemaID != "" {
		key := cacheKeyForID(category, schemaID)
		if err := s.schemaByIDCache.Delete(ctx, key); err != nil {
			s.logger.Error(ctx, "Failed to invalidate entity type cache by ID",
				log.String("schemaID", schemaID), log.Error(err))
		}
	}

	if handle != "" {
		key := cacheKeyForHandle(category, handle)
		if err := s.schemaByHandleCache.Delete(ctx, key); err != nil {
			s.logger.Error(ctx, "Failed to invalidate entity type cache by handle",
				log.String("handle", handle), log.Error(err))
		}
	}
}
