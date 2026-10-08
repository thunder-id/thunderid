// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entitytype

import (
	"context"
	"errors"
	"sort"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
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

// GetEntityTypeByHandle implements entityTypeStoreInterface.
func (f *entityTypeFileBasedStore) GetEntityTypeByHandle(ctx context.Context, category TypeCategory,
	handle string) (EntityType, error) {
	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return EntityType{}, ErrEntityTypeNotFound
	}
	for _, item := range list {
		if schema, ok := item.Data.(*EntityType); ok {
			if schema.Handle == handle && schema.Category == category {
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
				Handle:                schema.Handle,
				DisplayName:           schema.DisplayName,
				OUID:                  schema.OUID,
				AllowSelfRegistration: schema.AllowSelfRegistration,
				SystemAttributes:      schema.SystemAttributes,
			})
		}
	}

	sort.Slice(schemaList, func(i, j int) bool {
		if schemaList[i].DisplayName == schemaList[j].DisplayName {
			return schemaList[i].Handle < schemaList[j].Handle
		}
		return schemaList[i].DisplayName < schemaList[j].DisplayName
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
					Handle:                schema.Handle,
					DisplayName:           schema.DisplayName,
					OUID:                  schema.OUID,
					AllowSelfRegistration: schema.AllowSelfRegistration,
					SystemAttributes:      schema.SystemAttributes,
				})
			}
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].DisplayName == filtered[j].DisplayName {
			return filtered[i].Handle < filtered[j].Handle
		}
		return filtered[i].DisplayName < filtered[j].DisplayName
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

// GetDisplayAttributesByHandles retrieves display attributes for a list of entity type handles within a category.
func (f *entityTypeFileBasedStore) GetDisplayAttributesByHandles(
	ctx context.Context, category TypeCategory, handles []string,
) (map[string]string, error) {
	if len(handles) == 0 {
		return map[string]string{}, nil
	}

	handleSet := make(map[string]struct{}, len(handles))
	for _, handle := range handles {
		handleSet[handle] = struct{}{}
	}

	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return nil, err
	}

	displayAttrs := make(map[string]string, len(handles))
	for _, item := range list {
		if schema, ok := item.Data.(*EntityType); ok {
			if schema.Category != category {
				continue
			}
			if _, exists := handleSet[schema.Handle]; exists {
				if schema.SystemAttributes != nil {
					displayAttrs[schema.Handle] = schema.SystemAttributes.Display
				} else {
					displayAttrs[schema.Handle] = ""
				}
			}
		}
	}

	return displayAttrs, nil
}

// newEntityTypeFileBasedStore creates a new instance of a file-based store.
func newEntityTypeFileBasedStore() (entityTypeStoreInterface, providers.Transactioner) {
	genericStore := declarativeresource.NewGenericFileBasedStore(entity.KeyTypeEntityType)
	return &entityTypeFileBasedStore{
		GenericFileBasedStore: genericStore,
	}, transaction.NewNoOpTransactioner()
}
