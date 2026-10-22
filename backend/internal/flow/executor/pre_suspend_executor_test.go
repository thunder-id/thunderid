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

type PreSuspendExecutorTestSuite struct {
	suite.Suite
	governance *identitygovernanceprovidermock.IdentityGovernanceProviderMock
	apps       *applicationAdminProviderMock
	executor   *preSuspendExecutor
}

func TestPreSuspendExecutorTestSuite(t *testing.T) {
	suite.Run(t, new(PreSuspendExecutorTestSuite))
}

func (s *PreSuspendExecutorTestSuite) SetupTest() {
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), &config.Config{}))
	s.governance = identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(s.T())
	s.apps = newApplicationAdminProviderMock(s.T())
	s.executor = s.newExecutor(s.governance, 0)
}

func (s *PreSuspendExecutorTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (s *PreSuspendExecutorTestSuite) newExecutor(governance providers.IdentityGovernanceProvider,
	leeway time.Duration) *preSuspendExecutor {
	flowFactory, _ := core.Initialize(
		cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	executor := newPreSuspendExecutor(flowFactory, governance, leeway)
	executor.setApplicationProvider(s.apps)
	return executor
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

// The plan revokes by entity ID before the cutoff and is kept for a week.
func (s *PreSuspendExecutorTestSuite) TestPublishesABoundaryContainmentPlan() {
	s.governance.EXPECT().ValidateSuspend(mock.Anything, "u1").Return(nil).Once()
	s.apps.EXPECT().GetOAuthClientID(mock.Anything, "u1").Return("", nil).Once()

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

func (s *PreSuspendExecutorTestSuite) TestTheCutoffCoversTheJWTLeeway() {
	const leeway = 30 * time.Second
	executor := s.newExecutor(s.governance, leeway)
	s.governance.EXPECT().ValidateSuspend(mock.Anything, "u1").Return(nil).Once()
	s.apps.EXPECT().GetOAuthClientID(mock.Anything, "u1").Return("", nil).Once()

	before := time.Now().UTC()
	execResp, err := executor.Execute(preSuspendContext("u1"))
	after := time.Now().UTC()
	s.Require().NoError(err)

	plan := s.publishedPlan(execResp)
	s.False(plan.Cutoff.Before(before.Add(leeway)))
	s.False(plan.Cutoff.After(after.Add(leeway)))
}

// A zero or negative leeway keeps the plan time.
func (s *PreSuspendExecutorTestSuite) TestTheCutoffFollowsEachSuspendsOwnTime() {
	first := time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC)
	repeat := first.Add(time.Hour)

	s.Equal(first, buildSuspensionPlan("u1", "", first, 0).Cutoff)
	s.Equal(first, buildSuspensionPlan("u1", "", first, -time.Minute).Cutoff)
	s.Equal(first.Add(30*time.Second), buildSuspensionPlan("u1", "", first, 30*time.Second).Cutoff)
	s.Equal(repeat.Add(30*time.Second), buildSuspensionPlan("u1", "", repeat, 30*time.Second).Cutoff)
}

func (s *PreSuspendExecutorTestSuite) TestSuspendingAClientOwnerAlsoRevokesItsClientID() {
	cases := []struct {
		subject  string
		clientID string
	}{
		{subject: "app-1", clientID: "client-1"},
		{subject: "agent-1", clientID: "agent-client-1"},
	}
	for _, tc := range cases {
		s.apps.EXPECT().GetOAuthClientID(mock.Anything, tc.subject).Return(tc.clientID, nil).Once()
		s.governance.EXPECT().ValidateSuspend(mock.Anything, tc.subject).Return(nil).Once()

		execResp, err := s.executor.Execute(preSuspendContext(tc.subject))
		s.Require().NoError(err)

		plan := s.publishedPlan(execResp)
		s.Equal([]revocation.Criterion{
			{Type: revocation.CriterionTypeSubject, Value: tc.subject},
			{Type: revocation.CriterionTypeApplicationKey, Value: tc.clientID},
		}, plan.Criteria)
		s.Equal(int64(7*24*60*60), plan.TTLSeconds)
	}
}

func (s *PreSuspendExecutorTestSuite) TestSuspendingAUserRevokesByEntityIDOnly() {
	s.apps.EXPECT().GetOAuthClientID(mock.Anything, "u1").Return("", nil).Once()
	s.governance.EXPECT().ValidateSuspend(mock.Anything, "u1").Return(nil).Once()

	execResp, err := s.executor.Execute(preSuspendContext("u1"))
	s.Require().NoError(err)

	plan := s.publishedPlan(execResp)
	s.Equal([]revocation.Criterion{{Type: revocation.CriterionTypeSubject, Value: "u1"}}, plan.Criteria)
	s.Equal("u1", plan.TargetID)
	s.Equal(int64(7*24*60*60), plan.TTLSeconds)
}

func (s *PreSuspendExecutorTestSuite) TestAnUnresolvableClientPublishesNoPlan() {
	s.apps.EXPECT().GetOAuthClientID(mock.Anything, "app-1").Return("", &tidcommon.InternalServerError).Once()
	s.governance.EXPECT().ValidateSuspend(mock.Anything, "app-1").Return(nil).Once()

	execResp, err := s.executor.Execute(preSuspendContext("app-1"))

	s.Error(err)
	s.Nil(execResp)
}

func (s *PreSuspendExecutorTestSuite) TestARefusedTargetPublishesNoPlan() {
	s.governance.EXPECT().ValidateSuspend(mock.Anything, "u1").Return(&tidcommon.ErrorUnauthorized).Once()

	execResp, err := s.executor.Execute(preSuspendContext("u1"))

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, execResp.Status)
	s.Equal(ErrIdentityGovernanceFailed.Code, execResp.Error.Code)
	s.Empty(execResp.SharedRuntimeData)
	s.apps.AssertNotCalled(s.T(), "GetOAuthClientID", mock.Anything, mock.Anything)
}

func (s *PreSuspendExecutorTestSuite) TestAFailedValidationIsAFault() {
	s.governance.EXPECT().ValidateSuspend(mock.Anything, "u1").Return(&tidcommon.InternalServerError).Once()

	execResp, err := s.executor.Execute(preSuspendContext("u1"))

	s.Require().Error(err)
	s.Nil(execResp)
}

// The plan travels on SharedRuntimeData, which only an executor may write.
func (s *PreSuspendExecutorTestSuite) TestThePlanIsCarriedOnTheTrustedChannel() {
	s.governance.EXPECT().ValidateSuspend(mock.Anything, "u1").Return(nil).Once()
	s.apps.EXPECT().GetOAuthClientID(mock.Anything, "u1").Return("", nil).Once()

	execResp, err := s.executor.Execute(preSuspendContext("u1"))

	s.Require().NoError(err)
	s.NotContains(execResp.RuntimeData, common.RuntimeKeyRevocationPlan)
	s.NotContains(execResp.AdditionalData, common.RuntimeKeyRevocationPlan)
}

func (s *PreSuspendExecutorTestSuite) TestAMissingSubjectIsRequested() {
	ctx := preSuspendContext("")
	ctx.UserInputs = map[string]string{}

	execResp, err := s.executor.Execute(ctx)

	s.Require().NoError(err)
	s.Equal(providers.ExecUserInputRequired, execResp.Status)
	s.governance.AssertNotCalled(s.T(), "ValidateSuspend", mock.Anything, mock.Anything)
}

// The target is taken from the subject input alone.
func (s *PreSuspendExecutorTestSuite) TestAnEmptyOrNonInputSubjectIsAnError() {
	runtimeOnly := preSuspendContext("")
	runtimeOnly.UserInputs = map[string]string{}
	runtimeOnly.RuntimeData = map[string]string{revocationInputSubject: "u1"}

	cases := []struct {
		name string
		ctx  *providers.NodeContext
	}{
		{name: "empty input", ctx: preSuspendContext("")},
		{name: "runtime data only", ctx: runtimeOnly},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			execResp, err := s.executor.Execute(tc.ctx)

			s.Require().Error(err)
			s.Nil(execResp)
		})
	}
	s.governance.AssertNotCalled(s.T(), "ValidateSuspend", mock.Anything, mock.Anything)
}

