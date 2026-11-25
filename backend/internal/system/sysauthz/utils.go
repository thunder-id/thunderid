// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sysauthz

import (
	"context"
	"slices"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// ResolveListOU returns the organization unit a list of organization unit owned resources serves for
// the caller, given the one the request filters by, and false when the caller may list none of them.
// See AccessibleResources.listOU.
func ResolveListOU(ctx context.Context, authzService SystemAuthorizationServiceInterface, logger *log.Logger,
	action security.Action, requestedOUID string) (string, bool, *tidcommon.ServiceError) {
	accessible, svcErr := authzService.GetAccessibleResources(ctx, action, security.ResourceTypeOU)
	if svcErr != nil {
		logger.Error(ctx, "Failed to resolve the organization units the caller may list",
			log.String("action", string(action)), log.Any("error", svcErr))
		return "", false, &tidcommon.InternalServerError
	}
	listOUID, visible, err := accessible.listOU(requestedOUID)
	if err != nil {
		logger.Error(ctx, "Failed to resolve the organization unit to list",
			log.String("action", string(action)), log.Error(err))
		return "", false, &tidcommon.InternalServerError
	}
	return listOUID, visible, nil
}

// ResolveListOUs returns the organization units a list of organization unit owned resources serves for
// the caller, given the one the request filters by, and true when it serves every organization unit.
// Unlike ResolveListOU, it serves several organization units, such as the caller's own and the ones
// above it for an action they inherit.
func ResolveListOUs(ctx context.Context, authzService SystemAuthorizationServiceInterface, logger *log.Logger,
	action security.Action, requestedOUID string) ([]string, bool, *tidcommon.ServiceError) {
	accessible, svcErr := authzService.GetAccessibleResources(ctx, action, security.ResourceTypeOU)
	if svcErr != nil {
		logger.Error(ctx, "Failed to resolve the organization units the caller may list",
			log.String("action", string(action)), log.Any("error", svcErr))
		return nil, false, &tidcommon.InternalServerError
	}
	switch {
	case accessible.AllAllowed && requestedOUID == "":
		return nil, true, nil
	case accessible.AllAllowed || slices.Contains(accessible.IDs, requestedOUID):
		return []string{requestedOUID}, false, nil
	case requestedOUID != "":
		return []string{}, false, nil
	default:
		return accessible.IDs, false, nil
	}
}

// boundaryOU returns the organization unit the caller is confined to, and empty when the caller may act across
// the deployment or is the server itself.
func boundaryOU(ctx context.Context, authzService SystemAuthorizationServiceInterface) string {
	if security.IsRuntimeContext(ctx) {
		return ""
	}
	if boundary := authzService.GetBoundary(ctx); boundary.Type == BoundaryTypeOU {
		return boundary.OUID
	}
	return ""
}

// SharedAccessOU returns the organization unit in which a caller refused access to a resource in its owning
// organization unit may still reach the resource, when it is shared there: the one the caller is confined to,
// unless that unit owns the resource. It returns empty for any other refusal.
func SharedAccessOU(ctx context.Context, authzService SystemAuthorizationServiceInterface, ownerOUID string,
	refusal *tidcommon.ServiceError) string {
	if refusal == nil || refusal.Code != tidcommon.ErrorUnauthorized.Code {
		return ""
	}
	if ouID := boundaryOU(ctx, authzService); ouID != ownerOUID {
		return ouID
	}
	return ""
}

// CheckActionInAnyOU checks the caller may perform the action on a resource of the type in any of
// the organization units. It answers tidcommon.ErrorUnauthorized when the caller may act in none.
func CheckActionInAnyOU(ctx context.Context, authzService SystemAuthorizationServiceInterface,
	logger *log.Logger, action security.Action, resourceType security.ResourceType,
	ouIDs ...string) *tidcommon.ServiceError {
	for _, ouID := range ouIDs {
		allowed, svcErr := authzService.IsActionAllowed(ctx, action,
			&ActionContext{ResourceType: resourceType, OUID: ouID})
		if svcErr != nil {
			logger.Error(ctx, "Failed to check authorization for action",
				log.String("action", string(action)), log.Any("error", svcErr))
			return &tidcommon.InternalServerError
		}
		if allowed {
			return nil
		}
	}
	return &tidcommon.ErrorUnauthorized
}
