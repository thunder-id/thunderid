// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"slices"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/executormeta"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type RoleExecutorTestSuite struct {
	suite.Suite
	factory     core.FlowFactoryInterface
	assignments *roleAssignmentAdminProviderMock
}

func TestRoleExecutorTestSuite(t *testing.T) {
	suite.Run(t, new(RoleExecutorTestSuite))
}

func (s *RoleExecutorTestSuite) SetupTest() {
	s.factory = newAdministrationTestFactory(s.T())
	s.assignments = newRoleAssignmentAdminProviderMock(s.T())
}

func (s *RoleExecutorTestSuite) executor() *roleExecutor {
	return newRoleExecutor(s.factory, s.assignments)
}

// A node must name its mode: selecting roles and assigning them are separate steps, which a default would blur.
func (s *RoleExecutorTestSuite) TestMetaDeclaresEveryModeAndNoDefault() {
	meta := s.executor().GetMeta()

	s.Require().NotNil(meta)
	s.Empty(meta.DefaultMode)
	catalogMeta, ok := executormeta.MetaFor(ExecutorNameRole)
	s.Require().True(ok)
	s.Empty(catalogMeta.DefaultMode)
	s.Equal([]string{ExecutorModeRoleAssignment, ExecutorModeRoleSelection}, meta.SupportedModes)
	s.Equal(catalogMeta.SupportedModes, meta.SupportedModes)
	s.Equal([]providers.FlowType{providers.FlowTypeInvitation, providers.FlowTypeRegistration},
		meta.SupportedFlowTypes)
	s.Equal(catalogMeta.SupportedFlowTypes, meta.SupportedFlowTypes)
}

// A sign-up flow has no admin to pick roles, so it can only assign the ones its node names.
func (s *RoleExecutorTestSuite) TestSelectInASignUpFlowIsAnError() {
	ctx := s.signUpContext("role-1")
	ctx.ExecutorMode = ExecutorModeRoleSelection

	_, err := s.executor().Execute(ctx)

	s.Require().Error(err)
	s.Contains(err.Error(), "is not supported in REGISTRATION flows")
	s.assignments.AssertNotCalled(s.T(), "ListAssignableRoles", mock.Anything, mock.Anything)
}

func (s *RoleExecutorTestSuite) TestAnInvitationModeOutsideAnInvitationFlowIsAnError() {
	for _, mode := range []string{ExecutorModeRoleSelection, ExecutorModeRoleAssignment} {
		s.Run(mode, func() {
			ctx := s.nodeContext(adminWithPermissions(), mode, map[string]string{userInputRoles: "role-1"})
			ctx.FlowType = providers.FlowTypeAdministration

			_, err := s.executor().Execute(ctx)

			s.Require().Error(err)
			s.Contains(err.Error(), "is not supported in ADMINISTRATION flows")
		})
	}
	s.assignments.AssertNotCalled(s.T(), "ListAssignableRoles", mock.Anything, mock.Anything)
}

func (s *RoleExecutorTestSuite) TestAnInvitationModeRefusesWithoutItsSeam() {
	_, err := newRoleExecutor(s.factory, nil).Execute(
		s.nodeContext(adminWithPermissions(), ExecutorModeRoleSelection, map[string]string{}))

	s.EqualError(err, "role assignment service is not configured")
}

func (s *RoleExecutorTestSuite) TestUnknownModeIsAnError() {
	for _, mode := range []string{ExecutorModeRolePermissionRemoval, ExecutorModeGroupMembershipRemoval, ""} {
		s.Run(mode, func() {
			_, err := s.executor().Execute(administrationNodeContext(mode, nil))

			s.Require().Error(err)
			s.Contains(err.Error(), "unsupported mode")
		})
	}
	s.assignments.AssertNotCalled(s.T(), "RemoveAssignments", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything)
}

