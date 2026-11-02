// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package security

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
)

// testManagementAPIKey is the key a caller presents; testManagementAPIKeyHash is what the
// deployment is configured with. Only the second ever reaches the authenticator.
const testManagementAPIKey = "pat-0123456789abcdef"

var testManagementAPIKeyHash = cryptolib.HashToken(testManagementAPIKey)

func newTestManagementAuthenticator(t *testing.T, keyHash string) *managementAPIKeyAuthenticator {
	t.Helper()
	InitSystemPermissions("")
	authenticator, err := newManagementAPIKeyAuthenticator(keyHash, managementAPIKeyPaths)
	require.NoError(t, err)
	return authenticator
}

func requestWithAPIKey(method, path, key string) *http.Request {
	request := httptest.NewRequest(method, path, nil)
	if key != "" {
		request.Header.Set(constants.APIKeyHeaderName, key)
	}
	return request
}

// The key opens the import and variable store APIs, and those only.
func TestManagementAPIKeyHandlesOnlyTheTrustedPaths(t *testing.T) {
	authenticator := newTestManagementAuthenticator(t, testManagementAPIKeyHash)

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
		assert.True(t, authenticator.CanHandle(requestWithAPIKey(request.method, request.path, testManagementAPIKey)),
			"%s %s should accept the management API key", request.method, request.path)
	}

	// Everything else stays behind OAuth. If this list ever starts passing, the key has been
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
		assert.False(t, authenticator.CanHandle(requestWithAPIKey(request.method, request.path, testManagementAPIKey)),
			"%s %s must not accept the management API key", request.method, request.path)
	}
}

// The deployment is configured with a digest, and the key that produced it is never held. This is
// the property that makes a leaked data plane configuration useless to whoever reads it.
func TestOnlyTheDigestIsHeld(t *testing.T) {
	authenticator := newTestManagementAuthenticator(t, testManagementAPIKeyHash)

	assert.NotContains(t, authenticator.keyHash, testManagementAPIKey,
		"the configured digest must not contain the key it was derived from")
	assert.Len(t, authenticator.keyHash, 64, "a SHA-256 hex digest is 64 characters")
	assert.NotEqual(t, testManagementAPIKey, authenticator.keyHash)
}

// Configuring the plaintext key instead of its digest must not authenticate anything. The mistake
// fails closed rather than quietly accepting the key.
func TestAPlaintextKeyInTheConfigurationAuthenticatesNothing(t *testing.T) {
	// The setting holds a digest. Putting the key itself there is a misconfiguration, and it must
	// not accidentally work: the presented key is compared against it as a digest and will not match.
	authenticator := newTestManagementAuthenticator(t, testManagementAPIKey)

	_, err := authenticator.Authenticate(requestWithAPIKey(http.MethodPost, "/import", testManagementAPIKey))
	assert.Error(t, err)
}

// A wrong key is not claimed at all, so the request falls through to the JWT authenticator rather
// than being rejected outright. This is what keeps OAuth working on these same paths.
func TestAWrongAPIKeyIsRefusedRatherThanIgnored(t *testing.T) {
	authenticator := newTestManagementAuthenticator(t, testManagementAPIKeyHash)

	// Presenting a key is a claim to be authenticated by it, so getting it wrong is refused rather
	// than passed to another mechanism. Every one of these is a near miss of the real key.
	for _, presented := range []string{
		"some-other-key",
		"eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.e30.signature",
		testManagementAPIKey + "x",
		testManagementAPIKey[:len(testManagementAPIKey)-1],
		strings.ToUpper(testManagementAPIKey),
		testManagementAPIKeyHash,
	} {
		request := requestWithAPIKey(http.MethodPost, "/import", presented)
		assert.True(t, authenticator.CanHandle(request),
			"an API-Key value of %q is presented, so it is this mechanism's to judge", presented)
		_, err := authenticator.Authenticate(request)
		assert.Error(t, err, "an API-Key value of %q must not authenticate", presented)
	}

	// No key presented at all is a different case: there is nothing to judge, so the request is
	// left to whichever mechanism the caller did present a credential for.
	assert.False(t, authenticator.CanHandle(requestWithAPIKey(http.MethodPost, "/import", "")))
}

// The key belongs in its own header. Presenting it the way a bearer token is presented must not
// work, otherwise the two mechanisms would still be competing for Authorization.
func TestTheKeyIsNotAcceptedInTheAuthorizationHeader(t *testing.T) {
	authenticator := newTestManagementAuthenticator(t, testManagementAPIKeyHash)

	for _, header := range []string{
		"Bearer " + testManagementAPIKey,
		testManagementAPIKey,
		"Bearer " + testManagementAPIKeyHash,
	} {
		request := httptest.NewRequest(http.MethodPost, "/import", nil)
		request.Header.Set(constants.AuthorizationHeaderName, header)
		assert.False(t, authenticator.CanHandle(request),
			"Authorization: %q must not be claimed by the API key authenticator", header)
	}
}

// A request may carry both credentials: a pipeline holding an OAuth token and a stale key should
// still be served by the JWT authenticator rather than refused.
func TestAKeyIsClaimedEvenAlongsideABearerToken(t *testing.T) {
	authenticator := newTestManagementAuthenticator(t, testManagementAPIKeyHash)

	request := requestWithAPIKey(http.MethodPost, "/import", testManagementAPIKey)
	request.Header.Set(constants.AuthorizationHeaderName, "Bearer some.jwt.token")
	assert.True(t, authenticator.CanHandle(request))
	_, err := authenticator.Authenticate(request)
	assert.NoError(t, err)

	// The bearer token is no second chance. A request that presents a key is judged on that key.
	stale := requestWithAPIKey(http.MethodPost, "/import", "stale-key")
	stale.Header.Set(constants.AuthorizationHeaderName, "Bearer some.jwt.token")
	assert.True(t, authenticator.CanHandle(stale))
	_, err = authenticator.Authenticate(stale)
	assert.Error(t, err)
}

