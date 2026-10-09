// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const (
	testRoleID        = "role-1"
	testAssigneeID    = "01a01978-b3a5-7d2a-a4cf-0c57328e94cf"
	testCaliforniaAud = "https://api.dmv.ca.gov"
	testOhioAud       = "https://api.dmv.oh.gov"
	testGroupID       = "group-1"
	testMemberID      = "01a01978-b3a5-7d2a-a4cf-0c57328e94d0"
	testRSID          = "rs-ca"
	testResourceID    = "resource-1"
	testActionID      = "action-1"
	testLicenceScope  = "license"
	testPermissionSet = `[{"resourceServerId":"rs-ca","permissions":["license"]}]`
)

// newAdministrationTestFactory returns a real flow factory so executors report their own metadata.
func newAdministrationTestFactory(t *testing.T) core.FlowFactoryInterface {
	require.NoError(t, config.InitializeServerRuntime(t.TempDir(), &config.Config{}))
	t.Cleanup(config.ResetServerRuntime)
	factory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	return factory
}

func administrationNodeContext(mode string, inputs map[string]string) *providers.NodeContext {
	return &providers.NodeContext{
		Context:           context.Background(),
		ExecutorMode:      mode,
		UserInputs:        inputs,
		RuntimeData:       map[string]string{},
		SharedRuntimeData: map[string]string{},
	}
}

func withRevocationPlan(t *testing.T, mode string, plan revocationPlan) *providers.NodeContext {
	encoded, err := encodeRevocationPlan(plan)
	require.NoError(t, err)
	ctx := administrationNodeContext(mode, nil)
	ctx.SharedRuntimeData[common.RuntimeKeyRevocationPlan] = encoded
	return ctx
}

// oneCaliforniaScope is the smallest target that produces criteria.
func oneCaliforniaScope(entityIDs ...string) *revocation.AccessRevocationTarget {
	return &revocation.AccessRevocationTarget{
		EntityIDs: entityIDs,
		Scopes:    []revocation.AudienceScope{{Audience: testCaliforniaAud, Scope: testLicenceScope}},
	}
}

type AccessChangeValidatorTestSuite struct {
	suite.Suite
	factory     core.FlowFactoryInterface
	roles       *roleAdminProviderMock
	assignments *roleAssignmentAdminProviderMock
	resources   *resourceAdminProviderMock
}

func TestAccessChangeValidatorTestSuite(t *testing.T) {
	suite.Run(t, new(AccessChangeValidatorTestSuite))
}

func (s *AccessChangeValidatorTestSuite) SetupTest() {
	s.factory = newAdministrationTestFactory(s.T())
	s.roles = newRoleAdminProviderMock(s.T())
	s.assignments = newRoleAssignmentAdminProviderMock(s.T())
	s.resources = newResourceAdminProviderMock(s.T())
}

func (s *AccessChangeValidatorTestSuite) validator() *accessChangeValidator {
	return newAccessChangeValidator(s.factory, s.roles, s.assignments, s.resources)
}

func (s *AccessChangeValidatorTestSuite) planFrom(resp *providers.ExecutorResponse) revocationPlan {
	s.Require().NotNil(resp.SharedRuntimeData)
	encoded, ok := resp.SharedRuntimeData[common.RuntimeKeyRevocationPlan]
	s.Require().True(ok, "the validator must publish a plan")
	var plan revocationPlan
	s.Require().NoError(json.Unmarshal([]byte(encoded), &plan))
	return plan
}

// No default mode is declared, so flow validation requires every node to name the change it prepares.
func (s *AccessChangeValidatorTestSuite) TestMetaDeclaresEveryModeAndNoDefault() {
	meta := s.validator().GetMeta()

	s.Require().NotNil(meta)
	s.Empty(meta.DefaultMode)
	s.Equal([]string{
		ExecutorModeActionDeletion, ExecutorModeGroupDeletion, ExecutorModeGroupMemberRemoval,
		ExecutorModeRoleAssignmentRemoval, ExecutorModeRoleDeletion, ExecutorModeRolePermissionRemoval,
	}, meta.SupportedModes)
	s.Equal([]providers.FlowType{providers.FlowTypeAdministration}, meta.SupportedFlowTypes)
}

