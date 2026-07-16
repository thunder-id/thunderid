/*
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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

package security

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

var testPublicPaths = append([]string{
	"/health/**",
	"/flow/execute/**",
	"/oauth2/**",
	"/.well-known/openid-configuration/**",
	"/.well-known/oauth-authorization-server/**",
	"/gate/**",
	"/console/**",
	"/error/**",
	"/design/resolve/**",
	"/i18n/languages",
	"/i18n/languages/*/translations/resolve",
	"/i18n/languages/*/translations/ns/*/keys/*/resolve",
}, directAuthPaths...)

// SecurityServiceTestSuite defines the test suite for SecurityService
type SecurityServiceTestSuite struct {
	suite.Suite
	service        *securityService
	mockAuth1      *AuthenticatorInterfaceMock
	mockAuth2      *AuthenticatorInterfaceMock
	mockRevocation *RevocationEnforcerInterfaceMock
	testCtx        *SecurityContext
}

func (suite *SecurityServiceTestSuite) SetupTest() {
	suite.mockAuth1 = &AuthenticatorInterfaceMock{}
	suite.mockAuth2 = &AuthenticatorInterfaceMock{}
	suite.mockRevocation = &RevocationEnforcerInterfaceMock{}
	// Default to "not revoked" so existing authentication paths pass; Maybe() keeps it optional for
	// tests where authentication never yields a security context.
	suite.mockRevocation.On("EnsureNotRevoked", mock.Anything, mock.Anything).Return(nil).Maybe()

	var err error
	suite.service, err = newSecurityService(
		[]AuthenticatorInterface{suite.mockAuth1, suite.mockAuth2}, suite.mockRevocation,
		testPublicPaths, apiPermissionEntries, "")
	suite.Require().NoError(err)

	// Create test authentication context with "system" permission so that
	// the service-level authorization check passes for protected /api/* paths.
	suite.testCtx = newSecurityContext(
		"user123",
		"ou456",
		"test_token",
		[]string{"system"},
		map[string]interface{}{
			"scope": []string{"read", "write"},
			"role":  "admin",
		},
	)
}

func (suite *SecurityServiceTestSuite) TearDownTest() {
	suite.mockAuth1.AssertExpectations(suite.T())
	suite.mockAuth2.AssertExpectations(suite.T())
}

// Run the test suite
func TestSecurityServiceSuite(t *testing.T) {
	suite.Run(t, new(SecurityServiceTestSuite))
}

// Test Process method with public paths
func (suite *SecurityServiceTestSuite) TestProcess_PublicPaths() {
	testCases := []struct {
		name string
		path string
	}{
		{"Gate path", "/gate/login"},
		{"Gate path with subpath", "/gate/account/settings"},
		{"OAuth2 token", "/oauth2/token"},
		{"OAuth2 authorize", "/oauth2/authorize"},
		{"OAuth2 well-known", "/oauth2/.well-known/openid_configuration"},
		{"OAuth2 JWKS", "/oauth2/jwks"},
		{"OAuth2 register", "/oauth2/register"},
		{"Health check liveness", "/health/liveness"},
		{"Health check readiness", "/health/readiness"},
		{"Signin path", "/gate/verify"},
		{"Signin path with subpath", "/gate/forgot-password"},
		{"Console path", "/console/dashboard"},
		{"Console path with subpath", "/console/api/test"},
		{"Gate without trailing slash", "/gate"},
		{"OAuth2 token without params", "/oauth2/token"},
		{"Signin without trailing slash", "/gate/signin"},
		{"Console without trailing slash", "/console"},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)

			// With optional auth, authenticators are checked even for public paths
			// Since there is no auth header, CanHandle returns false
			suite.mockAuth1.On("CanHandle", req).Return(false)
			suite.mockAuth2.On("CanHandle", req).Return(false)

			ctx, err := suite.service.Process(req)

			assert.NoError(suite.T(), err)
			assert.NotNil(suite.T(), ctx)
			assert.True(suite.T(), IsRuntimeContext(ctx), "public path should return a runtime context")
		})
	}
}

