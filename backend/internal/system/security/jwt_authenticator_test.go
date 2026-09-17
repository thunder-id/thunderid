// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package security

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
)

// JWTAuthenticatorTestSuite defines the test suite for JWTAuthenticator
type JWTAuthenticatorTestSuite struct {
	suite.Suite
	mockJWT       *jwtmock.JWTServiceInterfaceMock
	authenticator *jwtAuthenticator
}

func (suite *JWTAuthenticatorTestSuite) SetupTest() {
	suite.mockJWT = jwtmock.NewJWTServiceInterfaceMock(suite.T())
	suite.authenticator = newJWTAuthenticator(suite.mockJWT)
	// Initialize an empty runtime so verifyFederatedToken sees an unconfigured trusted issuer
	// and returns false cleanly. Tests that need a specific trusted issuer config override this.
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("", &config.Config{})
}

func (suite *JWTAuthenticatorTestSuite) TearDownTest() {
	suite.mockJWT.AssertExpectations(suite.T())
	config.ResetServerRuntime()
}

// Run the test suite
func TestJWTAuthenticatorSuite(t *testing.T) {
	suite.Run(t, new(JWTAuthenticatorTestSuite))
}

func (suite *JWTAuthenticatorTestSuite) TestCanHandle() {
	tests := []struct {
		name           string
		authHeader     string
		expectedResult bool
	}{
		{
			name:           "Valid Bearer token",
			authHeader:     "Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.abc",
			expectedResult: true,
		},
		{
			name:           "No Authorization header",
			authHeader:     "",
			expectedResult: false,
		},
		{
			name:           "Basic auth header",
			authHeader:     "Basic dXNlcjpwYXNz",
			expectedResult: false,
		},
		{
			name:           "Bearer without token",
			authHeader:     "Bearer",
			expectedResult: false,
		},
		{
			name:           "Lowercase bearer",
			authHeader:     "bearer token123",
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			req := httptest.NewRequest(http.MethodGet, "/users", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			result := suite.authenticator.CanHandle(req)
			assert.Equal(suite.T(), tt.expectedResult, result)
		})
	}
}

func (suite *JWTAuthenticatorTestSuite) TestAuthenticate() {
	// A self-issued token only authenticates as an access token, so every fixture that is meant to
	// reach signature verification carries the RFC 9068 typ header.
	validToken := buildFakeJWT(
		accessTokenHeader(),
		map[string]interface{}{
			"sub": "user123", "scope": "system users:read", "ouId": "ou1", "app_id": "app1",
		},
	)
	badSignatureToken := buildFakeJWT(accessTokenHeader(), map[string]interface{}{"sub": "user123"})
	expiredToken := buildFakeJWT(accessTokenHeader(), map[string]interface{}{"sub": "expired-user"})
	malformedBase64PayloadToken := accessTokenHeaderB64() + ".invalid!base64!payload.signature"
	malformedJSONPayloadToken := accessTokenHeaderB64() + ".bm90X3ZhbGlkX2pzb24.signature"

	tests := []struct {
		name           string
		authHeader     string
		setupMock      func(*jwtmock.JWTServiceInterfaceMock)
		expectedError  error
		validateResult func(*testing.T, *SecurityContext)
	}{
		{
			name:       "Successful authentication with system scope",
			authHeader: "Bearer " + validToken,
			setupMock: func(m *jwtmock.JWTServiceInterfaceMock) {
				m.On("VerifyJWT", mock.Anything, validToken, "", "").Return(nil)
			},
			expectedError: nil,
			validateResult: func(t *testing.T, ctx *SecurityContext) {
				baseCtx := withSecurityContext(context.Background(), ctx)
				assert.Equal(t, "user123", GetSubject(baseCtx))
				assert.Equal(t, "ou1", GetOUID(baseCtx))
			},
		},
		{
			name:          "Missing Authorization header",
			authHeader:    "",
			setupMock:     func(m *jwtmock.JWTServiceInterfaceMock) {},
			expectedError: errMissingAuthHeader,
		},
		{
			name:          "Invalid header format",
			authHeader:    "Basic dXNlcjpwYXNz",
			setupMock:     func(m *jwtmock.JWTServiceInterfaceMock) {},
			expectedError: errMissingAuthHeader,
		},
		{
			name:          "Empty token",
			authHeader:    "Bearer   ",
			setupMock:     func(m *jwtmock.JWTServiceInterfaceMock) {},
			expectedError: errInvalidToken,
		},
		{
			name:       "Invalid JWT signature",
			authHeader: "Bearer " + badSignatureToken,
			setupMock: func(m *jwtmock.JWTServiceInterfaceMock) {
				m.On("VerifyJWT", mock.Anything, badSignatureToken, "", "").Return(&tidcommon.ServiceError{
					Type:             tidcommon.ServerErrorType,
					Code:             "INVALID_SIGNATURE",
					Error:            tidcommon.I18nMessage{DefaultValue: "Invalid signature"},
					ErrorDescription: tidcommon.I18nMessage{DefaultValue: "The JWT signature is invalid"},
				})
			},
			expectedError: errInvalidToken,
		},
		{
			name:       "Expired JWT token",
			authHeader: "Bearer " + expiredToken,
			setupMock: func(m *jwtmock.JWTServiceInterfaceMock) {
				m.On("VerifyJWT", mock.Anything, expiredToken, "", "").Return(&tidcommon.ServiceError{
					Type:             tidcommon.ClientErrorType,
					Code:             "JWT-60005",
					Error:            tidcommon.I18nMessage{DefaultValue: "Token has expired"},
					ErrorDescription: tidcommon.I18nMessage{DefaultValue: "The JWT token has expired"},
				})
			},
			expectedError: errInvalidToken,
		},
		{
			// Not 3 parts separated by dots, so the header cannot be decoded and the token is
			// rejected as not an access token, before any verifier is consulted.
			name:          "Invalid JWT format - decoding error",
			authHeader:    "Bearer invalidjwtformat",
			setupMock:     func(m *jwtmock.JWTServiceInterfaceMock) {},
			expectedError: errInvalidToken,
		},
		{
			name:       "Invalid JWT payload - malformed base64",
			authHeader: "Bearer " + malformedBase64PayloadToken,
			setupMock: func(m *jwtmock.JWTServiceInterfaceMock) {
				m.On("VerifyJWT", mock.Anything, malformedBase64PayloadToken, "", "").Return(nil)
			},
			expectedError: errInvalidToken,
		},
		{
			name:       "Invalid JWT payload - malformed JSON", // "not_valid_json" base64 encoded
			authHeader: "Bearer " + malformedJSONPayloadToken,
			setupMock: func(m *jwtmock.JWTServiceInterfaceMock) {
				m.On("VerifyJWT", mock.Anything, malformedJSONPayloadToken, "", "").Return(nil)
			},
			expectedError: errInvalidToken,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Reset mock for each test case
			suite.mockJWT = jwtmock.NewJWTServiceInterfaceMock(suite.T())
			if tt.setupMock != nil {
				tt.setupMock(suite.mockJWT)
			}
			suite.authenticator = newJWTAuthenticator(suite.mockJWT)

			req := httptest.NewRequest(http.MethodGet, "/users", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			authCtx, err := suite.authenticator.Authenticate(req)

			if tt.expectedError != nil {
				assert.ErrorIs(suite.T(), err, tt.expectedError)
				assert.Nil(suite.T(), authCtx)
			} else {
				assert.NoError(suite.T(), err)
				assert.NotNil(suite.T(), authCtx)
				if tt.validateResult != nil {
					tt.validateResult(suite.T(), authCtx)
				}
			}

			suite.mockJWT.AssertExpectations(suite.T())
		})
	}
}

