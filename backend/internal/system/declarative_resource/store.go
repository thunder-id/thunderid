// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package declarativeresource

import (
	"context"
	"errors"

	"github.com/thunder-id/thunderid/internal/system/declarative_resource/entity"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// GenericFileBasedStore provides a generic implementation for file-based stores.
type GenericFileBasedStore struct {
	storage entity.StoreInterface
	keyType entity.KeyType
}

// NewGenericFileBasedStore creates a new generic file-based store using the singleton instance.
func NewGenericFileBasedStore(keyType entity.KeyType) *GenericFileBasedStore {
	return &GenericFileBasedStore{
		storage: entity.GetInstance(),
		keyType: keyType,
	}
}

// NewGenericFileBasedStoreForTest creates a new generic file-based store with its own storage instance (for testing).
func NewGenericFileBasedStoreForTest(keyType entity.KeyType) *GenericFileBasedStore {
	return &GenericFileBasedStore{
		storage: entity.NewStore(),
		keyType: keyType,
	}
}

// Create stores an entity with the given ID and data.
func (s *GenericFileBasedStore) Create(id string, data interface{}) error {
	key := entity.NewCompositeKey(id, s.keyType)
	return s.storage.Set(key, data)
}

// Get retrieves an entity by its ID, when declarative resources are readable in this deployment.
func (s *GenericFileBasedStore) Get(ctx context.Context, id string) (interface{}, error) {
	if !VisibleTo(ctx) {
		return nil, errors.New("entity not found")
	}
	key := entity.NewCompositeKey(id, s.keyType)
	e, err := s.storage.Get(key)
	if err != nil {
		return nil, err
	}
	return e.Data, nil
}

// GetForLoad retrieves an entity without regard to who may see it.
//
// It exists for loading, where the file is being parsed and there is no request and no deployment to
// scope by. The uniqueness checks that run then must see every declarative resource, not the subset
// some tenant would be shown.
func (s *GenericFileBasedStore) GetForLoad(id string) (interface{}, error) {
	key := entity.NewCompositeKey(id, s.keyType)
	e, err := s.storage.Get(key)
	if err != nil {
		return nil, err
	}
	return e.Data, nil
}

// GetByField retrieves an entity by searching for a matching field value.
// The fieldGetter function extracts the field value from each entity.
func (s *GenericFileBasedStore) GetByField(
	ctx context.Context, fieldValue string, fieldGetter func(interface{}) string,
) (interface{}, error) {
	if !VisibleTo(ctx) {
		return nil, errors.New("entity not found")
	}
	list, err := s.storage.ListByType(s.keyType)
	if err != nil {
		return nil, err
	}

	for _, item := range list {
		if fieldGetter(item.Data) == fieldValue {
			return item.Data, nil
		}
	}

	return nil, errors.New("entity not found")
}

// List retrieves all entities of this type, when declarative resources are readable in this
// deployment. A deployment they are not readable in sees none rather than an error, so a listing
// simply does not include them.
func (s *GenericFileBasedStore) List(ctx context.Context) ([]*entity.Entity, error) {
	if !VisibleTo(ctx) {
		return nil, nil
	}
	return s.storage.ListByType(s.keyType)
}

// Count returns the count of entities of this type, when they are readable in this deployment.
func (s *GenericFileBasedStore) Count(ctx context.Context) (int, error) {
	if !VisibleTo(ctx) {
		return 0, nil
	}
	return s.storage.CountByType(s.keyType)
}

// Update is not supported in file-based store.
func (s *GenericFileBasedStore) Update(id string, data interface{}) error {
	return errors.New("update operation not supported in file-based store")
}

// Delete is not supported in file-based store.
func (s *GenericFileBasedStore) Delete(id string) error {
	return errors.New("delete operation not supported in file-based store")
}

// ClearByType removes all entities of this specific key type (primarily for testing).
func (s *GenericFileBasedStore) ClearByType() error {
	// Reads the storage directly rather than through List: clearing is maintenance, not a read on
	// behalf of a deployment, so it is not subject to who may see these resources.
	list, err := s.storage.ListByType(s.keyType)
	if err != nil {
		return err
	}
	for _, item := range list {
		err := s.storage.Delete(item.ID)
		if err != nil {
			return err
		}
	}
	return nil
}

// LogTypeAssertionError logs a type assertion error.
func LogTypeAssertionError(resourceType, id string) {
	// Declarative resource ID extraction runs during startup loading, outside any request.
	log.GetLogger().Error(context.Background(), "Type assertion failed while retrieving resource",
		log.String("resourceType", resourceType),
		log.String("id", id))
}
