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

type ApplicationDeleteExecutorTestSuite struct {
	suite.Suite
	factory core.FlowFactoryInterface
	apps    *applicationAdminProviderMock
}

func TestApplicationDeleteExecutorTestSuite(t *testing.T) {
	suite.Run(t, new(ApplicationDeleteExecutorTestSuite))
}

func (s *ApplicationDeleteExecutorTestSuite) SetupTest() {
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), &config.Config{}))
	s.T().Cleanup(config.ResetServerRuntime)
	s.factory, _ = core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	s.apps = newApplicationAdminProviderMock(s.T())
}

func (s *ApplicationDeleteExecutorTestSuite) nodeContext(inputs, shared map[string]string) *providers.NodeContext {
	return &providers.NodeContext{
		Context:           context.Background(),
		UserInputs:        inputs,
		RuntimeData:       map[string]string{},
		SharedRuntimeData: shared,
	}
}

// modeContext is nodeContext for the validator, whose action the node's revocation mode selects. It is
// always the first node in its flow, so it carries no shared runtime data.
func (s *ApplicationDeleteExecutorTestSuite) modeContext(
	mode revocation.Mode, inputs map[string]string) *providers.NodeContext {
	ctx := s.nodeContext(inputs, nil)
	ctx.ExecutorMode = string(mode)
	return ctx
}

// The executors are built the way the server builds them: constructed without the application service,
// then handed it in the second phase the registry performs at startup.
func (s *ApplicationDeleteExecutorTestSuite) validator() *applicationActionValidator {
	ex := newApplicationActionValidator(s.factory)
	ex.setApplicationProvider(s.apps)
	return ex
}

func (s *ApplicationDeleteExecutorTestSuite) deleteExecutor() *applicationDeleteExecutor {
	ex := newApplicationDeleteExecutor(s.factory)
	ex.setApplicationProvider(s.apps)
	return ex
}

// The whole deletion chain in one execution: validation publishes the plan, the criteria write takes its
// client key and TTL from it, the session node detaches the application, and the record goes last.
func (s *ApplicationDeleteExecutorTestSuite) TestApplicationDeletionFlow() {
	s.apps.EXPECT().ValidateDeleteApplication(mock.Anything, "app-1").Return(
		&appmodel.ApplicationArtifactProfile{ClientKey: "client-1", MaxLifetimeSeconds: 2592000}, nil)
	pre, err := s.validator().Execute(
		s.modeContext(revocation.ModeAll, map[string]string{revocationInputApplication: "app-1"}))
	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, pre.Status)
	s.NotEmpty(pre.SharedRuntimeData[common.RuntimeKeyRevocationPlan])

	revoker := revocationmock.NewCriteriaRevokerInterfaceMock(s.T())
	revoker.EXPECT().RevokeByCriteria(mock.Anything, mock.MatchedBy(
		func(value revocation.CriteriaRevocation) bool {
			return value.Criterion.Type == revocation.CriterionTypeApplicationKey &&
				value.Criterion.Value == "client-1" &&
				value.Mode == revocation.ModeAll &&
				value.Reason == revocation.ReasonApplicationDeleted &&
				value.Cutoff.IsZero() &&
				value.TTL.Seconds() == 2592000
		})).Return(nil)
	criteria, err := newCriteriaRevocationExecutor(s.factory, revoker).Execute(
		s.nodeContext(nil, pre.SharedRuntimeData))
	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, criteria.Status)

	sessions := sessionmock.NewServiceMock(s.T())
	sessions.EXPECT().DetachApplication(mock.Anything, "app-1").Return(nil)
	sessionResp, err := newSessionRevocationExecutor(s.factory, sessions).Execute(
		s.nodeContext(nil, pre.SharedRuntimeData))
	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, sessionResp.Status)

	s.apps.EXPECT().DeleteApplication(mock.Anything, "app-1").Return(nil)
	deleteResp, err := s.deleteExecutor().Execute(
		s.nodeContext(nil, pre.SharedRuntimeData))
	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, deleteResp.Status)
}

// An application with no OAuth component issues no artifacts. The plan says so explicitly, the revoke
// node writes nothing, and the deletion still proceeds.
func (s *ApplicationDeleteExecutorTestSuite) TestDeletion_WithoutOAuthComponentSkipsRevocation() {
	s.apps.EXPECT().ValidateDeleteApplication(mock.Anything, "app-embedded").Return(
		&appmodel.ApplicationArtifactProfile{}, nil)
	pre, err := s.validator().Execute(
		s.modeContext(revocation.ModeAll, map[string]string{revocationInputApplication: "app-embedded"}))
	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, pre.Status)

	revoker := revocationmock.NewCriteriaRevokerInterfaceMock(s.T())
	criteria, err := newCriteriaRevocationExecutor(s.factory, revoker).Execute(
		s.nodeContext(nil, pre.SharedRuntimeData))
	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, criteria.Status)
	revoker.AssertNotCalled(s.T(), "RevokeByCriteria", mock.Anything, mock.Anything)

	// Session participation is recorded per application whether or not it ever held a token, so the
	// detachment still runs.
	sessions := sessionmock.NewServiceMock(s.T())
	sessions.EXPECT().DetachApplication(mock.Anything, "app-embedded").Return(nil)
	_, err = newSessionRevocationExecutor(s.factory, sessions).Execute(
		s.nodeContext(nil, pre.SharedRuntimeData))
	s.Require().NoError(err)

	s.apps.EXPECT().DeleteApplication(mock.Anything, "app-embedded").Return(nil)
	deleteResp, err := s.deleteExecutor().Execute(
		s.nodeContext(nil, pre.SharedRuntimeData))
	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, deleteResp.Status)
}

// An acting node without a plan must fail rather than treat the absence as nothing to do.
func (s *ApplicationDeleteExecutorTestSuite) TestApplicationDelete_MissingPlanFails() {
	_, err := s.deleteExecutor().Execute(
		s.nodeContext(nil, map[string]string{}))

	s.Require().Error(err)
	s.apps.AssertNotCalled(s.T(), "DeleteApplication", mock.Anything, mock.Anything)
}

// An executor the second-phase injection never reached is a configuration fault, not a flow failure.
func (s *ApplicationDeleteExecutorTestSuite) TestActingNode_UnresolvedProviderFails() {
	_, err := newApplicationDeleteExecutor(s.factory).Execute(s.nodeContext(nil, map[string]string{}))

	s.Require().Error(err)
}