// TestAuthenticate_DoesNotValidateAudience locks in that the REST gate does not restrict a
// self-issued token to a particular audience/resource, regardless of what "aud" claim it carries.
// Authorization for REST is by scope (apiPermissionEntries), not audience. Only MCP (via
// BearerAuthenticator, constructed with expectedAud = its own resource URL) enforces an RFC 8707
// resource-indicator audience check; the REST gate's own jwtAuthenticator always passes "" here.
func (suite *JWTAuthenticatorTestSuite) TestAuthenticate_DoesNotValidateAudience() {
	token := buildFakeJWT(
		accessTokenHeader(),
		map[string]interface{}{"sub": "user123", "aud": "https://some-other-resource/mcp"},
	)

	// Registering the expectation with expectedAud "" only (not the token's own "aud" claim, and
	// not any other value) means the mock call itself fails this test if Authenticate ever starts
	// passing a real expected audience for the REST gate.
	suite.mockJWT.On("VerifyJWT", mock.Anything, token, "", "").Return(nil)

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	authCtx, err := suite.authenticator.Authenticate(req)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), authCtx)
	suite.mockJWT.AssertExpectations(suite.T())
}

func (suite *JWTAuthenticatorTestSuite) TestExtractPermissionsFromJWTClaims() {
	tests := []struct {
		name                string
		attributes          map[string]interface{}
		expectedPermissions []string
	}{
		{
			name: "OAuth2 standard scope attribute (space-separated)",
			attributes: map[string]interface{}{
				"scope": "users:read users:write applications:manage",
			},
			expectedPermissions: []string{"users:read", "users:write", "applications:manage"},
		},
		{
			name: "Scopes as array of strings",
			attributes: map[string]interface{}{
				"scopes": []string{"users:read", "users:write"},
			},
			expectedPermissions: []string{"users:read", "users:write"},
		},
		{
			name: "Scopes as array of interfaces",
			attributes: map[string]interface{}{
				"scopes": []interface{}{"users:read", "users:write"},
			},
			expectedPermissions: []string{"users:read", "users:write"},
		},
		{
			name: "Empty scope attribute",
			attributes: map[string]interface{}{
				"scope": "",
			},
			expectedPermissions: []string{},
		},
		{
			name:                "No scope attribute",
			attributes:          map[string]interface{}{},
			expectedPermissions: []string{},
		},
		{
			name: "Single scope",
			attributes: map[string]interface{}{
				"scope": "users:read",
			},
			expectedPermissions: []string{"users:read"},
		},
		{
			// An assertion's authorized_permissions never becomes a caller's permissions. Only an
			// access token authenticates, and its scopes are carried in scope.
			name: "Assertion authorized_permissions attribute is ignored",
			attributes: map[string]interface{}{
				"authorized_permissions": "perm1 perm2 perm3",
			},
			expectedPermissions: []string{},
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			permissions := extractScopes(tt.attributes)
			assert.ElementsMatch(suite.T(), tt.expectedPermissions, permissions)
		})
	}
}