func (s *RoleExecutorTestSuite) nodeContext(ctx context.Context, mode string,
	inputs map[string]string) *providers.NodeContext {
	return &providers.NodeContext{
		Context:      ctx,
		FlowType:     providers.FlowTypeInvitation,
		ExecutorMode: mode,
		UserInputs:   inputs,
		RuntimeData: map[string]string{
			common.RuntimeKeyOUID:    "ou-a",
			common.RuntimeKeyInviter: "admin-1",
			userAttributeUserID:      "user-1",
		},
	}
}

// adminWithPermissions is the inviter's context, holding the permissions the select step records.
func adminWithPermissions() context.Context {
	return security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest("admin-1", "ou-a", "token", []string{"system:user"},
			map[string]interface{}{}))
}

func (s *RoleExecutorTestSuite) offers(ids ...string) {
	roles := make([]role.Role, 0, len(ids))
	for _, id := range ids {
		roles = append(roles, role.Role{ID: id, Name: "Role " + id})
	}
	s.assignments.EXPECT().ListAssignableRoles(mock.Anything, "ou-a").Return(roles, nil)
}

func (s *RoleExecutorTestSuite) TestSelect_PromptsWithTheAssignableRoles() {
	s.offers("role-1", "role-2")

	resp, err := s.executor().Execute(
		s.nodeContext(adminWithPermissions(), ExecutorModeRoleSelection, map[string]string{}))

	s.Require().NoError(err)
	s.Equal(providers.ExecUserInputRequired, resp.Status)
	s.Require().Len(resp.Inputs, 1)
	s.Equal(providers.Input{
		Identifier: userInputRoles, Type: providers.InputTypeRoleSelect, Required: true,
		Options: []string{"role-1", "role-2"},
	}, resp.Inputs[0])
	s.JSONEq(`[{"id":"role-1","name":"Role role-1"},{"id":"role-2","name":"Role role-2"}]`,
		resp.AdditionalData[common.DataRoleOptions])
}

func (s *RoleExecutorTestSuite) TestSelect_RefusesARoleOutsideTheList() {
	s.offers("role-1")

	resp, err := s.executor().Execute(s.nodeContext(adminWithPermissions(), ExecutorModeRoleSelection,
		map[string]string{userInputRoles: "role-1 role-9"}))

	s.Require().NoError(err)
	s.Equal(providers.ExecUserInputRequired, resp.Status)
	s.Equal(ErrRoleNotAssignable.Code, resp.Error.Code)
	s.NotContains(resp.RuntimeData, common.RuntimeKeySelectedRoles)
}

func (s *RoleExecutorTestSuite) TestSelect_RecordsTheRoles() {
	s.offers("role-1", "role-2")

	resp, err := s.executor().Execute(s.nodeContext(adminWithPermissions(), ExecutorModeRoleSelection,
		map[string]string{userInputRoles: "role-2 role-1 role-2"}))

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
	s.Equal(map[string]string{common.RuntimeKeySelectedRoles: "role-2 role-1"}, resp.RuntimeData)
}

// With nothing the admin may assign, the prompt could never be answered, so no roles are recorded.
func (s *RoleExecutorTestSuite) TestSelect_NothingAssignableRecordsNothing() {
	s.offers()

	resp, err := s.executor().Execute(
		s.nodeContext(adminWithPermissions(), ExecutorModeRoleSelection, map[string]string{}))

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
	s.Empty(resp.RuntimeData)
}

func (s *RoleExecutorTestSuite) TestSelect_RefusesAnyCallerButTheInviter() {
	resp, err := s.executor().Execute(s.nodeContext(security.WithRuntimeContext(context.Background()),
		ExecutorModeRoleSelection, map[string]string{userInputRoles: "role-1"}))

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, resp.Status)
	s.Equal(ErrNotTheInviter.Code, resp.Error.Code)
	s.assignments.AssertNotCalled(s.T(), "ListAssignableRoles", mock.Anything, mock.Anything)
}

