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

// revokedPermissions plans exact rules or whole-subtree rules for lost hierarchical grants.
func revokedPermissions(
	ctx context.Context, resourceService resourcepkg.ResourceServiceInterface,
	current, next []ResourcePermissions,
) ([]revocation.AudienceScope, *tidcommon.ServiceError) {
	kept := map[string][]string{}
	for _, resource := range next {
		kept[resource.ResourceServerID] = append(kept[resource.ResourceServerID], resource.Permissions...)
	}

	revoked := []revocation.AudienceScope{}
	for _, resource := range current {
		// A failed hierarchy lookup must stop revocation rather than narrow its scope.
		resourceServer, svcErr := resourceService.GetResourceServer(
			security.WithRuntimeContext(ctx), resource.ResourceServerID)
		if svcErr != nil {
			return nil, svcErr
		}
		if resourceServer == nil {
			return nil, &tidcommon.InternalServerError
		}
		delimiter := ""
		if resourceServer.HierarchicalPermissions {
			delimiter = resourceServer.Delimiter
		}
		stillGranted := func(permission string) bool {
			return security.AnyPermissionCovers(kept[resource.ResourceServerID], permission, delimiter)
		}

		removed := []string{}
		for _, permission := range resource.Permissions {
			if !stillGranted(permission) {
				removed = append(removed, permission)
			}
		}
		if len(removed) == 0 {
			continue
		}

		roots := removed
		if delimiter != "" {
			roots = []string{}
			for _, permission := range removed {
				covered := false
				for _, other := range removed {
					if other != permission && security.PermissionCovers(other, permission, delimiter) {
						covered = true
						break
					}
				}
				if !covered {
					roots = append(roots, permission)
				}
			}
		}
		if resourceServer.Identifier == "" {
			continue
		}
		for _, permission := range roots {
			revoked = append(revoked, revocation.AudienceScope{
				Audience: resourceServer.Identifier, Scope: permission, SubtreeDelimiter: delimiter,
			})
		}
	}
	return revoked, nil
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