func (suite *JWTAuthenticatorTestSuite) TestExtractAttribute() {
	tests := []struct {
		name          string
		attributes    map[string]interface{}
		key           string
		expectedValue string
	}{
		{
			name:          "Existing string attribute",
			attributes:    map[string]interface{}{"ou_id": "ou123"},
			key:           "ou_id",
			expectedValue: "ou123",
		},
		{
			name:          "Non-existent attribute",
			attributes:    map[string]interface{}{"other": "value"},
			key:           "ou_id",
			expectedValue: "",
		},
		{
			name:          "Non-string attribute value",
			attributes:    map[string]interface{}{"ou_id": 123},
			key:           "ou_id",
			expectedValue: "",
		},
		{
			name:          "Empty attributes",
			attributes:    map[string]interface{}{},
			key:           "ou_id",
			expectedValue: "",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			result := extractAttribute(tt.attributes, tt.key)
			assert.Equal(suite.T(), tt.expectedValue, result)
		})
	}
}

func (suite *JWTAuthenticatorTestSuite) TestExtractPermissionsFromJWTClaims_EdgeCases() {
	tests := []struct {
		name                string
		attributes          map[string]interface{}
		expectedPermissions []string
	}{
		{
			name: "Scopes array with mixed types (should filter non-strings)",
			attributes: map[string]interface{}{
				"scopes": []interface{}{"valid", 123, true, "another_valid"},
			},
			expectedPermissions: []string{"valid", "another_valid"},
		},
		{
			name: "Scopes as non-array, non-string type",
			attributes: map[string]interface{}{
				"scopes": map[string]string{"invalid": "format"},
			},
			expectedPermissions: []string{},
		},
		{
			name: "Scope attribute with extra whitespace",
			attributes: map[string]interface{}{
				"scope": "  users:read   users:write  ",
			},
			expectedPermissions: []string{"users:read", "users:write"},
		},
		{
			name: "Both scope and scopes present (scope takes precedence)",
			attributes: map[string]interface{}{
				"scope":  "from_scope",
				"scopes": []string{"from_scopes"},
			},
			expectedPermissions: []string{"from_scope"},
		},
		{
			name: "Scope as non-string type",
			attributes: map[string]interface{}{
				"scope": 12345,
			},
			expectedPermissions: []string{},
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			permissions := extractScopes(tt.attributes)
			assert.ElementsMatch(suite.T(), tt.expectedPermissions, permissions)
		})
	}
}

func (suite *JWTAuthenticatorTestSuite) TestNewJWTAuthenticator() {
	mockJWTService := jwtmock.NewJWTServiceInterfaceMock(suite.T())

	authenticator := newJWTAuthenticator(mockJWTService)

	assert.NotNil(suite.T(), authenticator)
	assert.Equal(suite.T(), mockJWTService, authenticator.jwtService)
}

func (suite *JWTAuthenticatorTestSuite) TestCanHandle_EdgeCases() {
	tests := []struct {
		name           string
		setupRequest   func() *http.Request
		expectedResult bool
	}{
		{
			name: "Bearer with space but no token",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/users", nil)
				req.Header.Set("Authorization", "Bearer ")
				return req
			},
			expectedResult: true, // CanHandle only checks prefix, validation is in Authenticate
		},
		{
			name: "Bearer with tab character",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/users", nil)
				req.Header.Set("Authorization", "Bearer\ttoken123")
				return req
			},
			expectedResult: false,
		},
		{
			name: "Multiple Authorization headers",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/users", nil)
				req.Header.Add("Authorization", "Basic xyz")
				req.Header.Add("Authorization", "Bearer token123")
				return req
			},
			expectedResult: false, // Get() returns first header
		},
		{
			name: "Case insensitive BEARER",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/users", nil)
				req.Header.Set("Authorization", "BEARER token123")
				return req
			},
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			req := tt.setupRequest()
			result := suite.authenticator.CanHandle(req)
			assert.Equal(suite.T(), tt.expectedResult, result)
		})
	}
}

const (
	testFederatedIssuer      = "https://external-auth:8090"
	testFederatedJWKSURL     = "https://external-auth:8090/oauth2/jwks"
	testFederatedAudience    = "FEDERATED_CONSOLE"
	testResourceServerAudURL = "https://resource-server:9443"
	testLocalIssuer          = "https://localhost:8090"
)

