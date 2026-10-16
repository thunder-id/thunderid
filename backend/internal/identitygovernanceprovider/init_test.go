// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernanceprovider

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/identitygovernancemock"
)

type InitTestSuite struct {
	suite.Suite
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (s *InitTestSuite) TestNoServiceDisablesGovernance() {
	ctx := context.Background()
	provider := InitializeIdentityGovernanceProvider(nil)
	s.Require().NotNil(provider)
	for _, entityID := range []string{"", "e1"} {
		for _, plane := range []providers.AccessPlane{
			providers.AccessPlaneAuthentication,
			providers.AccessPlaneDispatch,
			providers.AccessPlaneRecovery,
			providers.AccessPlaneSession,
			providers.AccessPlaneIssuance,
			providers.AccessPlaneApplication,
		} {
			decision, err := provider.EvaluateAccess(ctx, entityID, plane, "credential")
			s.True(decision.Admitted, "plane %s must admit", plane)
			s.Nil(err)
		}
	}
}

func (s *InitTestSuite) TestBothProvidersSatisfyThePort() {
	var _ providers.IdentityGovernanceProvider = newDisabledIdentityGovernanceProvider()
	s.Require().Implements((*providers.IdentityGovernanceProvider)(nil),
		newDefaultIdentityGovernanceProvider(
			identitygovernancemock.NewIdentityGovernanceServiceInterfaceMock(s.T())))
}