// Test Process method with non-public paths and successful authentication
func (suite *SecurityServiceTestSuite) TestProcess_SuccessfulAuthentication_FirstAuthenticator() {
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)

	// First authenticator can handle the request
	suite.mockAuth1.On("CanHandle", req).Return(true)
	suite.mockAuth1.On("Authenticate", req).Return(suite.testCtx, nil)

	ctx, err := suite.service.Process(req)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), ctx)

	// Verify authentication context is added to the context
	userID := GetSubject(ctx)
	assert.Equal(suite.T(), "user123", userID)

	ouID := GetOUID(ctx)
	assert.Equal(suite.T(), "ou456", ouID)

	// Second authenticator should not be called
	suite.mockAuth2.AssertNotCalled(suite.T(), "CanHandle")
	suite.mockAuth2.AssertNotCalled(suite.T(), "Authenticate")
}

// Test Process method with second authenticator handling the request
func (suite *SecurityServiceTestSuite) TestProcess_SuccessfulAuthentication_SecondAuthenticator() {
	req := httptest.NewRequest(http.MethodPost, "/api/groups", nil)

	// First authenticator cannot handle the request, second can
	suite.mockAuth1.On("CanHandle", req).Return(false)
	suite.mockAuth2.On("CanHandle", req).Return(true)
	suite.mockAuth2.On("Authenticate", req).Return(suite.testCtx, nil)

	ctx, err := suite.service.Process(req)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), ctx)

	// Verify authentication context is added
	userID := GetSubject(ctx)
	assert.Equal(suite.T(), "user123", userID)
}

// newDirectAuthTestService builds a security service configured with the given Direct Auth secret,
// optionally including a passthrough authenticator so Direct-API/public paths fall through to a runtime
// context.
func (suite *SecurityServiceTestSuite) newDirectAuthTestService(
	configuredSecret string, auth AuthenticatorInterface) *securityService {
	authenticators := []AuthenticatorInterface{}
	if auth != nil {
		authenticators = append(authenticators, auth)
	}
	svc, err := newSecurityService(authenticators, nil, testPublicPaths, apiPermissionEntries, configuredSecret)
	suite.Require().NoError(err)
	return svc
}

// passthroughAuthenticator returns an authenticator that never handles the request.
func passthroughAuthenticator() *AuthenticatorInterfaceMock {
	m := &AuthenticatorInterfaceMock{}
	m.On("CanHandle", mock.Anything).Return(false)
	return m
}

// TestProcess_DirectAuthSecret tests the Direct Auth Secret gate on the Direct API endpoints.
func (suite *SecurityServiceTestSuite) TestProcess_DirectAuthSecret() {
	const secret = "s3cr3t-value"

	suite.Run("valid secret is admitted", func() {
		svc := suite.newDirectAuthTestService(secret, passthroughAuthenticator())
		req := httptest.NewRequest(http.MethodPost, "/auth/credentials/authenticate", nil)
		req.Header.Set(directAuthHeaderName, secret)

		ctx, err := svc.Process(req)

		suite.NoError(err)
		suite.True(IsRuntimeContext(ctx))
	})

	suite.Run("valid secret is admitted for AuthZEN access evaluation", func() {
		svc := suite.newDirectAuthTestService(secret, passthroughAuthenticator())
		req := httptest.NewRequest(http.MethodPost, "/access/v1/evaluation", nil)
		req.Header.Set(directAuthHeaderName, secret)

		ctx, err := svc.Process(req)

		suite.NoError(err)
		suite.True(IsRuntimeContext(ctx))
	})

	suite.Run("missing secret is rejected", func() {
		svc := suite.newDirectAuthTestService(secret, nil)
		req := httptest.NewRequest(http.MethodPost, "/auth/credentials/authenticate", nil)

		ctx, err := svc.Process(req)

		suite.Nil(ctx)
		suite.ErrorIs(err, errInvalidDirectAuthSecret)
	})

	suite.Run("wrong secret is rejected", func() {
		svc := suite.newDirectAuthTestService(secret, nil)
		req := httptest.NewRequest(http.MethodPost, "/register/passkey/start", nil)
		req.Header.Set(directAuthHeaderName, "wrong")

		ctx, err := svc.Process(req)

		suite.Nil(ctx)
		suite.ErrorIs(err, errInvalidDirectAuthSecret)
	})

	suite.Run("CORS preflight is exempt", func() {
		svc := suite.newDirectAuthTestService(secret, nil)
		req := httptest.NewRequest(http.MethodOptions, "/auth/credentials/authenticate", nil)

		ctx, err := svc.Process(req)

		suite.NoError(err)
		suite.NotNil(ctx)
	})

	suite.Run("non-Direct-API public path is not gated", func() {
		svc := suite.newDirectAuthTestService(secret, passthroughAuthenticator())
		req := httptest.NewRequest(http.MethodGet, "/health/liveness", nil)

		ctx, err := svc.Process(req)

		suite.NoError(err)
		suite.True(IsRuntimeContext(ctx))
	})

	suite.Run("no configured secret blocks Direct API paths (secure by default)", func() {
		svc := suite.newDirectAuthTestService("", nil)
		req := httptest.NewRequest(http.MethodPost, "/auth/credentials/authenticate", nil)

		ctx, err := svc.Process(req)

		suite.Nil(ctx)
		suite.ErrorIs(err, errDirectAuthSecretNotConfigured)
	})
}

