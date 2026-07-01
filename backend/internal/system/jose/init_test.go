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
	"errors"
	"testing"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	joseconfig "github.com/thunder-id/thunderid/internal/system/jose/config"
	"github.com/thunder-id/thunderid/internal/system/jose/jwe"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	kmprovider "github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	"github.com/thunder-id/thunderid/tests/mocks/crypto/cryptomock"
)

type JOSEInitTestSuite struct {
	suite.Suite
	mockRuntime    *cryptomock.RuntimeCryptoProviderMock
	testPrivateKey *rsa.PrivateKey
}

func TestJOSEInitTestSuite(t *testing.T) {
	suite.Run(t, new(JOSEInitTestSuite))
}

func (suite *JOSEInitTestSuite) SetupTest() {
	suite.mockRuntime = cryptomock.NewRuntimeCryptoProviderMock(suite.T())

	// Generate a test RSA private key
	var err error
	suite.testPrivateKey, err = rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(suite.T(), err)

	// Initialize server runtime config for testing
	testConfig := &config.Config{
		JWT: engineconfig.JWTConfig{
			PreferredKeyID: "test-key-id",
			Issuer:         "test-issuer",
			Audience:       "test-audience",
			ValidityPeriod: 3600,
			Leeway:         300,
		},
		Crypto: config.CryptoConfig{
			Encryption: engineconfig.EncryptionConfig{
				Key: "0579f866ac7c9273580d0ff163fa01a7b2401a7ff3ddc3e3b14ae3136fa6025e",
			},
		},
	}
	err = config.InitializeServerRuntime("/tmp/test", testConfig)
	assert.NoError(suite.T(), err)
}

func (suite *JOSEInitTestSuite) TearDownTest() {
	suite.mockRuntime.AssertExpectations(suite.T())
}

func (suite *JOSEInitTestSuite) TestInitialize_Success() {
	suite.mockRuntime.EXPECT().
		GetPublicKeys(mock.Anything, kmprovider.PublicKeyFilter{KeyID: "test-key-id"}).
		Return([]kmprovider.PublicKeyInfo{
			{
				KeyID:      "test-key-id",
				Algorithm:  cryptolib.AlgorithmRS256,
				PublicKey:  &suite.testPrivateKey.PublicKey,
				Thumbprint: "test-thumbprint",
			},
		}, nil)

	jwtService, jweService, err := Initialize(suite.mockRuntime, joseconfig.Config{PreferredKeyID: "test-key-id"})

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), jwtService)
	assert.NotNil(suite.T(), jweService)
	assert.Implements(suite.T(), (*jwt.JWTServiceInterface)(nil), jwtService)
	assert.Implements(suite.T(), (*jwe.JWEServiceInterface)(nil), jweService)
}

func (suite *JOSEInitTestSuite) TestInitialize_JWTInitializationFailure() {
	suite.mockRuntime.EXPECT().
		GetPublicKeys(mock.Anything, kmprovider.PublicKeyFilter{KeyID: "test-key-id"}).
		Return(nil, errors.New("provider unavailable"))

	jwtService, jweService, err := Initialize(suite.mockRuntime, joseconfig.Config{PreferredKeyID: "test-key-id"})

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), jwtService)
	assert.Nil(suite.T(), jweService)
	assert.Contains(suite.T(), err.Error(), "failed to retrieve public key")
}

func (suite *JOSEInitTestSuite) TestInitialize_NilRuntimeProvider() {
	defer func() {
		if r := recover(); r != nil {
			assert.NotNil(suite.T(), r)
		}
	}()

	jwtService, jweService, err := Initialize(nil, joseconfig.Config{PreferredKeyID: "test-key-id"})

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), jwtService)
	assert.Nil(suite.T(), jweService)
}

func (suite *JOSEInitTestSuite) TestInitialize_ValidatesServiceInterfaces() {
	suite.mockRuntime.EXPECT().
		GetPublicKeys(mock.Anything, kmprovider.PublicKeyFilter{KeyID: "test-key-id"}).
		Return([]kmprovider.PublicKeyInfo{
			{
				KeyID:      "test-key-id",
				Algorithm:  cryptolib.AlgorithmRS256,
				PublicKey:  &suite.testPrivateKey.PublicKey,
				Thumbprint: "test-thumbprint",
			},
		}, nil)

	jwtService, jweService, err := Initialize(suite.mockRuntime, joseconfig.Config{PreferredKeyID: "test-key-id"})

	assert.NoError(suite.T(), err)
	if jwtService != nil {
		assert.Implements(suite.T(), (*jwt.JWTServiceInterface)(nil), jwtService)
	}
	if jweService != nil {
		assert.Implements(suite.T(), (*jwe.JWEServiceInterface)(nil), jweService)
	}
}
