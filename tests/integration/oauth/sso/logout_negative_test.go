// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sso

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	logoutEndpoint         = "/oauth2/logout"
	logoutCallbackEndpoint = "/oauth2/logout/callback"

	// unregisteredPostLogoutRedirectURI is deliberately absent from the client's
	// postLogoutRedirectUris list: it stands in for an attacker-supplied landing page and must be
	// refused by the open-redirect guard.
	unregisteredPostLogoutRedirectURI = "https://attacker.example.com/logged-out"

	// foreignIssuer is an issuer this server never mints tokens for.
	foreignIssuer = "https://not-thunderid.example.com"

	// Fixtures for the short-lived ID token application used to obtain a genuinely expired,
	// server-signed id_token_hint. It is created and deleted inside the single test that needs it.
	expiredHintClientID     = "sso_logout_expired_hint_client"
	expiredHintClientSecret = "sso_logout_expired_hint_secret" //nolint:gosec // test credential
	expiredHintAppName      = "SSOLogoutExpiredHintApp"
	expiredHintUsername     = "sso_logout_expired_hint_user"
	// expiredHintIDTokenValidity is the per-application ID token lifetime, in seconds, that makes
	// the issued id_token expire almost immediately.
	expiredHintIDTokenValidity = 1

	// Error bodies written by the logout endpoint. Every rejection is text/plain, never a JSON
	// OAuth error and never a redirect back to the RP.
	errInvalidIDTokenHint     = "invalid id_token_hint"
	errClientMismatch         = "client_id does not match id_token_hint"
	errClientRequired         = "id_token_hint or client_id is required"
	errInvalidClient          = "invalid client"
	errInvalidPostLogoutRedir = "invalid post_logout_redirect_uri"
	errInvalidRequest         = "invalid request"
)

// TestLogoutOverGETRedirectsToGateSignOutPage exercises the GET binding of the
// end_session_endpoint: with a valid id_token_hint it must redirect to the gate sign-out page
// carrying the sign-out flow executionId and the logoutId, exactly as the POST binding does.
func (ts *SSOLogoutTestSuite) TestLogoutOverGETRedirectsToGateSignOutPage() {
	client := ts.newSessionClient()
	idToken := ts.login(client, logoutUsername, "logout_get_state_1")

	params := url.Values{}
	params.Set("id_token_hint", idToken)
	params.Set("post_logout_redirect_uri", postLogoutRedirectURI)
	params.Set("state", "logout_get_state_2")

	status, location, body := ts.rawLogout(client, http.MethodGet, params)
	executionID, logoutID := ts.requireGateSignOutRedirect(status, location, body, ts.applicationID)
	ts.NotEmpty(executionID, "the GET binding should carry the sign-out flow execution id")
	ts.NotEmpty(logoutID, "the GET binding should carry the logout id")
}

// TestLogoutAcceptsExpiredIDTokenHint pins the OIDC RP-Initiated Logout requirement that an
// expired id_token_hint is still a valid hint: the endpoint verifies the signature and the issuer
// but must not enforce exp. The hint used here is a real, server-issued ID token obtained from a
// throwaway application configured with a one second ID token lifetime, so it is genuinely expired
// and genuinely signed by this server (an unsigned or foreign-signed token would be rejected for a
// different reason and would not prove anything about expiry handling).
func (ts *SSOLogoutTestSuite) TestLogoutAcceptsExpiredIDTokenHint() {
	appID, userID := ts.createShortLivedIDTokenApp()
	defer func() {
		ts.deleteAppByID(appID)
		if err := testutils.DeleteUser(userID); err != nil {
			ts.T().Logf("Failed to delete expired-hint test user %s: %v", userID, err)
		}
	}()

	client := ts.newSessionClient()
	idToken := ts.loginWithClient(client, expiredHintClientID, expiredHintClientSecret,
		expiredHintUsername, "logout_expired_state_1")

	// Wait out the one second ID token lifetime and prove the hint really is expired before using it.
	time.Sleep((expiredHintIDTokenValidity + 1) * time.Second)
	claims, err := testutils.DecodeJWT(idToken)
	ts.Require().NoError(err, "failed to decode the id_token used as a hint")
	ts.Require().Less(int64(claims.Exp), time.Now().Unix(),
		"the id_token_hint must already be expired for this test to mean anything")

	params := url.Values{}
	params.Set("id_token_hint", idToken)
	params.Set("state", "logout_expired_state_2")

	status, location, body := ts.rawLogout(client, http.MethodPost, params)
	_, logoutID := ts.requireGateSignOutRedirect(status, location, body, appID)
	ts.NotEmpty(logoutID, "an expired id_token_hint must be accepted per OIDC RP-Initiated Logout")
}

