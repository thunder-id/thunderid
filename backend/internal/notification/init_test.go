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
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
	"github.com/thunder-id/thunderid/tests/mocks/templatemock"
)

const (
	testCryptoKey = "0579f866ac7c9273580d0ff163fa01a7b2401a7ff3ddc3e3b14ae3136fa6025e"
)

type InitTestSuite struct {
	suite.Suite
	mockJWTService      *jwtmock.JWTServiceInterfaceMock
	mockTemplateService *templatemock.TemplateServiceInterfaceMock
	mux                 *http.ServeMux
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (suite *InitTestSuite) SetupSuite() {
	config.ResetServerRuntime()
	testConfig := &config.Config{
		JWT: config.JWTConfig{
			Issuer:         "test-issuer",
			ValidityPeriod: 3600,
		},
		Crypto: config.CryptoConfig{
			Encryption: config.EncryptionConfig{
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
	suite.mockTemplateService = templatemock.NewTemplateServiceInterfaceMock(suite.T())
	suite.mux = http.NewServeMux()
}

func (suite *InitTestSuite) TearDownSuite() {
	config.ResetServerRuntime()
}

func (suite *InitTestSuite) TestInitialize() {
	mgtService, otpService, _, _, err := Initialize(suite.mux, suite.mockJWTService, suite.mockTemplateService)
	suite.NoError(err)

	suite.NotNil(mgtService)
	suite.NotNil(otpService)
	suite.Implements((*NotificationSenderMgtSvcInterface)(nil), mgtService)
	suite.Implements((*OTPServiceInterface)(nil), otpService)
}

// TestInitialize_WithDeclarativeResourcesEnabled_FileLoading tests that notification senders can be
// loaded from YAML files when declarative resources are enabled. This test verifies the file parsing
// and loading logic without triggering the service create operation which would cause os.Exit.
func (suite *InitTestSuite) TestInitialize_WithDeclarativeResourcesEnabled_FileLoading() {
	// Create a temporary directory for declarative resources
	tmpDir := suite.T().TempDir()
	confDir := tmpDir + "/repository/resources"
	senderDir := confDir + "/notification_senders"

	// Create the directory structure
	err := os.MkdirAll(senderDir, 0750)
	suite.NoError(err)

	// Create test notification sender YAML files
	twilioYAML := `id: "test-twilio-sender"
name: "Test Twilio Sender"
description: "Test Twilio notification sender"
provider: "twilio"
properties:
  - name: "account_sid"
    value: "AC00112233445566778899aabbccddeeff"
    is_secret: true
  - name: "auth_token"
    value: "test-auth-token"
    is_secret: true
  - name: "sender_id"
    value: "+15551234567"
    is_secret: false
`
	err = os.WriteFile(filepath.Join(senderDir, "twilio-sender.yaml"), []byte(twilioYAML), 0600)
	suite.NoError(err)

	vonageYAML := `id: "test-vonage-sender"
name: "Test Vonage Sender"
description: "Test Vonage notification sender"
provider: "vonage"
properties:
  - name: "api_key"
    value: "test-api-key"
    is_secret: true
  - name: "api_secret"
    value: "test-api-secret"
    is_secret: true
  - name: "sender_id"
    value: "VonageTest"
    is_secret: false
`
	err = os.WriteFile(filepath.Join(senderDir, "vonage-sender.yaml"), []byte(vonageYAML), 0600)
	suite.NoError(err)

	// Create tests/resources directory in tmpDir
	testsResourcesDir := filepath.Join(tmpDir, "tests", "resources")
	err = os.MkdirAll(testsResourcesDir, 0750)
	suite.NoError(err)

	// Reset and initialize config with declarative resources enabled
	config.ResetServerRuntime()
	testConfig := &config.Config{
		JWT: config.JWTConfig{
			Issuer:         "test-issuer",
			ValidityPeriod: 3600,
		},
		Crypto: config.CryptoConfig{
			Encryption: config.EncryptionConfig{
				Key: testCryptoKey,
			},
		},
		DeclarativeResources: config.DeclarativeResources{
			Enabled: true,
		},
	}
	err = config.InitializeServerRuntime(tmpDir, testConfig)
	suite.NoError(err)

	// Verify files can be loaded using the file-based runtime
	configs, err := declarativeresource.GetConfigs("notification_senders")
	suite.NoError(err)
	suite.Len(configs, 2, "Expected 2 notification sender configs to be loaded")

	// Parse the first config and verify structure
	senderDTO, err := parseToNotificationSenderDTO(configs[0])
	suite.NoError(err)
	suite.NotNil(senderDTO)
	suite.NotEmpty(senderDTO.ID)
	suite.NotEmpty(senderDTO.Name)
	suite.NotEmpty(senderDTO.Provider)
	suite.NotEmpty(senderDTO.Properties)

	// Verify properties include both secret and non-secret values
	hasSecretProp := false
	hasNonSecretProp := false
	for _, prop := range senderDTO.Properties {
		if prop.IsSecret() {
			hasSecretProp = true
		} else {
			hasNonSecretProp = true
		}
	}
	suite.True(hasSecretProp, "Expected at least one secret property")
	suite.True(hasNonSecretProp, "Expected at least one non-secret property")

	// Clean up - reset config and reinitialize with suite's test config
	config.ResetServerRuntime()
	suiteConfig := &config.Config{
		JWT: config.JWTConfig{
			Issuer:         "test-issuer",
			ValidityPeriod: 3600,
		},
		Crypto: config.CryptoConfig{
			Encryption: config.EncryptionConfig{
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
	err = config.InitializeServerRuntime("", suiteConfig)
	suite.NoError(err)
}

func (suite *InitTestSuite) TestRegisterRoutes_ListEndpoint() {
	_, _, _, _, err := Initialize(suite.mux, suite.mockJWTService, suite.mockTemplateService)
	suite.NoError(err)

	req := httptest.NewRequest(http.MethodGet, "/notification-senders/message", nil)
	w := httptest.NewRecorder()

	suite.mux.ServeHTTP(w, req)

	suite.NotEqual(http.StatusNotFound, w.Code)
}

func (suite *InitTestSuite) TestRegisterRoutes_CreateEndpoint() {
	_, _, _, _, err := Initialize(suite.mux, suite.mockJWTService, suite.mockTemplateService)
	suite.NoError(err)

	req := httptest.NewRequest(http.MethodPost, "/notification-senders/message", nil)
	w := httptest.NewRecorder()

	suite.mux.ServeHTTP(w, req)

	suite.NotEqual(http.StatusNotFound, w.Code)
}

func (suite *InitTestSuite) TestRegisterRoutes_GetByIDEndpoint() {
	_, _, _, _, err := Initialize(suite.mux, suite.mockJWTService, suite.mockTemplateService)
	suite.NoError(err)

	req := httptest.NewRequest(http.MethodGet, "/notification-senders/message/test-id", nil)
	w := httptest.NewRecorder()

	suite.mux.ServeHTTP(w, req)

	suite.NotEqual(http.StatusNotFound, w.Code)
}

func (suite *InitTestSuite) TestRegisterRoutes_UpdateEndpoint() {
	_, _, _, _, err := Initialize(suite.mux, suite.mockJWTService, suite.mockTemplateService)
	suite.NoError(err)

	req := httptest.NewRequest(http.MethodPut, "/notification-senders/message/test-id", nil)
	w := httptest.NewRecorder()

	suite.mux.ServeHTTP(w, req)

	suite.NotEqual(http.StatusNotFound, w.Code)
}

func (suite *InitTestSuite) TestRegisterRoutes_DeleteEndpoint() {
	_, _, _, _, err := Initialize(suite.mux, suite.mockJWTService, suite.mockTemplateService)
	suite.NoError(err)

	req := httptest.NewRequest(http.MethodDelete, "/notification-senders/message/test-id", nil)
	w := httptest.NewRecorder()

	suite.mux.ServeHTTP(w, req)

	suite.NotEqual(http.StatusNotFound, w.Code)
}

func (suite *InitTestSuite) TestRegisterRoutes_SendOTPEndpoint() {
	_, _, _, _, err := Initialize(suite.mux, suite.mockJWTService, suite.mockTemplateService)
	suite.NoError(err)

	req := httptest.NewRequest(http.MethodPost, "/notification-senders/otp/send", nil)
	w := httptest.NewRecorder()

	suite.mux.ServeHTTP(w, req)

	suite.NotEqual(http.StatusNotFound, w.Code)
}

func (suite *InitTestSuite) TestRegisterRoutes_VerifyOTPEndpoint() {
	_, _, _, _, err := Initialize(suite.mux, suite.mockJWTService, suite.mockTemplateService)
	suite.NoError(err)

	req := httptest.NewRequest(http.MethodPost, "/notification-senders/otp/verify", nil)
	w := httptest.NewRecorder()

	suite.mux.ServeHTTP(w, req)

	suite.NotEqual(http.StatusNotFound, w.Code)
}

func (suite *InitTestSuite) TestRegisterRoutes_CORSPreflight() {
	_, _, _, _, err := Initialize(suite.mux, suite.mockJWTService, suite.mockTemplateService)
	suite.NoError(err)

	paths := []string{
		"/notification-senders/message",
		"/notification-senders/message/test-id",
		"/notification-senders/otp/send",
		"/notification-senders/otp/verify",
	}

	for _, path := range paths {
		req := httptest.NewRequest(http.MethodOptions, path, nil)
		w := httptest.NewRecorder()

		suite.mux.ServeHTTP(w, req)

		suite.NotEqual(http.StatusNotFound, w.Code)
	}
}

// TestParseToNotificationSenderDTO_ValidYAML tests parsing a valid YAML configuration.
func (suite *InitTestSuite) TestParseToNotificationSenderDTO_ValidYAML() {
	yamlData := `
id: "twilio-sender-001"
name: "Twilio SMS Sender"
description: "Production Twilio SMS sender"
provider: "twilio"
properties:
  - name: "account_sid"
    value: "{{.TWILIO_ACCOUNT_SID}}"
    is_secret: false
  - name: "auth_token"
    value: "{{.TWILIO_AUTH_TOKEN}}"
    is_secret: true
  - name: "sender_id"
    value: "{{.TWILIO_FROM_NUMBER}}"
    is_secret: false
`

	sender, err := parseToNotificationSenderDTO([]byte(yamlData))

	suite.NoError(err)
	suite.NotNil(sender)
	suite.Equal("twilio-sender-001", sender.ID)
	suite.Equal("Twilio SMS Sender", sender.Name)
	suite.Equal("Production Twilio SMS sender", sender.Description)
	suite.Equal("twilio", string(sender.Provider))
	suite.Len(sender.Properties, 3)
}

// TestParseToNotificationSenderDTO_InvalidYAML tests parsing invalid YAML.
func (suite *InitTestSuite) TestParseToNotificationSenderDTO_InvalidYAML() {
	yamlData := `
invalid yaml content
  - this is not valid
`

	sender, err := parseToNotificationSenderDTO([]byte(yamlData))

	suite.Error(err)
	suite.Nil(sender)
}

// TestParseToNotificationSenderDTO_MinimalYAML tests parsing minimal YAML configuration.
func (suite *InitTestSuite) TestParseToNotificationSenderDTO_MinimalYAML() {
	yamlData := `
id: "minimal-sender"
name: "Minimal Sender"
provider: "custom"
properties:
  - name: "url"
    value: "https://custom.example.com/sms"
`

	sender, err := parseToNotificationSenderDTO([]byte(yamlData))

	suite.NoError(err)
	suite.NotNil(sender)
	suite.Equal("minimal-sender", sender.ID)
	suite.Equal("Minimal Sender", sender.Name)
	suite.Equal("", sender.Description)
	suite.Equal("custom", string(sender.Provider))
	suite.Len(sender.Properties, 1)
}

// TestParseProviderType_ValidProviders tests parsing valid provider types.
func (suite *InitTestSuite) TestParseProviderType_ValidProviders() {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Twilio lowercase", "twilio", "twilio"},
		{"Twilio uppercase", "TWILIO", "twilio"},
		{"Vonage lowercase", "vonage", "vonage"},
		{"Custom lowercase", "custom", "custom"},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			provider, err := parseProviderType(tt.input)
			suite.NoError(err)
			suite.Equal(tt.expected, string(provider))
		})
	}
}

// TestParseProviderType_InvalidProvider tests parsing invalid provider type.
func (suite *InitTestSuite) TestParseProviderType_InvalidProvider() {
	provider, err := parseProviderType("invalid_provider")

	suite.Error(err)
	suite.Equal("", string(provider))
	suite.Contains(err.Error(), "unsupported provider type")
}

// TestParseToNotificationSenderDTO_VonageYAML tests parsing Vonage YAML configuration.
func (suite *InitTestSuite) TestParseToNotificationSenderDTO_VonageYAML() {
	yamlData := `
id: "vonage-sender-001"
name: "Vonage SMS Sender"
description: "Production Vonage SMS sender"
provider: "vonage"
properties:
  - name: "api_key"
    value: "{{.VONAGE_API_KEY}}"
    is_secret: false
  - name: "api_secret"
    value: "{{.VONAGE_API_SECRET}}"
    is_secret: true
  - name: "sender_id"
    value: "{{.VONAGE_FROM_NUMBER}}"
    is_secret: false
`

	sender, err := parseToNotificationSenderDTO([]byte(yamlData))

	suite.NoError(err)
	suite.NotNil(sender)
	suite.Equal("vonage-sender-001", sender.ID)
	suite.Equal("Vonage SMS Sender", sender.Name)
	suite.Equal("Production Vonage SMS sender", sender.Description)
	suite.Equal("vonage", string(sender.Provider))
	suite.Len(sender.Properties, 3)
}

// TestParseProviderType_CaseSensitivity tests parsing provider type with different cases.
func (suite *InitTestSuite) TestParseProviderType_CaseSensitivity() {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Vonage mixed case", "VoNaGe", "vonage"},
		{"Twilio mixed case", "TwIlIo", "twilio"},
		{"Custom mixed case", "CuStOm", "custom"},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			provider, err := parseProviderType(tt.input)
			suite.NoError(err)
			suite.Equal(tt.expected, string(provider))
		})
	}
}

// TestInitialize_WithDeclarativeResourcesEnabled_InvalidYAML tests Initialize with declarative resources
// enabled but with invalid YAML files
//
//nolint:dupl // Similar test setup required for different error scenarios
func (suite *InitTestSuite) TestInitialize_WithDeclarativeResourcesEnabled_InvalidYAML() {
	tmpDir := suite.T().TempDir()
	confDir := tmpDir + "/repository/resources"
	senderDir := confDir + "/notification_senders"

	err := os.MkdirAll(senderDir, 0750)
	suite.NoError(err)

	// Create an invalid YAML file
	invalidYAML := `invalid yaml content
  - this is not: valid
`
	err = os.WriteFile(filepath.Join(senderDir, "invalid-sender.yaml"), []byte(invalidYAML), 0600)
	suite.NoError(err)

	config.ResetServerRuntime()
	testConfig := &config.Config{
		JWT: config.JWTConfig{
			Issuer:         "test-issuer",
			ValidityPeriod: 3600,
		},
		Crypto: config.CryptoConfig{
			Encryption: config.EncryptionConfig{
				Key: testCryptoKey,
			},
		},
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
		DeclarativeResources: config.DeclarativeResources{
			Enabled: true,
		},
	}
	err = config.InitializeServerRuntime(tmpDir, testConfig)
	suite.NoError(err)

	mux := http.NewServeMux()

	// Initialize should return an error due to invalid YAML
	_, _, _, _, err = Initialize(mux, suite.mockJWTService, suite.mockTemplateService)
	suite.Error(err)
	suite.Contains(err.Error(), "failed to load notification sender resources")

	// Clean up
	config.ResetServerRuntime()
	suiteConfig := &config.Config{
		JWT: config.JWTConfig{
			Issuer:         "test-issuer",
			ValidityPeriod: 3600,
		},
		Crypto: config.CryptoConfig{
			Encryption: config.EncryptionConfig{
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
	err = config.InitializeServerRuntime("", suiteConfig)
	suite.NoError(err)
}

// TestInitialize_WithDeclarativeResourcesEnabled_ValidationFailure tests Initialize when
// validation fails for loaded resources
//
//nolint:dupl // Similar test setup required for different error scenarios
func (suite *InitTestSuite) TestInitialize_WithDeclarativeResourcesEnabled_ValidationFailure() {
	tmpDir := suite.T().TempDir()
	confDir := tmpDir + "/repository/resources"
	senderDir := confDir + "/notification_senders"

	err := os.MkdirAll(senderDir, 0750)
	suite.NoError(err)

	// Create a YAML file with invalid configuration (missing name)
	invalidSenderYAML := `id: "invalid-sender"
name: ""
provider: "twilio"
properties:
  - name: "account_sid"
    value: "test"
    is_secret: false
`
	err = os.WriteFile(filepath.Join(senderDir, "invalid-sender.yaml"), []byte(invalidSenderYAML), 0600)
	suite.NoError(err)

	config.ResetServerRuntime()
	testConfig := &config.Config{
		JWT: config.JWTConfig{
			Issuer:         "test-issuer",
			ValidityPeriod: 3600,
		},
		Crypto: config.CryptoConfig{
			Encryption: config.EncryptionConfig{
				Key: testCryptoKey,
			},
		},
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
		DeclarativeResources: config.DeclarativeResources{
			Enabled: true,
		},
	}
	err = config.InitializeServerRuntime(tmpDir, testConfig)
	suite.NoError(err)

	mux := http.NewServeMux()

	// Initialize should return an error due to validation failure
	_, _, _, _, err = Initialize(mux, suite.mockJWTService, suite.mockTemplateService)
	suite.Error(err)
	suite.Contains(err.Error(), "failed to load notification sender resources")

	// Clean up
	config.ResetServerRuntime()
	suiteConfig := &config.Config{
		JWT: config.JWTConfig{
			Issuer:         "test-issuer",
			ValidityPeriod: 3600,
		},
		Crypto: config.CryptoConfig{
			Encryption: config.EncryptionConfig{
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
	err = config.InitializeServerRuntime("", suiteConfig)
	suite.NoError(err)
}
