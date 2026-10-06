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
	config.ResetServerRuntime()
	s.governance = identitygovernancemock.NewIdentityGovernanceServiceInterfaceMock(s.T())
}

func (s *InitTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (s *InitTestSuite) initRuntimeWithProviderType(providerType string) {
	s.Require().NoError(config.InitializeServerRuntime("test", &config.Config{
		IdentityGovernanceProvider: config.IdentityGovernanceProviderConfig{Type: providerType},
	}))
}

func (s *InitTestSuite) TestDisabledTypeYieldsDisabledProvider() {
	s.initRuntimeWithProviderType("disabled")

	s.IsType(&disabledIdentityGovernanceProvider{}, Initialize(s.governance))
}

func (s *InitTestSuite) TestOtherTypesYieldDefaultProvider() {
	for _, providerType := range []string{"default", "", "unknown"} {
		config.ResetServerRuntime()
		s.initRuntimeWithProviderType(providerType)

		s.IsType(&defaultIdentityGovernanceProvider{}, Initialize(s.governance), "type %q", providerType)
	}
}
