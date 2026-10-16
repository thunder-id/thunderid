// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/identitygovernanceprovidermock"
)

// admitAllAccess is the authority for the executor suites that are about something else. Every check
// admits, so the guard is transparent.
func admitAllAccess(t interface {
	Cleanup(func())
	mock.TestingT
}) providers.IdentityGovernanceProvider {
	governance := identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(t)
	governance.On("EvaluateAccess", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AccessDecision{Admitted: true}, (*tidcommon.ServiceError)(nil)).Maybe()
	governance.On("AdmitSignIn", mock.Anything, mock.Anything).
		Return(providers.AccessDecision{Admitted: true}, (*tidcommon.ServiceError)(nil)).Maybe()
	governance.On("RecordLogin", mock.Anything, mock.Anything).
		Return((*tidcommon.ServiceError)(nil)).Maybe()
	return governance
}

// refuseAllAccess is the opposite fixture: a held identity at every plane.
func refuseAllAccess(t interface {
	Cleanup(func())
	mock.TestingT
}) *identitygovernanceprovidermock.IdentityGovernanceProviderMock {
	governance := identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(t)
	governance.On("EvaluateAccess", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AccessDecision{Admitted: false}, (*tidcommon.ServiceError)(nil)).Maybe()
	governance.On("AdmitSignIn", mock.Anything, mock.Anything).
		Return(providers.AccessDecision{Admitted: false}, (*tidcommon.ServiceError)(nil)).Maybe()
	return governance
}

