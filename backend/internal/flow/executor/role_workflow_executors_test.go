// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/revocationmock"
)

const (
	testRoleID        = "role-1"
	testAssigneeID    = "01a01978-b3a5-7d2a-a4cf-0c57328e94cf"
	testCaliforniaAud = "https://api.dmv.ca.gov"
	testOhioAud       = "https://api.dmv.oh.gov"
)

type RoleWorkflowExecutorsTestSuite struct {
	suite.Suite
	factory core.FlowFactoryInterface
	roles   *roleAdminProviderMock
}

func TestRoleWorkflowExecutorsTestSuite(t *testing.T) {
	suite.Run(t, new(RoleWorkflowExecutorsTestSuite))
}

func (s *RoleWorkflowExecutorsTestSuite) SetupTest() {
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), &config.Config{}))
	s.T().Cleanup(config.ResetServerRuntime)
	s.factory, _ = core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	s.roles = newRoleAdminProviderMock(s.T())
}

func (s *RoleWorkflowExecutorsTestSuite) nodeContext(inputs map[string]string) *providers.NodeContext {
	return &providers.NodeContext{
		Context:           context.Background(),
		UserInputs:        inputs,
		RuntimeData:       map[string]string{},
		SharedRuntimeData: map[string]string{},
	}
}

func (s *RoleWorkflowExecutorsTestSuite) planFrom(resp *providers.ExecutorResponse) revocationPlan {
	s.Require().NotNil(resp.SharedRuntimeData)
	encoded, ok := resp.SharedRuntimeData[common.RuntimeKeyRevocationPlan]
	s.Require().True(ok, "the preparatory node must publish a plan")
	var plan revocationPlan
	s.Require().NoError(json.Unmarshal([]byte(encoded), &plan))
	return plan
}

// The plan is the cross product of the principals losing the role and the scopes it carried, because
// a criterion names one scope for one principal on one resource server.
func (s *RoleWorkflowExecutorsTestSuite) TestPreRoleAssignmentRemoval_PlansOneCriterionPerEntityScope() {
	s.roles.On("ValidateRemoveRoleAssignment", mock.Anything, testRoleID, testAssigneeID).
		Return(&revocation.ScopeRevocationTarget{
			EntityIDs: []string{testAssigneeID, "entity-2"},
			Scopes: []revocation.AudienceScope{
				{Audience: testCaliforniaAud, Scope: "license"},
				{Audience: testOhioAud, Scope: "license"},
			},
		}, nil)

	executor := newPreRoleAssignmentRemovalExecutor(s.factory, defaultMaxRevocationCriteria, s.roles)
	resp, err := executor.Execute(s.nodeContext(map[string]string{
		revocationInputRole: testRoleID, revocationInputAssignee: testAssigneeID,
	}))

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

	// The same scope name on two resource servers must produce two distinct criteria. This is the
	// case the dimension exists for: "license" is a different scope in California and in Ohio.
	s.Contains(plan.Criteria, revocation.Criterion{
		Type:  revocation.CriterionTypeEntityScope,
		Value: revocation.EntityScopeCriterionValue(testAssigneeID, testCaliforniaAud, "license"),
	})
	s.Contains(plan.Criteria, revocation.Criterion{
		Type:  revocation.CriterionTypeEntityScope,
		Value: revocation.EntityScopeCriterionValue(testAssigneeID, testOhioAud, "license"),
	})
}

// A role carrying no permissions has nothing to deny, but the unassignment must still happen. The
// flag is what distinguishes that from a plan that lost its criteria on the way.
func (s *RoleWorkflowExecutorsTestSuite) TestPreRoleAssignmentRemoval_NothingToRevoke() {
	s.roles.On("ValidateRemoveRoleAssignment", mock.Anything, testRoleID, testAssigneeID).
		Return(&revocation.ScopeRevocationTarget{EntityIDs: []string{testAssigneeID}}, nil)

	executor := newPreRoleAssignmentRemovalExecutor(s.factory, defaultMaxRevocationCriteria, s.roles)
	resp, err := executor.Execute(s.nodeContext(map[string]string{
		revocationInputRole: testRoleID, revocationInputAssignee: testAssigneeID,
	}))

	s.Require().NoError(err)
	plan := s.planFrom(resp)
	s.Empty(plan.Criteria)
	s.True(plan.NothingToRevoke)
}

// The validator's own refusal is what the console resolves to a message, so it must survive rather
// than being collapsed into this executor's generic one.
func (s *RoleWorkflowExecutorsTestSuite) TestPreRoleAssignmentRemoval_CarriesValidatorRefusal() {
	refusal := tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "ROL-1003",
		Error: tidcommon.I18nMessage{DefaultValue: "Role not found"},
	}
	s.roles.On("ValidateRemoveRoleAssignment", mock.Anything, testRoleID, testAssigneeID).
		Return(nil, &refusal)

	executor := newPreRoleAssignmentRemovalExecutor(s.factory, defaultMaxRevocationCriteria, s.roles)
	resp, err := executor.Execute(s.nodeContext(map[string]string{
		revocationInputRole: testRoleID, revocationInputAssignee: testAssigneeID,
	}))

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, resp.Status)
	s.Require().NotNil(resp.Error)
	s.Equal("ROL-1003", resp.Error.Code)
}

