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

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
)

// compositeEntityTypeStore implements a composite store that combines file-based (immutable) and
// database (mutable) stores.
// - Read operations query both stores and merge results
// - Write operations (Create/Update/Delete) only affect the database store
// - Declarative entity types (from YAML files) cannot be modified or deleted
type compositeEntityTypeStore struct {
	fileStore entityTypeStoreInterface
	dbStore   entityTypeStoreInterface
}

// newCompositeEntityTypeStore creates a new composite store with both file-based and database stores.
func newCompositeEntityTypeStore(fileStore, dbStore entityTypeStoreInterface) *compositeEntityTypeStore {
	return &compositeEntityTypeStore{
		fileStore: fileStore,
		dbStore:   dbStore,
	}
}

// GetEntityTypeListCount retrieves the total count of entity types from both stores within a category.
func (c *compositeEntityTypeStore) GetEntityTypeListCount(ctx context.Context,
	category TypeCategory) (int, error) {
	return declarativeresource.CompositeMergeCountHelper(
		func() (int, error) { return c.dbStore.GetEntityTypeListCount(ctx, category) },
		func() (int, error) { return c.fileStore.GetEntityTypeListCount(ctx, category) },
	)
}

// GetEntityTypeList retrieves entity types from both stores with pagination within a category.
func (c *compositeEntityTypeStore) GetEntityTypeList(
	ctx context.Context, category TypeCategory, limit, offset int,
) ([]EntityTypeListItem, error) {
	items, limitExceeded, err := declarativeresource.CompositeMergeListHelperWithLimit(
		func() (int, error) { return c.dbStore.GetEntityTypeListCount(ctx, category) },
		func() (int, error) { return c.fileStore.GetEntityTypeListCount(ctx, category) },
		func(count int) ([]EntityTypeListItem, error) {
			return c.dbStore.GetEntityTypeList(ctx, category, count, 0)
		},
		func(count int) ([]EntityTypeListItem, error) {
			return c.fileStore.GetEntityTypeList(ctx, category, count, 0)
		},
		mergeAndDeduplicateEntityTypes,
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
	return items, nil
}

// GetEntityTypeListCountByOUIDs retrieves the total count of entity types filtered by OU IDs and
// category from both stores.
func (c *compositeEntityTypeStore) GetEntityTypeListCountByOUIDs(ctx context.Context,
	category TypeCategory, ouIDs []string) (int, error) {
	return declarativeresource.CompositeMergeCountHelper(
		func() (int, error) { return c.dbStore.GetEntityTypeListCountByOUIDs(ctx, category, ouIDs) },
		func() (int, error) { return c.fileStore.GetEntityTypeListCountByOUIDs(ctx, category, ouIDs) },
	)
}

// GetEntityTypeListByOUIDs retrieves entity types filtered by OU IDs and category from both stores.
func (c *compositeEntityTypeStore) GetEntityTypeListByOUIDs(
	ctx context.Context, category TypeCategory, ouIDs []string, limit, offset int,
) ([]EntityTypeListItem, error) {
	items, limitExceeded, err := declarativeresource.CompositeMergeListHelperWithLimit(
		func() (int, error) { return c.dbStore.GetEntityTypeListCountByOUIDs(ctx, category, ouIDs) },
		func() (int, error) { return c.fileStore.GetEntityTypeListCountByOUIDs(ctx, category, ouIDs) },
		func(count int) ([]EntityTypeListItem, error) {
			return c.dbStore.GetEntityTypeListByOUIDs(ctx, category, ouIDs, count, 0)
		},
		func(count int) ([]EntityTypeListItem, error) {
			return c.fileStore.GetEntityTypeListByOUIDs(ctx, category, ouIDs, count, 0)
		},
		mergeAndDeduplicateEntityTypes,
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
	return items, nil
}

// CreateEntityType creates a new entity type in the database store only.
// Conflict checking is handled at the service layer.
func (c *compositeEntityTypeStore) CreateEntityType(ctx context.Context, schema EntityType) error {
	return c.dbStore.CreateEntityType(ctx, schema)
}

// GetEntityTypeByID retrieves an entity type by ID from either store within a category.
// Checks database store first, then falls back to file store.
func (c *compositeEntityTypeStore) GetEntityTypeByID(ctx context.Context, category TypeCategory,
	schemaID string) (EntityType, error) {
	return declarativeresource.CompositeGetHelper(
		func() (EntityType, error) { return c.dbStore.GetEntityTypeByID(ctx, category, schemaID) },
		func() (EntityType, error) { return c.fileStore.GetEntityTypeByID(ctx, category, schemaID) },
		ErrEntityTypeNotFound,
	)
}

// GetEntityTypeByName retrieves an entity type by name from either store within a category.
func (c *compositeEntityTypeStore) GetEntityTypeByName(ctx context.Context, category TypeCategory,
	schemaName string) (EntityType, error) {
	return declarativeresource.CompositeGetHelper(
		func() (EntityType, error) { return c.dbStore.GetEntityTypeByName(ctx, category, schemaName) },
		func() (EntityType, error) { return c.fileStore.GetEntityTypeByName(ctx, category, schemaName) },
		ErrEntityTypeNotFound,
	)
}

// UpdateEntityTypeByID updates an entity type in the database store only.
func (c *compositeEntityTypeStore) UpdateEntityTypeByID(
	ctx context.Context, category TypeCategory, schemaID string, schema EntityType,
) error {
	return c.dbStore.UpdateEntityTypeByID(ctx, category, schemaID, schema)
}

// DeleteEntityTypeByID deletes an entity type from the database store only.
func (c *compositeEntityTypeStore) DeleteEntityTypeByID(ctx context.Context, category TypeCategory,
	schemaID string) error {
	return c.dbStore.DeleteEntityTypeByID(ctx, category, schemaID)
}

// IsEntityTypeDeclarative checks if an entity type is immutable (exists in file store) within a category.
func (c *compositeEntityTypeStore) IsEntityTypeDeclarative(category TypeCategory, schemaID string) bool {
	return declarativeresource.CompositeIsDeclarativeHelper(
		schemaID,
		func(id string) (bool, error) {
			_, err := c.fileStore.GetEntityTypeByID(context.Background(), category, id)
			if err != nil {
				return false, nil
			}
			return true, nil
		},
	)
}

// GetDisplayAttributesByNames retrieves display attributes from both stores within a category.
func (c *compositeEntityTypeStore) GetDisplayAttributesByNames(
	ctx context.Context, category TypeCategory, names []string,
) (map[string]string, error) {
	if len(names) == 0 {
		return map[string]string{}, nil
	}

	dbResult, dbErr := c.dbStore.GetDisplayAttributesByNames(ctx, category, names)
	if dbErr != nil {
		return nil, dbErr
	}

	fileResult, fileErr := c.fileStore.GetDisplayAttributesByNames(ctx, category, names)
	if fileErr != nil {
		return nil, fileErr
	}

	merged := make(map[string]string, len(dbResult)+len(fileResult))
	for name, display := range fileResult {
		merged[name] = display
	}
	for name, display := range dbResult {
		merged[name] = display
	}

	return merged, nil
}

// mergeAndDeduplicateEntityTypes merges entity types from both stores and removes duplicates by ID.
func mergeAndDeduplicateEntityTypes(
	dbSchemas, fileSchemas []EntityTypeListItem,
) []EntityTypeListItem {
	seen := make(map[string]bool)
	result := make([]EntityTypeListItem, 0, len(dbSchemas)+len(fileSchemas))

	for i := range dbSchemas {
		if !seen[dbSchemas[i].ID] {
			seen[dbSchemas[i].ID] = true
			schemaCopy := dbSchemas[i]
			schemaCopy.IsReadOnly = false
			result = append(result, schemaCopy)
		}
	}

	for i := range fileSchemas {
		if !seen[fileSchemas[i].ID] {
			seen[fileSchemas[i].ID] = true
			schemaCopy := fileSchemas[i]
			schemaCopy.IsReadOnly = true
			result = append(result, schemaCopy)
		}
	}

	return result
}