// decideAccess answers every check with the given decision, or with the fault when it is non-nil.
func decideAccess(t interface {
	Cleanup(func())
	mock.TestingT
}, decision providers.AccessDecision, fault *tidcommon.ServiceError) providers.IdentityGovernanceProvider {
	governance := identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(t)
	governance.On("EvaluateAccess", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(decision, fault).Maybe()
	governance.On("AdmitSignIn", mock.Anything, mock.Anything).Return(decision, fault).Maybe()
	governance.On("RecordLogin", mock.Anything, mock.Anything).
		Return((*tidcommon.ServiceError)(nil)).Maybe()
	return governance
}

// decided requires a guard call to have reached a decision rather than a fault, and returns it.
func decided(t *testing.T) func(*tidcommon.ServiceError, error) *tidcommon.ServiceError {
	return func(svcErr *tidcommon.ServiceError, err error) *tidcommon.ServiceError {
		require.NoError(t, err)
		return svcErr
	}
}

func testAccessGuard(governance providers.IdentityGovernanceProvider) *accessGuard {
	return newAccessGuard(governance, log.GetLogger())
}

type AccessGuardTestSuite struct {
	suite.Suite
}

func TestAccessGuardTestSuite(t *testing.T) {
	suite.Run(t, new(AccessGuardTestSuite))
}

// A held identity produces the invalid-credentials code, and it is the same code a wrong secret
// produces. That is decided centrally rather than per executor: a held identity that answered
// differently would be an oracle, and one code in one place cannot drift into eight.
func (s *AccessGuardTestSuite) TestAccessGuardRefusesWithTheIndistinguishableCode() {
	guard := testAccessGuard(refuseAllAccess(s.T()))

	svcErr := decided(s.T())(guard.refuseIfHeld(context.Background(), "held-1",
		providers.AccessPlaneSession, ""))
	s.Require().NotNil(svcErr)
	s.Equal(ErrInvalidCredentials.Code, svcErr.Code)
}

// An admitted identity produces no refusal.
func (s *AccessGuardTestSuite) TestAccessGuardAdmits() {
	guard := testAccessGuard(admitAllAccess(s.T()))

	s.Nil(decided(s.T())(guard.refuseIfHeld(context.Background(), "u1",
		providers.AccessPlaneSession, "")))
}

// An unreadable authority is a fault, not a hold: the caller answers as it does when its own entity
// read fails, so an outage is not reported as invalid credentials.
func (s *AccessGuardTestSuite) TestAccessGuardReportsAFaultWhenTheAuthorityFails() {
	governance := identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(s.T())
	governance.On("EvaluateAccess", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AccessDecision{}, &tidcommon.InternalServerError).Twice()
	governance.On("AdmitSignIn", mock.Anything, mock.Anything).
		Return(providers.AccessDecision{}, &tidcommon.InternalServerError).Once()
	guard := testAccessGuard(governance)

	svcErr, err := guard.refuseIfHeld(context.Background(), "u1",
		providers.AccessPlaneAuthentication, providers.AccessScopeCredential)
	s.Error(err)
	s.Nil(svcErr)

	svcErr, err = guard.refuseSignInIfHeld(context.Background(), "u1")
	s.Error(err)
	s.Nil(svcErr)

	withheld, stated, err := guard.heldAtDispatch(context.Background(), "u1",
		providers.AccessScopeOTP, providers.FlowTypeAuthentication)
	s.Error(err)
	s.False(withheld)
	s.Nil(stated)
}

// No subject means nothing to check. The provisioning flows reach these nodes with an identity that
// has no entity yet, and refusing them would break the flows whose job is to create the account.
func (s *AccessGuardTestSuite) TestAccessGuardWithNoSubjectAdmits() {
	governance := identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(s.T())

	s.Nil(decided(s.T())(testAccessGuard(governance).refuseIfHeld(context.Background(), "",
		providers.AccessPlaneSession, "")))
	governance.AssertNotCalled(s.T(), "EvaluateAccess",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// The plane and scope the caller names are passed through untouched. The table that says what a
// plane withholds lives in the authority, so a gate must not paraphrase it.
func (s *AccessGuardTestSuite) TestAccessGuardPassesThePlaneAndScopeThrough() {
	governance := identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(s.T())
	governance.On("EvaluateAccess", mock.Anything, "u1", providers.AccessPlaneDispatch,
		providers.AccessScopeOTP).
		Return(providers.AccessDecision{Admitted: true}, (*tidcommon.ServiceError)(nil)).Once()

	s.Nil(decided(s.T())(testAccessGuard(governance).refuseIfHeld(context.Background(), "u1",
		providers.AccessPlaneDispatch, providers.AccessScopeOTP)))
}

// A stated hold reaches the refusal itself, which is the whole of what the setting buys.
func (s *AccessGuardTestSuite) TestAccessGuardStatesASuspension() {
	governance := identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(s.T())
	governance.On("EvaluateAccess", mock.Anything, "held-1", providers.AccessPlaneSession, providers.AccessScope("")).
		Return(providers.AccessDecision{
			Admitted: false, Reason: providers.AccessRefusalReasonSuspended,
		}, (*tidcommon.ServiceError)(nil)).Once()

	svcErr := decided(s.T())(testAccessGuard(governance).refuseIfHeld(context.Background(), "held-1",
		providers.AccessPlaneSession, ""))
	s.Require().NotNil(svcErr)
	s.Equal(ErrAccountSuspended.Code, svcErr.Code)
}

// A lock states the lock, not the suspension: the two are different sentences and only one of them
// has a self-service way out.
func (s *AccessGuardTestSuite) TestAccessGuardStatesALock() {
	governance := identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(s.T())
	governance.On("EvaluateAccess", mock.Anything, "held-1", providers.AccessPlaneAuthentication,
		providers.AccessScope("")).
		Return(providers.AccessDecision{
			Admitted: false, Reason: providers.AccessRefusalReasonLocked,
		}, (*tidcommon.ServiceError)(nil)).Once()

	svcErr := decided(s.T())(testAccessGuard(governance).refuseIfHeld(context.Background(), "held-1",
		providers.AccessPlaneAuthentication, ""))
	s.Require().NotNil(svcErr)
	s.Equal(ErrAccountLocked.Code, svcErr.Code)
}

// Without the policy's permission the refusal is the wrong-secret one, whatever the hold was.
func (s *AccessGuardTestSuite) TestAccessGuardStatesNothingByDefault() {
	svcErr := decided(s.T())(testAccessGuard(refuseAllAccess(s.T())).refuseIfHeld(context.Background(),
		"held-1", providers.AccessPlaneSession, ""))
	s.Require().NotNil(svcErr)
	s.Equal(ErrInvalidCredentials.Code, svcErr.Code)
}

// An authority that could not be read states nothing, even beside a decision that names a hold: it
// is a fault, and a decision it never completed carries no permission to speak.
func (s *AccessGuardTestSuite) TestAccessGuardStatesNothingWhenTheAuthorityFails() {
	governance := identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(s.T())
	governance.On("EvaluateAccess", mock.Anything, "u1", providers.AccessPlaneSession, providers.AccessScope("")).
		Return(providers.AccessDecision{
			Admitted: false, Reason: providers.AccessRefusalReasonSuspended,
		}, &tidcommon.ServiceError{Code: "boom"}).Once()

	svcErr, err := testAccessGuard(governance).refuseIfHeld(context.Background(), "u1",
		providers.AccessPlaneSession, "")
	s.Require().Error(err)
	s.Nil(svcErr)
}

// A completing sign-in is checked through AdmitSignIn, which is what clears the entity-wide record.
func (s *AccessGuardTestSuite) TestAccessGuardChecksAGrant() {
	governance := identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(s.T())
	governance.On("AdmitSignIn", mock.Anything, "u1").
		Return(providers.AccessDecision{Admitted: true}, (*tidcommon.ServiceError)(nil)).Once()

	s.Nil(decided(s.T())(testAccessGuard(governance).refuseSignInIfHeld(context.Background(), "u1")))
	governance.AssertNotCalled(s.T(), "EvaluateAccess", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// A held grant is refused like any other held identity.
func (s *AccessGuardTestSuite) TestAccessGuardRefusesAHeldGrant() {
	svcErr := decided(s.T())(testAccessGuard(refuseAllAccess(s.T())).refuseSignInIfHeld(context.Background(), "held-1"))
	s.Require().NotNil(svcErr)
	s.Equal(ErrInvalidCredentials.Code, svcErr.Code)
}

// A sign-in or login stamp with no subject, or no authority, reaches nothing: there is no identity
// to admit or stamp.
func (s *AccessGuardTestSuite) TestAccessGuardWithNoSubjectSkipsSignInAndLoginStamp() {
	governance := identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(s.T())
	guard := testAccessGuard(governance)

	s.Nil(decided(s.T())(guard.refuseSignInIfHeld(context.Background(), "")))
	guard.recordLogin(context.Background(), "")
	s.Nil(decided(s.T())(testAccessGuard(nil).refuseSignInIfHeld(context.Background(), "u1")))
	testAccessGuard(nil).recordLogin(context.Background(), "u1")

	governance.AssertNotCalled(s.T(), "AdmitSignIn", mock.Anything, mock.Anything)
	governance.AssertNotCalled(s.T(), "RecordLogin", mock.Anything, mock.Anything)
}

// A login stamp reaches the authority for the subject it names.
func (s *AccessGuardTestSuite) TestAccessGuardRecordsTheLogin() {
	governance := identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(s.T())
	governance.On("RecordLogin", mock.Anything, "u1").Return((*tidcommon.ServiceError)(nil)).Once()

	testAccessGuard(governance).recordLogin(context.Background(), "u1")
}

// A challenge outside an authentication flow is checked at the recovery plane, where a lock
// withholds nothing, so a locked user can still receive the code that recovers the account.
func (s *AccessGuardTestSuite) TestAccessGuardChecksNonAuthenticationDispatchAtTheRecoveryPlane() {
	for _, flowType := range []providers.FlowType{providers.FlowTypeRecovery, providers.FlowTypeRegistration} {
		governance := identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(s.T())
		governance.On("EvaluateAccess", mock.Anything, "u1", providers.AccessPlaneRecovery,
			providers.AccessScopeOTP).
			Return(providers.AccessDecision{Admitted: true}, (*tidcommon.ServiceError)(nil)).Once()

		withheld, stated, err := testAccessGuard(governance).heldAtDispatch(context.Background(), "u1",
			providers.AccessScopeOTP, flowType)
		s.Require().NoError(err)
		s.False(withheld, string(flowType))
		s.Nil(stated)
	}
}

// Only the manager's two hold codes classify as a hold; anything else, or nothing, states nothing.
func (s *AccessGuardTestSuite) TestStatedHoldClassifiesOnlyHoldCodes() {
	s.Equal(holdNone, statedHold(nil))
	s.Equal(holdNone, statedHold(&tidcommon.ErrorUnauthorized))
	s.Equal(holdLocked, statedHold(&authnprovidermgr.ErrorAccountLocked))
	s.Equal(holdSuspended, statedHold(&authnprovidermgr.ErrorAccountSuspended))
}
