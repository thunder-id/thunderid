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
	"strings"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	entitystore "github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
)

// entityFileBasedStore implements entityStoreInterface using an in-memory file-based store.
type entityFileBasedStore struct {
	*declarativeresource.GenericFileBasedStore
}

// newEntityFileBasedStore creates a new instance of a file-based entity store.
func newEntityFileBasedStore() *entityFileBasedStore {
	genericStore := declarativeresource.NewGenericFileBasedStore(entitystore.KeyTypeEntity)
	return &entityFileBasedStore{
		GenericFileBasedStore: genericStore,
	}
}

// Create implements declarativeresource.Storer interface for resource loader.
func (f *entityFileBasedStore) Create(id string, data interface{}) error {
	resource, ok := data.(*entityStoreEntry)
	if !ok {
		declarativeresource.LogTypeAssertionError("entity", id)
		return errors.New("invalid data type: expected *entityStoreEntry")
	}
	return f.GenericFileBasedStore.Create(id, resource)
}

// CreateEntity implements entityStoreInterface.
func (f *entityFileBasedStore) CreateEntity(ctx context.Context, entity Entity,
	credentials json.RawMessage, systemCredentials json.RawMessage) error {
	resource := &entityStoreEntry{
		Entity:            entity,
		Credentials:       credentials,
		SystemCredentials: systemCredentials,
	}
	return f.GenericFileBasedStore.Create(entity.ID, resource)
}

// GetEntity retrieves an entity by ID.
func (f *entityFileBasedStore) GetEntity(ctx context.Context, id string) (Entity, error) {
	data, err := f.GenericFileBasedStore.Get(id)
	if err != nil {
		return Entity{}, ErrEntityNotFound
	}
	resource, ok := data.(*entityStoreEntry)
	if !ok {
		declarativeresource.LogTypeAssertionError("entity", id)
		return Entity{}, errors.New("entity data corrupted")
	}
	return resource.Entity, nil
}

// GetEntityWithCredentials retrieves an entity with credentials from the file store.
func (f *entityFileBasedStore) GetEntityWithCredentials(ctx context.Context, id string) (
	*entityWithCredentials, error) {
	data, err := f.GenericFileBasedStore.Get(id)
	if err != nil {
		return nil, ErrEntityNotFound
	}
	resource, ok := data.(*entityStoreEntry)
	if !ok {
		declarativeresource.LogTypeAssertionError("entity", id)
		return nil, errors.New("entity data corrupted")
	}
	return &entityWithCredentials{
		Entity:            &resource.Entity,
		SchemaCredentials: resource.Credentials,
		SystemCredentials: resource.SystemCredentials,
	}, nil
}

// UpdateEntity is not supported in file-based store.
func (f *entityFileBasedStore) UpdateEntity(ctx context.Context, entity *Entity) error {
	return errors.New("UpdateEntity is not supported in file-based store")
}

// UpdateAttributes is not supported in file-based store.
func (f *entityFileBasedStore) UpdateAttributes(
	ctx context.Context, entityID string, attributes json.RawMessage) error {
	return errors.New("UpdateAttributes is not supported in file-based store")
}

// UpdateSystemAttributes is not supported in file-based store.
func (f *entityFileBasedStore) UpdateSystemAttributes(ctx context.Context, entityID string,
	attrs json.RawMessage) error {
	return errors.New("UpdateSystemAttributes is not supported in file-based store")
}

// UpdateCredentials is not supported in file-based store.
func (f *entityFileBasedStore) UpdateCredentials(ctx context.Context, entityID string,
	creds json.RawMessage) error {
	return errors.New("UpdateCredentials is not supported in file-based store")
}

// UpdateSystemCredentials is not supported in file-based store.
func (f *entityFileBasedStore) UpdateSystemCredentials(ctx context.Context, entityID string,
	creds json.RawMessage) error {
	return errors.New("UpdateSystemCredentials is not supported in file-based store")
}

// DeleteEntity is not supported in file-based store.
func (f *entityFileBasedStore) DeleteEntity(ctx context.Context, id string) error {
	return errors.New("DeleteEntity is not supported in file-based store")
}

// IdentifyEntity identifies an entity with the given filters by linear search.
func (f *entityFileBasedStore) IdentifyEntity(ctx context.Context,
	filters map[string]interface{}) (*string, error) {
	resources, err := f.listEntityResources()
	if err != nil {
		return nil, err
	}

	var matches []string
	for _, resource := range resources {
		combined := mergeJSONObjects(resource.Entity.Attributes, resource.Entity.SystemAttributes)
		if matchesFilters(combined, filters) {
			matches = append(matches, resource.Entity.ID)
		}
	}

	if len(matches) == 0 {
		return nil, ErrEntityNotFound
	}
	if len(matches) > 1 {
		return nil, ErrAmbiguousEntity
	}

	return &matches[0], nil
}

