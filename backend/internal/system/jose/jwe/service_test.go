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

package jwe

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	joseconfig "github.com/thunder-id/thunderid/internal/system/jose/config"
	kmprovider "github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/crypto/cryptomock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type JWEServiceTestSuite struct {
	suite.Suite
	jweService        *jweService
	testRSAPrivateKey *rsa.PrivateKey
	testECPrivateKey  *ecdsa.PrivateKey
}

func TestJWEServiceSuite(t *testing.T) {
	suite.Run(t, new(JWEServiceTestSuite))
}

func (suite *JWEServiceTestSuite) SetupTest() {
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	suite.testRSAPrivateKey = rsaKey

	ecKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	suite.testECPrivateKey = ecKey
}

func (suite *JWEServiceTestSuite) TestEncryptDecrypt_RSA() {
	encAlgs := []ContentEncAlgorithm{A128GCM, A192GCM, A256GCM}

	mockProvider := cryptomock.NewRuntimeCryptoProviderMock(suite.T())
	mockProvider.EXPECT().Decrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(
			ctx context.Context, keyRef *kmprovider.KeyRef,
			params cryptolib.AlgorithmParams, content []byte,
		) ([]byte, error) {
			return cryptolib.Decrypt(suite.testRSAPrivateKey, params, content)
		}).Times(len(encAlgs))

	suite.jweService = &jweService{
		cryptoProvider: mockProvider,
		keyRef:         kmprovider.KeyRef{KeyID: "test-kid"},
		logger:         log.GetLogger(),
	}

	payload := []byte("Hello, RSA JWE!")
	recipientPublicKey := &suite.testRSAPrivateKey.PublicKey

	for _, enc := range encAlgs {
		jweToken, sErr := suite.jweService.Encrypt(
			context.Background(),
			payload,
			recipientPublicKey,
			RSAOAEP256,
			enc,
			"",
			"")
		assert.Nil(suite.T(), sErr)
		decrypted, sErr := suite.jweService.Decrypt(context.Background(), jweToken)
		assert.Nil(suite.T(), sErr)
		assert.Equal(suite.T(), payload, decrypted)
	}
}

func (suite *JWEServiceTestSuite) TestEncryptDecrypt_ECDH() {
	testCases := []struct {
		alg KeyEncAlgorithm
		enc ContentEncAlgorithm
	}{
		{ECDHES, A128GCM},
		{ECDHES, A192GCM},
		{ECDHES, A256GCM},
		{ECDHESA128KW, A128GCM},
		{ECDHESA256KW, A256GCM},
	}

	mockProvider := cryptomock.NewRuntimeCryptoProviderMock(suite.T())
	mockProvider.EXPECT().Decrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(
			ctx context.Context, keyRef *kmprovider.KeyRef,
			params cryptolib.AlgorithmParams, content []byte,
		) ([]byte, error) {
			return cryptolib.Decrypt(suite.testECPrivateKey, params, content)
		}).Times(len(testCases))

	suite.jweService = &jweService{
		cryptoProvider: mockProvider,
		keyRef:         kmprovider.KeyRef{KeyID: "test-kid"},
		logger:         log.GetLogger(),
	}

	payload := []byte("Hello, ECDH JWE!")
	recipientPublicKey := &suite.testECPrivateKey.PublicKey

	for _, tc := range testCases {
		jweToken, sErr := suite.jweService.Encrypt(
			context.Background(),
			payload,
			recipientPublicKey,
			tc.alg,
			tc.enc,
			"",
			"")
		assert.Nil(suite.T(), sErr)
		decrypted, sErr := suite.jweService.Decrypt(context.Background(), jweToken)
		assert.Nil(suite.T(), sErr)
		assert.Equal(suite.T(), payload, decrypted)
	}
}

