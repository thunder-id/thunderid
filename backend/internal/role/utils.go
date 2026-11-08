// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"
	"slices"

	"github.com/thunder-id/thunderid/internal/group"
	resourcepkg "github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// resolveScopes pairs each permission with the audience of its resource server.
func resolveScopes(ctx context.Context, resourceService resourcepkg.ResourceServiceInterface,
	permissions []ResourcePermissions) ([]revocation.AudienceScope, *tidcommon.ServiceError) {
	scopes := make([]revocation.AudienceScope, 0, len(permissions))
	for _, resource := range permissions {
		if len(resource.Permissions) == 0 {
			continue
		}
		// The audience is read whichever organization unit owns the resource server.
		resourceServer, svcErr := resourceService.GetResourceServer(
			security.WithRuntimeContext(ctx), resource.ResourceServerID)
		if svcErr != nil {
			return nil, svcErr
		}
		if resourceServer == nil || resourceServer.Identifier == "" {
			continue
		}
		for _, permission := range resource.Permissions {
			scopes = append(scopes, revocation.AudienceScope{Audience: resourceServer.Identifier, Scope: permission})
		}
	}
	return scopes, nil
}

// droppedPermissions returns the permissions in current that next no longer grants.
func droppedPermissions(current, next []ResourcePermissions) []ResourcePermissions {
	kept := map[string]map[string]struct{}{}
	for _, resource := range next {
		if kept[resource.ResourceServerID] == nil {
			kept[resource.ResourceServerID] = map[string]struct{}{}
		}
		for _, permission := range resource.Permissions {
			kept[resource.ResourceServerID][permission] = struct{}{}
		}
	}

	dropped := []ResourcePermissions{}
	for _, resource := range current {
		removed := []string{}
		for _, permission := range resource.Permissions {
			if _, ok := kept[resource.ResourceServerID][permission]; !ok {
				removed = append(removed, permission)
			}
		}
		if len(removed) > 0 {
			dropped = append(dropped, ResourcePermissions{
				ResourceServerID: resource.ResourceServerID,
				Permissions:      removed,
			})
		}
	}
	return dropped
}

// expandPrincipal returns the entity itself, or every entity reachable from the group.
func expandPrincipal(ctx context.Context, groupService group.GroupServiceInterface, principalID string,
	isGroup bool) ([]string, *tidcommon.ServiceError) {
	if !isGroup {
		return []string{principalID}, nil
	}
	return groupService.GetTransitiveMemberEntities(ctx, principalID)
}

// grantsPermission reports whether a permission set includes the given permission on a resource server.
func grantsPermission(permissions []ResourcePermissions, resourceServerID, permission string) bool {
	for _, p := range permissions {
		if p.ResourceServerID == resourceServerID && slices.Contains(p.Permissions, permission) {
			return true
		}
	}
	return false
}

// checkRoleAccess checks the caller may perform the action on a role of the organization unit.
func checkRoleAccess(ctx context.Context, authzService sysauthz.SystemAuthorizationServiceInterface,
	logger *log.Logger, action security.Action, ouID string) *tidcommon.ServiceError {
	allowed, svcErr := authzService.IsActionAllowed(ctx, action,
		&sysauthz.ActionContext{ResourceType: security.ResourceTypeRole, OUID: ouID})
	if svcErr != nil {
		logger.Error(ctx, "Failed to check authorization for action",
			log.String("action", string(action)), log.Any("error", svcErr))
		return &tidcommon.InternalServerError
	}
	if !allowed {
		return &tidcommon.ErrorUnauthorized
	}
	return nil
}
