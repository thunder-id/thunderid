// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package security

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const testManagementToken = "pat-0123456789abcdef"

func newTestManagementAuthenticator(t *testing.T, token string) *managementTokenAuthenticator {
	t.Helper()
	InitSystemPermissions("")
	authenticator, err := newManagementTokenAuthenticator(token, managementTokenPaths)
	require.NoError(t, err)
	return authenticator
}

func requestWithBearer(method, path, token string) *http.Request {
	request := httptest.NewRequest(method, path, nil)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	return request
}

// The token opens the import and variable store APIs, and those only.
func TestManagementTokenHandlesOnlyTheTrustedPaths(t *testing.T) {
	authenticator := newTestManagementAuthenticator(t, testManagementToken)

	trusted := []struct{ method, path string }{
		{http.MethodPost, "/import"},
		{http.MethodPost, "/import/delete"},
		{http.MethodGet, "/variables"},
		{http.MethodPut, "/variables/API_URL"},
		{http.MethodDelete, "/variables/API_URL"},
		{http.MethodGet, "/secrets"},
		{http.MethodPut, "/secrets/DB_PASSWORD"},
	}
	for _, request := range trusted {
		assert.True(t, authenticator.CanHandle(requestWithBearer(request.method, request.path, testManagementToken)),
			"%s %s should accept the management token", request.method, request.path)
	}

	// Everything else stays behind OAuth. If this list ever starts passing, the token has been
	// widened into a master key for the whole management surface.
	refused := []struct{ method, path string }{
		{http.MethodGet, "/users"},
		{http.MethodPost, "/users"},
		{http.MethodGet, "/applications"},
		{http.MethodPost, "/organization-units"},
		{http.MethodGet, "/user-types"},
		{http.MethodPost, "/groups"},
		{http.MethodGet, "/flows"},
		{http.MethodGet, "/users/me"},
	}
	for _, request := range refused {
		assert.False(t, authenticator.CanHandle(requestWithBearer(request.method, request.path, testManagementToken)),
			"%s %s must not accept the management token", request.method, request.path)
	}
}

// A wrong token is not claimed at all, so the request falls through to the JWT authenticator rather
// than being rejected outright. This is what keeps OAuth working on these same paths.
func TestAWrongTokenIsLeftToTheJWTAuthenticator(t *testing.T) {
	authenticator := newTestManagementAuthenticator(t, testManagementToken)

	for _, presented := range []string{
		"some-other-token",
		"eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.e30.signature",
		testManagementToken + "x",
		testManagementToken[:len(testManagementToken)-1],
		"",
	} {
		assert.False(t, authenticator.CanHandle(requestWithBearer(http.MethodPost, "/import", presented)),
			"a bearer value of %q must not be claimed", presented)
	}
}

// With no token configured the mechanism is off entirely, which is the default.
func TestNoConfiguredTokenHandlesNothing(t *testing.T) {
	authenticator := newTestManagementAuthenticator(t, "")

	assert.False(t, authenticator.CanHandle(requestWithBearer(http.MethodPost, "/import", "")))
	assert.False(t, authenticator.CanHandle(requestWithBearer(http.MethodPost, "/import", "anything")))
	// An empty configured token must not be matched by an empty presented one.
	assert.False(t, hasManagementToken(""))
	assert.False(t, hasManagementToken("   "))
}

func TestAuthenticateGrantsTheRootSystemPermission(t *testing.T) {
	authenticator := newTestManagementAuthenticator(t, testManagementToken)

	securityCtx, err := authenticator.Authenticate(
		requestWithBearer(http.MethodPost, "/import", testManagementToken))

	require.NoError(t, err)
	require.NotNil(t, securityCtx)
	assert.Equal(t, managementTokenSubject, securityCtx.subject)
	assert.True(t, HasSufficientPermission(securityCtx.permissions, GetSystemRootPermission()))
	// The credential is not carried into the context.
	assert.Empty(t, securityCtx.token)
	// There is nothing to revoke, so the revocation step must see an empty identity and pass.
	assert.Empty(t, securityCtx.revocationID)
	assert.Empty(t, securityCtx.tokenFamilyID)
}

