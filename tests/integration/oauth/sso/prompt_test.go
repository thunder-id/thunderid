// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sso

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// The OIDC prompt parameter interacts with SSO in two opposite directions. prompt=login must force a
// fresh authentication even when a session exists, and prompt=none must be answered from a session
// without any interaction at all, or refused with login_required when there is none.
//
// Each test builds its own cookie jar via newSessionClient so its session is unambiguous regardless
// of the order tests run in.

// authorizeRaw issues an authorize request with the given extra parameters and returns the response
// without asserting its status, so a test can inspect an error redirect back to the client.
func (ts *SSOLogoutTestSuite) authorizeRaw(
	client *http.Client, scope, state string, extra map[string]string,
) *http.Response {
	params := url.Values{}
	params.Set("client_id", clientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	params.Set("scope", scope)
	params.Set("state", state)
	for k, v := range extra {
		params.Set(k, v)
	}

	req, err := http.NewRequest("GET", testutils.TestServerURL+"/oauth2/authorize?"+params.Encode(), nil)
	ts.Require().NoError(err)

	resp, err := client.Do(req)
	ts.Require().NoError(err, "authorize request failed")
	return resp
}

// redirectQuery returns the query parameters of the authorize response's Location header.
func (ts *SSOLogoutTestSuite) redirectQuery(resp *http.Response) url.Values {
	ts.Require().Equal(http.StatusFound, resp.StatusCode, "authorize should redirect")
	location := resp.Header.Get("Location")
	ts.Require().NotEmpty(location, "the redirect should carry a Location header")
	parsed, err := url.Parse(location)
	ts.Require().NoError(err)
	return parsed.Query()
}

// TestPromptLogin_ForcesReauthentication verifies that prompt=login is honored over a live session:
// the SSO-Check node declines to reuse it and the flow prompts for credentials, as OIDC Core
// §3.1.2.1 requires. Reusing the session here would silently ignore the parameter.
func (ts *SSOLogoutTestSuite) TestPromptLogin_ForcesReauthentication() {
	client := ts.newSessionClient()
	ts.login(client, ssoMaxAgeUsername, "prompt_login_1")
	ts.Require().NotEmpty(ts.ssoCookieNames(client), "an SSO cookie should be set after first login")

	resp := ts.authorizeRaw(client, "openid", "prompt_login_2", map[string]string{"prompt": "login"})
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusFound, resp.StatusCode, "authorize should redirect to the gate")

	_, executionID, err := testutils.ExtractAuthData(resp.Header.Get("Location"))
	ts.Require().NoError(err, "failed to extract auth data from authorize redirect")

	step := ts.flowExecute(client, map[string]interface{}{"executionId": executionID})

	ts.Require().NotEqual("COMPLETE", step.FlowStatus,
		"prompt=login must re-authenticate even when a session exists")
	ts.Empty(step.Assertion, "no assertion may be minted before the subject re-authenticates")
}

// TestPromptNone_WithLiveSessionSucceeds: with a live session, prompt=none redirects straight
// back to the client with a code.
func (ts *SSOLogoutTestSuite) TestPromptNone_WithLiveSessionSucceeds() {
	client := ts.newSessionClient()
	ts.login(client, ssoMaxAgeUsername, "prompt_none_live_1")
	ts.Require().NotEmpty(ts.ssoCookieNames(client), "an SSO cookie should be set after first login")

	resp := ts.authorizeRaw(client, "openid", "prompt_none_live_2", map[string]string{"prompt": "none"})
	defer resp.Body.Close()
	query := ts.redirectQuery(resp)

	ts.Require().Empty(query.Get("error"), "prompt=none must be honored when a live session exists")
	ts.Require().True(strings.HasPrefix(resp.Header.Get("Location"), redirectURI),
		"prompt=none must redirect straight to the client, not to the login page")
	ts.NotEmpty(query.Get("code"), "the silent redirect should carry an authorization code")
	ts.Equal("prompt_none_live_2", query.Get("state"), "state must be echoed back to the client")
}

// TestPromptNone_RejectedAssertionGoesToClient verifies that when a live session completes the flow
// but the callback rejects the result, here a sub claim constraint naming another subject, the error
// is redirected to the client rather than ending on the error page.
func (ts *SSOLogoutTestSuite) TestPromptNone_RejectedAssertionGoesToClient() {
	client := ts.newSessionClient()
	ts.login(client, ssoMaxAgeUsername, "prompt_none_reject_1")
	ts.Require().NotEmpty(ts.ssoCookieNames(client), "an SSO cookie should be set after first login")

	resp := ts.authorizeRaw(client, "openid", "prompt_none_reject_2", map[string]string{
		"prompt": "none",
		"claims": `{"id_token":{"sub":{"value":"another-subject"}}}`,
	})
	defer resp.Body.Close()
	query := ts.redirectQuery(resp)

	ts.Equal("access_denied", query.Get("error"), "the rejected assertion must be redirected to the client")
	ts.Equal("prompt_none_reject_2", query.Get("state"), "state must be echoed back to the client")
	ts.Empty(query.Get("code"), "no code may be issued when the assertion is rejected")
}