func (s *AccessChangeValidatorTestSuite) TestUnknownModeIsAnError() {
	for _, mode := range []string{"", "revoke_before_action", ExecutorModeDelete} {
		s.Run(mode, func() {
			_, err := s.validator().Execute(
				administrationNodeContext(mode, map[string]string{revocationInputRole: testRoleID}))

			s.Require().Error(err)
			s.Contains(err.Error(), "unsupported mode")
		})
	}
}

func (s *AccessChangeValidatorTestSuite) TestHasRequiredInputsPerMode() {
	cases := map[string][]string{
		ExecutorModeRoleAssignmentRemoval: {revocationInputRole, revocationInputAssignee},
		ExecutorModeRoleDeletion:          {revocationInputRole},
		ExecutorModeRolePermissionRemoval: {revocationInputRole, revocationInputPermissions},
		ExecutorModeGroupDeletion:         {revocationInputGroup},
		ExecutorModeGroupMemberRemoval:    {revocationInputGroup, revocationInputMember},
		ExecutorModeActionDeletion:        {revocationInputResourceServer, revocationInputAction},
	}
	s.Len(cases, len(accessChangesByMode), "every mode must be covered")
	executor := s.validator()
	for mode, want := range cases {
		s.Run(mode, func() {
			missing := &providers.ExecutorResponse{}
			s.False(executor.HasRequiredInputs(administrationNodeContext(mode, nil), missing))
			identifiers := make([]string, 0, len(missing.Inputs))
			for _, input := range missing.Inputs {
				s.True(input.Required, "%s must be required, or the engine will prompt for it", input.Identifier)
				identifiers = append(identifiers, input.Identifier)
			}
			s.ElementsMatch(want, identifiers)

			supplied := make(map[string]string, len(want))
			for _, identifier := range want {
				supplied[identifier] = "value"
			}
			complete := &providers.ExecutorResponse{}
			s.True(executor.HasRequiredInputs(administrationNodeContext(mode, supplied), complete))
			s.Empty(complete.Inputs)
		})
	}
}

func (s *AccessChangeValidatorTestSuite) TestHasRequiredInputsReportsOnlyTheMissingInput() {
	resp := &providers.ExecutorResponse{}
	ok := s.validator().HasRequiredInputs(
		administrationNodeContext(ExecutorModeRoleAssignmentRemoval,
			map[string]string{revocationInputRole: testRoleID}), resp)

	s.False(ok)
	s.Require().Len(resp.Inputs, 1)
	s.Equal(revocationInputAssignee, resp.Inputs[0].Identifier)
}

func (s *AccessChangeValidatorTestSuite) TestHasRequiredInputsRefusesAnUnknownMode() {
	s.False(s.validator().HasRequiredInputs(
		administrationNodeContext("unknown", map[string]string{revocationInputRole: testRoleID}),
		&providers.ExecutorResponse{}))
}

func (s *AccessChangeValidatorTestSuite) TestExecutePromptsForTheModeInputs() {
	resp, err := s.validator().Execute(
		administrationNodeContext(ExecutorModeGroupMemberRemoval, nil))

	s.Require().NoError(err)
	s.Equal(providers.ExecUserInputRequired, resp.Status)
	identifiers := make([]string, 0, len(resp.Inputs))
	for _, input := range resp.Inputs {
		identifiers = append(identifiers, input.Identifier)
	}
	s.ElementsMatch([]string{revocationInputGroup, revocationInputMember}, identifiers)
}

