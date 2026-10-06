// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/identitygovernanceprovidermock"
)

type AccessGuardTestSuite struct {
	suite.Suite
}

func TestAccessGuardTestSuite(t *testing.T) {
	suite.Run(t, new(AccessGuardTestSuite))
}

// admitAllAccess is a governance provider that admits every check, for suites about something else.
func admitAllAccess(t interface {
	Cleanup(func())
	mock.TestingT
}) providers.IdentityGovernanceProvider {
	governance := identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(t)
	governance.EXPECT().EvaluateAccess(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AccessDecision{Admitted: true}, nil).Maybe()
	governance.EXPECT().AdmitSignIn(mock.Anything, mock.Anything).
		Return(providers.AccessDecision{Admitted: true}, nil).Maybe()
	governance.EXPECT().RecordLogin(mock.Anything, mock.Anything).Return(nil).Maybe()
	return governance
}

// expectAccess is a governance provider that expects one check of the entity at the plane and method,
// and answers it with the decision and fault.
func expectAccess(t interface {
	Cleanup(func())
	mock.TestingT
}, entityID string, plane providers.AccessPlane, method providers.AuthenticationMethod,
	decision providers.AccessDecision,
	fault *tidcommon.ServiceError) *identitygovernanceprovidermock.IdentityGovernanceProviderMock {
	governance := identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(t)
	governance.EXPECT().EvaluateAccess(mock.Anything, entityID, plane, method).Return(decision, fault).Once()
	return governance
}

// expectSignIn is expectAccess for a completing sign-in.
func expectSignIn(t interface {
	Cleanup(func())
	mock.TestingT
}, entityID string, decision providers.AccessDecision,
	fault *tidcommon.ServiceError) *identitygovernanceprovidermock.IdentityGovernanceProviderMock {
	governance := identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(t)
	governance.EXPECT().AdmitSignIn(mock.Anything, entityID).Return(decision, fault).Once()
	return governance
}

// decided requires a guard call to have reached a decision rather than a fault, and returns it.
func decided(t *testing.T) func(*tidcommon.ServiceError, error) *tidcommon.ServiceError {
	return func(svcErr *tidcommon.ServiceError, err error) *tidcommon.ServiceError {
		require.NoError(t, err)
		return svcErr
	}
}

// testAccessGuard builds a guard over the governance provider with the default logger.
func testAccessGuard(governance providers.IdentityGovernanceProvider) *accessGuard {
	return newAccessGuard(governance, log.GetLogger())
}

// A refusal that names no hold answers as a wrong secret does, so the two cannot be told apart.
func (s *AccessGuardTestSuite) TestAccessGuardRefusesWithTheIndistinguishableCode() {
	guard := testAccessGuard(expectAccess(s.T(), "held-1", providers.AccessPlaneSession, "",
		providers.AccessDecision{}, nil))

	svcErr := decided(s.T())(guard.refuseIfHeld(context.Background(), "held-1",
		providers.AccessPlaneSession, ""))
	s.Require().NotNil(svcErr)
	s.Equal(ErrInvalidCredentials.Code, svcErr.Code)
}

func (s *AccessGuardTestSuite) TestAccessGuardAdmits() {
	guard := testAccessGuard(admitAllAccess(s.T()))

	s.Nil(decided(s.T())(guard.refuseIfHeld(context.Background(), "u1",
		providers.AccessPlaneSession, "")))
}

