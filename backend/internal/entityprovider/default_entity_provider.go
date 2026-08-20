// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package entityprovider implements the gateway-to-directory boundary for entity operations.
package entityprovider

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
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
func (p *defaultEntityProvider) IdentifyEntity(ctx context.Context,
	filters map[string]interface{},
) (*string, *EntityProviderError) {
	ctx = security.WithRuntimeContext(ctx)
	entityID, err := p.entitySvc.IdentifyEntity(ctx, filters)
	if err != nil {
		return nil, mapEntityError(err)
	}
	return entityID, nil
}

// SearchEntities searches for all entities matching the given filters.
// OUHandle is not resolved here — callers that need it (e.g. disambiguation flows)
// resolve it on demand via the OU service.
func (p *defaultEntityProvider) SearchEntities(ctx context.Context,
	filters map[string]interface{},
) ([]*providers.Entity, *EntityProviderError) {
	ctx = security.WithRuntimeContext(ctx)
	entities, err := p.entitySvc.SearchEntities(ctx, filters)
	if err != nil {
		return nil, mapEntityError(err)
	}
	result := make([]*providers.Entity, 0, len(entities))
	for i := range entities {
		result = append(result, toProviderEntity(&entities[i]))
	}
	return result, nil
}

// GetEntity retrieves an entity by ID.
func (p *defaultEntityProvider) GetEntity(ctx context.Context,
	entityID string,
) (*providers.Entity, *EntityProviderError) {
	ctx = security.WithRuntimeContext(ctx)
	result, err := p.entitySvc.GetEntity(ctx, entityID)
	if err != nil {
		return nil, mapEntityError(err)
	}
	return toProviderEntity(result), nil
}

// CreateEntity creates a new entity.
func (p *defaultEntityProvider) CreateEntity(ctx context.Context,
	e *providers.Entity, systemCredentials json.RawMessage,
) (*providers.Entity, *EntityProviderError) {
	if e == nil {
		return nil, NewEntityProviderError(ErrorCodeInvalidRequestFormat, "Invalid request",
			"Entity cannot be nil")
	}
	ctx = security.WithRuntimeContext(ctx)
	svcEntity := toServiceEntity(e)
	result, err := p.entitySvc.CreateEntity(ctx, svcEntity, systemCredentials)
	if err != nil {
		return nil, mapEntityError(err)
	}
	return toProviderEntity(result), nil
}

// UpdateEntity updates an existing entity.
func (p *defaultEntityProvider) UpdateEntity(ctx context.Context,
	entityID string, e *providers.Entity,
) (*providers.Entity, *EntityProviderError) {
	if e == nil {
		return nil, NewEntityProviderError(ErrorCodeInvalidRequestFormat, "Invalid request",
			"Entity cannot be nil")
	}
	ctx = security.WithRuntimeContext(ctx)
	svcEntity := toServiceEntity(e)
	result, err := p.entitySvc.UpdateEntity(ctx, entityID, svcEntity)
	if err != nil {
		return nil, mapEntityError(err)
	}
	return toProviderEntity(result), nil
}

