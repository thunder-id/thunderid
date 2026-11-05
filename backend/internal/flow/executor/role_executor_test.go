// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/executormeta"
	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type RoleExecutorTestSuite struct {
	suite.Suite
	factory     core.FlowFactoryInterface
	roles       *roleAdminProviderMock
	assignments *roleAssignmentAdminProviderMock
	permissions *entityPermissionResolverMock
}

const recordedAdmin = `{"subject":"admin-1","ouId":"ou-a","permissions":["system:user"]}`

func TestRoleExecutorTestSuite(t *testing.T) {
	suite.Run(t, new(RoleExecutorTestSuite))
}

func (s *RoleExecutorTestSuite) SetupTest() {
	s.factory = newAdministrationTestFactory(s.T())
	s.roles = newRoleAdminProviderMock(s.T())
	s.assignments = newRoleAssignmentAdminProviderMock(s.T())
	s.permissions = newEntityPermissionResolverMock(s.T())
}

func (s *RoleExecutorTestSuite) executor() *roleExecutor {
	return newRoleExecutor(s.factory, s.roles, s.assignments, s.permissions)
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

// A node must name its mode: the revocation plan modes must match the access change validator's, which a default
// would leave to chance.
func (s *RoleExecutorTestSuite) TestMetaDeclaresEveryModeAndNoDefault() {
	meta := s.executor().GetMeta()

	s.Require().NotNil(meta)
	s.Empty(meta.DefaultMode)
	catalogMeta, ok := executormeta.MetaFor(ExecutorNameRole)
	s.Require().True(ok)
	s.Empty(catalogMeta.DefaultMode)
	s.Equal([]string{
		ExecutorModeRoleAssignment, ExecutorModeRolePermissionRemoval, ExecutorModeRoleSelection,
	}, meta.SupportedModes)
	s.Equal([]providers.FlowType{providers.FlowTypeAdministration, providers.FlowTypeInvitation},
		meta.SupportedFlowTypes)
}

func (s *RoleExecutorTestSuite) TestARevocationModeOutsideAnAdministrationFlowIsAnError() {
	for _, mode := range []string{ExecutorModeRolePermissionRemoval} {
		s.Run(mode, func() {
			ctx := withRevocationPlan(s.T(), mode, rolePlan(revocation.ReasonRoleDeleted))
			ctx.FlowType = providers.FlowTypeInvitation

			_, err := s.executor().Execute(ctx)

			s.Require().Error(err)
			s.Contains(err.Error(), "is not supported in INVITATION flows")
		})
	}
	s.roles.AssertNotCalled(s.T(), "UpdateRoleWithPermissions", mock.Anything, mock.Anything, mock.Anything)
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
	_, err := newRoleExecutor(s.factory, s.roles, nil, s.permissions).Execute(
		s.nodeContext(adminWithPermissions(), ExecutorModeRoleSelection, map[string]string{}))

	s.EqualError(err, "role assignment service is not configured")
}

func (s *RoleExecutorTestSuite) TestUnknownModeIsAnError() {
	for _, mode := range []string{ExecutorModeRoleAssignmentRemoval, ExecutorModeGroupMembershipRemoval, ""} {
		s.Run(mode, func() {
			_, err := s.executor().Execute(
				withRevocationPlan(s.T(), mode, rolePlan(revocation.ReasonRoleDeleted)))

			s.Require().Error(err)
			s.Contains(err.Error(), "unsupported mode")
		})
	}
	s.assignments.AssertNotCalled(s.T(), "RemoveAssignments", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything)
}

func (s *RoleExecutorTestSuite) TestRemovePermissions_CarriesARefusedRoleRead() {
	s.roles.On("GetRoleWithPermissions", mock.Anything, testRoleID).Return(nil, &role.ErrorRoleNotFound)
	plan := rolePlan(revocation.ReasonRolePermissionRemoved)
	plan.ActionArgs = map[string]string{revocationInputPermissions: testPermissionSet}

	resp, err := s.executor().Execute(withRevocationPlan(s.T(), ExecutorModeRolePermissionRemoval, plan))

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, resp.Status)
	s.Require().NotNil(resp.Error)
	s.Equal(role.ErrorRoleNotFound.Code, resp.Error.Code)
	s.roles.AssertNotCalled(s.T(), "UpdateRoleWithPermissions", mock.Anything, mock.Anything, mock.Anything)
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
		withRevocationPlan(s.T(), ExecutorModeRolePermissionRemoval, plan))

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
		withRevocationPlan(s.T(), ExecutorModeRolePermissionRemoval, plan))

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
}