// buildFakeJWT creates a fake JWT string with the given header and payload claims.
// accessTokenHeader returns the header of a self-issued access token, the only self-issued token the
// gate authenticates.
func accessTokenHeader() map[string]interface{} {
	return map[string]interface{}{"alg": "RS256", "kid": "test-kid", "typ": jwt.TokenTypeAccessToken}
}

// accessTokenHeaderB64 returns that header already encoded, for building tokens whose payload is
// deliberately malformed and so cannot go through buildFakeJWT.
func accessTokenHeaderB64() string {
	headerJSON, _ := json.Marshal(accessTokenHeader())
	return base64.RawURLEncoding.EncodeToString(headerJSON)
}

func buildFakeJWT(header, payload map[string]interface{}) string {
	headerJSON, _ := json.Marshal(header)
	payloadJSON, _ := json.Marshal(payload)
	h := base64.RawURLEncoding.EncodeToString(headerJSON)
	p := base64.RawURLEncoding.EncodeToString(payloadJSON)
	return h + "." + p + ".fakesignature"
}

func (suite *JWTAuthenticatorTestSuite) TestVerifyFederatedToken_Disabled() {
	config.ResetServerRuntime()
	defer config.ResetServerRuntime()

	cfg := &config.Config{}
	_ = config.InitializeServerRuntime("", cfg)

	token := buildFakeJWT(
		map[string]interface{}{"alg": "RS256", "kid": "test-kid"},
		map[string]interface{}{"sub": "user1", "iss": testFederatedIssuer},
	)

	result := suite.authenticator.verifyFederatedToken(context.Background(), token)
	assert.False(suite.T(), result)
}

func (suite *JWTAuthenticatorTestSuite) TestVerifyFederatedToken_IssuerMismatch() {
	config.ResetServerRuntime()
	defer config.ResetServerRuntime()

	cfg := &config.Config{
		Server: engineconfig.ServerConfig{
			SecurityConfig: engineconfig.SecurityConfig{
				TrustedIssuer: engineconfig.TrustedIssuerConfig{
					Issuer:   "https://expected-auth:8090",
					JWKSURL:  "https://expected-auth:8090/oauth2/jwks",
					Audience: testFederatedAudience,
				},
			},
		},
	}
	_ = config.InitializeServerRuntime("", cfg)

	token := buildFakeJWT(
		map[string]interface{}{"alg": "RS256", "kid": "test-kid"},
		map[string]interface{}{"sub": "user1", "iss": "https://wrong-auth:8090"},
	)

	result := suite.authenticator.verifyFederatedToken(context.Background(), token)
	assert.False(suite.T(), result)
}

func (suite *JWTAuthenticatorTestSuite) TestVerifyFederatedToken_JWKSVerificationSuccess() {
	config.ResetServerRuntime()
	defer config.ResetServerRuntime()

	issuer := testFederatedIssuer
	jwksURL := testFederatedJWKSURL
	audience := testFederatedAudience

	cfg := &config.Config{
		Server: engineconfig.ServerConfig{
			SecurityConfig: engineconfig.SecurityConfig{
				TrustedIssuer: engineconfig.TrustedIssuerConfig{
					Issuer:   issuer,
					JWKSURL:  jwksURL,
					Audience: audience,
				},
			},
		},
	}
	_ = config.InitializeServerRuntime("", cfg)

	token := buildFakeJWT(
		map[string]interface{}{"alg": "RS256", "kid": "test-kid"},
		map[string]interface{}{"sub": "user1", "iss": issuer},
	)

	mockJWT := jwtmock.NewJWTServiceInterfaceMock(suite.T())
	mockJWT.On("VerifyJWTWithJWKS", mock.Anything, token, jwksURL, audience, issuer).Return(nil)
	auth := newJWTAuthenticator(mockJWT)

	result := auth.verifyFederatedToken(context.Background(), token)
	assert.True(suite.T(), result)
	mockJWT.AssertExpectations(suite.T())
}

func (suite *JWTAuthenticatorTestSuite) TestVerifyFederatedToken_JWKSVerificationFailure() {
	config.ResetServerRuntime()
	defer config.ResetServerRuntime()

	issuer := testFederatedIssuer
	jwksURL := testFederatedJWKSURL
	audience := testFederatedAudience

	cfg := &config.Config{
		Server: engineconfig.ServerConfig{
			SecurityConfig: engineconfig.SecurityConfig{
				TrustedIssuer: engineconfig.TrustedIssuerConfig{
					Issuer:   issuer,
					JWKSURL:  jwksURL,
					Audience: audience,
				},
			},
		},
	}
	_ = config.InitializeServerRuntime("", cfg)

	token := buildFakeJWT(
		map[string]interface{}{"alg": "RS256", "kid": "test-kid"},
		map[string]interface{}{"sub": "user1", "iss": issuer},
	)

	mockJWT := jwtmock.NewJWTServiceInterfaceMock(suite.T())
	mockJWT.On("VerifyJWTWithJWKS", mock.Anything, token, jwksURL, audience, issuer).Return(&tidcommon.ServiceError{
		Type:  tidcommon.ServerErrorType,
		Code:  "JWKS_ERROR",
		Error: tidcommon.I18nMessage{DefaultValue: "JWKS verification failed"},
	})
	auth := newJWTAuthenticator(mockJWT)

	result := auth.verifyFederatedToken(context.Background(), token)
	assert.False(suite.T(), result)
	mockJWT.AssertExpectations(suite.T())
}

