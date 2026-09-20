// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

const (
	testGroupID       = "group-1"
	testMemberID      = "01a01978-b3a5-7d2a-a4cf-0c57328e94d0"
	testRSID          = "rs-ca"
	testResourceID    = "resource-1"
	testActionID      = "action-1"
	testLicenceScope  = "license"
	testPermissionSet = `[{"resourceServerId":"rs-ca","permissions":["license"]}]`
)

type ScopeRevocationExecutorsTestSuite struct {
	suite.Suite
	factory   core.FlowFactoryInterface
	roles     *roleAdminProviderMock
	groups    *groupAdminProviderMock
	resources *resourceAdminProviderMock
}

func TestScopeRevocationExecutorsTestSuite(t *testing.T) {
	suite.Run(t, new(ScopeRevocationExecutorsTestSuite))
}

func (s *ScopeRevocationExecutorsTestSuite) SetupTest() {
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), &config.Config{}))
	s.T().Cleanup(config.ResetServerRuntime)
	s.factory, _ = core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	s.roles = newRoleAdminProviderMock(s.T())
	s.groups = newGroupAdminProviderMock(s.T())
	s.resources = newResourceAdminProviderMock(s.T())
}

func (s *ScopeRevocationExecutorsTestSuite) nodeContext(inputs map[string]string) *providers.NodeContext {
	return &providers.NodeContext{
		Context:           context.Background(),
		UserInputs:        inputs,
		RuntimeData:       map[string]string{},
		SharedRuntimeData: map[string]string{},
	}
}

func (s *ScopeRevocationExecutorsTestSuite) planFrom(resp *providers.ExecutorResponse) revocationPlan {
	s.Require().NotNil(resp.SharedRuntimeData)
	encoded, ok := resp.SharedRuntimeData[common.RuntimeKeyRevocationPlan]
	s.Require().True(ok, "the preparatory node must publish a plan")
	var plan revocationPlan
	s.Require().NoError(json.Unmarshal([]byte(encoded), &plan))
	return plan
}

func (s *ScopeRevocationExecutorsTestSuite) withPlan(plan revocationPlan) *providers.NodeContext {
	encoded, err := encodeRevocationPlan(plan)
	s.Require().NoError(err)
	ctx := s.nodeContext(nil)
	ctx.SharedRuntimeData[common.RuntimeKeyRevocationPlan] = encoded
	return ctx
}

// oneCaliforniaScope is the smallest target that produces criteria.
func oneCaliforniaScope(entityIDs ...string) *revocation.ScopeRevocationTarget {
	return &revocation.ScopeRevocationTarget{
		EntityIDs: entityIDs,
		Scopes:    []revocation.AudienceScope{{Audience: testCaliforniaAud, Scope: testLicenceScope}},
	}
}