// TestInitialize verifies the security middleware is constructed with and without an direct secret.
func (suite *SecurityServiceTestSuite) TestInitialize() {
	mw, err := Initialize(nil, nil, "some-direct-secret")
	suite.Require().NoError(err)
	suite.Require().NotNil(mw)

	mwOpen, err := Initialize(nil, nil, "")
	suite.Require().NoError(err)
	suite.Require().NotNil(mwOpen)
}

// TestIsDirectAPIPath covers the Direct-API-path matcher, including the length guard.
func (suite *SecurityServiceTestSuite) TestIsDirectAPIPath() {
	svc := suite.newDirectAuthTestService("some-direct-secret", nil)

	suite.True(svc.isDirectAuthPath(context.Background(), "/auth/credentials/authenticate"))
	suite.True(svc.isDirectAuthPath(context.Background(), "/register/passkey/start"))
	suite.True(svc.isDirectAuthPath(context.Background(), "/access/v1/evaluation"))
	suite.True(svc.isDirectAuthPath(context.Background(), "/access/v1/evaluations"))
	suite.True(svc.isDirectAuthPath(context.Background(), "/access/v1/search/action"))
	suite.True(svc.isDirectAuthPath(context.Background(), "/access/v1/future-endpoint"))
	suite.False(svc.isDirectAuthPath(context.Background(), "/.well-known/authzen-configuration"))
	suite.False(svc.isDirectAuthPath(context.Background(), "/health/liveness"))
	// Paths longer than the maximum allowed length are rejected by the length guard.
	suite.False(svc.isDirectAuthPath(context.Background(), "/auth/"+strings.Repeat("a", maxPublicPathLength)))
}

// Test Process method when no authenticator can handle the request
func (suite *SecurityServiceTestSuite) TestProcess_NoHandlerFound() {
	req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)

	// Both authenticators cannot handle the request
	suite.mockAuth1.On("CanHandle", req).Return(false)
	suite.mockAuth2.On("CanHandle", req).Return(false)

	ctx, err := suite.service.Process(req)

	assert.Nil(suite.T(), ctx)
	assert.Equal(suite.T(), errNoHandlerFound, err)

	// Verify neither authenticate method was called
	suite.mockAuth1.AssertNotCalled(suite.T(), "Authenticate")
	suite.mockAuth2.AssertNotCalled(suite.T(), "Authenticate")
}

// Test Process method when authentication fails
func (suite *SecurityServiceTestSuite) TestProcess_AuthenticationFailure() {
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	authError := errors.New("invalid credentials")

	suite.mockAuth1.On("CanHandle", req).Return(true)
	suite.mockAuth1.On("Authenticate", req).Return(nil, authError)

	ctx, err := suite.service.Process(req)

	assert.Nil(suite.T(), ctx)
	assert.Equal(suite.T(), authError, err)
}

// Test Process rejects a request whose token has been revoked, before authorization.
func (suite *SecurityServiceTestSuite) TestProcess_RevokedToken() {
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)

	suite.testCtx.revocationID = "jti-123"
	suite.mockAuth1.On("CanHandle", req).Return(true)
	suite.mockAuth1.On("Authenticate", req).Return(suite.testCtx, nil)

	mockRevocation := &RevocationEnforcerInterfaceMock{}
	revokedErr := errors.New("token has been revoked")
	mockRevocation.On("EnsureNotRevoked", mock.Anything, "jti-123").Return(revokedErr)

	service, err := newSecurityService(
		[]AuthenticatorInterface{suite.mockAuth1}, mockRevocation, testPublicPaths, apiPermissionEntries, "")
	suite.Require().NoError(err)

	ctx, err := service.Process(req)

	assert.Nil(suite.T(), ctx)
	// A revoked token is surfaced as an invalid token so the response does not disclose the reason.
	assert.Equal(suite.T(), errInvalidToken, err)
	mockRevocation.AssertExpectations(suite.T())
}

