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
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

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
			Runtime: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
			User: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
	}
	_ = config.InitializeServerRuntime("", testConfig)
	mux := http.NewServeMux()

	service, _, err := Initialize(cache.Initialize(), mux)
	s.NoError(err)
	s.NotNil(service)
	s.Implements((*IDPServiceInterface)(nil), service)
}

func (s *IDPInitTestSuite) TestRegisterRoutes() {
	mux := http.NewServeMux()
	handler := &idpHandler{}

	// This test mainly ensures registerRoutes doesn't panic
	s.NotPanics(func() {
		registerRoutes(mux, handler)
	})

	// Verify expected routes are registered on the mux without invoking handlers
	cases := []struct {
		method   string
		target   string
		expected string
	}{
		{method: http.MethodPost, target: "/identity-providers", expected: "POST /identity-providers"},
		{method: http.MethodGet, target: "/identity-providers", expected: "GET /identity-providers"},
		{method: http.MethodOptions, target: "/identity-providers", expected: "OPTIONS /identity-providers"},
		{method: http.MethodGet, target: "/identity-providers/123", expected: "GET /identity-providers/{id}"},
		{method: http.MethodPut, target: "/identity-providers/123", expected: "PUT /identity-providers/{id}"},
		{method: http.MethodDelete, target: "/identity-providers/123", expected: "DELETE /identity-providers/{id}"},
		{method: http.MethodOptions, target: "/identity-providers/123", expected: "OPTIONS /identity-providers/{id}"},
	}

	for _, c := range cases {
		req := httptest.NewRequest(c.method, c.target, nil)
		_, pattern := mux.Handler(req)
		s.Equal(c.expected, pattern)
	}
}

func (s *IDPInitTestSuite) TestNewIDPHandler() {
	service := &idpService{}
	handler := newIDPHandler(service)

	s.NotNil(handler)
	s.Equal(service, handler.idpService)
}

func (s *IDPInitTestSuite) TestNewIDPService() {
	store := &idpStore{}
	service := newIDPService(store, &mockTransactioner{})

	s.NotNil(service)
	s.Implements((*IDPServiceInterface)(nil), service)

	// Verify store is set correctly
	idpSvc, ok := service.(*idpService)
	s.True(ok)
	s.Equal(store, idpSvc.idpStore)
}

func (suite *IDPInitTestSuite) TestParseToIDPDTO_Valid() {
	yamlData := `
id: "test-idp-1"
name: "Test IDP"
description: "Test Identity Provider"
type: "GOOGLE"
properties:
  - name: "client_id"
    value: "test_client_id"
    is_secret: false
  - name: "client_secret"
    value: "test_secret"
    is_secret: false
`

	idp, err := parseToIDPDTO([]byte(yamlData))
	suite.NoError(err)
	suite.NotNil(idp)
	suite.Equal("test-idp-1", idp.ID)
	suite.Equal("Test IDP", idp.Name)
	suite.Equal("Test Identity Provider", idp.Description)
	suite.Equal(IDPTypeGoogle, idp.Type)
	suite.Len(idp.Properties, 2)
}

func (suite *IDPInitTestSuite) TestParseToIDPDTO_InvalidYAML() {
	yamlData := `
invalid yaml content
  - this is not valid
`

	idp, err := parseToIDPDTO([]byte(yamlData))
	suite.Error(err)
	suite.Nil(idp)
}

func (suite *IDPInitTestSuite) TestParseToIDPDTO_InvalidType() {
	yamlData := `
id: "test-idp-2"
name: "Test IDP"
type: "INVALID_TYPE"
`

	idp, err := parseToIDPDTO([]byte(yamlData))
	suite.Error(err)
	suite.Nil(idp)
	suite.Contains(err.Error(), "unsupported IDP type")
}

func (suite *IDPInitTestSuite) TestParseIDPType_Google() {
	idpType, err := parseIDPType("GOOGLE")
	suite.NoError(err)
	suite.Equal(IDPTypeGoogle, idpType)
}

func (suite *IDPInitTestSuite) TestParseIDPType_GitHub() {
	idpType, err := parseIDPType("GITHUB")
	suite.NoError(err)
	suite.Equal(IDPTypeGitHub, idpType)
}

func (suite *IDPInitTestSuite) TestParseIDPType_OIDC() {
	idpType, err := parseIDPType("OIDC")
	suite.NoError(err)
	suite.Equal(IDPTypeOIDC, idpType)
}

