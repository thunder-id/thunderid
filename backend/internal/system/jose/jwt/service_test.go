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

package jwt

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/cryptolab"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	httpservice "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/jose/jws"
	"github.com/thunder-id/thunderid/internal/system/kmprovider"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/crypto/cryptomock"
	"github.com/thunder-id/thunderid/tests/mocks/crypto/pki/pkimock"
)

const (
	testAudience     = "test-audience"
	testIssuer       = "test-issuer"
	testAud          = "test-aud"
	testIss          = "test-iss"
	wrongAudience    = "wrong-audience"
	wrongIssuer      = "wrong-issuer"
	expectedAudience = "expected-audience"
	expectedIssuer   = "expected-issuer"
)

type JWTServiceTestSuite struct {
	suite.Suite
	jwtService     *jwtService
	testPrivateKey *rsa.PrivateKey
	testKeyPath    string
	tempFiles      []string
	pkiMock        *pkimock.PKIServiceInterfaceMock
}

func TestJWTServiceSuite(t *testing.T) {
	suite.Run(t, new(JWTServiceTestSuite))
}

func generateTestEncryptionKey(t *testing.T) string {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	return hex.EncodeToString(key)
}

func (suite *JWTServiceTestSuite) SetupSuite() {
	// Generate a test RSA private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(suite.T(), err)
	suite.testPrivateKey = privateKey

	// Create a temporary private key file
	tempFile, err := os.CreateTemp("", "test_key_*.pem")
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

func (suite *JWTServiceTestSuite) TearDownSuite() {
	err := os.Remove(suite.testKeyPath)
	assert.NoError(suite.T(), err)
}

func (suite *JWTServiceTestSuite) AfterTest(_, _ string) {
	// Clean up any temporary files created during tests
	for _, file := range suite.tempFiles {
		err := os.Remove(file)
		if err != nil {
			suite.T().Logf("Failed to remove temp file %s: %v", file, err)
		}
	}
	suite.tempFiles = nil
}

func (suite *JWTServiceTestSuite) SetupTest() {
	// Reset server runtime before each test
	config.ResetServerRuntime()

	// Create PKI mock
	suite.pkiMock = pkimock.NewPKIServiceInterfaceMock(suite.T())

	cryptoMock := cryptomock.NewRuntimeCryptoProviderMock(suite.T())
	cryptoMock.EXPECT().
		Sign(mock.Anything, kmprovider.KeyRef{KeyID: "test-kid"}, cryptolab.RSASHA256, mock.Anything).
		RunAndReturn(func(
			_ context.Context, _ kmprovider.KeyRef, _ cryptolab.SignAlgorithm, content []byte,
		) ([]byte, error) {
			return cryptolab.Generate(content, cryptolab.RSASHA256, suite.testPrivateKey)
		}).Maybe()

	suite.jwtService = &jwtService{
		cryptoProvider: cryptoMock,
		keyRef:         kmprovider.KeyRef{KeyID: "test-kid"},
		publicKey:      &suite.testPrivateKey.PublicKey,
		signAlg:        cryptolab.RSASHA256,
		jwsAlg:         jws.RS256,
		kid:            "test-kid",
		logger:         log.GetLogger().With(log.String(log.LoggerKeyComponentName, "JWTService")),
	}

	testConfig := &config.Config{
		TLS: config.TLSConfig{
			KeyFile: suite.testKeyPath,
		},
		JWT: config.JWTConfig{
			Issuer:         "https://auth.example.com",
			ValidityPeriod: 3600, // Default validity period
			PreferredKeyID: "test-kid",
			Leeway:         30, // 30 seconds leeway for clock skew
		},
		Crypto: config.CryptoConfig{
			Encryption: config.EncryptionConfig{
				Key: generateTestEncryptionKey(suite.T()),
			},
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
	suite.jwtService.httpClient = httpservice.NewHTTPClientWithTimeout(10 * time.Second)
}

func (suite *JWTServiceTestSuite) TestNewJWTService() {
	// Set expectations for PKI interactions
	suite.pkiMock.EXPECT().GetPrivateKey(mock.Anything).Return(suite.testPrivateKey, nil)
	suite.pkiMock.EXPECT().GetCertThumbprint(mock.Anything).Return("test-kid")

	service, err := Initialize(suite.pkiMock)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), service)
	assert.Implements(suite.T(), (*JWTServiceInterface)(nil), service)
}

func (suite *JWTServiceTestSuite) TestInitScenarios() {
	testCases := []struct {
		name           string
		setupFunc      func() (string, *rsa.PrivateKey)
		expectSuccess  bool
		expectedErrMsg string
	}{
		{
			name: "Success",
			setupFunc: func() (string, *rsa.PrivateKey) {
				return suite.testKeyPath, suite.testPrivateKey // Use the existing valid key path
			},
			expectSuccess:  true,
			expectedErrMsg: "",
		},
		{
			name: "PKCS8Key",
			setupFunc: func() (string, *rsa.PrivateKey) {
				privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
				assert.NoError(suite.T(), err)

				pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
				assert.NoError(suite.T(), err)

				pkcs8KeyPEM := pem.EncodeToMemory(&pem.Block{
					Type:  "PRIVATE KEY", // This is the PKCS8 standard header
					Bytes: pkcs8Bytes,
				})

				tempFile, err := os.CreateTemp("", "pkcs8_key_*.pem")
				assert.NoError(suite.T(), err)
				suite.tempFiles = append(suite.tempFiles, tempFile.Name())

				_, err = tempFile.Write(pkcs8KeyPEM)
				assert.NoError(suite.T(), err)
				err = tempFile.Close()
				assert.NoError(suite.T(), err)

				return tempFile.Name(), privateKey
			},
			expectSuccess:  true,
			expectedErrMsg: "",
		},
		{
			name: "PrivateKeyRetrievalError",
			setupFunc: func() (string, *rsa.PrivateKey) {
				return suite.testKeyPath, suite.testPrivateKey
			},
			expectSuccess:  false,
			expectedErrMsg: "failed to retrieve private key",
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			_, privateKey := tc.setupFunc()

			// Create a new mock for each test case
			pkiMock := pkimock.NewPKIServiceInterfaceMock(t)

			if tc.name == "PrivateKeyRetrievalError" {
				testErr := serviceerror.CustomServiceError(serviceerror.InternalServerError, core.I18nMessage{
					Key:          "error.test.jwt_private_key_retrieval",
					DefaultValue: "test error",
				})
				pkiMock.EXPECT().GetPrivateKey(mock.Anything).Return(nil, testErr)
			} else {
				pkiMock.EXPECT().GetPrivateKey(mock.Anything).Return(privateKey, nil)
				pkiMock.EXPECT().GetCertThumbprint(mock.Anything).Return("test-kid")
			}

			service, err := Initialize(pkiMock)

			if tc.expectSuccess {
				assert.NoError(t, err)
				assert.NotNil(t, service)
			} else {
				assert.Error(t, err)
				if tc.expectedErrMsg != "" {
					assert.Contains(t, err.Error(), tc.expectedErrMsg)
				}
			}
		})
	}
}