// SearchEntities searches for all entities matching the provided filters from the file store.
func (f *entityFileBasedStore) SearchEntities(ctx context.Context,
	filters map[string]interface{}) ([]Entity, error) {
	resources, err := f.listEntityResources()
	if err != nil {
		return nil, err
	}

	var matched []Entity
	for _, resource := range resources {
		combined := mergeJSONObjects(resource.Entity.Attributes, resource.Entity.SystemAttributes)
		if matchesFilters(combined, filters) {
			matched = append(matched, resource.Entity)
		}
	}

	if len(matched) == 0 {
		return nil, ErrEntityNotFound
	}

	return matched, nil
}

// GetEntityListCount retrieves the total count of entities from the file store.
func (f *entityFileBasedStore) GetEntityListCount(ctx context.Context, category string,
	filters map[string]interface{}) (int, error) {
	resources, err := f.listEntityResources()
	if err != nil {
		return 0, err
	}

	count := 0
	for _, resource := range resources {
		if string(resource.Entity.Category) != category {
			continue
		}
		combined := mergeJSONObjects(resource.Entity.Attributes, resource.Entity.SystemAttributes)
		if matchesFilters(combined, filters) {
			count++
		}
	}
	return count, nil
}

// GetEntityList retrieves entities from the file store with pagination and filtering.
func (f *entityFileBasedStore) GetEntityList(ctx context.Context, category string,
	limit, offset int, filters map[string]interface{}) ([]Entity, error) {
	resources, err := f.listEntityResources()
	if err != nil {
		return nil, err
	}

	entities := make([]Entity, 0)
	for _, resource := range resources {
		if string(resource.Entity.Category) != category {
			continue
		}
		combined := mergeJSONObjects(resource.Entity.Attributes, resource.Entity.SystemAttributes)
		if matchesFilters(combined, filters) {
			entities = append(entities, resource.Entity)
		}
	}

	return applyPagination(entities, limit, offset), nil
}

// GetEntityListCountByOUIDs retrieves the total count of entities by OU IDs.
func (f *entityFileBasedStore) GetEntityListCountByOUIDs(ctx context.Context, category string,
	ouIDs []string, filters map[string]interface{}) (int, error) {
	resources, err := f.listEntityResources()
	if err != nil {
		return 0, err
	}

	ouIDSet := make(map[string]struct{}, len(ouIDs))
	for _, id := range ouIDs {
		ouIDSet[id] = struct{}{}
	}

	count := 0
	for _, resource := range resources {
		if string(resource.Entity.Category) != category {
			continue
		}
		if _, ok := ouIDSet[resource.Entity.OUID]; !ok {
			continue
		}
		combined := mergeJSONObjects(resource.Entity.Attributes, resource.Entity.SystemAttributes)
		if matchesFilters(combined, filters) {
			count++
		}
	}
	return count, nil
}

// GetEntityListByOUIDs retrieves entities scoped to OU IDs with pagination and filtering.
func (f *entityFileBasedStore) GetEntityListByOUIDs(ctx context.Context, category string,
	ouIDs []string, limit, offset int, filters map[string]interface{}) ([]Entity, error) {
	resources, err := f.listEntityResources()
	if err != nil {
		return nil, err
	}

	ouIDSet := make(map[string]struct{}, len(ouIDs))
	for _, id := range ouIDs {
		ouIDSet[id] = struct{}{}
	}

	entities := make([]Entity, 0)
	for _, resource := range resources {
		if string(resource.Entity.Category) != category {
			continue
		}
		if _, ok := ouIDSet[resource.Entity.OUID]; !ok {
			continue
		}
		combined := mergeJSONObjects(resource.Entity.Attributes, resource.Entity.SystemAttributes)
		if matchesFilters(combined, filters) {
			entities = append(entities, resource.Entity)
		}
	}

	return applyPagination(entities, limit, offset), nil
}

// GetGroupCountForEntity returns 0 for file-based store (groups are for mutable entities only).
func (f *entityFileBasedStore) GetGroupCountForEntity(ctx context.Context, entityID string) (int, error) {
	return 0, nil
}

// GetEntityGroups returns empty for file-based store (groups are for mutable entities only).
func (f *entityFileBasedStore) GetEntityGroups(ctx context.Context, entityID string,
	limit, offset int) ([]EntityGroup, error) {
	return []EntityGroup{}, nil
}

// GetTransitiveEntityGroups returns empty for file-based store (groups are for mutable entities only).
func (f *entityFileBasedStore) GetTransitiveEntityGroups(ctx context.Context, entityID string) ([]EntityGroup, error) {
	return []EntityGroup{}, nil
}

// ValidateEntityIDs checks if all provided entity IDs exist.
func (f *entityFileBasedStore) ValidateEntityIDs(ctx context.Context, entityIDs []string) ([]string, error) {
	invalid := make([]string, 0)
	for _, id := range entityIDs {
		_, err := f.GetEntity(ctx, id)
		if err != nil {
			if errors.Is(err, ErrEntityNotFound) {
				invalid = append(invalid, id)
				continue
			}
			return nil, err
		}
	}
	return invalid, nil
}