func (suite *IDPInitTestSuite) TestParseIDPType_OAuth() {
	idpType, err := parseIDPType("OAUTH")
	suite.NoError(err)
	suite.Equal(IDPTypeOAuth, idpType)
}

func (suite *IDPInitTestSuite) TestParseIDPType_Invalid() {
	idpType, err := parseIDPType("INVALID")
	suite.Error(err)
	suite.Empty(idpType)
	suite.Contains(err.Error(), "unsupported IDP type")
}

func (suite *IDPInitTestSuite) TestValidateIDPForInit_Valid() {
	prop1, _ := cmodels.NewProperty("client_id", "test_value", false)
	prop2, _ := cmodels.NewProperty("client_secret", "test_secret", false)
	prop3, _ := cmodels.NewProperty("redirect_uri", "http://localhost:3000/callback", false)

	idp := &IDPDTO{
		ID:          "test-idp-1",
		Name:        "Test IDP",
		Description: "Test",
		Type:        IDPTypeGoogle,
		Properties:  []cmodels.Property{*prop1, *prop2, *prop3},
	}

	logger := log.GetLogger()
	err := validateIDP(idp, logger)
	suite.Nil(err)
}

func (suite *IDPInitTestSuite) TestValidateIDPForInit_NilIDP() {
	logger := log.GetLogger()
	err := validateIDP(nil, logger)
	suite.NotNil(err)
	suite.Equal(ErrorIDPNil.Code, err.Code)
}

func (suite *IDPInitTestSuite) TestValidateIDPForInit_EmptyName() {
	idp := &IDPDTO{
		ID:   "test-idp-1",
		Name: "",
		Type: IDPTypeGoogle,
	}

	logger := log.GetLogger()
	err := validateIDP(idp, logger)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidIDPName.Code, err.Code)
}

func (suite *IDPInitTestSuite) TestValidateIDPForInit_EmptyType() {
	idp := &IDPDTO{
		ID:   "test-idp-1",
		Name: "Test IDP",
		Type: "",
	}

	logger := log.GetLogger()
	err := validateIDP(idp, logger)
	suite.NotNil(err)
	suite.Equal(ErrorInvalidIDPType.Code, err.Code)
}

func (suite *IDPInitTestSuite) TestValidateIDPForInit_InvalidType() {
	idp := &IDPDTO{
		ID:   "test-idp-1",
		Name: "Test IDP",
		Type: "INVALID",
	}

	logger := log.GetLogger()
	err := validateIDP(idp, logger)
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
			Runtime: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
			User: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
	}
	err := config.InitializeServerRuntime("", testConfig)
	assert.NoError(suite.T(), err)

	mux := http.NewServeMux()

	// Execute
	service, _, err := Initialize(cache.Initialize(), mux)

	// Assert
	suite.NoError(err)
	assert.NotNil(suite.T(), service)
	assert.Implements(suite.T(), (*IDPServiceInterface)(nil), service)
}

// TestInitialize_WithDeclarativeResourcesEnabled_EmptyDirectory tests Initialize with declarative resources
// enabled but no configuration files in the directory
func TestInitialize_WithDeclarativeResourcesEnabled_EmptyDirectory(t *testing.T) {
	// Setup minimal config for testing
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: true,
		},
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: "test.db"},
			},
			Runtime: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: "test.db"},
			},
			User: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: "test.db"},
			},
		},
	}

	// Create a temporary directory structure for file-based runtime
	tmpDir := t.TempDir()
	confDir := tmpDir + "/repository/resources"
	idpDir := confDir + "/identity_providers"

	// Create the directory structure
	err := os.MkdirAll(idpDir, 0750)
	assert.NoError(t, err)

	// Reset and initialize with test config
	config.ResetServerRuntime()
	err = config.InitializeServerRuntime(tmpDir, testConfig)
	assert.NoError(t, err)

	defer config.ResetServerRuntime() // Clean up after test

	mux := http.NewServeMux()

	// Execute
	service, _, err := Initialize(cache.Initialize(), mux)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, service)
	assert.Implements(t, (*IDPServiceInterface)(nil), service)

	// Verify no IDPs are loaded
	idps, svcErr := service.GetIdentityProviderList(context.Background())
	assert.Nil(t, svcErr)
	assert.Empty(t, idps)
}