func (suite *JWTServiceTestSuite) TestGenerateJWTScenarios() {
	testCases := []struct {
		name               string
		sub                string
		iss                string
		validity           int64
		claims             map[string]interface{}
		setupMock          func() func() // Returns cleanup function
		setupService       func() *jwtService
		expectError        bool
		errorCode          string
		validateSuccess    func(t *testing.T, token string, iat int64)
		useDefaultValidity bool
	}{
		{
			name:     "AudAsString",
			sub:      "test-subject",
			iss:      testIssuer,
			validity: 3600,
			claims: map[string]interface{}{
				"aud":   testAudience,
				"name":  "John Doe",
				"email": "john@example.com",
			},
			setupMock: func() func() {
				return func() {}
			},
			setupService: func() *jwtService {
				return suite.jwtService
			},
			expectError: false,
			validateSuccess: func(t *testing.T, token string, iat int64) {
				parts := strings.Split(token, ".")
				assert.Len(t, parts, 3)

				headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
				assert.NoError(t, err)

				var header map[string]string
				err = json.Unmarshal(headerBytes, &header)
				assert.NoError(t, err)

				assert.Equal(t, "RS256", header["alg"])
				assert.Equal(t, "JWT", header["typ"])
				assert.Equal(t, "test-kid", header["kid"])

				payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
				assert.NoError(t, err)

				var payload map[string]interface{}
				err = json.Unmarshal(payloadBytes, &payload)
				assert.NoError(t, err)

				assert.Equal(t, "test-subject", payload["sub"])
				assert.Equal(t, testAudience, payload["aud"])
				assert.Equal(t, testIssuer, payload["iss"])
				assert.NotEmpty(t, payload["jti"])

				// Check claims
				assert.Equal(t, "John Doe", payload["name"])
				assert.Equal(t, "john@example.com", payload["email"])

				assert.True(t, payload["exp"].(float64) > float64(time.Now().Unix()))
				assert.True(t, payload["exp"].(float64) <= float64(time.Now().Unix()+3600+5))
			},
		},
		{
			name:     "AudAsSlice",
			sub:      "test-subject",
			iss:      testIssuer,
			validity: 3600,
			claims: map[string]interface{}{
				"aud": []string{testAudience, "second-audience"},
			},
			setupMock:    func() func() { return func() {} },
			setupService: func() *jwtService { return suite.jwtService },
			expectError:  false,
			validateSuccess: func(t *testing.T, token string, iat int64) {
				parts := strings.Split(token, ".")
				payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
				assert.NoError(t, err)

				var payload map[string]interface{}
				err = json.Unmarshal(payloadBytes, &payload)
				assert.NoError(t, err)

				auds, ok := payload["aud"].([]interface{})
				assert.True(t, ok)
				assert.Len(t, auds, 2)
				assert.Equal(t, testAudience, auds[0])
				assert.Equal(t, "second-audience", auds[1])
			},
		},
		{
			name:     "MissingAud",
			sub:      "test-subject",
			iss:      testIssuer,
			validity: 3600,
			claims:   map[string]interface{}{"name": "no-aud"},
			setupMock: func() func() {
				return func() {}
			},
			setupService: func() *jwtService {
				return suite.jwtService
			},
			expectError: true,
			errorCode:   "SSE-5000",
		},
		{
			name:     "WrongTypeAud",
			sub:      "test-subject",
			iss:      testIssuer,
			validity: 3600,
			claims:   map[string]interface{}{"aud": 12345},
			setupMock: func() func() {
				return func() {}
			},
			setupService: func() *jwtService {
				return suite.jwtService
			},
			expectError: true,
			errorCode:   "SSE-5000",
		},
		{
			name:     "DefaultValidity",
			sub:      "test-subject",
			iss:      testIssuer,
			validity: 0, // Should use default
			claims:   map[string]interface{}{"aud": testAudience},
			setupMock: func() func() {
				return func() {}
			},
			setupService: func() *jwtService {
				return suite.jwtService
			},
			expectError:        false,
			useDefaultValidity: true,
		},
		{
			name:     "DefaultIssuer",
			sub:      "test-subject",
			iss:      "", // Should use default
			validity: 3600,
			claims:   map[string]interface{}{"aud": testAudience},
			setupMock: func() func() {
				return func() {}
			},
			setupService: func() *jwtService {
				return suite.jwtService
			},
			expectError: false,
		},
		{
			name:      "NilCryptoProvider",
			sub:       "sub",
			iss:       "iss",
			validity:  3600,
			claims:    map[string]interface{}{"aud": "aud"},
			setupMock: func() func() { return func() {} },
			setupService: func() *jwtService {
				return &jwtService{
					cryptoProvider: nil,
					logger:         log.GetLogger().With(log.String(log.LoggerKeyComponentName, "JWTService")),
				}
			},
			expectError: true,
			errorCode:   "SSE-5000",
		},
		{
			name:     "SigningError",
			sub:      "sub",
			iss:      "iss",
			validity: 3600,
			claims:   map[string]interface{}{"aud": "aud"},
			setupMock: func() func() {
				return func() {}
			},
			setupService: func() *jwtService {
				cryptoMock := cryptomock.NewRuntimeCryptoProviderMock(suite.T())
				cryptoMock.EXPECT().Sign(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(nil, errors.New("signing failed"))
				return &jwtService{
					cryptoProvider: cryptoMock,
					keyRef:         kmprovider.KeyRef{KeyID: "test-kid"},
					logger:         log.GetLogger().With(log.String(log.LoggerKeyComponentName, "JWTService")),
				}
			},
			expectError: true,
		},
		{
			name:     "LongValidityPeriod",
			sub:      "test-subject",
			iss:      testIssuer,
			validity: 86400, // 24 hours
			claims:   map[string]interface{}{"aud": testAudience},
			setupMock: func() func() {
				return func() {}
			},
			setupService: func() *jwtService {
				return suite.jwtService
			},
			expectError: false,
			validateSuccess: func(t *testing.T, token string, iat int64) {
				parts := strings.Split(token, ".")
				payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
				assert.NoError(t, err)

				var payload map[string]interface{}
				err = json.Unmarshal(payloadBytes, &payload)
				assert.NoError(t, err)

				exp := int64(payload["exp"].(float64))
				assert.True(t, exp-iat >= 86400-5) // Allow 5 second tolerance
			},
		},
		{
			name:     "ComplexClaims",
			sub:      "test-subject",
			iss:      testIssuer,
			validity: 3600,
			claims: map[string]interface{}{
				"aud":    testAudience,
				"roles":  []string{"admin", "user"},
				"nested": map[string]interface{}{"key": "value"},
				"number": 42,
				"bool":   true,
			},
			setupMock: func() func() {
				return func() {}
			},
			setupService: func() *jwtService {
				return suite.jwtService
			},
			expectError: false,
			validateSuccess: func(t *testing.T, token string, iat int64) {
				parts := strings.Split(token, ".")
				payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
				assert.NoError(t, err)

				var payload map[string]interface{}
				err = json.Unmarshal(payloadBytes, &payload)
				assert.NoError(t, err)

				roles := payload["roles"].([]interface{})
				assert.Len(t, roles, 2)
				assert.Equal(t, "admin", roles[0])
				assert.Equal(t, "user", roles[1])

				nested := payload["nested"].(map[string]interface{})
				assert.Equal(t, "value", nested["key"])

				assert.Equal(t, float64(42), payload["number"])
				assert.Equal(t, true, payload["bool"])
			},
		},
		{
			name:     "EmptySubject",
			sub:      "",
			iss:      testIssuer,
			validity: 3600,
			claims:   map[string]interface{}{"aud": testAudience},
			setupMock: func() func() {
				return func() {}
			},
			setupService: func() *jwtService {
				return suite.jwtService
			},
			expectError: false,
			validateSuccess: func(t *testing.T, token string, iat int64) {
				parts := strings.Split(token, ".")
				payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
				assert.NoError(t, err)

				var payload map[string]interface{}
				err = json.Unmarshal(payloadBytes, &payload)
				assert.NoError(t, err)

				assert.Equal(t, "", payload["sub"])
			},
		},
		{
			name:     "SpecialCharactersInClaims",
			sub:      "test-subject",
			iss:      testIssuer,
			validity: 3600,
			claims: map[string]interface{}{
				"aud":   testAudience,
				"email": "test+special@example.com",
				"name":  "Test User / Admin",
			},
			setupMock: func() func() {
				return func() {}
			},
			setupService: func() *jwtService {
				return suite.jwtService
			},
			expectError: false,
			validateSuccess: func(t *testing.T, token string, iat int64) {
				parts := strings.Split(token, ".")
				payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
				assert.NoError(t, err)

				var payload map[string]interface{}
				err = json.Unmarshal(payloadBytes, &payload)
				assert.NoError(t, err)

				assert.Equal(t, "test+special@example.com", payload["email"])
				assert.Equal(t, "Test User / Admin", payload["name"])
			},
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			cleanup := tc.setupMock()
			defer cleanup() // Ensure cleanup runs regardless of test outcome

			jwtService := tc.setupService()

			token, iat, err := jwtService.GenerateJWT(
				context.Background(), tc.sub, tc.iss, tc.validity, tc.claims, TokenTypeJWT, "")

			if tc.expectError {
				assert.NotNil(t, err)
				if tc.errorCode != "" {
					assert.Equal(t, tc.errorCode, err.Code)
				}
				assert.Empty(t, token)
				assert.Equal(t, int64(0), iat)
				return
			}

			assert.Nil(t, err)
			assert.NotEmpty(t, token)
			assert.True(t, iat > 0)

			parts := strings.Split(token, ".")
			assert.Len(t, parts, 3)

			if tc.validateSuccess != nil {
				tc.validateSuccess(t, token, iat)
			}

			if tc.useDefaultValidity {
				payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
				assert.NoError(t, err)

				var payload map[string]interface{}
				err = json.Unmarshal(payloadBytes, &payload)
				assert.NoError(t, err)

				now := time.Now().Unix()
				assert.True(t, payload["exp"].(float64) >= float64(now+3600-5))
				assert.True(t, payload["exp"].(float64) <= float64(now+3600+5))
			}
		})
	}
}