// GetEntitiesByIDs retrieves entities by a list of IDs from the file store.
func (f *entityFileBasedStore) GetEntitiesByIDs(ctx context.Context, entityIDs []string) ([]Entity, error) {
	if len(entityIDs) == 0 {
		return []Entity{}, nil
	}

	entities := make([]Entity, 0, len(entityIDs))
	for _, id := range entityIDs {
		entity, err := f.GetEntity(ctx, id)
		if err != nil {
			if errors.Is(err, ErrEntityNotFound) {
				continue
			}
			return nil, err
		}
		entities = append(entities, entity)
	}

	return entities, nil
}

// ValidateEntityIDsInOUs checks which of the provided entity IDs belong to the given OU scope.
func (f *entityFileBasedStore) ValidateEntityIDsInOUs(
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
		e, err := f.GetEntity(ctx, id)
		if err != nil {
			if errors.Is(err, ErrEntityNotFound) {
				outOfScope = append(outOfScope, id)
				continue
			}
			return nil, err
		}
		if !ouSet[e.OUID] {
			outOfScope = append(outOfScope, id)
		}
	}
	return outOfScope, nil
}

// IsEntityDeclarative checks if an entity exists in the file store (all file entities are declarative).
func (f *entityFileBasedStore) IsEntityDeclarative(ctx context.Context, id string) (bool, error) {
	_, err := f.GetEntity(ctx, id)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, ErrEntityNotFound) {
		return false, nil
	}
	return false, err
}

// GetIndexedAttributes returns nil for file-based store (no indexed attributes).
func (f *entityFileBasedStore) GetIndexedAttributes() map[string]bool {
	return nil
}

// LoadIndexedAttributes is a no-op for the file-based store.
func (f *entityFileBasedStore) LoadIndexedAttributes(_ []string) error {
	return nil
}

// listEntityResources lists all entity resources from the in-memory store.
func (f *entityFileBasedStore) listEntityResources() ([]*entityStoreEntry, error) {
	list, err := f.GenericFileBasedStore.List()
	if err != nil {
		return nil, err
	}

	resources := make([]*entityStoreEntry, 0, len(list))
	for _, item := range list {
		resource, ok := item.Data.(*entityStoreEntry)
		if !ok {
			declarativeresource.LogTypeAssertionError("entity", item.ID.ID)
			return nil, errors.New("entity data corrupted")
		}
		resources = append(resources, resource)
	}

	return resources, nil
}

func applyPagination(entities []Entity, limit, offset int) []Entity {
	if limit < 0 {
		return []Entity{}
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= len(entities) {
		return []Entity{}
	}

	end := offset + limit
	if limit == 0 {
		end = len(entities)
	}
	if end > len(entities) {
		end = len(entities)
	}

	return entities[offset:end]
}

// mergeJSONObjects merges two JSON objects into one. Keys in override take precedence over base.
func mergeJSONObjects(base, override json.RawMessage) json.RawMessage {
	if len(base) == 0 {
		return override
	}
	if len(override) == 0 {
		return base
	}
	var baseMap, overrideMap map[string]interface{}
	if err := json.Unmarshal(base, &baseMap); err != nil {
		return base
	}
	if err := json.Unmarshal(override, &overrideMap); err != nil {
		return base
	}
	for k, v := range overrideMap {
		baseMap[k] = v
	}
	merged, err := json.Marshal(baseMap)
	if err != nil {
		return base
	}
	return merged
}

func matchesFilters(attributes json.RawMessage, filters map[string]interface{}) bool {
	if len(filters) == 0 {
		return true
	}
	if len(attributes) == 0 {
		return false
	}

	var attrsMap map[string]interface{}
	if err := json.Unmarshal(attributes, &attrsMap); err != nil {
		return false
	}

	for key, expected := range filters {
		value, ok := getNestedValue(attrsMap, key)
		if !ok || !valuesEqual(value, expected) {
			return false
		}
	}

	return true
}

func getNestedValue(data map[string]interface{}, key string) (interface{}, bool) {
	parts := strings.Split(key, ".")
	current := interface{}(data)

	for _, part := range parts {
		obj, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		value, exists := obj[part]
		if !exists {
			return nil, false
		}
		current = value
	}

	return current, true
}

func valuesEqual(actual interface{}, expected interface{}) bool {
	switch actualValue := actual.(type) {
	case float64:
		switch expectedValue := expected.(type) {
		case int64:
			return actualValue == float64(expectedValue)
		case float64:
			return actualValue == expectedValue
		case int:
			return actualValue == float64(expectedValue)
		}
	case string:
		if expectedValue, ok := expected.(string); ok {
			return actualValue == expectedValue
		}
	case bool:
		if expectedValue, ok := expected.(bool); ok {
			return actualValue == expectedValue
		}
	}

	return false
}
