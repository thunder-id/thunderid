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

func (s *DefaultIdentityGovernanceProviderTestSuite) TestEvaluateAccessForwardsTheDecisionOnARuntimeContext() {
	held := providers.AccessDecision{Admitted: false, Reason: providers.AccessRefusalReasonLocked}
	s.svc.EXPECT().EvaluateAccess(mock.MatchedBy(security.IsRuntimeContext), "e1",
		providers.AccessPlaneAuthentication, providers.AuthenticationMethodCredential).Return(held, nil).Once()

	decision, err := s.provider.EvaluateAccess(s.ctx, "e1", providers.AccessPlaneAuthentication, "credential")

	s.Nil(err)
	s.Equal(held, decision)
}

func (s *DefaultIdentityGovernanceProviderTestSuite) TestAdmitSignInForwardsTheDecisionOnARuntimeContext() {
	held := providers.AccessDecision{Admitted: false, Reason: providers.AccessRefusalReasonLocked}
	s.svc.EXPECT().AdmitSignIn(mock.MatchedBy(security.IsRuntimeContext), "e1").Return(held, nil).Once()

	decision, err := s.provider.AdmitSignIn(s.ctx, "e1")

	s.Nil(err)
	s.Equal(held, decision)
}

func (s *DefaultIdentityGovernanceProviderTestSuite) TestAccessCheckFailuresPassThrough() {
	s.svc.EXPECT().EvaluateAccess(mock.Anything, "e1", providers.AccessPlaneIssuance,
		providers.AuthenticationMethod("")).Return(providers.AccessDecision{}, &tidcommon.InternalServerError).Once()
	s.svc.EXPECT().AdmitSignIn(mock.Anything, "e1").
		Return(providers.AccessDecision{}, &tidcommon.InternalServerError).Once()

	decision, err := s.provider.EvaluateAccess(s.ctx, "e1", providers.AccessPlaneIssuance, "")
	s.False(decision.Admitted)
	s.Equal(&tidcommon.InternalServerError, err)

	decision, err = s.provider.AdmitSignIn(s.ctx, "e1")
	s.False(decision.Admitted)
	s.Equal(&tidcommon.InternalServerError, err)
}

func (s *DefaultIdentityGovernanceProviderTestSuite) TestRecordLoginForwardsOnARuntimeContext() {
	s.svc.EXPECT().RecordLogin(mock.MatchedBy(security.IsRuntimeContext), "e1").Return(nil).Once()
	s.Nil(s.provider.RecordLogin(s.ctx, "e1"))
}

func (s *DefaultIdentityGovernanceProviderTestSuite) TestRecordLoginPassesAFailureThrough() {
	s.svc.EXPECT().RecordLogin(mock.Anything, "e1").Return(&tidcommon.InternalServerError).Once()
	s.Equal(&tidcommon.InternalServerError, s.provider.RecordLogin(s.ctx, "e1"))
}

// Lifecycle operations are not elevated, since the service checks the caller's boundary.
func (s *DefaultIdentityGovernanceProviderTestSuite) TestLifecycleOperationsKeepTheCallersContext() {
	notRuntime := mock.MatchedBy(func(ctx context.Context) bool { return !security.IsRuntimeContext(ctx) })
	s.svc.EXPECT().Unlock(notRuntime, "e1").Return(nil).Once()
	s.svc.EXPECT().Suspend(notRuntime, "e1", "Credential exposure").Return(nil).Once()
	s.svc.EXPECT().ValidateSuspend(notRuntime, "e1", "note").Return(nil).Once()
	s.svc.EXPECT().Unsuspend(notRuntime, "e1").Return(nil).Once()

	s.Nil(s.provider.Unlock(s.ctx, "e1"))
	s.Nil(s.provider.Suspend(s.ctx, "e1", "Credential exposure"))
	s.Nil(s.provider.ValidateSuspend(s.ctx, "e1", "note"))
	s.Nil(s.provider.Unsuspend(s.ctx, "e1"))
}

func (s *DefaultIdentityGovernanceProviderTestSuite) TestLifecycleErrorsPassThrough() {
	for _, svcErr := range []*tidcommon.ServiceError{
		&identitygovernance.ErrorNotApplicable, &tidcommon.ErrorUnauthorized, &tidcommon.InternalServerError,
	} {
		svc := identitygovernancemock.NewIdentityGovernanceServiceInterfaceMock(s.T())
		svc.EXPECT().Suspend(mock.Anything, "e1", "").Return(svcErr).Once()
		s.Equal(svcErr, newDefaultIdentityGovernanceProvider(svc).Suspend(s.ctx, "e1", ""))
	}
}