func (suite *JWTAuthenticatorTestSuite) TestVerifyFederatedToken_RequiredClaims() {
	issuer := testFederatedIssuer
	jwksURL := testFederatedJWKSURL
	audience := testResourceServerAudURL

	tests := []struct {
		name           string
		requiredClaims []engineconfig.RequiredClaim
		payloadClaims  map[string]interface{}
		expectedResult bool
	}{
		{
			name:           "RequiredClaimsMatch",
			requiredClaims: []engineconfig.RequiredClaim{{Claim: "ouId", Value: "tenant-org-1"}},
			payloadClaims:  map[string]interface{}{"sub": "user1", "iss": issuer, "ouId": "tenant-org-1"},
			expectedResult: true,
		},
		{
			name:           "RequiredClaimMismatch",
			requiredClaims: []engineconfig.RequiredClaim{{Claim: "ouId", Value: "tenant-org-1"}},
			payloadClaims:  map[string]interface{}{"sub": "user1", "iss": issuer, "ouId": "wrong-org"},
			expectedResult: false,
		},
		{
			name:           "RequiredClaimMissing",
			requiredClaims: []engineconfig.RequiredClaim{{Claim: "ouId", Value: "tenant-org-1"}},
			payloadClaims:  map[string]interface{}{"sub": "user1", "iss": issuer},
			expectedResult: false,
		},
		{
			name: "MultipleRequiredClaimsAllMatch",
			requiredClaims: []engineconfig.RequiredClaim{
				{Claim: "ouId", Value: "tenant-org-1"},
				{Claim: "access_tier", Value: "admin"},
			},
			payloadClaims: map[string]interface{}{
				"sub": "user1", "iss": issuer, "ouId": "tenant-org-1", "access_tier": "admin",
			},
			expectedResult: true,
		},
		{
			name: "MultipleRequiredClaimsOneFails",
			requiredClaims: []engineconfig.RequiredClaim{
				{Claim: "ouId", Value: "tenant-org-1"},
				{Claim: "access_tier", Value: "admin"},
			},
			payloadClaims: map[string]interface{}{
				"sub": "user1", "iss": issuer, "ouId": "tenant-org-1", "access_tier": "viewer",
			},
			expectedResult: false,
		},
		{
			name:           "NoRequiredClaims",
			requiredClaims: nil,
			payloadClaims:  map[string]interface{}{"sub": "user1", "iss": issuer},
			expectedResult: true,
		},
	}

	for _, tc := range tests {
		suite.Run(tc.name, func() {
			config.ResetServerRuntime()
			defer config.ResetServerRuntime()

			cfg := &config.Config{
				Server: engineconfig.ServerConfig{
					SecurityConfig: engineconfig.SecurityConfig{
						TrustedIssuer: engineconfig.TrustedIssuerConfig{
							Issuer:         issuer,
							JWKSURL:        jwksURL,
							Audience:       audience,
							RequiredClaims: tc.requiredClaims,
						},
					},
				},
			}
			_ = config.InitializeServerRuntime("", cfg)

			token := buildFakeJWT(
				map[string]interface{}{"alg": "RS256", "kid": "test-kid"},
				tc.payloadClaims,
			)

			mockJWT := jwtmock.NewJWTServiceInterfaceMock(suite.T())
			mockJWT.On("VerifyJWTWithJWKS", mock.Anything, token, jwksURL, audience, issuer).Return(nil)
			auth := newJWTAuthenticator(mockJWT)

			result := auth.verifyFederatedToken(context.Background(), token)
			assert.Equal(suite.T(), tc.expectedResult, result)
			mockJWT.AssertExpectations(suite.T())
		})
	}
}

