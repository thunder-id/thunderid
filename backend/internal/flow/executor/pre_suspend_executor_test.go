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
	"github.com/thunder-id/thunderid/tests/mocks/identitygovernanceprovidermock"
)

// PreSuspendExecutorTestSuite covers the suspend flow's preparatory node: the boundary check before
// containment, and the plan the containment nodes and the suspend node consume.
type PreSuspendExecutorTestSuite struct {
	suite.Suite
	governance *identitygovernanceprovidermock.IdentityGovernanceProviderMock
	executor   *preSuspendExecutor
}

func TestPreSuspendExecutorTestSuite(t *testing.T) {
	suite.Run(t, new(PreSuspendExecutorTestSuite))
}

func (s *PreSuspendExecutorTestSuite) SetupTest() {
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), &config.Config{}))
	s.governance = identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(s.T())
	s.executor = s.newExecutor(s.governance, 0)
}

func (s *PreSuspendExecutorTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (s *PreSuspendExecutorTestSuite) newExecutor(governance providers.IdentityGovernanceProvider,
	leeway time.Duration) *preSuspendExecutor {
	flowFactory, _ := core.Initialize(
		cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	return newPreSuspendExecutor(flowFactory, governance, leeway)
}

func preSuspendContext(subject string) *providers.NodeContext {
	return &providers.NodeContext{
		Context:     context.Background(),
		ExecutionID: "flow-1",
		FlowType:    providers.FlowTypeAdministration,
		UserInputs:  map[string]string{revocationInputSubject: subject},
	}
}

func (s *PreSuspendExecutorTestSuite) publishedPlan(execResp *providers.ExecutorResponse) revocationPlan {
	encoded, published := execResp.SharedRuntimeData[common.RuntimeKeyRevocationPlan]
	s.Require().True(published, "the containment nodes have nothing to read without a plan")
	var plan revocationPlan
	s.Require().NoError(json.Unmarshal([]byte(encoded), &plan))
	return plan
}

// The plan is a boundary revocation of what the subject held before the hold, named by entity ID,
// kept for a week whatever issued the subject's tokens.
func (s *PreSuspendExecutorTestSuite) TestPublishesABoundaryContainmentPlan() {
	s.governance.On("ValidateSuspend", mock.Anything, "u1").Return((*tidcommon.ServiceError)(nil)).Once()

	before := time.Now().UTC()
	execResp, err := s.executor.Execute(preSuspendContext("u1"))
	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, execResp.Status)

	plan := s.publishedPlan(execResp)
	s.Equal(revocation.ModeBeforeAction, plan.Mode)
	s.Equal(revocation.ReasonIdentitySuspended, plan.Reason)
	s.False(plan.Cutoff.Before(before))
	s.Equal([]revocation.Criterion{{Type: revocation.CriterionTypeSubject, Value: "u1"}}, plan.Criteria)
	s.Equal("u1", plan.TargetID)
	s.Equal(int64(7*24*60*60), plan.TTLSeconds)
}

// S15: the cutoff is moved forward by the JWT leeway, which covers the nodes between this one and
// the hold write as well as a token signed just after the hold.
func (s *PreSuspendExecutorTestSuite) TestTheCutoffCoversTheJWTLeeway() {
	const leeway = 30 * time.Second
	executor := s.newExecutor(s.governance, leeway)
	s.governance.On("ValidateSuspend", mock.Anything, "u1").Return((*tidcommon.ServiceError)(nil)).Once()

	before := time.Now().UTC()
	execResp, err := executor.Execute(preSuspendContext("u1"))
	after := time.Now().UTC()
	s.Require().NoError(err)

	plan := s.publishedPlan(execResp)
	s.False(plan.Cutoff.Before(before.Add(leeway)))
	s.False(plan.Cutoff.After(after.Add(leeway)))
}

// S16: a zero or negative leeway keeps the cutoff at the plan time, and each suspend, a repeat
// included, takes its own.
func (s *PreSuspendExecutorTestSuite) TestTheCutoffFollowsEachSuspendsOwnTime() {
	first := time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC)
	repeat := first.Add(time.Hour)

	s.Equal(first, buildSuspensionPlan("u1", "", first, 0).Cutoff)
	s.Equal(first, buildSuspensionPlan("u1", "", first, -time.Minute).Cutoff)
	s.Equal(first.Add(30*time.Second), buildSuspensionPlan("u1", "", first, 30*time.Second).Cutoff)
	s.Equal(repeat.Add(30*time.Second), buildSuspensionPlan("u1", "", repeat, 30*time.Second).Cutoff)
}