// Test Process consults the enforcer with the token's revocation identifier and proceeds when the
// token is not revoked.
func (suite *SecurityServiceTestSuite) TestProcess_NotRevokedToken() {
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)

	suite.testCtx.revocationID = "jti-456"
	suite.mockAuth1.On("CanHandle", req).Return(true)
	suite.mockAuth1.On("Authenticate", req).Return(suite.testCtx, nil)

	mockRevocation := &RevocationEnforcerInterfaceMock{}
	mockRevocation.On("EnsureNotRevoked", mock.Anything, "jti-456").Return(nil)

	service, err := newSecurityService(
		[]AuthenticatorInterface{suite.mockAuth1}, mockRevocation, testPublicPaths, apiPermissionEntries, "")
	suite.Require().NoError(err)

	ctx, err := service.Process(req)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), ctx)
	assert.Equal(suite.T(), "user123", GetSubject(ctx))
	mockRevocation.AssertExpectations(suite.T())
}

// Test Process method with specific security errors
func (suite *SecurityServiceTestSuite) TestProcess_SecurityErrors() {
	testCases := []struct {
		name  string
		error error
	}{
		{"Unauthorized error", errUnauthorized},
		{"Forbidden error", errForbidden},
		{"Invalid token error", errInvalidToken},
		{"Insufficient permissions error", errInsufficientPermissions},
		{"Missing auth header error", errMissingAuthHeader},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)

			// Reset mocks for each test case
			suite.mockAuth1 = &AuthenticatorInterfaceMock{}
			suite.mockAuth2 = &AuthenticatorInterfaceMock{}
			suite.service.authenticators = []AuthenticatorInterface{suite.mockAuth1, suite.mockAuth2}

			suite.mockAuth1.On("CanHandle", req).Return(true)
			suite.mockAuth1.On("Authenticate", req).Return(nil, tc.error)

			ctx, err := suite.service.Process(req)

			assert.Nil(suite.T(), ctx)
			assert.Equal(suite.T(), tc.error, err)

			suite.mockAuth1.AssertExpectations(suite.T())
		})
	}
}

// Test Process method with nil authenticator context.
// A nil SecurityContext means no permissions are available; the service-level
// authorization check allows the request through when the path requires no
// special permission (e.g. /users/me).
func (suite *SecurityServiceTestSuite) TestProcess_NilSecurityContext() {
	// /users/me requires no special permission, so a nil context must still pass.
	req := httptest.NewRequest(http.MethodGet, "/users/me", nil)

	suite.mockAuth1.On("CanHandle", req).Return(true)
	suite.mockAuth1.On("Authenticate", req).Return(nil, nil)

	ctx, err := suite.service.Process(req)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), ctx)

	// Verify empty context values when auth context is nil
	userID := GetSubject(ctx)
	assert.Empty(suite.T(), userID)
}

