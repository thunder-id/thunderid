// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sso

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

var sessionConfigURL = testutils.TestServerURL + "/server-config/session"

const (
	// A second application bound to the suite's SSO authentication flow, so it shares the per-flow
	// session that the main application keeps alive by refreshing tokens.
	refreshPeerClientID     = "sso_refresh_peer_client"
	refreshPeerClientSecret = "sso_refresh_peer_secret" //nolint:gosec // test credential
)

// SSO session timeouts are read from the session section each time a session uses them, so a write
// applies at once, with no restart. Every test here restores the original configuration, so a failure
// cannot leave the rest of the run with second-scale session lifetimes.
func (ts *SSOLogoutTestSuite) writableSessionConfig() string {
	req, err := http.NewRequest(http.MethodGet, sessionConfigURL, nil)
	ts.Require().NoError(err)

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err, "failed to read session server config")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, resp.StatusCode, "failed to read session config: %s", string(body))

	var layers struct {
		Writable json.RawMessage `json:"writable"`
	}
	ts.Require().NoError(json.Unmarshal(body, &layers))

	if len(layers.Writable) == 0 {
		return "{}"
	}
	return string(layers.Writable)
}

// putSessionConfig replaces the writable layer of the session section.
func (ts *SSOLogoutTestSuite) putSessionConfig(body string) {
	ts.Require().NoError(testutils.PutWritableServerConfig("session", []byte(body)),
		"failed to update session config with %s", body)
}

// applySessionTimeouts writes the given timeouts, which the next session use reads, registering the
// restore before making any change so a mid-test failure still returns the deployment to its original
// lifetimes.
//
// idle must not exceed absolute, and must be longer than the deployment's activity refresh interval
// (one second on the integration server); configuration validation refuses it otherwise.
func (ts *SSOLogoutTestSuite) applySessionTimeouts(idle, absolute int64) {
	original := ts.writableSessionConfig()
	ts.T().Cleanup(func() {
		ts.putSessionConfig(original)
	})

	ts.putSessionConfig(fmt.Sprintf(`{"idleTimeoutSeconds":%d,"absoluteTimeoutSeconds":%d}`, idle, absolute))
}

// sessionSurvives reports whether a fresh authorize is satisfied by the existing SSO session. A
// live session completes the flow on its initial step; an expired one falls through to the
// credential prompt.
func (ts *SSOLogoutTestSuite) sessionSurvives(client *http.Client, state string) bool {
	_, executionID := ts.authorize(client, "openid", state)
	step := ts.flowExecute(client, map[string]interface{}{"executionId": executionID})
	return step.FlowStatus == "COMPLETE"
}

// putSessionConfigExpectingRejection writes a session configuration that validation must refuse,
// and returns the response body for the assertion.
func (ts *SSOLogoutTestSuite) putSessionConfigExpectingRejection(body string) string {
	status, respBody, err := testutils.PutWritableServerConfigWithStatus("session", []byte(body))
	ts.Require().NoError(err, "failed to send session config update")
	ts.Require().Equal(http.StatusBadRequest, status,
		"an incoherent session configuration must be refused, got %d for %s: %s", status, body, string(respBody))

	return string(respBody)
}

// An incoherent set of session timeouts is refused when it is written, before any session reads it.
// These are the combinations that would let a session behave in a way the deadlines are meant to
// prevent.
func (ts *SSOLogoutTestSuite) TestSessionConfig_IncoherentTimeoutsRejected() {
	// The write is expected to fail, but restore the original on the way out regardless, so a
	// regression that lets one through cannot leave the rest of the run misconfigured.
	original := ts.writableSessionConfig()
	ts.T().Cleanup(func() {
		ts.putSessionConfig(original)
	})

	ts.Run("negative timeout", func() {
		ts.putSessionConfigExpectingRejection(`{"idleTimeoutSeconds":-1}`)
	})

	ts.Run("idle beyond absolute", func() {
		// An idle deadline past the absolute cap could never be reached, so the pair is meaningless.
		ts.putSessionConfigExpectingRejection(`{"idleTimeoutSeconds":600,"absoluteTimeoutSeconds":300}`)
	})

	ts.Run("idle not above the activity refresh interval", func() {
		// The deployment's refresh interval bounds how stale the persisted idle deadline may be. An idle
		// window no longer than it could let an active session be skipped past its own deadline.
		ts.putSessionConfigExpectingRejection(`{"idleTimeoutSeconds":1,"absoluteTimeoutSeconds":600}`)
	})
}

