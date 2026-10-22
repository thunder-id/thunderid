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
	"github.com/thunder-id/thunderid/internal/identitygovernanceprovider"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/identitygovernanceprovidermock"
)

type IdentityGovernanceExecutorTestSuite struct {
	suite.Suite
	governance *identitygovernanceprovidermock.IdentityGovernanceProviderMock
	executor   *identityGovernanceExecutor
}

func TestIdentityGovernanceExecutorTestSuite(t *testing.T) {
	suite.Run(t, new(IdentityGovernanceExecutorTestSuite))
}

func (s *IdentityGovernanceExecutorTestSuite) SetupTest() {
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), &config.Config{}))
	s.governance = identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(s.T())
	s.executor = s.newExecutor(s.governance)
}

// newExecutor builds the executor over a freshly initialized flow factory.
func (s *IdentityGovernanceExecutorTestSuite) newExecutor(
	governance providers.IdentityGovernanceProvider) *identityGovernanceExecutor {
	flowFactory, _ := core.Initialize(
		cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	return newIdentityGovernanceExecutor(flowFactory, governance)
}

func (s *IdentityGovernanceExecutorTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

// assertNothingApplied asserts that no lifecycle method reached the provider.
func (s *IdentityGovernanceExecutorTestSuite) assertNothingApplied() {
	s.governance.AssertNotCalled(s.T(), "Unlock", mock.Anything, mock.Anything)
}

// governanceContext builds an administration node context with the mode and the subject input.
func governanceContext(mode, subject string) *providers.NodeContext {
	return &providers.NodeContext{
		Context:      context.Background(),
		ExecutionID:  "flow-1",
		FlowType:     providers.FlowTypeAdministration,
		ExecutorMode: mode,
		UserInputs:   map[string]string{revocationInputSubject: subject},
	}
}

// adminOperation is a lifecycle mode run on u1, with the provider call it expects.
type adminOperation struct {
	mode   string
	expect func(governance *identitygovernanceprovidermock.IdentityGovernanceProviderMock,
		svcErr *tidcommon.ServiceError)
	ctx func() *providers.NodeContext
}

// adminOperations returns one adminOperation per mode.
func adminOperations() []adminOperation {
	return []adminOperation{
		{
			mode: ExecutorModeUnlock,
			expect: func(governance *identitygovernanceprovidermock.IdentityGovernanceProviderMock,
				svcErr *tidcommon.ServiceError) {
				governance.EXPECT().Unlock(mock.Anything, "u1").Return(svcErr).Once()
			},
			ctx: func() *providers.NodeContext { return governanceContext(ExecutorModeUnlock, "u1") },
		},
	}
}

func (s *IdentityGovernanceExecutorTestSuite) TestEachModeCallsItsOwnOperation() {
	for _, op := range adminOperations() {
		s.Run(op.mode, func() {
			governance := identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(s.T())
			executor := s.newExecutor(governance)
			op.expect(governance, nil)

			execResp, err := executor.Execute(op.ctx())
			s.Require().NoError(err)
			s.Equal(providers.ExecComplete, execResp.Status)
			s.Equal("u1", execResp.RuntimeData[userAttributeUserID])
		})
	}
}

func (s *IdentityGovernanceExecutorTestSuite) TestAnOmittedModeRefuses() {
	_, err := s.executor.Execute(governanceContext("", "u1"))
	s.Require().Error(err)
	s.assertNothingApplied()
}

func (s *IdentityGovernanceExecutorTestSuite) TestTheExecutorDeclaresNoDefaultMode() {
	meta := s.executor.GetMeta()
	s.Require().NotNil(meta)
	s.Empty(meta.DefaultMode)
	s.ElementsMatch([]string{ExecutorModeUnlock}, meta.SupportedModes)
}

func (s *IdentityGovernanceExecutorTestSuite) TestAnUnsupportedModeRefuses() {
	_, err := s.executor.Execute(governanceContext("delete", "u1"))
	s.Require().Error(err)
	s.assertNothingApplied()
}

func (s *IdentityGovernanceExecutorTestSuite) TestAnAbsentSubjectIsPromptedFor() {
	ctx := governanceContext(ExecutorModeUnlock, "")
	delete(ctx.UserInputs, revocationInputSubject)

	execResp, err := s.executor.Execute(ctx)

	s.Require().NoError(err)
	s.Equal(providers.ExecUserInputRequired, execResp.Status)
	s.Require().Len(execResp.Inputs, 1)
	s.Equal(revocationInputSubject, execResp.Inputs[0].Identifier)
	s.assertNothingApplied()
}

// A flow definition can declare an optional node input; leaving it out does not prompt for it.
func (s *IdentityGovernanceExecutorTestSuite) TestAnAbsentOptionalNodeInputIsNotPromptedFor() {
	s.governance.EXPECT().Unlock(mock.Anything, "u1").Return(nil).Once()
	ctx := governanceContext(ExecutorModeUnlock, "u1")
	ctx.NodeInputs = []providers.Input{
		{Identifier: revocationInputSubject, Type: providers.InputTypeText, Required: true},
		{Identifier: "reason", Type: providers.InputTypeText},
	}

	execResp, err := s.executor.Execute(ctx)

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, execResp.Status)
	s.Empty(execResp.Inputs)
}

func (s *IdentityGovernanceExecutorTestSuite) TestAnEmptySubjectIsAnError() {
	_, err := s.executor.Execute(governanceContext(ExecutorModeUnlock, ""))
	s.Require().Error(err)
	s.assertNothingApplied()
}

// A refused write publishes no subject, so a following notification node finds no one.
func (s *IdentityGovernanceExecutorTestSuite) TestARefusedOperationFailsTheNode() {
	for _, op := range adminOperations() {
		s.Run(op.mode, func() {
			governance := identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(s.T())
			executor := s.newExecutor(governance)
			op.expect(governance, &tidcommon.ErrorUnauthorized)

			execResp, err := executor.Execute(op.ctx())
			s.Require().NoError(err)
			s.Equal(providers.ExecFailure, execResp.Status)
			s.Equal(ErrIdentityGovernanceFailed.Code, execResp.Error.Code)
			s.Empty(execResp.RuntimeData)
			s.Empty(execResp.SharedRuntimeData)
		})
	}
}

func (s *IdentityGovernanceExecutorTestSuite) TestAFailedOperationIsAFault() {
	for _, op := range adminOperations() {
		s.Run(op.mode, func() {
			governance := identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(s.T())
			executor := s.newExecutor(governance)
			op.expect(governance, &tidcommon.InternalServerError)

			execResp, err := executor.Execute(op.ctx())
			s.Require().Error(err)
			s.Nil(execResp)
		})
	}
}

func (s *IdentityGovernanceExecutorTestSuite) TestNoAuthorityIsAnErrorInAnAdministrationFlow() {
	executor := s.newExecutor(nil)
	for _, op := range adminOperations() {
		_, err := executor.Execute(op.ctx())
		s.Error(err, op.mode)
	}
}

// recoveryUnlockContext builds a recovery unlock node context with the proven subject and a subject
// input, each omitted when empty.
func recoveryUnlockContext(provenSubject, subjectInput string) *providers.NodeContext {
	ctx := &providers.NodeContext{
		Context:      context.Background(),
		ExecutionID:  "flow-recovery-1",
		FlowType:     providers.FlowTypeRecovery,
		ExecutorMode: ExecutorModeUnlock,
		UserInputs:   map[string]string{},
	}
	if subjectInput != "" {
		ctx.UserInputs[revocationInputSubject] = subjectInput
	}
	if provenSubject != "" {
		ctx.SharedRuntimeData = map[string]string{common.RuntimeKeyProvenSubject: provenSubject}
	}
	return ctx
}

func (s *IdentityGovernanceExecutorTestSuite) TestARecoveryFlowUnlocksTheProvenSubject() {
	s.governance.EXPECT().Unlock(mock.Anything, "user-proven").Return(nil).Once()

	resp, err := s.executor.Execute(recoveryUnlockContext("user-proven", ""))

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
	s.Equal("user-proven", resp.RuntimeData[userAttributeUserID])
}

func (s *IdentityGovernanceExecutorTestSuite) TestARefusedRecoveryUnlockCompletesAndReleasesNothing() {
	refusing := identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(s.T())
	refusing.EXPECT().Unlock(mock.Anything, "user-proven").Return(&tidcommon.ErrorUnauthorized).Once()
	for name, governance := range map[string]providers.IdentityGovernanceProvider{
		"unauthorized": refusing,
		"governance disabled": identitygovernanceprovider.Initialize(nil,
			config.IdentityGovernanceProviderConfig{Type: "disabled"}),
	} {
		s.Run(name, func() {
			resp, err := s.newExecutor(governance).Execute(recoveryUnlockContext("user-proven", ""))

			s.Require().NoError(err)
			s.Equal(providers.ExecComplete, resp.Status)
			s.Nil(resp.Error)
			s.Empty(resp.RuntimeData)
		})
	}
}

func (s *IdentityGovernanceExecutorTestSuite) TestARecoveryUnlockWithoutGovernanceCompletes() {
	ctx := recoveryUnlockContext("user-proven", "someone-elses-account")

	resp, err := s.newExecutor(nil).Execute(ctx)

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
	s.Nil(resp.Error)
	s.Empty(resp.RuntimeData)
	s.Contains(ctx.GetConsumedInputs(), revocationInputSubject)
}

func (s *IdentityGovernanceExecutorTestSuite) TestAFailedRecoveryUnlockIsAFault() {
	s.governance.EXPECT().Unlock(mock.Anything, "user-proven").Return(&tidcommon.InternalServerError).Once()

	resp, err := s.executor.Execute(recoveryUnlockContext("user-proven", ""))

	s.Require().Error(err)
	s.Nil(resp)
}

// The caller of a recovery flow is unauthenticated, so a subject input must not pick the account.
func (s *IdentityGovernanceExecutorTestSuite) TestARecoveryFlowIgnoresASuppliedSubject() {
	s.governance.EXPECT().Unlock(mock.Anything, "user-proven").Return(nil).Once()

	resp, err := s.executor.Execute(recoveryUnlockContext("user-proven", "someone-elses-account"))

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
	s.governance.AssertNotCalled(s.T(), "Unlock", mock.Anything, "someone-elses-account")
}

// An unlock node placed before the proving step releases nothing.
func (s *IdentityGovernanceExecutorTestSuite) TestARecoveryFlowWithoutProofReleasesNothing() {
	resp, err := s.executor.Execute(recoveryUnlockContext("", "user-1"))

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, resp.Status)
	s.Equal(ErrIdentityGovernanceFailed.Code, resp.Error.Code)
	s.governance.AssertNotCalled(s.T(), "Unlock", mock.Anything, mock.Anything)
}