// Authenticate re-checks rather than trusting that CanHandle ran, so a direct call cannot bypass it.
func TestAuthenticateRefusesWhatItCannotHandle(t *testing.T) {
	authenticator := newTestManagementAuthenticator(t, testManagementToken)

	_, err := authenticator.Authenticate(requestWithBearer(http.MethodPost, "/import", "wrong"))
	assert.Error(t, err)

	_, err = authenticator.Authenticate(requestWithBearer(http.MethodGet, "/users", testManagementToken))
	assert.Error(t, err, "a trusted token on an untrusted path must still fail")
}

// A missing or malformed Authorization header is not claimed.
func TestMalformedAuthorizationHeaderIsNotClaimed(t *testing.T) {
	authenticator := newTestManagementAuthenticator(t, testManagementToken)

	for _, header := range []string{"", "Basic " + testManagementToken, testManagementToken, "Bearer"} {
		request := httptest.NewRequest(http.MethodPost, "/import", nil)
		if header != "" {
			request.Header.Set("Authorization", header)
		}
		assert.False(t, authenticator.CanHandle(request), "header %q must not be claimed", header)
	}
}

// The scheme is case-insensitive per RFC 7235 §2.1, as it is for the JWT authenticator.
func TestBearerSchemeIsCaseInsensitive(t *testing.T) {
	authenticator := newTestManagementAuthenticator(t, testManagementToken)

	request := httptest.NewRequest(http.MethodPost, "/import", nil)
	request.Header.Set("Authorization", "bearer "+testManagementToken)

	assert.True(t, authenticator.CanHandle(request))
}

// The tests above check the authenticator in isolation. These run a request through the whole
// security pipeline — authenticate, revocation, authorize — because that is what decides whether
// the mechanism actually works, and the authorize step is where a token without the right
// permission would still be refused.
func newPipelineWithManagementToken(t *testing.T) *securityService {
	t.Helper()
	InitSystemPermissions("")

	managementAuthenticator, err := newManagementTokenAuthenticator(testManagementToken, managementTokenPaths)
	require.NoError(t, err)

	// A management token carries no jti and no family id, so the enforcer is handed an empty
	// identity. Asserting that here pins the behavior the real enforcer relies on.
	revocation := &RevocationEnforcerInterfaceMock{}
	revocation.On("EnsureNotRevoked", mock.Anything, mock.MatchedBy(func(identity RevocationIdentity) bool {
		return identity.JTI == "" && identity.TokenFamilyID == ""
	})).Return(nil).Maybe()

	service, err := newSecurityService(
		[]AuthenticatorInterface{managementAuthenticator}, revocation, publicPaths, apiPermissionEntries)
	require.NoError(t, err)
	return service
}

func TestPipelineAdmitsTheTokenOnTheTrustedPaths(t *testing.T) {
	service := newPipelineWithManagementToken(t)

	for _, request := range []struct{ method, path string }{
		{http.MethodPost, "/import"},
		{http.MethodGet, "/variables"},
		{http.MethodPut, "/variables/API_URL"},
		{http.MethodPost, "/secrets"},
		{http.MethodDelete, "/secrets/DB_PASSWORD"},
	} {
		ctx, err := service.Process(requestWithBearer(request.method, request.path, testManagementToken))

		require.NoError(t, err, "%s %s should be admitted", request.method, request.path)
		require.NotNil(t, ctx)
		assert.Equal(t, managementTokenSubject, GetSubject(ctx))
	}
}

// The same valid token on any other management API is refused, and refused at authentication:
// no authenticator claims it, so it never reaches the permission check holding root.
func TestPipelineRefusesTheTokenElsewhere(t *testing.T) {
	service := newPipelineWithManagementToken(t)

	for _, request := range []struct{ method, path string }{
		{http.MethodGet, "/users"},
		{http.MethodPost, "/applications"},
		{http.MethodGet, "/organization-units"},
		{http.MethodPost, "/user-types"},
	} {
		ctx, err := service.Process(requestWithBearer(request.method, request.path, testManagementToken))

		require.Error(t, err, "%s %s must be refused", request.method, request.path)
		assert.Nil(t, ctx)
	}
}

// A public path stays public: the token changes nothing about what needs no credential at all.
func TestPipelineLeavesPublicPathsAlone(t *testing.T) {
	service := newPipelineWithManagementToken(t)

	ctx, err := service.Process(httptest.NewRequest(http.MethodGet, "/health/liveness", nil))

	require.NoError(t, err)
	assert.NotNil(t, ctx)
}
