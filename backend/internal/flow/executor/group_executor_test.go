// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/revocation"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type GroupExecutorTestSuite struct {
	suite.Suite
	factory core.FlowFactoryInterface
	groups  *groupAdminProviderMock
}

func TestGroupExecutorTestSuite(t *testing.T) {
	suite.Run(t, new(GroupExecutorTestSuite))
}

func (s *GroupExecutorTestSuite) SetupTest() {
	s.factory = newAdministrationTestFactory(s.T())
	s.groups = newGroupAdminProviderMock(s.T())
}

func groupPlan(assigneeID string) revocationPlan {
	plan := revocationPlan{
		Mode:       revocation.ModeBeforeAction,
		Reason:     revocation.ReasonGroupDeleted,
		TargetID:   testGroupID,
		AssigneeID: assigneeID,
		Criteria:   []revocation.Criterion{{Type: revocation.CriterionTypeEntityScope, Value: "digest"}},
	}
	if assigneeID != "" {
		plan.Reason = revocation.ReasonGroupMembershipRemoved
		plan.AssigneeType = string(group.MemberTypeUser)
	}
	return plan
}

func (s *GroupExecutorTestSuite) TestMetaDeclaresEveryModeAndNoDefault() {
	meta := newGroupExecutor(s.factory, s.groups).GetMeta()

	s.Require().NotNil(meta)
	s.Empty(meta.DefaultMode)
	s.Equal([]string{ExecutorModeGroupDeletion, ExecutorModeGroupMembershipRemoval}, meta.SupportedModes)
}

func (s *GroupExecutorTestSuite) TestUnknownModeIsAnError() {
	for _, mode := range []string{"", ExecutorModeRoleAssignmentRemoval} {
		s.Run(mode, func() {
			_, err := newGroupExecutor(s.factory, s.groups).Execute(
				withRevocationPlan(s.T(), mode, groupPlan(testMemberID)))

			s.Require().Error(err)
			s.Contains(err.Error(), "unsupported mode")
		})
	}
	s.groups.AssertNotCalled(s.T(), "DeleteGroup", mock.Anything, mock.Anything)
}

func (s *GroupExecutorTestSuite) TestRemoveMember_ConsumesTheValidatorPlan() {
	roles := newRoleAdminProviderMock(s.T())
	target := oneCaliforniaScope(testMemberID)
	target.AssigneeType = string(group.MemberTypeAgent)
	roles.On("ValidateGroupMembershipRemoval", mock.Anything, testGroupID, testMemberID).Return(target, nil)
	s.groups.On("RemoveGroupMembers", mock.Anything, testGroupID,
		[]group.Member{{ID: testMemberID, Type: group.MemberTypeAgent}}).Return(&group.Group{ID: testGroupID}, nil)

	pre, err := newAccessChangeValidator(s.factory, roles, nil, nil).Execute(
		administrationNodeContext(ExecutorModeGroupMembershipRemoval,
			map[string]string{revocationInputGroup: testGroupID, revocationInputMember: testMemberID}))
	s.Require().NoError(err)
	s.Require().Equal(providers.ExecComplete, pre.Status)

	ctx := administrationNodeContext(ExecutorModeGroupMembershipRemoval, nil)
	ctx.SharedRuntimeData = pre.SharedRuntimeData
	resp, err := newGroupExecutor(s.factory, s.groups).Execute(ctx)

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
}

func (s *GroupExecutorTestSuite) TestRemoveMember_RefusesAPlanWithoutAMember() {
	plan := groupPlan("")
	plan.Reason = revocation.ReasonGroupMembershipRemoved

	_, err := newGroupExecutor(s.factory, s.groups).Execute(
		withRevocationPlan(s.T(), ExecutorModeGroupMembershipRemoval, plan))

	s.Require().Error(err)
	s.Contains(err.Error(), "no departing member")
	s.groups.AssertNotCalled(s.T(), "DeleteGroup", mock.Anything, mock.Anything)
}

func (s *GroupExecutorTestSuite) TestRemoveMember_RefusesAPlanWithoutAMemberType() {
	plan := groupPlan(testMemberID)
	plan.AssigneeType = ""

	_, err := newGroupExecutor(s.factory, s.groups).Execute(
		withRevocationPlan(s.T(), ExecutorModeGroupMembershipRemoval, plan))

	s.Require().Error(err)
	s.Contains(err.Error(), "no departing member")
	s.groups.AssertNotCalled(s.T(), "RemoveGroupMembers", mock.Anything, mock.Anything, mock.Anything)
}