func (suite *JWTAuthenticatorTestSuite) TestVerifyFederatedToken_InvalidPayload() {
	// When the trusted issuer is configured but the bearer token is malformed in any
	// way that causes DecodeJWTPayload to fail, verifyFederatedToken must short-circuit
	// to false without ever calling the JWKS verifier. We exercise every distinct
	// failure mode of DecodeJWTPayload (wrong number of parts, undecodable base64,
	// non-JSON payload) so a regression that handles one case but misses another is
	// caught.
	validHeaderB64 := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","kid":"test-kid"}`))
	validPayloadB64 := base64.RawURLEncoding.EncodeToString(
		[]byte(`{"iss":"` + testFederatedIssuer + `","sub":"user1"}`))

	tests := []struct {
		name  string
		token string
	}{
		{
			// fewer than 3 dot-separated parts → "invalid JWT format"
			name:  "TwoParts",
			token: validHeaderB64 + "." + validPayloadB64,
		},
		{
			// empty string → 1 part → "invalid JWT format"
			name:  "EmptyString",
			token: "",
		},
		{
			// payload segment is not valid base64url → "failed to decode JWT payload"
			name:  "InvalidBase64Payload",
			token: validHeaderB64 + ".not!valid!base64." + "fakesignature",
		},
		{
			// payload decodes to bytes that aren't valid JSON → "failed to unmarshal JWT claims"
			name:  "PayloadNotJSON",
			token: validHeaderB64 + "." + base64.RawURLEncoding.EncodeToString([]byte("not-json")) + ".fakesignature",
		},
	}

	for _, tc := range tests {
		suite.Run(tc.name, func() {
			config.ResetServerRuntime()
			defer config.ResetServerRuntime()

			cfg := &config.Config{
				Server: engineconfig.ServerConfig{
					SecurityConfig: engineconfig.SecurityConfig{
						TrustedIssuer: engineconfig.TrustedIssuerConfig{
							Issuer:   testFederatedIssuer,
							JWKSURL:  testFederatedJWKSURL,
							Audience: testFederatedAudience,
						},
					},
				},
			}
			_ = config.InitializeServerRuntime("", cfg)

			mockJWT := jwtmock.NewJWTServiceInterfaceMock(suite.T())
			auth := newJWTAuthenticator(mockJWT)

			result := auth.verifyFederatedToken(context.Background(), tc.token)
			assert.False(suite.T(), result, "malformed token must not verify")
			mockJWT.AssertNotCalled(suite.T(), "VerifyJWTWithJWKS")
		})
	}
}

func (suite *JWTAuthenticatorTestSuite) TestAuthenticate_FederatedTokenFailure() {
	// A token whose issuer is the trusted issuer is routed to federated
	// verification. When that fails (here, JWKS verification returns an error),
	// Authenticate must reject the request with errInvalidToken and must NOT
	// fall back to local-key verification (no cross-issuer fallback).
	config.ResetServerRuntime()
	defer config.ResetServerRuntime()

	issuer := testFederatedIssuer
	jwksURL := testFederatedJWKSURL
	audience := testFederatedAudience

	cfg := &config.Config{
		Server: engineconfig.ServerConfig{
			SecurityConfig: engineconfig.SecurityConfig{
				TrustedIssuer: engineconfig.TrustedIssuerConfig{
					Issuer:   issuer,
					JWKSURL:  jwksURL,
					Audience: audience,
				},
			},
		},
	}
	_ = config.InitializeServerRuntime("", cfg)

	token := buildFakeJWT(
		map[string]interface{}{"alg": "RS256", "kid": "test-kid"},
		map[string]interface{}{
			"sub":   "federated-user",
			"iss":   issuer,
			"scope": "openid system",
		},
	)

	mockJWT := jwtmock.NewJWTServiceInterfaceMock(suite.T())
	mockJWT.On("VerifyJWTWithJWKS", mock.Anything, token, jwksURL, audience, issuer).Return(&tidcommon.ServiceError{
		Type:  tidcommon.ServerErrorType,
		Code:  "JWKS_ERROR",
		Error: tidcommon.I18nMessage{DefaultValue: "JWKS verification failed"},
	})
	auth := newJWTAuthenticator(mockJWT)

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	authCtx, err := auth.Authenticate(req)
	assert.ErrorIs(suite.T(), err, errInvalidToken)
	assert.Nil(suite.T(), authCtx)
	mockJWT.AssertExpectations(suite.T())
	mockJWT.AssertNotCalled(suite.T(), "VerifyJWT")
	mockJWT.AssertNotCalled(suite.T(), "VerifyJWTSignature")
}

func (suite *JWTAuthenticatorTestSuite) TestAuthenticate_FederatedTokenSuccess() {
	config.ResetServerRuntime()
	defer config.ResetServerRuntime()

	issuer := testFederatedIssuer
	jwksURL := testFederatedJWKSURL
	audience := testFederatedAudience

	cfg := &config.Config{
		Server: engineconfig.ServerConfig{
			SecurityConfig: engineconfig.SecurityConfig{
				TrustedIssuer: engineconfig.TrustedIssuerConfig{
					Issuer:   issuer,
					JWKSURL:  jwksURL,
					Audience: audience,
				},
			},
		},
	}
	_ = config.InitializeServerRuntime("", cfg)

	token := buildFakeJWT(
		map[string]interface{}{"alg": "RS256", "kid": "test-kid"},
		map[string]interface{}{
			"sub":   "federated-user",
			"iss":   issuer,
			"scope": "openid system",
		},
	)

	mockJWT := jwtmock.NewJWTServiceInterfaceMock(suite.T())
	// When trusted issuer is configured, the local-key path is skipped entirely.
	mockJWT.On("VerifyJWTWithJWKS", mock.Anything, token, jwksURL, audience, issuer).Return(nil)
	auth := newJWTAuthenticator(mockJWT)

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	authCtx, err := auth.Authenticate(req)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), authCtx)

	baseCtx := withSecurityContext(context.Background(), authCtx)
	assert.Equal(suite.T(), "federated-user", GetSubject(baseCtx))
	mockJWT.AssertExpectations(suite.T())
	mockJWT.AssertNotCalled(suite.T(), "VerifyJWTSignature")
}

