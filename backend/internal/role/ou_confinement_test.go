// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
	"github.com/thunder-id/thunderid/tests/mocks/resourcemock"
	"github.com/thunder-id/thunderid/tests/mocks/sharingmock"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

// OUConfinementTestSuite covers how role management confines a caller to the organization units it may act in.
type OUConfinementTestSuite struct {
	suite.Suite
	authz     *sysauthzmock.SystemAuthorizationServiceInterfaceMock
	store     *roleStoreInterfaceMock
	ous       *oumock.OrganizationUnitServiceInterfaceMock
	resources *resourcemock.ResourceServiceInterfaceMock
	sharing   *sharingmock.SharingServiceInterfaceMock
	service   RoleServiceInterface
}

func TestOUConfinementTestSuite(t *testing.T) {
	suite.Run(t, new(OUConfinementTestSuite))
}

func (s *OUConfinementTestSuite) SetupTest() {
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime("/tmp/test", &config.Config{}))
	s.authz = sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	s.store = newRoleStoreInterfaceMock(s.T())
	s.ous = oumock.NewOrganizationUnitServiceInterfaceMock(s.T())
	s.resources = resourcemock.NewResourceServiceInterfaceMock(s.T())
	s.sharing = sharingmock.NewSharingServiceInterfaceMock(s.T())
	s.service = newRoleService(s.store, nil, nil, s.ous, s.resources, &fakeTransactioner{}, s.authz, s.sharing,
		discardingBus{}, nil)
}

func (s *OUConfinementTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

// storedRole makes the store hold role-1, owned by the organization unit.
func (s *OUConfinementTestSuite) storedRole(ouID string) {
	s.store.On("GetRole", mock.Anything, "role-1").
		Return(RoleWithPermissions{ID: "role-1", Name: "role-1", OUID: ouID}, nil).Maybe()
}

// expectAccess expects one access check for the action on the organization unit and role, answered as given.
func (s *OUConfinementTestSuite) expectAccess(action security.Action, ouID, roleID string, allowed bool,
	svcErr *tidcommon.ServiceError) {
	s.authz.On("IsActionAllowed", mock.Anything, action,
		&sysauthz.ActionContext{ResourceType: security.ResourceTypeRole, OUID: ouID, ResourceID: roleID}).
		Return(allowed, svcErr).Once()
}

func (s *OUConfinementTestSuite) TestCheckRoleAccess() {
	tests := []struct {
		name     string
		allowed  bool
		authzErr *tidcommon.ServiceError
		want     *tidcommon.ServiceError
	}{
		{"Allowed", true, nil, nil},
		{"Denied", false, nil, &tidcommon.ErrorUnauthorized},
		{"AuthorizationFailure", false, &tidcommon.InternalServerError, &tidcommon.InternalServerError},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.SetupTest()
			s.expectAccess(security.ActionReadRole, "ou-1", "role-1", tt.allowed, tt.authzErr)
			rs := &roleService{authzService: s.authz, logger: log.GetLogger()}
			as := &roleAssignmentService{authzService: s.authz, logger: log.GetLogger()}
			s.Equal(tt.want, rs.checkRoleAccess(context.Background(), security.ActionReadRole, "ou-1", "role-1"))

			s.expectAccess(security.ActionReadRole, "ou-1", "role-1", tt.allowed, tt.authzErr)
			s.Equal(tt.want, as.checkRoleAccess(context.Background(), security.ActionReadRole, "ou-1", "role-1"))
		})
	}
}

func (s *OUConfinementTestSuite) TestCreateRole_RefusedOutsideTheCallersOU() {
	s.expectAccess(security.ActionCreateRole, "ou-2", "", false, nil)

	_, svcErr := s.service.CreateRole(context.Background(), RoleCreationDetail{Name: "auditor", OUID: "ou-2"})

	s.Equal(&tidcommon.ErrorUnauthorized, svcErr)
}

func (s *OUConfinementTestSuite) TestUpdateRole_RefusedInTheOwningOU() {
	s.storedRole("ou-1")
	s.expectAccess(security.ActionUpdateRole, "ou-1", "role-1", false, nil)

	_, svcErr := s.service.UpdateRoleWithPermissions(context.Background(), "role-1", RoleUpdateDetail{Name: "renamed"})

	s.Equal(&tidcommon.ErrorUnauthorized, svcErr)
}