// TestInitialize_WithDeclarativeResourcesEnabled_ValidConfigs tests Initialize with declarative resources
// enabled and valid YAML configuration files
func TestInitialize_WithDeclarativeResourcesEnabled_ValidConfigs(t *testing.T) {
	// Create a temporary directory structure for file-based runtime
	tmpDir := t.TempDir()
	confDir := tmpDir + "/repository/resources"
	idpDir := confDir + "/identity_providers"

	// Create the directory structure
	err := os.MkdirAll(idpDir, 0750)
	assert.NoError(t, err)

	// Setup config with encryption support (path relative to server home)
	testConfig := &config.Config{
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: "test.db"},
			},
			Runtime: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: "test.db"},
			},
			User: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: "test.db"},
			},
		},
		DeclarativeResources: config.DeclarativeResources{
			Enabled: true,
		},
		Crypto: config.CryptoConfig{
			Encryption: config.EncryptionConfig{
				Key: testCryptoKey,
			},
		},
	}

	// Create a valid Google IDP YAML file
	googleIDPYAML := `id: google-idp-1
name: Google IDP
description: Google Identity Provider for SSO
type: GOOGLE
properties:
  - name: client_id
    value: google-client-id
    is_secret: false
  - name: client_secret
    value: google-client-secret
    is_secret: true
  - name: redirect_uri
    value: http://localhost:3000/callback
    is_secret: false
`
	err = os.WriteFile(idpDir+"/google_idp.yaml", []byte(googleIDPYAML), 0600)
	assert.NoError(t, err)

	// Create a valid GitHub IDP YAML file
	githubIDPYAML := `id: github-idp-1
name: GitHub IDP
description: GitHub Identity Provider
type: GITHUB
properties:
  - name: client_id
    value: github-client-id
    is_secret: false
  - name: client_secret
    value: github-client-secret
    is_secret: true
  - name: redirect_uri
    value: http://localhost:3000/callback
    is_secret: false
`
	err = os.WriteFile(idpDir+"/github_idp.yaml", []byte(githubIDPYAML), 0600)
	assert.NoError(t, err)

	// Reset and initialize with test config
	config.ResetServerRuntime()
	err = config.InitializeServerRuntime(tmpDir, testConfig)
	assert.NoError(t, err)

	defer config.ResetServerRuntime() // Clean up after test

	mux := http.NewServeMux()

	// Execute
	service, _, err := Initialize(cache.Initialize(), mux)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, service)
	assert.Implements(t, (*IDPServiceInterface)(nil), service)

	// Verify IDPs are loaded
	idps, svcErr := service.GetIdentityProviderList(context.Background())
	assert.Nil(t, svcErr)
	assert.Len(t, idps, 2)

	// Verify IDP names (order may vary)
	idpNames := []string{idps[0].Name, idps[1].Name}
	assert.Contains(t, idpNames, "Google IDP")
	assert.Contains(t, idpNames, "GitHub IDP")

	// Verify we can get individual IDPs by name
	googleIDP, svcErr := service.GetIdentityProviderByName(context.Background(), "Google IDP")
	assert.Nil(t, svcErr)
	assert.NotNil(t, googleIDP)
	assert.Equal(t, "Google IDP", googleIDP.Name)
	assert.Equal(t, IDPTypeGoogle, googleIDP.Type)
	// Google IDP should have 8 properties after defaults are applied:
	// client_id, client_secret, redirect_uri (from YAML) + authorization_endpoint, token_endpoint,
	// jwks_endpoint, userinfo_endpoint, scopes (defaults)
	assert.Len(t, googleIDP.Properties, 8)

	githubIDP, svcErr := service.GetIdentityProviderByName(context.Background(), "GitHub IDP")
	assert.Nil(t, svcErr)
	assert.NotNil(t, githubIDP)
	assert.Equal(t, "GitHub IDP", githubIDP.Name)
	assert.Equal(t, IDPTypeGitHub, githubIDP.Type)
	// GitHub IDP should have 7 properties after defaults are applied:
	// client_id, client_secret, redirect_uri (from YAML) + authorization_endpoint, token_endpoint,
	// userinfo_endpoint, user_email_endpoint (defaults)
	assert.Len(t, githubIDP.Properties, 7)
}

