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
	"context"
	"encoding/json"
	"errors"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/system/security"
)

type defaultEntityProvider struct {
	entitySvc entity.EntityServiceInterface
}

// newDefaultEntityProvider creates a new default entity provider.
func newDefaultEntityProvider(
	entitySvc entity.EntityServiceInterface,
) EntityProviderInterface {
	return &defaultEntityProvider{
		entitySvc: entitySvc,
	}
}

// IdentifyEntity resolves an entity ID from indexed attribute filters.
func (p *defaultEntityProvider) IdentifyEntity(
	filters map[string]interface{},
) (*string, *EntityProviderError) {
	ctx := security.WithRuntimeContext(context.Background())
	entityID, err := p.entitySvc.IdentifyEntity(ctx, filters)
	if err != nil {
		return nil, mapEntityError(err)
	}
	return entityID, nil
}

// SearchEntities searches for all entities matching the given filters.
// OUHandle is not resolved here — callers that need it (e.g. disambiguation flows)
// resolve it on demand via the OU service.
func (p *defaultEntityProvider) SearchEntities(
	filters map[string]interface{},
) ([]*Entity, *EntityProviderError) {
	ctx := security.WithRuntimeContext(context.Background())
	entities, err := p.entitySvc.SearchEntities(ctx, filters)
	if err != nil {
		return nil, mapEntityError(err)
	}
	result := make([]*Entity, 0, len(entities))
	for i := range entities {
		result = append(result, toProviderEntity(&entities[i]))
	}
	return result, nil
}

// GetEntity retrieves an entity by ID.
func (p *defaultEntityProvider) GetEntity(
	entityID string,
) (*Entity, *EntityProviderError) {
	ctx := security.WithRuntimeContext(context.Background())
	result, err := p.entitySvc.GetEntity(ctx, entityID)
	if err != nil {
		return nil, mapEntityError(err)
	}
	return toProviderEntity(result), nil
}

// CreateEntity creates a new entity.
func (p *defaultEntityProvider) CreateEntity(
	e *Entity, systemCredentials json.RawMessage,
) (*Entity, *EntityProviderError) {
	if e == nil {
		return nil, NewEntityProviderError(ErrorCodeInvalidRequestFormat, "Invalid request",
			"Entity cannot be nil")
	}
	ctx := security.WithRuntimeContext(context.Background())
	svcEntity := toServiceEntity(e)
	result, err := p.entitySvc.CreateEntity(ctx, svcEntity, systemCredentials)
	if err != nil {
		return nil, mapEntityError(err)
	}
	return toProviderEntity(result), nil
}

// UpdateEntity updates an existing entity.
func (p *defaultEntityProvider) UpdateEntity(
	entityID string, e *Entity,
) (*Entity, *EntityProviderError) {
	if e == nil {
		return nil, NewEntityProviderError(ErrorCodeInvalidRequestFormat, "Invalid request",
			"Entity cannot be nil")
	}
	ctx := security.WithRuntimeContext(context.Background())
	svcEntity := toServiceEntity(e)
	result, err := p.entitySvc.UpdateEntity(ctx, entityID, svcEntity)
	if err != nil {
		return nil, mapEntityError(err)
	}
	return toProviderEntity(result), nil
}

// DeleteEntity deletes an entity by ID.
func (p *defaultEntityProvider) DeleteEntity(
	entityID string,
) *EntityProviderError {
	ctx := security.WithRuntimeContext(context.Background())
	err := p.entitySvc.DeleteEntity(ctx, entityID)
	if err != nil {
		if errors.Is(err, entity.ErrEntityNotFound) {
			return nil
		}
		return mapEntityError(err)
	}
	return nil
}

// UpdateCredentials updates schema-defined credentials for an entity.
func (p *defaultEntityProvider) UpdateCredentials(
	entityID string, credentials json.RawMessage,
) *EntityProviderError {
	ctx := security.WithRuntimeContext(context.Background())
	err := p.entitySvc.UpdateCredentials(ctx, entityID, credentials)
	if err != nil {
		return mapEntityError(err)
	}
	return nil
}

// UpdateAttributes updates schema-defined attributes for an entity.
func (p *defaultEntityProvider) UpdateAttributes(
	entityID string, attributes json.RawMessage,
) *EntityProviderError {
	ctx := security.WithRuntimeContext(context.Background())
	err := p.entitySvc.UpdateAttributes(ctx, entityID, attributes)
	if err != nil {
		return mapEntityError(err)
	}
	return nil
}

// UpdateSystemAttributes updates system-managed attributes for an entity.
func (p *defaultEntityProvider) UpdateSystemAttributes(
	entityID string, attributes json.RawMessage,
) *EntityProviderError {
	ctx := security.WithRuntimeContext(context.Background())
	err := p.entitySvc.UpdateSystemAttributes(ctx, entityID, attributes)
	if err != nil {
		return mapEntityError(err)
	}
	return nil
}

// UpdateSystemCredentials updates system-managed credentials for an entity.
// Uses merge behavior — existing credential types not in the update are preserved.
func (p *defaultEntityProvider) UpdateSystemCredentials(
	entityID string, credentials json.RawMessage,
) *EntityProviderError {
	ctx := security.WithRuntimeContext(context.Background())
	err := p.entitySvc.UpdateSystemCredentials(ctx, entityID, credentials)
	if err != nil {
		return mapEntityError(err)
	}
	return nil
}

