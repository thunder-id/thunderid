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

package jwt

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/tests/mocks/crypto/pki/pkimock"
)

type InitTestSuite struct {
	suite.Suite
	testKeyPath    string
	testPrivateKey *rsa.PrivateKey
	tempFiles      []string
}

func TestInitSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (suite *InitTestSuite) SetupSuite() {
	// Generate a test RSA private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(suite.T(), err)
	suite.testPrivateKey = privateKey

	// Create a temporary private key file
	tempFile, err := os.CreateTemp("", "test_init_key_*.pem")
	assert.NoError(suite.T(), err)
	suite.testKeyPath = tempFile.Name()

	// Encode the private key to PEM
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	// Write to file
	_, err = tempFile.Write(privateKeyPEM)
	assert.NoError(suite.T(), err)
	err = tempFile.Close()
	assert.NoError(suite.T(), err)
}

func (suite *InitTestSuite) TearDownSuite() {
	// Clean up any temporary files created during tests
	for _, file := range suite.tempFiles {
		err := os.Remove(file)
		if err != nil {
			suite.T().Logf("Failed to remove temp file %s: %v", file, err)
		}
	}
	if suite.testKeyPath != "" {
		err := os.Remove(suite.testKeyPath)
		if err != nil {
			suite.T().Logf("Failed to remove test key file %s: %v", suite.testKeyPath, err)
		}
	}
}

func (suite *InitTestSuite) SetupTest() {
	// Reset server runtime before each test
	config.ResetServerRuntime()
}

func (suite *InitTestSuite) TestInitialize_Success() {
	// Setup test configuration
	testConfig := &config.Config{
		JWT: config.JWTConfig{
			Issuer:         "https://auth.example.com",
			ValidityPeriod: 3600,
			PreferredKeyID: "test-kid",
		},
		Crypto: config.CryptoConfig{
			Keys: []config.KeyConfig{
				{
					ID:       "test-kid",
					CertFile: suite.testKeyPath,
					KeyFile:  suite.testKeyPath,
				},
			},
		},
	}
	err := config.InitializeServerRuntime("", testConfig)
	assert.NoError(suite.T(), err)

	// Create mock PKI service
	pkiMock := pkimock.NewPKIServiceInterfaceMock(suite.T())
	pkiMock.EXPECT().GetPrivateKey(mock.Anything).Return(suite.testPrivateKey, nil)
	pkiMock.EXPECT().GetCertThumbprint(mock.Anything).Return("test-kid")

	// Initialize JWT service
	jwtService, err := Initialize(pkiMock)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), jwtService)
	assert.Implements(suite.T(), (*JWTServiceInterface)(nil), jwtService)
}

func (suite *InitTestSuite) TestInitialize_PrivateKeyRetrievalError() {
	// Setup test configuration
	testConfig := &config.Config{
		JWT: config.JWTConfig{
			Issuer:         "https://auth.example.com",
			ValidityPeriod: 3600,
			PreferredKeyID: "test-kid",
		},
		Crypto: config.CryptoConfig{
			Keys: []config.KeyConfig{
				{
					ID:       "test-kid",
					CertFile: suite.testKeyPath,
					KeyFile:  suite.testKeyPath,
				},
			},
		},
	}
	err := config.InitializeServerRuntime("", testConfig)
	assert.NoError(suite.T(), err)

	pkiMock := pkimock.NewPKIServiceInterfaceMock(suite.T())
	testErr := serviceerror.CustomServiceError(serviceerror.InternalServerError, core.I18nMessage{
		Key:          "error.test.jwt_init",
		DefaultValue: "test error",
	})
	pkiMock.EXPECT().GetPrivateKey(mock.Anything).Return(nil, testErr)

	// Initialize JWT service should fail
	jwtService, err := Initialize(pkiMock)
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), jwtService)
}

func (suite *InitTestSuite) TestInitialize_WithoutPreferredKeyID() {
	// Setup test configuration without PreferredKeyID
	testConfig := &config.Config{
		JWT: config.JWTConfig{
			Issuer:         "https://auth.example.com",
			ValidityPeriod: 3600,
			// PreferredKeyID is empty
		},
		Crypto: config.CryptoConfig{
			Keys: []config.KeyConfig{
				{
					ID:       "",
					CertFile: suite.testKeyPath,
					KeyFile:  suite.testKeyPath,
				},
			},
		},
	}
	err := config.InitializeServerRuntime("", testConfig)
	assert.NoError(suite.T(), err)

	// Create mock PKI service
	pkiMock := pkimock.NewPKIServiceInterfaceMock(suite.T())
	pkiMock.EXPECT().GetPrivateKey("").Return(suite.testPrivateKey, nil)
	pkiMock.EXPECT().GetCertThumbprint("").Return("test-kid")

	// Initialize JWT service
	jwtService, err := Initialize(pkiMock)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), jwtService)
}
