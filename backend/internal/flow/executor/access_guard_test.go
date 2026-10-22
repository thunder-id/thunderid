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
	governance.EXPECT().AdmitSignIn(mock.Anything, mock.Anything).
		Return(providers.AccessDecision{Admitted: true}, nil).Maybe()
	return governance
}

// expectSignIn is a governance provider that expects one completing sign-in check of the entity, and
// answers it with the decision and fault.
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

func (s *AccessGuardTestSuite) TestAccessGuardChecksASignInThroughAdmitSignIn() {
	governance := expectSignIn(s.T(), "u1", providers.AccessDecision{Admitted: true}, nil)

	guard := newAccessGuard(governance)
	s.Nil(decided(s.T())(guard.refuseSignInIfHeld(context.Background(), "u1", log.GetLogger())))
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
			guard := newAccessGuard(expectSignIn(s.T(), "held-1", test.decision, nil))

			svcErr := decided(s.T())(guard.refuseSignInIfHeld(context.Background(), "held-1", log.GetLogger()))
			s.Require().NotNil(svcErr)
			s.Equal(test.want, svcErr.Code)
		})
	}
}

// Provisioning flows reach these nodes before the entity exists, and the embedded engine may run
// without a governance provider; neither is refused.
func (s *AccessGuardTestSuite) TestAccessGuardWithNoSubjectOrProviderChecksNothing() {
	governance := identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(s.T())
	guard := newAccessGuard(governance)

	s.Nil(decided(s.T())(guard.refuseSignInIfHeld(context.Background(), "", log.GetLogger())))
	governance.AssertNotCalled(s.T(), "AdmitSignIn", mock.Anything, mock.Anything)

	noGovernance := newAccessGuard(nil)
	s.Nil(decided(s.T())(noGovernance.refuseSignInIfHeld(context.Background(), "u1", log.GetLogger())))
}
