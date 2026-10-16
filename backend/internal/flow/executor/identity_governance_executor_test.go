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
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/identitygovernanceprovidermock"
)

// IdentityGovernanceExecutorTestSuite covers the administrative lifecycle executor: one executor,
// three modes. The suspend plan is PreSuspendExecutor's; see pre_suspend_executor_test.go.
type IdentityGovernanceExecutorTestSuite struct {
	suite.Suite
	governance *identitygovernanceprovidermock.IdentityGovernanceProviderMock
	executor   *identityGovernanceExecutor
}

func TestIdentityGovernanceExecutorTestSuite(t *testing.T) {
	suite.Run(t, new(IdentityGovernanceExecutorTestSuite))
}

func (s *IdentityGovernanceExecutorTestSuite) SetupTest() {
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

// nodeContext builds the context an administration flow hands this node: a mode from the node
// definition and the subject as a declared input.
func nodeContext(mode, subject string) *providers.NodeContext {
	return &providers.NodeContext{
		Context:      context.Background(),
		ExecutionID:  "flow-1",
		FlowType:     providers.FlowTypeAdministration,
		ExecutorMode: mode,
		UserInputs:   map[string]string{revocationInputSubject: subject},
	}
}

// nodeContextWithOperatorNote is the same, plus the optional operator note a suspend node may collect.
func nodeContextWithOperatorNote(mode, subject, operatorNote string) *providers.NodeContext {
	ctx := nodeContext(mode, subject)
	ctx.UserInputs[governanceInputOperatorNote] = operatorNote
	return ctx
}

// suspendContext builds what a suspend node sees after PreSuspendExecutor: the trusted plan naming
// u1, and no subject input of its own.
func suspendContext(operatorNote string) *providers.NodeContext {
	plan, _ := encodeRevocationPlan(buildSuspensionPlan("u1", "", time.Now(), 0))
	ctx := nodeContext(governanceModeSuspend, "")
	delete(ctx.UserInputs, revocationInputSubject)
	if operatorNote != "" {
		ctx.UserInputs[governanceInputOperatorNote] = operatorNote
	}
	ctx.SharedRuntimeData = map[string]string{common.RuntimeKeyRevocationPlan: plan}
	return ctx
}

// Each mode calls its own method and nothing else. One executor for the three is only safe if the
// dispatch is exact.
func (s *IdentityGovernanceExecutorTestSuite) TestEachModeCallsItsOwnOperation() {
	for _, test := range []struct{ mode, method string }{
		{governanceModeUnlock, "Unlock"},
		{governanceModeUnsuspend, "Unsuspend"},
	} {
		s.Run(test.mode, func() {
			governance := identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(s.T())
			executor := s.newExecutor(governance)

			governance.On(test.method, mock.Anything, "u1").
				Return((*tidcommon.ServiceError)(nil)).Once()

			execResp, err := executor.Execute(nodeContext(test.mode, "u1"))
			s.Require().NoError(err)
			s.Equal(providers.ExecComplete, execResp.Status)
		})
	}
}

// The operator's note reaches the port verbatim, and an absent one crosses as empty. The note is
// optional, so a suspend node that collects none is complete.
func (s *IdentityGovernanceExecutorTestSuite) TestTheOperatorNoteReachesThePort() {
	s.governance.On("Suspend", mock.Anything, "u1", "Credential exposure").
		Return((*tidcommon.ServiceError)(nil)).Once()

	execResp, err := s.executor.Execute(suspendContext("Credential exposure"))
	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, execResp.Status)
}

func (s *IdentityGovernanceExecutorTestSuite) TestSuspendWithoutAnOperatorNoteIsComplete() {
	s.governance.On("Suspend", mock.Anything, "u1", "").
		Return((*tidcommon.ServiceError)(nil)).Once()

	execResp, err := s.executor.Execute(suspendContext(""))
	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, execResp.Status)
}

