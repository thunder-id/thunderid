// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entityprovider

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
)

type InitEntityProviderTestSuite struct {
	suite.Suite
	mockEntityService *entitymock.EntityServiceInterfaceMock
}

func (suite *InitEntityProviderTestSuite) SetupTest() {
	suite.mockEntityService = entitymock.NewEntityServiceInterfaceMock(suite.T())

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
	}
	_ = config.InitializeServerRuntime("test", testConfig)
}

func (suite *InitEntityProviderTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func TestInitEntityProviderTestSuite(t *testing.T) {
	suite.Run(t, new(InitEntityProviderTestSuite))
}

func (suite *InitEntityProviderTestSuite) TestInitializeEntityProvider_WithDisabledType() {
	config.GetServerRuntime().Config.EntityProvider = config.EntityProviderConfig{
		Type: "disabled",
	}

	provider := InitializeEntityProvider(suite.mockEntityService, nil)

	suite.NotNil(provider)
	_, ok := provider.(*disabledEntityProvider)
	suite.True(ok, "Expected provider to be of type *disabledEntityProvider")
}

func (suite *InitEntityProviderTestSuite) TestInitializeEntityProvider_WithDefaultType() {
	config.GetServerRuntime().Config.EntityProvider = config.EntityProviderConfig{
		Type: "default",
	}

	provider := InitializeEntityProvider(suite.mockEntityService, nil)

	suite.NotNil(provider)
	_, ok := provider.(*defaultEntityProvider)
	suite.True(ok, "Expected provider to be of type *defaultEntityProvider")
}

func (suite *InitEntityProviderTestSuite) TestInitializeEntityProvider_WithEmptyType() {
	config.GetServerRuntime().Config.EntityProvider = config.EntityProviderConfig{
		Type: "",
	}

	provider := InitializeEntityProvider(suite.mockEntityService, nil)

	suite.NotNil(provider)
	_, ok := provider.(*defaultEntityProvider)
	suite.True(ok, "Expected provider to be of type *defaultEntityProvider when type is empty")
}

func (suite *InitEntityProviderTestSuite) TestInitializeEntityProvider_WithUnknownType() {
	config.GetServerRuntime().Config.EntityProvider = config.EntityProviderConfig{
		Type: "unknown",
	}

	provider := InitializeEntityProvider(suite.mockEntityService, nil)

	suite.NotNil(provider)
	_, ok := provider.(*defaultEntityProvider)
	suite.True(ok, "Expected provider to be of type *defaultEntityProvider for unknown type")
}
