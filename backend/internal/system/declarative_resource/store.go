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

package declarativeresource

import (
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

// Get retrieves an entity by its ID.
func (s *GenericFileBasedStore) Get(id string) (interface{}, error) {
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
	fieldValue string, fieldGetter func(interface{}) string,
) (interface{}, error) {
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

// List retrieves all entities of this type.
func (s *GenericFileBasedStore) List() ([]*entity.Entity, error) {
	return s.storage.ListByType(s.keyType)
}

// Count returns the count of entities of this type.
func (s *GenericFileBasedStore) Count() (int, error) {
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
	list, err := s.List()
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
	log.GetLogger().Error("Type assertion failed while retrieving resource",
		log.String("resourceType", resourceType),
		log.String("id", id))
}