// TestLogoutRejectsMalformedIDTokenHint checks that a hint which is not a JWT at all is refused.
func (ts *SSOLogoutTestSuite) TestLogoutRejectsMalformedIDTokenHint() {
	params := url.Values{}
	params.Set("id_token_hint", "not-a-jwt")

	status, _, body := ts.rawLogout(ts.newSessionClient(), http.MethodPost, params)
	ts.Equal(http.StatusBadRequest, status, "a non-JWT id_token_hint must be rejected")
	ts.Equal(errInvalidIDTokenHint, strings.TrimSpace(body))
}

// TestLogoutRejectsForeignSignedIDTokenHint forges an ID token that is well formed and carries the
// correct issuer and audience but is signed with a key this server does not hold. It must be
// refused: accepting it would let anyone terminate any session.
func (ts *SSOLogoutTestSuite) TestLogoutRejectsForeignSignedIDTokenHint() {
	foreignKey, err := rsa.GenerateKey(rand.Reader, 2048)
	ts.Require().NoError(err, "failed to generate a foreign signing key")

	// The forged token claims the server's own kid so the server resolves its real verification key
	// and the failure is a genuine signature mismatch rather than an unknown-key lookup miss.
	hint := ts.mintJWT(foreignKey, ts.serverSigningKID(), map[string]interface{}{
		"iss": ts.serverIssuer(),
		"aud": clientID,
		"sub": "sso-logout-foreign-subject",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	params := url.Values{}
	params.Set("id_token_hint", hint)

	status, _, body := ts.rawLogout(ts.newSessionClient(), http.MethodPost, params)
	ts.Equal(http.StatusBadRequest, status, "an id_token_hint signed by a foreign key must be rejected")
	ts.Equal(errInvalidIDTokenHint, strings.TrimSpace(body))
}

// TestLogoutRejectsIDTokenHintWithForeignIssuer isolates the issuer check. Both hints used here are
// signed with the server's own signing key, so the signature check passes for each and only the iss
// claim differs: the control hint (correct issuer) is accepted, the foreign-issuer hint is refused.
// Without the control a broken minting routine would produce the same rejection and the test would
// pass for the wrong reason.
func (ts *SSOLogoutTestSuite) TestLogoutRejectsIDTokenHintWithForeignIssuer() {
	serverKey := ts.readServerSigningKey()
	kid := ts.serverSigningKID()

	control := ts.mintJWT(serverKey, kid, map[string]interface{}{
		"iss": ts.serverIssuer(),
		"aud": clientID,
		"sub": "sso-logout-issuer-control",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	controlParams := url.Values{}
	controlParams.Set("id_token_hint", control)
	controlStatus, controlLocation, controlBody := ts.rawLogout(ts.newSessionClient(), http.MethodPost, controlParams)
	ts.Require().Equal(http.StatusFound, controlStatus,
		"the control hint proves the minted signature is accepted by the server: %s", strings.TrimSpace(controlBody))
	ts.Require().NotEmpty(controlLocation, "the control hint should redirect to the gate sign-out page")

	foreign := ts.mintJWT(serverKey, kid, map[string]interface{}{
		"iss": foreignIssuer,
		"aud": clientID,
		"sub": "sso-logout-issuer-foreign",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	foreignParams := url.Values{}
	foreignParams.Set("id_token_hint", foreign)

	status, _, body := ts.rawLogout(ts.newSessionClient(), http.MethodPost, foreignParams)
	ts.Equal(http.StatusBadRequest, status, "an id_token_hint issued by another issuer must be rejected")
	ts.Equal(errInvalidIDTokenHint, strings.TrimSpace(body))
}

// TestLogoutRejectsAccessTokenAsIDTokenHint isolates the token type check. The access token from a
// real login is signed by this server and carries the correct issuer, and its aud falls back to the
// client id because the application configures no defaultAudience, so it satisfies every other check
// the endpoint makes. Supplying any hint suppresses the End-User sign-out confirmation, so accepting
// a non ID token would hand that suppression to any holder of an access token for the client. The
// id_token from the same login is the control: it is still accepted, which proves the rejection is
// about the token type rather than the session, the client or the issuer.
func (ts *SSOLogoutTestSuite) TestLogoutRejectsAccessTokenAsIDTokenHint() {
	tokens := ts.loginTokens(ts.newSessionClient(), logoutUsername, "logout_hint_type_state_1")
	ts.Require().NotEmpty(tokens.AccessToken, "the login should issue an access token")
	ts.Require().NotEmpty(tokens.IDToken, "the login should issue an id_token")

	controlParams := url.Values{}
	controlParams.Set("id_token_hint", tokens.IDToken)
	controlStatus, controlLocation, controlBody := ts.rawLogout(ts.newSessionClient(), http.MethodPost, controlParams)
	ts.Require().Equal(http.StatusFound, controlStatus,
		"the id_token from the same login proves the hint is otherwise acceptable: %s", strings.TrimSpace(controlBody))
	ts.Require().NotEmpty(controlLocation, "the control hint should redirect to the gate sign-out page")

	params := url.Values{}
	params.Set("id_token_hint", tokens.AccessToken)

	status, _, body := ts.rawLogout(ts.newSessionClient(), http.MethodPost, params)
	ts.Equal(http.StatusBadRequest, status, "an access token must not be accepted as an id_token_hint")
	ts.Equal(errInvalidIDTokenHint, strings.TrimSpace(body))
}

// TestLogoutRejectsRequestWithoutHintOrClientID checks the endpoint refuses a request that names no
// client at all, rather than guessing one.
func (ts *SSOLogoutTestSuite) TestLogoutRejectsRequestWithoutHintOrClientID() {
	params := url.Values{}
	params.Set("state", "logout_no_client_state")

	status, _, body := ts.rawLogout(ts.newSessionClient(), http.MethodPost, params)
	ts.Equal(http.StatusBadRequest, status, "a logout request naming no client must be rejected")
	ts.Equal(errClientRequired, strings.TrimSpace(body))
}

// TestLogoutWithClientIDOnlyIsAccepted covers the prompt-required path: with no id_token_hint the
// spec requires the OP to confirm the logout with the End-User, and the endpoint still resolves the
// target from client_id and starts the sign-out flow.
func (ts *SSOLogoutTestSuite) TestLogoutWithClientIDOnlyIsAccepted() {
	client := ts.newSessionClient()
	ts.login(client, logoutUsername, "logout_client_only_state_1")

	params := url.Values{}
	params.Set("client_id", clientID)
	params.Set("state", "logout_client_only_state_2")

	status, location, body := ts.rawLogout(client, http.MethodPost, params)
	executionID, logoutID := ts.requireGateSignOutRedirect(status, location, body, ts.applicationID)
	ts.NotEmpty(executionID, "a client_id-only logout should still start the sign-out flow")
	ts.NotEmpty(logoutID)
}

// TestLogoutRejectsClientIDNotMatchingHint checks the endpoint refuses a request whose client_id
// contradicts the audience of the supplied id_token_hint.
func (ts *SSOLogoutTestSuite) TestLogoutRejectsClientIDNotMatchingHint() {
	client := ts.newSessionClient()
	idToken := ts.login(client, logoutUsername, "logout_mismatch_state_1")

	params := url.Values{}
	params.Set("id_token_hint", idToken)
	params.Set("client_id", "some_other_client")

	status, _, body := ts.rawLogout(client, http.MethodPost, params)
	ts.Equal(http.StatusBadRequest, status, "a client_id contradicting the hint must be rejected")
	ts.Equal(errClientMismatch, strings.TrimSpace(body))
}

// TestLogoutRejectsUnknownClientID checks that an unresolvable client_id is refused.
func (ts *SSOLogoutTestSuite) TestLogoutRejectsUnknownClientID() {
	params := url.Values{}
	params.Set("client_id", "sso_logout_no_such_client")

	status, _, body := ts.rawLogout(ts.newSessionClient(), http.MethodPost, params)
	ts.Equal(http.StatusBadRequest, status, "an unknown client_id must be rejected")
	ts.Equal(errInvalidClient, strings.TrimSpace(body))
}

// TestLogoutRejectsUnregisteredPostLogoutRedirectURI is the open-redirect guard: even with a valid
// id_token_hint, a post_logout_redirect_uri outside the client's registered list must be refused,
// and the rejection must not bounce the browser to the supplied URI.
func (ts *SSOLogoutTestSuite) TestLogoutRejectsUnregisteredPostLogoutRedirectURI() {
	client := ts.newSessionClient()
	idToken := ts.login(client, logoutUsername, "logout_open_redirect_state_1")

	params := url.Values{}
	params.Set("id_token_hint", idToken)
	params.Set("post_logout_redirect_uri", unregisteredPostLogoutRedirectURI)
	params.Set("state", "logout_open_redirect_state_2")

	status, location, body := ts.rawLogout(client, http.MethodPost, params)
	ts.Equal(http.StatusBadRequest, status, "an unregistered post_logout_redirect_uri must be rejected")
	ts.Equal(errInvalidPostLogoutRedir, strings.TrimSpace(body))
	ts.Empty(location, "a rejected logout must not redirect the browser anywhere")
	ts.NotContains(body, unregisteredPostLogoutRedirectURI,
		"the rejection must not echo the attacker-supplied URI back to the browser")
}

// TestLogoutHonoursPostLogoutRedirectURIWithoutHint proves a registered post_logout_redirect_uri is
// honoured on a request carrying only client_id: the client's registered list is the OP's means of
// confirming the landing page, so no id_token_hint is required for it to take effect. The request is
// driven all the way through the sign-out flow to the completion callback, which returns the
// validated URI with state appended.
func (ts *SSOLogoutTestSuite) TestLogoutHonoursPostLogoutRedirectURIWithoutHint() {
	client := ts.newSessionClient()
	ts.login(client, logoutUsername, "logout_no_hint_redirect_state_1")

	params := url.Values{}
	params.Set("client_id", clientID)
	params.Set("post_logout_redirect_uri", postLogoutRedirectURI)
	params.Set("state", "logout_no_hint_redirect_state_2")

	status, location, body := ts.rawLogout(client, http.MethodPost, params)
	executionID, logoutID := ts.requireGateSignOutRedirect(status, location, body, ts.applicationID)
	ts.Require().NotEmpty(executionID)
	ts.Require().NotEmpty(logoutID)

	step := ts.flowExecute(client, map[string]interface{}{"executionId": executionID})
	ts.Require().Equal("COMPLETE", step.FlowStatus, "the sign-out flow should complete")

	redirect := ts.completeLogout(client, logoutID)
	parsed, err := url.Parse(redirect)
	ts.Require().NoError(err, "failed to parse post-logout redirect")
	ts.Equal(postLogoutRedirectURI, parsed.Scheme+"://"+parsed.Host+parsed.Path,
		"a registered post_logout_redirect_uri must be honoured without an id_token_hint")
	ts.Equal("logout_no_hint_redirect_state_2", parsed.Query().Get("state"))
}

// TestLogoutRejectsUnregisteredPostLogoutRedirectURIWithoutHint checks the open-redirect guard also
// applies on the client_id-only path, where there is no id_token_hint to vouch for the request.
func (ts *SSOLogoutTestSuite) TestLogoutRejectsUnregisteredPostLogoutRedirectURIWithoutHint() {
	params := url.Values{}
	params.Set("client_id", clientID)
	params.Set("post_logout_redirect_uri", unregisteredPostLogoutRedirectURI)

	status, location, body := ts.rawLogout(ts.newSessionClient(), http.MethodPost, params)
	ts.Equal(http.StatusBadRequest, status,
		"an unregistered post_logout_redirect_uri must be rejected without an id_token_hint too")
	ts.Equal(errInvalidPostLogoutRedir, strings.TrimSpace(body))
	ts.Empty(location, "a rejected logout must not redirect the browser anywhere")
}

// TestLogoutCallbackReturnsEmptyRedirectWhenNoneRequested checks that a logout carrying no
// post_logout_redirect_uri completes normally and the callback reports an empty redirect, leaving
// the gate to decide where to land the browser.
func (ts *SSOLogoutTestSuite) TestLogoutCallbackReturnsEmptyRedirectWhenNoneRequested() {
	client := ts.newSessionClient()
	idToken := ts.login(client, logoutUsername, "logout_no_redirect_state_1")

	params := url.Values{}
	params.Set("id_token_hint", idToken)

	status, location, body := ts.rawLogout(client, http.MethodPost, params)
	executionID, logoutID := ts.requireGateSignOutRedirect(status, location, body, ts.applicationID)

	step := ts.flowExecute(client, map[string]interface{}{"executionId": executionID})
	ts.Require().Equal("COMPLETE", step.FlowStatus, "the sign-out flow should complete")

	ts.Empty(ts.completeLogout(client, logoutID),
		"with no post_logout_redirect_uri requested the callback should report an empty redirect")
}

// TestLogoutCallbackIsSingleUse checks a logout id cannot be replayed: the first callback consumes
// the stored request and returns the post-logout redirect, and a second callback with the same id is
// treated as unknown and reports an empty redirect.
func (ts *SSOLogoutTestSuite) TestLogoutCallbackIsSingleUse() {
	client := ts.newSessionClient()
	idToken := ts.login(client, logoutUsername, "logout_replay_state_1")

	executionID, logoutID := ts.initiateLogout(client, idToken, postLogoutRedirectURI, "logout_replay_state_2")
	step := ts.flowExecute(client, map[string]interface{}{"executionId": executionID})
	ts.Require().Equal("COMPLETE", step.FlowStatus, "the sign-out flow should complete")

	ts.Require().NotEmpty(ts.completeLogout(client, logoutID),
		"the first callback should return the post-logout redirect")
	ts.Empty(ts.completeLogout(client, logoutID),
		"replaying a consumed logout id must not re-issue the post-logout redirect")
}

// TestLogoutCallbackWithUnknownLogoutIDReturnsEmptyRedirect checks an unknown logout id is not an
// error: the callback reports an empty redirect rather than leaking whether the id ever existed.
func (ts *SSOLogoutTestSuite) TestLogoutCallbackWithUnknownLogoutIDReturnsEmptyRedirect() {
	status, body := ts.rawLogoutCallback(ts.newSessionClient(), `{"logoutId":"sso-logout-no-such-id"}`)
	ts.Require().Equal(http.StatusOK, status, "an unknown logout id should not fail the callback: %s", body)

	var out struct {
		RedirectURI string `json:"redirect_uri"`
	}
	ts.Require().NoError(json.Unmarshal([]byte(body), &out), "failed to decode logout callback response")
	ts.Empty(out.RedirectURI, "an unknown logout id must not yield a redirect")
}

// TestLogoutCallbackRejectsInvalidBody checks the callback refuses bodies it cannot act on: an empty
// body, malformed JSON, and a well formed body carrying no logout id.
func (ts *SSOLogoutTestSuite) TestLogoutCallbackRejectsInvalidBody() {
	cases := map[string]string{
		"empty body":     "",
		"malformed JSON": `{"logoutId":`,
		"empty logoutId": `{"logoutId":""}`,
	}

	for name, payload := range cases {
		status, body := ts.rawLogoutCallback(ts.newSessionClient(), payload)
		ts.Equal(http.StatusBadRequest, status, "%s should be rejected by the logout callback", name)
		ts.Equal(errInvalidRequest, strings.TrimSpace(body), "%s should be rejected as an invalid request", name)
	}
}

// TestLogoutEndpointsAnswerPreflight checks the CORS preflight on both logout endpoints, which a
// browser issues before a cross-origin call.
func (ts *SSOLogoutTestSuite) TestLogoutEndpointsAnswerPreflight() {
	client := ts.newSessionClient()

	for _, endpoint := range []string{logoutEndpoint, logoutCallbackEndpoint} {
		req, err := http.NewRequest(http.MethodOptions, testutils.TestServerURL+endpoint, nil)
		ts.Require().NoError(err)
		req.Header.Set("Origin", redirectURI)
		req.Header.Set("Access-Control-Request-Method", http.MethodPost)

		resp, err := client.Do(req)
		ts.Require().NoError(err, "preflight request to %s failed", endpoint)
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		ts.Equal(http.StatusNoContent, resp.StatusCode,
			"preflight on %s should return 204: %s", endpoint, string(body))
	}
}

// rawLogout issues an RP-initiated logout request over the given HTTP method and returns the raw
// status, Location header and body. Unlike initiateLogout it asserts nothing, so a test can express
// a rejection outcome.
func (ts *SSOLogoutTestSuite) rawLogout(
	client *http.Client, method string, params url.Values,
) (int, string, string) {
	var req *http.Request
	var err error
	if method == http.MethodGet {
		req, err = http.NewRequest(http.MethodGet, testutils.TestServerURL+logoutEndpoint+"?"+params.Encode(), nil)
		ts.Require().NoError(err)
	} else {
		req, err = http.NewRequest(method, testutils.TestServerURL+logoutEndpoint, strings.NewReader(params.Encode()))
		ts.Require().NoError(err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	resp, err := client.Do(req)
	ts.Require().NoError(err, "logout request failed")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err, "failed to read logout response body")
	return resp.StatusCode, resp.Header.Get("Location"), string(body)
}

// rawLogoutCallback posts a raw body to the completion callback and returns the status and body,
// so a test can assert on a rejection.
func (ts *SSOLogoutTestSuite) rawLogoutCallback(client *http.Client, payload string) (int, string) {
	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+logoutCallbackEndpoint,
		strings.NewReader(payload))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	ts.Require().NoError(err, "logout callback request failed")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err, "failed to read logout callback response body")
	return resp.StatusCode, string(body)
}

// requireGateSignOutRedirect asserts the response is the gate sign-out redirect for the given
// application and returns the executionId and logoutId it carries.
func (ts *SSOLogoutTestSuite) requireGateSignOutRedirect(
	status int, location, body, expectedAppID string,
) (string, string) {
	ts.Require().Equal(http.StatusFound, status, "logout should redirect to the gate sign-out page: %s", body)
	ts.Require().NotEmpty(location, "the gate sign-out redirect should carry a Location header")

	parsed, err := url.Parse(location)
	ts.Require().NoError(err, "failed to parse gate sign-out redirect")
	ts.Require().True(strings.HasSuffix(parsed.Path, "/signout"),
		"logout should land on the gate sign-out page, got %q", parsed.Path)

	query := parsed.Query()
	ts.Require().Equal(expectedAppID, query.Get("applicationId"),
		"the gate sign-out redirect should identify the target application")
	return query.Get("executionId"), query.Get("logoutId")
}

// serverIssuer returns the issuer this server mints tokens with, read from its OIDC discovery
// document rather than assumed.
func (ts *SSOLogoutTestSuite) serverIssuer() string {
	doc := ts.discoveryDocument()
	issuer, ok := doc["issuer"].(string)
	ts.Require().True(ok, "the discovery document should carry an issuer")
	ts.Require().NotEmpty(issuer)
	return issuer
}

// serverSigningKID returns the key id of the server's RSA signing key, read from its published JWKS.
func (ts *SSOLogoutTestSuite) serverSigningKID() string {
	jwksURI, ok := ts.discoveryDocument()["jwks_uri"].(string)
	ts.Require().True(ok, "the discovery document should carry a jwks_uri")

	resp, err := ts.newSessionClient().Get(jwksURI)
	ts.Require().NoError(err, "failed to fetch the server JWKS")
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode, "the JWKS endpoint should return 200")

	var jwks struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			Alg string `json:"alg"`
		} `json:"keys"`
	}
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&jwks), "failed to decode the server JWKS")

	for _, key := range jwks.Keys {
		if key.Kty == "RSA" {
			return key.Kid
		}
	}
	ts.Require().Fail("the server JWKS should publish an RSA signing key")
	return ""
}