// Suspending an application or an agent also revokes the client ID it owns, as deleting an
// application does. For an agent this reaches the tokens it obtained
// as a client for other subjects.
func (s *PreSuspendExecutorTestSuite) TestSuspendingAClientOwnerAlsoRevokesItsClientID() {
	for subject, clientID := range map[string]string{"app-1": "client-1", "agent-1": "agent-client-1"} {
		apps := newApplicationAdminProviderMock(s.T())
		apps.On("GetOAuthClientID", mock.Anything, subject).Return(clientID, (*tidcommon.ServiceError)(nil)).Once()
		s.executor.setApplicationProvider(apps)
		s.governance.On("ValidateSuspend", mock.Anything, subject).Return((*tidcommon.ServiceError)(nil)).Once()

		execResp, err := s.executor.Execute(preSuspendContext(subject))
		s.Require().NoError(err)

		plan := s.publishedPlan(execResp)
		s.Equal([]revocation.Criterion{
			{Type: revocation.CriterionTypeSubject, Value: subject},
			{Type: revocation.CriterionTypeApplicationKey, Value: clientID},
		}, plan.Criteria)
		s.Equal(int64(7*24*60*60), plan.TTLSeconds)
	}
}

// A user owns no OAuth client: its suspension revokes by entity ID only.
func (s *PreSuspendExecutorTestSuite) TestSuspendingAUserRevokesByEntityIDOnly() {
	apps := newApplicationAdminProviderMock(s.T())
	apps.On("GetOAuthClientID", mock.Anything, "u1").Return("", (*tidcommon.ServiceError)(nil)).Once()
	s.executor.setApplicationProvider(apps)
	s.governance.On("ValidateSuspend", mock.Anything, "u1").Return((*tidcommon.ServiceError)(nil)).Once()

	execResp, err := s.executor.Execute(preSuspendContext("u1"))
	s.Require().NoError(err)

	plan := s.publishedPlan(execResp)
	s.Equal([]revocation.Criterion{{Type: revocation.CriterionTypeSubject, Value: "u1"}}, plan.Criteria)
	s.Equal("u1", plan.TargetID)
	s.Equal(int64(7*24*60*60), plan.TTLSeconds)
}

// A client that cannot be resolved faults the node, so nothing downstream runs.
func (s *PreSuspendExecutorTestSuite) TestAnUnresolvableClientPublishesNoPlan() {
	apps := newApplicationAdminProviderMock(s.T())
	apps.On("GetOAuthClientID", mock.Anything, "app-1").Return("", &tidcommon.InternalServerError).Once()
	s.executor.setApplicationProvider(apps)
	s.governance.On("ValidateSuspend", mock.Anything, "app-1").Return((*tidcommon.ServiceError)(nil)).Once()

	execResp, err := s.executor.Execute(preSuspendContext("app-1"))

	s.Error(err)
	s.Nil(execResp)
}

// A target outside the operator's boundary is refused before containment: no plan, so no token or
// session of an identity the operator may not govern is touched.
func (s *PreSuspendExecutorTestSuite) TestARefusedTargetPublishesNoPlan() {
	apps := newApplicationAdminProviderMock(s.T())
	s.executor.setApplicationProvider(apps)
	s.governance.On("ValidateSuspend", mock.Anything, "u1").Return(
		&tidcommon.ErrorUnauthorized).Once()

	execResp, err := s.executor.Execute(preSuspendContext("u1"))

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, execResp.Status)
	s.Equal(ErrIdentityGovernanceFailed.Code, execResp.Error.Code)
	s.Empty(execResp.SharedRuntimeData)
	apps.AssertNotCalled(s.T(), "GetOAuthClientID", mock.Anything, mock.Anything)
}

