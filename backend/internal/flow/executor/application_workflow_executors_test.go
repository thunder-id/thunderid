// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
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
	"github.com/thunder-id/thunderid/tests/mocks/applicationadminprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/sessionmock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/revocationmock"
)

type ApplicationWorkflowExecutorsTestSuite struct {
	suite.Suite
	factory core.FlowFactoryInterface
	apps    *applicationadminprovidermock.ApplicationAdminProviderMock
}

func TestApplicationWorkflowExecutorsTestSuite(t *testing.T) {
	suite.Run(t, new(ApplicationWorkflowExecutorsTestSuite))
}

func (s *ApplicationWorkflowExecutorsTestSuite) SetupTest() {
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), &config.Config{}))
	s.T().Cleanup(config.ResetServerRuntime)
	s.factory, _ = core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	s.apps = applicationadminprovidermock.NewApplicationAdminProviderMock(s.T())
}

// provider returns the resolver the executors hold, standing in for the deferred injection the
// composition root performs.
// The executors below are built the way the server builds them: constructed without the application
// service, then handed it in the second phase the registry performs at startup.
func (s *ApplicationWorkflowExecutorsTestSuite) validateDeletion() *validateApplicationActionExecutor {
	ex := newValidateApplicationDeletionExecutor(s.factory)
	ex.setApplicationProvider(s.apps)
	return ex
}

func (s *ApplicationWorkflowExecutorsTestSuite) validateRegeneration() *validateApplicationActionExecutor {
	ex := newValidateSecretRegenerationExecutor(s.factory)
	ex.setApplicationProvider(s.apps)
	return ex
}

func (s *ApplicationWorkflowExecutorsTestSuite) deleteExecutor() *applicationDeleteExecutor {
	ex := newApplicationDeleteExecutor(s.factory)
	ex.setApplicationProvider(s.apps)
	return ex
}

func (s *ApplicationWorkflowExecutorsTestSuite) secretExecutor() *clientSecretExecutor {
	ex := newClientSecretExecutor(s.factory)
	ex.setApplicationProvider(s.apps)
	return ex
}

func (s *ApplicationWorkflowExecutorsTestSuite) nodeContext(
	inputs, shared map[string]string) *providers.NodeContext {
	return &providers.NodeContext{
		Context:           context.Background(),
		UserInputs:        inputs,
		RuntimeData:       map[string]string{},
		SharedRuntimeData: shared,
	}
}

// The whole deletion chain in one execution: validation publishes the plan, the criteria write takes its
// client key and TTL from it, the session node detaches the application, and the record goes last.
func (s *ApplicationWorkflowExecutorsTestSuite) TestApplicationDeletionFlow() {
	s.apps.EXPECT().ValidateDeleteApplication(mock.Anything, "app-1").Return(
		&providers.ApplicationArtifactProfile{ClientKey: "client-1", MaxLifetimeSeconds: 2592000}, nil)
	pre, err := s.validateDeletion().Execute(
		s.nodeContext(map[string]string{revocationInputApplication: "app-1"}, nil))
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
	sessions.EXPECT().RemoveApplication(mock.Anything, "app-1").Return(nil)
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

// Regeneration is bounded rather than terminal, so the plan carries a cutoff and the new secret comes
// back on AdditionalData, the only executor output the engine serializes.
func (s *ApplicationWorkflowExecutorsTestSuite) TestSecretRegenerationFlow() {
	s.apps.EXPECT().ValidateCredentialAction(mock.Anything, "app-1", providers.CredentialActionRegenerate).Return(
		&providers.ApplicationArtifactProfile{ClientKey: "client-1", MaxLifetimeSeconds: 86400}, nil)
	pre, err := s.validateRegeneration().Execute(
		s.nodeContext(map[string]string{revocationInputApplication: "app-1"}, nil))
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
		mock.Anything, "app-1", providers.CredentialActionRegenerate).Return("new-secret", nil)
	resp, err := s.secretExecutor().Execute(
		s.nodeContext(nil, pre.SharedRuntimeData))
	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
	s.Equal("new-secret", resp.AdditionalData[dataKeyClientSecret])
	s.Empty(resp.RuntimeData, "the secret must not travel on the persisted runtime data")
}

// An application with no OAuth component issues no artifacts. The plan says so explicitly, the revoke
// node writes nothing, and the deletion still proceeds.
func (s *ApplicationWorkflowExecutorsTestSuite) TestDeletion_WithoutOAuthComponentSkipsRevocation() {
	s.apps.EXPECT().ValidateDeleteApplication(mock.Anything, "app-embedded").Return(
		&providers.ApplicationArtifactProfile{}, nil)
	pre, err := s.validateDeletion().Execute(
		s.nodeContext(map[string]string{revocationInputApplication: "app-embedded"}, nil))
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
	sessions.EXPECT().RemoveApplication(mock.Anything, "app-embedded").Return(nil)
	_, err = newSessionRevocationExecutor(s.factory, sessions).Execute(
		s.nodeContext(nil, pre.SharedRuntimeData))
	s.Require().NoError(err)

	s.apps.EXPECT().DeleteApplication(mock.Anything, "app-embedded").Return(nil)
	deleteResp, err := s.deleteExecutor().Execute(
		s.nodeContext(nil, pre.SharedRuntimeData))
	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, deleteResp.Status)
}

