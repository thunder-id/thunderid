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

	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/filter"
	"github.com/thunder-id/thunderid/pkg/thunderidengine"
)

type ouBridge struct {
	provider thunderidengine.OUProvider
}

func newOUBridge(provider thunderidengine.OUProvider) *ouBridge {
	return &ouBridge{provider: provider}
}

func (b *ouBridge) GetOrganizationUnitList(
	_ context.Context, _, _ int, _ *filter.FilterGroup,
) (*ou.OrganizationUnitListResponse, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

func (b *ouBridge) CreateOrganizationUnit(
	_ context.Context, _ ou.OrganizationUnitRequestWithID,
) (ou.OrganizationUnit, *serviceerror.ServiceError) {
	return ou.OrganizationUnit{}, &serviceerror.InternalServerError
}

func (b *ouBridge) GetOrganizationUnit(ctx context.Context, ouID string) (
	ou.OrganizationUnit, *serviceerror.ServiceError,
) {
	if b.provider == nil {
		return ou.OrganizationUnit{}, &serviceerror.InternalServerError
	}
	unit, err := b.provider.GetOU(ctx, ouID)
	if err != nil {
		return ou.OrganizationUnit{}, providerError(err)
	}
	if unit == nil {
		return ou.OrganizationUnit{}, &ou.ErrorOrganizationUnitNotFound
	}
	return toOrganizationUnit(unit), nil
}

func (b *ouBridge) GetOrganizationUnitByPath(
	ctx context.Context, path string,
) (ou.OrganizationUnit, *serviceerror.ServiceError) {
	return b.GetOrganizationUnit(ctx, path)
}

func (b *ouBridge) IsOrganizationUnitExists(ctx context.Context, ouID string) (bool, *serviceerror.ServiceError) {
	_, svcErr := b.GetOrganizationUnit(ctx, ouID)
	if svcErr == nil {
		return true, nil
	}
	if svcErr.Code == ou.ErrorOrganizationUnitNotFound.Code {
		return false, nil
	}
	return false, svcErr
}

func (b *ouBridge) IsOrganizationUnitDeclarative(_ context.Context, _ string) bool {
	return false
}

func (b *ouBridge) IsParent(ctx context.Context, parentID, childID string) (bool, *serviceerror.ServiceError) {
	if b.provider == nil {
		return false, &serviceerror.InternalServerError
	}
	ancestors, err := b.provider.GetOUAncestors(ctx, childID)
	if err != nil {
		return false, providerError(err)
	}
	for _, ancestor := range ancestors {
		if ancestor.ID == parentID {
			return true, nil
		}
	}
	return false, nil
}

func (b *ouBridge) UpdateOrganizationUnit(
	_ context.Context, _ string, _ ou.OrganizationUnitRequestWithID,
) (ou.OrganizationUnit, *serviceerror.ServiceError) {
	return ou.OrganizationUnit{}, &serviceerror.InternalServerError
}

func (b *ouBridge) UpdateOrganizationUnitByPath(
	_ context.Context, _ string, _ ou.OrganizationUnitRequestWithID,
) (ou.OrganizationUnit, *serviceerror.ServiceError) {
	return ou.OrganizationUnit{}, &serviceerror.InternalServerError
}

func (b *ouBridge) DeleteOrganizationUnit(_ context.Context, _ string) *serviceerror.ServiceError {
	return &serviceerror.InternalServerError
}

func (b *ouBridge) DeleteOrganizationUnitByPath(_ context.Context, _ string) *serviceerror.ServiceError {
	return &serviceerror.InternalServerError
}

func (b *ouBridge) GetOrganizationUnitChildren(
	_ context.Context, _ string, _, _ int, _ *filter.FilterGroup,
) (*ou.OrganizationUnitListResponse, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

func (b *ouBridge) GetOrganizationUnitChildrenByPath(
	_ context.Context, _ string, _, _ int, _ *filter.FilterGroup,
) (*ou.OrganizationUnitListResponse, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

func (b *ouBridge) GetOrganizationUnitUsers(
	_ context.Context, _ string, _, _ int, _ bool,
) (*ou.UserListResponse, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

func (b *ouBridge) GetOrganizationUnitUsersByPath(
	_ context.Context, _ string, _, _ int, _ bool,
) (*ou.UserListResponse, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

func (b *ouBridge) GetOrganizationUnitGroups(
	_ context.Context, _ string, _, _ int,
) (*ou.GroupListResponse, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

func (b *ouBridge) GetOrganizationUnitGroupsByPath(
	_ context.Context, _ string, _, _ int,
) (*ou.GroupListResponse, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

func (b *ouBridge) GetOrganizationUnitHandlesByIDs(
	_ context.Context, _ []string,
) (map[string]string, *serviceerror.ServiceError) {
	return nil, &serviceerror.InternalServerError
}

var _ ou.OrganizationUnitServiceInterface = (*ouBridge)(nil)