func (s *IdentityGovernanceExecutorTestSuite) TestTheSelfServiceReleaseCarriesRuntimePrivilege() {
	var privileged bool
	s.governance.EXPECT().Unlock(mock.Anything, "user-proven").
		Run(func(callCtx context.Context, _ string) {
			privileged = security.IsRuntimeContext(callCtx)
		}).Return(nil).Once()

	_, err := s.executor.Execute(recoveryUnlockContext("user-proven", ""))

	s.Require().NoError(err)
	s.True(privileged, "a self-service release must reach the authority with runtime privilege")
}

func (s *IdentityGovernanceExecutorTestSuite) TestAnAdministrativeOperationKeepsTheCallersContext() {
	var privileged bool
	s.governance.EXPECT().Unlock(mock.Anything, "user-1").
		Run(func(callCtx context.Context, _ string) {
			privileged = security.IsRuntimeContext(callCtx)
		}).Return(nil).Once()

	_, err := s.executor.Execute(governanceContext(ExecutorModeUnlock, "user-1"))

	s.Require().NoError(err)
	s.False(privileged, "an operator's lifecycle call must not be elevated past their own authority")
}

// Without governance the mode check is all that stops a recovery flow.
func (s *IdentityGovernanceExecutorTestSuite) TestOnlyUnlockRunsInARecoveryFlow() {
	for name, executor := range map[string]*identityGovernanceExecutor{
		"with governance":    s.executor,
		"without governance": s.newExecutor(nil),
	} {
		ctx := recoveryUnlockContext("user-proven", "")
		ctx.ExecutorMode = "delete"

		_, err := executor.Execute(ctx)

		s.Error(err, name)
	}
	s.assertNothingApplied()
}
