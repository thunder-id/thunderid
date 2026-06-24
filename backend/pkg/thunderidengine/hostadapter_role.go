/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package thunderidengine

import (
	"context"

	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/host"
)

// roleAdapter implements role.RoleServiceInterface by delegating runtime authorization
// calls to a host.RoleProvider. Management operations are not supported through the host SDK.
type roleAdapter struct {
	h host.RoleProvider
}

func newRoleAdapter(h host.RoleProvider) role.RoleServiceInterface {
	return &roleAdapter{h: h}
}

func (a *roleAdapter) GetAuthorizedPermissions(
	ctx context.Context, entityID string, groups []string, requestedPermissions []string,
) ([]string, *serviceerror.ServiceError) {
	perms, err := a.h.GetAuthorizedPermissions(ctx, entityID, groups, requestedPermissions)
	if err != nil {
		return nil, &serviceerror.InternalServerError
	}
	return perms, nil
}

func (a *roleAdapter) GetUserRoles(
	ctx context.Context, entityID string, groupIDs []string,
) ([]string, *serviceerror.ServiceError) {
	roles, err := a.h.GetUserRoles(ctx, entityID, groupIDs)
	if err != nil {
		return nil, &serviceerror.InternalServerError
	}
	return roles, nil
}

func (*roleAdapter) GetRoleList(context.Context, int, int) (*role.RoleList, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

func (*roleAdapter) CreateRole(context.Context, role.RoleCreationDetail) (
	*role.RoleWithPermissionsAndAssignments, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

func (*roleAdapter) GetRoleWithPermissions(context.Context, string) (
	*role.RoleWithPermissions, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

func (*roleAdapter) UpdateRoleWithPermissions(context.Context, string, role.RoleUpdateDetail) (
	*role.RoleWithPermissions, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

func (*roleAdapter) DeleteRole(context.Context, string) *serviceerror.ServiceError {
	return &serviceerror.InternalServerError
}

func (*roleAdapter) IsRoleDeclarative(context.Context, string) (bool, *serviceerror.ServiceError) {
	return false, &serviceerror.InternalServerError
}

func (*roleAdapter) ResolveRoleOUHandle(
	context.Context, *role.RoleWithPermissionsAndAssignments,
) *serviceerror.ServiceError {
	return &serviceerror.InternalServerError
}
