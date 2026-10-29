// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"context"
	"slices"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entitytype"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// ouUserResolverAdapter implements oupkg.OUUserResolver using the entity service.
// This adapter allows the OU package to query user data without directly
// accessing the entity layer, maintaining proper package boundaries.
type ouUserResolverAdapter struct {
	entityService     entity.EntityServiceInterface
	entityTypeService entitytype.EntityTypeServiceInterface
}

// newOUUserResolver creates a new OUUserResolver backed by the given entity service.
func newOUUserResolver(
	entityService entity.EntityServiceInterface, entityTypeService entitytype.EntityTypeServiceInterface,
) oupkg.OUUserResolver {
	return &ouUserResolverAdapter{entityService: entityService, entityTypeService: entityTypeService}
}

// GetUserListByOUID returns one page of the users an organization unit owns, followed by the ones shared
// into it, and the total across every page. When includeDisplay is true, display names are resolved
// from user attributes using the schema service.
func (a *ouUserResolverAdapter) GetUserListByOUID(
	ctx context.Context, ouID string, limit, offset int, includeDisplay bool,
) ([]oupkg.User, int, error) {
	page, err := a.entityService.GetEntityPageInOU(
		ctx, providers.EntityCategoryUser, ouID, entity.OUEntityScopeAll, limit, offset, nil)
	if err != nil {
		return nil, 0, err
	}
	users := entitiesToUsers(slices.Concat(page.Owned, page.Shared))

	var displayAttrPaths map[string]string
	if includeDisplay {
		displayAttrPaths = resolveOUUserDisplayPaths(ctx, users, a.entityTypeService)
	}

	result := make([]oupkg.User, len(users))
	for i, u := range users {
		result[i] = oupkg.User{ID: u.ID, Type: u.Type, IsOwnedByOU: i < len(page.Owned)}
		if includeDisplay {
			result[i].Display = utils.ResolveDisplay(u.ID, u.Type, u.Attributes, displayAttrPaths)
		}
	}

	return result, page.TotalResults, nil
}

// GetResourceDependencies implements resourcedependency.Provider. It reports the users that belong
// to the given organization unit, which block the organization unit's deletion (a user cannot exist
// without its organization unit). Only organization unit targets are handled; other resource types
// have no user dependencies. The number of users scanned is bounded by MaxCompositeStoreRecords.
func (a *ouUserResolverAdapter) GetResourceDependencies(
	ctx context.Context, resourceType, id string) ([]resourcedependency.ResourceDependency, error) {
	if resourceType != resourcedependency.ResourceTypeOU {
		return []resourcedependency.ResourceDependency{}, nil
	}

	entities, err := a.entityService.GetEntityListByOUIDs(
		ctx, providers.EntityCategoryUser, []string{id}, serverconst.MaxCompositeStoreRecords, 0, nil)
	if err != nil {
		return nil, err
	}

	deps := make([]resourcedependency.ResourceDependency, 0, len(entities))
	for _, u := range entities {
		deps = append(deps, resourcedependency.ResourceDependency{
			ResourceType:     resourcedependency.ResourceTypeUser,
			ID:               u.ID,
			BehaviorOnDelete: resourcedependency.BehaviorRestrict,
		})
	}
	return deps, nil
}

// resolveOUUserDisplayPaths collects user types and resolves their display attribute paths.
func resolveOUUserDisplayPaths(
	ctx context.Context, users []providers.User, schemaService entitytype.EntityTypeServiceInterface,
) map[string]string {
	userTypes := make([]string, 0, len(users))
	for _, u := range users {
		userTypes = append(userTypes, u.Type)
	}

	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "OUUserResolver"))
	return entitytype.ResolveDisplayAttributePaths(ctx, entitytype.TypeCategoryUser, userTypes, schemaService, logger)
}
