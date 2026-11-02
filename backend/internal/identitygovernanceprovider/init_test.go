// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernanceprovider

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/tests/mocks/identitygovernancemock"
)

type InitTestSuite struct {
	suite.Suite
	governance *identitygovernancemock.IdentityGovernanceServiceInterfaceMock
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (s *InitTestSuite) SetupTest() {
	s.governance = identitygovernancemock.NewIdentityGovernanceServiceInterfaceMock(s.T())
}

func (s *InitTestSuite) TestDisabledTypeYieldsDisabledProvider() {
	provider := Initialize(s.governance, config.IdentityGovernanceProviderConfig{Type: providerTypeDisabled})

	s.IsType(&disabledIdentityGovernanceProvider{}, provider)
}

func (s *InitTestSuite) TestOtherTypesYieldDefaultProvider() {
	for _, providerType := range []string{"default", "", "unknown"} {
		provider := Initialize(s.governance, config.IdentityGovernanceProviderConfig{Type: providerType})

		s.IsType(&defaultIdentityGovernanceProvider{}, provider, "type %q", providerType)
	}
}