// Nothing in flow validation stops a preparatory node for one action being wired to the acting node
// of another. The acting node must refuse a plan it was not produced for.
func (s *RoleWorkflowExecutorsTestSuite) TestRoleAssignmentRemoval_RejectsForeignPlan() {
	encoded, err := encodeRevocationPlan(revocationPlan{
		Mode:     revocation.ModeAll,
		Reason:   revocation.ReasonApplicationDeleted,
		TargetID: "app-1",
		Criteria: []revocation.Criterion{{Type: revocation.CriterionTypeApplicationKey, Value: "client-1"}},
	})
	s.Require().NoError(err)

	executor := newRoleAssignmentRemovalExecutor(s.factory, s.roles)
	ctx := s.nodeContext(nil)
	ctx.SharedRuntimeData[common.RuntimeKeyRevocationPlan] = encoded
	_, execErr := executor.Execute(ctx)

	s.Require().Error(execErr)
	s.Contains(execErr.Error(), "was produced for")
}

func (s *RoleWorkflowExecutorsTestSuite) TestRoleAssignmentRemoval_RemovesTheAssignment() {
	s.roles.On("RemoveRoleAssignment", mock.Anything, testRoleID, testAssigneeID).Return(nil)
	encoded, err := encodeRevocationPlan(revocationPlan{
		Mode:       revocation.ModeBeforeAction,
		Reason:     revocation.ReasonRoleAssignmentRemoved,
		TargetID:   testRoleID,
		AssigneeID: testAssigneeID,
		Cutoff:     time.Now().UTC(),
		Criteria: []revocation.Criterion{{
			Type:  revocation.CriterionTypeEntityScope,
			Value: revocation.EntityScopeCriterionValue(testAssigneeID, testCaliforniaAud, "license"),
		}},
	})
	s.Require().NoError(err)

	executor := newRoleAssignmentRemovalExecutor(s.factory, s.roles)
	ctx := s.nodeContext(nil)
	ctx.SharedRuntimeData[common.RuntimeKeyRevocationPlan] = encoded
	resp, execErr := executor.Execute(ctx)

	s.Require().NoError(execErr)
	s.Equal(providers.ExecComplete, resp.Status)
}

// The re-stamp node closes the window between the revocation and the action it precedes: a token
// minted while the grant was still held has an iat past the original cutoff, so the cutoff must move
// forward once the action has committed.
func (s *RoleWorkflowExecutorsTestSuite) TestCriteriaRevocationRestamp_AdvancesTheCutoff() {
	revoker := revocationmock.NewCriteriaRevokerInterfaceMock(s.T())
	planCutoff := time.Now().UTC().Add(-time.Hour)
	var recorded time.Time
	revoker.On("RevokeCriteriaBatch", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			recorded = args.Get(1).([]revocation.CriteriaRevocation)[0].Cutoff
		}).Return(nil)

	encoded, err := encodeRevocationPlan(revocationPlan{
		Mode:     revocation.ModeBeforeAction,
		Reason:   revocation.ReasonRoleAssignmentRemoved,
		TargetID: testRoleID,
		Cutoff:   planCutoff,
		Criteria: []revocation.Criterion{{
			Type:  revocation.CriterionTypeEntityScope,
			Value: revocation.EntityScopeCriterionValue(testAssigneeID, testCaliforniaAud, "license"),
		}},
	})
	s.Require().NoError(err)

	executor := newCriteriaRevocationRestampExecutor(s.factory, revoker)
	ctx := s.nodeContext(nil)
	ctx.SharedRuntimeData[common.RuntimeKeyRevocationPlan] = encoded
	resp, execErr := executor.Execute(ctx)

	s.Require().NoError(execErr)
	s.Equal(providers.ExecComplete, resp.Status)
	s.True(recorded.After(planCutoff), "the re-stamp must advance the cutoff, not reuse the plan's")
}

// A terminal plan has no cutoff to advance, and rewriting one would be refused as a mode mismatch.
func (s *RoleWorkflowExecutorsTestSuite) TestCriteriaRevocationRestamp_SkipsTerminalPlans() {
	revoker := revocationmock.NewCriteriaRevokerInterfaceMock(s.T())
	encoded, err := encodeRevocationPlan(revocationPlan{
		Mode:     revocation.ModeAll,
		Reason:   revocation.ReasonApplicationDeleted,
		TargetID: "app-1",
		Criteria: []revocation.Criterion{{Type: revocation.CriterionTypeApplicationKey, Value: "client-1"}},
	})
	s.Require().NoError(err)

	executor := newCriteriaRevocationRestampExecutor(s.factory, revoker)
	ctx := s.nodeContext(nil)
	ctx.SharedRuntimeData[common.RuntimeKeyRevocationPlan] = encoded
	resp, execErr := executor.Execute(ctx)

	s.Require().NoError(execErr)
	s.Equal(providers.ExecComplete, resp.Status)
	revoker.AssertNotCalled(s.T(), "RevokeCriteriaBatch", mock.Anything, mock.Anything)
}

// An executor registered without its seam must refuse rather than panic. The engine can be embedded
// with the role flows registered and no role service behind them, and the refusal is checked before
// the plan is read so the node fails the same way whatever the plan says.
func (s *RoleWorkflowExecutorsTestSuite) TestRoleAssignmentRemoval_RefusesWithoutItsSeam() {
	executor := newRoleAssignmentRemovalExecutor(s.factory, nil)

	_, err := executor.Execute(s.nodeContext(nil))

	s.Require().Error(err)
	s.EqualError(err, "role service is not configured")
}

// The preparatory node reports the same absence, as an error rather than a refusal: a missing seam is
// a deployment fault the operator cannot correct by changing the request, so it must not be reported
// on the surface that invites them to retry.
func (s *RoleWorkflowExecutorsTestSuite) TestPreRoleAssignmentRemoval_RefusesWithoutItsSeam() {
	executor := newPreRoleAssignmentRemovalExecutor(s.factory, defaultMaxRevocationCriteria, nil)

	_, err := executor.Execute(s.nodeContext(map[string]string{
		revocationInputRole: testRoleID, revocationInputAssignee: testAssigneeID,
	}))

	s.Require().Error(err)
	s.Contains(err.Error(), "role service is not configured")
}
