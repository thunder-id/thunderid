// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agentmgtprovider

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
)

type InitAgentMgtProviderTestSuite struct {
	suite.Suite
}

func (suite *InitAgentMgtProviderTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func TestInitAgentMgtProviderTestSuite(t *testing.T) {
	suite.Run(t, new(InitAgentMgtProviderTestSuite))
}

func (suite *InitAgentMgtProviderTestSuite) initRuntimeWithProviderType(providerType string) {
	testConfig := &config.Config{
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
			RuntimeTransient: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
		AgentMgtProvider: config.AgentMgtProviderConfig{Type: providerType},
	}
	suite.Require().NoError(config.InitializeServerRuntime("test", testConfig))
}

func (suite *InitAgentMgtProviderTestSuite) TestDisabledTypeYieldsDisabledProvider() {
	suite.initRuntimeWithProviderType("disabled")

	provider := Initialize(nil)

	suite.IsType(&disabledAgentMgtProvider{}, provider)
}

// Deployments with no agent_provider block must keep provisioning available.
func (suite *InitAgentMgtProviderTestSuite) TestEmptyTypeYieldsDefaultProvider() {
	suite.initRuntimeWithProviderType("")

	provider := Initialize(nil)

	suite.IsType(&defaultAgentMgtProvider{}, provider)
}

// An unrecognized type must fall back to the default provider rather than yielding a nil provider
// that would panic on first use.
func (suite *InitAgentMgtProviderTestSuite) TestUnknownTypeYieldsDefaultProvider() {
	suite.initRuntimeWithProviderType("not-a-provider-type")

	provider := Initialize(nil)

	suite.IsType(&defaultAgentMgtProvider{}, provider)
}