func (suite *JWTServiceTestSuite) TestVerifyJWT() {
	testCases := []struct {
		name          string
		setupFunc     func() (string, string, string)
		expectError   bool
		expectedError serviceerror.ServiceError
	}{
		{
			name: "ValidJWT",
			setupFunc: func() (string, string, string) {
				aud := testAudience
				iss := testIssuer
				token := suite.createBasicJWT(aud, iss,
					time.Now().Add(time.Hour).Unix(), time.Now().Unix())
				return token, aud, iss
			},
			expectError: false,
		},
		{
			name: "ValidJWTWithEmptyExpectedAudience",
			setupFunc: func() (string, string, string) {
				iss := testIssuer
				token := suite.createBasicJWT("any-audience", iss,
					time.Now().Add(time.Hour).Unix(), time.Now().Unix())
				return token, "", iss
			},
			expectError: false,
		},
		{
			name: "ValidJWTWithEmptyExpectedIssuer",
			setupFunc: func() (string, string, string) {
				aud := testAudience
				token := suite.createBasicJWT(aud, "any-issuer",
					time.Now().Add(time.Hour).Unix(), time.Now().Unix())
				return token, aud, ""
			},
			expectError: false,
		},
		{
			name: "InvalidJWTFormat",
			setupFunc: func() (string, string, string) {
				return suite.createMalformedJWT(), testAud, testIss
			},
			expectError:   true,
			expectedError: ErrorInvalidTokenSignature,
		},
		{
			name: "InvalidSignature",
			setupFunc: func() (string, string, string) {
				token := suite.createBasicJWT(testAud, testIss, time.Now().Add(time.Hour).Unix(), time.Now().Unix())
				parts := strings.Split(token, ".")
				if len(parts) == 3 {
					token = parts[0] + "." + parts[1] + ".invalidSignature123"
				}
				return token, testAud, testIss
			},
			expectError:   true,
			expectedError: ErrorInvalidTokenSignature,
		},
		{
			name: "ExpiredToken",
			setupFunc: func() (string, string, string) {
				aud := testAudience
				iss := testIssuer
				expiredTime := time.Now().Add(-time.Hour).Unix()
				token := suite.createBasicJWT(aud, iss,
					expiredTime, time.Now().Add(-2*time.Hour).Unix())
				return token, aud, iss
			},
			expectError:   true,
			expectedError: ErrorTokenExpired,
		},
		{
			name: "TokenNotValidYet",
			setupFunc: func() (string, string, string) {
				aud := testAudience
				iss := testIssuer
				futureTime := time.Now().Add(time.Hour).Unix()
				token := suite.createBasicJWT(aud, iss,
					time.Now().Add(2*time.Hour).Unix(), futureTime)
				return token, aud, iss
			},
			expectError:   true,
			expectedError: ErrorInvalidJWTFormat,
		},
		{
			name: "InvalidAudience",
			setupFunc: func() (string, string, string) {
				aud := wrongAudience
				iss := testIssuer
				token := suite.createBasicJWT(aud, iss,
					time.Now().Add(time.Hour).Unix(), time.Now().Unix())
				return token, expectedAudience, iss
			},
			expectError:   true,
			expectedError: ErrorInvalidJWTFormat,
		},
		{
			name: "InvalidIssuer",
			setupFunc: func() (string, string, string) {
				aud := testAudience
				iss := wrongIssuer
				token := suite.createBasicJWT(aud, iss,
					time.Now().Add(time.Hour).Unix(), time.Now().Unix())
				return token, aud, expectedIssuer
			},
			expectError:   true,
			expectedError: ErrorInvalidJWTFormat,
		},
		{
			name: "PublicKeyNotAvailable",
			setupFunc: func() (string, string, string) {
				token := suite.createBasicJWT(testAudience, testIssuer,
					time.Now().Add(time.Hour).Unix(), time.Now().Unix())
				return token, testAudience, testIssuer
			},
			expectError:   true,
			expectedError: serviceerror.InternalServerError,
		},
		{
			name: "BothAudienceAndIssuerEmpty",
			setupFunc: func() (string, string, string) {
				token := suite.createBasicJWT("any-aud", "any-iss",
					time.Now().Add(time.Hour).Unix(), time.Now().Unix())
				return token, "", ""
			},
			expectError: false,
		},
		{
			name: "TokenExpiringInOneSecond",
			setupFunc: func() (string, string, string) {
				aud := testAudience
				iss := testIssuer
				token := suite.createBasicJWT(aud, iss,
					time.Now().Add(time.Second).Unix(), time.Now().Unix())
				return token, aud, iss
			},
			expectError: false,
		},
		{
			name: "TokenValidFromOneSecondAgo",
			setupFunc: func() (string, string, string) {
				aud := testAudience
				iss := testIssuer
				token := suite.createBasicJWT(aud, iss,
					time.Now().Add(time.Hour).Unix(), time.Now().Add(-time.Second).Unix())
				return token, aud, iss
			},
			expectError: false,
		},
		{
			name: "EmptyToken",
			setupFunc: func() (string, string, string) {
				return "", testAudience, testIssuer
			},
			expectError:   true,
			expectedError: ErrorInvalidTokenSignature,
		},
		{
			name: "TokenWithOnlyTwoParts",
			setupFunc: func() (string, string, string) {
				return "header.payload", testAudience, testIssuer
			},
			expectError:   true,
			expectedError: ErrorInvalidTokenSignature,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			token, expectedAud, expectedIss := tc.setupFunc()

			jwtSvc := suite.jwtService
			if tc.name == "PublicKeyNotAvailable" {
				jwtSvc = &jwtService{
					publicKey: nil,
					logger:    log.GetLogger().With(log.String(log.LoggerKeyComponentName, "JWTService")),
				}
			}

			err := jwtSvc.VerifyJWT(token, expectedAud, expectedIss)

			if tc.expectError {
				assert.NotNil(t, err)
				assert.Equal(t, tc.expectedError, *err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func (suite *JWTServiceTestSuite) TestVerifyJWTWithPublicKey() {
	testCases := []struct {
		name          string
		setupFunc     func() (string, crypto.PublicKey, string, string)
		expectError   bool
		expectedError serviceerror.ServiceError
	}{
		{
			name: "ValidJWT",
			setupFunc: func() (string, crypto.PublicKey, string, string) {
				aud := testAudience
				iss := testIssuer
				token := suite.createBasicJWT(aud, iss,
					time.Now().Add(time.Hour).Unix(), time.Now().Unix())
				return token, &suite.testPrivateKey.PublicKey, aud, iss
			},
			expectError: false,
		},
		{
			name: "ValidJWTWithEmptyExpectedAudience",
			setupFunc: func() (string, crypto.PublicKey, string, string) {
				iss := testIssuer
				token := suite.createBasicJWT("any-audience", iss,
					time.Now().Add(time.Hour).Unix(), time.Now().Unix())
				return token, &suite.testPrivateKey.PublicKey, "", iss
			},
			expectError: false,
		},
		{
			name: "ValidJWTWithEmptyExpectedIssuer",
			setupFunc: func() (string, crypto.PublicKey, string, string) {
				aud := testAudience
				token := suite.createBasicJWT(aud, "any-issuer",
					time.Now().Add(time.Hour).Unix(), time.Now().Unix())
				return token, &suite.testPrivateKey.PublicKey, aud, ""
			},
			expectError: false,
		},
		{
			name: "InvalidJWTFormat",
			setupFunc: func() (string, crypto.PublicKey, string, string) {
				return suite.createMalformedJWT(), &suite.testPrivateKey.PublicKey, testAud, testIss
			},
			expectError:   true,
			expectedError: ErrorInvalidJWTFormat,
		},
		{
			name: "InvalidSignature",
			setupFunc: func() (string, crypto.PublicKey, string, string) {
				token := suite.createBasicJWT(testAud, testIss, time.Now().Add(time.Hour).Unix(), time.Now().Unix())
				parts := strings.Split(token, ".")
				if len(parts) == 3 {
					token = parts[0] + "." + parts[1] + ".invalidSignature123"
				}
				return token, &suite.testPrivateKey.PublicKey, testAud, testIss
			},
			expectError:   true,
			expectedError: ErrorInvalidTokenSignature,
		},
		{
			name: "ExpiredToken",
			setupFunc: func() (string, crypto.PublicKey, string, string) {
				aud := testAudience
				iss := testIssuer
				expiredTime := time.Now().Add(-time.Hour).Unix()
				token := suite.createBasicJWT(aud, iss,
					expiredTime, time.Now().Add(-2*time.Hour).Unix())
				return token, &suite.testPrivateKey.PublicKey, aud, iss
			},
			expectError:   true,
			expectedError: ErrorTokenExpired,
		},
		{
			name: "TokenNotValidYet",
			setupFunc: func() (string, crypto.PublicKey, string, string) {
				aud := testAudience
				iss := testIssuer
				futureTime := time.Now().Add(time.Hour).Unix()
				token := suite.createBasicJWT(aud, iss,
					time.Now().Add(2*time.Hour).Unix(), futureTime)
				return token, &suite.testPrivateKey.PublicKey, aud, iss
			},
			expectError:   true,
			expectedError: ErrorInvalidJWTFormat,
		},
		{
			name: "InvalidAudience",
			setupFunc: func() (string, crypto.PublicKey, string, string) {
				aud := "wrong-audience"
				iss := testIssuer
				token := suite.createBasicJWT(aud, iss,
					time.Now().Add(time.Hour).Unix(), time.Now().Unix())
				return token, &suite.testPrivateKey.PublicKey, "expected-audience", iss
			},
			expectError:   true,
			expectedError: ErrorInvalidJWTFormat,
		},
		{
			name: "InvalidIssuer",
			setupFunc: func() (string, crypto.PublicKey, string, string) {
				aud := testAudience
				iss := "wrong-issuer"
				token := suite.createBasicJWT(aud, iss,
					time.Now().Add(time.Hour).Unix(), time.Now().Unix())
				return token, &suite.testPrivateKey.PublicKey, aud, "expected-issuer"
			},
			expectError:   true,
			expectedError: ErrorInvalidJWTFormat,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			token, pubKey, expectedAud, expectedIss := tc.setupFunc()

			err := suite.jwtService.VerifyJWTWithPublicKey(token, pubKey, expectedAud, expectedIss)

			if tc.expectError {
				assert.NotNil(t, err)
				assert.Equal(t, tc.expectedError, *err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func (suite *JWTServiceTestSuite) TestVerifyJWTWithJWKS() {
	testCases := []struct {
		name          string
		setupFunc     func() (string, string, string, string)
		expectError   bool
		expectedError serviceerror.ServiceError
	}{
		{
			name: "ValidJWTWithJWKS",
			setupFunc: func() (string, string, string, string) {
				aud := testAudience
				iss := testIssuer
				token := suite.createBasicJWT(aud, iss,
					time.Now().Add(time.Hour).Unix(), time.Now().Unix())

				mockServer := suite.mockJWKSServer()
				suite.T().Cleanup(mockServer.Close)

				return token, mockServer.URL, aud, iss
			},
			expectError: false,
		},
		{
			name: "ValidJWTWithEmptyExpectedClaims",
			setupFunc: func() (string, string, string, string) {
				token := suite.createBasicJWT("any-aud", "any-iss",
					time.Now().Add(time.Hour).Unix(), time.Now().Unix())

				mockServer := suite.mockJWKSServer()
				suite.T().Cleanup(mockServer.Close)

				return token, mockServer.URL, "", "" // Empty expected aud and iss
			},
			expectError: false,
		},
		{
			name: "InvalidJWTFormat",
			setupFunc: func() (string, string, string, string) {
				mockServer := suite.mockJWKSServer()
				suite.T().Cleanup(mockServer.Close)

				return suite.createMalformedJWT(), mockServer.URL, testAud, testIss
			},
			expectError:   true,
			expectedError: ErrorInvalidJWTFormat,
		},
		{
			name: "InvalidSignatureWithJWKS",
			setupFunc: func() (string, string, string, string) {
				// Create a valid token first, then invalidate the signature
				token := suite.createBasicJWT(testAud, testIss, time.Now().Add(time.Hour).Unix(), time.Now().Unix())

				// Replace signature to make it invalid
				parts := strings.Split(token, ".")
				if len(parts) == 3 {
					token = parts[0] + "." + parts[1] + ".invalidSignature123"
				}

				mockServer := suite.mockJWKSServer()
				suite.T().Cleanup(mockServer.Close)

				return token, mockServer.URL, testAud, testIss
			},
			expectError:   true,
			expectedError: ErrorInvalidTokenSignature,
		},
		{
			name: "ExpiredTokenWithJWKS",
			setupFunc: func() (string, string, string, string) {
				aud := testAudience
				iss := testIssuer
				expiredTime := time.Now().Add(-time.Hour).Unix() // Expired 1 hour ago
				token := suite.createBasicJWT(aud, iss,
					expiredTime, time.Now().Add(-2*time.Hour).Unix())

				mockServer := suite.mockJWKSServer()
				suite.T().Cleanup(mockServer.Close)

				return token, mockServer.URL, aud, iss
			},
			expectError:   true,
			expectedError: ErrorTokenExpired,
		},
		{
			name: "TokenNotValidYetWithJWKS",
			setupFunc: func() (string, string, string, string) {
				aud := testAudience
				iss := testIssuer
				futureTime := time.Now().Add(time.Hour).Unix() // Valid 1 hour from now
				token := suite.createBasicJWT(aud, iss,
					time.Now().Add(2*time.Hour).Unix(), futureTime)

				mockServer := suite.mockJWKSServer()
				suite.T().Cleanup(mockServer.Close)

				return token, mockServer.URL, aud, iss
			},
			expectError:   true,
			expectedError: ErrorInvalidJWTFormat,
		},
		{
			name: "InvalidAudienceWithJWKS",
			setupFunc: func() (string, string, string, string) {
				aud := "wrong-audience"
				iss := testIssuer
				token := suite.createBasicJWT(aud, iss,
					time.Now().Add(time.Hour).Unix(), time.Now().Unix())

				mockServer := suite.mockJWKSServer()
				suite.T().Cleanup(mockServer.Close)

				return token, mockServer.URL, "expected-audience", iss
			},
			expectError:   true,
			expectedError: ErrorInvalidJWTFormat,
		},
		{
			name: "InvalidIssuerWithJWKS",
			setupFunc: func() (string, string, string, string) {
				aud := testAudience
				iss := "wrong-issuer"
				token := suite.createBasicJWT(aud, iss,
					time.Now().Add(time.Hour).Unix(), time.Now().Unix())

				mockServer := suite.mockJWKSServer()
				suite.T().Cleanup(mockServer.Close)

				return token, mockServer.URL, aud, "expected-issuer"
			},
			expectError:   true,
			expectedError: ErrorInvalidJWTFormat,
		},
		{
			name: "JWKSNetworkError",
			setupFunc: func() (string, string, string, string) {
				token := suite.createBasicJWT(testAud, testIss,
					time.Now().Add(time.Hour).Unix(), time.Now().Unix())

				return token, "http://localhost:99999/invalid", testAud, testIss
			},
			expectError:   true,
			expectedError: ErrorInvalidTokenSignature,
		},
		{
			name: "JWKSHTTPError",
			setupFunc: func() (string, string, string, string) {
				token := suite.createBasicJWT(testAud, testIss,
					time.Now().Add(time.Hour).Unix(), time.Now().Unix())

				// Create a server that returns 404
				errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
				}))
				suite.T().Cleanup(errorServer.Close)

				return token, errorServer.URL, testAud, testIss
			},
			expectError:   true,
			expectedError: ErrorInvalidTokenSignature,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			token, jwksURL, expectedAud, expectedIss := tc.setupFunc()

			err := suite.jwtService.VerifyJWTWithJWKS(token, jwksURL, expectedAud, expectedIss)

			if tc.expectError {
				assert.NotNil(t, err)
				assert.Equal(t, tc.expectedError, *err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func (suite *JWTServiceTestSuite) TestVerifyJWTClaimsEdgeCases() {
	testCases := []struct {
		name          string
		setupFunc     func(t *testing.T) string
		expectedAud   string
		expectedIss   string
		expectError   bool
		expectedError serviceerror.ServiceError
	}{
		{
			name: "MissingExpClaim",
			setupFunc: func(t *testing.T) string {
				payload := map[string]interface{}{
					"sub": "test-subject",
					"aud": testAudience,
					"iss": testIssuer,
					"iat": time.Now().Unix(),
					"nbf": time.Now().Unix(),
					// Missing exp claim
				}
				return suite.createJWTWithCustomPayload(t, payload)
			},
			expectedAud:   testAudience,
			expectedIss:   testIssuer,
			expectError:   true,
			expectedError: ErrorInvalidJWTFormat,
		},
		{
			name: "MissingNbfClaim",
			setupFunc: func(t *testing.T) string {
				payload := map[string]interface{}{
					"sub": "test-subject",
					"aud": testAudience,
					"iss": testIssuer,
					"exp": time.Now().Add(time.Hour).Unix(),
					"iat": time.Now().Unix(),
					// Missing nbf claim
				}
				return suite.createJWTWithCustomPayload(t, payload)
			},
			expectedAud: testAudience,
			expectedIss: testIssuer,
			expectError: false,
		},
		{
			name: "AudClaimAsArrayContainingExpected",
			setupFunc: func(t *testing.T) string {
				payload := map[string]interface{}{
					"sub": "test-subject",
					"aud": []interface{}{testAudience, "https://example.auth0.com/userinfo"},
					"iss": testIssuer,
					"exp": time.Now().Add(time.Hour).Unix(),
					"iat": time.Now().Unix(),
				}
				return suite.createJWTWithCustomPayload(t, payload)
			},
			expectedAud: testAudience,
			expectedIss: testIssuer,
			expectError: false,
		},
		{
			name: "AudClaimAsArrayWithoutExpected",
			setupFunc: func(t *testing.T) string {
				payload := map[string]interface{}{
					"sub": "test-subject",
					"aud": []interface{}{"https://other.example.com", "https://example.auth0.com/userinfo"},
					"iss": testIssuer,
					"exp": time.Now().Add(time.Hour).Unix(),
					"iat": time.Now().Unix(),
				}
				return suite.createJWTWithCustomPayload(t, payload)
			},
			expectedAud:   testAudience,
			expectedIss:   testIssuer,
			expectError:   true,
			expectedError: ErrorInvalidJWTFormat,
		},
		{
			name: "MissingAudClaim",
			setupFunc: func(t *testing.T) string {
				payload := map[string]interface{}{
					"sub": "test-subject",
					"iss": testIssuer,
					"exp": time.Now().Add(time.Hour).Unix(),
					"iat": time.Now().Unix(),
					"nbf": time.Now().Unix(),
					// Missing aud claim
				}
				return suite.createJWTWithCustomPayload(t, payload)
			},
			expectedAud:   testAudience,
			expectedIss:   testIssuer,
			expectError:   true,
			expectedError: ErrorInvalidJWTFormat,
		},
		{
			name: "MissingIssClaim",
			setupFunc: func(t *testing.T) string {
				payload := map[string]interface{}{
					"sub": "test-subject",
					"aud": testAudience,
					"exp": time.Now().Add(time.Hour).Unix(),
					"iat": time.Now().Unix(),
					"nbf": time.Now().Unix(),
					// Missing iss claim
				}
				return suite.createJWTWithCustomPayload(t, payload)
			},
			expectedAud:   testAudience,
			expectedIss:   testIssuer,
			expectError:   true,
			expectedError: ErrorInvalidJWTFormat,
		},
		{
			name: "InvalidExpClaimType",
			setupFunc: func(t *testing.T) string {
				payload := map[string]interface{}{
					"sub": "test-subject",
					"aud": testAudience,
					"iss": testIssuer,
					"exp": "invalid-exp-type", // Wrong type
					"iat": time.Now().Unix(),
					"nbf": time.Now().Unix(),
				}
				return suite.createJWTWithCustomPayload(t, payload)
			},
			expectedAud:   testAudience,
			expectedIss:   testIssuer,
			expectError:   true,
			expectedError: ErrorInvalidJWTFormat,
		},
		{
			name: "InvalidNbfClaimType",
			setupFunc: func(t *testing.T) string {
				payload := map[string]interface{}{
					"sub": "test-subject",
					"aud": testAudience,
					"iss": testIssuer,
					"exp": time.Now().Add(time.Hour).Unix(),
					"iat": time.Now().Unix(),
					"nbf": "invalid-nbf-type", // Wrong type
				}
				return suite.createJWTWithCustomPayload(t, payload)
			},
			expectedAud:   testAudience,
			expectedIss:   testIssuer,
			expectError:   true,
			expectedError: ErrorInvalidJWTFormat,
		},
		{
			name: "InvalidAudClaimType",
			setupFunc: func(t *testing.T) string {
				payload := map[string]interface{}{
					"sub": "test-subject",
					"aud": 12345, // Wrong type
					"iss": testIssuer,
					"exp": time.Now().Add(time.Hour).Unix(),
					"iat": time.Now().Unix(),
					"nbf": time.Now().Unix(),
				}
				return suite.createJWTWithCustomPayload(t, payload)
			},
			expectedAud:   testAudience,
			expectedIss:   testIssuer,
			expectError:   true,
			expectedError: ErrorInvalidJWTFormat,
		},
		{
			name: "InvalidIssClaimType",
			setupFunc: func(t *testing.T) string {
				payload := map[string]interface{}{
					"sub": "test-subject",
					"aud": testAudience,
					"iss": 12345, // Wrong type
					"exp": time.Now().Add(time.Hour).Unix(),
					"iat": time.Now().Unix(),
					"nbf": time.Now().Unix(),
				}
				return suite.createJWTWithCustomPayload(t, payload)
			},
			expectedAud:   testAudience,
			expectedIss:   testIssuer,
			expectError:   true,
			expectedError: ErrorInvalidJWTFormat,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			token := tc.setupFunc(t)
			publicKey := &suite.testPrivateKey.PublicKey

			err := suite.jwtService.VerifyJWTWithPublicKey(token, publicKey, tc.expectedAud, tc.expectedIss)

			if tc.expectError {
				assert.NotNil(t, err)
				assert.Equal(t, tc.expectedError, *err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func (suite *JWTServiceTestSuite) TestVerifyJWTSignature() {
	testCases := []struct {
		name        string
		setupFunc   func() string
		expectError bool
	}{
		{
			name: "ValidToken",
			setupFunc: func() string {
				token, _, err := suite.jwtService.GenerateJWT(context.Background(),
					"test-subject", testIssuer, 3600, map[string]interface{}{"aud": testAudience}, TokenTypeJWT, "")
				assert.Nil(suite.T(), err)
				return token
			},
			expectError: false,
		},
		{
			name: "InvalidToken",
			setupFunc: func() string {
				return "invalid.token"
			},
			expectError: true,
		},
		{
			name: "TamperedToken",
			setupFunc: func() string {
				parts := []string{}
				for _, part := range []string{"header", "payload", "signature"} {
					jsonData, _ := json.Marshal(map[string]string{"tampered": part})
					parts = append(parts, base64.RawURLEncoding.EncodeToString(jsonData))
				}
				return parts[0] + "." + parts[1] + "." + parts[2]
			},
			expectError: true,
		},
		{
			name: "PublicKeyNotAvailable",
			setupFunc: func() string {
				token, _, err := suite.jwtService.GenerateJWT(context.Background(),
					"test-subject", testIssuer, 3600, map[string]interface{}{"aud": testAudience}, TokenTypeJWT, "")
				assert.Nil(suite.T(), err)
				return token
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			token := tc.setupFunc()

			jwtSvc := suite.jwtService
			if tc.name == "PublicKeyNotAvailable" {
				jwtSvc = &jwtService{
					publicKey: nil,
				}
			}

			err := jwtSvc.VerifyJWTSignature(token)
			if tc.expectError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func (suite *JWTServiceTestSuite) TestVerifyJWTSignatureWithPublicKey() {
	validToken, _, err := suite.jwtService.GenerateJWT(context.Background(),
		"test-subject", testIssuer, 3600, map[string]interface{}{"aud": testAudience}, TokenTypeJWT, "")
	assert.Nil(suite.T(), err)

	wrongKey, _ := rsa.GenerateKey(rand.Reader, 2048)

	parts := []string{}
	for _, part := range []string{"header", "payload", "signature"} {
		jsonData, _ := json.Marshal(map[string]string{"tampered": part})
		parts = append(parts, base64.RawURLEncoding.EncodeToString(jsonData))
	}
	tamperedToken := parts[0] + "." + parts[1] + "." + parts[2]

	testCases := []struct {
		name        string
		token       string
		publicKey   crypto.PublicKey
		expectError bool
	}{
		{"ValidToken", validToken, &suite.testPrivateKey.PublicKey, false},
		{"WrongKey", validToken, &wrongKey.PublicKey, true},
		{"InvalidToken", "invalid.token", &suite.testPrivateKey.PublicKey, true},
		{"TamperedToken", tamperedToken, &suite.testPrivateKey.PublicKey, true},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			err := suite.jwtService.VerifyJWTSignatureWithPublicKey(tc.token, tc.publicKey)
			if tc.expectError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func (suite *JWTServiceTestSuite) TestVerifyJWTSignatureWithJWKS() {
	token, _, err := suite.jwtService.GenerateJWT(context.Background(),
		"test-subject", testIssuer, 3600, map[string]interface{}{"aud": testAudience}, TokenTypeJWT, "")
	assert.Nil(suite.T(), err)

	testServer := suite.mockJWKSServer()
	defer testServer.Close()

	err = suite.jwtService.VerifyJWTSignatureWithJWKS(token, testServer.URL)
	assert.Nil(suite.T(), err)
}

func (suite *JWTServiceTestSuite) TestVerifyJWTSignatureWithJWKSUsesCache() {
	// Verifies the in-process JWKS cache contract:
	//   1. First fetch for a given URL hits the network.
	//   2. Subsequent fetches for the same URL hit the cache and do NOT re-fetch.
	//   3. A different URL is keyed independently — fetching it does NOT consume the
	//      cached entry from another URL, and asking for the original URL again still
	//      returns its own cached value (one fetch each).
	//
	// Point 3 catches a buggy cache that stores or returns entries without keying by
	// URL — without it, a single shared `cached` slot would still pass points 1 and 2.
	//
	// The default SetupTest config has SecurityConfig.JWKSCacheTTL == 0, which would
	// cause the cache to expire instantly (Now().Before(Now()) == false). Re-initialize
	// the runtime here with a positive TTL so the cache actually retains entries.
	config.ResetServerRuntime()
	defer config.ResetServerRuntime()
	testConfig := &config.Config{
		JWT: config.JWTConfig{
			Issuer:         testIssuer,
			ValidityPeriod: 3600,
			Leeway:         30,
		},
		Server: config.ServerConfig{
			SecurityConfig: config.SecurityConfig{
				JWKSCacheTTL: 300,
			},
		},
	}
	err := config.InitializeServerRuntime("", testConfig)
	assert.NoError(suite.T(), err)

	jwksData := suite.createMockJWKSData()
	makeServer := func(counter *int32) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(counter, 1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if _, writeErr := fmt.Fprintln(w, jwksData); writeErr != nil {
				suite.T().Errorf("Failed to write JWKS response: %v", writeErr)
			}
		}))
	}

	var fetchCountA, fetchCountB int32
	serverA := makeServer(&fetchCountA)
	defer serverA.Close()
	serverB := makeServer(&fetchCountB)
	defer serverB.Close()

	token, _, genErr := suite.jwtService.GenerateJWT(context.Background(),
		"test-subject", testIssuer, 3600, map[string]interface{}{"aud": testAudience}, TokenTypeJWT, "")
	assert.Nil(suite.T(), genErr)

	// 1. First call against serverA — cache miss, one fetch.
	assert.Nil(suite.T(), suite.jwtService.VerifyJWTSignatureWithJWKS(token, serverA.URL))
	assert.Equal(suite.T(), int32(1), atomic.LoadInt32(&fetchCountA),
		"first call to serverA should fetch JWKS once")
	assert.Equal(suite.T(), int32(0), atomic.LoadInt32(&fetchCountB),
		"serverB should not have been touched yet")

	// 2. Second call against serverA — cache hit, no additional fetch.
	assert.Nil(suite.T(), suite.jwtService.VerifyJWTSignatureWithJWKS(token, serverA.URL))
	assert.Equal(suite.T(), int32(1), atomic.LoadInt32(&fetchCountA),
		"second call to serverA should hit the cache, not re-fetch")

	// 3a. First call against serverB — must miss the cache (different URL key) and
	//     fetch independently. A buggy cache that returns any entry would skip this
	//     fetch and the count would stay at 0.
	assert.Nil(suite.T(), suite.jwtService.VerifyJWTSignatureWithJWKS(token, serverB.URL))
	assert.Equal(suite.T(), int32(1), atomic.LoadInt32(&fetchCountB),
		"first call to serverB should fetch independently (cache is keyed by URL)")
	assert.Equal(suite.T(), int32(1), atomic.LoadInt32(&fetchCountA),
		"fetching serverB must not provoke a re-fetch of serverA")

	// 3b. Going back to serverA must STILL be a cache hit — the serverB fetch must
	//     not have evicted or overwritten serverA's cache entry.
	assert.Nil(suite.T(), suite.jwtService.VerifyJWTSignatureWithJWKS(token, serverA.URL))
	assert.Equal(suite.T(), int32(1), atomic.LoadInt32(&fetchCountA),
		"serverA's cache entry must survive an unrelated fetch of serverB")
}

func (suite *JWTServiceTestSuite) TestVerifyJWTSignatureWithJWKSInvalidToken() {
	testServer := suite.mockJWKSServer()
	defer testServer.Close()

	testCases := []struct {
		name  string
		token string
	}{
		{"EmptyToken", ""},
		{"MalformedToken", "not.valid.jwt"},
		{"InvalidFormat", "header.payload"},                 // Missing signature part
		{"CorruptedHeader", "aGVhZGVyCg.payload.signature"}, // Non-decodable header
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			err := suite.jwtService.VerifyJWTSignatureWithJWKS(tc.token, testServer.URL)
			assert.NotNil(t, err)
		})
	}
}

func (suite *JWTServiceTestSuite) TestVerifyJWTSignatureWithJWKSKeyIDNotFound() {
	testServer := suite.mockJWKSServer()
	defer testServer.Close()

	nonExistentKidJWT := suite.createJWTWithCustomHeader(map[string]interface{}{
		"alg": "RS256",
		"typ": "JWT",
		"kid": "non-existent-key-id",
	})

	err := suite.jwtService.VerifyJWTSignatureWithJWKS(nonExistentKidJWT, testServer.URL)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorNoMatchingJWKFound, *err)
}

func (suite *JWTServiceTestSuite) TestVerifyJWTSignatureWithJWKSNoKeyID() {
	testServer := suite.mockJWKSServer()
	defer testServer.Close()

	noKidJWT := suite.createJWTWithCustomHeader(map[string]interface{}{
		"alg": "RS256",
		"typ": "JWT",
		// No kid field
	})

	err := suite.jwtService.VerifyJWTSignatureWithJWKS(noKidJWT, testServer.URL)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorDecodingJWTHeader, *err)
}

func (suite *JWTServiceTestSuite) TestVerifyJWTSignatureWithJWKSHTTPErrors() {
	testCases := []struct {
		name          string
		setupServer   func() *httptest.Server
		setupToken    func() string
		expectedError serviceerror.ServiceError
	}{
		{
			name: "HTTPError404",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
				}))
			},
			setupToken: func() string {
				token, _, err := suite.jwtService.GenerateJWT(context.Background(),
					"test-subject", testIssuer, 3600, map[string]interface{}{"aud": testAudience}, TokenTypeJWT, "")
				assert.Nil(suite.T(), err)
				return token
			},
			expectedError: ErrorFailedToGetJWKS,
		},
		{
			name: "InvalidJSONResponse",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					if _, err := w.Write([]byte("invalid json")); err != nil {
						suite.T().Errorf("Failed to write response: %v", err)
					}
				}))
			},
			setupToken: func() string {
				token, _, err := suite.jwtService.GenerateJWT(context.Background(),
					"test-subject", testIssuer, 3600, map[string]interface{}{"aud": testAudience}, TokenTypeJWT, "")
				assert.Nil(suite.T(), err)
				return token
			},
			expectedError: ErrorFailedToParseJWKS,
		},
		{
			name: "JWKSWithoutMatchingKid",
			setupServer: func() *httptest.Server {
				// Create JWKS with different kid
				jwks := map[string]interface{}{
					"keys": []interface{}{
						map[string]interface{}{
							"kty": "RSA",
							"kid": "different-kid",
							"n":   "some-n",
							"e":   "AQAB",
						},
					},
				}
				jwksData, _ := json.Marshal(jwks)
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					if _, err := w.Write(jwksData); err != nil {
						suite.T().Errorf("Failed to write response: %v", err)
					}
				}))
			},
			setupToken: func() string {
				token, _, err := suite.jwtService.GenerateJWT(context.Background(),
					"test-subject", testIssuer, 3600, map[string]interface{}{"aud": testAudience}, TokenTypeJWT, "")
				assert.Nil(suite.T(), err)
				return token
			},
			expectedError: ErrorNoMatchingJWKFound,
		},
		{
			name: "InvalidJWKFormat",
			setupServer: func() *httptest.Server {
				// Create JWKS with invalid JWK (missing n and e)
				jwks := map[string]interface{}{
					"keys": []interface{}{
						map[string]interface{}{
							"kty": "RSA",
							"kid": "test-kid",
							// Missing n and e
						},
					},
				}
				jwksData, _ := json.Marshal(jwks)
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					if _, err := w.Write(jwksData); err != nil {
						suite.T().Errorf("Failed to write response: %v", err)
					}
				}))
			},
			setupToken: func() string {
				token, _, err := suite.jwtService.GenerateJWT(context.Background(),
					"test-subject", testIssuer, 3600, map[string]interface{}{"aud": testAudience}, TokenTypeJWT, "")
				assert.Nil(suite.T(), err)
				return token
			},
			expectedError: ErrorFailedToParseJWKS,
		},
		{
			name: "InvalidTokenSignature",
			setupServer: func() *httptest.Server {
				return suite.mockJWKSServer()
			},
			setupToken: func() string {
				// Create a token with wrong signature
				token := suite.createJWTWithCustomHeader(map[string]interface{}{
					"alg": "RS256",
					"typ": "JWT",
					"kid": "test-kid",
				})
				// Modify the last part (signature) to make it invalid
				parts := strings.Split(token, ".")
				parts[2] = "invalid-signature"
				return strings.Join(parts, ".")
			},
			expectedError: ErrorInvalidTokenSignature,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			testServer := tc.setupServer()
			defer testServer.Close()

			token := tc.setupToken()

			err := suite.jwtService.VerifyJWTSignatureWithJWKS(token, testServer.URL)
			assert.NotNil(t, err)
			assert.Equal(t, tc.expectedError, *err)
		})
	}
}

func (suite *JWTServiceTestSuite) TestVerifyJWTSignatureWithJWKSNetworkError() {
	// Test with invalid URL to trigger network error
	token, _, err := suite.jwtService.GenerateJWT(context.Background(),
		"test-subject", testIssuer, 3600, map[string]interface{}{"aud": testAudience}, TokenTypeJWT, "")
	assert.Nil(suite.T(), err)

	err = suite.jwtService.VerifyJWTSignatureWithJWKS(token, "http://localhost:99999/invalid")
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), ErrorFailedToGetJWKS, *err)
}

