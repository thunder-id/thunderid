// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"
	"errors"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
)

// checkRoleAccess reports whether the caller may perform the action on a role in the organization unit.
// It decides who may manage roles at all. Whether the caller may confer the role's permissions is
// checked separately, by CanGrantPermissions and CanGrantMembership.
func checkRoleAccess(ctx context.Context, authz sysauthz.SystemAuthorizationServiceInterface,
	action security.Action, ouID, roleID string) *tidcommon.ServiceError {
	allowed, svcErr := authz.IsActionAllowed(ctx, action, &sysauthz.ActionContext{
		ResourceType: security.ResourceTypeRole,
		OUID:         ouID,
		ResourceID:   roleID,
	})
	if svcErr != nil {
		log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName)).
			Error(ctx, "Failed to check role authorization", log.String("err", svcErr.Error.DefaultValue))
		return &tidcommon.InternalServerError
	}
	if !allowed {
		return &tidcommon.ErrorUnauthorized
	}
	return nil
}

// loadRoleForAction loads a role and checks the caller may perform the action on it in the role's
// organization unit.
func loadRoleForAction(ctx context.Context, store roleStoreInterface,
	authz sysauthz.SystemAuthorizationServiceInterface, action security.Action, id string,
) (*RoleWithPermissions, *tidcommon.ServiceError) {
	if id == "" {
		return nil, &ErrorMissingRoleID
	}
	role, err := store.GetRole(ctx, id)
	if err != nil {
		if errors.Is(err, ErrRoleNotFound) {
			return nil, &ErrorRoleNotFound
		}
		log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName)).
			Error(ctx, "Failed to retrieve role", log.String("id", id), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if svcErr := checkRoleAccess(ctx, authz, action, role.OUID, id); svcErr != nil {
		return nil, svcErr
	}
	return &role, nil
}

// listRolesInOUs returns one page of the roles in the organization units, taken in the given order.
func listRolesInOUs(ctx context.Context, store roleStoreInterface, ouIDs []string, limit, offset int,
) ([]Role, int, error) {
	roles := make([]Role, 0)
	total := 0
	for _, ouID := range ouIDs {
		count, err := store.GetRoleListCountByOUID(ctx, ouID)
		if err != nil {
			return nil, 0, err
		}
		start := offset - total
		total += count
		if start >= count || len(roles) >= limit {
			continue
		}
		if start < 0 {
			start = 0
		}
		page, err := store.GetRoleListByOUID(ctx, ouID, limit-len(roles), start)
		if err != nil {
			return nil, 0, err
		}
		roles = append(roles, page...)
	}
	return roles, total, nil
}