func (s *PreSuspendExecutorTestSuite) TestAClientErrorResolvingTheClientIsAFault() {
	s.apps.EXPECT().GetOAuthClientID(mock.Anything, "u1").Return("", &tidcommon.ErrorUnauthorized).Once()
	s.governance.EXPECT().ValidateSuspend(mock.Anything, "u1").Return(nil).Once()

	execResp, err := s.executor.Execute(preSuspendContext("u1"))

	s.Error(err)
	s.Nil(execResp)
}

func (s *PreSuspendExecutorTestSuite) TestNoAuthorityIsAnError() {
	executor := s.newExecutor(nil, 0)

	_, err := executor.Execute(preSuspendContext("u1"))

	s.Error(err)
}

func (s *PreSuspendExecutorTestSuite) TestNoApplicationServiceRevokesByEntityIDOnly() {
	s.executor.setApplicationProvider(nil)
	s.governance.EXPECT().ValidateSuspend(mock.Anything, "app-1").Return(nil).Once()

	execResp, err := s.executor.Execute(preSuspendContext("app-1"))
	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, execResp.Status)

	plan := s.publishedPlan(execResp)
	s.Equal([]revocation.Criterion{{Type: revocation.CriterionTypeSubject, Value: "app-1"}}, plan.Criteria)
	s.Equal("app-1", plan.TargetID)
}

func (s *PreSuspendExecutorTestSuite) TestTheExecutorIsAdministrationOnly() {
	meta := s.executor.GetMeta()
	s.Require().NotNil(meta)
	s.Equal([]providers.FlowType{providers.FlowTypeAdministration}, meta.SupportedFlowTypes)
}
