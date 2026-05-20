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

package auth

import (
	"context"
	"crypto"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	i18ncore "github.com/thunder-id/thunderid/internal/system/i18n/core"
)

const (
	testIssuer = "https://localhost:8090"
	testMCPURL = "https://localhost:8090/mcp"
)

// MockJWTService is a mock implementation of jwt.JWTServiceInterface
type MockJWTService struct {
	mock.Mock
}

func (m *MockJWTService) GetPublicKey() crypto.PublicKey {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(crypto.PublicKey)
}

func (m *MockJWTService) GenerateJWT(
	ctx context.Context,
	sub, iss string,
	validityPeriod int64,
	claims map[string]interface{},
	typ, alg string,
) (string, int64, *serviceerror.ServiceError) {
	args := m.Called(ctx, sub, iss, validityPeriod, claims, typ, alg)
	return args.String(0), args.Get(1).(int64), args.Get(2).(*serviceerror.ServiceError)
}

func (m *MockJWTService) VerifyJWT(
	jwtToken string,
	expectedAud string,
	expectedIss string,
) *serviceerror.ServiceError {
	args := m.Called(jwtToken, expectedAud, expectedIss)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*serviceerror.ServiceError)
}

func (m *MockJWTService) VerifyJWTWithPublicKey(
	jwtToken string,
	jwtPublicKey crypto.PublicKey,
	expectedAud string,
	expectedIss string,
) *serviceerror.ServiceError {
	args := m.Called(jwtToken, jwtPublicKey, expectedAud, expectedIss)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*serviceerror.ServiceError)
}

func (m *MockJWTService) VerifyJWTWithJWKS(
	jwtToken string,
	jwksURL string,
	expectedAud string,
	expectedIss string,
) *serviceerror.ServiceError {
	args := m.Called(jwtToken, jwksURL, expectedAud, expectedIss)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*serviceerror.ServiceError)
}

func (m *MockJWTService) VerifyJWTSignature(jwtToken string) *serviceerror.ServiceError {
	args := m.Called(jwtToken)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*serviceerror.ServiceError)
}

func (m *MockJWTService) VerifyJWTSignatureWithPublicKey(
	jwtToken string,
	jwtPublicKey crypto.PublicKey,
) *serviceerror.ServiceError {
	args := m.Called(jwtToken, jwtPublicKey)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*serviceerror.ServiceError)
}

func (m *MockJWTService) VerifyJWTSignatureWithJWKS(
	jwtToken string,
	jwksURL string,
) *serviceerror.ServiceError {
	args := m.Called(jwtToken, jwksURL)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*serviceerror.ServiceError)
}

type TokenVerifierTestSuite struct {
	suite.Suite
}

func TestTokenVerifierTestSuite(t *testing.T) {
	suite.Run(t, new(TokenVerifierTestSuite))
}

