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

package entity

import (
	"context"
	"encoding/json"

	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// defaultCacheableIdentifiers lists the filter keys eligible for single-key
// IdentifyEntity cache lookups.
var defaultCacheableIdentifiers = []string{"clientId"}

// cacheBackedEntityStore wraps an entityStoreInterface with in-memory caching
// for individual entity lookups by ID and identifier filter resolution.
type cacheBackedEntityStore struct {
	entityByIDCache                cache.CacheInterface[*Entity]
	entityWithCredentialsByIDCache cache.CacheInterface[*entityWithCredentials]
	entityIDByIdentifierCache      cache.CacheInterface[*string]
	cacheableIdentifiers           map[string]bool
	store                          entityStoreInterface
	logger                         *log.Logger
}

// newCacheBackedEntityStore wraps a store with read-through caching.
func newCacheBackedEntityStore(store entityStoreInterface,
	entityByIDCache cache.CacheInterface[*Entity],
	entityWithCredentialsByIDCache cache.CacheInterface[*entityWithCredentials],
	entityIDByIdentifierCache cache.CacheInterface[*string]) entityStoreInterface {
	idSet := make(map[string]bool, len(defaultCacheableIdentifiers))
	for _, id := range defaultCacheableIdentifiers {
		idSet[id] = true
	}
	return &cacheBackedEntityStore{
		entityByIDCache:                entityByIDCache,
		entityWithCredentialsByIDCache: entityWithCredentialsByIDCache,
		entityIDByIdentifierCache:      entityIDByIdentifierCache,
		cacheableIdentifiers:           idSet,
		store:                          store,
		logger: log.GetLogger().With(
			log.String(log.LoggerKeyComponentName, "CacheBackedEntityStore")),
	}
}

func (s *cacheBackedEntityStore) CreateEntity(ctx context.Context, entity Entity,
	credentials json.RawMessage, systemCredentials json.RawMessage) error {
	if err := s.store.CreateEntity(ctx, entity, credentials, systemCredentials); err != nil {
		return err
	}
	s.cacheEntityByID(ctx, &entity)
	s.cacheEntityIDByIdentifiers(ctx, &entity)
	return nil
}

func (s *cacheBackedEntityStore) GetEntity(ctx context.Context, id string) (Entity, error) {
	cacheKey := cache.CacheKey{Key: id}
	if cached, ok := s.entityByIDCache.Get(ctx, cacheKey); ok {
		return *cached, nil
	}

	entity, err := s.store.GetEntity(ctx, id)
	if err != nil {
		return entity, err
	}

	s.cacheEntityByID(ctx, &entity)
	return entity, nil
}

func (s *cacheBackedEntityStore) GetEntityWithCredentials(ctx context.Context,
	id string) (*entityWithCredentials, error) {
	cacheKey := cache.CacheKey{Key: id}
	if cached, ok := s.entityWithCredentialsByIDCache.Get(ctx, cacheKey); ok {
		return cached, nil
	}

	result, err := s.store.GetEntityWithCredentials(ctx, id)
	if err != nil {
		return nil, err
	}

	s.cacheEntityWithCredentialsByID(ctx, result)
	return result, nil
}

func (s *cacheBackedEntityStore) UpdateEntity(ctx context.Context, entity *Entity) error {
	s.invalidateIdentifierCache(ctx, entity.ID)
	s.invalidateEntityByID(ctx, entity.ID)

	if err := s.store.UpdateEntity(ctx, entity); err != nil {
		return err
	}

	s.cacheEntityByID(ctx, entity)
	s.cacheEntityIDByIdentifiers(ctx, entity)
	return nil
}

func (s *cacheBackedEntityStore) UpdateAttributes(ctx context.Context,
	entityID string, attributes json.RawMessage) error {
	s.invalidateIdentifierCache(ctx, entityID)
	s.invalidateEntityByID(ctx, entityID)

	return s.store.UpdateAttributes(ctx, entityID, attributes)
}

func (s *cacheBackedEntityStore) UpdateSystemAttributes(ctx context.Context,
	entityID string, attrs json.RawMessage) error {
	s.invalidateIdentifierCache(ctx, entityID)
	s.invalidateEntityByID(ctx, entityID)

	return s.store.UpdateSystemAttributes(ctx, entityID, attrs)
}

func (s *cacheBackedEntityStore) UpdateCredentials(ctx context.Context,
	entityID string, creds json.RawMessage) error {
	s.invalidateEntityByID(ctx, entityID)

	return s.store.UpdateCredentials(ctx, entityID, creds)
}

func (s *cacheBackedEntityStore) UpdateSystemCredentials(ctx context.Context,
	entityID string, creds json.RawMessage) error {
	s.invalidateEntityByID(ctx, entityID)

	return s.store.UpdateSystemCredentials(ctx, entityID, creds)
}

func (s *cacheBackedEntityStore) DeleteEntity(ctx context.Context, id string) error {
	// Invalidate identifier cache before the store delete so the store fallback
	// can still fetch the entity if the by-ID cache is cold.
	s.invalidateIdentifierCache(ctx, id)

	if err := s.store.DeleteEntity(ctx, id); err != nil {
		return err
	}

	s.invalidateEntityByID(ctx, id)
	return nil
}

func (s *cacheBackedEntityStore) IdentifyEntity(ctx context.Context,
	filters map[string]interface{}) (*string, error) {
	if len(filters) == 1 {
		for filterKey, filterVal := range filters {
			val, ok := filterVal.(string)
			if !ok || val == "" || !s.cacheableIdentifiers[filterKey] {
				return s.store.IdentifyEntity(ctx, filters)
			}
			compositeKey := identifierCacheKey(filterKey, val)
			if cached, hit := s.entityIDByIdentifierCache.Get(ctx, compositeKey); hit {
				return cached, nil
			}

			entityID, err := s.store.IdentifyEntity(ctx, filters)
			if err != nil || entityID == nil {
				return entityID, err
			}

			if err := s.entityIDByIdentifierCache.Set(ctx,
				compositeKey, entityID); err != nil {
				s.logger.Error(ctx, "Failed to cache entity ID by identifier",
					log.String("key", filterKey), log.String("value", val), log.Error(err))
			}
			return entityID, nil
		}
	}

	return s.store.IdentifyEntity(ctx, filters)
}

// Pass-through methods.

func (s *cacheBackedEntityStore) SearchEntities(ctx context.Context,
	filters map[string]interface{}) ([]Entity, error) {
	return s.store.SearchEntities(ctx, filters)
}

func (s *cacheBackedEntityStore) GetEntityListCount(ctx context.Context,
	category string, filters map[string]interface{}) (int, error) {
	return s.store.GetEntityListCount(ctx, category, filters)
}

func (s *cacheBackedEntityStore) GetEntityList(ctx context.Context,
	category string, limit, offset int, filters map[string]interface{}) ([]Entity, error) {
	return s.store.GetEntityList(ctx, category, limit, offset, filters)
}

func (s *cacheBackedEntityStore) GetEntityListCountByOUIDs(ctx context.Context,
	category string, ouIDs []string, filters map[string]interface{}) (int, error) {
	return s.store.GetEntityListCountByOUIDs(ctx, category, ouIDs, filters)
}

func (s *cacheBackedEntityStore) GetEntityListByOUIDs(ctx context.Context,
	category string, ouIDs []string, limit, offset int,
	filters map[string]interface{}) ([]Entity, error) {
	return s.store.GetEntityListByOUIDs(ctx, category, ouIDs, limit, offset, filters)
}

func (s *cacheBackedEntityStore) ValidateEntityIDs(ctx context.Context,
	entityIDs []string) ([]string, error) {
	return s.store.ValidateEntityIDs(ctx, entityIDs)
}

func (s *cacheBackedEntityStore) GetEntitiesByIDs(ctx context.Context,
	entityIDs []string) ([]Entity, error) {
	return s.store.GetEntitiesByIDs(ctx, entityIDs)
}

func (s *cacheBackedEntityStore) ValidateEntityIDsInOUs(ctx context.Context,
	entityIDs []string, ouIDs []string) ([]string, error) {
	return s.store.ValidateEntityIDsInOUs(ctx, entityIDs, ouIDs)
}

func (s *cacheBackedEntityStore) GetGroupCountForEntity(ctx context.Context,
	entityID string) (int, error) {
	return s.store.GetGroupCountForEntity(ctx, entityID)
}

func (s *cacheBackedEntityStore) GetEntityGroups(ctx context.Context,
	entityID string, limit, offset int) ([]EntityGroup, error) {
	return s.store.GetEntityGroups(ctx, entityID, limit, offset)
}

func (s *cacheBackedEntityStore) GetTransitiveEntityGroups(ctx context.Context,
	entityID string) ([]EntityGroup, error) {
	return s.store.GetTransitiveEntityGroups(ctx, entityID)
}

func (s *cacheBackedEntityStore) IsEntityDeclarative(ctx context.Context, id string) (bool, error) {
	return s.store.IsEntityDeclarative(ctx, id)
}

func (s *cacheBackedEntityStore) GetIndexedAttributes() map[string]bool {
	return s.store.GetIndexedAttributes()
}

func (s *cacheBackedEntityStore) LoadIndexedAttributes(attributes []string) error {
	return s.store.LoadIndexedAttributes(attributes)
}

// --- Cache helpers ---

func identifierCacheKey(filterKey, filterValue string) cache.CacheKey {
	return cache.CacheKey{Key: filterKey + ":" + filterValue}
}

// parseEntityAttributes unmarshals the entity's SystemAttributes and Attributes
// into a single merged map. SystemAttributes take precedence on key collisions.
func (s *cacheBackedEntityStore) parseEntityAttributes(ctx context.Context, entity *Entity) map[string]interface{} {
	if entity == nil {
		return nil
	}
	merged := make(map[string]interface{})
	for _, raw := range []json.RawMessage{entity.Attributes, entity.SystemAttributes} {
		if len(raw) == 0 {
			continue
		}
		var attrs map[string]interface{}
		if err := json.Unmarshal(raw, &attrs); err != nil {
			s.logger.Warn(ctx, "Failed to unmarshal entity attributes for cache key resolution",
				log.String("entityID", entity.ID), log.Error(err))
			continue
		}
		for k, v := range attrs {
			merged[k] = v
		}
	}
	return merged
}

func (s *cacheBackedEntityStore) cacheEntityIDByIdentifiers(ctx context.Context, entity *Entity) {
	if entity == nil || entity.ID == "" {
		return
	}
	attrs := s.parseEntityAttributes(ctx, entity)
	for key := range s.cacheableIdentifiers {
		val, _ := attrs[key].(string)
		if val == "" {
			continue
		}
		if err := s.entityIDByIdentifierCache.Set(ctx,
			identifierCacheKey(key, val), &entity.ID); err != nil {
			s.logger.Error(ctx, "Failed to cache entity ID by identifier",
				log.String("key", key), log.String("value", val), log.Error(err))
		}
	}
}

func (s *cacheBackedEntityStore) invalidateIdentifierCache(ctx context.Context, entityID string) {
	if entityID == "" || len(s.cacheableIdentifiers) == 0 {
		return
	}
	var entity *Entity
	if cached, ok := s.entityByIDCache.Get(ctx, cache.CacheKey{Key: entityID}); ok && cached != nil {
		entity = cached
	} else {
		fetched, err := s.store.GetEntity(ctx, entityID)
		if err != nil {
			s.logger.Error(ctx, "Failed to fetch entity for identifier cache invalidation",
				log.String("entityID", entityID), log.Error(err))
			return
		}
		entity = &fetched
	}
	attrs := s.parseEntityAttributes(ctx, entity)
	for key := range s.cacheableIdentifiers {
		val, _ := attrs[key].(string)
		if val == "" {
			continue
		}
		if err := s.entityIDByIdentifierCache.Delete(ctx,
			identifierCacheKey(key, val)); err != nil {
			s.logger.Error(ctx, "Failed to invalidate identifier cache",
				log.String("key", key), log.String("value", val), log.Error(err))
		}
	}
}

func (s *cacheBackedEntityStore) cacheEntityByID(ctx context.Context, entity *Entity) {
	if entity == nil || entity.ID == "" {
		return
	}
	if err := s.entityByIDCache.Set(ctx, cache.CacheKey{Key: entity.ID}, entity); err != nil {
		s.logger.Error(ctx, "Failed to cache entity by ID",
			log.String("entityID", entity.ID), log.Error(err))
	}
}

func (s *cacheBackedEntityStore) cacheEntityWithCredentialsByID(ctx context.Context,
	ewc *entityWithCredentials) {
	if ewc == nil || ewc.Entity == nil || ewc.Entity.ID == "" {
		return
	}
	if err := s.entityWithCredentialsByIDCache.Set(ctx,
		cache.CacheKey{Key: ewc.Entity.ID}, ewc); err != nil {
		s.logger.Error(ctx, "Failed to cache entity with credentials by ID",
			log.String("entityID", ewc.Entity.ID), log.Error(err))
	}
}

func (s *cacheBackedEntityStore) invalidateEntityByID(ctx context.Context, entityID string) {
	if entityID == "" {
		return
	}
	if err := s.entityByIDCache.Delete(ctx, cache.CacheKey{Key: entityID}); err != nil {
		s.logger.Error(ctx, "Failed to invalidate entity cache by ID",
			log.String("entityID", entityID), log.Error(err))
	}
	if err := s.entityWithCredentialsByIDCache.Delete(ctx, cache.CacheKey{Key: entityID}); err != nil {
		s.logger.Error(ctx, "Failed to invalidate entity with credentials cache by ID",
			log.String("entityID", entityID), log.Error(err))
	}
}