// inviteeStep is the context the invitee's step runs with: the admin who started the flow, with the permissions
// they still hold.
func inviteeStep() context.Context {
	return security.WithRecordedCaller(context.Background(), "admin-1", "ou-a", []string{"system:user"})
}

func (s *RoleExecutorTestSuite) recorded(ctx *providers.NodeContext) *providers.NodeContext {
	ctx.RuntimeData[common.RuntimeKeySelectedRoles] = "role-1 role-2"
	return ctx
}

// The role service checks the assignments as the admin the step runs as, and makes all of them or none.
func (s *RoleExecutorTestSuite) TestAssign_AssignsAsTheStepsCaller() {
	asAdmin := mock.MatchedBy(func(ctx context.Context) bool {
		return !security.IsRuntimeContext(ctx) && security.GetSubject(ctx) == "admin-1" &&
			security.GetOUID(ctx) == "ou-a" && slices.Equal(security.GetPermissions(ctx), []string{"system:user"})
	})
	user := []role.RoleAssignment{{ID: "user-1", Type: role.AssigneeTypeUser, OUID: "ou-a"}}
	s.assignments.EXPECT().AddAssigneesToRoles(asAdmin, user, []string{"role-1", "role-2"}).Return(nil).Once()

	resp, err := s.executor().Execute(s.recorded(s.nodeContext(
		inviteeStep(), ExecutorModeRoleAssignment, map[string]string{})))

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
}

func (s *RoleExecutorTestSuite) TestAssign_IgnoresInputsNamedLikeTheRecord() {
	ctx := s.nodeContext(inviteeStep(), ExecutorModeRoleAssignment,
		map[string]string{
			common.RuntimeKeySelectedRoles: "role-9",
			userInputRoles:                 "role-9",
		})

	resp, err := s.executor().Execute(ctx)

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
	s.assignments.AssertNotCalled(s.T(), "AddAssigneesToRoles", mock.Anything, mock.Anything, mock.Anything)
}

// Roles are never assigned with the server's authority.
func (s *RoleExecutorTestSuite) TestAssign_WithTheServersAuthorityIsRefused() {
	ctx := s.recorded(s.nodeContext(security.WithRuntimeContext(context.Background()), ExecutorModeRoleAssignment,
		map[string]string{}))

	resp, err := s.executor().Execute(ctx)

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, resp.Status)
	s.Equal(ErrRoleAssignmentFailed.Code, resp.Error.Code)
	s.assignments.AssertNotCalled(s.T(), "AddAssigneesToRoles", mock.Anything, mock.Anything, mock.Anything)
}

func (s *RoleExecutorTestSuite) TestAssign_AFailureIsReported() {
	s.assignments.EXPECT().AddAssigneesToRoles(mock.Anything, mock.Anything, mock.Anything).Return(
		&tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "ROL-1010"}).Once()

	resp, err := s.executor().Execute(s.recorded(s.nodeContext(
		inviteeStep(), ExecutorModeRoleAssignment, map[string]string{})))

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, resp.Status)
	s.Equal(ErrRoleAssignmentFailed.Code, resp.Error.Code)
}

func (s *RoleExecutorTestSuite) TestAssign_AServerFailureFailsTheFlow() {
	s.assignments.EXPECT().AddAssigneesToRoles(mock.Anything, mock.Anything, mock.Anything).
		Return(&tidcommon.InternalServerError).Once()

	_, err := s.executor().Execute(s.recorded(s.nodeContext(
		inviteeStep(), ExecutorModeRoleAssignment, map[string]string{})))

	s.Require().Error(err)
	s.Contains(err.Error(), "failed to assign the selected roles")
}

func (s *RoleExecutorTestSuite) TestSelect_AFailedListingIsReported() {
	s.assignments.EXPECT().ListAssignableRoles(mock.Anything, "ou-a").Return(nil, &tidcommon.InternalServerError).Once()

	_, err := s.executor().Execute(
		s.nodeContext(adminWithPermissions(), ExecutorModeRoleSelection, map[string]string{}))

	s.Require().Error(err)
	s.Contains(err.Error(), "failed to list assignable roles")
}

