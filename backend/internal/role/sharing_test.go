// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
)

const testOwnerOU = "ou-owner"

type RoleSharingTestSuite struct {
	suite.Suite
	mockService           *RoleServiceInterfaceMock
	mockAssignmentService *RoleAssignmentServiceInterfaceMock
	mockStore             *roleStoreInterfaceMock
	mockOUService         *oumock.OrganizationUnitServiceInterfaceMock
	decl                  *roleSharingDeclaration
	ctx                   context.Context
}

func TestRoleSharingTestSuite(t *testing.T) {
	suite.Run(t, new(RoleSharingTestSuite))
}

func (suite *RoleSharingTestSuite) SetupTest() {
	suite.mockService = NewRoleServiceInterfaceMock(suite.T())
	suite.mockAssignmentService = NewRoleAssignmentServiceInterfaceMock(suite.T())
	suite.mockStore = newRoleStoreInterfaceMock(suite.T())
	suite.mockOUService = oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())
	suite.decl = newRoleSharingDeclaration(
		suite.mockService, suite.mockAssignmentService, suite.mockStore, suite.mockOUService)
	suite.ctx = context.Background()
}

// A role declares its assignments alone, open to every organization unit it reaches unless a
// policy says otherwise, so a policy need not mention them to allow them. A policy only switches
// them on or off.
func (suite *RoleSharingTestSuite) TestDeclaration_NamesRolesAndDeclaresAssignments() {
	decl := suite.decl

	suite.Equal(RoleSharingType, decl.ResourceType())
	suite.Equal([]sharing.FieldDeclaration{{
		Key:          "assignments",
		Kind:         sharing.FieldReferenceSet,
		EditableOnly: true,
		Default:      &sharing.OverlayRule{Editable: true},
	}}, decl.Fields())
}

func (suite *RoleSharingTestSuite) TestDeclaration_OwningOUID() {
	decl := suite.decl

	suite.mockService.EXPECT().GetRoleWithPermissions(mock.MatchedBy(security.IsRuntimeContext), "role1").
		Return(&RoleWithPermissions{ID: "role1", OUID: testOwnerOU}, nil).Once()
	ouID, svcErr := decl.OwningOUID(suite.ctx, "role1")
	suite.Nil(svcErr)
	suite.Equal(testOwnerOU, ouID)

	suite.mockService.EXPECT().GetRoleWithPermissions(mock.Anything, "missing").
		Return(nil, &ErrorRoleNotFound).Once()
	_, svcErr = decl.OwningOUID(suite.ctx, "missing")
	suite.Require().NotNil(svcErr)
	suite.Equal(ErrorRoleNotFound.Code, svcErr.Code)

	suite.mockService.EXPECT().GetRoleWithPermissions(mock.Anything, "broken").
		Return(nil, &tidcommon.InternalServerError).Once()
	_, svcErr = decl.OwningOUID(suite.ctx, "broken")
	suite.Require().NotNil(svcErr)
	suite.Equal(tidcommon.InternalServerError.Code, svcErr.Code)
}

// Once a role is no longer shared with an organization unit, the assignments made there go.
func (suite *RoleSharingTestSuite) TestDeclaration_DeleteOverlayValuesDropsTheOUsAssignments() {
	decl := suite.decl
	suite.mockStore.On("ListMandatedAssignmentsForRole", mock.Anything, "role1", mock.Anything).
		Return([]RoleAssignment{}, nil)
	suite.mockAssignmentService.EXPECT().DeleteAssignmentsInOU(mock.Anything, "role1", "ou-child").Return(nil).Once()
	suite.NoError(decl.DeleteOverlayValues(suite.ctx, "role1", "ou-child"))

	suite.mockAssignmentService.EXPECT().DeleteAssignmentsInOU(mock.Anything, "role1", "ou-gone").
		Return(errors.New("store down")).Once()
	suite.Error(decl.DeleteOverlayValues(suite.ctx, "role1", "ou-gone"))
}

// While a user sharing policy mandates the role in the organization unit, losing the role there is
// refused through the framework, which answers the change with ErrorRoleMandatedInOU.
func (suite *RoleSharingTestSuite) TestDeclaration_DeleteOverlayValuesRefusesWhileMandated() {
	suite.mockStore.On("ListMandatedAssignmentsForRole", mock.Anything, "role1", "ou-child").Return([]RoleAssignment{
		{ID: "user1", Type: assigneeTypeEntity, OUID: "ou-child", PolicyID: "user-policy"},
	}, nil)
	suite.mockOUService.EXPECT().IsOrganizationUnitExists(mock.Anything, "ou-child").Return(true, nil)

	err := suite.decl.DeleteOverlayValues(suite.ctx, "role1", "ou-child")

	var refusal *sharing.ResourceTypeError
	suite.Require().ErrorAs(err, &refusal)
	suite.Equal(sharing.NewResourceTypeError(&ErrorRoleMandatedInOU), err)
	suite.mockAssignmentService.AssertNotCalled(suite.T(), "DeleteAssignmentsInOU",
		mock.Anything, mock.Anything, mock.Anything)
}

// An organization unit deleted since cannot keep the role, so its mandated assignments go with the rest
// rather than leaving every later change to the role's sharing refused there.
func (suite *RoleSharingTestSuite) TestDeclaration_DeleteOverlayValuesClearsADeletedOU() {
	suite.mockStore.On("ListMandatedAssignmentsForRole", mock.Anything, "role1", "ou-gone").Return([]RoleAssignment{
		{ID: "user1", Type: assigneeTypeEntity, OUID: "ou-gone", PolicyID: "user-policy"},
	}, nil)
	suite.mockOUService.EXPECT().IsOrganizationUnitExists(mock.Anything, "ou-gone").Return(false, nil)
	suite.mockAssignmentService.EXPECT().DeleteAssignmentsInOU(mock.Anything, "role1", "ou-gone").Return(nil).Once()

	suite.NoError(suite.decl.DeleteOverlayValues(suite.ctx, "role1", "ou-gone"))
}

// A failure to find out whether the role is mandated there fails the cleanup as an internal error.
func (suite *RoleSharingTestSuite) TestDeclaration_DeleteOverlayValuesFailures() {
	suite.mockStore.On("ListMandatedAssignmentsForRole", mock.Anything, "role1", "ou-a").
		Return(nil, errors.New("db down"))
	suite.mockStore.On("ListMandatedAssignmentsForRole", mock.Anything, "role1", "ou-b").Return([]RoleAssignment{
		{ID: "user1", Type: assigneeTypeEntity, OUID: "ou-b", PolicyID: "user-policy"},
	}, nil)
	suite.mockOUService.EXPECT().IsOrganizationUnitExists(mock.Anything, "ou-b").
		Return(false, &tidcommon.InternalServerError)

	for _, ouID := range []string{"ou-a", "ou-b"} {
		err := suite.decl.DeleteOverlayValues(suite.ctx, "role1", ouID)

		suite.Require().Error(err, ouID)
		var refusal *sharing.ResourceTypeError
		suite.False(errors.As(err, &refusal), "%s: a failure is not a refusal", ouID)
	}
	suite.mockAssignmentService.AssertNotCalled(suite.T(), "DeleteAssignmentsInOU",
		mock.Anything, mock.Anything, mock.Anything)
}