// Helper method to create a JWT with a custom header
func (suite *JWTServiceTestSuite) createJWTWithCustomHeader(header map[string]interface{}) string {
	// Create payload
	payload := map[string]interface{}{
		"sub":  "1234567890",
		"name": "Test User",
		"iat":  time.Now().Unix(),
		"exp":  time.Now().Add(time.Hour).Unix(),
	}

	headerJSON, _ := json.Marshal(header)
	payloadJSON, _ := json.Marshal(payload)

	// Encode header and payload
	headerBase64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	payloadBase64 := base64.RawURLEncoding.EncodeToString(payloadJSON)

	// Create signature input
	signingInput := headerBase64 + "." + payloadBase64

	// Sign
	signature, err := cryptolab.Generate([]byte(signingInput), cryptolab.RSASHA256, suite.testPrivateKey)
	if err != nil {
		suite.T().Fatalf("Failed to sign JWT for signing input %q: %v", signingInput, err)
	}

	// Encode signature
	signatureBase64 := base64.RawURLEncoding.EncodeToString(signature)

	// Create full JWT
	return headerBase64 + "." + payloadBase64 + "." + signatureBase64
}

// Helper method to create mock JWKS data
func (suite *JWTServiceTestSuite) createMockJWKSData() string {
	n := base64.RawURLEncoding.EncodeToString(suite.testPrivateKey.PublicKey.N.Bytes())

	// Convert exponent to bytes
	eBytes := []byte{1, 0, 1} // 65537 in big-endian
	e := base64.RawURLEncoding.EncodeToString(eBytes)

	jwk := map[string]interface{}{
		"kty": "RSA",
		"n":   n,
		"e":   e,
		"kid": "test-kid",
		"use": "sig",
		"alg": "RS256",
	}

	jwks := map[string]interface{}{
		"keys": []interface{}{jwk},
	}

	jwksData, _ := json.Marshal(jwks)
	return string(jwksData)
}

