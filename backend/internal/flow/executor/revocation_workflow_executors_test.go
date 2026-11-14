// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
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
	"github.com/thunder-id/thunderid/tests/mocks/actorprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/sessionmock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/revocationmock"
)

type RevocationWorkflowExecutorsTestSuite struct {
	suite.Suite
	factory core.FlowFactoryInterface
	actors  *actorprovidermock.ActorProviderMock
	planner *preRevocationExecutor
}

func TestRevocationWorkflowExecutorsTestSuite(t *testing.T) {
	suite.Run(t, new(RevocationWorkflowExecutorsTestSuite))
}

func (s *RevocationWorkflowExecutorsTestSuite) SetupTest() {
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), &config.Config{}))
	s.T().Cleanup(config.ResetServerRuntime)
	s.factory, _ = core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	s.actors = actorprovidermock.NewActorProviderMock(s.T())
	s.planner = newPreRevocationExecutor(s.factory, func(*providers.OAuthClient) time.Duration { return time.Hour },
		nil)
	s.planner.setActorProvider(s.actors)
}

// plan runs the planner in the given mode for the entity and returns what it shared.
func (s *RevocationWorkflowExecutorsTestSuite) plan(mode, entityID string) map[string]string {
	resp, err := s.planner.Execute(&providers.NodeContext{
		Context:      context.Background(),
		ExecutorMode: mode,
		UserInputs:   map[string]string{revocationInputEntity: entityID},
	})
	s.Require().NoError(err)
	s.Require().Equal(providers.ExecComplete, resp.Status)
	return resp.SharedRuntimeData
}

// The entity deletion flow revokes what was issued to the entity as a subject and ends its sessions.
func (s *RevocationWorkflowExecutorsTestSuite) TestEntityDeletionFlow() {
	shared := s.plan(ExecutorModeEntityRemoval, "user-123")

	revoker := revocationmock.NewCriteriaRevokerInterfaceMock(s.T())
	revoker.EXPECT().RevokeCriteriaBatch(mock.Anything,
		mock.MatchedBy(func(batch []revocation.CriteriaRevocation) bool {
			return len(batch) == 1 && batch[0].Criterion.Type == revocation.CriterionTypeSubject &&
				batch[0].Criterion.Value == "user-123" && batch[0].Mode == revocation.ModeBeforeAction &&
				batch[0].Reason == revocation.ReasonEntityRemoved
		})).Return(nil)
	criteriaResp, err := newCriteriaRevocationExecutor(s.factory, revoker).Execute(&providers.NodeContext{
		Context: context.Background(), SharedRuntimeData: shared,
	})
	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, criteriaResp.Status)

	sessions := sessionmock.NewServiceMock(s.T())
	sessions.EXPECT().TerminateBySubject(mock.Anything, "user-123").Return(nil)
	sessionResp, err := newSessionRevocationExecutor(s.factory, sessions).Execute(&providers.NodeContext{
		Context: context.Background(), SharedRuntimeData: shared,
	})
	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, sessionResp.Status)
}

// The inbound client deletion flow revokes what the client issued and detaches the entity from the
// sessions it took part in, leaving the sessions themselves to their subjects.
func (s *RevocationWorkflowExecutorsTestSuite) TestInboundClientDeletionFlow() {
	s.actors.EXPECT().GetOAuthClientByID(mock.Anything, "agent-1").
		Return(&providers.OAuthClient{ClientID: "client-1"}, nil)
	shared := s.plan(ExecutorModeClientRemoval, "agent-1")

	revoker := revocationmock.NewCriteriaRevokerInterfaceMock(s.T())
	revoker.EXPECT().RevokeCriteriaBatch(mock.Anything,
		mock.MatchedBy(func(batch []revocation.CriteriaRevocation) bool {
			return len(batch) == 1 && batch[0].Criterion.Type == revocation.CriterionTypeApplicationKey &&
				batch[0].Criterion.Value == "client-1" && batch[0].Reason == revocation.ReasonClientRemoved &&
				batch[0].TTL == time.Hour
		})).Return(nil)
	criteriaResp, err := newCriteriaRevocationExecutor(s.factory, revoker).Execute(&providers.NodeContext{
		Context: context.Background(), SharedRuntimeData: shared,
	})
	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, criteriaResp.Status)

	sessions := sessionmock.NewServiceMock(s.T())
	sessions.EXPECT().DetachApplication(mock.Anything, "agent-1").Return(nil)
	sessionResp, err := newSessionRevocationExecutor(s.factory, sessions).Execute(&providers.NodeContext{
		Context: context.Background(), SharedRuntimeData: shared,
	})
	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, sessionResp.Status)
}

func (s *RevocationWorkflowExecutorsTestSuite) TestCriteriaRevocationFailure() {
	encoded, err := encodeRevocationPlan(revocationPlan{
		Criteria: []revocation.Criterion{{Type: revocation.CriterionTypeSubject, Value: "user-123"}},
		Mode:     revocation.ModeAll, Reason: revocation.ReasonUserDeleted,
	})
	s.Require().NoError(err)
	revoker := revocationmock.NewCriteriaRevokerInterfaceMock(s.T())
	revoker.EXPECT().RevokeCriteriaBatch(mock.Anything, mock.Anything).Return(&tidcommon.InternalServerError)

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

// The breadth the planner records must survive the hop to the node that writes the revocation.
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
