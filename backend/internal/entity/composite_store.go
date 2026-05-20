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
	"errors"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
)

// entityCompositeStore implements a composite store that combines file-based (immutable) and
// database (mutable) stores.
type entityCompositeStore struct {
	fileStore entityStoreInterface
	dbStore   entityStoreInterface
}

// newEntityCompositeStore creates a new composite store with both file-based and database stores.
func newEntityCompositeStore(fileStore, dbStore entityStoreInterface) *entityCompositeStore {
	return &entityCompositeStore{
		fileStore: fileStore,
		dbStore:   dbStore,
	}
}

// CreateEntity creates a new entity in the database store only.
func (c *entityCompositeStore) CreateEntity(ctx context.Context, entity Entity,
	credentials json.RawMessage, systemCredentials json.RawMessage) error {
	return c.dbStore.CreateEntity(ctx, entity, credentials, systemCredentials)
}

// GetEntity retrieves an entity by ID from either store (DB first, then file fallback).
func (c *entityCompositeStore) GetEntity(ctx context.Context, id string) (Entity, error) {
	return declarativeresource.CompositeGetHelper(
		func() (Entity, error) { return c.dbStore.GetEntity(ctx, id) },
		func() (Entity, error) {
			entity, err := c.fileStore.GetEntity(ctx, id)
			if err != nil {
				return Entity{}, err
			}
			entity.IsReadOnly = true
			return entity, nil
		},
		ErrEntityNotFound,
	)
}

// GetEntityWithCredentials retrieves an entity with credentials from either store.
func (c *entityCompositeStore) GetEntityWithCredentials(ctx context.Context, id string) (
	*entityWithCredentials, error) {
	result, err := c.dbStore.GetEntityWithCredentials(ctx, id)
	if err == nil {
		return result, nil
	}
	if !errors.Is(err, ErrEntityNotFound) {
		return nil, err
	}

	result, err = c.fileStore.GetEntityWithCredentials(ctx, id)
	if err != nil {
		return nil, err
	}
	result.Entity.IsReadOnly = true
	return result, nil
}

// UpdateEntity fully updates an entity in the database store only.
func (c *entityCompositeStore) UpdateEntity(ctx context.Context, entity *Entity) error {
	return c.dbStore.UpdateEntity(ctx, entity)
}

// UpdateAttributes updates only the schema attributes in the database store only.
func (c *entityCompositeStore) UpdateAttributes(
	ctx context.Context, entityID string, attributes json.RawMessage) error {
	return c.dbStore.UpdateAttributes(ctx, entityID, attributes)
}

// UpdateSystemAttributes updates system attributes in the database store only.
func (c *entityCompositeStore) UpdateSystemAttributes(ctx context.Context, entityID string,
	attrs json.RawMessage) error {
	return c.dbStore.UpdateSystemAttributes(ctx, entityID, attrs)
}

// UpdateCredentials updates credentials in the database store only.
func (c *entityCompositeStore) UpdateCredentials(ctx context.Context, entityID string,
	creds json.RawMessage) error {
	return c.dbStore.UpdateCredentials(ctx, entityID, creds)
}

// UpdateSystemCredentials updates system credentials in the database store only.
func (c *entityCompositeStore) UpdateSystemCredentials(ctx context.Context, entityID string,
	creds json.RawMessage) error {
	return c.dbStore.UpdateSystemCredentials(ctx, entityID, creds)
}

// DeleteEntity deletes an entity from the database store only.
func (c *entityCompositeStore) DeleteEntity(ctx context.Context, id string) error {
	return c.dbStore.DeleteEntity(ctx, id)
}

// IdentifyEntity identifies an entity from either store (DB first, then file fallback).
func (c *entityCompositeStore) IdentifyEntity(ctx context.Context,
	filters map[string]interface{}) (*string, error) {
	return declarativeresource.CompositeGetHelper(
		func() (*string, error) { return c.dbStore.IdentifyEntity(ctx, filters) },
		func() (*string, error) { return c.fileStore.IdentifyEntity(ctx, filters) },
		ErrEntityNotFound,
	)
}

// SearchEntities searches for entities matching the given filters from both stores.
func (c *entityCompositeStore) SearchEntities(ctx context.Context,
	filters map[string]interface{}) ([]Entity, error) {
	var allEntities []Entity

	dbEntities, err := c.dbStore.SearchEntities(ctx, filters)
	if err != nil && !errors.Is(err, ErrEntityNotFound) {
		return nil, err
	}
	if len(dbEntities) > 0 {
		allEntities = append(allEntities, dbEntities...)
	}

	fileEntities, err := c.fileStore.SearchEntities(ctx, filters)
	if err != nil && !errors.Is(err, ErrEntityNotFound) {
		return nil, err
	}
	if len(fileEntities) > 0 {
		allEntities = append(allEntities, fileEntities...)
	}

	if len(allEntities) == 0 {
		return nil, ErrEntityNotFound
	}

	return mergeAndDeduplicateEntities(dbEntities, fileEntities), nil
}

