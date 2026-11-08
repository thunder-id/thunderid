// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sysauthz

import (
	"context"

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