func (s *OUConfinementTestSuite) TestUpdateRole_MoveNeedsCreateInTheTargetOU() {
	s.storedRole("ou-1")
	s.expectAccess(security.ActionUpdateRole, "ou-1", "role-1", true, nil)
	s.expectAccess(security.ActionCreateRole, "ou-2", "", false, nil)

	_, svcErr := s.service.UpdateRoleWithPermissions(context.Background(), "role-1",
		RoleUpdateDetail{Name: "role-1", OUID: "ou-2"})

	s.Equal(&tidcommon.ErrorUnauthorized, svcErr)
	s.store.AssertNotCalled(s.T(), "UpdateRole", mock.Anything, mock.Anything, mock.Anything)
}

func (s *OUConfinementTestSuite) TestDeleteRole_RefusedInTheOwningOU() {
	s.storedRole("ou-1")
	s.expectAccess(security.ActionDeleteRole, "ou-1", "role-1", false, nil)

	s.Equal(&tidcommon.ErrorUnauthorized, s.service.DeleteRole(context.Background(), "role-1"))
	s.store.AssertNotCalled(s.T(), "DeleteRole", mock.Anything, mock.Anything)
}

func (s *OUConfinementTestSuite) TestDeleteRole_UnknownIDChecksBeforeAnsweringNotFound() {
	s.store.On("GetRole", mock.Anything, "missing").Return(RoleWithPermissions{}, ErrRoleNotFound)
	s.expectAccess(security.ActionDeleteRole, "", "", false, nil)
	s.Equal(&tidcommon.ErrorUnauthorized, s.service.DeleteRole(context.Background(), "missing"))

	s.expectAccess(security.ActionDeleteRole, "", "", true, nil)
	s.Nil(s.service.DeleteRole(context.Background(), "missing"))
}

// runtimeContext matches a context carrying the runtime security context.
var runtimeContext = mock.MatchedBy(security.IsRuntimeContext)

// callerIn returns a context of a caller whose token names the organization unit.
func callerIn(ouID string) context.Context {
	return security.WithSecurityContext(context.Background(),
		security.NewSecurityContextForTest("caller", ouID, "", []string{"system:role"}, nil))
}

// unitHandle makes the organization unit service resolve the unit's handle.
func (s *OUConfinementTestSuite) unitHandle(ouID string) {
	s.ous.On("GetOrganizationUnit", mock.Anything, ouID).
		Return(oupkg.OrganizationUnit{ID: ouID, Handle: ouID + "-handle"}, nil).Maybe()
}

func (s *OUConfinementTestSuite) TestGetRole_OwnerReads() {
	s.storedRole("ou-a")
	s.unitHandle("ou-a")
	s.expectAccess(security.ActionReadRole, "ou-a", "role-1", true, nil)

	role, svcErr := s.service.GetRoleWithPermissions(callerIn("ou-a"), "role-1")

	s.Require().Nil(svcErr)
	s.Equal("ou-a", role.OUID)
	s.True(role.IsOwnedByOU)
}

func (s *OUConfinementTestSuite) TestGetRole_ShareeReadsItsView() {
	s.storedRole("ou-a")
	s.unitHandle("ou-a")
	s.unitHandle("ou-a1")
	s.expectAccess(security.ActionReadRole, "ou-a", "role-1", false, nil)
	s.expectAccess(security.ActionReadRole, "ou-a1", "role-1", true, nil)
	s.sharing.EXPECT().IsVisible(mock.Anything, RoleSharingType, "role-1", "ou-a1").Return(true, nil).Once()

	role, svcErr := s.service.GetRoleWithPermissions(callerIn("ou-a1"), "role-1")

	s.Require().Nil(svcErr)
	s.Equal("ou-a1", role.OUID)
	s.Equal("ou-a1-handle", role.OUHandle)
	s.False(role.IsOwnedByOU)
}

func (s *OUConfinementTestSuite) TestGetRole_UnrelatedOURefused() {
	s.storedRole("ou-a")
	s.unitHandle("ou-a")
	s.expectAccess(security.ActionReadRole, "ou-a", "role-1", false, nil)
	s.expectAccess(security.ActionReadRole, "ou-b", "role-1", true, nil)
	s.sharing.EXPECT().IsVisible(mock.Anything, RoleSharingType, "role-1", "ou-b").Return(false, nil).Once()

	_, svcErr := s.service.GetRoleWithPermissions(callerIn("ou-b"), "role-1")

	s.Equal(&tidcommon.ErrorUnauthorized, svcErr)
}

