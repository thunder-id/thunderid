// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sso

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	deviceSSOScope = "openid device_sso"
	// otherAppClientID is the second application of the reuse test, a client of its own so no other test
	// deletes an application with the same client ID just before it signs in.
	otherAppClientID = "sso_device_secret_other_app"
)

// TestCodeExchange_ReturnsDeviceSecret verifies that a code whose callback posted the session the flow
// started, requested with device_sso, returns that session as a server-signed device secret addressed to
// this server.
func (ts *SSOLogoutTestSuite) TestCodeExchange_ReturnsDeviceSecret() {
	client := ts.newSessionClient()

	token, posted := ts.loginDeviceSecret(client, deviceSSOScope, "device_secret_state", true)

	sessions := ts.deviceSecretClaims(token.DeviceSecret)
	ts.Require().Len(posted, 1, "the flow started one session")
	ts.Equal(posted, sessions, "the device secret carries the posted session")
	ts.Equal(map[string]string{testutils.SSOCookiePrefix + sessions[0].FlowIdentifier: sessions[0].SessionHandle},
		ts.ssoCookies(client), "the device secret names the same session as the per-flow cookie")
}

// TestCodeExchange_NoDeviceSecret verifies the cases where a code exchange returns no device secret.
func (ts *SSOLogoutTestSuite) TestCodeExchange_NoDeviceSecret() {
	ts.Run("device_sso was not requested", func() {
		token, _ := ts.loginDeviceSecret(ts.newSessionClient(), "openid", "no_device_sso_state", true)
		ts.Empty(token.DeviceSecret)
	})
	ts.Run("the callback posted no session", func() {
		token, _ := ts.loginDeviceSecret(ts.newSessionClient(), deviceSSOScope, "no_posted_session_state", false)
		ts.Empty(token.DeviceSecret)
	})
}

// TestCodeExchange_CarriesEveryPostedSession verifies that the device secret carries every session the
// callback posted, as posted: the code exchange does not look them up.
func (ts *SSOLogoutTestSuite) TestCodeExchange_CarriesEveryPostedSession() {
	client := ts.newSessionClient()
	authID, executionID := ts.authorize(client, deviceSSOScope, "every_posted_session_state")
	step, collected := ts.runCredentialLogin(client, executionID, ssoReuseUsername)
	posted := append([]testutils.FlowSessionData{unrelatedSession}, collected...)

	token := ts.completeAndExchange(client, authID, step.Assertion, posted)

	ts.Equal(posted, ts.deviceSecretClaims(token.DeviceSecret))
}

// TestCodeExchange_IgnoresAnOverLongSession verifies that a posted entry with a field longer than the server
// keeps is left out of the device secret without failing the sign-in.
func (ts *SSOLogoutTestSuite) TestCodeExchange_IgnoresAnOverLongSession() {
	client := ts.newSessionClient()
	authID, executionID := ts.authorize(client, deviceSSOScope, "over_long_session_state")
	step, collected := ts.runCredentialLogin(client, executionID, ssoReuseUsername)
	overLong := testutils.FlowSessionData{FlowIdentifier: "0000000000000000", SessionHandle: strings.Repeat("h", 129)}

	token := ts.completeAndExchange(client, authID, step.Assertion, append(collected, overLong))

	ts.Equal(collected, ts.deviceSecretClaims(token.DeviceSecret))
}

// TestCallback_OversizedBodyRefused verifies that a callback body over the limit is refused before anything
// is spent, so the same sign-in can still complete with a normal callback.
func (ts *SSOLogoutTestSuite) TestCallback_OversizedBodyRefused() {
	client := ts.newSessionClient()
	authID, executionID := ts.authorize(client, deviceSSOScope, "oversized_callback_state")
	step, collected := ts.runCredentialLogin(client, executionID, ssoReuseUsername)
	oversized := testutils.FlowSessionData{FlowIdentifier: "0000000000000000",
		SessionHandle: strings.Repeat("h", 1<<20)}

	_, err := testutils.CompleteAuthorizationWithSessionData(client, authID, step.Assertion,
		append([]testutils.FlowSessionData{oversized}, collected...))
	ts.Require().Error(err)
	ts.Contains(err.Error(), "status 400")

	token := ts.completeAndExchange(client, authID, step.Assertion, collected)
	ts.Equal(collected, ts.deviceSecretClaims(token.DeviceSecret))
}

