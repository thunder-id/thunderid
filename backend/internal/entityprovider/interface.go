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

package entityprovider

import (
	"encoding/json"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// EntityProviderInterface defines the boundary contract between the gateway layer and the
// directory layer for entity operations.
type EntityProviderInterface interface {
	// IdentifyEntity resolves an entity ID from indexed attribute filters (e.g., email, clientId).
	IdentifyEntity(filters map[string]interface{}) (*string, *EntityProviderError)

	// SearchEntities searches for all entities matching the given filters.
	SearchEntities(filters map[string]interface{}) ([]*providers.Entity, *EntityProviderError)

	// GetEntity retrieves an entity by ID. Credentials are never returned.
	GetEntity(entityID string) (*providers.Entity, *EntityProviderError)

	// CreateEntity creates a new entity.
	CreateEntity(entity *providers.Entity,
		systemCredentials json.RawMessage) (*providers.Entity, *EntityProviderError)

	// UpdateEntity updates an existing entity's core fields.
	UpdateEntity(entityID string, entity *providers.Entity) (*providers.Entity, *EntityProviderError)

	// DeleteEntity deletes an entity by ID. Cascades to identifiers.
	DeleteEntity(entityID string) *EntityProviderError

	// UpdateCredentials updates schema-defined credentials for an entity.
	UpdateCredentials(entityID string,
		credentials json.RawMessage) *EntityProviderError

	// UpdateAttributes updates schema-defined attributes for an entity.
	UpdateAttributes(entityID string,
		attributes json.RawMessage) *EntityProviderError

	// UpdateSystemAttributes updates system-managed attributes for an entity.
	UpdateSystemAttributes(entityID string,
		attributes json.RawMessage) *EntityProviderError

	// UpdateSystemCredentials updates system-managed credentials for an entity.
	UpdateSystemCredentials(entityID string,
		credentials json.RawMessage) *EntityProviderError

	// GetTransitiveEntityGroups retrieves all groups an entity belongs to, including inherited groups.
	GetTransitiveEntityGroups(entityID string) ([]providers.EntityGroup, *EntityProviderError)

	// ValidateEntityIDs validates that the given entity IDs exist. Returns IDs that are invalid.
	ValidateEntityIDs(entityIDs []string) ([]string, *EntityProviderError)

	// GetEntitiesByIDs retrieves multiple entities by their IDs.
	GetEntitiesByIDs(entityIDs []string) ([]providers.Entity, *EntityProviderError)

	// GetEntityListCount returns the total number of entities in the given category.
	GetEntityListCount(category providers.EntityCategory, filters map[string]interface{}) (int, *EntityProviderError)

	// GetEntityList returns a page of entities in the given category.
	GetEntityList(category providers.EntityCategory, limit, offset int,
		filters map[string]interface{}) ([]providers.Entity, *EntityProviderError)
}