func (s *AccessGuardTestSuite) TestAccessGuardReportsAFaultWhenTheAuthorityFails() {
	governance := identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(s.T())
	governance.EXPECT().EvaluateAccess(mock.Anything, "u1", providers.AccessPlaneAuthentication,
		providers.AuthenticationMethodCredential).
		Return(providers.AccessDecision{}, &tidcommon.InternalServerError).Once()
	governance.EXPECT().EvaluateAccess(mock.Anything, "u1", providers.AccessPlaneDispatch,
		providers.AuthenticationMethodOTP).
		Return(providers.AccessDecision{}, &tidcommon.InternalServerError).Once()
	governance.EXPECT().AdmitSignIn(mock.Anything, "u1").
		Return(providers.AccessDecision{}, &tidcommon.InternalServerError).Once()
	guard := testAccessGuard(governance)

	svcErr, err := guard.refuseIfHeld(context.Background(), "u1",
		providers.AccessPlaneAuthentication, providers.AuthenticationMethodCredential)
	s.Error(err)
	s.Nil(svcErr)

	svcErr, err = guard.refuseSignInIfHeld(context.Background(), "u1")
	s.Error(err)
	s.Nil(svcErr)

	withheld, stated, err := guard.heldAtDispatch(context.Background(), "u1",
		providers.AuthenticationMethodOTP, providers.FlowTypeAuthentication)
	s.Error(err)
	s.False(withheld)
	s.Nil(stated)
}

func (s *AccessGuardTestSuite) TestAccessGuardPassesThePlaneAndMethodThrough() {
	governance := identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(s.T())
	governance.EXPECT().EvaluateAccess(mock.Anything, "u1", providers.AccessPlaneDispatch,
		providers.AuthenticationMethodOTP).Return(providers.AccessDecision{Admitted: true}, nil).Once()

	s.Nil(decided(s.T())(testAccessGuard(governance).refuseIfHeld(context.Background(), "u1",
		providers.AccessPlaneDispatch, providers.AuthenticationMethodOTP)))
}

func (s *AccessGuardTestSuite) TestAccessGuardStatesTheHold() {
	for name, test := range map[string]struct {
		reason providers.AccessRefusalReason
		want   string
	}{
		"suspension": {providers.AccessRefusalReasonSuspended, ErrAccountSuspended.Code},
		"lock":       {providers.AccessRefusalReasonLocked, ErrAccountLocked.Code},
	} {
		s.Run(name, func() {
			governance := expectAccess(s.T(), "held-1", providers.AccessPlaneAuthentication, "",
				providers.AccessDecision{Reason: test.reason}, nil)

			svcErr := decided(s.T())(testAccessGuard(governance).refuseIfHeld(context.Background(), "held-1",
				providers.AccessPlaneAuthentication, ""))
			s.Require().NotNil(svcErr)
			s.Equal(test.want, svcErr.Code)
		})
	}
}

// The decision returned beside a fault is not used, even when it names a hold.
func (s *AccessGuardTestSuite) TestAccessGuardStatesNothingWhenTheAuthorityFails() {
	governance := expectAccess(s.T(), "u1", providers.AccessPlaneSession, "",
		providers.AccessDecision{Reason: providers.AccessRefusalReasonSuspended}, &tidcommon.ServiceError{Code: "boom"})

	svcErr, err := testAccessGuard(governance).refuseIfHeld(context.Background(), "u1",
		providers.AccessPlaneSession, "")
	s.Require().Error(err)
	s.Nil(svcErr)
}

