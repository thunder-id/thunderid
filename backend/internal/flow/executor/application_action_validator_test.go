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
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type ApplicationActionValidatorTestSuite struct {
	suite.Suite
	factory core.FlowFactoryInterface
	apps    *applicationAdminProviderMock
}

func TestApplicationActionValidatorTestSuite(t *testing.T) {
	suite.Run(t, new(ApplicationActionValidatorTestSuite))
}

func (s *ApplicationActionValidatorTestSuite) SetupTest() {
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), &config.Config{}))
	s.T().Cleanup(config.ResetServerRuntime)
	s.factory, _ = core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	s.apps = newApplicationAdminProviderMock(s.T())
}

func (s *ApplicationActionValidatorTestSuite) nodeContext(inputs, shared map[string]string) *providers.NodeContext {
	return &providers.NodeContext{
		Context:           context.Background(),
		UserInputs:        inputs,
		RuntimeData:       map[string]string{},
		SharedRuntimeData: shared,
	}
}

// modeContext is nodeContext for the validator, whose action the node's revocation mode selects. It is
// always the first node in its flow, so it carries no shared runtime data.
func (s *ApplicationActionValidatorTestSuite) modeContext(
	mode revocation.Mode, inputs map[string]string) *providers.NodeContext {
	ctx := s.nodeContext(inputs, nil)
	ctx.ExecutorMode = string(mode)
	return ctx
}

// The executors are built the way the server builds them: constructed without the application service,
// then handed it in the second phase the registry performs at startup.
func (s *ApplicationActionValidatorTestSuite) validator() *applicationActionValidator {
	ex := newApplicationActionValidator(s.factory)
	ex.setApplicationProvider(s.apps)
	return ex
}

// A refusal must land on the preparatory node, before anything is revoked or mutated, and must say
// which refusal it was: the operator's next step differs between a declarative application, one with
// no secret to rotate, and one that is simply gone.
func (s *ApplicationActionValidatorTestSuite) TestPreApplicationDelete_RefusalPublishesNoPlan() {
	s.apps.EXPECT().ValidateDeleteApplication(mock.Anything, "app-declarative").Return(
		nil, &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "APP-1030"})

	resp, err := s.validator().Execute(
		s.modeContext(revocation.ModeAll, map[string]string{revocationInputApplication: "app-declarative"}))

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, resp.Status)
	s.Equal("APP-1030", resp.Error.Code, "the validator's own refusal should reach the caller")
	s.Empty(resp.SharedRuntimeData[common.RuntimeKeyRevocationPlan])
}

// A client refusal carrying no code of its own still has to name something, so the executor's generic
// error stands in rather than reaching the caller as an empty envelope.
func (s *ApplicationActionValidatorTestSuite) TestPreApplicationDelete_UncodedRefusalFallsBack() {
	s.apps.EXPECT().ValidateDeleteApplication(mock.Anything, "app-uncoded").Return(
		nil, &tidcommon.ServiceError{Type: tidcommon.ClientErrorType})

	resp, err := s.validator().Execute(
		s.modeContext(revocation.ModeAll, map[string]string{revocationInputApplication: "app-uncoded"}))

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, resp.Status)
	s.Equal(ErrApplicationDeletionNotAllowed.Code, resp.Error.Code)
	s.Empty(resp.SharedRuntimeData[common.RuntimeKeyRevocationPlan])
}

// A server-side validation failure is an execution error, not a flow failure the caller can act on.
func (s *ApplicationActionValidatorTestSuite) TestPreSecretRegeneration_ServerErrorFailsExecution() {
	s.apps.EXPECT().ValidateCredentialAction(mock.Anything, "app-1", appmodel.CredentialActionRegenerate).Return(
		nil, &tidcommon.InternalServerError)

	_, err := s.validator().Execute(
		s.modeContext(revocation.ModeBeforeAction, map[string]string{revocationInputApplication: "app-1"}))

	s.Require().Error(err)
}

// The target is a declared required input, so omitting it asks for it rather than acting on nothing.
func (s *ApplicationActionValidatorTestSuite) TestPreApplicationDelete_MissingInputAsksForIt() {
	resp, err := s.validator().Execute(
		s.modeContext(revocation.ModeAll, map[string]string{}))

	s.Require().NoError(err)
	s.Equal(providers.ExecUserInputRequired, resp.Status)
}

// A node whose mode names no application action is a configuration fault, not a flow failure.
func (s *ApplicationActionValidatorTestSuite) TestValidator_UnsupportedModeFails() {
	_, err := s.validator().Execute(
		s.modeContext("revoke_sideways", map[string]string{revocationInputApplication: "app-1"}))

	s.Require().Error(err)
	s.apps.AssertNotCalled(s.T(), "ValidateDeleteApplication", mock.Anything, mock.Anything)
}
