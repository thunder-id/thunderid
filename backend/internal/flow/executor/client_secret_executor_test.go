// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	appmodel "github.com/thunder-id/thunderid/internal/application/model"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/flow/sessionmock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/revocationmock"
)

type ClientSecretExecutorTestSuite struct {
	suite.Suite
	factory core.FlowFactoryInterface
	apps    *applicationAdminProviderMock
}

func TestClientSecretExecutorTestSuite(t *testing.T) {
	suite.Run(t, new(ClientSecretExecutorTestSuite))
}

func (s *ClientSecretExecutorTestSuite) SetupTest() {
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), &config.Config{}))
	s.T().Cleanup(config.ResetServerRuntime)
	s.factory, _ = core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	s.apps = newApplicationAdminProviderMock(s.T())
}

func (s *ClientSecretExecutorTestSuite) nodeContext(inputs, shared map[string]string) *providers.NodeContext {
	return &providers.NodeContext{
		Context:           context.Background(),
		UserInputs:        inputs,
		RuntimeData:       map[string]string{},
		SharedRuntimeData: shared,
	}
}

// modeContext is nodeContext for the validator, whose action the node's revocation mode selects. It is
// always the first node in its flow, so it carries no shared runtime data.
func (s *ClientSecretExecutorTestSuite) modeContext(
	mode revocation.Mode, inputs map[string]string) *providers.NodeContext {
	ctx := s.nodeContext(inputs, nil)
	ctx.ExecutorMode = string(mode)
	return ctx
}

// The executors are built the way the server builds them: constructed without the application service,
// then handed it in the second phase the registry performs at startup.
func (s *ClientSecretExecutorTestSuite) validator() *applicationActionValidator {
	ex := newApplicationActionValidator(s.factory)
	ex.setApplicationProvider(s.apps)
	return ex
}

func (s *ClientSecretExecutorTestSuite) secretExecutor() *clientSecretExecutor {
	ex := newClientSecretExecutor(s.factory)
	ex.setApplicationProvider(s.apps)
	return ex
}

// Regeneration is bounded rather than terminal, so the plan carries a cutoff and the new secret comes
// back on AdditionalData, the only executor output the engine serializes.
func (s *ClientSecretExecutorTestSuite) TestSecretRegenerationFlow() {
	s.apps.EXPECT().ValidateCredentialAction(mock.Anything, "app-1", appmodel.CredentialActionRegenerate).Return(
		&appmodel.ApplicationArtifactProfile{ClientKey: "client-1", MaxLifetimeSeconds: 86400}, nil)
	pre, err := s.validator().Execute(
		s.modeContext(revocation.ModeBeforeAction, map[string]string{revocationInputApplication: "app-1"}))
	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, pre.Status)

	revoker := revocationmock.NewCriteriaRevokerInterfaceMock(s.T())
	revoker.EXPECT().RevokeByCriteria(mock.Anything, mock.MatchedBy(
		func(value revocation.CriteriaRevocation) bool {
			return value.Mode == revocation.ModeBeforeAction &&
				value.Reason == revocation.ReasonApplicationSecretRegenerated &&
				!value.Cutoff.IsZero() &&
				value.TTL.Seconds() == 86400
		})).Return(nil)
	_, err = newCriteriaRevocationExecutor(s.factory, revoker).Execute(
		s.nodeContext(nil, pre.SharedRuntimeData))
	s.Require().NoError(err)

	s.apps.EXPECT().ApplyCredentialAction(
		mock.Anything, "app-1", appmodel.CredentialActionRegenerate).Return("new-secret", nil)
	resp, err := s.secretExecutor().Execute(
		s.nodeContext(nil, pre.SharedRuntimeData))
	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
	s.Equal("new-secret", resp.AdditionalData[common.DataClientSecret])
	s.Empty(resp.RuntimeData, "the secret must not travel on the persisted runtime data")
}

// A rotation must leave sessions alone even when a hand-built flow wires the session node into it: the
// artifacts issued under the old secret are denied, but the user's session is still theirs.
func (s *ClientSecretExecutorTestSuite) TestSecretRegeneration_DoesNotDetachSessions() {
	s.apps.EXPECT().ValidateCredentialAction(mock.Anything, "app-1", appmodel.CredentialActionRegenerate).Return(
		&appmodel.ApplicationArtifactProfile{ClientKey: "client-1"}, nil)
	pre, err := s.validator().Execute(
		s.modeContext(revocation.ModeBeforeAction, map[string]string{revocationInputApplication: "app-1"}))
	s.Require().NoError(err)

	sessions := sessionmock.NewServiceMock(s.T())
	resp, err := newSessionRevocationExecutor(s.factory, sessions).Execute(
		s.nodeContext(nil, pre.SharedRuntimeData))

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
	sessions.AssertNotCalled(s.T(), "DetachApplication", mock.Anything, mock.Anything)
	sessions.AssertNotCalled(s.T(), "TerminateBySubject", mock.Anything, mock.Anything)
}

// Pairing a preparatory node with the acting node of a different action is not caught by flow
// validation, so the acting node checks the plan's reason and refuses rather than mutating.
func (s *ClientSecretExecutorTestSuite) TestActingNodes_RejectAPlanForAnotherAction() {
	s.apps.EXPECT().ValidateDeleteApplication(mock.Anything, "app-1").Return(
		&appmodel.ApplicationArtifactProfile{ClientKey: "client-1"}, nil)
	pre, err := s.validator().Execute(
		s.modeContext(revocation.ModeAll, map[string]string{revocationInputApplication: "app-1"}))
	s.Require().NoError(err)

	_, err = s.secretExecutor().Execute(
		s.nodeContext(nil, pre.SharedRuntimeData))

	s.Require().Error(err)
	s.apps.AssertNotCalled(s.T(), "ApplyCredentialAction", mock.Anything, mock.Anything, mock.Anything)
}