// federatedConfigWithLocalIssuer returns a config where a trusted issuer is
// configured (federated mode) and the server's own JWT issuer is set, so that
// issuer-based routing can distinguish self-issued tokens from federated ones.
func federatedConfigWithLocalIssuer() *config.Config {
	return &config.Config{
		Server: engineconfig.ServerConfig{
			SecurityConfig: engineconfig.SecurityConfig{
				TrustedIssuer: engineconfig.TrustedIssuerConfig{
					Issuer:   testFederatedIssuer,
					JWKSURL:  testFederatedJWKSURL,
					Audience: testFederatedAudience,
				},
			},
		},
		JWT: engineconfig.JWTConfig{Issuer: testLocalIssuer},
	}
}

func (suite *JWTAuthenticatorTestSuite) TestAuthenticate_SelfIssuedTokenUnderFederation() {
	// Regression: a token this server issued itself (e.g. via client_credentials)
	// must still authenticate against the server's own secured APIs even when a
	// trusted issuer is configured. It is routed to local-key verification by its
	// iss claim and must never be sent to the trusted issuer's JWKS.
	config.ResetServerRuntime()
	defer config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("", federatedConfigWithLocalIssuer())

	token := buildFakeJWT(
		map[string]interface{}{"alg": "RS256", "kid": "local-kid", "typ": jwt.TokenTypeAccessToken},
		map[string]interface{}{
			"sub":              "service-app",
			"access_token_sub": "user-123",
			"iat":              float64(1_700_000_000),
			"iss":              testLocalIssuer,
			"scope":            "system",
		},
	)

	mockJWT := jwtmock.NewJWTServiceInterfaceMock(suite.T())
	mockJWT.On("VerifyJWT", mock.Anything, token, "", "").Return(nil)
	auth := newJWTAuthenticator(mockJWT)

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	authCtx, err := auth.Authenticate(req)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), authCtx)

	baseCtx := withSecurityContext(context.Background(), authCtx)
	assert.Equal(suite.T(), "service-app", GetSubject(baseCtx))
	assert.Equal(suite.T(), "user-123", authCtx.revocationSubject)
	assert.Equal(suite.T(), time.Unix(1_700_000_000, 0).UTC(), authCtx.establishedAt)
	mockJWT.AssertExpectations(suite.T())
	mockJWT.AssertNotCalled(suite.T(), "VerifyJWTWithJWKS")
}

func (suite *JWTAuthenticatorTestSuite) TestAuthenticate_SelfIssuedTokenInvalidUnderFederation() {
	// A self-issued token routed to local verification that fails the signature
	// check is rejected; it must not be retried against the trusted issuer JWKS.
	config.ResetServerRuntime()
	defer config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("", federatedConfigWithLocalIssuer())

	token := buildFakeJWT(
		map[string]interface{}{"alg": "RS256", "kid": "local-kid", "typ": jwt.TokenTypeAccessToken},
		map[string]interface{}{"sub": "service-app", "iss": testLocalIssuer},
	)

	mockJWT := jwtmock.NewJWTServiceInterfaceMock(suite.T())
	mockJWT.On("VerifyJWT", mock.Anything, token, "", "").Return(&tidcommon.ServiceError{
		Type:  tidcommon.ServerErrorType,
		Code:  "INVALID_SIGNATURE",
		Error: tidcommon.I18nMessage{DefaultValue: "Invalid signature"},
	})
	auth := newJWTAuthenticator(mockJWT)

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	authCtx, err := auth.Authenticate(req)
	assert.ErrorIs(suite.T(), err, errInvalidToken)
	assert.Nil(suite.T(), authCtx)
	mockJWT.AssertExpectations(suite.T())
	mockJWT.AssertNotCalled(suite.T(), "VerifyJWTWithJWKS")
}

func (suite *JWTAuthenticatorTestSuite) TestAuthenticate_UnknownIssuerUnderFederation() {
	// A token whose iss is neither the trusted issuer nor this server's own
	// JWT issuer is rejected outright by the issuer allowlist, with no
	// verifier invoked.
	config.ResetServerRuntime()
	defer config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("", federatedConfigWithLocalIssuer())

	token := buildFakeJWT(
		map[string]interface{}{"alg": "RS256", "kid": "rogue-kid"},
		map[string]interface{}{"sub": "attacker", "iss": "https://rogue-issuer:9999"},
	)

	mockJWT := jwtmock.NewJWTServiceInterfaceMock(suite.T())
	auth := newJWTAuthenticator(mockJWT)

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	authCtx, err := auth.Authenticate(req)
	assert.ErrorIs(suite.T(), err, errInvalidToken)
	assert.Nil(suite.T(), authCtx)
	mockJWT.AssertNotCalled(suite.T(), "VerifyJWT")
	mockJWT.AssertNotCalled(suite.T(), "VerifyJWTWithJWKS")
}