// discoveryDocument fetches the OIDC discovery document.
func (ts *SSOLogoutTestSuite) discoveryDocument() map[string]interface{} {
	resp, err := ts.newSessionClient().Get(testutils.TestServerURL + "/.well-known/openid-configuration")
	ts.Require().NoError(err, "failed to fetch the discovery document")
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode, "the discovery endpoint should return 200")

	var doc map[string]interface{}
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&doc), "failed to decode the discovery document")
	return doc
}

// readServerSigningKey loads the RSA signing key of the product under test from the extracted
// deployment. Forging a hint with the server's own key is the only way to reach the issuer check:
// any other key fails the signature check first, which would collapse the issuer case into the
// foreign-key case.
func (ts *SSOLogoutTestSuite) readServerSigningKey() *rsa.PrivateKey {
	home := os.Getenv("SERVER_EXTRACTED_HOME")
	ts.Require().NotEmpty(home, "SERVER_EXTRACTED_HOME should be exported by the integration test harness")

	pemBytes, err := os.ReadFile(filepath.Join(home, "config", "certs", "signing.key"))
	ts.Require().NoError(err, "failed to read the server signing key")

	block, _ := pem.Decode(pemBytes)
	ts.Require().NotNil(block, "the server signing key should be PEM encoded")

	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	ts.Require().NoError(err, "failed to parse the server signing key")

	key, ok := parsed.(*rsa.PrivateKey)
	ts.Require().True(ok, "the server signing key should be an RSA key")
	return key
}

