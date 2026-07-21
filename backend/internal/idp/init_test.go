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

package idp

import (
	"context"
	"errors"
	"testing"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

const (
	testCryptoKey = "0579f866ac7c9273580d0ff163fa01a7b2401a7ff3ddc3e3b14ae3136fa6025e"
)

type IDPInitTestSuite struct {
	suite.Suite
}

func TestIDPInitTestSuite(t *testing.T) {
	suite.Run(t, new(IDPInitTestSuite))
}

func (s *IDPInitTestSuite) SetupTest() {
	config.ResetServerRuntime()
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)
}

func (s *IDPInitTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (s *IDPInitTestSuite) TestInitialize() {
	config.ResetServerRuntime()
	// Initialize runtime config for the test
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
			Entity: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
	}
	_ = config.InitializeServerRuntime("", testConfig)

	service, err := Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"), nil)
	s.NoError(err)
	s.NotNil(service)
	s.Implements((*IDPServiceInterface)(nil), service)
}

func (s *IDPInitTestSuite) TestNewIDPService() {
	store := &idpStore{}
	service := newIDPService(store, nil, &mockTransactioner{})

	s.NotNil(service)
	s.Implements((*IDPServiceInterface)(nil), service)

	// Verify store is set correctly
	idpSvc, ok := service.(*idpService)
	s.True(ok)
	s.Equal(store, idpSvc.idpStore)
}

func (suite *IDPInitTestSuite) TestValidateIDPForInit_Valid() {
	prop1, _ := cmodels.NewProperty("client_id", "test_value", false)
	prop2, _ := cmodels.NewProperty("client_secret", "test_secret", false)
	prop3, _ := cmodels.NewProperty("redirect_uri", "http://localhost:3000/callback", false)

	idp := &providers.IDPDTO{
		ID:          "test-idp-1",
		Name:        "Test IDP",
		Description: "Test",
		Type:        providers.IDPTypeGoogle,
		Properties:  []cmodels.Property{*prop1, *prop2, *prop3},
	}

	logger := log.GetLogger()
	err := validateIDP(context.Background(), idp, logger)
	suite.Nil(err)
}

func (suite *IDPInitTestSuite) TestValidateIDPForInit_NilIDP() {
	logger := log.GetLogger()
	err := validateIDP(context.Background(), nil, logger)
	suite.NotNil(err)
	suite.Equal(ErrorIDPNil.Code, err.Code)
}

func (suite *IDPInitTestSuite) TestValidateIDPForInit_EmptyName() {
	idp := &providers.IDPDTO{
		ID:   "test-idp-1",
		Name: "",
		Type: providers.IDPTypeGoogle,
	}

	logger := log.GetLogger()
	err := validateIDP(context.Background(), idp, logger)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidIDPName.Code, err.Code)
}

func (suite *IDPInitTestSuite) TestValidateIDPForInit_EmptyType() {
	idp := &providers.IDPDTO{
		ID:   "test-idp-1",
		Name: "Test IDP",
		Type: "",
	}

	logger := log.GetLogger()
	err := validateIDP(context.Background(), idp, logger)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidIDPType.Code, err.Code)
}

func (suite *IDPInitTestSuite) TestValidateIDPForInit_InvalidType() {
	idp := &providers.IDPDTO{
		ID:   "test-idp-1",
		Name: "Test IDP",
		Type: "INVALID",
	}

	logger := log.GetLogger()
	err := validateIDP(context.Background(), idp, logger)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidIDPType.Code, err.Code)
}

// TestInitialize_WithDeclarativeResourcesDisabled tests the Initialize function when declarative resources are disabled
func (suite *IDPInitTestSuite) TestInitialize_WithDeclarativeResourcesDisabled() {
	// Setup - ensure config is reset and initialized for this test
	config.ResetServerRuntime()
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
			RuntimeTransient: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
			Entity: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
	}
	err := config.InitializeServerRuntime("", testConfig)
	assert.NoError(suite.T(), err)

	// Execute
	service, err := Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"), nil)

	// Assert
	suite.NoError(err)
	assert.NotNil(suite.T(), service)
	assert.Implements(suite.T(), (*IDPServiceInterface)(nil), service)
}

