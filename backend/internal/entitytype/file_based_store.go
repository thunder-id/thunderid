/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
	"sort"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
	"github.com/thunder-id/thunderid/internal/system/transaction"
)

type entityTypeFileBasedStore struct {
	*declarativeresource.GenericFileBasedStore
}

// Create implements declarative_resource.Storer interface for resource loader
func (f *entityTypeFileBasedStore) Create(id string, data interface{}) error {
	schema := data.(*EntityType)
	return f.CreateEntityType(context.Background(), *schema)
}

// CreateEntityType implements entityTypeStoreInterface.
func (f *entityTypeFileBasedStore) CreateEntityType(ctx context.Context, schema EntityType) error {
	return f.GenericFileBasedStore.Create(schema.ID, &schema)
}

// DeleteEntityTypeByID implements entityTypeStoreInterface.
func (f *entityTypeFileBasedStore) DeleteEntityTypeByID(ctx context.Context, category TypeCategory,
	id string) error {
	return errors.New("DeleteEntityTypeByID is not supported in file-based store")
}

// GetEntityTypeByID implements entityTypeStoreInterface.
func (f *entityTypeFileBasedStore) GetEntityTypeByID(ctx context.Context, category TypeCategory,
	schemaID string) (EntityType, error) {
	data, err := f.GenericFileBasedStore.Get(schemaID)
	if err != nil {
		return EntityType{}, ErrEntityTypeNotFound
	}
	schema, ok := data.(*EntityType)
	if !ok {
		declarativeresource.LogTypeAssertionError("entity type", schemaID)
		return EntityType{}, errors.New("entity type data corrupted")
	}
	if schema.Category != category {
		return EntityType{}, ErrEntityTypeNotFound
	}
	return *schema, nil
}

// GetEntityTypeByName implements entityTypeStoreInterface.
func (f *entityTypeFileBasedStore) GetEntityTypeByName(ctx context.Context, category TypeCategory,
	schemaName string) (EntityType, error) {
	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return EntityType{}, ErrEntityTypeNotFound
	}
	for _, item := range list {
		if schema, ok := item.Data.(*EntityType); ok {
			if schema.Name == schemaName && schema.Category == category {
				return *schema, nil
			}
		}
	}
	return EntityType{}, ErrEntityTypeNotFound
}

// GetEntityTypeList implements entityTypeStoreInterface.
func (f *entityTypeFileBasedStore) GetEntityTypeList(
	ctx context.Context, category TypeCategory, limit, offset int,
) ([]EntityTypeListItem, error) {
	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return nil, err
	}

	var schemaList []EntityTypeListItem
	for _, item := range list {
		if schema, ok := item.Data.(*EntityType); ok {
			if schema.Category != category {
				continue
			}
			schemaList = append(schemaList, EntityTypeListItem{
				ID:                    schema.ID,
				Category:              schema.Category,
				Name:                  schema.Name,
				OUID:                  schema.OUID,
				AllowSelfRegistration: schema.AllowSelfRegistration,
				SystemAttributes:      schema.SystemAttributes,
			})
		}
	}

	sort.Slice(schemaList, func(i, j int) bool {
		return schemaList[i].Name < schemaList[j].Name
	})

	start := offset
	end := offset + limit
	if start > len(schemaList) {
		return []EntityTypeListItem{}, nil
	}
	if end > len(schemaList) {
		end = len(schemaList)
	}

	return schemaList[start:end], nil
}

// GetEntityTypeListCount implements entityTypeStoreInterface.
func (f *entityTypeFileBasedStore) GetEntityTypeListCount(ctx context.Context,
	category TypeCategory) (int, error) {
	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return 0, err
	}
	count := 0
	for _, item := range list {
		if schema, ok := item.Data.(*EntityType); ok {
			if schema.Category == category {
				count++
			}
		}
	}
	return count, nil
}