func (s *OUConfinementTestSuite) TestGetRoleInOU_RefusedWithoutReadInThatOU() {
	s.expectAccess(security.ActionReadRole, "ou-a1", "", false, nil)

	_, svcErr := s.service.GetRoleInOU(callerIn("ou-b"), "role-1", "ou-a1")

	s.Equal(&tidcommon.ErrorUnauthorized, svcErr)
}

func (s *OUConfinementTestSuite) TestGetRoleInOU_ShareeView() {
	s.storedRole("ou-a")
	s.unitHandle("ou-a")
	s.unitHandle("ou-a1")
	s.expectAccess(security.ActionReadRole, "ou-a1", "", true, nil)
	s.sharing.EXPECT().IsVisible(mock.Anything, RoleSharingType, "role-1", "ou-a1").Return(true, nil).Once()

	role, svcErr := s.service.GetRoleInOU(callerIn("ou-a1"), "role-1", "ou-a1")

	s.Require().Nil(svcErr)
	s.Equal("ou-a1", role.OUID)
	s.False(role.IsOwnedByOU)
}

func (s *OUConfinementTestSuite) TestValidateDeleteRole_RefusedInTheOwningOU() {
	s.storedRole("ou-a")
	s.unitHandle("ou-a")
	s.expectAccess(security.ActionDeleteRole, "ou-a", "role-1", false, nil)

	_, svcErr := s.service.ValidateDeleteRole(callerIn("ou-a"), "role-1")

	s.Equal(&tidcommon.ErrorUnauthorized, svcErr)
}

func (s *OUConfinementTestSuite) TestValidateUpdateRolePermissions_RefusedInTheOwningOU() {
	s.storedRole("ou-a")
	s.unitHandle("ou-a")
	s.expectAccess(security.ActionUpdateRole, "ou-a", "role-1", false, nil)

	_, svcErr := s.service.ValidateUpdateRolePermissions(callerIn("ou-a"), "role-1", nil)

	s.Equal(&tidcommon.ErrorUnauthorized, svcErr)
}

// expectListable answers which organization units the caller may list roles in.
func (s *OUConfinementTestSuite) expectListable(result *sysauthz.AccessibleResources, svcErr *tidcommon.ServiceError) {
	s.authz.On("GetAccessibleResources", mock.Anything, security.ActionListRoles, security.ResourceTypeOU).
		Return(result, svcErr).Once()
}

// rolesInOU makes the store hold the roles an organization unit owns, and the sharing service the roles shared into it.
func (s *OUConfinementTestSuite) rolesInOU(ouID string, owned []Role, shared []Role) {
	s.store.On("GetRoleListCountByOUID", mock.Anything, ouID).Return(len(owned), nil).Once()
	s.store.On("GetRoleListByOUID", mock.Anything, ouID, mock.Anything, mock.Anything).Return(owned, nil).Maybe()
	sharedIDs := make([]string, 0, len(shared))
	for _, r := range shared {
		sharedIDs = append(sharedIDs, r.ID)
	}
	s.sharing.EXPECT().ListVisibleResourceIDs(mock.Anything, RoleSharingType, ouID).Return(sharedIDs, nil).Once()
	s.store.On("GetRolesByIDs", mock.Anything, sharedIDs).Return(shared, nil).Maybe()
	s.ous.On("GetOrganizationUnitHandlesByIDs", mock.Anything, mock.Anything).Return(map[string]string{}, nil).Maybe()
}

