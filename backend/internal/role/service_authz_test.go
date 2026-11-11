// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/filter"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
	"github.com/thunder-id/thunderid/tests/mocks/sharingmock"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

const (
	authzOwnOU   = "ou-own"
	authzOtherOU = "ou-other"
	authzRoleID  = "role-authz"
)

// expectOwnOUOnly expects each action to be checked, and allows it on the caller's own organization unit
// or in a runtime context. A list action lists the caller's own organization unit alone. Any other
// action is unexpected.
func expectOwnOUOnly(m *sysauthzmock.SystemAuthorizationServiceInterfaceMock, actions ...security.Action) {
	for _, action := range actions {
		if strings.HasSuffix(string(action), ":list") {
			m.On("GetAccessibleResources", mock.Anything, action, security.ResourceTypeOU).
				Return(&sysauthz.AccessibleResources{IDs: []string{authzOwnOU}}, nil)
			continue
		}
		m.On("IsActionAllowed", mock.Anything, action, mock.Anything).Return(
			func(ctx context.Context, _ security.Action, c *sysauthz.ActionContext) (bool, *tidcommon.ServiceError) {
				return security.IsRuntimeContext(ctx) || c.OUID == authzOwnOU, nil
			})
	}
}

// RoleAuthzTestSuite covers the organization unit checks on the role management operations.
type RoleAuthzTestSuite struct {
	suite.Suite
	authz       *sysauthzmock.SystemAuthorizationServiceInterfaceMock
	store       *roleStoreInterfaceMock
	sharing     *sharingmock.SharingServiceInterfaceMock
	service     RoleServiceInterface
	assignments *roleAssignmentService
}

func TestRoleAuthzTestSuite(t *testing.T) {
	suite.Run(t, new(RoleAuthzTestSuite))
}

func (s *RoleAuthzTestSuite) SetupTest() {
	s.authz = sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	s.store = newRoleStoreInterfaceMock(s.T())
	s.sharing = sharingmock.NewSharingServiceInterfaceMock(s.T())
	ous := oumock.NewOrganizationUnitServiceInterfaceMock(s.T())
	ous.On("GetOrganizationUnit", mock.Anything, mock.Anything).Return(oupkg.OrganizationUnit{}, nil).Maybe()
	ous.On("GetOrganizationUnitHandlesByIDs", mock.Anything, mock.Anything).Return(map[string]string{}, nil).Maybe()
	authz := s.authz
	authz.On("CanGrantPermissions", mock.Anything, mock.Anything).Return((*tidcommon.ServiceError)(nil)).Maybe()
	authz.On("CanGrantMembership", mock.Anything, mock.Anything, mock.Anything).
		Return((*tidcommon.ServiceError)(nil)).Maybe()
	s.service = newRoleService(s.store, nil, nil, ous, nil, &fakeTransactioner{}, authz, s.sharing,
		discardingBus{}, nil)
	s.assignments = newRoleAssignmentService(s.store, nil, nil, nil, nil, &fakeTransactioner{}, authz, s.sharing,
		discardingBus{})
}

// boundary sets the organization unit boundary the caller acts within.
func (s *RoleAuthzTestSuite) boundary(b sysauthz.Boundary) {
	s.authz.On("GetBoundary", mock.Anything).Return(b).Maybe()
}

func (s *RoleAuthzTestSuite) storedRole(ouID string) {
	s.store.On("GetRole", mock.Anything, authzRoleID).
		Return(RoleWithPermissions{ID: authzRoleID, Name: "role", OUID: ouID}, nil).Maybe()
}

func (s *RoleAuthzTestSuite) TestGetRoleInOU_OtherOU() {
	s.boundary(sysauthz.Boundary{Type: sysauthz.BoundaryTypeDeployment})
	expectOwnOUOnly(s.authz, security.ActionReadRole)
	s.storedRole(authzOtherOU)

	role, svcErr := s.service.GetRoleInOU(context.Background(), authzRoleID, "")
	s.Nil(role)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
}