func (s *AccessChangeValidatorTestSuite) TestRoleAssignmentRemoval_PlansOneCriterionPerEntityScope() {
	s.assignments.On("ValidateRemoveAssignment", mock.Anything, testRoleID, testAssigneeID).
		Return(&revocation.AccessRevocationTarget{
			EntityIDs: []string{testAssigneeID, "entity-2"},
			Scopes: []revocation.AudienceScope{
				{Audience: testCaliforniaAud, Scope: testLicenceScope},
				{Audience: testOhioAud, Scope: testLicenceScope},
			},
			AssigneeType: string(role.AssigneeTypeGroup),
		}, nil)

	resp, err := s.validator().Execute(administrationNodeContext(
		ExecutorModeRoleAssignmentRemoval,
		map[string]string{revocationInputRole: testRoleID, revocationInputAssignee: testAssigneeID}))

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
	plan := s.planFrom(resp)
	s.Len(plan.Criteria, 4)
	s.Equal(revocation.ReasonRoleAssignmentRemoved, plan.Reason)
	s.Equal(revocation.ModeBeforeAction, plan.Mode,
		"a lost scope can be regranted, so the revocation must be bounded")
	s.False(plan.Cutoff.IsZero())
	s.Equal(testRoleID, plan.TargetID)
	s.Equal(testAssigneeID, plan.AssigneeID)
	s.Equal(string(role.AssigneeTypeGroup), plan.AssigneeType)

	// The same scope name on two resource servers must produce two distinct criteria.
	s.Contains(plan.Criteria, revocation.Criterion{
		Type:  revocation.CriterionTypeEntityScope,
		Value: revocation.EntityScopeCriterionValue(testAssigneeID, testCaliforniaAud, testLicenceScope),
	})
	s.Contains(plan.Criteria, revocation.Criterion{
		Type:  revocation.CriterionTypeEntityScope,
		Value: revocation.EntityScopeCriterionValue(testAssigneeID, testOhioAud, testLicenceScope),
	})
}

// A role carrying no permissions has nothing to deny, but the unassignment must still happen.
func (s *AccessChangeValidatorTestSuite) TestRoleAssignmentRemoval_NothingToRevoke() {
	s.assignments.On("ValidateRemoveAssignment", mock.Anything, testRoleID, testAssigneeID).
		Return(&revocation.AccessRevocationTarget{EntityIDs: []string{testAssigneeID}}, nil)

	resp, err := s.validator().Execute(administrationNodeContext(
		ExecutorModeRoleAssignmentRemoval,
		map[string]string{revocationInputRole: testRoleID, revocationInputAssignee: testAssigneeID}))

	s.Require().NoError(err)
	plan := s.planFrom(resp)
	s.Empty(plan.Criteria)
	s.True(plan.NothingToRevoke)
}

func (s *AccessChangeValidatorTestSuite) TestCarriesValidatorRefusal() {
	refusal := tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "GRP-1015",
		Error: tidcommon.I18nMessage{DefaultValue: "Cannot modify declarative group"},
	}
	s.roles.On("ValidateGroupMembershipChange", mock.Anything, testGroupID, "").Return(nil, &refusal)

	resp, err := s.validator().Execute(administrationNodeContext(
		ExecutorModeGroupDeletion, map[string]string{revocationInputGroup: testGroupID}))

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, resp.Status)
	s.Require().NotNil(resp.Error)
	s.Equal("GRP-1015", resp.Error.Code)
}

func (s *AccessChangeValidatorTestSuite) TestUncodedRefusalFallsBackToTheModeError() {
	uncoded := &tidcommon.ServiceError{Type: tidcommon.ClientErrorType}
	s.assignments.On("ValidateRemoveAssignment", mock.Anything, mock.Anything, mock.Anything).Return(nil, uncoded)
	s.roles.On("ValidateDeleteRole", mock.Anything, mock.Anything).Return(nil, uncoded)
	s.roles.On("ValidateUpdateRolePermissions", mock.Anything, mock.Anything, mock.Anything).Return(nil, uncoded)
	s.roles.On("ValidateGroupMembershipChange", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, uncoded)
	s.resources.On("ValidateDeleteAction", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, uncoded)

	cases := []struct {
		mode   string
		inputs map[string]string
		want   tidcommon.ServiceError
	}{
		{ExecutorModeRoleAssignmentRemoval, map[string]string{
			revocationInputRole: testRoleID, revocationInputAssignee: testAssigneeID},
			ErrRoleAssignmentRemovalNotAllowed},
		{ExecutorModeRoleDeletion, map[string]string{revocationInputRole: testRoleID},
			ErrRoleDeletionNotAllowed},
		{ExecutorModeRolePermissionRemoval, map[string]string{
			revocationInputRole: testRoleID, revocationInputPermissions: testPermissionSet},
			ErrRolePermissionRemovalNotAllowed},
		{ExecutorModeGroupDeletion, map[string]string{revocationInputGroup: testGroupID},
			ErrGroupDeletionNotAllowed},
		{ExecutorModeGroupMemberRemoval, map[string]string{
			revocationInputGroup: testGroupID, revocationInputMember: testMemberID},
			ErrGroupMembershipRemovalNotAllowed},
		{ExecutorModeActionDeletion, map[string]string{
			revocationInputResourceServer: testRSID, revocationInputAction: testActionID},
			ErrActionDeletionNotAllowed},
	}
	for _, tc := range cases {
		s.Run(tc.mode, func() {
			resp, err := s.validator().Execute(
				administrationNodeContext(tc.mode, tc.inputs))

			s.Require().NoError(err)
			s.Equal(providers.ExecFailure, resp.Status)
			s.Require().NotNil(resp.Error)
			s.Equal(tc.want.Code, resp.Error.Code)
		})
	}
}

