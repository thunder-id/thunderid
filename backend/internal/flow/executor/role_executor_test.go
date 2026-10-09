// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/internal/role"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type RoleExecutorTestSuite struct {
	suite.Suite
	factory     core.FlowFactoryInterface
	roles       *roleAdminProviderMock
	assignments *roleAssignmentAdminProviderMock
}

func TestRoleExecutorTestSuite(t *testing.T) {
	suite.Run(t, new(RoleExecutorTestSuite))
}

func (s *RoleExecutorTestSuite) SetupTest() {
	s.factory = newAdministrationTestFactory(s.T())
	s.roles = newRoleAdminProviderMock(s.T())
	s.assignments = newRoleAssignmentAdminProviderMock(s.T())
}

func (s *RoleExecutorTestSuite) executor() *roleExecutor {
	return newRoleExecutor(s.factory, s.roles, s.assignments)
}

func rolePlan(reason revocation.Reason) revocationPlan {
	return revocationPlan{
		Mode:     revocation.ModeBeforeAction,
		Reason:   reason,
		TargetID: testRoleID,
		Cutoff:   time.Now().UTC(),
		Criteria: []revocation.Criterion{{
			Type:  revocation.CriterionTypeEntityScope,
			Value: revocation.EntityScopeCriterionValue(testAssigneeID, testCaliforniaAud, testLicenceScope),
		}},
	}
}

// No default mode is declared, so flow validation requires every node to name the change it makes.
func (s *RoleExecutorTestSuite) TestMetaDeclaresEveryModeAndNoDefault() {
	meta := s.executor().GetMeta()

	s.Require().NotNil(meta)
	s.Empty(meta.DefaultMode)
	s.Equal([]string{ExecutorModeDelete, ExecutorModeRemoveAssignment, ExecutorModeRemovePermissions},
		meta.SupportedModes)
}

func (s *RoleExecutorTestSuite) TestUnknownModeIsAnError() {
	for _, mode := range []string{"", ExecutorModeRemoveMember} {
		s.Run(mode, func() {
			_, err := s.executor().Execute(
				withRevocationPlan(s.T(), mode, rolePlan(revocation.ReasonRoleDeleted)))

			s.Require().Error(err)
			s.Contains(err.Error(), "unsupported mode")
		})
	}
	s.roles.AssertNotCalled(s.T(), "DeleteRole", mock.Anything, mock.Anything)
}

func (s *RoleExecutorTestSuite) TestRemoveAssignment_ConsumesTheValidatorPlan() {
	target := oneCaliforniaScope(testAssigneeID)
	target.AssigneeType = string(role.AssigneeTypeApp)
	s.assignments.On("ValidateRemoveAssignment", mock.Anything, testRoleID, testAssigneeID).Return(target, nil)
	s.assignments.On("RemoveAssignments", mock.Anything, testRoleID,
		[]role.RoleAssignment{{ID: testAssigneeID, Type: role.AssigneeTypeApp}}).Return(nil)

	pre, err := newAccessChangeValidator(s.factory, s.roles, s.assignments,
		nil).Execute(
		administrationNodeContext(ExecutorModeRoleAssignmentRemoval,
			map[string]string{revocationInputRole: testRoleID, revocationInputAssignee: testAssigneeID}))
	s.Require().NoError(err)
	s.Require().Equal(providers.ExecComplete, pre.Status)

	ctx := administrationNodeContext(ExecutorModeRemoveAssignment, nil)
	ctx.SharedRuntimeData = pre.SharedRuntimeData
	resp, err := s.executor().Execute(ctx)

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
}

func (s *RoleExecutorTestSuite) TestRemoveAssignment_RefusesAPlanWithoutAnAssignee() {
	_, err := s.executor().Execute(
		withRevocationPlan(s.T(), ExecutorModeRemoveAssignment, rolePlan(revocation.ReasonRoleAssignmentRemoved)))

	s.Require().Error(err)
	s.Contains(err.Error(), "no target assignee")
}