// A server error validating the target is a fault, and publishes no plan.
func (s *PreSuspendExecutorTestSuite) TestAFailedValidationIsAFault() {
	s.governance.On("ValidateSuspend", mock.Anything, "u1").Return(&tidcommon.InternalServerError).Once()

	execResp, err := s.executor.Execute(preSuspendContext("u1"))

	s.Require().Error(err)
	s.Nil(execResp)
}

// The plan travels on SharedRuntimeData, which only an executor may write, so it binds every node
// that follows rather than being something a caller can influence per request.
func (s *PreSuspendExecutorTestSuite) TestThePlanIsCarriedOnTheTrustedChannel() {
	s.governance.On("ValidateSuspend", mock.Anything, "u1").Return((*tidcommon.ServiceError)(nil)).Once()

	execResp, err := s.executor.Execute(preSuspendContext("u1"))

	s.Require().NoError(err)
	s.NotContains(execResp.RuntimeData, common.RuntimeKeyRevocationPlan)
	s.NotContains(execResp.AdditionalData, common.RuntimeKeyRevocationPlan)
}

// The target is a required input; without it the node asks rather than guessing.
func (s *PreSuspendExecutorTestSuite) TestAMissingSubjectIsRequested() {
	ctx := preSuspendContext("")
	ctx.UserInputs = map[string]string{}

	execResp, err := s.executor.Execute(ctx)

	s.Require().NoError(err)
	s.Equal(providers.ExecUserInputRequired, execResp.Status)
	s.governance.AssertNotCalled(s.T(), "ValidateSuspend", mock.Anything, mock.Anything)
}

// A subject that is present but empty, or only in runtime data, is a malformed flow: the target is
// taken from the gated input alone, so the node faults before checking anyone.
func (s *PreSuspendExecutorTestSuite) TestAnEmptyOrNonInputSubjectIsAnError() {
	runtimeOnly := preSuspendContext("")
	runtimeOnly.UserInputs = map[string]string{}
	runtimeOnly.RuntimeData = map[string]string{revocationInputSubject: "u1"}

	for name, ctx := range map[string]*providers.NodeContext{
		"empty input": preSuspendContext(""), "runtime data only": runtimeOnly,
	} {
		execResp, err := s.executor.Execute(ctx)

		s.Require().Error(err, name)
		s.Nil(execResp, name)
	}
	s.governance.AssertNotCalled(s.T(), "ValidateSuspend", mock.Anything, mock.Anything)
}

// A client error resolving the subject's client means it owns none, so the plan revokes by entity ID
// only rather than faulting the suspension.
func (s *PreSuspendExecutorTestSuite) TestAClientErrorResolvingTheClientRevokesByEntityIDOnly() {
	apps := newApplicationAdminProviderMock(s.T())
	apps.On("GetOAuthClientID", mock.Anything, "u1").Return("", &tidcommon.ErrorUnauthorized).Once()
	s.executor.setApplicationProvider(apps)
	s.governance.On("ValidateSuspend", mock.Anything, "u1").Return((*tidcommon.ServiceError)(nil)).Once()

	execResp, err := s.executor.Execute(preSuspendContext("u1"))
	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, execResp.Status)

	plan := s.publishedPlan(execResp)
	s.Equal([]revocation.Criterion{{Type: revocation.CriterionTypeSubject, Value: "u1"}}, plan.Criteria)
}

// An unwired authority is a wiring fault, not a permit.
func (s *PreSuspendExecutorTestSuite) TestNoAuthorityIsAnError() {
	executor := s.newExecutor(nil, 0)

	_, err := executor.Execute(preSuspendContext("u1"))

	s.Error(err)
}

// Administration flows only: a suspend plan in an unauthenticated flow would let anyone revoke the
// tokens of an identity they can name.
func (s *PreSuspendExecutorTestSuite) TestTheExecutorIsAdministrationOnly() {
	meta := s.executor.GetMeta()
	s.Require().NotNil(meta)
	s.Equal([]providers.FlowType{providers.FlowTypeAdministration}, meta.SupportedFlowTypes)
}