// mintJWT hand-builds a compact RS256 JWT with the given key, key id and claims.
func (ts *SSOLogoutTestSuite) mintJWT(key *rsa.PrivateKey, kid string, claims map[string]interface{}) string {
	header, err := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": kid})
	ts.Require().NoError(err, "failed to encode the JWT header")
	payload, err := json.Marshal(claims)
	ts.Require().NoError(err, "failed to encode the JWT claims")

	signingInput := base64.RawURLEncoding.EncodeToString(header) + "." +
		base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(signingInput))

	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	ts.Require().NoError(err, "failed to sign the JWT")
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
}

// createShortLivedIDTokenApp registers an application whose ID tokens live for one second, plus a
// user allowed to log in through it, and returns their ids. It exists so a test can obtain a real,
// server-signed id_token that is already expired.
func (ts *SSOLogoutTestSuite) createShortLivedIDTokenApp() (string, string) {
	app := map[string]interface{}{
		"name":                      expiredHintAppName,
		"description":               "Application issuing near-instantly expiring ID tokens for logout hint tests",
		"ouId":                      testOUID,
		"type":                      "fullstack",
		"authFlowId":                ts.authFlowID,
		"isRegistrationFlowEnabled": false,
		"signOutFlowId":             ts.signOutFlowID,
		"allowedUserTypes":          []string{testUserType.Name},
		"inboundAuthConfig": []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                expiredHintClientID,
					"clientSecret":            expiredHintClientSecret,
					"redirectUris":            []string{redirectURI},
					"grantTypes":              []string{"authorization_code", "refresh_token"},
					"responseTypes":           []string{"code"},
					"tokenEndpointAuthMethod": "client_secret_basic",
					"scopes":                  []string{"openid"},
					"token": map[string]interface{}{
						"idToken": map[string]interface{}{
							"validityPeriod": expiredHintIDTokenValidity,
						},
					},
				},
			},
		},
	}

	jsonData, err := json.Marshal(app)
	ts.Require().NoError(err, "failed to marshal the short-lived ID token application")

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/applications", bytes.NewBuffer(jsonData))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err, "failed to create the short-lived ID token application")
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	ts.Require().Equal(http.StatusCreated, resp.StatusCode,
		"failed to create the short-lived ID token application: %s", string(body))

	var respData map[string]interface{}
	ts.Require().NoError(json.Unmarshal(body, &respData), "failed to parse the application response")
	appID, ok := respData["id"].(string)
	ts.Require().True(ok, "the application response should carry an id")

	userID, err := testutils.CreateUser(testutils.User{
		OUID: testOUID,
		Type: testUserType.Name,
		Attributes: json.RawMessage(fmt.Sprintf(`{
			"username": "%s",
			"password": "%s",
			"email": "%s@example.com"
		}`, expiredHintUsername, testPassword, expiredHintUsername)),
	})
	ts.Require().NoError(err, "failed to create the expired-hint test user")

	return appID, userID
}

