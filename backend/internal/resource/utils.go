// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package resource

import (
	"context"

	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// DeclaredPermissions lists covered resource permissions and resource-owned actions.
func DeclaredPermissions(
	ctx context.Context, resourceService ResourceServiceInterface, resourceServerID string,
	roots []string, delimiter string,
) ([]string, *tidcommon.ServiceError) {
	if len(roots) == 0 {
		return nil, nil
	}
	resources, svcErr := resourceService.GetAllResourceList(ctx, resourceServerID)
	if svcErr != nil {
		return nil, svcErr
	}

	permissions := make([]string, 0, len(resources))
	owners := make([]*string, 0, len(resources))
	for _, resource := range resources {
		if !security.AnyPermissionCovers(roots, resource.Permission, delimiter) {
			continue
		}
		permissions = append(permissions, resource.Permission)
		if resource.ID != "" {
			owners = append(owners, &resource.ID)
		}
	}

	for _, owner := range owners {
		actions, svcErr := allActions(ctx, resourceService, resourceServerID, owner)
		if svcErr != nil {
			return nil, svcErr
		}
		for _, action := range actions {
			if security.AnyPermissionCovers(roots, action.Permission, delimiter) {
				permissions = append(permissions, action.Permission)
			}
		}
	}

	return permissions, nil
}

// allActions pages through every action an owner declares.
func allActions(
	ctx context.Context, resourceService ResourceServiceInterface,
	resourceServerID string, resourceID *string,
) ([]providers.Action, *tidcommon.ServiceError) {
	const pageSize = 100

	actions := []providers.Action{}
	for offset := 0; ; offset += pageSize {
		page, svcErr := resourceService.GetActionList(ctx, resourceServerID, resourceID, "", pageSize, offset)
		if svcErr != nil {
			return nil, svcErr
		}
		if page == nil {
			return actions, nil
		}
		actions = append(actions, page.Actions...)
		if len(page.Actions) < pageSize || len(actions) >= page.TotalResults {
			return actions, nil
		}
	}
}