// Helper method to mock a JWKS server
func (suite *JWTServiceTestSuite) mockJWKSServer() *httptest.Server {
	jwksData := suite.createMockJWKSData()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := fmt.Fprintln(w, jwksData); err != nil {
			suite.T().Errorf("Failed to write JWKS response: %v", err)
		}
	}))

	return server
}

// Helper method to create a JWT with custom claims and validity
func (suite *JWTServiceTestSuite) createJWTWithClaims(sub, aud, iss string, exp int64, nbf int64,
	customClaims map[string]interface{}) string {
	// Create payload
	payload := map[string]interface{}{
		"sub": sub,
		"aud": aud,
		"iss": iss,
		"exp": exp,
		"iat": time.Now().Unix(),
		"nbf": nbf,
		"jti": "test-jti-" + fmt.Sprintf("%d", time.Now().UnixNano()),
	}

	// Add custom claims if provided
	for k, v := range customClaims {
		payload[k] = v
	}

	// Create header
	header := map[string]interface{}{
		"alg": "RS256",
		"typ": "JWT",
		"kid": "test-kid",
	}

	headerJSON, _ := json.Marshal(header)
	payloadJSON, _ := json.Marshal(payload)

	// Encode header and payload
	headerBase64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	payloadBase64 := base64.RawURLEncoding.EncodeToString(payloadJSON)

	// Create signature input
	signingInput := headerBase64 + "." + payloadBase64

	// Sign
	signature, err := cryptolab.Generate([]byte(signingInput), cryptolab.RSASHA256, suite.testPrivateKey)
	if err != nil {
		suite.T().Fatalf("Failed to sign JWT for signing input %q: %v", signingInput, err)
	}

	// Encode signature
	signatureBase64 := base64.RawURLEncoding.EncodeToString(signature)

	// Create full JWT
	return headerBase64 + "." + payloadBase64 + "." + signatureBase64
}

