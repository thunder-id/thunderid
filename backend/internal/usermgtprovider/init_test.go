// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package usermgtprovider

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/tests/mocks/usermock"
)

type InitUserMgtProviderTestSuite struct {
	suite.Suite
	mockUserService *usermock.UserServiceInterfaceMock
}

func (suite *InitUserMgtProviderTestSuite) SetupTest() {
	suite.mockUserService = usermock.NewUserServiceInterfaceMock(suite.T())
}

func (suite *InitUserMgtProviderTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func TestInitUserMgtProviderTestSuite(t *testing.T) {
	suite.Run(t, new(InitUserMgtProviderTestSuite))
}

func (suite *InitUserMgtProviderTestSuite) initRuntimeWithProviderType(providerType string) {
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
		UserMgtProvider: config.UserMgtProviderConfig{Type: providerType},
	}
	suite.Require().NoError(config.InitializeServerRuntime("test", testConfig))
}

func (suite *InitUserMgtProviderTestSuite) TestDisabledTypeYieldsDisabledProvider() {
	suite.initRuntimeWithProviderType("disabled")

	provider := Initialize(suite.mockUserService)

	suite.IsType(&disabledUserMgtProvider{}, provider)
}

// Deployments with no user_mgt_provider block must keep provisioning available.
func (suite *InitUserMgtProviderTestSuite) TestEmptyTypeYieldsDefaultProvider() {
	suite.initRuntimeWithProviderType("")

	provider := Initialize(suite.mockUserService)

	suite.IsType(&defaultUserMgtProvider{}, provider)
}

// An unrecognized type must fall back to the default provider rather than yielding a nil provider
// that would panic on first use.
func (suite *InitUserMgtProviderTestSuite) TestUnknownTypeYieldsDefaultProvider() {
	suite.initRuntimeWithProviderType("not-a-provider-type")

	provider := Initialize(suite.mockUserService)

	suite.IsType(&defaultUserMgtProvider{}, provider)
}
