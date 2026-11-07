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
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// defaultEntityProvider is the flow-facing boundary to the entity service. It reads with the system's
// authority and authorizes the caller's writes, which run as the caller.
type defaultEntityProvider struct {
	entitySvc entity.EntityServiceInterface
	authzSvc  sysauthz.SystemAuthorizationServiceInterface
}

// newDefaultEntityProvider creates a new default entity provider.
func newDefaultEntityProvider(
	entitySvc entity.EntityServiceInterface,
	authzSvc sysauthz.SystemAuthorizationServiceInterface,
) EntityProviderInterface {
	return &defaultEntityProvider{
		entitySvc: entitySvc,
		authzSvc:  authzSvc,
	}
}

// IdentifyEntity resolves an entity ID from indexed attribute filters.
func (p *defaultEntityProvider) IdentifyEntity(
	ctx context.Context,
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
func (p *defaultEntityProvider) SearchEntities(
	ctx context.Context,
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

// GetLinkedEntities returns the entities linked to a subject at a connection, with OUHandle set.
func (p *defaultEntityProvider) GetLinkedEntities(
	ctx context.Context,
	idpID, sub string,
) ([]*providers.Entity, *EntityProviderError) {
	ctx = security.WithRuntimeContext(ctx)
	entities, err := p.entitySvc.GetLinkedEntities(ctx, idpID, sub)
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
func (p *defaultEntityProvider) GetEntity(
	ctx context.Context, entityID string,
) (*providers.Entity, *EntityProviderError) {
	ctx = security.WithRuntimeContext(ctx)
	result, err := p.entitySvc.GetEntity(ctx, entityID)
	if err != nil {
		return nil, mapEntityError(err)
	}
	return toProviderEntity(result), nil
}

// GetEntityProfile retrieves an entity's profile by ID, without reading its runtime state.
func (p *defaultEntityProvider) GetEntityProfile(
	ctx context.Context, entityID string,
) (*providers.Entity, *EntityProviderError) {
	ctx = security.WithRuntimeContext(ctx)
	result, err := p.entitySvc.GetEntityProfile(ctx, entityID)
	if err != nil {
		return nil, mapEntityError(err)
	}
	return toProviderEntity(result), nil
}

// UpdateEntity updates an existing entity.
func (p *defaultEntityProvider) UpdateEntity(
	ctx context.Context,
	entityID string, e *providers.Entity,
) (*providers.Entity, *EntityProviderError) {
	if e == nil {
		return nil, NewEntityProviderError(ErrorCodeInvalidRequestFormat, "Invalid request",
			"Entity cannot be nil")
	}
	if authErr := p.authorizeWrite(ctx, entityID, false); authErr != nil {
		return nil, authErr
	}
	if authErr := p.requireEntityCoverage(ctx, entityID); authErr != nil {
		return nil, authErr
	}
	svcEntity := toServiceEntity(e)
	result, err := p.entitySvc.UpdateEntity(ctx, entityID, svcEntity)
	if err != nil {
		return nil, mapEntityError(err)
	}
	return toProviderEntity(result), nil
}

// DeleteEntity deletes an entity by ID.
func (p *defaultEntityProvider) DeleteEntity(
	ctx context.Context,
	entityID string,
) *EntityProviderError {
	if authErr := p.authorizeWrite(ctx, entityID, true); authErr != nil {
		if authErr.Code == ErrorCodeEntityNotFound {
			return nil
		}
		return authErr
	}
	if authErr := p.requireEntityCoverage(ctx, entityID); authErr != nil {
		return authErr
	}
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
	ctx context.Context,
	entityID string, credentials json.RawMessage,
) *EntityProviderError {
	if authErr := p.authorizeWrite(ctx, entityID, false); authErr != nil {
		return authErr
	}
	if authErr := p.requireEntityCoverage(ctx, entityID); authErr != nil {
		return authErr
	}
	err := p.entitySvc.UpdateCredentials(ctx, entityID, credentials)
	if err != nil {
		return mapEntityError(err)
	}
	return nil
}

// UpdateAttributes updates schema-defined attributes for an entity.
func (p *defaultEntityProvider) UpdateAttributes(
	ctx context.Context,
	entityID string, attributes json.RawMessage,
) *EntityProviderError {
	if authErr := p.authorizeWrite(ctx, entityID, false); authErr != nil {
		return authErr
	}
	if authErr := p.requireEntityCoverage(ctx, entityID); authErr != nil {
		return authErr
	}
	err := p.entitySvc.UpdateAttributes(ctx, entityID, attributes)
	if err != nil {
		return mapEntityError(err)
	}
	return nil
}

// UpdateSystemAttributes updates system-managed attributes for an entity.
func (p *defaultEntityProvider) UpdateSystemAttributes(
	ctx context.Context,
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
func (p *defaultEntityProvider) UpdateSystemCredentials(
	ctx context.Context,
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
func (p *defaultEntityProvider) GetTransitiveEntityGroups(
	ctx context.Context, entityID string,
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

// GetTransitiveGroupAncestors resolves the ancestor chain of a single group.
func (p *defaultEntityProvider) GetTransitiveGroupAncestors(
	ctx context.Context, groupID string,
) ([]string, *EntityProviderError) {
	ctx = security.WithRuntimeContext(ctx)
	ancestors, err := p.entitySvc.GetTransitiveGroupAncestors(ctx, groupID)
	if err != nil {
		return nil, mapEntityError(err)
	}
	return ancestors, nil
}

// ValidateEntityIDs validates that the given entity IDs exist.
func (p *defaultEntityProvider) ValidateEntityIDs(
	ctx context.Context,
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
func (p *defaultEntityProvider) GetEntitiesByIDs(
	ctx context.Context,
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
func (p *defaultEntityProvider) GetEntityListCount(
	ctx context.Context,
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
func (p *defaultEntityProvider) GetEntityList(
	ctx context.Context,
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

// authorizeWrite reads the entity with the system's authority and checks the caller may update or delete it.
func (p *defaultEntityProvider) authorizeWrite(
	ctx context.Context, entityID string, isDelete bool,
) *EntityProviderError {
	target, err := p.entitySvc.GetEntityProfile(security.WithRuntimeContext(ctx), entityID)
	if err != nil {
		return mapEntityError(err)
	}

	var action security.Action
	var actionCtx *sysauthz.ActionContext
	switch target.Category {
	case providers.EntityCategoryUser:
		action = security.ActionUpdateUser
		if isDelete {
			action = security.ActionDeleteUser
		}
		actionCtx = &sysauthz.ActionContext{
			OUID: target.OUID, ResourceType: security.ResourceTypeUser, ResourceID: entityID,
		}
	case providers.EntityCategoryAgent:
		action = security.ActionUpdateAgent
		if isDelete {
			action = security.ActionDeleteAgent
		}
		actionCtx = &sysauthz.ActionContext{
			OUID: target.OUID, ResourceType: security.ResourceTypeAgent, ResourceID: entityID,
		}
	case providers.EntityCategoryApp:
		action = security.ActionUpdateApplication
		if isDelete {
			action = security.ActionDeleteApplication
		}
		actionCtx = &sysauthz.ActionContext{ResourceType: security.ResourceTypeApplication, ResourceID: entityID}
	default:
		if security.IsRuntimeContext(ctx) {
			return nil
		}
		return NewEntityProviderError(ErrorCodeUnauthorized, "Unauthorized",
			"No permission governs writes to this entity category")
	}

	allowed, svcErr := p.authzSvc.IsActionAllowed(ctx, action, actionCtx)
	if svcErr != nil {
		return NewEntityProviderError(ErrorCodeSystemError, "System error", svcErr.Error.DefaultValue)
	}
	if !allowed {
		return NewEntityProviderError(ErrorCodeUnauthorized, "Unauthorized",
			"The caller is not permitted to change this entity")
	}
	return nil
}

func (p *defaultEntityProvider) requireEntityCoverage(ctx context.Context, entityID string) *EntityProviderError {
	if security.IsRuntimeContext(ctx) {
		return nil
	}
	svcErr := p.authzSvc.CanControlEntity(ctx, entityID)
	if svcErr != nil {
		if svcErr.Code == sysauthz.ErrorGrantNotPermitted.Code {
			return NewEntityProviderError(ErrorCodeUnauthorized, "Unauthorized",
				"The caller is not permitted to change this entity")
		}
		return NewEntityProviderError(ErrorCodeSystemError, "System error", svcErr.Error.DefaultValue)
	}
	return nil
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