func (s *RoleExecutorTestSuite) TestSelect_WithoutTheFlowsUnitIsAnError() {
	ctx := s.nodeContext(adminWithPermissions(), ExecutorModeRoleSelection, map[string]string{})
	delete(ctx.RuntimeData, common.RuntimeKeyOUID)

	_, err := s.executor().Execute(ctx)

	s.Require().Error(err)
	s.assignments.AssertNotCalled(s.T(), "ListAssignableRoles", mock.Anything, mock.Anything)
}

func (s *RoleExecutorTestSuite) TestAssign_WithoutTheFlowsUnitOrUserIsAnError() {
	for _, key := range []string{common.RuntimeKeyOUID, userAttributeUserID} {
		s.Run(key, func() {
			ctx := s.recorded(s.nodeContext(inviteeStep(), ExecutorModeRoleAssignment, map[string]string{}))
			delete(ctx.RuntimeData, key)

			_, err := s.executor().Execute(ctx)

			s.Require().Error(err)
		})
	}
	s.assignments.AssertNotCalled(s.T(), "AddAssigneesToRoles", mock.Anything, mock.Anything, mock.Anything)
}

// configuredSelection is a role selection node whose roles property names the given value.
func (s *RoleExecutorTestSuite) configuredSelection(roles string, inputs map[string]string) *providers.NodeContext {
	ctx := s.nodeContext(adminWithPermissions(), ExecutorModeRoleSelection, inputs)
	ctx.NodeProperties = map[string]interface{}{common.NodePropertyRoles: roles}
	return ctx
}

// The roles the node names are recorded without asking the admin, still only if the admin may assign them.
func (s *RoleExecutorTestSuite) TestSelect_ConfiguredRolesAreRecordedWithoutAPrompt() {
	s.offers("role-1", "role-2")

	resp, err := s.executor().Execute(s.configuredSelection("role-2 role-1 role-2", map[string]string{}))

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
	s.Empty(resp.Inputs)
	s.Equal(map[string]string{common.RuntimeKeySelectedRoles: "role-2 role-1"}, resp.RuntimeData)
}

func (s *RoleExecutorTestSuite) TestSelect_ConfiguredRolesIgnoreTheSubmittedOnes() {
	s.offers("role-1", "role-2")

	resp, err := s.executor().Execute(s.configuredSelection("role-1", map[string]string{userInputRoles: "role-2"}))

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
	s.Equal("role-1", resp.RuntimeData[common.RuntimeKeySelectedRoles])
}

// A configured role the admin may not assign fails the step: the admin has no prompt to pick another.
func (s *RoleExecutorTestSuite) TestSelect_AConfiguredRoleTheAdminCannotAssignFails() {
	for name, offered := range map[string][]string{"another role": {"role-1"}, "nothing": nil} {
		s.Run(name, func() {
			s.SetupTest()
			s.offers(offered...)

			resp, err := s.executor().Execute(s.configuredSelection("role-1 role-9", map[string]string{}))

			s.Require().NoError(err)
			s.Equal(providers.ExecFailure, resp.Status)
			s.Equal(ErrRoleNotAssignable.Code, resp.Error.Code)
			s.Empty(resp.RuntimeData[common.RuntimeKeySelectedRoles])
		})
	}
}

func (s *RoleExecutorTestSuite) TestSelect_AnUnresolvedRolesPropertyIsAnError() {
	_, err := s.executor().Execute(s.configuredSelection("{{ctx(missing)}}", map[string]string{}))

	s.Require().Error(err)
	s.assignments.AssertNotCalled(s.T(), "ListAssignableRoles", mock.Anything, mock.Anything)
}

