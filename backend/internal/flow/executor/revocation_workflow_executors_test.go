// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/flow/sessionmock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/revocationmock"
)

type RevocationWorkflowExecutorsTestSuite struct {
	suite.Suite
	factory core.FlowFactoryInterface
	users   *userDeletionProviderMock
}

func TestRevocationWorkflowExecutorsTestSuite(t *testing.T) {
	suite.Run(t, new(RevocationWorkflowExecutorsTestSuite))
}

func (s *RevocationWorkflowExecutorsTestSuite) SetupTest() {
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), &config.Config{}))
	s.T().Cleanup(config.ResetServerRuntime)
	s.factory, _ = core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	s.users = newUserDeletionProviderMock(s.T())
}

func (s *RevocationWorkflowExecutorsTestSuite) TestUserDeletionFlow() {
	s.users.EXPECT().ValidateDeleteUser(mock.Anything, "user-123").Return(nil)
	validationResp, err := newPreDeleteExecutor(s.factory, s.users).Execute(&providers.NodeContext{
		Context:     context.Background(),
		UserInputs:  map[string]string{revocationInputSubject: "user-123"},
		RuntimeData: map[string]string{},
	})
	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, validationResp.Status)
	s.NotEmpty(validationResp.SharedRuntimeData[common.RuntimeKeyRevocationPlan])

	revoker := revocationmock.NewCriteriaRevokerInterfaceMock(s.T())
	revoker.EXPECT().RevokeCriteriaBatch(mock.Anything,
		mock.MatchedBy(func(batch []revocation.CriteriaRevocation) bool {
			return len(batch) == 1 && batch[0].Criterion.Type == revocation.CriterionTypeSubject &&
				batch[0].Criterion.Value == "user-123" && batch[0].Mode == revocation.ModeAll &&
				batch[0].Reason == revocation.ReasonUserDeleted
		})).Return(nil)
	criteriaResp, err := newCriteriaRevocationExecutor(s.factory, revoker).Execute(&providers.NodeContext{
		Context: context.Background(), SharedRuntimeData: validationResp.SharedRuntimeData,
	})
	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, criteriaResp.Status)

	sessions := sessionmock.NewServiceMock(s.T())
	sessions.EXPECT().TerminateBySubject(mock.Anything, "user-123").Return(nil)
	sessionResp, err := newSessionRevocationExecutor(s.factory, sessions).Execute(&providers.NodeContext{
		Context: context.Background(), SharedRuntimeData: validationResp.SharedRuntimeData,
	})
	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, sessionResp.Status)

	s.users.EXPECT().DeleteUser(mock.Anything, "user-123").Return(nil)
	deleteResp, err := newUserDeleteExecutor(s.factory, s.users).Execute(&providers.NodeContext{
		Context: context.Background(), SharedRuntimeData: validationResp.SharedRuntimeData,
	})
	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, deleteResp.Status)
}

func (s *RevocationWorkflowExecutorsTestSuite) TestValidationRequestsInputs() {
	resp, err := newPreDeleteExecutor(s.factory, s.users).Execute(
		&providers.NodeContext{Context: context.Background()})
	s.Require().NoError(err)
	s.Equal(providers.ExecUserInputRequired, resp.Status)
	// Only the deletion target is requested at execution time. The revocation breadth is fixed by the
	// flow definition, so it must never be surfaced as something the caller can supply.
	s.Len(resp.Inputs, 1)
	s.Equal(revocationInputSubject, resp.Inputs[0].Identifier)
}

// A mode the executor does not support is rejected. Flow creation validates the mode, so this only
// guards a flow persisted before the mode was constrained.
func (s *RevocationWorkflowExecutorsTestSuite) TestValidationRejectsUnsupportedMode() {
	resp, err := newPreDeleteExecutor(s.factory, s.users).Execute(&providers.NodeContext{
		Context:      context.Background(),
		ExecutorMode: "revoke_before_action",
		UserInputs:   map[string]string{revocationInputSubject: "user-123"},
	})
	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, resp.Status)
	s.Equal(ErrInvalidRevocationMode.Code, resp.Error.Code)
}

// A node that omits the mode falls back to the executor's default rather than failing.
func (s *RevocationWorkflowExecutorsTestSuite) TestValidationDefaultsModeWhenNodeOmitsIt() {
	s.users.EXPECT().ValidateDeleteUser(mock.Anything, "user-123").Return(nil)

	resp, err := newPreDeleteExecutor(s.factory, s.users).Execute(&providers.NodeContext{
		Context:    context.Background(),
		UserInputs: map[string]string{revocationInputSubject: "user-123"},
	})

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
	plan, decodeErr := decodeRevocationPlan(resp.SharedRuntimeData)
	s.Require().NoError(decodeErr)
	s.Equal(revocation.ModeAll, plan.Mode)
}

// The caller cannot widen or narrow the revocation by supplying it as an input: the field is not a
// declared input, so it is ignored and the flow-configured mode stands.
func (s *RevocationWorkflowExecutorsTestSuite) TestValidationIgnoresModeSuppliedAsInput() {
	s.users.EXPECT().ValidateDeleteUser(mock.Anything, "user-123").Return(nil)

	resp, err := newPreDeleteExecutor(s.factory, s.users).Execute(&providers.NodeContext{
		Context:      context.Background(),
		ExecutorMode: string(revocation.ModeAll),
		UserInputs: map[string]string{
			revocationInputSubject: "user-123",
			"revocationMode":       "revoke_before_action",
		},
	})

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
	plan, decodeErr := decodeRevocationPlan(resp.SharedRuntimeData)
	s.Require().NoError(decodeErr)
	s.Equal(revocation.ModeAll, plan.Mode)
}

