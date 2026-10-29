// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"
	"errors"

	"github.com/thunder-id/thunderid/internal/group"
	resourcepkg "github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// checkRoleAccess authorizes the caller to perform the action on a role in the organization unit.
func checkRoleAccess(ctx context.Context, authzService sysauthz.SystemAuthorizationServiceInterface,
	action security.Action, ouID, roleID string) *tidcommon.ServiceError {
	allowed, svcErr := authzService.IsActionAllowed(ctx, action,
		&sysauthz.ActionContext{ResourceType: security.ResourceTypeRole, OUID: ouID, ResourceID: roleID})
	if svcErr != nil {
		log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName)).Error(ctx,
			"Failed to check authorization for action",
			log.String("action", string(action)), log.Any("error", svcErr))
		return &tidcommon.InternalServerError
	}
	if !allowed {
		return &tidcommon.ErrorUnauthorized
	}
	return nil
}

// checkRoleAccessByID authorizes the caller to perform the action on the role, against the
// organization unit that owns it.
func checkRoleAccessByID(ctx context.Context, roleStore roleStoreInterface,
	authzService sysauthz.SystemAuthorizationServiceInterface, action security.Action,
	id string) *tidcommon.ServiceError {
	if id == "" {
		return &ErrorMissingRoleID
	}
	role, err := roleStore.GetRole(ctx, id)
	if err != nil {
		if errors.Is(err, ErrRoleNotFound) {
			return &ErrorRoleNotFound
		}
		log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName)).Error(ctx,
			"Failed to retrieve role", log.String("id", id), log.Error(err))
		return &tidcommon.InternalServerError
	}
	return checkRoleAccess(ctx, authzService, action, role.OUID, id)
}

// resolveScopes pairs each permission with the audience of its resource server.
func resolveScopes(ctx context.Context, resourceService resourcepkg.ResourceServiceInterface,
	permissions []ResourcePermissions) ([]revocation.AudienceScope, *tidcommon.ServiceError) {
	// A role's permissions may come from a resource server another organization unit owns, so the
	// lookup is not bounded to the caller's organization unit.
	runtimeCtx := security.WithRuntimeContext(ctx)
	scopes := make([]revocation.AudienceScope, 0, len(permissions))
	for _, resource := range permissions {
		if len(resource.Permissions) == 0 {
			continue
		}
		resourceServer, svcErr := resourceService.GetResourceServer(runtimeCtx, resource.ResourceServerID)
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