// A rotation must leave sessions alone even when a hand-built flow wires the session node into it: the
// artifacts issued under the old secret are denied, but the user's session is still theirs.
func (s *ApplicationWorkflowExecutorsTestSuite) TestSecretRegeneration_DoesNotDetachSessions() {
	s.apps.EXPECT().ValidateCredentialAction(mock.Anything, "app-1", providers.CredentialActionRegenerate).Return(
		&providers.ApplicationArtifactProfile{ClientKey: "client-1"}, nil)
	pre, err := s.validateRegeneration().Execute(
		s.nodeContext(map[string]string{revocationInputApplication: "app-1"}, nil))
	s.Require().NoError(err)

	sessions := sessionmock.NewServiceMock(s.T())
	resp, err := newSessionRevocationExecutor(s.factory, sessions).Execute(
		s.nodeContext(nil, pre.SharedRuntimeData))

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
	sessions.AssertNotCalled(s.T(), "RemoveApplication", mock.Anything, mock.Anything)
	sessions.AssertNotCalled(s.T(), "TerminateBySubject", mock.Anything, mock.Anything)
}

// A refusal must land on the preparatory node, before anything is revoked or mutated, and must say
// which refusal it was: the operator's next step differs between a declarative application, one with
// no secret to rotate, and one that is simply gone.
func (s *ApplicationWorkflowExecutorsTestSuite) TestPreApplicationDelete_RefusalPublishesNoPlan() {
	s.apps.EXPECT().ValidateDeleteApplication(mock.Anything, "app-declarative").Return(
		nil, &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "APP-1030"})

	resp, err := s.validateDeletion().Execute(
		s.nodeContext(map[string]string{revocationInputApplication: "app-declarative"}, nil))

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, resp.Status)
	s.Equal("APP-1030", resp.Error.Code, "the validator's own refusal should reach the caller")
	s.Empty(resp.SharedRuntimeData[common.RuntimeKeyRevocationPlan])
}

// A client refusal carrying no code of its own still has to name something, so the executor's generic
// error stands in rather than reaching the caller as an empty envelope.
func (s *ApplicationWorkflowExecutorsTestSuite) TestPreApplicationDelete_UncodedRefusalFallsBack() {
	s.apps.EXPECT().ValidateDeleteApplication(mock.Anything, "app-uncoded").Return(
		nil, &tidcommon.ServiceError{Type: tidcommon.ClientErrorType})

	resp, err := s.validateDeletion().Execute(
		s.nodeContext(map[string]string{revocationInputApplication: "app-uncoded"}, nil))

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, resp.Status)
	s.Equal(ErrApplicationDeletionNotAllowed.Code, resp.Error.Code)
	s.Empty(resp.SharedRuntimeData[common.RuntimeKeyRevocationPlan])
}

// A server-side validation failure is an execution error, not a flow failure the caller can act on.
func (s *ApplicationWorkflowExecutorsTestSuite) TestPreSecretRegeneration_ServerErrorFailsExecution() {
	s.apps.EXPECT().ValidateCredentialAction(mock.Anything, "app-1", providers.CredentialActionRegenerate).Return(
		nil, &tidcommon.InternalServerError)

	_, err := s.validateRegeneration().Execute(
		s.nodeContext(map[string]string{revocationInputApplication: "app-1"}, nil))

	s.Require().Error(err)
}

// The target is a declared required input, so omitting it asks for it rather than acting on nothing.
func (s *ApplicationWorkflowExecutorsTestSuite) TestPreApplicationDelete_MissingInputAsksForIt() {
	resp, err := s.validateDeletion().Execute(
		s.nodeContext(map[string]string{}, nil))

	s.Require().NoError(err)
	s.Equal(providers.ExecUserInputRequired, resp.Status)
}

// Pairing a preparatory node with the acting node of a different action is not caught by flow
// validation, so the acting node checks the plan's reason and refuses rather than mutating.
func (s *ApplicationWorkflowExecutorsTestSuite) TestActingNodes_RejectAPlanForAnotherAction() {
	s.apps.EXPECT().ValidateDeleteApplication(mock.Anything, "app-1").Return(
		&providers.ApplicationArtifactProfile{ClientKey: "client-1"}, nil)
	pre, err := s.validateDeletion().Execute(
		s.nodeContext(map[string]string{revocationInputApplication: "app-1"}, nil))
	s.Require().NoError(err)

	_, err = s.secretExecutor().Execute(
		s.nodeContext(nil, pre.SharedRuntimeData))

	s.Require().Error(err)
	s.apps.AssertNotCalled(s.T(), "ApplyCredentialAction", mock.Anything, mock.Anything, mock.Anything)
}

// An acting node without a plan must fail rather than treat the absence as nothing to do.
func (s *ApplicationWorkflowExecutorsTestSuite) TestApplicationDelete_MissingPlanFails() {
	_, err := s.deleteExecutor().Execute(
		s.nodeContext(nil, map[string]string{}))

	s.Require().Error(err)
	s.apps.AssertNotCalled(s.T(), "DeleteApplication", mock.Anything, mock.Anything)
}

// An executor the second-phase injection never reached is a configuration fault, not a flow failure.
func (s *ApplicationWorkflowExecutorsTestSuite) TestActingNode_UnresolvedProviderFails() {
	_, err := newApplicationDeleteExecutor(s.factory).Execute(s.nodeContext(nil, map[string]string{}))

	s.Require().Error(err)
}