func (suite *JWEServiceTestSuite) TestEncrypt_Errors() {
	suite.jweService = &jweService{
		logger: log.GetLogger(),
	}

	// Unsupported Encryption algorithm
	_, sErr := suite.jweService.Encrypt(
		context.Background(),
		[]byte("p"),
		&suite.testRSAPrivateKey.PublicKey,
		RSAOAEP256,
		"INVALID",
		"",
		"")
	assert.NotNil(suite.T(), sErr)
	assert.Equal(suite.T(), ErrorUnsupportedEncryptionAlgorithm, *sErr)

	// EncryptKey failure (RSA with EC key)
	_, sErr = suite.jweService.Encrypt(
		context.Background(),
		[]byte("p"),
		&suite.testECPrivateKey.PublicKey,
		RSAOAEP256,
		A128GCM,
		"",
		"")
	assert.NotNil(suite.T(), sErr)
	assert.Equal(suite.T(), ErrorUnsupportedJWEAlgorithm, *sErr)
}

func (suite *JWEServiceTestSuite) TestDecrypt_Errors() {
	mockProvider := cryptomock.NewRuntimeCryptoProviderMock(suite.T())
	suite.jweService = &jweService{
		cryptoProvider: mockProvider,
		keyRef:         kmprovider.KeyRef{KeyID: "test-kid"},
		logger:         log.GetLogger(),
	}

	// Invalid JWE format — no provider call
	_, sErr := suite.jweService.Decrypt(context.Background(), "invalid.jwe")
	assert.NotNil(suite.T(), sErr)
	assert.Equal(suite.T(), ErrorDecodingJWE, *sErr)

	// Encrypt a valid token (Encrypt does not use the provider)
	payload := []byte("data")
	jweToken, _ := suite.jweService.Encrypt(
		context.Background(),
		payload,
		&suite.testRSAPrivateKey.PublicKey,
		RSAOAEP256,
		A128GCM,
		"",
		"")

	// DecryptKey failure: provider returns an error
	mockProvider.EXPECT().Decrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("key decryption error")).Once()
	_, sErr = suite.jweService.Decrypt(context.Background(), jweToken)
	assert.NotNil(suite.T(), sErr)
	assert.Equal(suite.T(), ErrorJWEDecryptionFailed, *sErr)

	// DecryptContent failure (tampered tag): provider returns correct CEK but tag is wrong
	mockProvider.EXPECT().Decrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(
			ctx context.Context, keyRef *kmprovider.KeyRef,
			params cryptolib.AlgorithmParams, content []byte,
		) ([]byte, error) {
			return cryptolib.Decrypt(suite.testRSAPrivateKey, params, content)
		}).Once()
	parts := strings.Split(jweToken, ".")
	parts[4] = base64.RawURLEncoding.EncodeToString([]byte("wrong-tag"))
	tamperedToken := strings.Join(parts, ".")
	_, sErr = suite.jweService.Decrypt(context.Background(), tamperedToken)
	assert.NotNil(suite.T(), sErr)
	assert.Equal(suite.T(), ErrorJWEDecryptionFailed, *sErr)
}

func (suite *JWEServiceTestSuite) TestInitialize() {
	mockProvider := cryptomock.NewRuntimeCryptoProviderMock(suite.T())

	cfg := joseconfig.Config{PreferredKeyID: "test-kid"}
	service, err := Initialize(mockProvider, cfg)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), service)
}

func (suite *JWEServiceTestSuite) TestEncrypt_ErrorCases() {
	suite.jweService = &jweService{
		logger: log.GetLogger(),
	}

	payload := []byte("test data")

	// Nil recipient key
	_, sErr := suite.jweService.Encrypt(context.Background(), payload, nil, RSAOAEP256, A128GCM, "", "")
	assert.NotNil(suite.T(), sErr)

	// Unsupported key type
	fakeKey := "not-a-real-key"
	_, sErr = suite.jweService.Encrypt(context.Background(), payload, fakeKey, RSAOAEP256, A128GCM, "", "")
	assert.NotNil(suite.T(), sErr)
}