// DeleteEntity deletes an entity by ID.
func (p *defaultEntityProvider) DeleteEntity(ctx context.Context,
	entityID string,
) *EntityProviderError {
	ctx = security.WithRuntimeContext(ctx)
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
func (p *defaultEntityProvider) UpdateCredentials(ctx context.Context,
	entityID string, credentials json.RawMessage,
) *EntityProviderError {
	ctx = security.WithRuntimeContext(ctx)
	err := p.entitySvc.UpdateCredentials(ctx, entityID, credentials)
	if err != nil {
		return mapEntityError(err)
	}
	return nil
}

// UpdateAttributes updates schema-defined attributes for an entity.
func (p *defaultEntityProvider) UpdateAttributes(ctx context.Context,
	entityID string, attributes json.RawMessage,
) *EntityProviderError {
	ctx = security.WithRuntimeContext(ctx)
	err := p.entitySvc.UpdateAttributes(ctx, entityID, attributes)
	if err != nil {
		return mapEntityError(err)
	}
	return nil
}

// UpdateSystemAttributes updates system-managed attributes for an entity.
func (p *defaultEntityProvider) UpdateSystemAttributes(ctx context.Context,
	entityID string, attributes json.RawMessage,
) *EntityProviderError {
	ctx = security.WithRuntimeContext(ctx)
	err := p.entitySvc.UpdateSystemAttributes(ctx, entityID, attributes)
	if err != nil {
		return mapEntityError(err)
	}
	return nil
}

// UpdateSystemCredentials updates system-managed credentials for an entity.
// Uses merge behavior — existing credential types not in the update are preserved.
func (p *defaultEntityProvider) UpdateSystemCredentials(ctx context.Context,
	entityID string, credentials json.RawMessage,
) *EntityProviderError {
	ctx = security.WithRuntimeContext(ctx)
	err := p.entitySvc.UpdateSystemCredentials(ctx, entityID, credentials)
	if err != nil {
		return mapEntityError(err)
	}
	return nil
}

// GetTransitiveEntityGroups retrieves all groups an entity belongs to, including inherited groups.
func (p *defaultEntityProvider) GetTransitiveEntityGroups(ctx context.Context,
	entityID string,
) ([]providers.EntityGroup, *EntityProviderError) {
	ctx = security.WithRuntimeContext(ctx)
	groups, err := p.entitySvc.GetTransitiveEntityGroups(ctx, entityID)
	if err != nil {
		return nil, mapEntityError(err)
	}

	result := make([]providers.EntityGroup, len(groups))
	copy(result, groups)
	return result, nil
}

// ValidateEntityIDs validates that the given entity IDs exist.
func (p *defaultEntityProvider) ValidateEntityIDs(ctx context.Context,
	entityIDs []string,
) ([]string, *EntityProviderError) {
	ctx = security.WithRuntimeContext(ctx)
	invalidIDs, err := p.entitySvc.ValidateEntityIDs(ctx, entityIDs)
	if err != nil {
		return nil, mapEntityError(err)
	}
	return invalidIDs, nil
}

// GetEntitiesByIDs retrieves multiple entities by their IDs.
func (p *defaultEntityProvider) GetEntitiesByIDs(ctx context.Context,
	entityIDs []string,
) ([]providers.Entity, *EntityProviderError) {
	ctx = security.WithRuntimeContext(ctx)
	entities, err := p.entitySvc.GetEntitiesByIDs(ctx, entityIDs)
	if err != nil {
		return nil, mapEntityError(err)
	}

	result := make([]providers.Entity, len(entities))
	for i := range entities {
		result[i] = *toProviderEntity(&entities[i])
	}
	return result, nil
}

// GetEntityListCount returns the total number of entities in the given category.
func (p *defaultEntityProvider) GetEntityListCount(ctx context.Context,
	category providers.EntityCategory, filters map[string]interface{},
) (int, *EntityProviderError) {
	ctx = security.WithRuntimeContext(ctx)
	count, err := p.entitySvc.GetEntityListCount(ctx, category, filters)
	if err != nil {
		return 0, mapEntityError(err)
	}
	return count, nil
}

// GetEntityList returns a page of entities in the given category.
func (p *defaultEntityProvider) GetEntityList(ctx context.Context,
	category providers.EntityCategory, limit, offset int, filters map[string]interface{},
) ([]providers.Entity, *EntityProviderError) {
	ctx = security.WithRuntimeContext(ctx)
	entities, err := p.entitySvc.GetEntityList(ctx, category, limit, offset, filters)
	if err != nil {
		return nil, mapEntityError(err)
	}
	result := make([]providers.Entity, len(entities))
	for i := range entities {
		result[i] = *toProviderEntity(&entities[i])
	}
	return result, nil
}

// toProviderEntity converts an entity service Entity to a provider Entity.
func toProviderEntity(e *providers.Entity) *providers.Entity {
	if e == nil {
		return nil
	}
	return &providers.Entity{
		ID:               e.ID,
		Category:         e.Category,
		Type:             e.Type,
		State:            e.State,
		OUID:             e.OUID,
		OUHandle:         e.OUHandle,
		Attributes:       e.Attributes,
		SystemAttributes: e.SystemAttributes,
	}
}

// toServiceEntity converts a provider Entity to an entity service Entity.
func toServiceEntity(e *providers.Entity) *providers.Entity {
	if e == nil {
		return nil
	}
	return &providers.Entity{
		ID:               e.ID,
		Category:         e.Category,
		Type:             e.Type,
		State:            e.State,
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