func (suite *TokenVerifierTestSuite) TestNewTokenVerifier_Success() {
	mockJWTService := new(MockJWTService)
	issuer := testIssuer
	mcpURL := testMCPURL

	// Create test JWT payload
	now := time.Now().Unix()
	payload := map[string]interface{}{
		"sub":   "user123",
		"exp":   float64(now + 3600),
		"scope": "openid profile email",
	}
	payloadJSON, _ := json.Marshal(payload)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)
	testToken := "header." + payloadB64 + ".signature"

	// Mock JWT verification to succeed
	mockJWTService.On("VerifyJWT", testToken, mcpURL, issuer).Return(nil)

	// Create token verifier
	verifier := NewTokenVerifier(mockJWTService, issuer, mcpURL)

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/mcp/tools", nil)
	ctx := context.Background()

	// Call verifier
	tokenInfo, err := verifier(ctx, testToken, req)

	// Assertions
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), tokenInfo)
	assert.Equal(suite.T(), "user123", tokenInfo.UserID)
	assert.Contains(suite.T(), tokenInfo.Scopes, "openid")
	assert.Contains(suite.T(), tokenInfo.Scopes, "profile")
	assert.Contains(suite.T(), tokenInfo.Scopes, "email")
	assert.False(suite.T(), tokenInfo.Expiration.IsZero())

	mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenVerifierTestSuite) TestNewTokenVerifier_JWTVerificationFailed() {
	mockJWTService := new(MockJWTService)
	issuer := testIssuer
	mcpURL := testMCPURL
	testToken := "invalid.token.here"

	// Mock JWT verification to fail
	mockJWTService.On("VerifyJWT", testToken, mcpURL, issuer).Return(&serviceerror.ServiceError{
		ErrorDescription: i18ncore.I18nMessage{DefaultValue: "invalid token"},
	})

	// Create token verifier
	verifier := NewTokenVerifier(mockJWTService, issuer, mcpURL)

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/mcp/tools", nil)
	ctx := context.Background()

	// Call verifier
	tokenInfo, err := verifier(ctx, testToken, req)

	// Assertions
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), tokenInfo)
	assert.Equal(suite.T(), auth.ErrInvalidToken, err)

	mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenVerifierTestSuite) TestNewTokenVerifier_InvalidPayload() {
	mockJWTService := new(MockJWTService)
	issuer := testIssuer
	mcpURL := testMCPURL

	// Create invalid JWT payload (not base64)
	testToken := "header.invalid-payload.signature"

	// Mock JWT verification to succeed
	mockJWTService.On("VerifyJWT", testToken, mcpURL, issuer).Return(nil)

	// Create token verifier
	verifier := NewTokenVerifier(mockJWTService, issuer, mcpURL)

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/mcp/tools", nil)
	ctx := context.Background()

	// Call verifier
	tokenInfo, err := verifier(ctx, testToken, req)

	// Assertions
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), tokenInfo)
	assert.Equal(suite.T(), auth.ErrInvalidToken, err)

	mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenVerifierTestSuite) TestNewTokenVerifier_NoScopes() {
	mockJWTService := new(MockJWTService)
	issuer := testIssuer
	mcpURL := testMCPURL

	// Create test JWT payload without scopes
	now := time.Now().Unix()
	payload := map[string]interface{}{
		"sub": "user123",
		"exp": float64(now + 3600),
	}
	payloadJSON, _ := json.Marshal(payload)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)
	testToken := "header." + payloadB64 + ".signature"

	// Mock JWT verification to succeed
	mockJWTService.On("VerifyJWT", testToken, mcpURL, issuer).Return(nil)

	// Create token verifier
	verifier := NewTokenVerifier(mockJWTService, issuer, mcpURL)

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/mcp/tools", nil)
	ctx := context.Background()

	// Call verifier
	tokenInfo, err := verifier(ctx, testToken, req)

	// Assertions
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), tokenInfo)
	assert.Equal(suite.T(), "user123", tokenInfo.UserID)
	assert.Empty(suite.T(), tokenInfo.Scopes)

	mockJWTService.AssertExpectations(suite.T())
}

func (suite *TokenVerifierTestSuite) TestNewTokenVerifier_EmptyUserID() {
	mockJWTService := new(MockJWTService)
	issuer := testIssuer
	mcpURL := testMCPURL

	// Create test JWT payload without sub claim
	now := time.Now().Unix()
	payload := map[string]interface{}{
		"exp":   float64(now + 3600),
		"scope": "openid",
	}
	payloadJSON, _ := json.Marshal(payload)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)
	testToken := "header." + payloadB64 + ".signature"

	// Mock JWT verification to succeed
	mockJWTService.On("VerifyJWT", testToken, mcpURL, issuer).Return(nil)

	// Create token verifier
	verifier := NewTokenVerifier(mockJWTService, issuer, mcpURL)

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/mcp/tools", nil)
	ctx := context.Background()

	// Call verifier
	tokenInfo, err := verifier(ctx, testToken, req)

	// Assertions
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), tokenInfo)
	assert.Equal(suite.T(), "", tokenInfo.UserID)
	assert.Contains(suite.T(), tokenInfo.Scopes, "openid")

	mockJWTService.AssertExpectations(suite.T())
}