func (s *RoleAuthzTestSuite) TestGetRoleInOU_AsSharedOUTheCallerMayNotRead() {
	expectOwnOUOnly(s.authz, security.ActionReadRole)
	s.storedRole(authzOtherOU)

	_, svcErr := s.service.GetRoleInOU(context.Background(), authzRoleID, "ou-third")
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	s.sharing.AssertNotCalled(s.T(), "IsVisible", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *RoleAuthzTestSuite) TestUpdateRole_MoveToOtherOU() {
	expectOwnOUOnly(s.authz, security.ActionUpdateRole)
	s.storedRole(authzOwnOU)

	_, svcErr := s.service.UpdateRoleWithPermissions(context.Background(), authzRoleID,
		RoleUpdateDetail{Name: "role", OUID: authzOtherOU})
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	s.store.AssertNotCalled(s.T(), "UpdateRole", mock.Anything, mock.Anything, mock.Anything)
}

func (s *RoleAuthzTestSuite) TestDeleteRole_OtherOU() {
	s.boundary(sysauthz.Boundary{Type: sysauthz.BoundaryTypeDeployment})
	expectOwnOUOnly(s.authz, security.ActionDeleteRole)
	s.storedRole(authzOtherOU)

	svcErr := s.service.DeleteRole(context.Background(), authzRoleID)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	s.store.AssertNotCalled(s.T(), "DeleteRole", mock.Anything, mock.Anything)
}

func (s *RoleAuthzTestSuite) TestCreateSharingPolicy_InitiatedByOtherOU() {
	expectOwnOUOnly(s.authz, security.ActionUpdateRole)
	s.storedRole(authzOwnOU)

	_, svcErr := s.service.CreateSharingPolicy(context.Background(), authzRoleID,
		sharing.PolicyRequest{InitiatingOUID: authzOtherOU})
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
}

func (s *RoleAuthzTestSuite) TestGetRoleAssignments_OtherOU() {
	expectOwnOUOnly(s.authz, security.ActionReadRole)
	s.storedRole(authzOtherOU)

	_, svcErr := s.assignments.GetRoleAssignments(context.Background(), authzRoleID, "", 10, 0, false)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
}

func (s *RoleAuthzTestSuite) TestAddAssignments_InOtherOU() {
	expectOwnOUOnly(s.authz, security.ActionUpdateRole)
	s.storedRole(authzOwnOU)

	svcErr := s.assignments.AddAssignments(context.Background(), authzRoleID, authzOtherOU,
		[]RoleAssignment{{ID: "user-1", Type: AssigneeTypeUser}})
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	s.store.AssertNotCalled(s.T(), "AddAssignments", mock.Anything, mock.Anything, mock.Anything)
}

func (s *RoleAuthzTestSuite) TestValidateRemoveAssignment_InOtherOU() {
	expectOwnOUOnly(s.authz, security.ActionUpdateRole)
	s.storedRole(authzOwnOU)

	_, svcErr := s.assignments.ValidateRemoveAssignment(context.Background(), authzRoleID, authzOtherOU, "user-1")
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	s.store.AssertNotCalled(s.T(), "GetRoleAssignments", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything)
}

// A caller confined to its own organization unit lists no roles of another one, not even its own roles shared there.
func (s *RoleAuthzTestSuite) TestGetRoleList_AnotherOUIsEmpty() {
	expectOwnOUOnly(s.authz, security.ActionListRoles)

	list, svcErr := s.service.GetRoleList(context.Background(), 10, 0,
		map[string]interface{}{filter.AttributeOUID: "ou-child"})
	s.Require().Nil(svcErr)
	s.Zero(list.TotalResults)
	s.Empty(list.Roles)
	s.sharing.AssertNotCalled(s.T(), "ListVisibleResourceIDs", mock.Anything, mock.Anything, mock.Anything)
	s.store.AssertNotCalled(s.T(), "GetRoleListByOUID", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything)
}

func (s *RoleAuthzTestSuite) TestGetRoleList_OwnOUOnly() {
	expectOwnOUOnly(s.authz, security.ActionListRoles)
	s.store.On("GetRoleListCountByOUID", mock.Anything, authzOwnOU).Return(1, nil)
	s.store.On("GetRoleListByOUID", mock.Anything, authzOwnOU, mock.Anything, 0).
		Return([]Role{{ID: "role-own", OUID: authzOwnOU}}, nil)
	s.sharing.On("ListVisibleResourceIDs", mock.Anything, RoleSharingType, authzOwnOU).
		Return([]string{"role-shared"}, (*tidcommon.ServiceError)(nil))
	s.store.On("GetRolesByIDs", mock.Anything, []string{"role-shared"}).
		Return([]Role{{ID: "role-shared", OUID: authzOtherOU}}, nil)

	list, svcErr := s.service.GetRoleList(context.Background(), 10, 0, nil)
	s.Require().Nil(svcErr)
	s.Equal(2, list.TotalResults)
	s.Require().Len(list.Roles, 2)
	s.True(list.Roles[0].IsOwnedByOU)
	s.False(list.Roles[1].IsOwnedByOU)
	s.Equal(authzOwnOU, list.Roles[1].OUID, "a shared role is listed under the unit it is shared with")
}

func (s *RoleAuthzTestSuite) TestGetRolesByPermission_OnlyReadable() {
	expectOwnOUOnly(s.authz, security.ActionReadRole)
	s.store.On("GetRolesByPermission", mock.Anything, "rs-1", "orders:read").Return([]Role{
		{ID: "role-own", OUID: authzOwnOU}, {ID: "role-other", OUID: authzOtherOU},
	}, nil)

	roles, svcErr := s.service.GetRolesByPermission(context.Background(), "rs-1", "orders:read")
	s.Require().Nil(svcErr)
	s.Require().Len(roles, 1)
	s.Equal("role-own", roles[0].ID)
}

func (s *RoleAuthzTestSuite) TestGetRolesByNames_OnlyReadable() {
	expectOwnOUOnly(s.authz, security.ActionReadRole)
	s.store.On("GetRolesByNames", mock.Anything, []string{"support"}).Return([]Role{
		{ID: "role-own", Name: "support", OUID: authzOwnOU}, {ID: "role-other", Name: "support", OUID: authzOtherOU},
	}, nil)

	byName, svcErr := s.service.GetRolesByNames(context.Background(), []string{"support"}, "")
	s.Require().Nil(svcErr)
	s.Require().Len(byName["support"], 1)
	s.Equal("role-own", byName["support"][0].ID)
}

func (s *RoleAuthzTestSuite) TestGetRoleInOU_OwnedByCallersOU() {
	expectOwnOUOnly(s.authz, security.ActionReadRole)
	s.boundary(sysauthz.Boundary{Type: sysauthz.BoundaryTypeOU, OUID: authzOwnOU})
	s.storedRole(authzOwnOU)

	role, svcErr := s.service.GetRoleInOU(context.Background(), authzRoleID, "")
	s.Require().Nil(svcErr)
	s.True(role.IsOwnedByOU)
	s.Equal(authzOwnOU, role.OUID)
}

func (s *RoleAuthzTestSuite) TestGetRoleInOU_SharedIntoCallersOU() {
	tests := []struct {
		name    string
		visible bool
		wantErr string
	}{
		{name: "shared", visible: true},
		{name: "not shared", wantErr: tidcommon.ErrorUnauthorized.Code},
	}
	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			expectOwnOUOnly(s.authz, security.ActionReadRole)
			s.boundary(sysauthz.Boundary{Type: sysauthz.BoundaryTypeOU, OUID: authzOwnOU})
			s.storedRole(authzOtherOU)
			s.sharing.On("IsVisible", mock.Anything, RoleSharingType, authzRoleID, authzOwnOU).
				Return(tc.visible, (*tidcommon.ServiceError)(nil))

			role, svcErr := s.service.GetRoleInOU(context.Background(), authzRoleID, "")
			if tc.wantErr != "" {
				s.Nil(role)
				s.Require().NotNil(svcErr)
				s.Equal(tc.wantErr, svcErr.Code)
				return
			}
			s.Require().Nil(svcErr)
			s.False(role.IsOwnedByOU)
			s.Equal(authzOwnOU, role.OUID, "a shared role is read under the unit it is shared into")
		})
	}
}