// The note input is consumed on every mode, not only on suspend. An input a node leaves unconsumed
// is re-prompted by the engine, so an unsuspend flow carrying a stray note would loop.
func (s *IdentityGovernanceExecutorTestSuite) TestTheOperatorNoteInputIsConsumedOnEveryMode() {
	s.governance.On("Unsuspend", mock.Anything, "u1").
		Return((*tidcommon.ServiceError)(nil)).Once()

	ctx := nodeContextWithOperatorNote(governanceModeUnsuspend, "u1", "ignored here")
	execResp, err := s.executor.Execute(ctx)
	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, execResp.Status)
	s.Contains(ctx.GetConsumedInputs(), governanceInputOperatorNote,
		"an input the node does not mark consumed is re-prompted")
}

// A suspend writes the hold on the subject the trusted plan names, never on a subject input: the
// hold must land on exactly the identity the containment nodes acted on.
func (s *IdentityGovernanceExecutorTestSuite) TestSuspendTakesItsTargetFromThePlan() {
	s.governance.On("Suspend", mock.Anything, "u1", "").
		Return((*tidcommon.ServiceError)(nil)).Once()

	ctx := suspendContext("")
	ctx.UserInputs[revocationInputSubject] = "someone-else"
	execResp, err := s.executor.Execute(ctx)

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, execResp.Status)
	s.governance.AssertNotCalled(s.T(), "Suspend", mock.Anything, "someone-else", mock.Anything)
}

// A suspend node with no PreSuspendExecutor ahead of it is a malformed flow, and it faults rather
// than writing a hold nothing contained.
func (s *IdentityGovernanceExecutorTestSuite) TestSuspendWithoutAPlanFaults() {
	_, err := s.executor.Execute(nodeContext(governanceModeSuspend, "u1"))

	s.Require().Error(err)
	s.governance.AssertNotCalled(s.T(), "Suspend", mock.Anything, mock.Anything, mock.Anything)
}