// TestPromptNone_WithoutSessionIsLoginRequired verifies the refusal path: with no session cookie,
// prompt=none cannot be satisfied without interaction, so the client is redirected back with
// login_required rather than being shown a login page.
func (ts *SSOLogoutTestSuite) TestPromptNone_WithoutSessionIsLoginRequired() {
	client := ts.newSessionClient()

	resp := ts.authorizeRaw(client, "openid", "prompt_none_absent", map[string]string{"prompt": "none"})
	defer resp.Body.Close()
	query := ts.redirectQuery(resp)

	ts.Equal("login_required", query.Get("error"), "prompt=none without a session must be refused")
	ts.Equal("prompt_none_absent", query.Get("state"), "state must be echoed back to the client")
}

// TestPromptNone_AfterSessionTerminationIsLoginRequired verifies that the decision follows the
// session's lifecycle: once the session is gone, the same client that was silently satisfied a
// moment ago is refused.
func (ts *SSOLogoutTestSuite) TestPromptNone_AfterSessionTerminationIsLoginRequired() {
	client := ts.newSessionClient()
	ts.login(client, ssoMaxAgeUsername, "prompt_none_terminated_1")

	// A fresh jar stands in for a client that never held the session cookie, which is the state a
	// terminated session leaves the browser in.
	fresh := ts.newSessionClient()

	resp := ts.authorizeRaw(fresh, "openid", "prompt_none_terminated_2", map[string]string{"prompt": "none"})
	defer resp.Body.Close()
	query := ts.redirectQuery(resp)

	ts.Equal("login_required", query.Get("error"),
		"a client without the session cookie must not be silently authorized")
}

// TestPromptNone_IDTokenHintMatchingSubjectSucceeds covers the parameter this change exists for: a
// hint naming the signed-in subject narrows the silent path rather than blocking it, so the request
// is answered from the session exactly as a bare prompt=none would be.
func (ts *SSOLogoutTestSuite) TestPromptNone_IDTokenHintMatchingSubjectSucceeds() {
	client := ts.newSessionClient()
	idToken := ts.login(client, ssoMaxAgeUsername, "hint_match_1")
	ts.Require().NotEmpty(idToken, "the first login should yield an id_token to use as the hint")
	ts.Require().NotEmpty(ts.ssoCookieNames(client), "an SSO cookie should be set after first login")

	resp := ts.authorizeRaw(client, "openid", "hint_match_2", map[string]string{
		"prompt":        "none",
		"id_token_hint": idToken,
	})
	defer resp.Body.Close()
	query := ts.redirectQuery(resp)

	ts.Require().Empty(query.Get("error"),
		"a hint naming the signed-in subject must not block the silent path")
	ts.NotEmpty(query.Get("code"), "the hinted subject matches, so the client should get a code")
}

// TestPromptNone_IDTokenHintWithoutSessionIsLoginRequired pins the ordering of the two checks: the
// session is resolved before the hint is read, so a hint cannot stand in for an authentication that
// never happened. A hint identifies who, never that they are still signed in.
func (ts *SSOLogoutTestSuite) TestPromptNone_IDTokenHintWithoutSessionIsLoginRequired() {
	seed := ts.newSessionClient()
	idToken := ts.login(seed, ssoMaxAgeUsername, "hint_nosession_1")
	ts.Require().NotEmpty(idToken)

	// A jar that never held the session cookie, carrying a valid hint for a real, known subject.
	fresh := ts.newSessionClient()

	resp := ts.authorizeRaw(fresh, "openid", "hint_nosession_2", map[string]string{
		"prompt":        "none",
		"id_token_hint": idToken,
	})
	defer resp.Body.Close()
	query := ts.redirectQuery(resp)

	ts.Equal("login_required", query.Get("error"),
		"a valid hint must not authorize a client that holds no session")
	ts.Equal("hint_nosession_2", query.Get("state"), "state must be echoed back to the client")
}