func (s *RoleExecutorTestSuite) TestRemovePermissions_MissingCarriedSetIsAnError() {
	_, err := s.executor().Execute(withRevocationPlan(s.T(), ExecutorModeRolePermissionRemoval,
		rolePlan(revocation.ReasonRolePermissionRemoved)))

	s.Require().Error(err)
	s.Contains(err.Error(), "failed to decode role permissions")
	s.roles.AssertNotCalled(s.T(), "UpdateRoleWithPermissions", mock.Anything, mock.Anything, mock.Anything)
}

func (s *RoleExecutorTestSuite) TestRemovePermissions_MalformedCarriedSetIsAnError() {
	plan := rolePlan(revocation.ReasonRolePermissionRemoved)
	plan.ActionArgs = map[string]string{revocationInputPermissions: "{not json"}

	_, err := s.executor().Execute(
		withRevocationPlan(s.T(), ExecutorModeRolePermissionRemoval, plan))

	s.Require().Error(err)
	s.Contains(err.Error(), "failed to decode role permissions")
	s.roles.AssertNotCalled(s.T(), "UpdateRoleWithPermissions", mock.Anything, mock.Anything, mock.Anything)
}

func (s *RoleExecutorTestSuite) TestEveryModeRejectsAForeignPlan() {
	foreign := map[string]revocation.Reason{
		ExecutorModeRolePermissionRemoval: revocation.ReasonUserDeleted,
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
	s.roles.On("GetRoleWithPermissions", mock.Anything, mock.Anything).
		Return(&role.RoleWithPermissions{ID: testRoleID}, nil)
	s.roles.On("UpdateRoleWithPermissions", mock.Anything, mock.Anything, mock.Anything).Return(nil, uncoded)

	permissions := rolePlan(revocation.ReasonRolePermissionRemoved)
	permissions.ActionArgs = map[string]string{revocationInputPermissions: testPermissionSet}
	cases := []struct {
		mode string
		plan revocationPlan
		want tidcommon.ServiceError
	}{
		{ExecutorModeRolePermissionRemoval, permissions, ErrRolePermissionRemovalFailed},
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
	s.roles.On("GetRoleWithPermissions", mock.Anything, testRoleID).
		Return(&role.RoleWithPermissions{ID: testRoleID}, nil)
	s.roles.On("UpdateRoleWithPermissions", mock.Anything, testRoleID, mock.Anything).
		Return(nil, &tidcommon.InternalServerError)
	plan := rolePlan(revocation.ReasonRolePermissionRemoved)
	plan.ActionArgs = map[string]string{revocationInputPermissions: testPermissionSet}

	_, err := s.executor().Execute(withRevocationPlan(s.T(), ExecutorModeRolePermissionRemoval, plan))

	s.Require().Error(err)
	s.Contains(err.Error(), "failed to perform")
}

func (s *RoleExecutorTestSuite) TestRefusesWithoutItsSeam() {
	_, err := newRoleExecutor(s.factory, nil, nil, nil).Execute(
		administrationNodeContext(ExecutorModeRolePermissionRemoval, nil))

	s.Require().Error(err)
	s.EqualError(err, "role service is not configured")
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

// adminWithPermissions is the starter's context, holding the permissions the select step records.
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

func (s *RoleExecutorTestSuite) TestSelect_RecordsTheRolesAndTheAdmin() {
	s.offers("role-1", "role-2")

	resp, err := s.executor().Execute(s.nodeContext(adminWithPermissions(), ExecutorModeRoleSelection,
		map[string]string{userInputRoles: "role-2 role-1 role-2"}))

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
	s.Equal("role-2 role-1", resp.RuntimeData[common.RuntimeKeySelectedRoles])
	s.JSONEq(recordedAdmin, resp.RuntimeData[common.RuntimeKeyRolesSelectedBy])
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

func (s *RoleExecutorTestSuite) TestSelect_RefusesAnyCallerButTheStarter() {
	resp, err := s.executor().Execute(s.nodeContext(security.WithRuntimeContext(context.Background()),
		ExecutorModeRoleSelection, map[string]string{userInputRoles: "role-1"}))

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, resp.Status)
	s.Equal(ErrNotTheInviter.Code, resp.Error.Code)
	s.assignments.AssertNotCalled(s.T(), "ListAssignableRoles", mock.Anything, mock.Anything)
}

// holdsNow makes the recorded admin hold the given permissions at acceptance.
func (s *RoleExecutorTestSuite) holdsNow(permissions ...string) {
	s.permissions.EXPECT().ResolveForEntity(mock.Anything, "admin-1").
		Return(security.PermissionSet{"rs-1": permissions}, nil).Once()
}

func (s *RoleExecutorTestSuite) recorded(ctx *providers.NodeContext) *providers.NodeContext {
	ctx.RuntimeData[common.RuntimeKeySelectedRoles] = "role-1 role-2"
	ctx.RuntimeData[common.RuntimeKeyRolesSelectedBy] = recordedAdmin
	return ctx
}

// The role service checks the assignments as the recorded admin, not as the invitee's runtime context, and makes
// all of them or none.
func (s *RoleExecutorTestSuite) TestAssign_AssignsAsTheRecordedAdmin() {
	asAdmin := mock.MatchedBy(func(ctx context.Context) bool {
		return !security.IsRuntimeContext(ctx) && security.GetSubject(ctx) == "admin-1" &&
			security.GetOUID(ctx) == "ou-a" && len(security.GetPermissions(ctx)) == 1 &&
			security.GetPermissions(ctx)[0] == "system:user"
	})
	user := []role.RoleAssignment{{ID: "user-1", Type: role.AssigneeTypeUser, OUID: "ou-a"}}
	s.holdsNow("system:user")
	s.assignments.EXPECT().AddAssigneesToRoles(asAdmin, user, []string{"role-1", "role-2"}).Return(nil).Once()

	resp, err := s.executor().Execute(s.recorded(s.nodeContext(
		security.WithRuntimeContext(context.Background()), ExecutorModeRoleAssignment, map[string]string{})))

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
}

func (s *RoleExecutorTestSuite) TestAssign_IgnoresInputsNamedLikeTheRecord() {
	forged, _ := json.Marshal(map[string]interface{}{"subject": "root", "permissions": []string{"system"}})
	ctx := s.nodeContext(security.WithRuntimeContext(context.Background()), ExecutorModeRoleAssignment,
		map[string]string{
			common.RuntimeKeySelectedRoles:   "role-9",
			common.RuntimeKeyRolesSelectedBy: string(forged),
			userInputRoles:                   "role-9",
		})

	resp, err := s.executor().Execute(ctx)

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
	s.assignments.AssertNotCalled(s.T(), "AddAssigneesToRoles", mock.Anything, mock.Anything, mock.Anything)
}

func (s *RoleExecutorTestSuite) TestAssign_RolesWithoutARecordedAdminAreRefused() {
	ctx := s.recorded(s.nodeContext(security.WithRuntimeContext(context.Background()), ExecutorModeRoleAssignment,
		map[string]string{}))
	delete(ctx.RuntimeData, common.RuntimeKeyRolesSelectedBy)

	resp, err := s.executor().Execute(ctx)

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, resp.Status)
	s.Equal(ErrRoleAssignmentFailed.Code, resp.Error.Code)
	s.assignments.AssertNotCalled(s.T(), "AddAssigneesToRoles", mock.Anything, mock.Anything, mock.Anything)
}

func (s *RoleExecutorTestSuite) TestAssign_AFailureIsReported() {
	s.holdsNow("system:user")
	s.assignments.EXPECT().AddAssigneesToRoles(mock.Anything, mock.Anything, mock.Anything).Return(
		&tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "ROL-1010"}).Once()

	resp, err := s.executor().Execute(s.recorded(s.nodeContext(
		security.WithRuntimeContext(context.Background()), ExecutorModeRoleAssignment, map[string]string{})))

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, resp.Status)
	s.Equal(ErrRoleAssignmentFailed.Code, resp.Error.Code)
}