func (s *GroupExecutorTestSuite) TestDelete_RefusesAPlanNamingAMember() {
	plan := groupPlan(testMemberID)
	plan.Reason = revocation.ReasonGroupDeleted

	_, err := newGroupExecutor(s.factory, s.groups).Execute(
		withRevocationPlan(s.T(), ExecutorModeGroupDeletion, plan))

	s.Require().Error(err)
	s.Contains(err.Error(), "names a departing member")
	s.groups.AssertNotCalled(s.T(), "DeleteGroup", mock.Anything, mock.Anything)
}

func (s *GroupExecutorTestSuite) TestDelete_ConsumesTheValidatorPlan() {
	roles := newRoleAdminProviderMock(s.T())
	roles.On("ValidateGroupDeletion", mock.Anything, testGroupID).
		Return(oneCaliforniaScope("member-1", "member-2"), nil)
	s.groups.On("DeleteGroup", mock.Anything, testGroupID).Return(nil)

	pre, err := newAccessChangeValidator(s.factory, roles, nil, nil).Execute(
		administrationNodeContext(ExecutorModeGroupDeletion, map[string]string{revocationInputGroup: testGroupID}))
	s.Require().NoError(err)
	s.Require().Equal(providers.ExecComplete, pre.Status)

	ctx := administrationNodeContext(ExecutorModeGroupDeletion, nil)
	ctx.SharedRuntimeData = pre.SharedRuntimeData
	resp, err := newGroupExecutor(s.factory, s.groups).Execute(ctx)

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
}

func (s *GroupExecutorTestSuite) TestDelete_DeletesTheGroup() {
	s.groups.On("DeleteGroup", mock.Anything, testGroupID).Return(nil)

	resp, err := newGroupExecutor(s.factory, s.groups).Execute(
		withRevocationPlan(s.T(), ExecutorModeGroupDeletion, groupPlan("")))

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
}

func (s *GroupExecutorTestSuite) TestEveryModeRejectsAForeignPlan() {
	foreign := map[string]revocation.Reason{
		ExecutorModeGroupDeletion:          revocation.ReasonGroupMembershipRemoved,
		ExecutorModeGroupMembershipRemoval: revocation.ReasonGroupDeleted,
	}
	s.Len(foreign, len(groupActionsByMode), "every mode must be covered")
	for mode, reason := range foreign {
		s.Run(mode, func() {
			plan := groupPlan(testMemberID)
			plan.Reason = reason
			_, err := newGroupExecutor(s.factory, s.groups).Execute(withRevocationPlan(s.T(), mode, plan))

			s.Require().Error(err)
			s.Contains(err.Error(), "was produced for")
		})
	}
}

func (s *GroupExecutorTestSuite) TestUncodedRefusalFallsBackToTheModeError() {
	uncoded := &tidcommon.ServiceError{Type: tidcommon.ClientErrorType}
	s.groups.On("DeleteGroup", mock.Anything, mock.Anything).Return(uncoded)
	s.groups.On("RemoveGroupMembers", mock.Anything, mock.Anything, mock.Anything).Return(nil, uncoded)

	cases := []struct {
		mode string
		plan revocationPlan
		want tidcommon.ServiceError
	}{
		{ExecutorModeGroupDeletion, groupPlan(""), ErrGroupDeletionFailed},
		{ExecutorModeGroupMembershipRemoval, groupPlan(testMemberID), ErrGroupMembershipRemovalFailed},
	}
	for _, tc := range cases {
		s.Run(tc.mode, func() {
			resp, err := newGroupExecutor(s.factory, s.groups).Execute(withRevocationPlan(s.T(), tc.mode, tc.plan))

			s.Require().NoError(err)
			s.Equal(providers.ExecFailure, resp.Status)
			s.Require().NotNil(resp.Error)
			s.Equal(tc.want.Code, resp.Error.Code)
		})
	}
}

func (s *GroupExecutorTestSuite) TestServerErrorFailsTheFlow() {
	s.groups.On("DeleteGroup", mock.Anything, testGroupID).Return(&tidcommon.InternalServerError)

	_, err := newGroupExecutor(s.factory, s.groups).Execute(
		withRevocationPlan(s.T(), ExecutorModeGroupDeletion, groupPlan("")))

	s.Require().Error(err)
	s.Contains(err.Error(), "failed to perform group_deleted")
}

func (s *GroupExecutorTestSuite) TestRefusesWithoutItsSeam() {
	_, err := newGroupExecutor(s.factory, nil).Execute(administrationNodeContext(ExecutorModeGroupDeletion, nil))

	s.Require().Error(err)
	s.EqualError(err, "group service is not configured")
}