// The validator reports a missing seam as an error rather than a refusal.
func (s *AccessChangeValidatorTestSuite) TestRefusesWithoutItsSeam() {
	executor := newAccessChangeValidator(s.factory, nil, nil, nil)
	cases := map[string]struct {
		inputs map[string]string
		want   string
	}{
		ExecutorModeRoleAssignmentRemoval: {map[string]string{
			revocationInputRole: testRoleID, revocationInputAssignee: testAssigneeID}, "role service"},
		ExecutorModeGroupMemberRemoval: {map[string]string{
			revocationInputGroup: testGroupID, revocationInputMember: testMemberID}, "role service"},
		ExecutorModeActionDeletion: {map[string]string{
			revocationInputResourceServer: testRSID, revocationInputAction: testActionID}, "resource service"},
	}
	for mode, tc := range cases {
		s.Run(mode, func() {
			_, err := executor.Execute(administrationNodeContext(mode, tc.inputs))

			s.Require().Error(err)
			s.Contains(err.Error(), tc.want+" is not configured")
		})
	}
}

func (s *AccessChangeValidatorTestSuite) TestRoleDeletion_PlansPerPrincipalCriteria() {
	s.roles.On("ValidateDeleteRole", mock.Anything, testRoleID).
		Return(oneCaliforniaScope(testAssigneeID, "entity-2"), nil)

	resp, err := s.validator().Execute(administrationNodeContext(
		ExecutorModeRoleDeletion, map[string]string{revocationInputRole: testRoleID}))

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
	plan := s.planFrom(resp)
	s.Equal(revocation.ReasonRoleDeleted, plan.Reason)
	s.Equal(revocation.ModeBeforeAction, plan.Mode,
		"a role name can be reused and its scopes regranted, so the row must be bounded")
	s.Equal(testRoleID, plan.TargetID)
	s.Len(plan.Criteria, 2)
	s.Equal(revocation.CriterionTypeEntityScope, plan.Criteria[0].Type)
}

func (s *AccessChangeValidatorTestSuite) TestRolePermissionRemoval_CarriesTheNewSet() {
	s.roles.On("ValidateUpdateRolePermissions", mock.Anything, testRoleID, []role.ResourcePermissions{
		{ResourceServerID: testRSID, Permissions: []string{testLicenceScope}}}).
		Return(oneCaliforniaScope(testAssigneeID), nil)

	resp, err := s.validator().Execute(administrationNodeContext(
		ExecutorModeRolePermissionRemoval, map[string]string{
			revocationInputRole:        testRoleID,
			revocationInputPermissions: testPermissionSet,
		}))

	s.Require().NoError(err)
	plan := s.planFrom(resp)
	s.Equal(revocation.ReasonRolePermissionRemoved, plan.Reason)
	s.Equal(testPermissionSet, plan.ActionArgs[revocationInputPermissions])
	s.Len(plan.Criteria, 1)
}