// TestAuthenticate_RequiresAccessTokenTypeForSelfIssuedToken asserts the gate accepts only an access
// token (RFC 9068 typ) on the self-issued branch, so no other JWT this server signs with the same key
// — an auth assertion, ID token, magic link, OTP, consent or flow token — passes as an API credential.
func (suite *JWTAuthenticatorTestSuite) TestAuthenticate_RequiresAccessTokenTypeForSelfIssuedToken() {
	tests := []struct {
		name     string
		typ      interface{}
		accepted bool
	}{
		{name: "at+jwt is accepted", typ: jwt.TokenTypeAccessToken, accepted: true},
		{name: "media type form is accepted", typ: jwt.TokenTypeAccessTokenWithPrefix, accepted: true},
		{name: "typ is compared case insensitively", typ: "AT+JWT", accepted: true},
		{name: "plain JWT is rejected", typ: jwt.TokenTypeJWT, accepted: false},
		{name: "ID-JAG is rejected", typ: jwt.TokenTypeIDJAG, accepted: false},
		{name: "missing typ is rejected", typ: nil, accepted: false},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			header := map[string]interface{}{"alg": "RS256", "kid": "test-kid"}
			if tt.typ != nil {
				header["typ"] = tt.typ
			}
			token := buildFakeJWT(header, map[string]interface{}{"sub": "user123"})

			mockJWT := jwtmock.NewJWTServiceInterfaceMock(suite.T())
			if tt.accepted {
				mockJWT.On("VerifyJWT", mock.Anything, token, "", "").Return(nil)
			}
			auth := newJWTAuthenticator(mockJWT)

			req := httptest.NewRequest(http.MethodGet, "/users", nil)
			req.Header.Set("Authorization", "Bearer "+token)

			authCtx, err := auth.Authenticate(req)

			if tt.accepted {
				assert.NoError(suite.T(), err)
				assert.NotNil(suite.T(), authCtx)
			} else {
				assert.ErrorIs(suite.T(), err, errInvalidToken)
				assert.Nil(suite.T(), authCtx)
				// The type check must short-circuit before signature verification.
				mockJWT.AssertNotCalled(suite.T(), "VerifyJWT")
			}
			mockJWT.AssertExpectations(suite.T())
		})
	}
}

// TestAuthenticate_RejectsFlowAuthAssertion covers the concrete confusion the type check closes. The
// assertion the sign-in flow returns is self-issued and carries authorized_permissions, which the
// gate read as the caller's permissions; it is meant only for exchange at the token endpoint.
func (suite *JWTAuthenticatorTestSuite) TestAuthenticate_RejectsFlowAuthAssertion() {
	assertion := buildFakeJWT(
		map[string]interface{}{"alg": "RS256", "kid": "test-kid", "typ": jwt.TokenTypeJWT},
		map[string]interface{}{
			"sub":                    "user123",
			"aud":                    "app1",
			"assurance":              map[string]interface{}{"aal": "AAL1", "ial": "IAL1"},
			"authorized_permissions": "users:read users:write",
		},
	)

	mockJWT := jwtmock.NewJWTServiceInterfaceMock(suite.T())
	auth := newJWTAuthenticator(mockJWT)

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	req.Header.Set("Authorization", "Bearer "+assertion)

	authCtx, err := auth.Authenticate(req)

	assert.ErrorIs(suite.T(), err, errInvalidToken)
	assert.Nil(suite.T(), authCtx)
}

// TestAuthenticate_FederatedTokenTypeNotRestricted pins the access-token type check to self-issued
// tokens only. A trusted issuer may be a generic OIDC provider that stamps typ JWT on its access
// tokens, and those must keep authenticating.
func (suite *JWTAuthenticatorTestSuite) TestAuthenticate_FederatedTokenTypeNotRestricted() {
	config.ResetServerRuntime()
	defer config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("", federatedConfigWithLocalIssuer())

	token := buildFakeJWT(
		map[string]interface{}{"alg": "RS256", "kid": "test-kid", "typ": jwt.TokenTypeJWT},
		map[string]interface{}{"sub": "federated-user", "iss": testFederatedIssuer},
	)

	mockJWT := jwtmock.NewJWTServiceInterfaceMock(suite.T())
	mockJWT.On("VerifyJWTWithJWKS", mock.Anything, token,
		testFederatedJWKSURL, testFederatedAudience, testFederatedIssuer).Return(nil)
	auth := newJWTAuthenticator(mockJWT)

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	authCtx, err := auth.Authenticate(req)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), authCtx)
	mockJWT.AssertExpectations(suite.T())
}