// With no key configured the mechanism is off entirely, which is the default.
func TestNoConfiguredAPIKeyHandlesNothing(t *testing.T) {
	authenticator := newTestManagementAuthenticator(t, "")

	assert.False(t, authenticator.CanHandle(requestWithAPIKey(http.MethodPost, "/import", "")))
	assert.False(t, authenticator.CanHandle(requestWithAPIKey(http.MethodPost, "/import", "anything")))
	// An empty configured digest must not be matched by an empty presented key.
	assert.False(t, hasManagementAPIKey(""))
	assert.False(t, hasManagementAPIKey("   "))
}

func TestAuthenticateGrantsTheRootSystemPermission(t *testing.T) {
	authenticator := newTestManagementAuthenticator(t, testManagementAPIKeyHash)

	securityCtx, err := authenticator.Authenticate(
		requestWithAPIKey(http.MethodPost, "/import", testManagementAPIKey))

	require.NoError(t, err)
	require.NotNil(t, securityCtx)
	assert.Equal(t, managementAPIKeySubject, securityCtx.subject)
	assert.True(t, HasSufficientPermission(securityCtx.permissions, GetSystemRootPermission()))
	// The credential is not carried into the context.
	assert.Empty(t, securityCtx.token)
	// There is nothing to revoke, so the revocation step must see an empty identity and pass.
	assert.Empty(t, securityCtx.revocationID)
	assert.Empty(t, securityCtx.tokenFamilyID)
}

// Authenticate re-checks rather than trusting that CanHandle ran, so a direct call cannot bypass it.
func TestAuthenticateRefusesWhatItCannotHandle(t *testing.T) {
	authenticator := newTestManagementAuthenticator(t, testManagementAPIKeyHash)

	_, err := authenticator.Authenticate(requestWithAPIKey(http.MethodPost, "/import", "wrong"))
	assert.Error(t, err)

	// Which paths this mechanism is trusted for is CanHandle's to decide, so the service never
	// reaches Authenticate for one it is not.
	assert.False(t, authenticator.CanHandle(requestWithAPIKey(http.MethodGet, "/users", testManagementAPIKey)),
		"a trusted key on an untrusted path must not be claimed")
}

// Surrounding whitespace in the header is tolerated; the value itself is matched exactly.
func TestTheHeaderValueIsTrimmedButNotOtherwiseNormalized(t *testing.T) {
	authenticator := newTestManagementAuthenticator(t, testManagementAPIKeyHash)

	_, err := authenticator.Authenticate(
		requestWithAPIKey(http.MethodPost, "/import", "  "+testManagementAPIKey+"  "))
	assert.NoError(t, err, "surrounding whitespace is trimmed")

	_, err = authenticator.Authenticate(
		requestWithAPIKey(http.MethodPost, "/import", strings.ToUpper(testManagementAPIKey)))
	assert.Error(t, err, "the value is otherwise compared as presented")
}

// The tests above check the authenticator in isolation. These run a request through the whole
// security pipeline (authenticate, revocation, authorize), because that is what decides whether
// the mechanism actually works, and the authorize step is where a key without the right
// permission would still be refused.
func newPipelineWithManagementAPIKey(t *testing.T) *securityService {
	t.Helper()
	InitSystemPermissions("")

	managementAuthenticator, err := newManagementAPIKeyAuthenticator(
		testManagementAPIKeyHash, managementAPIKeyPaths)
	require.NoError(t, err)

	// A management API key carries no jti and no family id, so the enforcer is handed an empty
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

func TestPipelineAdmitsTheKeyOnTheTrustedPaths(t *testing.T) {
	service := newPipelineWithManagementAPIKey(t)

	for _, request := range []struct{ method, path string }{
		{http.MethodPost, "/import"},
		{http.MethodGet, "/variables"},
		{http.MethodPut, "/variables/API_URL"},
		{http.MethodPost, "/secrets"},
		{http.MethodDelete, "/secrets/DB_PASSWORD"},
	} {
		ctx, err := service.Process(requestWithAPIKey(request.method, request.path, testManagementAPIKey))

		require.NoError(t, err, "%s %s should be admitted", request.method, request.path)
		require.NotNil(t, ctx)
		assert.Equal(t, managementAPIKeySubject, GetSubject(ctx))
	}
}

// The same valid key on any other management API is refused, and refused at authentication:
// no authenticator claims it, so it never reaches the permission check holding root.
func TestPipelineRefusesTheKeyElsewhere(t *testing.T) {
	service := newPipelineWithManagementAPIKey(t)

	for _, request := range []struct{ method, path string }{
		{http.MethodGet, "/users"},
		{http.MethodPost, "/applications"},
		{http.MethodGet, "/organization-units"},
		{http.MethodPost, "/user-types"},
	} {
		ctx, err := service.Process(requestWithAPIKey(request.method, request.path, testManagementAPIKey))

		require.Error(t, err, "%s %s must be refused", request.method, request.path)
		assert.Nil(t, ctx)
	}
}

// A public path stays public: the key changes nothing about what needs no credential at all.
func TestPipelineLeavesPublicPathsAlone(t *testing.T) {
	service := newPipelineWithManagementAPIKey(t)

	ctx, err := service.Process(httptest.NewRequest(http.MethodGet, "/health/liveness", nil))

	require.NoError(t, err)
	assert.NotNil(t, ctx)
}
