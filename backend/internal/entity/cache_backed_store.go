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

// cacheBackedEntityStore wraps an entityStoreInterface with in-memory caching
// for individual entity lookups by ID and identifier filter resolution.
type cacheBackedEntityStore struct {
	entityByIDCache cache.CacheInterface[*Entity]
	store           entityStoreInterface
	logger          *log.Logger
}

// newCacheBackedEntityStore creates a cache-backed wrapper around the given store.
func newCacheBackedEntityStore(store entityStoreInterface,
	entityByIDCache cache.CacheInterface[*Entity]) entityStoreInterface {
	return &cacheBackedEntityStore{
		entityByIDCache: entityByIDCache,
		store:           store,
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
	return s.store.GetEntityWithCredentials(ctx, id)
}

func (s *cacheBackedEntityStore) UpdateEntity(ctx context.Context, entity *Entity) error {
	if err := s.store.UpdateEntity(ctx, entity); err != nil {
		return err
	}

	s.invalidateEntityByID(ctx, entity.ID)
	s.cacheEntityByID(ctx, entity)
	return nil
}

func (s *cacheBackedEntityStore) UpdateAttributes(ctx context.Context,
	entityID string, attributes json.RawMessage) error {
	if err := s.store.UpdateAttributes(ctx, entityID, attributes); err != nil {
		return err
	}

	s.invalidateEntityByID(ctx, entityID)
	return nil
}

func (s *cacheBackedEntityStore) UpdateSystemAttributes(ctx context.Context,
	entityID string, attrs json.RawMessage) error {
	if err := s.store.UpdateSystemAttributes(ctx, entityID, attrs); err != nil {
		return err
	}

	s.invalidateEntityByID(ctx, entityID)
	return nil
}

func (s *cacheBackedEntityStore) UpdateCredentials(ctx context.Context,
	entityID string, creds json.RawMessage) error {
	if err := s.store.UpdateCredentials(ctx, entityID, creds); err != nil {
		return err
	}

	s.invalidateEntityByID(ctx, entityID)
	return nil
}

func (s *cacheBackedEntityStore) UpdateSystemCredentials(ctx context.Context,
	entityID string, creds json.RawMessage) error {
	if err := s.store.UpdateSystemCredentials(ctx, entityID, creds); err != nil {
		return err
	}

	s.invalidateEntityByID(ctx, entityID)
	return nil
}

func (s *cacheBackedEntityStore) DeleteEntity(ctx context.Context, id string) error {
	if err := s.store.DeleteEntity(ctx, id); err != nil {
		return err
	}

	s.invalidateEntityByID(ctx, id)
	return nil
}

func (s *cacheBackedEntityStore) IdentifyEntity(ctx context.Context,
	filters map[string]interface{}) (*string, error) {
	entityID, err := s.store.IdentifyEntity(ctx, filters)
	if err != nil || entityID == nil {
		return entityID, err
	}

	return entityID, nil
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

func (s *cacheBackedEntityStore) cacheEntityByID(ctx context.Context, entity *Entity) {
	if entity == nil || entity.ID == "" {
		return
	}
	if err := s.entityByIDCache.Set(ctx, cache.CacheKey{Key: entity.ID}, entity); err != nil {
		s.logger.Error("Failed to cache entity by ID",
			log.String("entityID", entity.ID), log.Error(err))
	}
}

func (s *cacheBackedEntityStore) invalidateEntityByID(ctx context.Context, entityID string) {
	if entityID == "" {
		return
	}
	if err := s.entityByIDCache.Delete(ctx, cache.CacheKey{Key: entityID}); err != nil {
		s.logger.Error("Failed to invalidate entity cache by ID",
			log.String("entityID", entityID), log.Error(err))
	}
}
