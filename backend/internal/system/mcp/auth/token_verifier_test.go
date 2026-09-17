// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
)

const testMCPURL = "https://localhost:8090/mcp"

// fakeRevocationEnforcer is a minimal test double for security.RevocationEnforcerInterface — that
// interface has no mockery-generated mock exported outside the security package, so this is
// standalone rather than a generated mock.
type fakeRevocationEnforcer struct {
	err error
}

func (f *fakeRevocationEnforcer) EnsureNotRevoked(context.Context, security.RevocationIdentity) error {
	return f.err
}

type TokenVerifierTestSuite struct {
	suite.Suite
	mockJWT    *jwtmock.JWTServiceInterfaceMock
	revocation *fakeRevocationEnforcer
}

func (suite *TokenVerifierTestSuite) SetupTest() {
	suite.mockJWT = jwtmock.NewJWTServiceInterfaceMock(suite.T())
	suite.revocation = &fakeRevocationEnforcer{}
	// Empty runtime config so the token's absent "iss" claim ("") matches the self-issued branch
	// (Config.JWT.Issuer == ""), same setup the security package's own authenticator tests use.
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("", &config.Config{})
}

func (suite *TokenVerifierTestSuite) TearDownTest() {
	suite.mockJWT.AssertExpectations(suite.T())
	config.ResetServerRuntime()
}

func TestTokenVerifierTestSuite(t *testing.T) {
	suite.Run(t, new(TokenVerifierTestSuite))
}

// encodeTestToken builds a self-issued access token. The RFC 9068 typ header is required: a
// self-issued token that is not an access token is rejected before verification.
func encodeTestToken(payload map[string]interface{}) string {
	headerJSON, _ := json.Marshal(map[string]interface{}{"alg": "RS256", "typ": jwt.TokenTypeAccessToken})
	payloadJSON, _ := json.Marshal(payload)
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)
	return headerB64 + "." + payloadB64 + ".signature"
}

func (suite *TokenVerifierTestSuite) newVerifier() auth.TokenVerifier {
	bearerAuthenticator := security.NewBearerAuthenticator(suite.mockJWT, suite.revocation, testMCPURL)
	return NewTokenVerifier(bearerAuthenticator)
}

func (suite *TokenVerifierTestSuite) TestNewTokenVerifier_Success() {
	now := time.Now().Unix()
	testToken := encodeTestToken(map[string]interface{}{
		"sub":   "user123",
		"exp":   float64(now + 3600),
		"scope": "openid profile email",
	})

	suite.mockJWT.On("VerifyJWT", mock.Anything, testToken, testMCPURL, "").Return(nil)

	verifier := suite.newVerifier()
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	tokenInfo, err := verifier(context.Background(), testToken, req)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), tokenInfo)
	assert.Equal(suite.T(), "user123", tokenInfo.UserID)
	assert.Contains(suite.T(), tokenInfo.Scopes, "openid")
	assert.Contains(suite.T(), tokenInfo.Scopes, "profile")
	assert.Contains(suite.T(), tokenInfo.Scopes, "email")
	assert.False(suite.T(), tokenInfo.Expiration.IsZero())

	// The SecurityContext survives the round trip through TokenInfo.Extra, so mcp.DefaultGuard can
	// attach it to the outgoing request context exactly as the REST gate would.
	secCtx := SecurityContextFromTokenInfo(tokenInfo)
	if assert.NotNil(suite.T(), secCtx) {
		enrichedCtx := security.WithSecurityContext(context.Background(), secCtx)
		assert.Equal(suite.T(), "user123", security.GetSubject(enrichedCtx))
	}
}

func (suite *TokenVerifierTestSuite) TestNewTokenVerifier_JWTVerificationFailed() {
	testToken := encodeTestToken(map[string]interface{}{"sub": "user123"})

	suite.mockJWT.On("VerifyJWT", mock.Anything, testToken, testMCPURL, "").Return(&tidcommon.ServiceError{
		Type:  tidcommon.ServerErrorType,
		Code:  "INVALID_SIGNATURE",
		Error: tidcommon.I18nMessage{DefaultValue: "Invalid signature"},
	})

	verifier := suite.newVerifier()
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	tokenInfo, err := verifier(context.Background(), testToken, req)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), tokenInfo)
	assert.Equal(suite.T(), auth.ErrInvalidToken, err)
}

func (suite *TokenVerifierTestSuite) TestNewTokenVerifier_RevokedTokenRejected() {
	now := time.Now().Unix()
	testToken := encodeTestToken(map[string]interface{}{
		"sub": "user123",
		"exp": float64(now + 3600),
		"jti": "revoked-jti",
	})

	suite.mockJWT.On("VerifyJWT", mock.Anything, testToken, testMCPURL, "").Return(nil)
	suite.revocation.err = errors.New("token revoked")

	verifier := suite.newVerifier()
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	tokenInfo, err := verifier(context.Background(), testToken, req)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), tokenInfo)
	assert.Equal(suite.T(), auth.ErrInvalidToken, err)
}

func (suite *TokenVerifierTestSuite) TestNewTokenVerifier_InvalidPayload() {
	headerJSON, _ := json.Marshal(map[string]interface{}{"alg": "RS256", "typ": jwt.TokenTypeAccessToken})
	testToken := base64.RawURLEncoding.EncodeToString(headerJSON) + ".invalid-payload.signature"

	suite.mockJWT.On("VerifyJWT", mock.Anything, testToken, testMCPURL, "").Return(nil)

	verifier := suite.newVerifier()
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	tokenInfo, err := verifier(context.Background(), testToken, req)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), tokenInfo)
	assert.Equal(suite.T(), auth.ErrInvalidToken, err)
}

func (suite *TokenVerifierTestSuite) TestNewTokenVerifier_NoScopes() {
	now := time.Now().Unix()
	testToken := encodeTestToken(map[string]interface{}{
		"sub": "user123",
		"exp": float64(now + 3600),
	})

	suite.mockJWT.On("VerifyJWT", mock.Anything, testToken, testMCPURL, "").Return(nil)

	verifier := suite.newVerifier()
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	tokenInfo, err := verifier(context.Background(), testToken, req)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), tokenInfo)
	assert.Equal(suite.T(), "user123", tokenInfo.UserID)
	assert.Empty(suite.T(), tokenInfo.Scopes)
}

func (suite *TokenVerifierTestSuite) TestNewTokenVerifier_EmptyUserID() {
	now := time.Now().Unix()
	testToken := encodeTestToken(map[string]interface{}{
		"exp":   float64(now + 3600),
		"scope": "openid",
	})

	suite.mockJWT.On("VerifyJWT", mock.Anything, testToken, testMCPURL, "").Return(nil)

	verifier := suite.newVerifier()
	req := httptest.NewRequest(http.MethodGet, "/mcp", nil)
	tokenInfo, err := verifier(context.Background(), testToken, req)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), tokenInfo)
	assert.Equal(suite.T(), "", tokenInfo.UserID)
	assert.Contains(suite.T(), tokenInfo.Scopes, "openid")
}
