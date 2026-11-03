// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"

	resourcepkg "github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// revokedPermissions plans exact rules or whole-subtree rules for lost hierarchical grants.
func revokedPermissions(
	ctx context.Context, resourceService resourcepkg.ResourceServiceInterface,
	current, next []ResourcePermissions,
) ([]ResourcePermissions, *tidcommon.ServiceError) {
	kept := map[string][]string{}
	for _, resource := range next {
		kept[resource.ResourceServerID] = append(kept[resource.ResourceServerID], resource.Permissions...)
	}

	revoked := []ResourcePermissions{}
	for _, resource := range current {
		// A failed hierarchy lookup must stop revocation rather than narrow its scope.
		resourceServer, svcErr := resourceService.GetResourceServer(ctx, resource.ResourceServerID)
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

		if delimiter == "" {
			revoked = append(revoked, ResourcePermissions{
				ResourceServerID: resource.ResourceServerID, Permissions: removed,
			})
			continue
		}
		roots := []string{}
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
		if len(roots) > 0 {
			revoked = append(revoked, ResourcePermissions{
				ResourceServerID: resource.ResourceServerID, Permissions: roots, subtreeDelimiter: delimiter,
			})
		}
	}
	return revoked, nil
}