// Deleting a role denies its scopes for every principal that held it, in the per-principal dimension.
func (s *ScopeRevocationExecutorsTestSuite) TestPreRoleDeletion_PlansPerPrincipalCriteria() {
	s.roles.On("ValidateRoleScopeChange", mock.Anything, testRoleID).
		Return(oneCaliforniaScope(testAssigneeID, "entity-2"), nil)

	executor := newPreRoleDeletionExecutor(s.factory, defaultMaxRevocationCriteria, s.roles)
	resp, err := executor.Execute(s.nodeContext(map[string]string{revocationInputRole: testRoleID}))

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

// The new permission set is the acting node's business, not the revocation's: the plan denies every
// scope the role grants today, and carries the set through untouched.
func (s *ScopeRevocationExecutorsTestSuite) TestPreRolePermissionRemoval_CarriesTheNewSet() {
	s.roles.On("ValidateRoleScopeChange", mock.Anything, testRoleID).
		Return(oneCaliforniaScope(testAssigneeID), nil)

	executor := newPreRolePermissionRemovalExecutor(s.factory, defaultMaxRevocationCriteria, s.roles)
	resp, err := executor.Execute(s.nodeContext(map[string]string{
		revocationInputRole:        testRoleID,
		revocationInputPermissions: testPermissionSet,
	}))

	s.Require().NoError(err)
	plan := s.planFrom(resp)
	s.Equal(revocation.ReasonRolePermissionRemoved, plan.Reason)
	s.Equal(testPermissionSet, plan.ActionArgs[revocationInputPermissions])
	s.Len(plan.Criteria, 1)
}

// Deleting a group and removing one member deny the same shape of artifact, so they record one reason.
func (s *ScopeRevocationExecutorsTestSuite) TestPreGroupDeletion_PlansForEveryMember() {
	s.groups.On("ValidateGroupMembershipChange", mock.Anything, testGroupID, "").
		Return(oneCaliforniaScope("member-1", "member-2"), nil)

	executor := newPreGroupDeletionExecutor(s.factory, defaultMaxRevocationCriteria, s.groups)
	resp, err := executor.Execute(s.nodeContext(map[string]string{revocationInputGroup: testGroupID}))

	s.Require().NoError(err)
	plan := s.planFrom(resp)
	s.Equal(revocation.ReasonGroupMembershipRemoved, plan.Reason)
	s.Equal(testGroupID, plan.TargetID)
	s.Empty(plan.AssigneeID, "a deletion names no departing member")
	s.Len(plan.Criteria, 2)
}

// The departing member has to reach the acting node, which cannot re-derive it from a digest.
func (s *ScopeRevocationExecutorsTestSuite) TestPreGroupMembershipRemoval_CarriesTheMember() {
	s.groups.On("ValidateGroupMembershipChange", mock.Anything, testGroupID, testMemberID).
		Return(oneCaliforniaScope(testMemberID), nil)

	executor := newPreGroupMembershipRemovalExecutor(
		s.factory, defaultMaxRevocationCriteria, s.groups)
	resp, err := executor.Execute(s.nodeContext(map[string]string{
		revocationInputGroup: testGroupID, revocationInputMember: testMemberID,
	}))

	s.Require().NoError(err)
	plan := s.planFrom(resp)
	s.Equal(testGroupID, plan.TargetID)
	s.Equal(testMemberID, plan.AssigneeID)
}

// A retired scope is denied deployment-wide, in the scope dimension rather than the per-principal one.
// This is the only flow that writes it, and getting the dimension wrong would make the row match
// nothing at all.
func (s *ScopeRevocationExecutorsTestSuite) TestPreScopeDeletion_PlansInTheScopeDimension() {
	s.resources.On("ValidateDeleteAction", mock.Anything, testRSID, testResourceID, testActionID).
		Return(&revocation.ScopeRevocationTarget{
			Scopes: []revocation.AudienceScope{{Audience: testCaliforniaAud, Scope: testLicenceScope}},
		}, nil)

	executor := newPreScopeDeletionExecutor(s.factory, defaultMaxRevocationCriteria, s.resources)
	resp, err := executor.Execute(s.nodeContext(map[string]string{
		revocationInputResourceServer: testRSID,
		revocationInputResource:       testResourceID,
		revocationInputAction:         testActionID,
	}))

	s.Require().NoError(err)
	plan := s.planFrom(resp)
	s.Equal(revocation.ReasonScopeDeleted, plan.Reason)
	s.Equal([]revocation.Criterion{{
		Type:  revocation.CriterionTypeScope,
		Value: revocation.ScopeCriterionValue(testCaliforniaAud, testLicenceScope),
	}}, plan.Criteria)
	s.Equal(testRSID, plan.TargetID)
	s.Equal(testActionID, plan.ActionArgs[revocationInputAction])
	s.Equal(testResourceID, plan.ActionArgs[revocationInputResource])
}

// An action defined on the resource server itself has no resource, so the optional input must be
// allowed to be absent rather than pausing the flow for it.
func (s *ScopeRevocationExecutorsTestSuite) TestPreScopeDeletion_ResourceIsOptional() {
	s.resources.On("ValidateDeleteAction", mock.Anything, testRSID, "", testActionID).
		Return(&revocation.ScopeRevocationTarget{
			Scopes: []revocation.AudienceScope{{Audience: testCaliforniaAud, Scope: testLicenceScope}},
		}, nil)

	executor := newPreScopeDeletionExecutor(s.factory, defaultMaxRevocationCriteria, s.resources)
	resp, err := executor.Execute(s.nodeContext(map[string]string{
		revocationInputResourceServer: testRSID, revocationInputAction: testActionID,
	}))

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
	s.Empty(s.planFrom(resp).ActionArgs[revocationInputResource])
}

// A deployment-wide plan has no principals by design, so the emptiness that means "nothing to revoke"
// per principal must not be read that way here.
func (s *ScopeRevocationExecutorsTestSuite) TestPreScopeDeletion_NoPrincipalsIsNotNothingToRevoke() {
	s.resources.On("ValidateDeleteAction", mock.Anything, testRSID, "", testActionID).
		Return(&revocation.ScopeRevocationTarget{
			Scopes: []revocation.AudienceScope{{Audience: testCaliforniaAud, Scope: testLicenceScope}},
		}, nil)

	executor := newPreScopeDeletionExecutor(s.factory, defaultMaxRevocationCriteria, s.resources)
	resp, err := executor.Execute(s.nodeContext(map[string]string{
		revocationInputResourceServer: testRSID, revocationInputAction: testActionID,
	}))

	s.Require().NoError(err)
	plan := s.planFrom(resp)
	s.False(plan.NothingToRevoke)
	s.Len(plan.Criteria, 1)
}

// A scope whose resource server has no audience cannot be matched by any criterion, so the flow deletes
// the action and denies nothing.
func (s *ScopeRevocationExecutorsTestSuite) TestPreScopeDeletion_NothingToRevoke() {
	s.resources.On("ValidateDeleteAction", mock.Anything, testRSID, "", testActionID).
		Return(&revocation.ScopeRevocationTarget{}, nil)

	executor := newPreScopeDeletionExecutor(s.factory, defaultMaxRevocationCriteria, s.resources)
	resp, err := executor.Execute(s.nodeContext(map[string]string{
		revocationInputResourceServer: testRSID, revocationInputAction: testActionID,
	}))

	s.Require().NoError(err)
	plan := s.planFrom(resp)
	s.True(plan.NothingToRevoke)
	s.Empty(plan.Criteria)
}

// The cap is what stands between a role assigned to a large group and a revocation that takes minutes
// while every affected principal can still mint tokens. It must refuse before writing anything.
func (s *ScopeRevocationExecutorsTestSuite) TestPreScopeRevocation_RefusesAnOversizedFanOut() {
	entityIDs := make([]string, 6)
	for i := range entityIDs {
		entityIDs[i] = fmt.Sprintf("entity-%d", i)
	}
	s.roles.On("ValidateRoleScopeChange", mock.Anything, testRoleID).
		Return(&revocation.ScopeRevocationTarget{
			EntityIDs: entityIDs,
			Scopes: []revocation.AudienceScope{
				{Audience: testCaliforniaAud, Scope: testLicenceScope},
				{Audience: testOhioAud, Scope: testLicenceScope},
			},
		}, nil)

	executor := newPreRoleDeletionExecutor(s.factory, 10, s.roles)
	resp, err := executor.Execute(s.nodeContext(map[string]string{revocationInputRole: testRoleID}))

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, resp.Status)
	s.Require().NotNil(resp.Error)
	s.Equal(ErrRevocationFanOutTooLarge.Code, resp.Error.Code)
	s.Nil(resp.SharedRuntimeData, "no plan may be published for a refused change")
	// The operator needs to know how far the change reached, not only the ceiling. Both travel as i18n
	// params so every locale renders them, rather than only the default string.
	s.Equal("12", resp.Error.ErrorDescription.Params["criteria"])
	s.Equal("10", resp.Error.ErrorDescription.Params["max"])
	s.Contains(resp.Error.ErrorDescription.DefaultValue, "{{criteria}}")
	s.Contains(resp.Error.ErrorDescription.DefaultValue, "{{max}}")
}

