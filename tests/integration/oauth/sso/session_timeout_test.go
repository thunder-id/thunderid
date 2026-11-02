// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sso

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const sessionConfigURL = testutils.TestServerURL + "/server-config/session"

// SSO session timeouts are read once, when the session service is constructed, so changing them
// requires a server restart rather than only a configuration write. Every test here therefore
// restores the original configuration and restarts again, so a failure cannot leave the rest of the
// run with second-scale session lifetimes.
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
	req, err := http.NewRequest(http.MethodPut, sessionConfigURL, strings.NewReader(body))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err, "failed to update session server config")
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, resp.StatusCode,
		"failed to update session config with %s: %s", body, string(respBody))
}

// applySessionTimeouts writes the given timeouts and restarts the server so the session service
// picks them up, registering the restore before making any change so a mid-test failure still
// returns the deployment to its original lifetimes.
//
// idle must not exceed absolute, and the activity-refresh interval must be strictly less than idle;
// both are rejected by configuration validation otherwise.
func (ts *SSOLogoutTestSuite) applySessionTimeouts(idle, absolute, activityRefresh int64) {
	original := ts.writableSessionConfig()
	ts.T().Cleanup(func() {
		ts.putSessionConfig(original)
		if err := testutils.RestartServer(); err != nil {
			ts.T().Logf("cleanup: server did not restart cleanly after session config restore: %v", err)
		}
		if err := testutils.ObtainAdminAccessToken(); err != nil {
			ts.T().Logf("cleanup: failed to re-obtain admin token after restore: %v", err)
		}
	})

	ts.putSessionConfig(fmt.Sprintf(
		`{"idleTimeoutSeconds":%d,"absoluteTimeoutSeconds":%d,"activityRefreshIntervalSeconds":%d}`,
		idle, absolute, activityRefresh))

	ts.Require().NoError(testutils.RestartServer(), "failed to restart server with session timeouts")
	ts.Require().NoError(testutils.ObtainAdminAccessToken(),
		"failed to re-obtain admin token after restart")
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
	req, err := http.NewRequest(http.MethodPut, sessionConfigURL, strings.NewReader(body))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err, "failed to send session config update")
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusBadRequest, resp.StatusCode,
		"an incoherent session configuration must be refused, got %d for %s: %s",
		resp.StatusCode, body, string(respBody))

	return string(respBody)
}

// Session timeouts are only read at startup, so an incoherent set has to be refused when it is
// written rather than discovered at the next restart. These are the combinations that would let a
// session behave in a way the deadlines are meant to prevent.
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
		ts.putSessionConfigExpectingRejection(
			`{"idleTimeoutSeconds":600,"absoluteTimeoutSeconds":300,"activityRefreshIntervalSeconds":60}`)
	})

	ts.Run("refresh not below idle", func() {
		// The refresh interval bounds how stale the persisted idle deadline may be. At or above the
		// idle window, an active session could be skipped past its own deadline.
		ts.putSessionConfigExpectingRejection(
			`{"idleTimeoutSeconds":60,"absoluteTimeoutSeconds":600,"activityRefreshIntervalSeconds":60}`)
	})
}

// An SSO session left untouched past its idle timeout must not satisfy a later authorize, so an
// abandoned browser session cannot skip authentication indefinitely.
func (ts *SSOLogoutTestSuite) TestSSOSession_IdleTimeoutForcesReauthentication() {
	// A generous absolute timeout isolates the idle deadline as the only thing that can expire.
	ts.applySessionTimeouts(2, 600, 1)

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
	ts.applySessionTimeouts(4, 4, 1)

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
