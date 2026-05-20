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

package userinfo

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	appmodel "github.com/thunder-id/thunderid/internal/application/model"
	certmodel "github.com/thunder-id/thunderid/internal/cert"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/jwksresolver"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/jose/jwe"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwemock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
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
		JWT: config.JWTConfig{Issuer: "test-issuer", ValidityPeriod: 600},
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
		mock.Anything, mock.Anything,
		jwe.KeyEncAlgorithm("RSA-OAEP-256"),
		jwe.ContentEncAlgorithm("A256GCM"),
		"json",
		"",
	).Return("compact.jwe.token", (*serviceerror.ServiceError)(nil))

	svc := &userInfoService{
		jweService:   mockJWE,
		jwksResolver: jwksresolver.Initialize(nil),
		logger:       log.GetLogger(),
	}
	cfg := &inboundmodel.UserInfoConfig{EncryptionAlg: "RSA-OAEP-256", EncryptionEnc: "A256GCM"}
	cert := &appmodel.ApplicationCertificate{Type: certmodel.CertificateTypeJWKS, Value: pubJWKS}

	result, svcErr := svc.generateJWEUserInfo(context.Background(), map[string]interface{}{"sub": "user1"}, cfg, cert)
	assert.Nil(s.T(), svcErr)
	assert.NotNil(s.T(), result)
	assert.Equal(s.T(), inboundmodel.UserInfoResponseTypeJWE, result.Type)
	assert.Equal(s.T(), "compact.jwe.token", result.JWTBody)
}

// TestGenerateJWEUserInfo_NoCert verifies missing cert returns server error.
func (s *JWEUserInfoTestSuite) TestGenerateJWEUserInfo_NoCert() {
	svc := &userInfoService{
		jweService:   jwemock.NewJWEServiceInterfaceMock(s.T()),
		jwksResolver: jwksresolver.Initialize(nil),
		logger:       log.GetLogger(),
	}
	cfg := &inboundmodel.UserInfoConfig{EncryptionAlg: "RSA-OAEP-256", EncryptionEnc: "A256GCM"}

	result, svcErr := svc.generateJWEUserInfo(context.Background(), map[string]interface{}{"sub": "user1"}, cfg, nil)
	assert.Nil(s.T(), result)
	assert.NotNil(s.T(), svcErr)
	assert.Equal(s.T(), serviceerror.InternalServerError.Code, svcErr.Code)
}

// TestGenerateJWEUserInfo_EncryptFailure verifies JWE encryption failure returns server error.
func (s *JWEUserInfoTestSuite) TestGenerateJWEUserInfo_EncryptFailure() {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	pubJWKS := rsaPublicKeyToJWKS(&privateKey.PublicKey)

	mockJWE := jwemock.NewJWEServiceInterfaceMock(s.T())
	mockJWE.On("Encrypt",
		mock.Anything, mock.Anything,
		jwe.KeyEncAlgorithm("RSA-OAEP-256"),
		jwe.ContentEncAlgorithm("A256GCM"),
		"json",
		"",
	).Return("", &serviceerror.InternalServerError)

	svc := &userInfoService{
		jweService:   mockJWE,
		jwksResolver: jwksresolver.Initialize(nil),
		logger:       log.GetLogger(),
	}
	cfg := &inboundmodel.UserInfoConfig{EncryptionAlg: "RSA-OAEP-256", EncryptionEnc: "A256GCM"}
	cert := &appmodel.ApplicationCertificate{Type: certmodel.CertificateTypeJWKS, Value: pubJWKS}

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
	).Return("signed.jwt.token", int64(0), (*serviceerror.ServiceError)(nil))

	mockJWE := jwemock.NewJWEServiceInterfaceMock(s.T())
	mockJWE.On("Encrypt",
		mock.Anything, mock.Anything,
		jwe.KeyEncAlgorithm("RSA-OAEP-256"),
		jwe.ContentEncAlgorithm("A256GCM"),
		"JWT",
		"",
	).Return("nested.jwe.token", (*serviceerror.ServiceError)(nil))

	svc := &userInfoService{
		jwtService:   mockJWT,
		jweService:   mockJWE,
		jwksResolver: jwksresolver.Initialize(nil),
		logger:       log.GetLogger(),
	}

	cfg := &inboundmodel.UserInfoConfig{SigningAlg: "RS256", EncryptionAlg: "RSA-OAEP-256", EncryptionEnc: "A256GCM"}
	cert := &appmodel.ApplicationCertificate{Type: certmodel.CertificateTypeJWKS, Value: pubJWKS}

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
	assert.Equal(s.T(), inboundmodel.UserInfoResponseTypeNESTEDJWT, result.Type)
	assert.Equal(s.T(), "nested.jwe.token", result.JWTBody)
}

// TestGenerateJWEUserInfo_EncryptErrorPropagated verifies that the exact error from Encrypt is returned,
// not a generic InternalServerError.
func (s *JWEUserInfoTestSuite) TestGenerateJWEUserInfo_EncryptErrorPropagated() {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	pubJWKS := rsaPublicKeyToJWKS(&privateKey.PublicKey)

	mockJWE := jwemock.NewJWEServiceInterfaceMock(s.T())
	unsupportedErr := &serviceerror.ServiceError{Code: "JWE-1003", Type: serviceerror.ClientErrorType}
	mockJWE.On("Encrypt",
		mock.Anything, mock.Anything,
		jwe.KeyEncAlgorithm("RSA-OAEP-256"),
		jwe.ContentEncAlgorithm("A256GCM"),
		"json",
		"",
	).Return("", unsupportedErr)

	svc := &userInfoService{
		jweService:   mockJWE,
		jwksResolver: jwksresolver.Initialize(nil),
		logger:       log.GetLogger(),
	}
	cfg := &inboundmodel.UserInfoConfig{EncryptionAlg: "RSA-OAEP-256", EncryptionEnc: "A256GCM"}
	cert := &appmodel.ApplicationCertificate{Type: certmodel.CertificateTypeJWKS, Value: pubJWKS}

	result, svcErr := svc.generateJWEUserInfo(context.Background(), map[string]interface{}{"sub": "user1"}, cfg, cert)
	assert.Nil(s.T(), result)
	assert.NotNil(s.T(), svcErr)
	assert.Equal(s.T(), "JWE-1003", svcErr.Code)
}

// TestGenerateJWSUserInfo_UnsupportedAlg verifies that an algorithm incompatible with the server key
// returns InternalServerError (server misconfiguration, not a client auth error).
func (s *JWEUserInfoTestSuite) TestGenerateJWSUserInfo_UnsupportedAlg() {
	mockJWT := jwtmock.NewJWTServiceInterfaceMock(s.T())
	mockJWT.On("GenerateJWT",
		mock.Anything, "user1", "test-issuer", int64(600),
		mock.Anything, mock.Anything, "ES256",
	).Return("", int64(0), &jwt.ErrorUnsupportedJWSAlgorithm)

	svc := &userInfoService{jwtService: mockJWT, logger: log.GetLogger()}
	cfg := &inboundmodel.UserInfoConfig{SigningAlg: "ES256"}

	result, svcErr := svc.generateJWSUserInfo(
		context.Background(),
		"user1",
		map[string]interface{}{"client_id": "client1"},
		map[string]interface{}{"sub": "user1"},
		cfg,
	)
	assert.Nil(s.T(), result)
	assert.NotNil(s.T(), svcErr)
	assert.Equal(s.T(), serviceerror.InternalServerError.Code, svcErr.Code)
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