// GetEntityListCount retrieves the total count of entities from both stores.
func (c *entityCompositeStore) GetEntityListCount(ctx context.Context, category string,
	filters map[string]interface{}) (int, error) {
	return c.getDistinctEntityCount(
		func() (int, error) { return c.dbStore.GetEntityListCount(ctx, category, filters) },
		func() (int, error) { return c.fileStore.GetEntityListCount(ctx, category, filters) },
		func(count int) ([]Entity, error) {
			return c.dbStore.GetEntityList(ctx, category, count, 0, filters)
		},
		func(count int) ([]Entity, error) {
			return c.fileStore.GetEntityList(ctx, category, count, 0, filters)
		},
	)
}

// GetEntityList retrieves entities from both stores with pagination.
func (c *entityCompositeStore) GetEntityList(ctx context.Context, category string,
	limit, offset int, filters map[string]interface{}) ([]Entity, error) {
	entities, limitExceeded, err := declarativeresource.CompositeMergeListHelperWithLimit(
		func() (int, error) { return c.dbStore.GetEntityListCount(ctx, category, filters) },
		func() (int, error) { return c.fileStore.GetEntityListCount(ctx, category, filters) },
		func(count int) ([]Entity, error) {
			return c.dbStore.GetEntityList(ctx, category, count, 0, filters)
		},
		func(count int) ([]Entity, error) {
			return c.fileStore.GetEntityList(ctx, category, count, 0, filters)
		},
		mergeAndDeduplicateEntities,
		limit,
		offset,
		serverconst.MaxCompositeStoreRecords,
	)
	if err != nil {
		return nil, err
	}
	if limitExceeded {
		return nil, errResultLimitExceededInCompositeMode
	}
	return entities, nil
}

// GetEntityListCountByOUIDs retrieves the total count of entities by OU IDs from both stores.
func (c *entityCompositeStore) GetEntityListCountByOUIDs(ctx context.Context, category string,
	ouIDs []string, filters map[string]interface{}) (int, error) {
	return c.getDistinctEntityCount(
		func() (int, error) { return c.dbStore.GetEntityListCountByOUIDs(ctx, category, ouIDs, filters) },
		func() (int, error) { return c.fileStore.GetEntityListCountByOUIDs(ctx, category, ouIDs, filters) },
		func(count int) ([]Entity, error) {
			return c.dbStore.GetEntityListByOUIDs(ctx, category, ouIDs, count, 0, filters)
		},
		func(count int) ([]Entity, error) {
			return c.fileStore.GetEntityListByOUIDs(ctx, category, ouIDs, count, 0, filters)
		},
	)
}

// GetEntityListByOUIDs retrieves entities scoped to OU IDs from both stores with pagination.
func (c *entityCompositeStore) GetEntityListByOUIDs(ctx context.Context, category string,
	ouIDs []string, limit, offset int, filters map[string]interface{}) ([]Entity, error) {
	entities, limitExceeded, err := declarativeresource.CompositeMergeListHelperWithLimit(
		func() (int, error) { return c.dbStore.GetEntityListCountByOUIDs(ctx, category, ouIDs, filters) },
		func() (int, error) { return c.fileStore.GetEntityListCountByOUIDs(ctx, category, ouIDs, filters) },
		func(count int) ([]Entity, error) {
			return c.dbStore.GetEntityListByOUIDs(ctx, category, ouIDs, count, 0, filters)
		},
		func(count int) ([]Entity, error) {
			return c.fileStore.GetEntityListByOUIDs(ctx, category, ouIDs, count, 0, filters)
		},
		mergeAndDeduplicateEntities,
		limit,
		offset,
		serverconst.MaxCompositeStoreRecords,
	)
	if err != nil {
		return nil, err
	}
	if limitExceeded {
		return nil, errResultLimitExceededInCompositeMode
	}
	return entities, nil
}

// ValidateEntityIDs checks if all provided entity IDs exist in either store.
func (c *entityCompositeStore) ValidateEntityIDs(ctx context.Context, entityIDs []string) ([]string, error) {
	invalidIDs := make([]string, 0)

	for _, id := range entityIDs {
		_, err := c.GetEntity(ctx, id)
		if err != nil {
			if errors.Is(err, ErrEntityNotFound) {
				invalidIDs = append(invalidIDs, id)
				continue
			}
			return nil, err
		}
	}

	return invalidIDs, nil
}

