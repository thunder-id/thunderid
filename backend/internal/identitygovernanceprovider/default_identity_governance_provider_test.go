// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernanceprovider

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/identitygovernance"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/identitygovernancemock"
)

type DefaultIdentityGovernanceProviderTestSuite struct {
	suite.Suite
	ctx      context.Context
	svc      *identitygovernancemock.IdentityGovernanceServiceInterfaceMock
	provider providers.IdentityGovernanceProvider
}

func TestDefaultIdentityGovernanceProviderTestSuite(t *testing.T) {
	suite.Run(t, new(DefaultIdentityGovernanceProviderTestSuite))
}

func (s *DefaultIdentityGovernanceProviderTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.svc = identitygovernancemock.NewIdentityGovernanceServiceInterfaceMock(s.T())
	s.provider = newDefaultIdentityGovernanceProvider(s.svc)
}

// The provider forwards the answer unchanged, admitted or not.
func (s *DefaultIdentityGovernanceProviderTestSuite) TestEvaluateAccessForwardsTheDecision() {
	held := providers.AccessDecision{Admitted: false, Reason: providers.AccessRefusalReasonLocked}
	s.svc.On("EvaluateAccess", mock.Anything, "e1", providers.AccessPlaneAuthentication,
		providers.AccessScopeCredential).Return(held, nil).Once()

	decision, err := s.provider.EvaluateAccess(s.ctx, "e1", providers.AccessPlaneAuthentication, "credential")

	s.Nil(err)
	s.Equal(held, decision)
}

// A service failure becomes a refusal, never an admission.
func (s *DefaultIdentityGovernanceProviderTestSuite) TestEvaluateAccessRefusesWhenTheServiceFails() {
	s.svc.On("EvaluateAccess", mock.Anything, "e1", providers.AccessPlaneIssuance, providers.AccessScope("")).
		Return(providers.AccessDecision{Admitted: true}, &tidcommon.InternalServerError).Once()

	decision, err := s.provider.EvaluateAccess(s.ctx, "e1", providers.AccessPlaneIssuance, "")

	s.False(decision.Admitted)
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

// Access checks run on a runtime context: the subject is not the caller.
func (s *DefaultIdentityGovernanceProviderTestSuite) TestEvaluateAccessRunsOnARuntimeContext() {
	s.svc.On("EvaluateAccess", mock.MatchedBy(security.IsRuntimeContext), "e1",
		providers.AccessPlaneAuthentication, providers.AccessScopeCredential).
		Return(providers.AccessDecision{Admitted: true}, nil).Once()

	_, err := s.provider.EvaluateAccess(s.ctx, "e1", providers.AccessPlaneAuthentication, "credential")

	s.Nil(err)
}

// AdmitSignIn forwards on a runtime context and refuses when the service fails.
func (s *DefaultIdentityGovernanceProviderTestSuite) TestAdmitSignInForwardsAndFailsClosed() {
	s.svc.On("AdmitSignIn", mock.MatchedBy(security.IsRuntimeContext), "e1").
		Return(providers.AccessDecision{Admitted: true}, nil).Once()
	decision, err := s.provider.AdmitSignIn(s.ctx, "e1")
	s.Nil(err)
	s.True(decision.Admitted)

	s.svc.On("AdmitSignIn", mock.Anything, "e2").
		Return(providers.AccessDecision{}, &tidcommon.InternalServerError).Once()
	decision, err = s.provider.AdmitSignIn(s.ctx, "e2")
	s.False(decision.Admitted)
	s.Require().NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

// RecordLogin forwards on a runtime context.
func (s *DefaultIdentityGovernanceProviderTestSuite) TestRecordLoginForwardsOnARuntimeContext() {
	s.svc.On("RecordLogin", mock.MatchedBy(security.IsRuntimeContext), "e1").Return(nil).Once()

	s.Nil(s.provider.RecordLogin(s.ctx, "e1"))
}

// A failed stamp is never returned, so no caller can refuse a grant over it.
func (s *DefaultIdentityGovernanceProviderTestSuite) TestRecordLoginSwallowsAFailure() {
	s.svc.On("RecordLogin", mock.Anything, "e1").Return(&tidcommon.InternalServerError).Once()

	s.Nil(s.provider.RecordLogin(s.ctx, "e1"))
}

// Lifecycle operations keep the caller's context, since the service checks the caller's boundary.
func (s *DefaultIdentityGovernanceProviderTestSuite) TestLifecycleOperationsKeepTheCallersContext() {
	notRuntime := mock.MatchedBy(func(ctx context.Context) bool { return !security.IsRuntimeContext(ctx) })
	s.svc.On("Unlock", notRuntime, "e1").Return(nil).Once()
	s.svc.On("Suspend", notRuntime, "e1", "Credential exposure").Return(nil).Once()
	s.svc.On("ValidateSuspend", notRuntime, "e1").Return(nil).Once()
	s.svc.On("Unsuspend", notRuntime, "e1").Return(nil).Once()

	s.Nil(s.provider.Unlock(s.ctx, "e1"))
	s.Nil(s.provider.Suspend(s.ctx, "e1", "Credential exposure"))
	s.Nil(s.provider.ValidateSuspend(s.ctx, "e1"))
	s.Nil(s.provider.Unsuspend(s.ctx, "e1"))
}

// A lifecycle error reaches the caller unchanged, so its type still tells a refusal from a fault.
func (s *DefaultIdentityGovernanceProviderTestSuite) TestLifecycleErrorsPassThrough() {
	for _, svcErr := range []*tidcommon.ServiceError{
		&identitygovernance.ErrorNotApplicable, &tidcommon.ErrorUnauthorized, &tidcommon.InternalServerError,
	} {
		svc := identitygovernancemock.NewIdentityGovernanceServiceInterfaceMock(s.T())
		svc.On("Suspend", mock.Anything, "e1", "").Return(svcErr).Once()

		s.Equal(svcErr, newDefaultIdentityGovernanceProvider(svc).Suspend(s.ctx, "e1", ""))
	}
}

// Both providers satisfy the port.
func (s *DefaultIdentityGovernanceProviderTestSuite) TestBothProvidersSatisfyThePort() {
	var _ providers.IdentityGovernanceProvider = newDisabledIdentityGovernanceProvider()
	var _ providers.IdentityGovernanceProvider = s.provider
}