func (suite *JWEServiceTestSuite) TestDecrypt_EdgeCases() {
	suite.jweService = &jweService{
		logger: log.GetLogger(),
	}

	// Test with malformed JWE (wrong number of parts)
	_, sErr := suite.jweService.Decrypt(context.Background(), "malformed.jwe")
	assert.NotNil(suite.T(), sErr)
	assert.Equal(suite.T(), ErrorDecodingJWE, *sErr)

	// Test with invalid base64 in header
	_, sErr = suite.jweService.Decrypt(context.Background(), "invalid-base64.key.iv.ciphertext.tag")
	assert.NotNil(suite.T(), sErr)
	assert.Equal(suite.T(), ErrorDecodingJWE, *sErr)

	// Test with invalid JSON in header
	invalidHeader := base64.RawURLEncoding.EncodeToString([]byte("{invalid json"))
	_, sErr = suite.jweService.Decrypt(context.Background(), invalidHeader+".key.iv.ciphertext.tag")
	assert.NotNil(suite.T(), sErr)
	assert.Equal(suite.T(), ErrorDecodingJWE, *sErr)

	// Test with missing required header fields
	headerMissingAlg := base64.RawURLEncoding.EncodeToString([]byte(`{"enc":"A128GCM"}`))
	_, sErr = suite.jweService.Decrypt(context.Background(), headerMissingAlg+".key.iv.ciphertext.tag")
	assert.NotNil(suite.T(), sErr)
	assert.Equal(suite.T(), ErrorUnsupportedJWEAlgorithm, *sErr)
}

func (suite *JWEServiceTestSuite) TestIsSupportedEnc() {
	valid := []ContentEncAlgorithm{A128GCM, A192GCM, A256GCM, A128CBCHS256, A192CBCHS384, A256CBCHS512}
	for _, enc := range valid {
		assert.True(suite.T(), isSupportedEnc(enc), "expected %s to be supported", enc)
	}
	assert.False(suite.T(), isSupportedEnc("INVALID"))
	assert.False(suite.T(), isSupportedEnc(""))
}

func (suite *JWEServiceTestSuite) TestBuildEncryptParams() {
	testCases := []struct {
		alg         KeyEncAlgorithm
		expectError bool
	}{
		{RSAOAEP, false},
		{RSAOAEP256, false},
		{ECDHES, false},
		{ECDHESA128KW, false},
		{ECDHESA192KW, false},
		{ECDHESA256KW, false},
		{A128KW, false},
		{A192KW, false},
		{A256KW, false},
		{A128GCMKW, false},
		{A192GCMKW, false},
		{A256GCMKW, false},
		{"UNSUPPORTED", true},
	}

	for _, tc := range testCases {
		suite.T().Run(string(tc.alg), func(t *testing.T) {
			params, err := buildEncryptParams(tc.alg, A128GCM)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, params.Algorithm)
			}
		})
	}
}

func (suite *JWEServiceTestSuite) TestBuildDecryptParams() {
	// RSAOAEP — no EPK needed
	header := map[string]interface{}{"alg": "RSA-OAEP", "enc": "A128GCM"}
	params, err := buildDecryptParams(RSAOAEP, A128GCM, header)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), cryptolib.AlgorithmRSAOAEP, params.Algorithm)

	// ECDHESA192KW — needs a valid EPK in the header
	privKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	ecdhPub, _ := privKey.PublicKey.ECDH()
	epkMap, _ := epkToMap(ecdhPub)
	headerWithEPK := map[string]interface{}{
		"alg": "ECDH-ES+A192KW",
		"enc": "A256GCM",
		"epk": epkMap,
	}
	params, err = buildDecryptParams(ECDHESA192KW, A256GCM, headerWithEPK)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), cryptolib.AlgorithmECDHESA192KW, params.Algorithm)

	// Missing EPK for all ECDH-ES variants
	headerNoEPK := map[string]interface{}{"alg": "ECDH-ES", "enc": "A128GCM"}
	for _, alg := range []KeyEncAlgorithm{ECDHES, ECDHESA128KW, ECDHESA192KW, ECDHESA256KW} {
		_, err = buildDecryptParams(alg, A128GCM, headerNoEPK)
		assert.Error(suite.T(), err, "expected error for %s with missing EPK", alg)
	}

	// Unsupported algorithm (default branch)
	_, err = buildDecryptParams("A128KW", A128GCM, header)
	assert.Error(suite.T(), err)
}