// Helper method to create an invalid JWT (malformed)
func (suite *JWTServiceTestSuite) createMalformedJWT() string {
	return "invalid.jwt"
}

// Helper method to create a JWT with custom payload for testing edge cases
func (suite *JWTServiceTestSuite) createJWTWithCustomPayload(t *testing.T, payload map[string]interface{}) string {
	t.Helper()

	header := map[string]interface{}{
		"alg": "RS256",
		"typ": "JWT",
		"kid": "test-kid",
	}

	headerJSON, _ := json.Marshal(header)
	payloadJSON, _ := json.Marshal(payload)

	headerBase64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	payloadBase64 := base64.RawURLEncoding.EncodeToString(payloadJSON)

	signingInput := headerBase64 + "." + payloadBase64
	signature, err := cryptolab.Generate([]byte(signingInput), cryptolab.RSASHA256, suite.testPrivateKey)
	if err != nil {
		t.Fatalf("Failed to sign JWT for signing input %q: %v", signingInput, err)
	}
	signatureBase64 := base64.RawURLEncoding.EncodeToString(signature)

	return headerBase64 + "." + payloadBase64 + "." + signatureBase64
}

// Helper method to create a JWT with basic claims for testing
func (suite *JWTServiceTestSuite) createBasicJWT(aud, iss string, exp int64, nbf int64) string {
	return suite.createJWTWithClaims("test-subject", aud, iss, exp, nbf, nil)
}

