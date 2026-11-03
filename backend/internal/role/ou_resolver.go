// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"
	"errors"

	oupkg "github.com/thunder-id/thunderid/internal/ou"
)

// ouRoleResolverAdapter implements oupkg.OURoleResolver using the role service, so the OU package
// lists roles without depending on the role package.
type ouRoleResolverAdapter struct {
	service RoleServiceInterface
}

// newOURoleResolver creates a new OURoleResolver backed by the given role service.
func newOURoleResolver(service RoleServiceInterface) oupkg.OURoleResolver {
	return &ouRoleResolverAdapter{service: service}
}

// GetRoleListByOUID returns one page of the roles an organization unit owns, followed by the ones
// shared with it, and the total across every page.
func (a *ouRoleResolverAdapter) GetRoleListByOUID(
	ctx context.Context, ouID string, limit, offset int,
) ([]oupkg.Role, int, error) {
	list, svcErr := a.service.GetRoleList(ctx, limit, offset, map[string]interface{}{"ouId": ouID})
	if svcErr != nil {
		return nil, 0, errors.New(svcErr.Error.DefaultValue)
	}

	result := make([]oupkg.Role, len(list.Roles))
	for i, r := range list.Roles {
		result[i] = oupkg.Role{
			ID: r.ID, Name: r.Name, Description: r.Description, IsReadOnly: r.IsReadOnly, IsOwnedByOU: r.IsOwnedByOU,
		}
	}

	return result, list.TotalResults, nil
}