// A fan-out exactly at the ceiling is allowed: the cap is a maximum, not an exclusive bound.
func (s *ScopeRevocationExecutorsTestSuite) TestPreScopeRevocation_AllowsExactlyTheCap() {
	s.roles.On("ValidateRoleScopeChange", mock.Anything, testRoleID).
		Return(oneCaliforniaScope("entity-1", "entity-2"), nil)

	executor := newPreRoleDeletionExecutor(s.factory, 2, s.roles)
	resp, err := executor.Execute(s.nodeContext(map[string]string{revocationInputRole: testRoleID}))

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
	s.Len(s.planFrom(resp).Criteria, 2)
}

// A deployment that configures no cap, or a nonsensical one, gets the built-in ceiling rather than an
// unbounded revocation.
func (s *ScopeRevocationExecutorsTestSuite) TestPreScopeRevocation_UnconfiguredCapFallsBack() {
	executor := newPreRoleDeletionExecutor(s.factory, 0, s.roles)
	s.Equal(defaultMaxRevocationCriteria, executor.maxCriteria)

	executor = newPreRoleDeletionExecutor(s.factory, -1, s.roles)
	s.Equal(defaultMaxRevocationCriteria, executor.maxCriteria)
}

// The validator's own refusal is what the console resolves to a message, so it must survive rather than
// being collapsed into the executor's generic one.
func (s *ScopeRevocationExecutorsTestSuite) TestPreScopeRevocation_CarriesValidatorRefusal() {
	refusal := tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "GRP-1015",
		Error: tidcommon.I18nMessage{DefaultValue: "Cannot modify declarative group"},
	}
	s.groups.On("ValidateGroupMembershipChange", mock.Anything, testGroupID, "").Return(nil, &refusal)

	executor := newPreGroupDeletionExecutor(s.factory, defaultMaxRevocationCriteria, s.groups)
	resp, err := executor.Execute(s.nodeContext(map[string]string{revocationInputGroup: testGroupID}))

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, resp.Status)
	s.Require().NotNil(resp.Error)
	s.Equal("GRP-1015", resp.Error.Code)
}