func (suite *JWTServiceTestSuite) TestInitWithECDSAKeys() {
	testCases := []struct {
		name            string
		curve           elliptic.Curve
		expectedAlg     jws.Algorithm
		expectedSignAlg cryptolab.SignAlgorithm
	}{
		{
			name:            "P256Key",
			curve:           elliptic.P256(),
			expectedAlg:     jws.ES256,
			expectedSignAlg: cryptolab.ECDSASHA256,
		},
		{
			name:            "P384Key",
			curve:           elliptic.P384(),
			expectedAlg:     jws.ES384,
			expectedSignAlg: cryptolab.ECDSASHA384,
		},
		{
			name:            "P521Key",
			curve:           elliptic.P521(),
			expectedAlg:     jws.ES512,
			expectedSignAlg: cryptolab.ECDSASHA512,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			// Generate ECDSA key
			ecKey, err := ecdsa.GenerateKey(tc.curve, rand.Reader)
			assert.NoError(t, err)

			// Marshal to PKCS8
			pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(ecKey)
			assert.NoError(t, err)

			keyPEM := pem.EncodeToMemory(&pem.Block{
				Type:  "PRIVATE KEY",
				Bytes: pkcs8Bytes,
			})

			// Write to temp file
			tempFile, err := os.CreateTemp("", "ec_key_*.pem")
			assert.NoError(t, err)
			defer func() {
				if err := os.Remove(tempFile.Name()); err != nil {
					t.Logf("Failed to remove temp file: %v", err)
				}
			}()

			_, err = tempFile.Write(keyPEM)
			assert.NoError(t, err)
			err = tempFile.Close()
			assert.NoError(t, err)

			// Initialize JWT service with mock
			pkiMock := pkimock.NewPKIServiceInterfaceMock(t)
			pkiMock.EXPECT().GetPrivateKey(mock.Anything).Return(ecKey, nil)
			pkiMock.EXPECT().GetCertThumbprint(mock.Anything).Return("test-kid")

			service, err := Initialize(pkiMock)

			assert.NoError(t, err)
			assert.NotNil(t, service)

			// Cast to access internal fields for testing
			jwtSvc, ok := service.(*jwtService)
			assert.True(t, ok)
			assert.NotNil(t, jwtSvc.publicKey)
			assert.Equal(t, tc.expectedSignAlg, jwtSvc.signAlg)
			assert.Equal(t, tc.expectedAlg, jwtSvc.jwsAlg)

			// Test JWT generation with ECDSA key
			token, _, svcErr := service.GenerateJWT(context.Background(),
				"test-subject", "test-iss", 3600, map[string]interface{}{"aud": "test-aud"}, TokenTypeJWT, "")
			assert.Nil(t, svcErr)
			assert.NotEmpty(t, token)

			// Verify token header has correct alg
			header, err := DecodeJWTHeader(token)
			assert.NoError(t, err)
			assert.Equal(t, string(tc.expectedAlg), header["alg"])

			// Verify signature
			svcErr = service.VerifyJWTSignature(token)
			assert.Nil(t, svcErr)
		})
	}
}

func (suite *JWTServiceTestSuite) TestInitWithEd25519Key() {
	// Generate Ed25519 key
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	assert.NoError(suite.T(), err)
	_ = pub // silence unused

	// Marshal to PKCS8
	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(priv)
	assert.NoError(suite.T(), err)

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: pkcs8Bytes,
	})

	// Write to temp file
	tempFile, err := os.CreateTemp("", "ed25519_key_*.pem")
	assert.NoError(suite.T(), err)
	defer func() {
		if err := os.Remove(tempFile.Name()); err != nil {
			suite.T().Logf("Failed to remove temp file: %v", err)
		}
	}()

	_, err = tempFile.Write(keyPEM)
	assert.NoError(suite.T(), err)
	err = tempFile.Close()
	assert.NoError(suite.T(), err)

	pkiMock := pkimock.NewPKIServiceInterfaceMock(suite.T())
	pkiMock.EXPECT().GetPrivateKey(mock.Anything).Return(priv, nil)
	pkiMock.EXPECT().GetCertThumbprint(mock.Anything).Return("test-kid")

	service, err := Initialize(pkiMock)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), service)

	// Cast to access internal fields for testing
	jwtSvc, ok := service.(*jwtService)
	assert.True(suite.T(), ok)
	assert.NotNil(suite.T(), jwtSvc.publicKey)
	assert.Equal(suite.T(), cryptolab.ED25519, jwtSvc.signAlg)
	assert.Equal(suite.T(), jws.EdDSA, jwtSvc.jwsAlg)

	// Test JWT generation with Ed25519 key
	token, _, svcErr := service.GenerateJWT(context.Background(),
		"test-subject", "test-iss", 3600, map[string]interface{}{"aud": "test-aud"}, TokenTypeJWT, "")
	assert.Nil(suite.T(), svcErr)
	assert.NotEmpty(suite.T(), token)

	// Verify token header has correct alg
	header, err := DecodeJWTHeader(token)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "EdDSA", header["alg"])

	// Verify signature
	svcErr = service.VerifyJWTSignature(token)
	assert.Nil(suite.T(), svcErr)
}

func (suite *JWTServiceTestSuite) TestInitWithECPrivateKeyFormat() {
	// Test EC PRIVATE KEY format (not PKCS8)
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	assert.NoError(suite.T(), err)

	// Marshal as EC PRIVATE KEY (not PKCS8)
	ecBytes, err := x509.MarshalECPrivateKey(ecKey)
	assert.NoError(suite.T(), err)

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: ecBytes,
	})

	tempFile, err := os.CreateTemp("", "ec_priv_key_*.pem")
	assert.NoError(suite.T(), err)
	defer func() {
		if err := os.Remove(tempFile.Name()); err != nil {
			suite.T().Logf("Failed to remove temp file: %v", err)
		}
	}()

	_, err = tempFile.Write(keyPEM)
	assert.NoError(suite.T(), err)
	err = tempFile.Close()
	assert.NoError(suite.T(), err)

	pkiMock := pkimock.NewPKIServiceInterfaceMock(suite.T())
	pkiMock.EXPECT().GetPrivateKey(mock.Anything).Return(ecKey, nil)
	pkiMock.EXPECT().GetCertThumbprint(mock.Anything).Return("test-kid")

	service, err := Initialize(pkiMock)

	assert.NoError(suite.T(), err)

	// Cast to access internal fields for testing
	jwtSvc, ok := service.(*jwtService)
	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), cryptolab.ECDSASHA256, jwtSvc.signAlg)
	assert.Equal(suite.T(), jws.ES256, jwtSvc.jwsAlg)
}

func (suite *JWTServiceTestSuite) TestInitWithUnsupportedECCurve() {
	// Generate P-224 key (unsupported curve)
	ecKey, err := ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
	assert.NoError(suite.T(), err)

	ecBytes, err := x509.MarshalECPrivateKey(ecKey)
	assert.NoError(suite.T(), err)

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: ecBytes,
	})

	tempFile, err := os.CreateTemp("", "ec_unsupported_*.pem")
	assert.NoError(suite.T(), err)
	defer func() {
		if err := os.Remove(tempFile.Name()); err != nil {
			suite.T().Logf("Failed to remove temp file: %v", err)
		}
	}()

	_, err = tempFile.Write(keyPEM)
	assert.NoError(suite.T(), err)
	pkiMock := pkimock.NewPKIServiceInterfaceMock(suite.T())
	testErr := serviceerror.CustomServiceError(serviceerror.InternalServerError, core.I18nMessage{
		Key:          "error.test.jwt_unsupported_ec_curve",
		DefaultValue: "unsupported EC curve",
	})
	pkiMock.EXPECT().GetPrivateKey(mock.Anything).Return(nil, testErr)
	_, err = Initialize(pkiMock)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to retrieve private key")
}