// An SSO session left untouched past its idle timeout must not satisfy a later authorize, so an
// abandoned browser session cannot skip authentication indefinitely.
func (ts *SSOLogoutTestSuite) TestSSOSession_IdleTimeoutForcesReauthentication() {
	// A generous absolute timeout isolates the idle deadline as the only thing that can expire.
	ts.applySessionTimeouts(2, 600)

	client := ts.newSessionClient()
	ts.login(client, ssoReuseUsername, "idle_timeout_state_1")
	ts.Require().NotEmpty(ts.ssoCookieNames(client), "an SSO cookie should be set after first login")

	ts.Require().True(ts.sessionSurvives(client, "idle_timeout_state_2"),
		"the session should still be live immediately after login")

	// Idle past the deadline with no activity at all.
	time.Sleep(4 * time.Second)

	ts.False(ts.sessionSurvives(client, "idle_timeout_state_3"),
		"an idle-expired session must not skip authentication")
}

// The absolute timeout caps total session lifetime regardless of activity, so a session kept alive
// by continued use is still retired once it is old enough.
func (ts *SSOLogoutTestSuite) TestSSOSession_AbsoluteTimeoutForcesReauthentication() {
	// Idle equals absolute so that refreshing activity mid-way slides the idle deadline beyond the
	// absolute one, leaving the absolute cap as the only reason the session can expire.
	ts.applySessionTimeouts(4, 4)

	client := ts.newSessionClient()
	ts.login(client, ssoReuseUsername, "absolute_timeout_state_1")

	// Use the session inside the idle window. This slides the idle deadline forward but must not move
	// the absolute one.
	time.Sleep(2 * time.Second)
	ts.Require().True(ts.sessionSurvives(client, "absolute_timeout_state_2"),
		"the session should still be live within both deadlines")

	// Now past the absolute cap (about 5s since login) but inside the refreshed idle window (about 6s
	// from the activity above), so only the absolute timeout can end the session.
	time.Sleep(3 * time.Second)

	ts.False(ts.sessionSurvives(client, "absolute_timeout_state_3"),
		"a session past its absolute timeout must not skip authentication even when recently used")
}

// An application that keeps a user signed in by refreshing tokens is using the SSO session, so a
// refresh inside the idle window must slide the idle deadline. Another application on the same flow
// then still signs the user in silently after the original idle deadline has passed.
func (ts *SSOLogoutTestSuite) TestSSOSession_TokenRefreshExtendsIdleTimeout() {
	// A generous absolute timeout isolates the idle deadline as the only thing that can expire.
	ts.applySessionTimeouts(6, 600)
	ts.createParticipantApplication("SSORefreshPeerApp", refreshPeerClientID, refreshPeerClientSecret)

	client := ts.newSessionClient()
	token := ts.loginTokens(client, ssoReuseUsername, "refresh_idle_state_1")
	ts.Require().NotEmpty(token.RefreshToken, "a refresh token should be issued to the main application")

	// Refresh inside the idle window and past the activity-refresh throttle.
	time.Sleep(3 * time.Second)
	_, err := testutils.RefreshAccessToken(clientID, clientSecret, token.RefreshToken)
	ts.Require().NoError(err, "the refresh token grant should succeed")

	// Now past the original idle deadline (about 7s since login) but inside the one the refresh moved
	// (about 4s since the refresh), so only the refresh can have kept the session alive.
	time.Sleep(4 * time.Second)

	ts.True(ts.ssoSatisfies(client, refreshPeerClientID, "refresh_idle_state_2"),
		"a token refresh must keep the SSO session alive for another application")
}