func (s *AccessGuardTestSuite) TestAccessGuardChecksASignInThroughAdmitSignIn() {
	governance := expectSignIn(s.T(), "u1", providers.AccessDecision{Admitted: true}, nil)

	s.Nil(decided(s.T())(testAccessGuard(governance).refuseSignInIfHeld(context.Background(), "u1")))
	governance.AssertNotCalled(s.T(), "EvaluateAccess", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *AccessGuardTestSuite) TestAccessGuardRefusesAHeldSignIn() {
	for name, test := range map[string]struct {
		decision providers.AccessDecision
		want     string
	}{
		"no hold named": {providers.AccessDecision{}, ErrInvalidCredentials.Code},
		"suspension": {providers.AccessDecision{Reason: providers.AccessRefusalReasonSuspended},
			ErrAccountSuspended.Code},
		"lock": {providers.AccessDecision{Reason: providers.AccessRefusalReasonLocked}, ErrAccountLocked.Code},
	} {
		s.Run(name, func() {
			guard := testAccessGuard(expectSignIn(s.T(), "held-1", test.decision, nil))

			svcErr := decided(s.T())(guard.refuseSignInIfHeld(context.Background(), "held-1"))
			s.Require().NotNil(svcErr)
			s.Equal(test.want, svcErr.Code)
		})
	}
}

// Provisioning flows reach these nodes before the entity exists, and the embedded engine may run
// without a governance provider; neither is refused.
func (s *AccessGuardTestSuite) TestAccessGuardWithNoSubjectOrProviderChecksNothing() {
	governance := identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(s.T())
	guard := testAccessGuard(governance)

	s.Nil(decided(s.T())(guard.refuseIfHeld(context.Background(), "", providers.AccessPlaneSession, "")))
	s.Nil(decided(s.T())(guard.refuseSignInIfHeld(context.Background(), "")))
	guard.recordLogin(context.Background(), "")
	governance.AssertNotCalled(s.T(), "EvaluateAccess", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	governance.AssertNotCalled(s.T(), "AdmitSignIn", mock.Anything, mock.Anything)
	governance.AssertNotCalled(s.T(), "RecordLogin", mock.Anything, mock.Anything)

	noGovernance := testAccessGuard(nil)
	s.Nil(decided(s.T())(noGovernance.refuseIfHeld(context.Background(), "u1", providers.AccessPlaneSession, "")))
	s.Nil(decided(s.T())(noGovernance.refuseSignInIfHeld(context.Background(), "u1")))
	noGovernance.recordLogin(context.Background(), "u1")
}

func (s *AccessGuardTestSuite) TestAccessGuardRecordsTheLogin() {
	governance := identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(s.T())
	governance.EXPECT().RecordLogin(mock.Anything, "u1").Return(nil).Once()

	testAccessGuard(governance).recordLogin(context.Background(), "u1")
}

func (s *AccessGuardTestSuite) TestAccessGuardChecksNonAuthenticationDispatchAtTheRecoveryPlane() {
	for _, flowType := range []providers.FlowType{providers.FlowTypeRecovery, providers.FlowTypeRegistration} {
		governance := expectAccess(s.T(), "u1", providers.AccessPlaneRecovery, providers.AuthenticationMethodOTP,
			providers.AccessDecision{Admitted: true}, nil)

		withheld, stated, err := testAccessGuard(governance).heldAtDispatch(context.Background(), "u1",
			providers.AuthenticationMethodOTP, flowType)
		s.Require().NoError(err)
		s.False(withheld, string(flowType))
		s.Nil(stated)
	}
}

func (s *AccessGuardTestSuite) TestAccessGuardWithholdsADispatchWithTheStatedHold() {
	guard := testAccessGuard(expectAccess(s.T(), "u1", providers.AccessPlaneDispatch,
		providers.AuthenticationMethodOTP, providers.AccessDecision{Reason: providers.AccessRefusalReasonLocked}, nil))

	withheld, stated, err := guard.heldAtDispatch(context.Background(), "u1",
		providers.AuthenticationMethodOTP, providers.FlowTypeAuthentication)
	s.Require().NoError(err)
	s.True(withheld)
	s.Require().NotNil(stated)
	s.Equal(ErrAccountLocked.Code, stated.Code)
}

func (s *AccessGuardTestSuite) TestAccessGuardWithholdsADispatchWithoutAStatedHold() {
	guard := testAccessGuard(expectAccess(s.T(), "u1", providers.AccessPlaneDispatch,
		providers.AuthenticationMethodOTP, providers.AccessDecision{}, nil))

	withheld, stated, err := guard.heldAtDispatch(context.Background(), "u1",
		providers.AuthenticationMethodOTP, providers.FlowTypeAuthentication)
	s.Require().NoError(err)
	s.True(withheld)
	s.Nil(stated)
}