// Test isPublicPath method directly
func (suite *SecurityServiceTestSuite) TestIsPublicPath() {
	testCases := []struct {
		name     string
		path     string
		expected bool
	}{
		// Public paths - should return true
		{"Auth root", "/auth/", true},
		{"Auth login", "/auth/credentials/authenticate", true},
		{"OAuth2 token", "/oauth2/token", true},
		{"OAuth2 authorize", "/oauth2/authorize", true},
		{"OAuth2 well-known", "/.well-known/openid-configuration", true},
		{"OAuth2 JWKS", "/oauth2/jwks", true},
		{"OAuth2 register", "/oauth2/register", true},
		{"Health check", "/health/liveness", true},
		{"Signin root", "/gate/signin", true},
		{"Signin logo", "/gate/signin/logo/123", true},
		{"Console root", "/console/", true},
		{"Console dashboard", "/console/dashboard", true},
		{"I18n languages", "/i18n/languages", true},

		// Exact matches without trailing slash
		{"Auth exact", "/auth", true},
		{"OAuth2 token exact", "/oauth2/token", true},
		{"Signin exact", "/gate/signin", true},
		{"Console exact", "/console", true},

		// Non-public paths - should return false
		{"API users", "/api/users", false},
		{"API groups", "/api/groups", false},
		{"Admin panel", "/admin/dashboard", false},
		{"Root path", "/", false},
		{"Random path", "/random/path", false},
		{"Similar but not exact", "/authentication", false},
		{"Similar prefix", "/oauth", false},
		{"Not allowed sub prefix", "/flow", false},

		// Edge cases
		{"Empty path", "", false},
		{"Just slash", "/", false},

		// Parameterized paths
		{"Parameterized path match", "/i18n/languages/en/translations/resolve", true},
		{"Parameterized path mismatch prefix", "/i18n/languages/en/translations", false},
		{"Parameterized path mismatch suffix", "/i18n/languages/en/translations/resolve/extra", false},
		{"Parameterized path empty param", "/i18n/languages//translations/resolve", false},

		// Multi-parameter paths
		{"Multi-param path match", "/i18n/languages/en/translations/ns/common/keys/btn.submit/resolve", true},
		{"Multi-param path mismatch namespace", "/i18n/languages/en/translations/ns//keys/btn.submit/resolve", false},
		{"Multi-param path mismatch key", "/i18n/languages/en/translations/ns/common/keys//resolve", false},
		{"Multi-param path mismatch structure", "/i18n/languages/en/translations/ns/common/keys/btn.submit", false},

		// Special characters in parameters
		{"Parameterized path with hyphen", "/i18n/languages/en-US/translations/resolve", true},
		{"Multi-param path with dots", "/i18n/languages/en/translations/ns/common/keys/btn.submit.label/resolve", true},

		// Performance/Robustness edge cases
		{"Long parameter value within limit",
			"/i18n/languages/" + strings.Repeat("a", 255) + "/translations/resolve", true},
		{"Exceeds max path length", "/i18n/languages/" + strings.Repeat("a", 4096) + "/translations/resolve", false},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			result := suite.service.isPublicPath(context.Background(), tc.path)
			assert.Equal(suite.T(), tc.expected, result, "Path: %s", tc.path)
		})
	}
}

// Test SecurityService with empty authenticators list
func (suite *SecurityServiceTestSuite) TestProcess_EmptyAuthenticators() {
	service, err := newSecurityService([]AuthenticatorInterface{}, nil, testPublicPaths, apiPermissionEntries, "")
	suite.Require().NoError(err)

	req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)

	ctx, err := service.Process(req)

	assert.Nil(suite.T(), ctx)
	assert.Equal(suite.T(), errNoHandlerFound, err)
}

// Test SecurityService with nil authenticators list
func (suite *SecurityServiceTestSuite) TestProcess_NilAuthenticators() {
	service, err := newSecurityService(nil, nil, testPublicPaths, apiPermissionEntries, "")
	suite.Require().NoError(err)

	req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)

	ctx, err := service.Process(req)

	assert.Nil(suite.T(), ctx)
	assert.Equal(suite.T(), errNoHandlerFound, err)
}

// Test Process with different HTTP methods
func (suite *SecurityServiceTestSuite) TestProcess_DifferentHTTPMethods() {
	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
		http.MethodHead,
	}

	for _, method := range methods {
		suite.Run("Method_"+method, func() {
			req := httptest.NewRequest(method, "/api/test", nil)

			// Reset mocks for each test case
			suite.mockAuth1 = &AuthenticatorInterfaceMock{}
			suite.mockAuth2 = &AuthenticatorInterfaceMock{}
			suite.service.authenticators = []AuthenticatorInterface{suite.mockAuth1, suite.mockAuth2}

			suite.mockAuth1.On("CanHandle", req).Return(true)
			suite.mockAuth1.On("Authenticate", req).Return(suite.testCtx, nil)

			ctx, err := suite.service.Process(req)

			assert.NoError(suite.T(), err)
			assert.NotNil(suite.T(), ctx)

			userID := GetSubject(ctx)
			assert.Equal(suite.T(), "user123", userID)

			suite.mockAuth1.AssertExpectations(suite.T())
		})
	}
}

// Test Process with various public path variations
func (suite *SecurityServiceTestSuite) TestProcess_PublicPathVariations() {
	testCases := []struct {
		name string
		path string
	}{
		// Test case sensitivity and exact matching
		{"OAuth2 with query params", "/oauth2/token?grant_type=authorization_code"},
		{"Gate with fragment", "/gate/login#section"},
		{"Well-known with path", "/oauth2/.well-known/openid_configuration"},
		{"Nested signin path", "/gate/forgot-password/confirm"},
		{"Deep console path", "/console/api/v1/test"},
		{"Health check with query", "/health/liveness?detailed=true"},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)

			// With optional auth, authenticators are checked even for public paths
			suite.mockAuth1.On("CanHandle", req).Return(false)
			suite.mockAuth2.On("CanHandle", req).Return(false)

			ctx, err := suite.service.Process(req)

			assert.NoError(suite.T(), err, "Path should be public: %s", tc.path)
			assert.NotNil(suite.T(), ctx)
			assert.True(suite.T(), IsRuntimeContext(ctx), "public path should return a runtime context: %s", tc.path)
		})
	}
}