// TestInitialize_WithDeclarativeResourcesEnabled_InvalidYAML tests Initialize with invalid YAML files
//
//nolint:dupl // Similar test setup required for different error scenarios
func TestInitialize_WithDeclarativeResourcesEnabled_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	confDir := tmpDir + "/repository/resources"
	idpDir := confDir + "/identity_providers"

	err := os.MkdirAll(idpDir, 0750)
	assert.NoError(t, err)

	// Create an invalid YAML file
	invalidYAML := `invalid yaml content
  - this is not: valid
`
	err = os.WriteFile(idpDir+"/invalid-idp.yaml", []byte(invalidYAML), 0600)
	assert.NoError(t, err)

	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: true,
		},
		Crypto: config.CryptoConfig{
			Encryption: config.EncryptionConfig{
				Key: testCryptoKey,
			},
		},
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: "test.db"},
			},
			Runtime: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: "test.db"},
			},
			User: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: "test.db"},
			},
		},
	}

	config.ResetServerRuntime()
	err = config.InitializeServerRuntime(tmpDir, testConfig)
	assert.NoError(t, err)
	defer config.ResetServerRuntime()

	mux := http.NewServeMux()

	// Initialize should return an error due to invalid YAML
	_, _, err = Initialize(cache.Initialize(), mux)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load identity provider resources")
}

// TestInitialize_WithDeclarativeResourcesEnabled_ValidationFailure tests Initialize with validation errors
//
//nolint:dupl // Similar test setup required for different error scenarios
func TestInitialize_WithDeclarativeResourcesEnabled_ValidationFailure(t *testing.T) {
	tmpDir := t.TempDir()
	confDir := tmpDir + "/repository/resources"
	idpDir := confDir + "/identity_providers"

	err := os.MkdirAll(idpDir, 0750)
	assert.NoError(t, err)

	// Create a YAML file with invalid configuration (empty name)
	invalidIDPYAML := `id: "invalid-idp"
name: ""
type: GOOGLE
properties:
  - name: "client_id"
    value: "test"
    is_secret: false
`
	err = os.WriteFile(idpDir+"/invalid-idp.yaml", []byte(invalidIDPYAML), 0600)
	assert.NoError(t, err)

	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: true,
		},
		Crypto: config.CryptoConfig{
			Encryption: config.EncryptionConfig{
				Key: testCryptoKey,
			},
		},
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: "test.db"},
			},
			Runtime: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: "test.db"},
			},
			User: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: "test.db"},
			},
		},
	}

	config.ResetServerRuntime()
	err = config.InitializeServerRuntime(tmpDir, testConfig)
	assert.NoError(t, err)
	defer config.ResetServerRuntime()

	mux := http.NewServeMux()

	// Initialize should return an error due to validation failure
	_, _, err = Initialize(cache.Initialize(), mux)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load identity provider resources")
}

// TestInitialize_WithDeclarativeResourcesEnabled_InvalidIDPType tests Initialize with invalid IDP type
//
//nolint:dupl // Similar test setup required for different error scenarios
func TestInitialize_WithDeclarativeResourcesEnabled_InvalidIDPType(t *testing.T) {
	tmpDir := t.TempDir()
	confDir := tmpDir + "/repository/resources"
	idpDir := confDir + "/identity_providers"

	err := os.MkdirAll(idpDir, 0750)
	assert.NoError(t, err)

	// Create a YAML file with invalid IDP type
	invalidTypeYAML := `id: "invalid-type-idp"
name: "Invalid Type IDP"
type: "UNSUPPORTED_TYPE"
properties:
  - name: "client_id"
    value: "test"
    is_secret: false
`
	err = os.WriteFile(idpDir+"/invalid-type.yaml", []byte(invalidTypeYAML), 0600)
	assert.NoError(t, err)

	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: true,
		},
		Crypto: config.CryptoConfig{
			Encryption: config.EncryptionConfig{
				Key: testCryptoKey,
			},
		},
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: "test.db"},
			},
			Runtime: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: "test.db"},
			},
			User: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: "test.db"},
			},
		},
	}

	config.ResetServerRuntime()
	err = config.InitializeServerRuntime(tmpDir, testConfig)
	assert.NoError(t, err)
	defer config.ResetServerRuntime()

	mux := http.NewServeMux()

	// Initialize should return an error due to invalid IDP type
	_, _, err = Initialize(cache.Initialize(), mux)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load identity provider resources")
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

	mux := http.NewServeMux()
	_, _, err := Initialize(cache.Initialize(), mux)

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

	mux := http.NewServeMux()
	_, _, err := Initialize(cache.Initialize(), mux)

	s.Error(err)
	s.Equal("mock transactioner error", err.Error())
	mockProvider.AssertExpectations(s.T())
	mockClient.AssertExpectations(s.T())
}