func (s *AccessChangeValidatorTestSuite) TestRolePermissionRemoval_RefusesAMalformedSetBeforeRevoking() {
	resp, err := s.validator().Execute(administrationNodeContext(
		ExecutorModeRolePermissionRemoval, map[string]string{
			revocationInputRole:        testRoleID,
			revocationInputPermissions: "{not json",
		}))

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, resp.Status)
	s.Require().NotNil(resp.Error)
	s.Equal(ErrInvalidRolePermissions.Code, resp.Error.Code)
	s.Nil(resp.SharedRuntimeData, "no plan may be published, so nothing downstream can revoke")
	s.roles.AssertNotCalled(s.T(), "ValidateUpdateRolePermissions", mock.Anything, mock.Anything, mock.Anything)
}

func (s *AccessChangeValidatorTestSuite) TestRolePermissionRemoval_RefusesAnUngrantableSetBeforeRevoking() {
	refusal := tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "ROL-1012",
		Error: tidcommon.I18nMessage{DefaultValue: "Invalid permissions"},
	}
	s.roles.On("ValidateUpdateRolePermissions", mock.Anything, testRoleID, mock.Anything).Return(nil, &refusal)

	resp, err := s.validator().Execute(administrationNodeContext(
		ExecutorModeRolePermissionRemoval, map[string]string{
			revocationInputRole:        testRoleID,
			revocationInputPermissions: `[{"resourceServerId":"rs-unknown","permissions":["license"]}]`,
		}))

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, resp.Status)
	s.Require().NotNil(resp.Error)
	s.Equal("ROL-1012", resp.Error.Code)
	s.Nil(resp.SharedRuntimeData, "no plan may be published, so nothing downstream can revoke")
}

// Deleting a group and removing one member deny the same shape of artifact, so they record one reason.
func (s *AccessChangeValidatorTestSuite) TestGroupDeletion_PlansForEveryMember() {
	s.roles.On("ValidateGroupMembershipChange", mock.Anything, testGroupID, "").
		Return(oneCaliforniaScope("member-1", "member-2"), nil)

	resp, err := s.validator().Execute(administrationNodeContext(
		ExecutorModeGroupDeletion, map[string]string{revocationInputGroup: testGroupID}))

	s.Require().NoError(err)
	plan := s.planFrom(resp)
	s.Equal(revocation.ReasonGroupMembershipRemoved, plan.Reason)
	s.Equal(testGroupID, plan.TargetID)
	s.Empty(plan.AssigneeID, "a deletion names no departing member")
	s.Len(plan.Criteria, 2)
}

// The departing member has to reach the acting node, which cannot re-derive it from a digest.
func (s *AccessChangeValidatorTestSuite) TestGroupMemberRemoval_CarriesTheMember() {
	target := oneCaliforniaScope(testMemberID)
	target.AssigneeType = string(group.MemberTypeUser)
	s.roles.On("ValidateGroupMembershipChange", mock.Anything, testGroupID, testMemberID).Return(target, nil)

	resp, err := s.validator().Execute(administrationNodeContext(
		ExecutorModeGroupMemberRemoval,
		map[string]string{revocationInputGroup: testGroupID, revocationInputMember: testMemberID}))

	s.Require().NoError(err)
	plan := s.planFrom(resp)
	s.Equal(testGroupID, plan.TargetID)
	s.Equal(testMemberID, plan.AssigneeID)
	s.Equal(string(group.MemberTypeUser), plan.AssigneeType)
}

func (s *AccessChangeValidatorTestSuite) TestActionDeletion_PlansInTheScopeDimension() {
	s.resources.On("ValidateDeleteAction", mock.Anything, testRSID, optionalID(testResourceID), testActionID).
		Return(&revocation.AccessRevocationTarget{
			Scopes: []revocation.AudienceScope{{Audience: testCaliforniaAud, Scope: testLicenceScope}},
		}, nil)

	resp, err := s.validator().Execute(administrationNodeContext(
		ExecutorModeActionDeletion, map[string]string{
			revocationInputResourceServer: testRSID,
			revocationInputResource:       testResourceID,
			revocationInputAction:         testActionID,
		}))

	s.Require().NoError(err)
	plan := s.planFrom(resp)
	s.Equal(revocation.ReasonActionDeleted, plan.Reason)
	s.Equal([]revocation.Criterion{{
		Type:  revocation.CriterionTypeScope,
		Value: revocation.ScopeCriterionValue(testCaliforniaAud, testLicenceScope),
	}}, plan.Criteria)
	s.Equal(testRSID, plan.TargetID)
	s.Equal(testActionID, plan.ActionArgs[revocationInputAction])
	s.Equal(testResourceID, plan.ActionArgs[revocationInputResource])
}