func (suite *JWEServiceTestSuite) TestEncrypt_WithKidAndCty() {
	suite.jweService = &jweService{
		cryptoProvider: nil,
		keyRef:         kmprovider.KeyRef{KeyID: "test-kid"},
		logger:         log.GetLogger(),
	}

	token, sErr := suite.jweService.Encrypt(context.Background(),
		[]byte("payload"), &suite.testRSAPrivateKey.PublicKey, RSAOAEP256, A128GCM, "JWT", "my-kid")
	assert.Nil(suite.T(), sErr)

	parsedHeader, _, _, _, _, _, err := DecodeJWE(token)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "my-kid", parsedHeader["kid"])
	assert.Equal(suite.T(), "JWT", parsedHeader["cty"])
}

func (suite *JWEServiceTestSuite) TestDecrypt_MissingEncField() {
	suite.jweService = &jweService{
		logger: log.GetLogger(),
	}

	headerNoEnc := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RSA-OAEP-256"}`))
	_, sErr := suite.jweService.Decrypt(context.Background(), headerNoEnc+".key.iv.ciphertext.tag")
	assert.NotNil(suite.T(), sErr)
	assert.Equal(suite.T(), ErrorUnsupportedEncryptionAlgorithm, *sErr)
}

func (suite *JWEServiceTestSuite) TestDecrypt_UnsupportedAlgorithmForDecrypt() {
	suite.jweService = &jweService{
		logger: log.GetLogger(),
	}

	// A128KW is valid for encrypt but hits the default branch in buildDecryptParams
	headerAESKW := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"A128KW","enc":"A128GCM"}`))
	_, sErr := suite.jweService.Decrypt(context.Background(), headerAESKW+".key.iv.ciphertext.tag")
	assert.NotNil(suite.T(), sErr)
	assert.Equal(suite.T(), ErrorUnsupportedJWEAlgorithm, *sErr)
}

func (suite *JWEServiceTestSuite) TestEncryptDecrypt_CBC() {
	encAlgs := []ContentEncAlgorithm{A128CBCHS256, A192CBCHS384, A256CBCHS512}

	mockProvider := cryptomock.NewRuntimeCryptoProviderMock(suite.T())
	mockProvider.EXPECT().Decrypt(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(
			ctx context.Context, keyRef *kmprovider.KeyRef,
			params cryptolib.AlgorithmParams, content []byte,
		) ([]byte, error) {
			return cryptolib.Decrypt(suite.testRSAPrivateKey, params, content)
		}).Times(len(encAlgs))

	suite.jweService = &jweService{
		cryptoProvider: mockProvider,
		keyRef:         kmprovider.KeyRef{KeyID: "test-kid"},
		logger:         log.GetLogger(),
	}

	payload := []byte("Hello, CBC JWE!")
	for _, enc := range encAlgs {
		jweToken, sErr := suite.jweService.Encrypt(
			context.Background(),
			payload,
			&suite.testRSAPrivateKey.PublicKey,
			RSAOAEP256,
			enc,
			"",
			"")
		assert.Nil(suite.T(), sErr, "enc=%s", enc)
		decrypted, sErr := suite.jweService.Decrypt(context.Background(), jweToken)
		assert.Nil(suite.T(), sErr, "enc=%s", enc)
		assert.Equal(suite.T(), payload, decrypted, "enc=%s", enc)
	}
}