// A permission revoked since the roles were selected must not authorize their assignment at acceptance.
func (s *RoleExecutorTestSuite) TestAssign_AnAdminWhoLostAPermissionAssignsNothing() {
	s.holdsNow("system:user:view")

	resp, err := s.executor().Execute(s.recorded(s.nodeContext(
		security.WithRuntimeContext(context.Background()), ExecutorModeRoleAssignment, map[string]string{})))

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, resp.Status)
	s.Equal(ErrRoleAssignmentFailed.Code, resp.Error.Code)
	s.assignments.AssertNotCalled(s.T(), "AddAssigneesToRoles", mock.Anything, mock.Anything, mock.Anything)
}

// An admin token carries OIDC scopes no resource server holds; they are left out, and the roles are assigned with
// the permissions the admin still holds.
func (s *RoleExecutorTestSuite) TestAssign_ScopesNoResourceServerHoldsAreLeftOut() {
	withHeldOnly := mock.MatchedBy(func(ctx context.Context) bool {
		return slices.Equal(security.GetPermissions(ctx), []string{"system:user"})
	})
	s.holdsNow("system:user")
	s.assignments.EXPECT().AddAssigneesToRoles(withHeldOnly, mock.Anything, mock.Anything).Return(nil).Once()
	ctx := s.recorded(s.nodeContext(security.WithRuntimeContext(context.Background()), ExecutorModeRoleAssignment,
		map[string]string{}))
	ctx.RuntimeData[common.RuntimeKeyRolesSelectedBy] =
		`{"subject":"admin-1","ouId":"ou-a","permissions":["openid","profile","email","ou","system:user"]}`

	resp, err := s.executor().Execute(ctx)

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
}