// A plan another action's preparatory node published, or one naming no target, is not a suspension
// plan, so the suspend faults rather than guessing a target.
func (s *IdentityGovernanceExecutorTestSuite) TestSuspendWithAForeignOrTargetlessPlanFaults() {
	for name, test := range map[string]struct {
		plan revocationPlan
		want string
	}{
		"foreign": {revocationPlan{
			Criteria: []revocation.Criterion{{Type: revocation.CriterionTypeApplicationKey, Value: "client-1"}},
			Reason:   revocation.ReasonApplicationDeleted,
			TargetID: "app-1",
		}, "not \"identity_suspended\""},
		"targetless": {revocationPlan{
			Criteria: []revocation.Criterion{{Type: revocation.CriterionTypeSubject, Value: "u1"}},
			Reason:   revocation.ReasonIdentitySuspended,
		}, "no target"},
	} {
		s.Run(name, func() {
			plan, err := encodeRevocationPlan(test.plan)
			s.Require().NoError(err)
			ctx := nodeContext(governanceModeSuspend, "u1")
			ctx.SharedRuntimeData = map[string]string{common.RuntimeKeyRevocationPlan: plan}

			_, err = s.executor.Execute(ctx)

			s.Require().ErrorContains(err, test.want)
			s.governance.AssertNotCalled(s.T(), "Suspend", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

// No mode publishes a plan. A flow for unlock or unsuspend carries no revocation nodes, and the
// ratified rule is that **unsuspend does not withdraw the revocation** a suspend recorded.
func (s *IdentityGovernanceExecutorTestSuite) TestUnlockAndUnsuspendPublishNoPlan() {
	for _, test := range []struct{ mode, method string }{
		{governanceModeUnlock, "Unlock"},
		{governanceModeUnsuspend, "Unsuspend"},
	} {
		s.Run(test.mode, func() {
			governance := identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(s.T())
			executor := s.newExecutor(governance)

			governance.On(test.method, mock.Anything, "u1").
				Return((*tidcommon.ServiceError)(nil)).Once()

			execResp, err := executor.Execute(nodeContext(test.mode, "u1"))
			s.Require().NoError(err)
			s.Empty(execResp.SharedRuntimeData,
				"%s must not revoke anything that was already issued", test.mode)
		})
	}
}

// An omitted mode refuses, and the executor declares no default mode, so a flow omitting it cannot
// be saved.
func (s *IdentityGovernanceExecutorTestSuite) TestAnOmittedModeRefuses() {
	_, err := s.executor.Execute(nodeContext("", "u1"))
	s.Require().Error(err)
	s.governance.AssertNotCalled(s.T(), "Suspend", mock.Anything, mock.Anything, mock.Anything)
}

// The subject is published under the shared key, so a following node, such as a notification, can
// resolve who was governed.
func (s *IdentityGovernanceExecutorTestSuite) TestTheGovernedSubjectIsPublishedForLaterNodes() {
	s.governance.On("Suspend", mock.Anything, "u1", "").
		Return((*tidcommon.ServiceError)(nil)).Once()

	execResp, err := s.executor.Execute(suspendContext(""))
	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, execResp.Status)
	s.Equal("u1", execResp.RuntimeData[userAttributeUserID])
}

// The other half of the same guarantee: with no declared default, flow validation rejects a node
// that omits the mode, so the runtime refusal above is a backstop rather than the enforcement.
func (s *IdentityGovernanceExecutorTestSuite) TestTheExecutorDeclaresNoDefaultMode() {
	meta := s.executor.GetMeta()
	s.Require().NotNil(meta)
	s.Empty(meta.DefaultMode)
	s.ElementsMatch([]string{
		governanceModeUnlock, governanceModeSuspend, governanceModeUnsuspend,
	}, meta.SupportedModes)
}

// An unsupported mode refuses rather than defaulting. This backstops flows persisted before the
// mode was validated.
func (s *IdentityGovernanceExecutorTestSuite) TestAnUnsupportedModeRefuses() {
	_, err := s.executor.Execute(nodeContext("delete", "u1"))
	s.Require().Error(err)
	s.governance.AssertNotCalled(s.T(), "Suspend", mock.Anything, mock.Anything, mock.Anything)
}

// No subject is a flow-definition error, not a client error: the flow is supposed to have resolved
// one before this node runs.
func (s *IdentityGovernanceExecutorTestSuite) TestAMissingSubjectIsAnError() {
	_, err := s.executor.Execute(nodeContext(governanceModeUnsuspend, ""))
	s.Require().Error(err)
	s.governance.AssertNotCalled(s.T(), "Unsuspend", mock.Anything, mock.Anything)
}

// A refused write fails the node, so the flow stops before it notifies anyone.
func (s *IdentityGovernanceExecutorTestSuite) TestARefusedSuspendFailsTheNode() {
	s.governance.On("Suspend", mock.Anything, "u1", "").
		Return(&tidcommon.ErrorUnauthorized).Once()

	execResp, err := s.executor.Execute(suspendContext(""))
	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, execResp.Status)
	s.Equal(ErrIdentityGovernanceFailed.Code, execResp.Error.Code)
	s.Empty(execResp.SharedRuntimeData)
}

// A server error is a fault, not a refusal.
func (s *IdentityGovernanceExecutorTestSuite) TestAFailedSuspendIsAFault() {
	s.governance.On("Suspend", mock.Anything, "u1", "").
		Return(&tidcommon.InternalServerError).Once()

	execResp, err := s.executor.Execute(suspendContext(""))
	s.Require().Error(err)
	s.Nil(execResp)
}

// An unwired authority is a flow-engine wiring fault, and it must not read as a completed operation.
func (s *IdentityGovernanceExecutorTestSuite) TestNoAuthorityIsAnError() {
	executor := s.newExecutor(nil)

	_, err := executor.Execute(suspendContext(""))
	s.Require().Error(err)
}

// --- The self-service release ---
// Unlock reached from a recovery flow, which changes the subject source, privilege and modes.

// recoveryContext builds the context a recovery flow hands an Unlock Identity node. provenSubject is
// what an earlier executor recorded, and subjectInput is what a flow author may have left on the node.
func recoveryContext(provenSubject, subjectInput string) *providers.NodeContext {
	ctx := &providers.NodeContext{
		Context:      context.Background(),
		ExecutionID:  "flow-recovery-1",
		FlowType:     providers.FlowTypeRecovery,
		ExecutorMode: governanceModeUnlock,
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

// A recovery flow releases the identity an earlier node proved control of.
func (s *IdentityGovernanceExecutorTestSuite) TestARecoveryFlowUnlocksTheProvenSubject() {
	s.governance.On("Unlock", mock.Anything, "user-proven").Return(nil).Once()

	resp, err := s.executor.Execute(recoveryContext("user-proven", ""))

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
}

// A recovery flow ignores a supplied subject input: the caller is unauthenticated, so trusting it
// would let anyone release any account they can name.
func (s *IdentityGovernanceExecutorTestSuite) TestARecoveryFlowIgnoresASuppliedSubject() {
	s.governance.On("Unlock", mock.Anything, "user-proven").Return(nil).Once()

	resp, err := s.executor.Execute(recoveryContext("user-proven", "someone-elses-account"))

	s.Require().NoError(err)
	s.Equal(providers.ExecComplete, resp.Status)
	s.governance.AssertNotCalled(s.T(), "Unlock", mock.Anything, "someone-elses-account")
}

// No proof, no release. This is the criterion that keeps an Unlock node placed before the proving
// step, after identification but before the credential write, from being a way in.
func (s *IdentityGovernanceExecutorTestSuite) TestARecoveryFlowWithoutProofReleasesNothing() {
	resp, err := s.executor.Execute(recoveryContext("", "user-1"))

	s.Require().NoError(err)
	s.Equal(providers.ExecFailure, resp.Status)
	s.Equal(ErrIdentityGovernanceFailed.Code, resp.Error.Code)
	s.governance.AssertNotCalled(s.T(), "Unlock", mock.Anything, mock.Anything)
}

// The self-service release carries runtime privilege, since there is no operator to authorize it;
// the proof consumed earlier is what earns it.
func (s *IdentityGovernanceExecutorTestSuite) TestTheSelfServiceReleaseCarriesRuntimePrivilege() {
	var privileged bool
	s.governance.On("Unlock", mock.Anything, "user-proven").
		Run(func(args mock.Arguments) {
			callCtx, _ := args.Get(0).(context.Context)
			privileged = security.IsRuntimeContext(callCtx)
		}).Return(nil).Once()

	_, err := s.executor.Execute(recoveryContext("user-proven", ""))

	s.Require().NoError(err)
	s.True(privileged, "a self-service release must reach the authority with runtime privilege")
}

// An administration flow does **not**, so the boundary check runs against the operator.
func (s *IdentityGovernanceExecutorTestSuite) TestAnAdministrativeOperationKeepsTheCallersContext() {
	var privileged bool
	s.governance.On("Unlock", mock.Anything, "user-1").
		Run(func(args mock.Arguments) {
			callCtx, _ := args.Get(0).(context.Context)
			privileged = security.IsRuntimeContext(callCtx)
		}).Return(nil).Once()

	_, err := s.executor.Execute(nodeContext(governanceModeUnlock, "user-1"))

	s.Require().NoError(err)
	s.False(privileged, "an operator's lifecycle call must not be elevated past their own authority")
}

// Only unlock is reachable from a recovery flow; suspend from an unauthenticated flow would be a
// denial-of-service primitive.
func (s *IdentityGovernanceExecutorTestSuite) TestSuspendAndUnsuspendAreAdministrationOnly() {
	for _, mode := range []string{governanceModeSuspend, governanceModeUnsuspend} {
		ctx := recoveryContext("user-proven", "")
		ctx.ExecutorMode = mode

		_, err := s.executor.Execute(ctx)

		s.Error(err, "%s must not be reachable from a recovery flow", mode)
	}
	s.governance.AssertNotCalled(s.T(), "Suspend", mock.Anything, mock.Anything, mock.Anything)
	s.governance.AssertNotCalled(s.T(), "Unsuspend", mock.Anything, mock.Anything)
}
