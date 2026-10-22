// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernanceprovider

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type DisabledIdentityGovernanceProviderTestSuite struct {
	suite.Suite
}

func TestDisabledIdentityGovernanceProviderTestSuite(t *testing.T) {
	suite.Run(t, new(DisabledIdentityGovernanceProviderTestSuite))
}

func (s *DisabledIdentityGovernanceProviderTestSuite) TestDisabledProviderRejectsLifecycleOperations() {
	ctx := context.Background()
	provider := newDisabledIdentityGovernanceProvider()
	for _, operation := range []func(context.Context, string) *tidcommon.ServiceError{
		provider.Unlock, provider.Unsuspend, provider.ValidateSuspend,
	} {
		err := operation(ctx, "e1")
		s.Require().NotNil(err)
		s.Equal(errGovernanceUnavailable.Code, err.Code)
	}

	err := provider.Suspend(ctx, "e1", "")
	s.Require().NotNil(err)
	s.Equal(errGovernanceUnavailable.Code, err.Code)
}

func (s *DisabledIdentityGovernanceProviderTestSuite) TestDisabledProviderAdmitsEveryCheck() {
	ctx := context.Background()
	provider := newDisabledIdentityGovernanceProvider()
	for _, plane := range []providers.AccessPlane{
		providers.AccessPlaneAuthentication,
		providers.AccessPlaneDispatch,
		providers.AccessPlaneRecovery,
		providers.AccessPlaneSession,
		providers.AccessPlaneIssuance,
		providers.AccessPlaneApplication,
	} {
		decision, err := provider.EvaluateAccess(ctx, "e1", plane, providers.AuthenticationMethodCredential)
		s.Nil(err)
		s.True(decision.Admitted, "plane %s must admit", plane)
	}

	decision, err := provider.AdmitSignIn(ctx, "e1")
	s.Nil(err)
	s.True(decision.Admitted)
}