// TestPromptNone_MalformedIDTokenHintIsRejected verifies that a hint which cannot be verified is
// refused rather than ignored. Silently dropping an unparseable hint would answer a request the
// client did not make, authorizing whoever happens to hold the session.
func (ts *SSOLogoutTestSuite) TestPromptNone_MalformedIDTokenHintIsRejected() {
	client := ts.newSessionClient()
	ts.login(client, ssoMaxAgeUsername, "hint_malformed_1")
	ts.Require().NotEmpty(ts.ssoCookieNames(client), "an SSO cookie should be set after first login")

	resp := ts.authorizeRaw(client, "openid", "hint_malformed_2", map[string]string{
		"prompt":        "none",
		"id_token_hint": "not.a.jwt",
	})
	defer resp.Body.Close()
	query := ts.redirectQuery(resp)

	ts.NotEmpty(query.Get("error"), "an unverifiable hint must not be silently ignored")
	ts.Contains([]string{"invalid_request", "login_required"}, query.Get("error"),
		"the refusal should name the malformed parameter or fall back to login_required")
}

// TestPromptNone_InteractiveStepAfterSSOCheckIsLoginRequired verifies that prompt=none never
// prompts: with a live session, a flow that still needs the user after the SSO check is answered
// with login_required instead of the login page.
func (ts *SSOLogoutTestSuite) TestPromptNone_InteractiveStepAfterSSOCheckIsLoginRequired() {
	// Establish the session: sign in with credentials, which saves the checkpoint, then stop at the
	// interactive step.
	client := ts.newSessionClient()
	resp := ts.authorizeRaw(client, "openid", "step_login_1", map[string]string{"client_id": promptStepClientID})
	resp.Body.Close()
	_, executionID, err := testutils.ExtractAuthData(resp.Header.Get("Location"))
	ts.Require().NoError(err, "failed to extract auth data from authorize redirect")

	initial := ts.flowExecute(client, map[string]interface{}{"executionId": executionID})
	ts.Require().NotEqual("COMPLETE", initial.FlowStatus, "first login must prompt for credentials")
	step := ts.flowExecute(client, map[string]interface{}{
		"executionId":    executionID,
		"inputs":         map[string]string{"username": ssoMaxAgeUsername, "password": testPassword},
		"action":         "action_001",
		"challengeToken": initial.ChallengeToken,
	})
	ts.Require().NotEqual("COMPLETE", step.FlowStatus, "the interactive step must prompt after sign-in")
	ts.Require().NotEmpty(ts.ssoCookieNames(client), "an SSO cookie should be set after sign-in")

	// The session is live, but the flow still needs the user after the SSO check.
	resp = ts.authorizeRaw(client, "openid", "step_none_2", map[string]string{
		"client_id": promptStepClientID,
		"prompt":    "none",
	})
	defer resp.Body.Close()
	query := ts.redirectQuery(resp)

	ts.Equal("login_required", query.Get("error"), "prompt=none must not prompt for the interactive step")
	ts.Equal("step_none_2", query.Get("state"), "state must be echoed back to the client")
	ts.Empty(query.Get("code"), "no code may be issued when the flow needs the user")
}

// TestPromptNone_FlowFailureAfterSSOCheckGoesToClient verifies that prompt=none never ends on the
// error page: with a live session, a flow that fails after the SSO check redirects its mapped
// error back to the client.
func (ts *SSOLogoutTestSuite) TestPromptNone_FlowFailureAfterSSOCheckGoesToClient() {
	// Establish the session: sign in with credentials, which saves the checkpoint and completes.
	client := ts.newSessionClient()
	resp := ts.authorizeRaw(client, "openid", "fail_login_1", map[string]string{"client_id": failOnReuseClientID})
	resp.Body.Close()
	_, executionID, err := testutils.ExtractAuthData(resp.Header.Get("Location"))
	ts.Require().NoError(err, "failed to extract auth data from authorize redirect")

	initial := ts.flowExecute(client, map[string]interface{}{"executionId": executionID})
	ts.Require().NotEqual("COMPLETE", initial.FlowStatus, "first login must prompt for credentials")
	step := ts.flowExecute(client, map[string]interface{}{
		"executionId":    executionID,
		"inputs":         map[string]string{"username": ssoMaxAgeUsername, "password": testPassword},
		"action":         "action_001",
		"challengeToken": initial.ChallengeToken,
	})
	ts.Require().Equal("COMPLETE", step.FlowStatus, "sign-in should complete")
	ts.Require().NotEmpty(ts.ssoCookieNames(client), "an SSO cookie should be set after sign-in")

	// The session is live, but reusing it fails the flow.
	resp = ts.authorizeRaw(client, "openid", "fail_none_2", map[string]string{
		"client_id": failOnReuseClientID,
		"prompt":    "none",
	})
	defer resp.Body.Close()
	query := ts.redirectQuery(resp)

	ts.Equal("access_denied", query.Get("error"), "the flow failure must be redirected to the client")
	ts.Equal("fail_none_2", query.Get("state"), "state must be echoed back to the client")
	ts.Empty(query.Get("code"), "no code may be issued when the flow fails")
}