// A server-side failure is not a refusal the caller could act on, so it fails the flow rather than
// being reported as a decision.
func (s *ScopeRevocationExecutorsTestSuite) TestPreScopeRevocation_ServerErrorFailsTheFlow() {
	s.roles.On("ValidateRoleScopeChange", mock.Anything, testRoleID).
		Return(nil, &tidcommon.InternalServerError)

	executor := newPreRoleDeletionExecutor(s.factory, defaultMaxRevocationCriteria, s.roles)
	_, err := executor.Execute(s.nodeContext(map[string]string{revocationInputRole: testRoleID}))

	s.Require().Error(err)
	s.Contains(err.Error(), "failed to validate")
}

// Nothing in flow validation stops a preparatory node for one action being wired to the acting node of
// another. Each acting node must refuse a plan it was not produced for.
func (s *ScopeRevocationExecutorsTestSuite) TestActingNodes_RejectForeignPlans() {
	foreign := revocationPlan{
		Mode:     revocation.ModeBeforeAction,
		Reason:   revocation.ReasonRoleAssignmentRemoved,
		TargetID: testRoleID,
		Criteria: []revocation.Criterion{{Type: revocation.CriterionTypeEntityScope, Value: "digest"}},
	}
	nodes := map[string]providers.Executor{
		"role deletion":            newRoleDeletionExecutor(s.factory, s.roles),
		"role permission removal":  newRolePermissionRemovalExecutor(s.factory, s.roles),
		"group deletion":           newGroupDeletionExecutor(s.factory, s.groups),
		"group membership removal": newGroupMembershipRemovalExecutor(s.factory, s.groups),
		"scope deletion":           newScopeDeletionExecutor(s.factory, s.resources),
	}
	for name, node := range nodes {
		s.Run(name, func() {
			_, err := node.Execute(s.withPlan(foreign))
			s.Require().Error(err)
			s.Contains(err.Error(), "was produced for")
		})
	}
}

func (s *ScopeRevocationExecutorsTestSuite) TestRoleDeletion_DeletesTheRole() {
	s.roles.On("DeleteRole", mock.Anything, testRoleID).Return(nil)

	resp, err := newRoleDeletionExecutor(s.factory, s.roles).Execute(s.withPlan(revocationPlan{
		Mode:     revocation.ModeBeforeAction,
		Reason:   revocation.ReasonRoleDeleted,
		TargetID: testRoleID,
		Criteria: []revocation.Criterion{{Type: revocation.CriterionTypeEntityScope, Value: "digest"}},
	}))

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
}

func (s *ScopeRevocationExecutorsTestSuite) TestRolePermissionRemoval_AppliesTheCarriedSet() {
	s.roles.On("UpdateRolePermissions", mock.Anything, testRoleID,
		[]revocation.RolePermissions{{ResourceServerID: testRSID, Permissions: []string{testLicenceScope}}}).
		Return(nil)

	resp, err := newRolePermissionRemovalExecutor(s.factory, s.roles).
		Execute(s.withPlan(revocationPlan{
			Mode:       revocation.ModeBeforeAction,
			Reason:     revocation.ReasonRolePermissionRemoved,
			TargetID:   testRoleID,
			ActionArgs: map[string]string{revocationInputPermissions: testPermissionSet},
			Criteria: []revocation.Criterion{
				{Type: revocation.CriterionTypeEntityScope, Value: "digest"}},
		}))

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
}

