/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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
			Runtime: config.DataSource{
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

	provider := InitializeEntityProvider(suite.mockEntityService)

	suite.NotNil(provider)
	_, ok := provider.(*disabledEntityProvider)
	suite.True(ok, "Expected provider to be of type *disabledEntityProvider")
}

func (suite *InitEntityProviderTestSuite) TestInitializeEntityProvider_WithDefaultType() {
	config.GetServerRuntime().Config.EntityProvider = config.EntityProviderConfig{
		Type: "default",
	}

	provider := InitializeEntityProvider(suite.mockEntityService)

	suite.NotNil(provider)
	_, ok := provider.(*defaultEntityProvider)
	suite.True(ok, "Expected provider to be of type *defaultEntityProvider")
}

func (suite *InitEntityProviderTestSuite) TestInitializeEntityProvider_WithEmptyType() {
	config.GetServerRuntime().Config.EntityProvider = config.EntityProviderConfig{
		Type: "",
	}

	provider := InitializeEntityProvider(suite.mockEntityService)

	suite.NotNil(provider)
	_, ok := provider.(*defaultEntityProvider)
	suite.True(ok, "Expected provider to be of type *defaultEntityProvider when type is empty")
}

func (suite *InitEntityProviderTestSuite) TestInitializeEntityProvider_WithUnknownType() {
	config.GetServerRuntime().Config.EntityProvider = config.EntityProviderConfig{
		Type: "unknown",
	}

	provider := InitializeEntityProvider(suite.mockEntityService)

	suite.NotNil(provider)
	_, ok := provider.(*defaultEntityProvider)
	suite.True(ok, "Expected provider to be of type *defaultEntityProvider for unknown type")
}
