// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package userinfo

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"testing"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	certmodel "github.com/thunder-id/thunderid/internal/cert"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/jwksresolver"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/jose/jwe"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwemock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
)

const (
	testRawJWSToken = "header.payload.signature"
	testRawJWEToken = "header.encryptedKey.iv.ciphertext.tag"
)

// JWEUserInfoTestSuite defines the test suite for JWE/JWS userinfo generation.
type JWEUserInfoTestSuite struct {
	suite.Suite
}

// TestJWEUserInfoSuite runs the JWE userinfo test suite.
func TestJWEUserInfoSuite(t *testing.T) {
	suite.Run(t, new(JWEUserInfoTestSuite))
}

func (s *JWEUserInfoTestSuite) SetupTest() {
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("test-home", &config.Config{
		JWT: engineconfig.JWTConfig{Issuer: "test-issuer", ValidityPeriod: 600},
	})
}

func (s *JWEUserInfoTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

// TestGenerateJWEUserInfo_Success verifies a JWE response from an inline JWKS.
func (s *JWEUserInfoTestSuite) TestGenerateJWEUserInfo_Success() {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	pubJWKS := rsaPublicKeyToJWKS(&privateKey.PublicKey)

	mockJWE := jwemock.NewJWEServiceInterfaceMock(s.T())
	mockJWE.On("Encrypt",
		mock.Anything, mock.Anything, mock.Anything,
		"RSA-OAEP-256",
		jwe.ContentEncAlgorithm("A256GCM"),
		"json",
		"",
	).Return("compact.jwe.token", (*tidcommon.ServiceError)(nil))

	svc := &userInfoService{
		cfg:          userInfoTestConfig(),
		jweService:   mockJWE,
		jwksResolver: jwksresolver.Initialize(nil),
		logger:       log.GetLogger(),
	}
	cfg := &providers.UserInfoConfig{EncryptionAlg: "RSA-OAEP-256", EncryptionEnc: "A256GCM"}
	cert := &providers.Certificate{Type: certmodel.CertificateTypeJWKS, Value: pubJWKS}

	result, svcErr := svc.generateJWEUserInfo(context.Background(), map[string]interface{}{"sub": "user1"}, cfg, cert)
	assert.Nil(s.T(), svcErr)
	assert.NotNil(s.T(), result)
	assert.Equal(s.T(), providers.UserInfoResponseTypeJWE, result.Type)
	assert.Equal(s.T(), "compact.jwe.token", result.JWTBody)
}

// TestGenerateJWEUserInfo_NoCert verifies missing cert returns server error.
func (s *JWEUserInfoTestSuite) TestGenerateJWEUserInfo_NoCert() {
	svc := &userInfoService{
		cfg:          userInfoTestConfig(),
		jweService:   jwemock.NewJWEServiceInterfaceMock(s.T()),
		jwksResolver: jwksresolver.Initialize(nil),
		logger:       log.GetLogger(),
	}
	cfg := &providers.UserInfoConfig{EncryptionAlg: "RSA-OAEP-256", EncryptionEnc: "A256GCM"}

	result, svcErr := svc.generateJWEUserInfo(context.Background(), map[string]interface{}{"sub": "user1"}, cfg, nil)
	assert.Nil(s.T(), result)
	assert.NotNil(s.T(), svcErr)
	assert.Equal(s.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

// TestGenerateJWEUserInfo_EncryptFailure verifies JWE encryption failure returns server error.
func (s *JWEUserInfoTestSuite) TestGenerateJWEUserInfo_EncryptFailure() {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	pubJWKS := rsaPublicKeyToJWKS(&privateKey.PublicKey)

	mockJWE := jwemock.NewJWEServiceInterfaceMock(s.T())
	mockJWE.On("Encrypt",
		mock.Anything, mock.Anything, mock.Anything,
		"RSA-OAEP-256",
		jwe.ContentEncAlgorithm("A256GCM"),
		"json",
		"",
	).Return("", &tidcommon.InternalServerError)

	svc := &userInfoService{
		cfg:          userInfoTestConfig(),
		jweService:   mockJWE,
		jwksResolver: jwksresolver.Initialize(nil),
		logger:       log.GetLogger(),
	}
	cfg := &providers.UserInfoConfig{EncryptionAlg: "RSA-OAEP-256", EncryptionEnc: "A256GCM"}
	cert := &providers.Certificate{Type: certmodel.CertificateTypeJWKS, Value: pubJWKS}

	result, svcErr := svc.generateJWEUserInfo(context.Background(), map[string]interface{}{"sub": "user1"}, cfg, cert)
	assert.Nil(s.T(), result)
	assert.NotNil(s.T(), svcErr)
}

// TestGenerateNestedJWTUserInfo_Success verifies a sign-then-encrypt nested JWT.
func (s *JWEUserInfoTestSuite) TestGenerateNestedJWTUserInfo_Success() {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	pubJWKS := rsaPublicKeyToJWKS(&privateKey.PublicKey)

	mockJWT := jwtmock.NewJWTServiceInterfaceMock(s.T())
	mockJWT.On("GenerateJWT",
		mock.Anything, "user1", "test-issuer", int64(600),
		mock.Anything, mock.Anything, "RS256",
	).Return("signed.jwt.token", int64(0), (*tidcommon.ServiceError)(nil))

	mockJWE := jwemock.NewJWEServiceInterfaceMock(s.T())
	mockJWE.On("Encrypt",
		mock.Anything, mock.Anything, mock.Anything,
		"RSA-OAEP-256",
		jwe.ContentEncAlgorithm("A256GCM"),
		"JWT",
		"",
	).Return("nested.jwe.token", (*tidcommon.ServiceError)(nil))

	svc := &userInfoService{
		cfg:          userInfoTestConfig(),
		jwtService:   mockJWT,
		jweService:   mockJWE,
		jwksResolver: jwksresolver.Initialize(nil),
		logger:       log.GetLogger(),
	}

	cfg := &providers.UserInfoConfig{SigningAlg: "RS256", EncryptionAlg: "RSA-OAEP-256", EncryptionEnc: "A256GCM"}
	cert := &providers.Certificate{Type: certmodel.CertificateTypeJWKS, Value: pubJWKS}

	result, svcErr := svc.generateNestedJWTUserInfo(
		context.Background(),
		"user1",
		map[string]interface{}{"client_id": "client1"},
		map[string]interface{}{"sub": "user1"},
		cfg,
		cert,
	)
	assert.Nil(s.T(), svcErr)
	assert.NotNil(s.T(), result)
	assert.Equal(s.T(), providers.UserInfoResponseTypeNESTEDJWT, result.Type)
	assert.Equal(s.T(), "nested.jwe.token", result.JWTBody)
}

// TestGenerateJWEUserInfo_EncryptErrorPropagated verifies that the exact error from Encrypt is returned,
// not a generic InternalServerError.
func (s *JWEUserInfoTestSuite) TestGenerateJWEUserInfo_EncryptErrorPropagated() {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	pubJWKS := rsaPublicKeyToJWKS(&privateKey.PublicKey)

	mockJWE := jwemock.NewJWEServiceInterfaceMock(s.T())
	unsupportedErr := &tidcommon.ServiceError{Code: "JWE-1003", Type: tidcommon.ClientErrorType}
	mockJWE.On("Encrypt",
		mock.Anything, mock.Anything, mock.Anything,
		"RSA-OAEP-256",
		jwe.ContentEncAlgorithm("A256GCM"),
		"json",
		"",
	).Return("", unsupportedErr)

	svc := &userInfoService{
		cfg:          userInfoTestConfig(),
		jweService:   mockJWE,
		jwksResolver: jwksresolver.Initialize(nil),
		logger:       log.GetLogger(),
	}
	cfg := &providers.UserInfoConfig{EncryptionAlg: "RSA-OAEP-256", EncryptionEnc: "A256GCM"}
	cert := &providers.Certificate{Type: certmodel.CertificateTypeJWKS, Value: pubJWKS}

	result, svcErr := svc.generateJWEUserInfo(context.Background(), map[string]interface{}{"sub": "user1"}, cfg, cert)
	assert.Nil(s.T(), result)
	assert.NotNil(s.T(), svcErr)
	assert.Equal(s.T(), "JWE-1003", svcErr.Code)
}

// TestGenerateJWSUserInfo_UnsupportedAlg verifies that an algorithm with no matching signing key
// is reported as a client error, so the caller learns its registered algorithm is unusable
// instead of receiving an opaque 500.
func (s *JWEUserInfoTestSuite) TestGenerateJWSUserInfo_UnsupportedAlg() {
	mockJWT := jwtmock.NewJWTServiceInterfaceMock(s.T())
	mockJWT.On("GenerateJWT",
		mock.Anything, "user1", "test-issuer", int64(600),
		mock.Anything, mock.Anything, "ES256",
	).Return("", int64(0), &jwt.ErrorUnsupportedJWSAlgorithm)

	svc := &userInfoService{cfg: userInfoTestConfig(), jwtService: mockJWT, logger: log.GetLogger()}
	cfg := &providers.UserInfoConfig{SigningAlg: "ES256"}

	result, svcErr := svc.generateJWSUserInfo(
		context.Background(),
		"user1",
		map[string]interface{}{"client_id": "client1"},
		map[string]interface{}{"sub": "user1"},
		cfg,
	)
	assert.Nil(s.T(), result)
	assert.NotNil(s.T(), svcErr)
	assert.Equal(s.T(), errorUnsupportedSigningAlg.Code, svcErr.Code)
	assert.Equal(s.T(), tidcommon.ClientErrorType, svcErr.Type)
}

// TestBuildRawJWTResponse_PassesThroughWithoutEncrypting covers the cases where buildRawJWTResponse
// must not encrypt: the value is already a JWE (5 parts, regardless of config), or it's a signed
// JWT (3 parts) and the client's UserInfo config does not request encryption (including no config
// at all).
func (s *JWEUserInfoTestSuite) TestBuildRawJWTResponse_PassesThroughWithoutEncrypting() {
	testCases := []struct {
		name     string
		rawToken string
		cfg      *providers.UserInfoConfig
		wantType providers.UserInfoResponseType
	}{
		{
			name:     "already JWE, passthrough regardless of config",
			rawToken: testRawJWEToken,
			cfg:      &providers.UserInfoConfig{ResponseType: providers.UserInfoResponseTypeJWE},
			wantType: providers.UserInfoResponseTypeJWE,
		},
		{
			name:     "signed JWT, no encryption configured",
			rawToken: testRawJWSToken,
			cfg:      &providers.UserInfoConfig{ResponseType: providers.UserInfoResponseTypeJWS},
			wantType: providers.UserInfoResponseTypeJWS,
		},
		{
			name:     "signed JWT, nil UserInfo config",
			rawToken: testRawJWSToken,
			cfg:      nil,
			wantType: providers.UserInfoResponseTypeJWS,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			mockJWE := jwemock.NewJWEServiceInterfaceMock(s.T())

			svc := &userInfoService{
				cfg:          userInfoTestConfig(),
				jweService:   mockJWE,
				jwksResolver: jwksresolver.Initialize(nil),
				logger:       log.GetLogger(),
			}

			result, svcErr := svc.buildRawJWTResponse(context.Background(), tc.rawToken, tc.cfg, nil)

			assert.Nil(s.T(), svcErr)
			assert.NotNil(s.T(), result)
			assert.Equal(s.T(), tc.wantType, result.Type)
			assert.Equal(s.T(), tc.rawToken, result.JWTBody)
			mockJWE.AssertNotCalled(s.T(), "Encrypt", mock.Anything, mock.Anything, mock.Anything,
				mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

// TestBuildRawJWTResponse_JWSValue_ClientRequestsEncryption_EncryptsToNestedJWT verifies that a
// signed JWT is encrypted (not re-signed) into a nested JWT when the client's UserInfo config
// requests JWE or NESTED_JWT.
func (s *JWEUserInfoTestSuite) TestBuildRawJWTResponse_JWSValue_ClientRequestsEncryption_EncryptsToNestedJWT() {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	pubJWKS := rsaPublicKeyToJWKS(&privateKey.PublicKey)

	mockJWE := jwemock.NewJWEServiceInterfaceMock(s.T())
	mockJWE.On("Encrypt",
		mock.Anything, []byte(testRawJWSToken), mock.Anything,
		"RSA-OAEP-256",
		jwe.ContentEncAlgorithm("A256GCM"),
		"JWT",
		"",
	).Return("nested.jwe.token", (*tidcommon.ServiceError)(nil))

	svc := &userInfoService{
		cfg:          userInfoTestConfig(),
		jweService:   mockJWE,
		jwksResolver: jwksresolver.Initialize(nil),
		logger:       log.GetLogger(),
	}
	cfg := &providers.UserInfoConfig{
		ResponseType:  providers.UserInfoResponseTypeNESTEDJWT,
		EncryptionAlg: "RSA-OAEP-256",
		EncryptionEnc: "A256GCM",
	}
	cert := &providers.Certificate{Type: certmodel.CertificateTypeJWKS, Value: pubJWKS}

	result, svcErr := svc.buildRawJWTResponse(context.Background(), testRawJWSToken, cfg, cert)

	assert.Nil(s.T(), svcErr)
	assert.NotNil(s.T(), result)
	assert.Equal(s.T(), providers.UserInfoResponseTypeNESTEDJWT, result.Type)
	assert.Equal(s.T(), "nested.jwe.token", result.JWTBody)
}

// TestEncryptSignedJWT_ResolveKeyFailure verifies that a key resolution failure (e.g. no
// certificate configured) is returned as-is, without attempting to encrypt.
func (s *JWEUserInfoTestSuite) TestEncryptSignedJWT_ResolveKeyFailure() {
	mockJWE := jwemock.NewJWEServiceInterfaceMock(s.T())

	svc := &userInfoService{
		cfg:          userInfoTestConfig(),
		jweService:   mockJWE,
		jwksResolver: jwksresolver.Initialize(nil),
		logger:       log.GetLogger(),
	}
	cfg := &providers.UserInfoConfig{EncryptionAlg: "RSA-OAEP-256", EncryptionEnc: "A256GCM"}

	compact, svcErr := svc.encryptSignedJWT(context.Background(), testRawJWSToken, cfg, nil)

	assert.Empty(s.T(), compact)
	assert.NotNil(s.T(), svcErr)
	assert.Equal(s.T(), tidcommon.InternalServerError.Code, svcErr.Code)
	mockJWE.AssertNotCalled(s.T(), "Encrypt", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// TestEncryptSignedJWT_EncryptFailure verifies that a JWE encryption failure is propagated.
func (s *JWEUserInfoTestSuite) TestEncryptSignedJWT_EncryptFailure() {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	pubJWKS := rsaPublicKeyToJWKS(&privateKey.PublicKey)

	mockJWE := jwemock.NewJWEServiceInterfaceMock(s.T())
	mockJWE.On("Encrypt",
		mock.Anything, []byte(testRawJWSToken), mock.Anything,
		"RSA-OAEP-256",
		jwe.ContentEncAlgorithm("A256GCM"),
		"JWT",
		"",
	).Return("", &tidcommon.InternalServerError)

	svc := &userInfoService{
		cfg:          userInfoTestConfig(),
		jweService:   mockJWE,
		jwksResolver: jwksresolver.Initialize(nil),
		logger:       log.GetLogger(),
	}
	cfg := &providers.UserInfoConfig{EncryptionAlg: "RSA-OAEP-256", EncryptionEnc: "A256GCM"}
	cert := &providers.Certificate{Type: certmodel.CertificateTypeJWKS, Value: pubJWKS}

	compact, svcErr := svc.encryptSignedJWT(context.Background(), testRawJWSToken, cfg, cert)

	assert.Empty(s.T(), compact)
	assert.NotNil(s.T(), svcErr)
}

// TestBuildRawJWTResponse_EncryptionFailure_PropagatesError verifies that when the client requests
// encryption but the raw JWT can't be encrypted (e.g. no certificate configured), buildRawJWTResponse
// returns the error instead of a response.
func (s *JWEUserInfoTestSuite) TestBuildRawJWTResponse_EncryptionFailure_PropagatesError() {
	mockJWE := jwemock.NewJWEServiceInterfaceMock(s.T())

	svc := &userInfoService{
		cfg:          userInfoTestConfig(),
		jweService:   mockJWE,
		jwksResolver: jwksresolver.Initialize(nil),
		logger:       log.GetLogger(),
	}
	cfg := &providers.UserInfoConfig{
		ResponseType:  providers.UserInfoResponseTypeNESTEDJWT,
		EncryptionAlg: "RSA-OAEP-256",
		EncryptionEnc: "A256GCM",
	}

	result, svcErr := svc.buildRawJWTResponse(context.Background(), testRawJWSToken, cfg, nil)

	assert.Nil(s.T(), result)
	assert.NotNil(s.T(), svcErr)
	assert.Equal(s.T(), tidcommon.InternalServerError.Code, svcErr.Code)
}

// rsaPublicKeyToJWKS builds a minimal RSA JWKS JSON for tests.
func rsaPublicKeyToJWKS(pub *rsa.PublicKey) string {
	eBytes := big.NewInt(int64(pub.E)).Bytes()
	key := map[string]interface{}{
		"kty": "RSA",
		"use": "enc",
		"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(eBytes),
	}
	b, _ := json.Marshal(map[string]interface{}{"keys": []interface{}{key}})
	return string(b)
}