func (s *OUConfinementTestSuite) TestGetRoleList_Confinement() {
	owned := []Role{{ID: "role-a1", OUID: "ou-a1"}}
	shared := []Role{{ID: "role-a", OUID: "ou-a"}}
	tests := []struct {
		name      string
		filters   map[string]interface{}
		listable  *sysauthz.AccessibleResources
		authzErr  *tidcommon.ServiceError
		arrange   func()
		wantIDs   []string
		wantError *tidcommon.ServiceError
	}{
		{
			name:     "AllAllowedUnfiltered",
			listable: &sysauthz.AccessibleResources{AllAllowed: true},
			arrange: func() {
				s.store.On("GetRoleListCount", mock.Anything).Return(2, nil).Once()
				s.store.On("GetRoleList", mock.Anything, 10, 0).
					Return([]Role{{ID: "role-a", OUID: "ou-a"}, {ID: "role-b", OUID: "ou-b"}}, nil).Once()
				s.ous.On("GetOrganizationUnitHandlesByIDs", mock.Anything, mock.Anything).
					Return(map[string]string{}, nil).Maybe()
			},
			wantIDs: []string{"role-a", "role-b"},
		},
		{
			name:     "ConfinedUnfiltered",
			listable: &sysauthz.AccessibleResources{IDs: []string{"ou-a1"}},
			arrange:  func() { s.rolesInOU("ou-a1", owned, shared) },
			wantIDs:  []string{"role-a1", "role-a"},
		},
		{
			name:     "ConfinedFilteredOwnOU",
			filters:  map[string]interface{}{"ouId": "ou-a1"},
			listable: &sysauthz.AccessibleResources{IDs: []string{"ou-a1"}},
			arrange:  func() { s.rolesInOU("ou-a1", owned, shared) },
			wantIDs:  []string{"role-a1", "role-a"},
		},
		{
			name:     "ConfinedFilteredOtherOU",
			filters:  map[string]interface{}{"ouId": "ou-b"},
			listable: &sysauthz.AccessibleResources{IDs: []string{"ou-a1"}},
			wantIDs:  []string{},
		},
		{
			name:      "AccessibilityError",
			authzErr:  &tidcommon.InternalServerError,
			wantError: &tidcommon.InternalServerError,
		},
	}
	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.SetupTest()
			s.expectListable(tt.listable, tt.authzErr)
			if tt.arrange != nil {
				tt.arrange()
			}

			list, svcErr := s.service.GetRoleList(callerIn("ou-a1"), 10, 0, tt.filters)

			if tt.wantError != nil {
				s.Equal(tt.wantError, svcErr)
				return
			}
			s.Require().Nil(svcErr)
			ids := make([]string, 0, len(list.Roles))
			for _, r := range list.Roles {
				ids = append(ids, r.ID)
			}
			s.Equal(tt.wantIDs, ids)
		})
	}
}

func (s *OUConfinementTestSuite) TestGetRoleList_ConfinedSharedItemCarriesTheListedOU() {
	s.expectListable(&sysauthz.AccessibleResources{IDs: []string{"ou-a1"}}, nil)
	s.rolesInOU("ou-a1", nil, []Role{{ID: "role-a", OUID: "ou-a"}})

	list, svcErr := s.service.GetRoleList(callerIn("ou-a1"), 10, 0, nil)

	s.Require().Nil(svcErr)
	s.Require().Len(list.Roles, 1)
	s.Equal("ou-a1", list.Roles[0].OUID)
	s.False(list.Roles[0].IsOwnedByOU)
	s.True(list.Roles[0].IsReadOnly)
}

func (s *OUConfinementTestSuite) TestGetRoleListByProject_Confinement() {
	s.Run("ConfinedUnfiltered", func() {
		s.SetupTest()
		s.expectListable(&sysauthz.AccessibleResources{IDs: []string{"ou-a1"}}, nil)
		s.store.On("GetRoleListCountByProject", mock.Anything, "project-1", "ou-a1").Return(1, nil).Once()
		s.store.On("GetRoleListByProject", mock.Anything, "project-1", "ou-a1", 10, 0).
			Return([]Role{{ID: "role-a1", OUID: "ou-a1"}}, nil).Once()
		s.ous.On("GetOrganizationUnitHandlesByIDs", mock.Anything, mock.Anything).
			Return(map[string]string{}, nil).Maybe()

		list, svcErr := s.service.GetRoleListByProject(callerIn("ou-a1"), "project-1", 10, 0, nil)

		s.Require().Nil(svcErr)
		s.Equal(1, list.TotalResults)
	})
	s.Run("ConfinedFilteredOtherOU", func() {
		s.SetupTest()
		s.expectListable(&sysauthz.AccessibleResources{IDs: []string{"ou-a1"}}, nil)

		list, svcErr := s.service.GetRoleListByProject(callerIn("ou-a1"), "project-1", 10, 0,
			map[string]interface{}{"ouId": "ou-b"})

		s.Require().Nil(svcErr)
		s.Equal(0, list.TotalResults)
		s.Empty(list.Roles)
	})
}

// assignmentService returns an assignment service over the suite's mocks, whose grant guard lets every caller
// through so the organization unit check decides.
func (s *OUConfinementTestSuite) assignmentService() RoleAssignmentServiceInterface {
	s.authz.On("CanGrantMembership", mock.Anything, mock.Anything, mock.Anything).
		Return((*tidcommon.ServiceError)(nil)).Maybe()
	return newRoleAssignmentService(s.store, nil, nil, nil, nil, &fakeTransactioner{}, s.authz, s.sharing,
		discardingBus{})
}