func (s *RoleExecutorTestSuite) TestRemoveAssignment_RefusesAPlanWithoutAnAssigneeType() {
	plan := rolePlan(revocation.ReasonRoleAssignmentRemoved)
	plan.AssigneeID = testAssigneeID

	_, err := s.executor().Execute(withRevocationPlan(s.T(), ExecutorModeRemoveAssignment, plan))

	s.Require().Error(err)
	s.Contains(err.Error(), "no target assignee")
	s.assignments.AssertNotCalled(s.T(), "RemoveAssignments", mock.Anything, mock.Anything, mock.Anything)
}

func (s *RoleExecutorTestSuite) TestRemovePermissions_CarriesARefusedRoleRead() {
	s.roles.On("GetRoleWithPermissions", mock.Anything, testRoleID).Return(nil, &role.ErrorRoleNotFound)
	plan := rolePlan(revocation.ReasonRolePermissionRemoved)
	plan.ActionArgs = map[string]string{revocationInputPermissions: testPermissionSet}

	resp, err := s.executor().Execute(withRevocationPlan(s.T(), ExecutorModeRemovePermissions, plan))

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, resp.Status)
	s.Require().NotNil(resp.Error)
	s.Equal(role.ErrorRoleNotFound.Code, resp.Error.Code)
	s.roles.AssertNotCalled(s.T(), "UpdateRoleWithPermissions", mock.Anything, mock.Anything, mock.Anything)
}

func (s *RoleExecutorTestSuite) TestDelete_DeletesTheRole() {
	s.roles.On("DeleteRole", mock.Anything, testRoleID).Return(nil)

	resp, err := s.executor().Execute(
		withRevocationPlan(s.T(), ExecutorModeDelete, rolePlan(revocation.ReasonRoleDeleted)))

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
}

func (s *RoleExecutorTestSuite) TestRemovePermissions_AppliesTheCarriedSet() {
	s.roles.On("GetRoleWithPermissions", mock.Anything, testRoleID).Return(&role.RoleWithPermissions{
		ID: testRoleID, Name: "DMV Clerk", Description: "Counter staff", OUID: "ou-1",
		Permissions: []role.ResourcePermissions{
			{ResourceServerID: testRSID, Permissions: []string{testLicenceScope, "permits"}}},
	}, nil)
	// The update replaces the whole role, so the other attributes must be carried over, not sent empty.
	s.roles.On("UpdateRoleWithPermissions", mock.Anything, testRoleID, role.RoleUpdateDetail{
		Name: "DMV Clerk", Description: "Counter staff", OUID: "ou-1",
		Permissions: []role.ResourcePermissions{{ResourceServerID: testRSID, Permissions: []string{testLicenceScope}}},
	}).Return(&role.RoleWithPermissions{ID: testRoleID}, nil)
	plan := rolePlan(revocation.ReasonRolePermissionRemoved)
	plan.ActionArgs = map[string]string{revocationInputPermissions: testPermissionSet}

	resp, err := s.executor().Execute(
		withRevocationPlan(s.T(), ExecutorModeRemovePermissions, plan))

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
}

func (s *RoleExecutorTestSuite) TestRemovePermissions_AppliesAnEmptySet() {
	s.roles.On("GetRoleWithPermissions", mock.Anything, testRoleID).
		Return(&role.RoleWithPermissions{ID: testRoleID, Name: "DMV Clerk", OUID: "ou-1"}, nil)
	s.roles.On("UpdateRoleWithPermissions", mock.Anything, testRoleID, role.RoleUpdateDetail{
		Name: "DMV Clerk", OUID: "ou-1", Permissions: []role.ResourcePermissions{},
	}).Return(&role.RoleWithPermissions{ID: testRoleID}, nil)
	plan := rolePlan(revocation.ReasonRolePermissionRemoved)
	plan.ActionArgs = map[string]string{revocationInputPermissions: "[]"}

	resp, err := s.executor().Execute(
		withRevocationPlan(s.T(), ExecutorModeRemovePermissions, plan))

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
}

