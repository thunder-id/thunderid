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

package jose

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	i18ncore "github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/jose/jwe"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/tests/mocks/crypto/pki/pkimock"
)

type JOSEInitTestSuite struct {
	suite.Suite
	mockPKIService *pkimock.PKIServiceInterfaceMock
	testPrivateKey *rsa.PrivateKey
}

func TestJOSEInitTestSuite(t *testing.T) {
	suite.Run(t, new(JOSEInitTestSuite))
}

func (suite *JOSEInitTestSuite) SetupTest() {
	suite.mockPKIService = &pkimock.PKIServiceInterfaceMock{}

	// Generate a test RSA private key
	var err error
	suite.testPrivateKey, err = rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(suite.T(), err)

	// Initialize server runtime config for testing
	testConfig := &config.Config{
		JWT: config.JWTConfig{
			PreferredKeyID: "test-key-id",
			Issuer:         "test-issuer",
			Audience:       "test-audience",
			ValidityPeriod: 3600,
			Leeway:         300,
		},
		Crypto: config.CryptoConfig{
			Encryption: config.EncryptionConfig{
				Key: "0579f866ac7c9273580d0ff163fa01a7b2401a7ff3ddc3e3b14ae3136fa6025e",
			},
		},
	}
	err = config.InitializeServerRuntime("/tmp/test", testConfig)
	assert.NoError(suite.T(), err)
}

func (suite *JOSEInitTestSuite) TearDownTest() {
	suite.mockPKIService.AssertExpectations(suite.T())
}

func (suite *JOSEInitTestSuite) TestInitialize_Success() {
	// Setup mock expectations for successful initialization
	// Both JWT and JWE services will try to get private key and cert thumbprint with "test-key-id"
	suite.mockPKIService.On("GetPrivateKey", "test-key-id").Return(suite.testPrivateKey, nil).Twice()
	suite.mockPKIService.On("GetCertThumbprint", "test-key-id").Return("test-thumbprint").Twice()

	jwtService, jweService, err := Initialize(suite.mockPKIService)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), jwtService)
	assert.NotNil(suite.T(), jweService)

	// Verify the returned services implement the correct interfaces
	assert.Implements(suite.T(), (*jwt.JWTServiceInterface)(nil), jwtService)
	assert.Implements(suite.T(), (*jwe.JWEServiceInterface)(nil), jweService)
}

func (suite *JOSEInitTestSuite) TestInitialize_JWTInitializationFailure() {
	// Test case where JWT initialization fails due to PKI service error
	expectedErr := &serviceerror.ServiceError{
		Code:             "PKI-001",
		Type:             serviceerror.ServerErrorType,
		Error:            i18ncore.I18nMessage{DefaultValue: "private key not found"},
		ErrorDescription: i18ncore.I18nMessage{DefaultValue: "The requested private key could not be found"},
	}
	suite.mockPKIService.On("GetPrivateKey", "test-key-id").Return(nil, expectedErr).Once()

	jwtService, jweService, err := Initialize(suite.mockPKIService)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), jwtService)
	assert.Nil(suite.T(), jweService)
	assert.Contains(suite.T(), err.Error(), "failed to retrieve private key")
}

func (suite *JOSEInitTestSuite) TestInitialize_JWEInitializationFailure() {
	// Test case where JWT succeeds but JWE fails
	// JWT will succeed with first call
	suite.mockPKIService.On("GetPrivateKey", "test-key-id").Return(suite.testPrivateKey, nil).Once()
	suite.mockPKIService.On("GetCertThumbprint", "test-key-id").Return("test-thumbprint").Once()

	// JWE will fail with second call
	expectedErr := &serviceerror.ServiceError{
		Code:             "PKI-002",
		Type:             serviceerror.ServerErrorType,
		Error:            i18ncore.I18nMessage{DefaultValue: "certificate error"},
		ErrorDescription: i18ncore.I18nMessage{DefaultValue: "The certificate could not be processed"},
	}
	suite.mockPKIService.On("GetPrivateKey", "test-key-id").Return(nil, expectedErr).Once()

	jwtService, jweService, err := Initialize(suite.mockPKIService)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), jwtService)
	assert.Nil(suite.T(), jweService)
	assert.Contains(suite.T(), err.Error(), "failed to retrieve private key")
}

func (suite *JOSEInitTestSuite) TestInitialize_NilPKIService() {
	// Test with nil PKI service - should panic or return error
	defer func() {
		if r := recover(); r != nil {
			// Panic is expected with nil service
			assert.NotNil(suite.T(), r)
		}
	}()

	jwtService, jweService, err := Initialize(nil)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), jwtService)
	assert.Nil(suite.T(), jweService)
}

func (suite *JOSEInitTestSuite) TestInitialize_PKIServiceGetPrivateKeyError() {
	// Test specific error scenarios from PKI service
	testCases := []struct {
		name        string
		pkiError    *serviceerror.ServiceError
		expectError string
	}{
		{
			name: "KeyNotFound",
			pkiError: &serviceerror.ServiceError{
				Code:             "PKI-003",
				Type:             serviceerror.ServerErrorType,
				Error:            i18ncore.I18nMessage{DefaultValue: "key not found"},
				ErrorDescription: i18ncore.I18nMessage{DefaultValue: "The requested key was not found"},
			},
			expectError: "failed to retrieve private key",
		},
		{
			name: "InvalidKey",
			pkiError: &serviceerror.ServiceError{
				Code:             "PKI-004",
				Type:             serviceerror.ServerErrorType,
				Error:            i18ncore.I18nMessage{DefaultValue: "invalid key format"},
				ErrorDescription: i18ncore.I18nMessage{DefaultValue: "The key format is invalid"},
			},
			expectError: "failed to retrieve private key",
		},
		{
			name: "AccessDenied",
			pkiError: &serviceerror.ServiceError{
				Code:             "PKI-005",
				Type:             serviceerror.ServerErrorType,
				Error:            i18ncore.I18nMessage{DefaultValue: "access denied"},
				ErrorDescription: i18ncore.I18nMessage{DefaultValue: "Access to the key was denied"},
			},
			expectError: "failed to retrieve private key",
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			mockPKI := &pkimock.PKIServiceInterfaceMock{}
			mockPKI.On("GetPrivateKey", "test-key-id").Return(nil, tc.pkiError).Once()

			jwtService, jweService, err := Initialize(mockPKI)

			assert.Error(t, err)
			assert.Nil(t, jwtService)
			assert.Nil(t, jweService)
			assert.Contains(t, err.Error(), tc.expectError)

			mockPKI.AssertExpectations(t)
		})
	}
}

func (suite *JOSEInitTestSuite) TestInitialize_ValidatesServiceInterfaces() {
	// Test that Initialize returns valid service interfaces
	suite.mockPKIService.On("GetPrivateKey", "test-key-id").Return(suite.testPrivateKey, nil).Twice()
	suite.mockPKIService.On("GetCertThumbprint", "test-key-id").Return("test-thumbprint").Twice()

	jwtService, jweService, err := Initialize(suite.mockPKIService)

	assert.NoError(suite.T(), err)

	// Ensure services are not just non-nil but actually implement expected interfaces
	if jwtService != nil {
		assert.Implements(suite.T(), (*jwt.JWTServiceInterface)(nil), jwtService)
	}

	if jweService != nil {
		assert.Implements(suite.T(), (*jwe.JWEServiceInterface)(nil), jweService)
	}
}
