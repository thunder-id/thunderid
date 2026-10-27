// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

// RoleAccessTestSuite tests who may manage roles in which organization units.
type RoleAccessTestSuite struct {
	suite.Suite
	store       *roleStoreInterfaceMock
	ous         *oumock.OrganizationUnitServiceInterfaceMock
	authz       *sysauthzmock.SystemAuthorizationServiceInterfaceMock
	service     RoleServiceInterface
	assignments RoleAssignmentServiceInterface
}

func TestRoleAccessTestSuite(t *testing.T) {
	suite.Run(t, new(RoleAccessTestSuite))
}

func (s *RoleAccessTestSuite) SetupTest() {
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime("/tmp/test", &config.Config{}))
	s.store = newRoleStoreInterfaceMock(s.T())
	s.ous = oumock.NewOrganizationUnitServiceInterfaceMock(s.T())
	s.authz = sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	s.service = newRoleService(s.store, nil, nil, s.ous, nil, &fakeTransactioner{}, s.authz)
	s.assignments = newRoleAssignmentService(s.store, nil, nil, nil, nil, &fakeTransactioner{}, s.authz)
}

func (s *RoleAccessTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

// allow sets whether the caller may perform the action on roles in the organization unit.
func (s *RoleAccessTestSuite) allow(action security.Action, ouID, roleID string, allowed bool) {
	s.authz.On("IsActionAllowed", mock.Anything, action, &sysauthz.ActionContext{
		ResourceType: security.ResourceTypeRole, OUID: ouID, ResourceID: roleID,
	}).Return(allowed, nil).Once()
}

func (s *RoleAccessTestSuite) roleIn(id, ouID string) {
	s.store.On("GetRole", mock.Anything, id).Return(RoleWithPermissions{ID: id, OUID: ouID}, nil).Once()
}

func (s *RoleAccessTestSuite) TestGetRoleListShowsOnlyAccessibleOU() {
	s.authz.On("GetAccessibleResources", mock.Anything, security.ActionListRoles, security.ResourceTypeOU).
		Return(&sysauthz.AccessibleResources{IDs: []string{"ou-1"}}, nil).Once()
	s.store.On("GetRoleListCountByOUID", mock.Anything, "ou-1").Return(2, nil).Once()
	s.store.On("GetRoleListByOUID", mock.Anything, "ou-1", 10, 0).Return([]Role{
		{ID: "role-1", OUID: "ou-1"}, {ID: "role-2", OUID: "ou-1"},
	}, nil).Once()
	s.ous.On("GetOrganizationUnitHandlesByIDs", mock.Anything, []string{"ou-1"}).
		Return(map[string]string{"ou-1": "sales"}, nil).Once()

	result, svcErr := s.service.GetRoleList(context.Background(), 10, 0)

	s.Require().Nil(svcErr)
	s.Equal(2, result.TotalResults)
	s.Equal(2, result.Count)
	s.Equal("sales", result.Roles[0].OUHandle)
	s.store.AssertNotCalled(s.T(), "GetRoleListCount", mock.Anything)
	s.store.AssertNotCalled(s.T(), "GetRoleList", mock.Anything, mock.Anything, mock.Anything)
}

func (s *RoleAccessTestSuite) TestGetRoleListAccessCheckFailure() {
	s.authz.On("GetAccessibleResources", mock.Anything, security.ActionListRoles, security.ResourceTypeOU).
		Return(nil, &tidcommon.InternalServerError).Once()

	result, svcErr := s.service.GetRoleList(context.Background(), 10, 0)

	s.Nil(result)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

func (s *RoleAccessTestSuite) TestListRolesInOUsPagesAcrossOUs() {
	ctx := context.Background()
	s.store.On("GetRoleListCountByOUID", mock.Anything, "ou-1").Return(3, nil)
	s.store.On("GetRoleListCountByOUID", mock.Anything, "ou-2").Return(4, nil)
	s.store.On("GetRoleListCountByOUID", mock.Anything, "ou-3").Return(2, nil)

	// The page starts inside ou-1 and is filled from ou-2; ou-3 is only counted.
	s.store.On("GetRoleListByOUID", mock.Anything, "ou-1", 3, 2).Return([]Role{{ID: "a3"}}, nil).Once()
	s.store.On("GetRoleListByOUID", mock.Anything, "ou-2", 2, 0).
		Return([]Role{{ID: "b1"}, {ID: "b2"}}, nil).Once()

	roles, total, err := listRolesInOUs(ctx, s.store, []string{"ou-1", "ou-2", "ou-3"}, 3, 2)

	s.Require().NoError(err)
	s.Equal(9, total)
	s.Equal([]Role{{ID: "a3"}, {ID: "b1"}, {ID: "b2"}}, roles)

	// An offset past ou-1 skips it and starts inside ou-2.
	s.store.On("GetRoleListByOUID", mock.Anything, "ou-2", 3, 1).
		Return([]Role{{ID: "b2"}, {ID: "b3"}, {ID: "b4"}}, nil).Once()

	roles, total, err = listRolesInOUs(ctx, s.store, []string{"ou-1", "ou-2", "ou-3"}, 3, 4)

	s.Require().NoError(err)
	s.Equal(9, total)
	s.Equal([]Role{{ID: "b2"}, {ID: "b3"}, {ID: "b4"}}, roles)
}

func (s *RoleAccessTestSuite) TestCreateRoleRefusedOutsideCallersOU() {
	s.allow(security.ActionCreateRole, "ou-1", "", false)

	result, svcErr := s.service.CreateRole(context.Background(), RoleCreationDetail{Name: "Clerk", OUID: "ou-1"})

	s.Nil(result)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	s.store.AssertNotCalled(s.T(), "CreateRole", mock.Anything, mock.Anything, mock.Anything)
}

func (s *RoleAccessTestSuite) TestUpdateRoleRefusedWhenMovingIntoDeniedOU() {
	s.authz.On("CanGrantPermissions", mock.Anything, mock.Anything).Return(nil).Once()
	s.roleIn("role-1", "ou-1")
	s.allow(security.ActionUpdateRole, "ou-1", "role-1", true)
	s.allow(security.ActionUpdateRole, "ou-2", "role-1", false)

	result, svcErr := s.service.UpdateRoleWithPermissions(context.Background(), "role-1",
		RoleUpdateDetail{Name: "Clerk", OUID: "ou-2", Permissions: []ResourcePermissions{}})

	s.Nil(result)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	s.store.AssertNotCalled(s.T(), "UpdateRole", mock.Anything, mock.Anything, mock.Anything)
}

func (s *RoleAccessTestSuite) TestDeleteRoleRefusedInDeniedOU() {
	s.roleIn("role-1", "ou-1")
	s.allow(security.ActionDeleteRole, "ou-1", "role-1", false)

	svcErr := s.service.DeleteRole(context.Background(), "role-1")

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	s.store.AssertNotCalled(s.T(), "DeleteRole", mock.Anything, mock.Anything)
	s.store.AssertNotCalled(s.T(), "DeleteAssignmentsByRoleID", mock.Anything, mock.Anything)
}

func (s *RoleAccessTestSuite) TestAddAssignmentsRefusedBeforeMembershipCheck() {
	s.roleIn("role-1", "ou-1")
	s.allow(security.ActionUpdateRole, "ou-1", "role-1", false)

	svcErr := s.assignments.AddAssignments(context.Background(), "role-1",
		[]RoleAssignment{{ID: testUserID1, Type: AssigneeTypeUser}})

	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	s.authz.AssertNotCalled(s.T(), "CanGrantMembership", mock.Anything, mock.Anything, mock.Anything)
	s.store.AssertNotCalled(s.T(), "AddAssignments", mock.Anything, mock.Anything, mock.Anything)
}

func (s *RoleAccessTestSuite) TestGetRoleRefusedWhenReadDenied() {
	s.roleIn("role-1", "ou-1")
	s.allow(security.ActionReadRole, "ou-1", "role-1", false)

	result, svcErr := s.service.GetRoleWithPermissions(context.Background(), "role-1")

	s.Nil(result)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.ErrorUnauthorized.Code, svcErr.Code)
	s.ous.AssertNotCalled(s.T(), "GetOrganizationUnit", mock.Anything, mock.Anything)
}

func (s *RoleAccessTestSuite) TestAccessCheckFailureIsAServerError() {
	s.roleIn("role-1", "ou-1")
	s.authz.On("IsActionAllowed", mock.Anything, security.ActionReadRole, mock.Anything).
		Return(false, &tidcommon.ServiceError{Code: "AUTHZ-ERR"}).Once()

	result, svcErr := s.service.GetRoleWithPermissions(context.Background(), "role-1")

	s.Nil(result)
	s.Require().NotNil(svcErr)
	s.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}