// signUpContext is a sign-up flow's role assignment node naming the given roles, after the flow created the
// organization unit and the user.
func (s *RoleExecutorTestSuite) signUpContext(roles string) *providers.NodeContext {
	return &providers.NodeContext{
		Context:        security.WithRuntimeContext(context.Background()),
		FlowType:       providers.FlowTypeRegistration,
		ExecutorMode:   ExecutorModeRoleAssignment,
		NodeProperties: map[string]interface{}{common.NodePropertyRoles: roles},
		UserInputs:     map[string]string{},
		RuntimeData: map[string]string{
			common.RuntimeKeyOUID: "org-1",
			userAttributeUserID:   "user-1",
		},
	}
}

// A sign-up assigns the roles its node names to the new user in the flow's unit, with no admin to record.
func (s *RoleExecutorTestSuite) TestSignUpAssign_AssignsTheNamedRolesInTheFlowsUnit() {
	user := []role.RoleAssignment{{ID: "user-1", Type: role.AssigneeTypeUser, OUID: "org-1"}}
	s.assignments.EXPECT().AddAssigneesToRoles(mock.MatchedBy(security.IsRuntimeContext), user,
		[]string{"role-2", "role-1"}).Return(nil).Once()

	resp, err := s.executor().Execute(s.signUpContext("role-2 role-1 role-2"))

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
}

// The roles come from the node alone: a selection recorded in the runtime data is not assigned.
func (s *RoleExecutorTestSuite) TestSignUpAssign_IgnoresARecordedSelection() {
	ctx := s.signUpContext("role-1")
	ctx.RuntimeData[common.RuntimeKeySelectedRoles] = "role-9"
	s.assignments.EXPECT().AddAssigneesToRoles(mock.Anything, mock.Anything, []string{"role-1"}).Return(nil).Once()

	resp, err := s.executor().Execute(ctx)

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
}

func (s *RoleExecutorTestSuite) TestSignUpAssign_WithoutRolesIsAnError() {
	for name, roles := range map[string]string{"none": "", "unresolved": "{{ctx(missing)}}"} {
		s.Run(name, func() {
			_, err := s.executor().Execute(s.signUpContext(roles))

			s.Require().Error(err)
		})
	}
	s.assignments.AssertNotCalled(s.T(), "AddAssigneesToRoles", mock.Anything, mock.Anything, mock.Anything)
}

func (s *RoleExecutorTestSuite) TestSignUpAssign_WithoutTheFlowsUnitOrUserIsAnError() {
	for _, key := range []string{common.RuntimeKeyOUID, userAttributeUserID} {
		s.Run(key, func() {
			ctx := s.signUpContext("role-1")
			delete(ctx.RuntimeData, key)

			_, err := s.executor().Execute(ctx)

			s.Require().Error(err)
		})
	}
	s.assignments.AssertNotCalled(s.T(), "AddAssigneesToRoles", mock.Anything, mock.Anything, mock.Anything)
}

// A role the unit cannot assign fails the step, and a server failure fails the flow.
func (s *RoleExecutorTestSuite) TestSignUpAssign_AFailureIsReported() {
	s.assignments.EXPECT().AddAssigneesToRoles(mock.Anything, mock.Anything, mock.Anything).Return(
		&tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "ROL-1010"}).Once()

	resp, err := s.executor().Execute(s.signUpContext("role-1"))

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, resp.Status)
	s.Equal(ErrRoleAssignmentFailed.Code, resp.Error.Code)

	s.assignments.EXPECT().AddAssigneesToRoles(mock.Anything, mock.Anything, mock.Anything).
		Return(&tidcommon.InternalServerError).Once()

	_, err = s.executor().Execute(s.signUpContext("role-1"))

	s.Require().Error(err)
	s.Contains(err.Error(), "failed to assign the named roles")
}

func (s *RoleExecutorTestSuite) TestMetaDeclaresTheRolesProperty() {
	s.Contains(s.executor().GetMeta().SupportedProperties,
		providers.ExecutorSupportedProperties{Property: common.NodePropertyRoles})
}