// TestGetIdentityProviderStoreMode_MutableMode verifies mutable mode detection
func (s *IDPInitTestSuite) TestGetIdentityProviderStoreMode_MutableMode() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
		IdentityProvider: config.IdentityProviderConfig{
			Store: "mutable",
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)

	mode := getIdentityProviderStoreMode()

	s.NotZero(mode)
	s.Equal(mode, serverconst.StoreModeMutable)
}

// TestGetIdentityProviderStoreMode_DeclarativeMode verifies declarative mode detection
func (s *IDPInitTestSuite) TestGetIdentityProviderStoreMode_DeclarativeMode() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: true,
		},
		IdentityProvider: config.IdentityProviderConfig{
			Store: "declarative",
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)

	mode := getIdentityProviderStoreMode()

	s.NotZero(mode)
	s.Equal(mode, serverconst.StoreModeDeclarative)
}

// TestGetIdentityProviderStoreMode_CompositeMode verifies composite mode detection
func (s *IDPInitTestSuite) TestGetIdentityProviderStoreMode_CompositeMode() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
		IdentityProvider: config.IdentityProviderConfig{
			Store: "composite",
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)

	mode := getIdentityProviderStoreMode()

	s.NotZero(mode)
	s.Equal(mode, serverconst.StoreModeComposite)
}

// TestGetIdentityProviderStoreMode_FallbackToGlobalSetting verifies fallback behavior
func (s *IDPInitTestSuite) TestGetIdentityProviderStoreMode_FallbackToGlobalSetting() {
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: true,
		},
		IdentityProvider: config.IdentityProviderConfig{
			Store: "", // Empty means use global setting
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)

	mode := getIdentityProviderStoreMode()

	s.Equal(mode, serverconst.StoreModeDeclarative)
}

// TestIsCompositeModeEnabled verifies composite mode flag
func (s *IDPInitTestSuite) TestIsCompositeModeEnabled() {
	testConfig := &config.Config{
		IdentityProvider: config.IdentityProviderConfig{
			Store: "composite",
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)

	enabled := isCompositeModeEnabled()

	s.True(enabled)
}

// TestIsMutableModeEnabled verifies mutable mode flag
func (s *IDPInitTestSuite) TestIsMutableModeEnabled() {
	testConfig := &config.Config{
		IdentityProvider: config.IdentityProviderConfig{
			Store: "mutable",
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)

	enabled := isMutableModeEnabled()

	s.True(enabled)
}

// TestIsDeclarativeModeEnabled verifies declarative mode flag
func (s *IDPInitTestSuite) TestIsDeclarativeModeEnabled() {
	testConfig := &config.Config{
		IdentityProvider: config.IdentityProviderConfig{
			Store: "declarative",
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)

	enabled := isDeclarativeModeEnabled()

	s.True(enabled)
}

// TestInitialize_DBClientError tests Initialize when DB client retrieval fails
func (s *IDPInitTestSuite) TestInitialize_DBClientError() {
	mockProvider := &providermock.DBProviderInterfaceMock{}
	mockProvider.On("GetConfigDBClient").Return(nil, errors.New("mock db client error"))

	originalGetDBProvider := getDBProvider
	getDBProvider = func() provider.DBProviderInterface {
		return mockProvider
	}
	defer func() {
		getDBProvider = originalGetDBProvider
	}()

	_, err := Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"), nil)

	s.Error(err)
	s.Equal("mock db client error", err.Error())
	mockProvider.AssertExpectations(s.T())
}

// TestInitialize_TransactionerError tests Initialize when transactioner retrieval fails
func (s *IDPInitTestSuite) TestInitialize_TransactionerError() {
	mockClient := &providermock.DBClientInterfaceMock{}
	mockClient.On("GetTransactioner").Return(nil, errors.New("mock transactioner error"))

	mockProvider := &providermock.DBProviderInterfaceMock{}
	mockProvider.On("GetConfigDBClient").Return(mockClient, nil)

	originalGetDBProvider := getDBProvider
	getDBProvider = func() provider.DBProviderInterface {
		return mockProvider
	}
	defer func() {
		getDBProvider = originalGetDBProvider
	}()

	_, err := Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"), nil)

	s.Error(err)
	s.Equal("mock transactioner error", err.Error())
	mockProvider.AssertExpectations(s.T())
	mockClient.AssertExpectations(s.T())
}