// GetEntityTypeListByOUIDs implements entityTypeStoreInterface.
func (f *entityTypeFileBasedStore) GetEntityTypeListByOUIDs(
	ctx context.Context, category TypeCategory, ouIDs []string, limit, offset int,
) ([]EntityTypeListItem, error) {
	ouIDSet := make(map[string]struct{}, len(ouIDs))
	for _, id := range ouIDs {
		ouIDSet[id] = struct{}{}
	}

	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return nil, err
	}

	var filtered []EntityTypeListItem
	for _, item := range list {
		if schema, ok := item.Data.(*EntityType); ok {
			if schema.Category != category {
				continue
			}
			if _, exists := ouIDSet[schema.OUID]; exists {
				filtered = append(filtered, EntityTypeListItem{
					ID:                    schema.ID,
					Category:              schema.Category,
					Name:                  schema.Name,
					OUID:                  schema.OUID,
					AllowSelfRegistration: schema.AllowSelfRegistration,
					SystemAttributes:      schema.SystemAttributes,
				})
			}
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Name < filtered[j].Name
	})

	start := offset
	end := offset + limit
	if start > len(filtered) {
		return []EntityTypeListItem{}, nil
	}
	if end > len(filtered) {
		end = len(filtered)
	}

	return filtered[start:end], nil
}

// GetEntityTypeListCountByOUIDs implements entityTypeStoreInterface.
func (f *entityTypeFileBasedStore) GetEntityTypeListCountByOUIDs(ctx context.Context,
	category TypeCategory, ouIDs []string) (int, error) {
	ouIDSet := make(map[string]struct{}, len(ouIDs))
	for _, id := range ouIDs {
		ouIDSet[id] = struct{}{}
	}

	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return 0, err
	}

	count := 0
	for _, item := range list {
		if schema, ok := item.Data.(*EntityType); ok {
			if schema.Category != category {
				continue
			}
			if _, exists := ouIDSet[schema.OUID]; exists {
				count++
			}
		}
	}

	return count, nil
}

// UpdateEntityTypeByID implements entityTypeStoreInterface.
func (f *entityTypeFileBasedStore) UpdateEntityTypeByID(ctx context.Context, category TypeCategory,
	schemaID string, schema EntityType) error {
	return errors.New("UpdateEntityTypeByID is not supported in file-based store")
}

// IsEntityTypeDeclarative returns true if the given schema id is present in the file-based
// store under the given category.
func (f *entityTypeFileBasedStore) IsEntityTypeDeclarative(category TypeCategory, schemaID string) bool {
	data, err := f.GenericFileBasedStore.Get(schemaID)
	if err != nil {
		return false
	}
	schema, ok := data.(*EntityType)
	if !ok {
		return false
	}
	return schema.Category == category
}

// GetDisplayAttributesByNames retrieves display attributes for a list of entity type names within a category.
func (f *entityTypeFileBasedStore) GetDisplayAttributesByNames(
	ctx context.Context, category TypeCategory, names []string,
) (map[string]string, error) {
	if len(names) == 0 {
		return map[string]string{}, nil
	}

	nameSet := make(map[string]struct{}, len(names))
	for _, name := range names {
		nameSet[name] = struct{}{}
	}

	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return nil, err
	}

	displayAttrs := make(map[string]string, len(names))
	for _, item := range list {
		if schema, ok := item.Data.(*EntityType); ok {
			if schema.Category != category {
				continue
			}
			if _, exists := nameSet[schema.Name]; exists {
				if schema.SystemAttributes != nil {
					displayAttrs[schema.Name] = schema.SystemAttributes.Display
				} else {
					displayAttrs[schema.Name] = ""
				}
			}
		}
	}

	return displayAttrs, nil
}

// newEntityTypeFileBasedStore creates a new instance of a file-based store.
func newEntityTypeFileBasedStore() (entityTypeStoreInterface, transaction.Transactioner) {
	genericStore := declarativeresource.NewGenericFileBasedStore(entity.KeyTypeEntityType)
	return &entityTypeFileBasedStore{
		GenericFileBasedStore: genericStore,
	}, transaction.NewNoOpTransactioner()
}