// An action defined on the resource server itself has no resource, so that argument is optional.
func (s *AccessChangeValidatorTestSuite) TestActionDeletion_ResourceIsOptional() {
	s.resources.On("ValidateDeleteAction", mock.Anything, testRSID, (*string)(nil), testActionID).
		Return(&revocation.AccessRevocationTarget{
			Scopes: []revocation.AudienceScope{{Audience: testCaliforniaAud, Scope: testLicenceScope}},
		}, nil)

	resp, err := s.validator().Execute(administrationNodeContext(
		ExecutorModeActionDeletion,
		map[string]string{revocationInputResourceServer: testRSID, revocationInputAction: testActionID}))

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
	plan := s.planFrom(resp)
	s.Empty(plan.ActionArgs[revocationInputResource])
	// A deployment-wide plan has no principals by design, so that emptiness is not "nothing to revoke".
	s.False(plan.NothingToRevoke)
	s.Len(plan.Criteria, 1)
}

// A scope whose resource server has no audience is deleted with nothing denied.
func (s *AccessChangeValidatorTestSuite) TestActionDeletion_NothingToRevoke() {
	s.resources.On("ValidateDeleteAction", mock.Anything, testRSID, (*string)(nil), testActionID).
		Return(&revocation.AccessRevocationTarget{}, nil)

	resp, err := s.validator().Execute(administrationNodeContext(
		ExecutorModeActionDeletion,
		map[string]string{revocationInputResourceServer: testRSID, revocationInputAction: testActionID}))

	s.Require().NoError(err)
	plan := s.planFrom(resp)
	s.True(plan.NothingToRevoke)
	s.Empty(plan.Criteria)
}

// The cap belongs to the revoker, so the validator plans every criterion the change implies.
func (s *AccessChangeValidatorTestSuite) TestPlansEveryCriterionWithoutCapping() {
	entityIDs := make([]string, 6)
	for i := range entityIDs {
		entityIDs[i] = fmt.Sprintf("entity-%d", i)
	}
	s.roles.On("ValidateDeleteRole", mock.Anything, testRoleID).
		Return(&revocation.AccessRevocationTarget{
			EntityIDs: entityIDs,
			Scopes: []revocation.AudienceScope{
				{Audience: testCaliforniaAud, Scope: testLicenceScope},
				{Audience: testOhioAud, Scope: testLicenceScope},
			},
		}, nil)

	resp, err := s.validator().Execute(administrationNodeContext(
		ExecutorModeRoleDeletion, map[string]string{revocationInputRole: testRoleID}))

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
	s.Len(s.planFrom(resp).Criteria, 12)
}

// A server-side failure is not a refusal the caller could act on, so it fails the flow.
func (s *AccessChangeValidatorTestSuite) TestServerErrorFailsTheFlow() {
	s.roles.On("ValidateDeleteRole", mock.Anything, testRoleID).
		Return(nil, &tidcommon.InternalServerError)

	_, err := s.validator().Execute(administrationNodeContext(
		ExecutorModeRoleDeletion, map[string]string{revocationInputRole: testRoleID}))

	s.Require().Error(err)
	s.Contains(err.Error(), "failed to validate")
}

func (s *AccessChangeValidatorTestSuite) TestReadsAnInputFromRuntimeData() {
	s.roles.On("ValidateDeleteRole", mock.Anything, testRoleID).
		Return(oneCaliforniaScope(testAssigneeID), nil)

	ctx := administrationNodeContext(ExecutorModeRoleDeletion, map[string]string{"unrelated": "x"})
	ctx.RuntimeData[revocationInputRole] = testRoleID
	resp, err := s.validator().Execute(ctx)

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
	s.Equal(testRoleID, s.planFrom(resp).TargetID)
}

func (s *AccessChangeValidatorTestSuite) TestEveryModeReasonIsABoundaryReason() {
	for mode, change := range accessChangesByMode {
		s.True(revocation.IsBoundaryReason(change.reason), "%s must be bounded, not terminal", mode)
	}
}