func (s *OUConfinementTestSuite) TestGetRoleAssignments_ReadInTheNamedOU() {
	s.storedRole("ou-a")
	s.expectAccess(security.ActionReadRole, "ou-a1", "role-1", false, nil)

	_, svcErr := s.assignmentService().GetRoleAssignments(callerIn("ou-b"), "role-1", "ou-a1", 10, 0, false)

	s.Equal(&tidcommon.ErrorUnauthorized, svcErr)
}

func (s *OUConfinementTestSuite) TestGetRoleAssignments_DefaultsToTheOwner() {
	s.storedRole("ou-a")
	s.expectAccess(security.ActionReadRole, "ou-a", "role-1", false, nil)

	_, svcErr := s.assignmentService().GetRoleAssignments(callerIn("ou-b"), "role-1", "", 10, 0, false)

	s.Equal(&tidcommon.ErrorUnauthorized, svcErr)
}

func (s *OUConfinementTestSuite) TestAddAssignments_RefusedWithoutUpdateInTheOU() {
	s.storedRole("ou-a")
	s.expectAccess(security.ActionUpdateRole, "ou-a1", "role-1", false, nil)

	svcErr := s.assignmentService().AddAssignments(callerIn("ou-b"), "role-1", "ou-a1",
		[]RoleAssignment{{ID: "user-1", Type: AssigneeTypeUser}})

	s.Equal(&tidcommon.ErrorUnauthorized, svcErr)
	s.store.AssertNotCalled(s.T(), "AddAssignments", mock.Anything, mock.Anything, mock.Anything)
}

func (s *OUConfinementTestSuite) TestRemoveAssignments_RefusedWithoutUpdateInTheOwner() {
	s.storedRole("ou-a")
	s.expectAccess(security.ActionUpdateRole, "ou-a", "role-1", false, nil)

	svcErr := s.assignmentService().RemoveAssignments(callerIn("ou-b"), "role-1", "",
		[]RoleAssignment{{ID: "user-1", Type: AssigneeTypeUser}})

	s.Equal(&tidcommon.ErrorUnauthorized, svcErr)
	s.store.AssertNotCalled(s.T(), "RemoveAssignments", mock.Anything, mock.Anything, mock.Anything)
}

func (s *OUConfinementTestSuite) TestValidateRemoveAssignment_Refused() {
	s.storedRole("ou-a")
	s.expectAccess(security.ActionUpdateRole, "ou-a", "role-1", false, nil)

	_, svcErr := s.assignmentService().ValidateRemoveAssignment(callerIn("ou-b"), "role-1", "", "user-1")

	s.Equal(&tidcommon.ErrorUnauthorized, svcErr)
}

func (s *OUConfinementTestSuite) TestListAssignableRoles_EmptyWithoutUpdate() {
	s.expectAccess(security.ActionUpdateRole, "ou-b", "", false, nil)

	roles, svcErr := s.assignmentService().ListAssignableRoles(callerIn("ou-a"), "ou-b")

	s.Require().Nil(svcErr)
	s.Empty(roles)
}

func (s *OUConfinementTestSuite) TestAddAssigneesToRoles_RefusedForAnyAssignmentOU() {
	s.storedRole("ou-a")
	s.expectAccess(security.ActionUpdateRole, "ou-b", "role-1", false, nil)

	svcErr := s.assignmentService().AddAssigneesToRoles(callerIn("ou-a"),
		[]RoleAssignment{{ID: "user-1", Type: AssigneeTypeUser, OUID: "ou-b"}}, []string{"role-1"})

	s.Equal(&tidcommon.ErrorUnauthorized, svcErr)
	s.store.AssertNotCalled(s.T(), "AddAssignments", mock.Anything, mock.Anything, mock.Anything)
}

// A caller allowed to create roles in an organization unit may not read the unit itself, so the role
// service reads it as the runtime.
func (s *OUConfinementTestSuite) TestCreateRole_ReadsTheOUAsTheRuntime() {
	s.expectAccess(security.ActionCreateRole, "ou-a1", "", true, nil)
	s.ous.On("GetOrganizationUnit", runtimeContext, "ou-a1").
		Return(oupkg.OrganizationUnit{}, &oupkg.ErrorOrganizationUnitNotFound).Once()

	_, svcErr := s.service.CreateRole(callerIn("ou-a1"), RoleCreationDetail{Name: "auditor", OUID: "ou-a1"})

	s.Equal(&ErrorOrganizationUnitNotFound, svcErr)
}
