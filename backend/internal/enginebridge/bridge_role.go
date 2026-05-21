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

package enginebridge

import (
	"context"

	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/pkg/thunderidengine"
)

type roleBridge struct {
	provider thunderidengine.RoleProvider
}

func newRoleBridge(provider thunderidengine.RoleProvider) *roleBridge {
	return &roleBridge{provider: provider}
}

func (b *roleBridge) GetUserRoles(
	ctx context.Context, entityID string, groupIDs []string,
) ([]string, *serviceerror.ServiceError) {
	if b.provider == nil {
		return nil, &serviceerror.InternalServerError
	}
	roles, err := b.provider.GetUserRoles(ctx, entityID, groupIDs)
	if err != nil {
		return nil, &serviceerror.InternalServerError
	}
	names := make([]string, 0, len(roles))
	for _, r := range roles {
		if r.Name != "" {
			names = append(names, r.Name)
		}
	}
	return names, nil
}

func (b *roleBridge) CreateRole(
	_ context.Context, _ role.RoleCreationDetail,
) (*role.RoleWithPermissionsAndAssignments, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

func (b *roleBridge) DeleteRole(_ context.Context, _ string) *serviceerror.ServiceError {
	return &serviceerror.InternalServerError
}

func (b *roleBridge) GetAuthorizedPermissions(
	_ context.Context, _ string, _ []string, _ []string,
) ([]string, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

func (b *roleBridge) GetRoleList(
	_ context.Context, _, _ int,
) (*role.RoleList, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

func (b *roleBridge) GetRoleWithPermissions(
	_ context.Context, _ string,
) (*role.RoleWithPermissions, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

func (b *roleBridge) IsRoleDeclarative(_ context.Context, _ string) (bool, *serviceerror.ServiceError) {
	return false, &serviceerror.InternalServerError
}

func (b *roleBridge) ResolveRoleOUHandle(
	_ context.Context, _ *role.RoleWithPermissionsAndAssignments,
) *serviceerror.ServiceError {
	return &serviceerror.InternalServerError
}

func (b *roleBridge) UpdateRoleWithPermissions(
	_ context.Context, _ string, _ role.RoleUpdateDetail,
) (*role.RoleWithPermissions, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

var _ role.RoleServiceInterface = (*roleBridge)(nil)