// TestCodeExchange_ReturnsDeviceSecretOnSSOReuse verifies that a sign-in that reuses the session returns that
// session, as a new one would, so its code returns a device secret for the same session.
func (ts *SSOLogoutTestSuite) TestCodeExchange_ReturnsDeviceSecretOnSSOReuse() {
	client := ts.newSessionClient()
	_, started := ts.loginDeviceSecret(client, deviceSSOScope, "device_reuse_state_1", true)
	ts.Require().Len(started, 1, "the first sign-in started one session")
	cookies := ts.ssoCookies(client)

	authID, executionID := ts.authorize(client, deviceSSOScope, "device_reuse_state_2")
	step := ts.flowExecute(client, map[string]interface{}{"executionId": executionID})
	ts.Require().Equal("COMPLETE", step.FlowStatus, "the second authorize should skip authentication via SSO")
	ts.Require().Len(step.SessionData, 1, "a reused session is returned")
	reused := []testutils.FlowSessionData{step.SessionData[0].FlowSessionData}
	ts.Equal(started, reused, "the reused session is the one the first sign-in started")

	second := ts.completeAndExchange(client, authID, step.Assertion, reused)

	ts.Equal(started, ts.deviceSecretClaims(second.DeviceSecret))
	ts.Equal(cookies, ts.ssoCookies(client), "the per-flow cookie still names the same session")
}

// TestCodeExchange_ReturnsDeviceSecretOnPromptNone verifies that a prompt=none sign-in, which the server completes
// on the session with no gate, keeps that session with its code too.
func (ts *SSOLogoutTestSuite) TestCodeExchange_ReturnsDeviceSecretOnPromptNone() {
	client := ts.newSessionClient()
	_, started := ts.loginDeviceSecret(client, deviceSSOScope, "device_prompt_none_1", true)
	ts.Require().Len(started, 1, "the first sign-in started one session")

	resp := ts.authorizeRaw(client, deviceSSOScope, "device_prompt_none_2", map[string]string{"prompt": "none"})
	defer resp.Body.Close()
	code := ts.redirectQuery(resp).Get("code")
	ts.Require().NotEmpty(code, "prompt=none should redirect straight back with a code")

	token := ts.exchangeCode(client, code)

	ts.Equal(started, ts.deviceSecretClaims(token.DeviceSecret))
}

// TestCodeExchange_ReturnsDeviceSecretOnReauthentication verifies that a sign-in made to authenticate again over a
// live session, by prompt=login or max_age=0, joins that session and returns it, so its code returns a device secret
// for the same session and the per-flow cookie is left as it was.
func (ts *SSOLogoutTestSuite) TestCodeExchange_ReturnsDeviceSecretOnReauthentication() {
	cases := []struct {
		name         string
		param, value string
		firstState   string
		secondState  string
	}{
		{"prompt=login", "prompt", "login", "device_prompt_login_1", "device_prompt_login_2"},
		{"max_age=0", "max_age", "0", "device_max_age_zero_1", "device_max_age_zero_2"},
	}
	for _, tc := range cases {
		ts.Run(tc.name, func() {
			client := ts.newSessionClient()
			_, started := ts.loginDeviceSecret(client, deviceSSOScope, tc.firstState, true)
			ts.Require().Len(started, 1, "the first sign-in started one session")
			cookies := ts.ssoCookies(client)

			resp := ts.authorizeRaw(client, deviceSSOScope, tc.secondState, map[string]string{tc.param: tc.value})
			defer resp.Body.Close()
			ts.Require().Equal(http.StatusFound, resp.StatusCode, "authorize should redirect to the gate")
			authID, executionID, err := testutils.ExtractAuthData(resp.Header.Get("Location"))
			ts.Require().NoError(err, "failed to extract auth data from authorize redirect")
			step, joined := ts.runCredentialLogin(client, executionID, ssoReuseUsername)
			ts.Equal(started, joined, "authenticating again joins the session the first sign-in started")

			token := ts.completeAndExchange(client, authID, step.Assertion, joined)

			ts.Equal(started, ts.deviceSecretClaims(token.DeviceSecret))
			ts.Equal(cookies, ts.ssoCookies(client), "the per-flow cookie still names the same session")
		})
	}
}