func (s *RevocationWorkflowExecutorsTestSuite) TestCriteriaRevocationFailure() {
	encoded, err := encodeRevocationPlan(revocationPlan{
		Criteria: []revocation.Criterion{{Type: revocation.CriterionTypeSubject, Value: "user-123"}},
		Mode:     revocation.ModeAll, Reason: revocation.ReasonUserDeleted,
	})
	s.Require().NoError(err)
	revoker := revocationmock.NewCriteriaRevokerInterfaceMock(s.T())
	revoker.EXPECT().RevokeCriteriaBatch(mock.Anything, mock.Anything).Return(errors.New("store unavailable"))

	resp, err := newCriteriaRevocationExecutor(s.factory, revoker).Execute(&providers.NodeContext{
		Context: context.Background(), SharedRuntimeData: map[string]string{common.RuntimeKeyRevocationPlan: encoded},
	})
	s.Nil(resp)
	s.ErrorContains(err, "failed to revoke tokens by criteria")
}

func (s *RevocationWorkflowExecutorsTestSuite) TestRejectsMissingTrustedPlan() {
	resp, err := newCriteriaRevocationExecutor(s.factory,
		revocationmock.NewCriteriaRevokerInterfaceMock(s.T())).Execute(&providers.NodeContext{
		Context: context.Background(), SharedRuntimeData: map[string]string{},
	})
	s.Nil(resp)
	s.ErrorContains(err, "trusted revocation plan is missing")
}

// Flow-creation validation enforces the revocation breadth from this declaration, so the declared
// modes are part of the executor's contract rather than an implementation detail.
func (s *RevocationWorkflowExecutorsTestSuite) TestValidationExecutorDeclaresModeContract() {
	meta := newPreDeleteExecutor(s.factory, s.users).GetMeta()

	s.Require().NotNil(meta)
	s.Equal(string(revocation.ModeAll), meta.DefaultMode)
	s.Equal([]string{string(revocation.ModeAll)}, meta.SupportedModes)
	s.Equal([]providers.FlowType{providers.FlowTypeAdministration}, meta.SupportedFlowTypes)
}

// The breadth the pre-executor records must survive the hop to the node that writes the revocation.
// This pins the consumer half of that path: whatever mode and cutoff the plan carries is what reaches
// the revoker, so a future mode needs no further plumbing.
func (s *RevocationWorkflowExecutorsTestSuite) TestCriteriaRevocationUsesPlanMode() {
	cutoff := time.Date(2026, time.August, 6, 10, 0, 0, 0, time.UTC)
	encoded, err := encodeRevocationPlan(revocationPlan{
		Criteria: []revocation.Criterion{{Type: revocation.CriterionTypeSubject, Value: "user-123"}},
		Mode:     revocation.ModeBeforeAction,
		Cutoff:   cutoff,
		Reason:   revocation.ReasonRoleAssignmentRemoved,
	})
	s.Require().NoError(err)

	revoker := revocationmock.NewCriteriaRevokerInterfaceMock(s.T())
	revoker.EXPECT().RevokeCriteriaBatch(mock.Anything,
		mock.MatchedBy(func(batch []revocation.CriteriaRevocation) bool {
			return len(batch) == 1 && batch[0].Mode == revocation.ModeBeforeAction &&
				batch[0].Cutoff.Equal(cutoff) && batch[0].Reason == revocation.ReasonRoleAssignmentRemoved
		})).Return(nil)

	resp, err := newCriteriaRevocationExecutor(s.factory, revoker).Execute(&providers.NodeContext{
		Context:           context.Background(),
		SharedRuntimeData: map[string]string{common.RuntimeKeyRevocationPlan: encoded},
	})

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
}

// End to end across the two nodes: the mode configured on the pre-executor is the mode the revoker
// receives, with no request input involved.
func (s *RevocationWorkflowExecutorsTestSuite) TestConfiguredModeReachesRevoker() {
	s.users.EXPECT().ValidateDeleteUser(mock.Anything, "user-123").Return(nil)
	preResp, err := newPreDeleteExecutor(s.factory, s.users).Execute(&providers.NodeContext{
		Context:      context.Background(),
		ExecutorMode: string(revocation.ModeAll),
		UserInputs:   map[string]string{revocationInputSubject: "user-123"},
	})
	s.Require().NoError(err)
	s.Require().Equal(providers.ExecComplete, preResp.Status)

	revoker := revocationmock.NewCriteriaRevokerInterfaceMock(s.T())
	revoker.EXPECT().RevokeCriteriaBatch(mock.Anything,
		mock.MatchedBy(func(batch []revocation.CriteriaRevocation) bool {
			return len(batch) == 1 && batch[0].Mode == revocation.ModeAll
		})).Return(nil)

	revokeResp, err := newCriteriaRevocationExecutor(s.factory, revoker).Execute(&providers.NodeContext{
		Context:           context.Background(),
		SharedRuntimeData: preResp.SharedRuntimeData,
	})

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, revokeResp.Status)
}