// Test OPTIONS method bypasses authentication
func (suite *SecurityServiceTestSuite) TestProcess_OptionsMethod() {
	req := httptest.NewRequest(http.MethodOptions, "/api/protected", nil)

	ctx, err := suite.service.Process(req)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), req.Context(), ctx)

	// Verify no authenticators were called for OPTIONS method
	suite.mockAuth1.AssertNotCalled(suite.T(), "CanHandle")
	suite.mockAuth2.AssertNotCalled(suite.T(), "CanHandle")
}

// TestNewSecurityService_Error verifies that newSecurityService returns an error
// when either the public path patterns or the API permission entry patterns are invalid.
func (suite *SecurityServiceTestSuite) TestNewSecurityService_Error() {
	tests := []struct {
		name        string
		publicPaths []string
		apiPerms    []apiPermissionEntry
		errContains string
	}{
		{
			name:        "invalid public path pattern",
			publicPaths: []string{"/valid", "/invalid/**/middle/**"},
			apiPerms:    apiPermissionEntries,
			errContains: "invalid pattern",
		},
		{
			name:        "invalid API permission entry pattern",
			publicPaths: []string{},
			apiPerms:    []apiPermissionEntry{{"GET /invalid/**/middle/**", "system:user"}},
			errContains: "invalid pattern",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			service, err := newSecurityService(nil, nil, tt.publicPaths, tt.apiPerms, "")
			assert.Error(suite.T(), err)
			assert.Nil(suite.T(), service)
			assert.Contains(suite.T(), err.Error(), tt.errContains)
		})
	}
}

// Test Process method with public path and valid token
func (suite *SecurityServiceTestSuite) TestProcess_PublicPath_WithToken() {
	req := httptest.NewRequest(http.MethodGet, "/gate/login", nil)
	// Add Authorization header to simulate optional auth
	req.Header.Add("Authorization", "Bearer valid_token")

	// First authenticator can handle the request
	suite.mockAuth1.On("CanHandle", req).Return(true)
	suite.mockAuth1.On("Authenticate", req).Return(suite.testCtx, nil)

	ctx, err := suite.service.Process(req)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), ctx)

	// Verify authentication context is added
	userID := GetSubject(ctx)
	assert.Equal(suite.T(), "user123", userID)
}

// Test Process method with public path and invalid token (optional auth failure)
func (suite *SecurityServiceTestSuite) TestProcess_PublicPath_WithInvalidToken() {
	req := httptest.NewRequest(http.MethodGet, "/gate/login", nil)
	// Add Authorization header
	req.Header.Add("Authorization", "Bearer invalid_token")

	authError := errors.New("invalid token")

	// First authenticator handles it but fails
	suite.mockAuth1.On("CanHandle", req).Return(true)
	suite.mockAuth1.On("Authenticate", req).Return(nil, authError)

	ctx, err := suite.service.Process(req)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), ctx)
	assert.True(suite.T(), IsRuntimeContext(ctx), "public path with invalid token should return a runtime context")

	userID := GetSubject(ctx)
	assert.Empty(suite.T(), userID)
}

// Test that the service returns errInsufficientPermissions when the authenticated
// subject lacks the required permission for a protected path.
func (suite *SecurityServiceTestSuite) TestProcess_AuthorizationFailure_InsufficientPermissions() {
	req := httptest.NewRequest(http.MethodGet, "/api/protected", nil)

	// Subject has no useful permissions — /api/protected requires "system".
	unprivCtx := newSecurityContext("user123", "ou456", "test_token", []string{"other"}, nil)

	suite.mockAuth1.On("CanHandle", req).Return(true)
	suite.mockAuth1.On("Authenticate", req).Return(unprivCtx, nil)

	ctx, err := suite.service.Process(req)

	assert.Nil(suite.T(), ctx)
	assert.ErrorIs(suite.T(), err, errInsufficientPermissions)
}