// loginWithClient drives a first-time login through the given OAuth client to completion and returns
// the issued id_token. It mirrors loginTokens, which is bound to the suite's primary client.
func (ts *SSOLogoutTestSuite) loginWithClient(
	client *http.Client, oauthClientID, oauthClientSecret, username, state string,
) string {
	params := url.Values{}
	params.Set("client_id", oauthClientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	params.Set("scope", "openid")
	params.Set("state", state)

	req, err := http.NewRequest("GET", testutils.TestServerURL+"/oauth2/authorize?"+params.Encode(), nil)
	ts.Require().NoError(err)

	resp, err := client.Do(req)
	ts.Require().NoError(err, "authorize request failed")
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusFound, resp.StatusCode, "authorize should redirect to the gate")

	authID, executionID, err := testutils.ExtractAuthData(resp.Header.Get("Location"))
	ts.Require().NoError(err, "failed to extract auth data from the authorize redirect")

	initial := ts.flowExecute(client, map[string]interface{}{"executionId": executionID})
	ts.Require().NotEqual("COMPLETE", initial.FlowStatus, "first login must prompt for credentials")

	step := ts.flowExecute(client, map[string]interface{}{
		"executionId":    executionID,
		"inputs":         map[string]string{"username": username, "password": testPassword},
		"action":         "action_001",
		"challengeToken": initial.ChallengeToken,
	})
	ts.Require().Equal("COMPLETE", step.FlowStatus, "credential login should complete the flow")

	clientRedirect := ts.completeAuthorization(client, authID, step.Assertion)
	code, err := testutils.ExtractAuthorizationCode(clientRedirect)
	ts.Require().NoError(err, "failed to extract the authorization code")

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)

	tokenReq, err := http.NewRequest("POST", testutils.TestServerURL+"/oauth2/token", strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenReq.SetBasicAuth(oauthClientID, oauthClientSecret)

	tokenResp, err := client.Do(tokenReq)
	ts.Require().NoError(err, "token request failed")
	defer tokenResp.Body.Close()

	tokenBody, _ := io.ReadAll(tokenResp.Body)
	ts.Require().Equal(http.StatusOK, tokenResp.StatusCode, "token request failed: %s", string(tokenBody))

	var token testutils.TokenResponse
	ts.Require().NoError(json.Unmarshal(tokenBody, &token), "failed to decode the token response")
	ts.Require().NotEmpty(token.IDToken, "an id_token should be issued for the openid scope")
	return token.IDToken
}