// GetEntitiesByIDs retrieves entities by a list of IDs from both stores.
func (c *entityCompositeStore) GetEntitiesByIDs(ctx context.Context, entityIDs []string) ([]Entity, error) {
	if len(entityIDs) == 0 {
		return []Entity{}, nil
	}

	dbEntities, err := c.dbStore.GetEntitiesByIDs(ctx, entityIDs)
	if err != nil {
		return nil, err
	}

	fileEntities, err := c.fileStore.GetEntitiesByIDs(ctx, entityIDs)
	if err != nil {
		return nil, err
	}

	return mergeAndDeduplicateEntities(dbEntities, fileEntities), nil
}

// ValidateEntityIDsInOUs checks which of the provided entity IDs belong to the given OU scope.
func (c *entityCompositeStore) ValidateEntityIDsInOUs(
	ctx context.Context, entityIDs []string, ouIDs []string,
) ([]string, error) {
	if len(entityIDs) == 0 {
		return []string{}, nil
	}
	if len(ouIDs) == 0 {
		return append([]string{}, entityIDs...), nil
	}

	ouSet := make(map[string]bool, len(ouIDs))
	for _, ou := range ouIDs {
		ouSet[ou] = true
	}

	outOfScope := make([]string, 0)
	for _, id := range entityIDs {
		entity, err := c.GetEntity(ctx, id)
		if err != nil {
			if errors.Is(err, ErrEntityNotFound) {
				outOfScope = append(outOfScope, id)
				continue
			}
			return nil, err
		}
		if !ouSet[entity.OUID] {
			outOfScope = append(outOfScope, id)
		}
	}
	return outOfScope, nil
}

// GetGroupCountForEntity delegates to DB store only (groups are for mutable entities).
func (c *entityCompositeStore) GetGroupCountForEntity(ctx context.Context, entityID string) (int, error) {
	return c.dbStore.GetGroupCountForEntity(ctx, entityID)
}

// GetEntityGroups delegates to DB store only (groups are for mutable entities).
func (c *entityCompositeStore) GetEntityGroups(ctx context.Context, entityID string,
	limit, offset int) ([]EntityGroup, error) {
	return c.dbStore.GetEntityGroups(ctx, entityID, limit, offset)
}

// GetTransitiveEntityGroups delegates to DB store only (groups are for mutable entities).
func (c *entityCompositeStore) GetTransitiveEntityGroups(ctx context.Context, entityID string) ([]EntityGroup, error) {
	return c.dbStore.GetTransitiveEntityGroups(ctx, entityID)
}

// IsEntityDeclarative checks if an entity is declarative (exists in file store).
func (c *entityCompositeStore) IsEntityDeclarative(ctx context.Context, id string) (bool, error) {
	isDeclarative, err := c.fileStore.IsEntityDeclarative(ctx, id)
	if err != nil {
		return false, err
	}
	if isDeclarative {
		return true, nil
	}
	// Not in file store; check DB to confirm entity exists and is mutable.
	return c.dbStore.IsEntityDeclarative(ctx, id)
}

// GetIndexedAttributes delegates to the database store.
func (c *entityCompositeStore) GetIndexedAttributes() map[string]bool {
	return c.dbStore.GetIndexedAttributes()
}

// LoadIndexedAttributes delegates to the database store.
func (c *entityCompositeStore) LoadIndexedAttributes(attributes []string) error {
	return c.dbStore.LoadIndexedAttributes(attributes)
}

// getDistinctEntityCount retrieves the count of distinct entities from both stores.
func (c *entityCompositeStore) getDistinctEntityCount(
	dbCount func() (int, error),
	fileCount func() (int, error),
	dbList func(count int) ([]Entity, error),
	fileList func(count int) ([]Entity, error),
) (int, error) {
	count, err := dbCount()
	if err != nil {
		return 0, err
	}
	fileCountValue, err := fileCount()
	if err != nil {
		return 0, err
	}

	entityIDs := make(map[string]struct{}, count+fileCountValue)
	if count > 0 {
		entities, err := dbList(count)
		if err != nil {
			return 0, err
		}
		for _, e := range entities {
			entityIDs[e.ID] = struct{}{}
		}
	}

	if fileCountValue > 0 {
		entities, err := fileList(fileCountValue)
		if err != nil {
			return 0, err
		}
		for _, e := range entities {
			entityIDs[e.ID] = struct{}{}
		}
	}

	return len(entityIDs), nil
}

// mergeAndDeduplicateEntities merges and deduplicates entities from two lists.
// Database entities take precedence over file-based entities when IDs conflict.
func mergeAndDeduplicateEntities(dbEntities, fileEntities []Entity) []Entity {
	seen := make(map[string]bool)
	result := make([]Entity, 0, len(dbEntities)+len(fileEntities))

	for i := range dbEntities {
		dbEntities[i].IsReadOnly = false
		result = append(result, dbEntities[i])
		seen[dbEntities[i].ID] = true
	}

	for i := range fileEntities {
		if !seen[fileEntities[i].ID] {
			fileEntities[i].IsReadOnly = true
			result = append(result, fileEntities[i])
			seen[fileEntities[i].ID] = true
		}
	}

	return result
}