func (s *RoleExecutorTestSuite) TestRemovePermissions_MalformedCarriedSetIsAnError() {
	plan := rolePlan(revocation.ReasonRolePermissionRemoved)
	plan.ActionArgs = map[string]string{revocationInputPermissions: "{not json"}

	_, err := s.executor().Execute(
		withRevocationPlan(s.T(), ExecutorModeRemovePermissions, plan))

	s.Require().Error(err)
	s.Contains(err.Error(), "failed to decode role permissions")
	s.roles.AssertNotCalled(s.T(), "UpdateRoleWithPermissions", mock.Anything, mock.Anything, mock.Anything)
}

// Nothing in flow validation stops a validator in one mode being wired to a role node in another.
func (s *RoleExecutorTestSuite) TestEveryModeRejectsAForeignPlan() {
	foreign := map[string]revocation.Reason{
		ExecutorModeRemoveAssignment:  revocation.ReasonRoleDeleted,
		ExecutorModeDelete:            revocation.ReasonRoleAssignmentRemoved,
		ExecutorModeRemovePermissions: revocation.ReasonApplicationDeleted,
	}
	s.Len(foreign, len(roleActionsByMode), "every mode must be covered")
	for mode, reason := range foreign {
		s.Run(mode, func() {
			_, err := s.executor().Execute(
				withRevocationPlan(s.T(), mode, rolePlan(reason)))

			s.Require().Error(err)
			s.Contains(err.Error(), "was produced for")
		})
	}
}

func (s *RoleExecutorTestSuite) TestUncodedRefusalFallsBackToTheModeError() {
	uncoded := &tidcommon.ServiceError{Type: tidcommon.ClientErrorType}
	s.assignments.On("RemoveAssignments", mock.Anything, mock.Anything, mock.Anything).Return(uncoded)
	s.roles.On("DeleteRole", mock.Anything, mock.Anything).Return(uncoded)
	s.roles.On("GetRoleWithPermissions", mock.Anything, mock.Anything).
		Return(&role.RoleWithPermissions{ID: testRoleID}, nil)
	s.roles.On("UpdateRoleWithPermissions", mock.Anything, mock.Anything, mock.Anything).Return(nil, uncoded)

	assignment := rolePlan(revocation.ReasonRoleAssignmentRemoved)
	assignment.AssigneeID = testAssigneeID
	assignment.AssigneeType = string(role.AssigneeTypeUser)
	cases := []struct {
		mode string
		plan revocationPlan
		want tidcommon.ServiceError
	}{
		{ExecutorModeRemoveAssignment, assignment, ErrRoleAssignmentRemovalFailed},
		{ExecutorModeDelete, rolePlan(revocation.ReasonRoleDeleted), ErrRoleDeletionFailed},
		{ExecutorModeRemovePermissions, rolePlan(revocation.ReasonRolePermissionRemoved),
			ErrRolePermissionRemovalFailed},
	}
	for _, tc := range cases {
		s.Run(tc.mode, func() {
			resp, err := s.executor().Execute(withRevocationPlan(s.T(), tc.mode, tc.plan))

			s.Require().NoError(err)
			s.Equal(providers.ExecFailure, resp.Status)
			s.Require().NotNil(resp.Error)
			s.Equal(tc.want.Code, resp.Error.Code)
		})
	}
}

func (s *RoleExecutorTestSuite) TestServerErrorFailsTheFlow() {
	s.roles.On("DeleteRole", mock.Anything, testRoleID).Return(&tidcommon.InternalServerError)

	_, err := s.executor().Execute(
		withRevocationPlan(s.T(), ExecutorModeDelete, rolePlan(revocation.ReasonRoleDeleted)))

	s.Require().Error(err)
	s.Contains(err.Error(), "failed to perform")
}

func (s *RoleExecutorTestSuite) TestRefusesWithoutItsSeam() {
	_, err := newRoleExecutor(s.factory, nil, nil).Execute(administrationNodeContext(ExecutorModeDelete, nil))

	s.Require().Error(err)
	s.EqualError(err, "role service is not configured")
}