// An edit that leaves the role granting nothing is the largest permission removal there is, so an
// empty set must be applied rather than skipped.
func (s *ScopeRevocationExecutorsTestSuite) TestRolePermissionRemoval_AppliesAnEmptySet() {
	s.roles.On("UpdateRolePermissions", mock.Anything, testRoleID,
		[]revocation.RolePermissions{}).Return(nil)

	resp, err := newRolePermissionRemovalExecutor(s.factory, s.roles).
		Execute(s.withPlan(revocationPlan{
			Mode:       revocation.ModeBeforeAction,
			Reason:     revocation.ReasonRolePermissionRemoved,
			TargetID:   testRoleID,
			ActionArgs: map[string]string{revocationInputPermissions: "[]"},
			Criteria: []revocation.Criterion{
				{Type: revocation.CriterionTypeEntityScope, Value: "digest"}},
		}))

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
}

// Every reason these flows record must be in the boundary set. A terminal row would deny tokens issued
// after the grant was restored, which is the failure the whole boundary design exists to avoid.
func (s *ScopeRevocationExecutorsTestSuite) TestEveryFlowReasonIsABoundaryReason() {
	for _, reason := range []revocation.Reason{
		revocation.ReasonRoleAssignmentRemoved,
		revocation.ReasonRoleDeleted,
		revocation.ReasonRolePermissionRemoved,
		revocation.ReasonGroupMembershipRemoved,
		revocation.ReasonScopeDeleted,
	} {
		s.True(revocation.IsBoundaryReason(reason), "%s must be bounded, not terminal", reason)
	}
}

// An input the engine accepted from runtime data must be readable here. Reading only the caller's own
// inputs would reject a graph whose earlier node supplied the target, after the gate had accepted it.
func (s *ScopeRevocationExecutorsTestSuite) TestPreScopeRevocation_ReadsAnInputFromRuntimeData() {
	s.roles.On("ValidateRoleScopeChange", mock.Anything, testRoleID).
		Return(oneCaliforniaScope(testAssigneeID), nil)

	executor := newPreRoleDeletionExecutor(s.factory, defaultMaxRevocationCriteria, s.roles)
	ctx := s.nodeContext(map[string]string{"unrelated": "x"})
	ctx.RuntimeData[revocationInputRole] = testRoleID

	resp, err := executor.Execute(ctx)

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
	s.Equal(testRoleID, s.planFrom(resp).TargetID)
}

// A malformed permission set must be refused before anything is revoked. Decoding it only in the
// acting node would leave every holder's tokens denied, the role unchanged, and nothing to restore
// them: the payload is the caller's, so a typo must not cost other people their sessions.
func (s *ScopeRevocationExecutorsTestSuite) TestPreRolePermissionRemoval_RefusesAMalformedSetBeforeRevoking() {
	executor := newPreRolePermissionRemovalExecutor(
		s.factory, defaultMaxRevocationCriteria, s.roles)

	resp, err := executor.Execute(s.nodeContext(map[string]string{
		revocationInputRole:        testRoleID,
		revocationInputPermissions: "{not json",
	}))

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, resp.Status)
	s.Require().NotNil(resp.Error)
	s.Equal(ErrInvalidRolePermissions.Code, resp.Error.Code)
	s.Nil(resp.SharedRuntimeData, "no plan may be published, so nothing downstream can revoke")
	// The validator must not even have been consulted: the refusal precedes it.
	s.roles.AssertNotCalled(s.T(), "ValidateRoleScopeChange", mock.Anything, mock.Anything)
}

// The engine prompts for any declared input the caller left out, optional ones included. An
// administration flow is driven by an API caller, so a declared-optional resource id would turn a
// single-call deletion into a pause the caller has no way to answer.
func (s *ScopeRevocationExecutorsTestSuite) TestPreScopeDeletion_ResourceIsNotADeclaredInput() {
	executor := newPreScopeDeletionExecutor(s.factory, defaultMaxRevocationCriteria, s.resources)

	declared := make([]string, 0, 2)
	for _, input := range executor.GetDefaultInputs() {
		s.True(input.Required, "%s must be required, or the engine will prompt for it", input.Identifier)
		declared = append(declared, input.Identifier)
	}
	s.ElementsMatch([]string{revocationInputResourceServer, revocationInputAction}, declared)
}
