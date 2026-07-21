/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package notification

import (
	"errors"
	"testing"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
)

const (
	testCryptoKey = "0579f866ac7c9273580d0ff163fa01a7b2401a7ff3ddc3e3b14ae3136fa6025e"
)

type InitTestSuite struct {
	suite.Suite
	mockJWTService *jwtmock.JWTServiceInterfaceMock
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (suite *InitTestSuite) SetupSuite() {
	config.ResetServerRuntime()
	testConfig := &config.Config{
		JWT: engineconfig.JWTConfig{
			Issuer:         "test-issuer",
			ValidityPeriod: 3600,
		},
		Crypto: config.CryptoConfig{
			Encryption: engineconfig.EncryptionConfig{
				Key: testCryptoKey,
			},
		},
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
	}
	err := config.InitializeServerRuntime("", testConfig)
	if err != nil {
		suite.T().Fatalf("Failed to initialize server runtime: %v", err)
	}
}

func (suite *InitTestSuite) SetupTest() {
	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())
}

func (suite *InitTestSuite) TearDownSuite() {
	config.ResetServerRuntime()
}

func (suite *InitTestSuite) TestInitialize() {
	mgtService, _, _, err := Initialize(suite.mockJWTService)
	suite.NoError(err)

	suite.NotNil(mgtService)
	suite.Implements((*NotificationSenderMgtSvcInterface)(nil), mgtService)
}

func (suite *InitTestSuite) TestInitialize_StoreErrorPropagates() {
	originalGetDBProvider := getDBProvider
	mockProvider := providermock.NewDBProviderInterfaceMock(suite.T())
	mockProvider.On("GetConfigDBClient").Return(nil, errors.New("db unavailable"))
	getDBProvider = func() provider.DBProviderInterface { return mockProvider }
	defer func() { getDBProvider = originalGetDBProvider }()

	mgtService, otpService, senderService, err := Initialize(suite.mockJWTService)

	suite.Error(err)
	suite.Nil(mgtService)
	suite.Nil(otpService)
	suite.Nil(senderService)
}