// A permission revoked since selection is left out too, and the role service decides with what remains.
func (s *RoleExecutorTestSuite) TestAssign_ARevokedPermissionIsLeftOutOfTheGrant() {
	withHeldOnly := mock.MatchedBy(func(ctx context.Context) bool {
		return slices.Equal(security.GetPermissions(ctx), []string{"read"})
	})
	s.holdsNow("read")
	s.assignments.EXPECT().AddAssigneesToRoles(withHeldOnly, mock.Anything, mock.Anything).Return(
		&tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "ROL-1010"}).Once()
	ctx := s.recorded(s.nodeContext(security.WithRuntimeContext(context.Background()), ExecutorModeRoleAssignment,
		map[string]string{}))
	ctx.RuntimeData[common.RuntimeKeyRolesSelectedBy] =
		`{"subject":"admin-1","ouId":"ou-a","permissions":["system","read"]}`

	resp, err := s.executor().Execute(ctx)

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, resp.Status)
	s.Equal(ErrRoleAssignmentFailed.Code, resp.Error.Code)
}

func (s *RoleExecutorTestSuite) TestAssign_APermissionHeldThroughABroaderOneCounts() {
	s.holdsNow("system")
	s.assignments.EXPECT().AddAssigneesToRoles(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	resp, err := s.executor().Execute(s.recorded(s.nodeContext(
		security.WithRuntimeContext(context.Background()), ExecutorModeRoleAssignment, map[string]string{})))

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
}

// A server failure fails the flow rather than reading as the user's refusal.
func (s *RoleExecutorTestSuite) TestAssign_AFailedPermissionLookupFailsTheFlow() {
	s.permissions.EXPECT().ResolveForEntity(mock.Anything, "admin-1").Return(nil, errors.New("store down")).Once()

	_, err := s.executor().Execute(s.recorded(s.nodeContext(
		security.WithRuntimeContext(context.Background()), ExecutorModeRoleAssignment, map[string]string{})))

	s.Require().Error(err)
	s.assignments.AssertNotCalled(s.T(), "AddAssigneesToRoles", mock.Anything, mock.Anything, mock.Anything)
}

func (s *RoleExecutorTestSuite) TestAssign_WithoutAPermissionResolverFailsTheFlow() {
	_, err := newRoleExecutor(s.factory, s.roles, s.assignments, nil).Execute(s.recorded(s.nodeContext(
		security.WithRuntimeContext(context.Background()), ExecutorModeRoleAssignment, map[string]string{})))

	s.Require().Error(err)
	s.assignments.AssertNotCalled(s.T(), "AddAssigneesToRoles", mock.Anything, mock.Anything, mock.Anything)
}

func (s *RoleExecutorTestSuite) TestAssign_AServerFailureFailsTheFlow() {
	s.holdsNow("system:user")
	s.assignments.EXPECT().AddAssigneesToRoles(mock.Anything, mock.Anything, mock.Anything).
		Return(&tidcommon.InternalServerError).Once()

	_, err := s.executor().Execute(s.recorded(s.nodeContext(
		security.WithRuntimeContext(context.Background()), ExecutorModeRoleAssignment, map[string]string{})))

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
			ctx := s.recorded(s.nodeContext(security.WithRuntimeContext(context.Background()),
				ExecutorModeRoleAssignment, map[string]string{}))
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
	ctx.NodeProperties = map[string]interface{}{propertyKeyRoles: roles}
	return ctx
}

// The roles the node names are recorded without asking the admin, still only if the admin may assign them.
func (s *RoleExecutorTestSuite) TestSelect_ConfiguredRolesAreRecordedWithoutAPrompt() {
	s.offers("role-1", "role-2")

	resp, err := s.executor().Execute(s.configuredSelection("role-2 role-1 role-2", map[string]string{}))

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
	s.Empty(resp.Inputs)
	s.Equal("role-2 role-1", resp.RuntimeData[common.RuntimeKeySelectedRoles])
	s.JSONEq(recordedAdmin, resp.RuntimeData[common.RuntimeKeyRolesSelectedBy])
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

func (s *RoleExecutorTestSuite) TestMetaDeclaresTheRolesProperty() {
	s.Contains(s.executor().GetMeta().SupportedProperties,
		providers.ExecutorSupportedProperties{Property: propertyKeyRoles})
}
