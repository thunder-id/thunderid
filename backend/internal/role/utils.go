// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"

	"github.com/thunder-id/thunderid/internal/group"
	resourcepkg "github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/revocation"
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
		resourceServer, svcErr := resourceService.GetResourceServer(ctx, resource.ResourceServerID)
		if svcErr != nil {
			return nil, svcErr
		}
		if resourceServer == nil || resourceServer.Identifier == "" {
			continue
		}
		for _, permission := range resource.Permissions {
			scopes = append(scopes, revocation.AudienceScope{
				Audience: resourceServer.Identifier, Scope: permission, SubtreeDelimiter: resource.subtreeDelimiter,
			})
		}
	}
	return scopes, nil
}

// expandPrincipal returns the entity itself, or every entity reachable from the group.
func expandPrincipal(ctx context.Context, groupService group.GroupServiceInterface, principalID string,
	isGroup bool) ([]string, *tidcommon.ServiceError) {
	if !isGroup {
		return []string{principalID}, nil
	}
	return groupService.GetTransitiveMemberEntities(ctx, principalID)
}