func (s *RoleAuthzTestSuite) TestChangeRole_SharedIntoCallersOU() {
	writes := map[string]struct {
		action security.Action
		write  func(RoleServiceInterface) *tidcommon.ServiceError
	}{
		"update": {action: security.ActionUpdateRole, write: func(svc RoleServiceInterface) *tidcommon.ServiceError {
			_, svcErr := svc.UpdateRoleWithPermissions(context.Background(), authzRoleID,
				RoleUpdateDetail{Name: "renamed"})
			return svcErr
		}},
		"delete": {action: security.ActionDeleteRole, write: func(svc RoleServiceInterface) *tidcommon.ServiceError {
			return svc.DeleteRole(context.Background(), authzRoleID)
		}},
	}
	visibility := []struct {
		name       string
		visible    bool
		visibleErr *tidcommon.ServiceError
		wantErr    string
	}{
		{name: "shared", visible: true, wantErr: ErrorSharedRoleReadOnly.Code},
		{name: "not shared", wantErr: tidcommon.ErrorUnauthorized.Code},
		{name: "visibility check fails", visibleErr: &tidcommon.InternalServerError,
			wantErr: tidcommon.InternalServerError.Code},
		{name: "the token's unit is gone", visibleErr: &oupkg.ErrorOrganizationUnitNotFound,
			wantErr: tidcommon.ErrorUnauthorized.Code},
	}
	for writeName, w := range writes {
		for _, tc := range visibility {
			s.Run(writeName+" "+tc.name, func() {
				s.SetupTest()
				expectOwnOUOnly(s.authz, w.action)
				s.boundary(sysauthz.Boundary{Type: sysauthz.BoundaryTypeOU, OUID: authzOwnOU})
				s.storedRole(authzOtherOU)
				s.sharing.On("IsVisible", mock.MatchedBy(security.IsRuntimeContext), RoleSharingType, authzRoleID,
					authzOwnOU).Return(tc.visible, tc.visibleErr)

				svcErr := w.write(s.service)
				s.Require().NotNil(svcErr)
				s.Equal(tc.wantErr, svcErr.Code)
				s.store.AssertNotCalled(s.T(), "UpdateRole", mock.Anything, mock.Anything, mock.Anything)
				s.store.AssertNotCalled(s.T(), "DeleteRole", mock.Anything, mock.Anything)
			})
		}
	}
}
