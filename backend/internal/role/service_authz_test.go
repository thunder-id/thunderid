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

func (s *RoleAuthzTestSuite) storedRole(ouID string) {
	s.store.On("GetRole", mock.Anything, authzRoleID).
		Return(RoleWithPermissions{ID: authzRoleID, Name: "role", OUID: ouID}, nil).Maybe()
}

func (s *RoleAuthzTestSuite) TestGetRoleInOU_OtherOU() {
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

func (s *RoleAuthzTestSuite) TestGetRoleList_SharedIntoOUTheCallerMayNotList() {
	expectOwnOUOnly(s.authz, security.ActionListRoles, security.ActionReadRole)
	s.sharing.On("ListVisibleResourceIDs", mock.Anything, RoleSharingType, "ou-child").
		Return([]string{"role-own", "role-other"}, nil)
	s.store.On("GetRolesByIDs", mock.Anything, []string{"role-own", "role-other"}).Return([]Role{
		{ID: "role-own", OUID: authzOwnOU}, {ID: "role-other", OUID: authzOtherOU},
	}, nil)

	list, svcErr := s.service.GetRoleList(context.Background(), 10, 0,
		map[string]interface{}{filter.AttributeOUID: "ou-child"})
	s.Require().Nil(svcErr)
	s.Equal(1, list.TotalResults)
	s.Require().Len(list.Roles, 1)
	s.Equal("role-own", list.Roles[0].ID)
	s.Equal("ou-child", list.Roles[0].OUID)
	s.False(list.Roles[0].IsOwnedByOU)
	s.store.AssertNotCalled(s.T(), "GetRoleListByOUID", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything)
}

func (s *RoleAuthzTestSuite) TestGetRoleList_OwnOUOnly() {
	expectOwnOUOnly(s.authz, security.ActionListRoles)
	s.store.On("GetRoleListCountByOUID", mock.Anything, authzOwnOU).Return(1, nil)
	s.store.On("GetRoleListByOUID", mock.Anything, authzOwnOU, 10, 0).
		Return([]Role{{ID: "role-own", OUID: authzOwnOU}}, nil)

	list, svcErr := s.service.GetRoleList(context.Background(), 10, 0, nil)
	s.Require().Nil(svcErr)
	s.Equal(1, list.TotalResults)
	s.Require().Len(list.Roles, 1)
	s.True(list.Roles[0].IsOwnedByOU)
	s.sharing.AssertNotCalled(s.T(), "ListVisibleResourceIDs", mock.Anything, mock.Anything, mock.Anything)
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

	byName, svcErr := s.service.GetRolesByNames(context.Background(), []string{"support"})
	s.Require().Nil(svcErr)
	s.Require().Len(byName["support"], 1)
	s.Equal("role-own", byName["support"][0].ID)
}
