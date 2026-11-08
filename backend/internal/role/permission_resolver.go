// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"
	"fmt"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/system/security"
)

// EffectivePermissionResolver enumerates the permissions a principal effectively holds. It
// satisfies sysauthz.PermissionResolver structurally, so no import edge to that package is needed.
// It lives here because roles and groups are in separate databases, and this is the only package
// with all three inputs available.
type EffectivePermissionResolver struct {
	roleService   RoleServiceInterface
	groupService  group.GroupServiceInterface
	entityService entity.EntityServiceInterface
}

// NewEffectivePermissionResolver builds a resolver over the services it needs.
func NewEffectivePermissionResolver(
	roleService RoleServiceInterface,
	groupService group.GroupServiceInterface,
	entityService entity.EntityServiceInterface,
) *EffectivePermissionResolver {
	return &EffectivePermissionResolver{
		roleService:   roleService,
		groupService:  groupService,
		entityService: entityService,
	}
}

// ResolveForEntity returns every permission an entity holds, directly and through its groups.
func (r *EffectivePermissionResolver) ResolveForEntity(
	ctx context.Context, entityID string,
) (security.PermissionSet, error) {
	if entityID == "" {
		return security.PermissionSet{}, nil
	}

	entityGroups, err := r.entityService.GetTransitiveEntityGroups(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve groups for entity: %w", err)
	}

	groupIDs := make([]string, 0, len(entityGroups))
	for _, entityGroup := range entityGroups {
		groupIDs = append(groupIDs, entityGroup.ID)
	}

	permissions, svcErr := r.roleService.GetAllPermissions(ctx, entityID, groupIDs)
	if svcErr != nil {
		return nil, fmt.Errorf("failed to resolve permissions for entity: %s",
			svcErr.Error.DefaultValue)
	}

	return permissions, nil
}

// GetEntityPermissions returns the permissions an entity holds now, directly and through its groups,
// keyed by resource server ID.
func (r *EffectivePermissionResolver) GetEntityPermissions(
	ctx context.Context, entityID string,
) (map[string][]string, error) {
	return r.ResolveForEntity(ctx, entityID)
}

// ResolveForGroup returns every permission that membership in a group confers, including the roles
// its ancestors carry. Omitting ancestors would let a caller join a harmless group nested inside a
// privileged one.
func (r *EffectivePermissionResolver) ResolveForGroup(
	ctx context.Context, groupID string,
) (security.PermissionSet, error) {
	if groupID == "" {
		return security.PermissionSet{}, nil
	}

	ancestors, svcErr := r.groupService.GetTransitiveAncestorGroups(ctx, groupID)
	if svcErr != nil {
		return nil, fmt.Errorf("failed to resolve ancestor groups: %s", svcErr.Error.DefaultValue)
	}

	groupIDs := make([]string, 0, len(ancestors)+1)
	groupIDs = append(groupIDs, groupID)
	groupIDs = append(groupIDs, ancestors...)

	permissions, svcErr := r.roleService.GetAllPermissions(ctx, "", groupIDs)
	if svcErr != nil {
		return nil, fmt.Errorf("failed to resolve permissions for group: %s",
			svcErr.Error.DefaultValue)
	}

	return permissions, nil
}

// ResolveForRole returns the permissions a role confers on its assignees.
func (r *EffectivePermissionResolver) ResolveForRole(
	ctx context.Context, roleID string,
) (security.PermissionSet, error) {
	if roleID == "" {
		return security.PermissionSet{}, nil
	}

	// What a role confers is resolved for the server's own checks, whichever organization unit holds it.
	role, svcErr := r.roleService.GetRoleWithPermissions(security.WithRuntimeContext(ctx), roleID)
	if svcErr != nil {
		return nil, fmt.Errorf("failed to resolve permissions for role: %s",
			svcErr.Error.DefaultValue)
	}

	permissions := make(security.PermissionSet, len(role.Permissions))
	for _, rp := range role.Permissions {
		permissions[rp.ResourceServerID] = append(
			permissions[rp.ResourceServerID], rp.Permissions...)
	}

	return permissions, nil
}

// toPermissionSet converts per-resource-server slices into the map form Covers expects.
func toPermissionSet(resourcePermissions []ResourcePermissions) security.PermissionSet {
	permissionSet := make(security.PermissionSet, len(resourcePermissions))
	for _, rp := range resourcePermissions {
		permissionSet[rp.ResourceServerID] = append(
			permissionSet[rp.ResourceServerID], rp.Permissions...)
	}
	return permissionSet
}
