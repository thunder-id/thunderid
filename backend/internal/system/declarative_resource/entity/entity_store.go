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

// Package entity provides generic entity storage functionality.
package entity

import (
	"fmt"
	"sort"
	"sync"
)

// KeyType represents the allowed types for entities in the store.
type KeyType string

// Predefined key types for common entities
const (
	KeyTypeApplication             KeyType = "application"
	KeyTypeNotification            KeyType = "notification"
	KeyTypeIDP                     KeyType = "idp"
	KeyTypeNotificationSender      KeyType = "notification-sender"
	KeyTypeEntityType              KeyType = "user-type"
	KeyTypeOU                      KeyType = "ou"
	KeyTypeFlow                    KeyType = "flow"
	KeyTypeTranslation             KeyType = "translation"
	KeyTypeTheme                   KeyType = "theme"
	KeyTypeLayout                  KeyType = "layout"
	KeyTypeResourceServer          KeyType = "resource-server"
	KeyTypeResource                KeyType = "resource"
	KeyTypeAction                  KeyType = "action"
	KeyTypeRole                    KeyType = "role"
	KeyTypeUser                    KeyType = "user"
	KeyTypeTemplate                KeyType = "template"
	KeyTypeEntity                  KeyType = "entity"
	KeyTypeInboundAuth             KeyType = "inbound-auth"
	KeyTypeGroup                   KeyType = "group"
	KeyTypePresentationDefinition  KeyType = "presentation-definition"
	KeyTypeCredentialConfiguration KeyType = "credential-configuration" //nolint:gosec
	KeyTypeServerConfig            KeyType = "server-config"
)

// String returns the string representation of KeyType
func (kt KeyType) String() string {
	return string(kt)
}

// IsValid checks if the KeyType is one of the predefined types
func (kt KeyType) IsValid() bool {
	switch kt {
	case KeyTypeApplication, KeyTypeNotification, KeyTypeIDP, KeyTypeNotificationSender,
		KeyTypeEntityType, KeyTypeOU, KeyTypeFlow, KeyTypeTranslation, KeyTypeTheme, KeyTypeLayout,
		KeyTypeResourceServer, KeyTypeResource, KeyTypeAction, KeyTypeRole, KeyTypeUser, KeyTypeTemplate,
		KeyTypeInboundAuth, KeyTypeGroup, KeyTypePresentationDefinition, KeyTypeCredentialConfiguration,
		KeyTypeServerConfig,
		KeyTypeEntity:
		return true
	default:
		return false
	}
}

// CompositeKey represents a key made of ID and Type values.
type CompositeKey struct {
	ID   string  `json:"id"`
	Type KeyType `json:"type"`
}

// String returns a string representation of the composite key.
func (ck CompositeKey) String() string {
	return fmt.Sprintf("%s:%s", ck.Type, ck.ID)
}

// NewCompositeKey creates a new composite key with validation.
func NewCompositeKey(id string, keyType KeyType) CompositeKey {
	return CompositeKey{
		ID:   id,
		Type: keyType,
	}
}

// NewCompositeKeyFromString creates a composite key from string type (for backward compatibility).
func NewCompositeKeyFromString(id, keyTypeStr string) CompositeKey {
	return CompositeKey{
		ID:   id,
		Type: KeyType(keyTypeStr),
	}
}

// Entity represents a stored entity with its metadata.
type Entity struct {
	ID   CompositeKey `json:"id"`
	Data interface{}  `json:"data"`
}

// StoreInterface defines the interface for the key-value store operations.
type StoreInterface interface {
	// Get retrieves an entity by its composite key
	Get(key CompositeKey) (*Entity, error)

	// Set stores an entity with the given composite key
	Set(key CompositeKey, data interface{}) error

	// Delete removes an entity by its composite key
	Delete(key CompositeKey) error

	// List returns all entities in the store
	List() ([]*Entity, error)

	// ListByID returns all entities with the specified ID
	ListByID(id string) ([]*Entity, error)

	// ListByType returns all entities with the specified type
	ListByType(keyType KeyType) ([]*Entity, error)

	// CountByType returns the number of entities with the specified type
	CountByType(keyType KeyType) (int, error)

	// Clear removes all entities from the store
	Clear() error
}

// Store is the concrete implementation of StoreInterface.
type Store struct {
	mu       sync.RWMutex
	entities map[CompositeKey]*Entity
}

var (
	instance *Store
	once     sync.Once
)

// GetInstance returns the singleton instance of the store.
func GetInstance() StoreInterface {
	once.Do(func() {
		instance = &Store{
			entities: make(map[CompositeKey]*Entity),
		}
	})
	return instance
}

// NewStore creates a new instance of the store (for testing purposes).
// For production use, use GetInstance() to get the singleton instance.
func NewStore() StoreInterface {
	return &Store{
		entities: make(map[CompositeKey]*Entity),
	}
}

// Get retrieves an entity by its composite key.
func (s *Store) Get(key CompositeKey) (*Entity, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entity, exists := s.entities[key]
	if !exists {
		return nil, fmt.Errorf("entity with key '%s' not found", key.String())
	}

	return entity, nil
}

// Set stores an entity with the given composite key.
func (s *Store) Set(key CompositeKey, data interface{}) error {
	if key.ID == "" {
		return fmt.Errorf("key ID cannot be empty")
	}
	if !key.Type.IsValid() {
		return fmt.Errorf("invalid key type: %s", key.Type)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.entities[key] = &Entity{
		ID:   key,
		Data: data,
	}

	return nil
}

// Delete removes an entity by its composite key.
func (s *Store) Delete(key CompositeKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.entities[key]; !exists {
		return fmt.Errorf("entity with key '%s' not found", key.String())
	}

	delete(s.entities, key)
	return nil
}

// List returns all entities in the store.
func (s *Store) List() ([]*Entity, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entities := make([]*Entity, 0, len(s.entities))
	for _, entity := range s.entities {
		entities = append(entities, entity)
	}

	return entities, nil
}

// ListByID returns all entities with the specified ID.
func (s *Store) ListByID(id string) ([]*Entity, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var entities []*Entity
	for key, entity := range s.entities {
		if key.ID == id {
			entities = append(entities, entity)
		}
	}

	return entities, nil
}

// ListByType returns all entities with the specified type.
func (s *Store) ListByType(keyType KeyType) ([]*Entity, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var entities []*Entity
	for key, entity := range s.entities {
		if key.Type == keyType {
			entities = append(entities, entity)
		}
	}

	sort.Slice(entities, func(i, j int) bool {
		return entities[i].ID.ID < entities[j].ID.ID
	})

	return entities, nil
}

// CountByType returns the number of entities with the specified type.
func (s *Store) CountByType(keyType KeyType) (int, error) {
	entities, err := s.ListByType(keyType)
	if err != nil {
		return 0, err
	}
	return len(entities), nil
}

// Clear removes all entities from the store.
func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.entities = make(map[CompositeKey]*Entity)
	return nil
}