func (suite *JWTServiceTestSuite) TestJWKToPublicKeyErrorCases() {
	testCases := []struct {
		name          string
		jwk           map[string]interface{}
		errorContains string
	}{
		{
			name:          "MissingKty",
			jwk:           map[string]interface{}{},
			errorContains: "JWK missing kty",
		},
		{
			name:          "InvalidKty",
			jwk:           map[string]interface{}{"kty": 123},
			errorContains: "JWK missing kty",
		},
		{
			name:          "UnsupportedKty",
			jwk:           map[string]interface{}{"kty": "oct"},
			errorContains: "unsupported JWK kty",
		},
		{
			name:          "RSA_MissingModulus",
			jwk:           map[string]interface{}{"kty": "RSA", "e": "AQAB"},
			errorContains: "JWK missing RSA modulus or exponent",
		},
		{
			name:          "RSA_MissingExponent",
			jwk:           map[string]interface{}{"kty": "RSA", "n": "test"},
			errorContains: "JWK missing RSA modulus or exponent",
		},
		{
			name:          "RSA_InvalidModulus",
			jwk:           map[string]interface{}{"kty": "RSA", "n": "invalid!base64", "e": "AQAB"},
			errorContains: "failed to decode RSA modulus",
		},
		{
			name:          "RSA_InvalidExponent",
			jwk:           map[string]interface{}{"kty": "RSA", "n": "AQAB", "e": "invalid!base64"},
			errorContains: "failed to decode RSA exponent",
		},
		{
			name:          "EC_MissingCurve",
			jwk:           map[string]interface{}{"kty": "EC", "x": "test", "y": "test"},
			errorContains: "JWK missing EC parameters",
		},
		{
			name:          "EC_MissingX",
			jwk:           map[string]interface{}{"kty": "EC", "crv": "P-256", "y": "test"},
			errorContains: "JWK missing EC parameters",
		},
		{
			name:          "EC_MissingY",
			jwk:           map[string]interface{}{"kty": "EC", "crv": "P-256", "x": "test"},
			errorContains: "JWK missing EC parameters",
		},
		{
			name:          "EC_UnsupportedCurve",
			jwk:           map[string]interface{}{"kty": "EC", "crv": "P-224", "x": "test", "y": "test"},
			errorContains: "unsupported EC curve",
		},
		{
			name:          "EC_InvalidX",
			jwk:           map[string]interface{}{"kty": "EC", "crv": "P-256", "x": "invalid!base64", "y": "AQAB"},
			errorContains: "failed to decode EC x",
		},
		{
			name:          "EC_InvalidY",
			jwk:           map[string]interface{}{"kty": "EC", "crv": "P-256", "x": "AQAB", "y": "invalid!base64"},
			errorContains: "failed to decode EC y",
		},
		{
			name: "EC_InvalidXLength",
			jwk: map[string]interface{}{
				"kty": "EC", "crv": "P-256",
				"x": base64.RawURLEncoding.EncodeToString([]byte{1}),        // 1 byte
				"y": base64.RawURLEncoding.EncodeToString(make([]byte, 32)), // 32 bytes
			},
			errorContains: "invalid EC coordinate length",
		},
		{
			name: "EC_InvalidYLength",
			jwk: map[string]interface{}{
				"kty": "EC", "crv": "P-256",
				"x": base64.RawURLEncoding.EncodeToString(make([]byte, 32)), // 32 bytes
				"y": base64.RawURLEncoding.EncodeToString([]byte{1}),        // 1 byte
			},
			errorContains: "invalid EC coordinate length",
		},
		{
			name: "EC_PointNotOnCurve",
			jwk: map[string]interface{}{
				"kty": "EC", "crv": "P-256",
				"x": base64.RawURLEncoding.EncodeToString(make([]byte, 32)), // 32 zero bytes
				"y": base64.RawURLEncoding.EncodeToString(make([]byte, 32)), // 32 zero bytes
			},
			errorContains: "point not on curve",
		},
		{
			name:          "OKP_MissingCurve",
			jwk:           map[string]interface{}{"kty": "OKP", "x": "test"},
			errorContains: "JWK missing OKP parameters",
		},
		{
			name:          "OKP_MissingX",
			jwk:           map[string]interface{}{"kty": "OKP", "crv": "Ed25519"},
			errorContains: "JWK missing OKP parameters",
		},
		{
			name:          "OKP_UnsupportedCurve",
			jwk:           map[string]interface{}{"kty": "OKP", "crv": "Ed448", "x": "test"},
			errorContains: "unsupported OKP curve",
		},
		{
			name:          "OKP_InvalidX",
			jwk:           map[string]interface{}{"kty": "OKP", "crv": "Ed25519", "x": "invalid!base64"},
			errorContains: "failed to decode Ed25519 x",
		},
		{
			name: "OKP_InvalidKeyLength",
			jwk: map[string]interface{}{
				"kty": "OKP", "crv": "Ed25519", "x": base64.RawURLEncoding.EncodeToString([]byte("short")),
			},
			errorContains: "invalid Ed25519 public key length",
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			_, err := jws.JWKToPublicKey(tc.jwk)
			assert.NotNil(t, err)
			assert.Contains(t, err.Error(), tc.errorContains)
		})
	}
}

func (suite *JWTServiceTestSuite) TestVerifyJWTSignatureWithPublicKeyAlgorithmDetection() {
	// Test that VerifyJWTSignatureWithPublicKey correctly detects algorithm from header
	testCases := []struct {
		name        string
		setupKey    func() (crypto.PrivateKey, crypto.PublicKey, cryptolab.SignAlgorithm, jws.Algorithm)
		expectError bool
	}{
		{
			name: "jws.RS256Token",
			setupKey: func() (crypto.PrivateKey, crypto.PublicKey, cryptolab.SignAlgorithm, jws.Algorithm) {
				key, _ := rsa.GenerateKey(rand.Reader, 2048)
				return key, &key.PublicKey, cryptolab.RSASHA256, jws.RS256
			},
			expectError: false,
		},
		{
			name: "jws.ES256Token",
			setupKey: func() (crypto.PrivateKey, crypto.PublicKey, cryptolab.SignAlgorithm, jws.Algorithm) {
				key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
				return key, &key.PublicKey, cryptolab.ECDSASHA256, jws.ES256
			},
			expectError: false,
		},
		{
			name: "jws.EdDSAToken",
			setupKey: func() (crypto.PrivateKey, crypto.PublicKey, cryptolab.SignAlgorithm, jws.Algorithm) {
				_, priv, _ := ed25519.GenerateKey(rand.Reader)
				return priv, priv.Public(), cryptolab.ED25519, jws.EdDSA
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			priv, pub, signAlg, jwsAlg := tc.setupKey()
			cryptoMock := cryptomock.NewRuntimeCryptoProviderMock(t)
			keyRef := kmprovider.KeyRef{KeyID: "test-sign-key"}
			cryptoMock.EXPECT().Sign(mock.Anything, keyRef, signAlg, mock.Anything).
				RunAndReturn(func(
					_ context.Context, _ kmprovider.KeyRef, _ cryptolab.SignAlgorithm, content []byte,
				) ([]byte, error) {
					return cryptolab.Generate(content, signAlg, priv)
				}).Maybe()
			jwtService := &jwtService{
				cryptoProvider: cryptoMock,
				keyRef:         keyRef,
				publicKey:      pub,
				signAlg:        signAlg,
				jwsAlg:         jwsAlg,
			}

			// Generate token
			token, _, err := jwtService.GenerateJWT(context.Background(),
				"test-sub", "test-iss", 3600, map[string]interface{}{"aud": "test-aud"}, TokenTypeJWT, "")
			assert.Nil(t, err)

			// Verify with public key (should detect algorithm from header)
			err = jwtService.VerifyJWTSignatureWithPublicKey(token, pub)
			if tc.expectError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func (suite *JWTServiceTestSuite) TestVerifyJWTWithLeeway() {
	// Test that leeway is applied correctly to time-based claims
	testCases := []struct {
		name          string
		setupFunc     func() (string, string, string)
		setupConfig   func()
		expectError   bool
		expectedError serviceerror.ServiceError
	}{
		{
			name: "TokenExpiredWithinLeeway_ShouldPass",
			setupFunc: func() (string, string, string) {
				aud := testAudience
				iss := testIssuer
				// Token expired 10 seconds ago, but leeway is 30 seconds
				expiredTime := time.Now().Add(-10 * time.Second).Unix()
				token := suite.createBasicJWT(aud, iss, expiredTime, time.Now().Add(-time.Hour).Unix())
				return token, aud, iss
			},
			setupConfig: func() {
				// Leeway of 30 seconds is already configured in SetupTest
			},
			expectError: false,
		},
		{
			name: "TokenExpiredBeyondLeeway_ShouldFail",
			setupFunc: func() (string, string, string) {
				aud := testAudience
				iss := testIssuer
				// Token expired 60 seconds ago, leeway is 30 seconds
				expiredTime := time.Now().Add(-60 * time.Second).Unix()
				token := suite.createBasicJWT(aud, iss, expiredTime, time.Now().Add(-time.Hour).Unix())
				return token, aud, iss
			},
			setupConfig: func() {
				// Leeway of 30 seconds is already configured in SetupTest
			},
			expectError:   true,
			expectedError: ErrorTokenExpired,
		},
		{
			name: "TokenNbfInFutureWithinLeeway_ShouldPass",
			setupFunc: func() (string, string, string) {
				aud := testAudience
				iss := testIssuer
				// Token nbf is 10 seconds in the future, but leeway is 30 seconds
				nbfTime := time.Now().Add(10 * time.Second).Unix()
				token := suite.createBasicJWT(aud, iss, time.Now().Add(time.Hour).Unix(), nbfTime)
				return token, aud, iss
			},
			setupConfig: func() {
				// Leeway of 30 seconds is already configured in SetupTest
			},
			expectError: false,
		},
		{
			name: "TokenNbfInFutureBeyondLeeway_ShouldFail",
			setupFunc: func() (string, string, string) {
				aud := testAudience
				iss := testIssuer
				// Token nbf is 60 seconds in the future, leeway is 30 seconds
				nbfTime := time.Now().Add(60 * time.Second).Unix()
				token := suite.createBasicJWT(aud, iss, time.Now().Add(time.Hour).Unix(), nbfTime)
				return token, aud, iss
			},
			setupConfig: func() {
				// Leeway of 30 seconds is already configured in SetupTest
			},
			expectError:   true,
			expectedError: ErrorInvalidJWTFormat,
		},
		{
			name: "TokenExpiredExactlyAtLeewayBoundary_ShouldFail",
			setupFunc: func() (string, string, string) {
				aud := testAudience
				iss := testIssuer
				// Token expired exactly 31 seconds ago (just beyond 30s leeway)
				expiredTime := time.Now().Add(-31 * time.Second).Unix()
				token := suite.createBasicJWT(aud, iss, expiredTime, time.Now().Add(-time.Hour).Unix())
				return token, aud, iss
			},
			setupConfig: func() {
				// Leeway of 30 seconds is already configured in SetupTest
			},
			expectError:   true,
			expectedError: ErrorTokenExpired,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			tc.setupConfig()
			token, expectedAud, expectedIss := tc.setupFunc()

			err := suite.jwtService.VerifyJWT(token, expectedAud, expectedIss)

			if tc.expectError {
				assert.NotNil(t, err)
				assert.Equal(t, tc.expectedError, *err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func (suite *JWTServiceTestSuite) TestVerifyJWTWithZeroLeeway() {
	// Test behavior when leeway is set to 0
	config.ResetServerRuntime()
	testConfig := &config.Config{
		TLS: config.TLSConfig{
			KeyFile: suite.testKeyPath,
		},
		JWT: config.JWTConfig{
			Issuer:         "https://auth.example.com",
			ValidityPeriod: 3600,
			PreferredKeyID: "test-kid",
			Leeway:         0, // No leeway
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

	// Token expired 1 second ago should fail with zero leeway
	expiredTime := time.Now().Add(-1 * time.Second).Unix()
	token := suite.createBasicJWT(testAudience, testIssuer, expiredTime, time.Now().Add(-time.Hour).Unix())

	svcErr := suite.jwtService.VerifyJWT(token, testAudience, testIssuer)
	assert.NotNil(suite.T(), svcErr)
	assert.Equal(suite.T(), ErrorTokenExpired, *svcErr)
}