// TestCodeExchange_DeviceSecretExpiresWithItsSession verifies that the expiresIn posted with the callback sets
// when the device secret ends: the time the session has left, counted from the callback.
func (ts *SSOLogoutTestSuite) TestCodeExchange_DeviceSecretExpiresWithItsSession() {
	client := ts.newSessionClient()
	authID, executionID := ts.authorize(client, deviceSSOScope, "device_secret_exp_left")
	step, collected := ts.runCredentialLogin(client, executionID, ssoReuseUsername)
	ts.Require().Len(collected, 1, "the sign-in started one session")
	posted := collected[0]
	posted.ExpiresIn = 600

	before := time.Now().Unix()
	token := ts.completeAndExchange(client, authID, step.Assertion, []testutils.FlowSessionData{posted})
	after := time.Now().Unix()

	claims, err := testutils.DecodeJWT(token.DeviceSecret)
	ts.Require().NoError(err, "the code exchange should return a device secret")
	ts.GreaterOrEqual(int64(claims.Exp), before+600-1, "the device secret ends 600 seconds after the callback")
	ts.LessOrEqual(int64(claims.Exp), after+600+1, "the device secret ends 600 seconds after the callback")
}

// TestCodeExchange_ReturnsDeviceSecretToAnotherApp verifies the case the issue reports: a session one application
// started is reused, with device_sso, by a second public client on the same sign-in flow, and that client's code
// returns a device secret for the session.
func (ts *SSOLogoutTestSuite) TestCodeExchange_ReturnsDeviceSecretToAnotherApp() {
	appID := ts.createTokenExchangeApp(otherAppClientID, ts.authFlowID, true)
	defer ts.deleteAppByID(appID)
	client := ts.newSessionClient()
	_, started := ts.loginDeviceSecret(client, deviceSSOScope, "device_other_app_1", true)
	ts.Require().Len(started, 1, "the first application's sign-in started one session")
	verifier, err := testutils.GenerateCodeVerifier()
	ts.Require().NoError(err)
	params := url.Values{}
	params.Set("client_id", otherAppClientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	params.Set("scope", deviceSSOScope)
	params.Set("state", "device_other_app_2")
	params.Set("code_challenge", testutils.GenerateCodeChallenge(verifier))
	params.Set("code_challenge_method", "S256")

	authID, executionID, err := testutils.StartAuthorizationWithSession(client, params)
	ts.Require().NoError(err, "authorize request failed")
	step := ts.flowExecute(client, map[string]interface{}{"executionId": executionID})
	ts.Require().Equal("COMPLETE", step.FlowStatus, "the second application signs in through the session")
	ts.Require().Len(step.SessionData, 1, "the reused session is returned")
	reused := []testutils.FlowSessionData{step.SessionData[0].FlowSessionData}
	authzResponse, err := testutils.CompleteAuthorizationWithSessionData(client, authID, step.Assertion, reused)
	ts.Require().NoError(err, "auth callback failed")
	code, err := testutils.ExtractAuthorizationCode(authzResponse.RedirectURI)
	ts.Require().NoError(err, "failed to extract authorization code")
	result, err := testutils.RequestTokenWithPKCE(otherAppClientID, "", code, redirectURI, "authorization_code",
		verifier)
	ts.Require().NoError(err, "token request failed")
	ts.Require().Equal(http.StatusOK, result.StatusCode, string(result.Body))

	ts.Equal(started, ts.deviceSecretClaims(result.Token.DeviceSecret),
		"the second application's device secret carries the first application's session")
}

// loginDeviceSecret drives a first-time login as a gate would: it collects the sessions every flow
// response returns and, when post is set, posts them with the assertion to the callback. It returns the
// code exchange's tokens and the sessions collected.
func (ts *SSOLogoutTestSuite) loginDeviceSecret(client *http.Client, scope, state string, post bool) (
	*testutils.TokenResponse, []testutils.FlowSessionData) {
	authID, executionID := ts.authorize(client, scope, state)
	step, collected := ts.runCredentialLogin(client, executionID, ssoReuseUsername)
	posted := collected
	if !post {
		posted = nil
	}
	return ts.completeAndExchange(client, authID, step.Assertion, posted), collected
}

// runCredentialLogin runs the credential login to completion and returns the final step and every
// session the flow's responses returned.
func (ts *SSOLogoutTestSuite) runCredentialLogin(client *http.Client, executionID, username string) (
	*testutils.FlowStep, []testutils.FlowSessionData) {
	var collected []testutils.FlowSessionData
	collect := func(step *testutils.FlowStep) {
		for _, issued := range step.SessionData {
			collected = append(collected, issued.FlowSessionData)
		}
	}

	initial := ts.flowExecute(client, map[string]interface{}{"executionId": executionID})
	ts.Require().NotEqual("COMPLETE", initial.FlowStatus, "first login must prompt for credentials")
	collect(initial)
	step := ts.flowExecute(client, map[string]interface{}{
		"executionId":    executionID,
		"inputs":         map[string]string{"username": username, "password": testPassword},
		"action":         "action_001",
		"challengeToken": initial.ChallengeToken,
	})
	ts.Require().Equal("COMPLETE", step.FlowStatus, "credential login should complete the flow")
	ts.Require().NotEmpty(step.Assertion, "login should yield an assertion")
	collect(step)
	return step, collected
}

// completeAndExchange posts the assertion and sessions to the callback and exchanges the code.
func (ts *SSOLogoutTestSuite) completeAndExchange(client *http.Client, authID, assertion string,
	sessionData []testutils.FlowSessionData) *testutils.TokenResponse {
	authzResponse, err := testutils.CompleteAuthorizationWithSessionData(client, authID, assertion, sessionData)
	ts.Require().NoError(err, "auth callback failed")
	code, err := testutils.ExtractAuthorizationCode(authzResponse.RedirectURI)
	ts.Require().NoError(err, "failed to extract authorization code")
	return ts.exchangeCode(client, code)
}

// unrelatedSession is a session entry of no flow the tests run, posted alongside a real one.
var unrelatedSession = testutils.FlowSessionData{FlowIdentifier: "0000000000000000", SessionHandle: "unrelated"}

// deviceSecretClaims checks the shape of a device secret and returns the sessions it carries.
func (ts *SSOLogoutTestSuite) deviceSecretClaims(deviceSecret string) []testutils.FlowSessionData {
	ts.Require().NotEmpty(deviceSecret, "the code exchange should return a device secret")
	header, err := testutils.DecodeJWTHeaderMap(deviceSecret)
	ts.Require().NoError(err, "failed to decode the device secret header")
	ts.Equal("sso-session+jwt", header["typ"], "the device secret has its own token type")

	claims, err := testutils.DecodeJWT(deviceSecret)
	ts.Require().NoError(err, "failed to decode the device secret")
	ts.Equal(claims.Iss, claims.Aud, "the device secret is addressed to this server")
	ts.Greater(int64(claims.Exp), time.Now().Unix(), "the device secret expires in the future")
	ts.NotEmpty(claims.Sub, "the device secret names the user who signed in")
	entries, _ := claims.Additional["session_data"].([]interface{})
	sessions := make([]testutils.FlowSessionData, 0, len(entries))
	for _, raw := range entries {
		entry, _ := raw.(map[string]interface{})
		flowIdentifier, _ := entry["flow_identifier"].(string)
		sessionHandle, _ := entry["session_handle"].(string)
		ts.Require().Len(flowIdentifier, 16, "a flow identifier is a cookie name's hash part")
		ts.Require().NotEmpty(sessionHandle, "each session carries its handle")
		sessions = append(sessions,
			testutils.FlowSessionData{FlowIdentifier: flowIdentifier, SessionHandle: sessionHandle})
	}
	ts.Require().NotEmpty(sessions, "the device secret carries at least one session")
	return sessions
}