// GetTransitiveEntityGroups retrieves all groups an entity belongs to, including inherited groups.
func (p *defaultEntityProvider) GetTransitiveEntityGroups(
	entityID string,
) ([]EntityGroup, *EntityProviderError) {
	ctx := security.WithRuntimeContext(context.Background())
	groups, err := p.entitySvc.GetTransitiveEntityGroups(ctx, entityID)
	if err != nil {
		return nil, mapEntityError(err)
	}

	result := make([]EntityGroup, len(groups))
	for i, g := range groups {
		result[i] = EntityGroup{
			ID:   g.ID,
			Name: g.Name,
			OUID: g.OUID,
		}
	}
	return result, nil
}

// ValidateEntityIDs validates that the given entity IDs exist.
func (p *defaultEntityProvider) ValidateEntityIDs(
	entityIDs []string,
) ([]string, *EntityProviderError) {
	ctx := security.WithRuntimeContext(context.Background())
	invalidIDs, err := p.entitySvc.ValidateEntityIDs(ctx, entityIDs)
	if err != nil {
		return nil, mapEntityError(err)
	}
	return invalidIDs, nil
}

// GetEntitiesByIDs retrieves multiple entities by their IDs.
func (p *defaultEntityProvider) GetEntitiesByIDs(
	entityIDs []string,
) ([]Entity, *EntityProviderError) {
	ctx := security.WithRuntimeContext(context.Background())
	entities, err := p.entitySvc.GetEntitiesByIDs(ctx, entityIDs)
	if err != nil {
		return nil, mapEntityError(err)
	}

	result := make([]Entity, len(entities))
	for i := range entities {
		result[i] = *toProviderEntity(&entities[i])
	}
	return result, nil
}

// GetEntityListCount returns the total number of entities in the given category.
func (p *defaultEntityProvider) GetEntityListCount(
	category EntityCategory, filters map[string]interface{},
) (int, *EntityProviderError) {
	ctx := security.WithRuntimeContext(context.Background())
	count, err := p.entitySvc.GetEntityListCount(ctx, entity.EntityCategory(category), filters)
	if err != nil {
		return 0, mapEntityError(err)
	}
	return count, nil
}

// GetEntityList returns a page of entities in the given category.
func (p *defaultEntityProvider) GetEntityList(
	category EntityCategory, limit, offset int, filters map[string]interface{},
) ([]Entity, *EntityProviderError) {
	ctx := security.WithRuntimeContext(context.Background())
	entities, err := p.entitySvc.GetEntityList(ctx, entity.EntityCategory(category), limit, offset, filters)
	if err != nil {
		return nil, mapEntityError(err)
	}
	result := make([]Entity, len(entities))
	for i := range entities {
		result[i] = *toProviderEntity(&entities[i])
	}
	return result, nil
}

// toProviderEntity converts an entity service Entity to a provider Entity.
func toProviderEntity(e *entity.Entity) *Entity {
	if e == nil {
		return nil
	}
	return &Entity{
		ID:               e.ID,
		Category:         EntityCategory(e.Category.String()),
		Type:             e.Type,
		State:            EntityState(e.State.String()),
		OUID:             e.OUID,
		OUHandle:         e.OUHandle,
		Attributes:       e.Attributes,
		SystemAttributes: e.SystemAttributes,
	}
}

// toServiceEntity converts a provider Entity to an entity service Entity.
func toServiceEntity(e *Entity) *entity.Entity {
	if e == nil {
		return nil
	}
	return &entity.Entity{
		ID:               e.ID,
		Category:         entity.EntityCategory(e.Category.String()),
		Type:             e.Type,
		State:            entity.EntityState(e.State.String()),
		OUID:             e.OUID,
		Attributes:       e.Attributes,
		SystemAttributes: e.SystemAttributes,
	}
}

// mapEntityError converts an entity service error into an EntityProviderError,
// preserving the underlying error code semantics where possible.
func mapEntityError(err error) *EntityProviderError {
	switch {
	case errors.Is(err, entity.ErrEntityNotFound):
		return NewEntityProviderError(ErrorCodeEntityNotFound, "Entity not found", err.Error())
	case errors.Is(err, entity.ErrAmbiguousEntity):
		return NewEntityProviderError(ErrorCodeAmbiguousEntity, "Ambiguous entity", err.Error())
	case errors.Is(err, entity.ErrAttributeConflict):
		return NewEntityProviderError(ErrorCodeAttributeConflict, "Attribute conflict", err.Error())
	case errors.Is(err, entity.ErrSchemaValidationFailed):
		return NewEntityProviderError(ErrorCodeSchemaValidationFailed, "Schema validation failed", err.Error())
	case errors.Is(err, entity.ErrInvalidCredential):
		return NewEntityProviderError(ErrorCodeInvalidRequestFormat, "Invalid credential", err.Error())
	case errors.Is(err, entity.ErrBadAttributesInRequest):
		return NewEntityProviderError(ErrorCodeInvalidRequestFormat, "Invalid request", err.Error())
	default:
		return NewEntityProviderError(ErrorCodeSystemError, "System error", err.Error())
	}
}
