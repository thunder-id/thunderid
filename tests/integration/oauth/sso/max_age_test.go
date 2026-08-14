// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sso

import "time"

// The OIDC max_age parameter is enforced by the AuthAssertExecutor's assurance check, which only has
// an authentication timestamp to compare against when an SSO checkpoint was loaded. On a first-time
// login the subject authenticates during the execution itself, so the constraint is trivially met.
// These tests therefore always establish a session first and then re-authorize over it, which is the
// only path where max_age can actually bite.
//
// Every test builds its own cookie jar via newSessionClient so its session, and therefore its
// authentication time, is fresh and unambiguous regardless of the order tests run in.

// TestMaxAge_WithinWindowReusesSession verifies that a generous max_age leaves SSO reuse intact: the
// session was established moments ago, so the assurance check passes and the re-authorize completes
// on its first step without a credential prompt.
func (ts *SSOLogoutTestSuite) TestMaxAge_WithinWindowReusesSession() {
	client := ts.newSessionClient()
	ts.login(client, ssoMaxAgeUsername, "max_age_within_1")
	ts.Require().NotEmpty(ts.ssoCookieNames(client), "an SSO cookie should be set after first login")

	_, executionID := ts.authorizeWithMaxAge(client, "openid", "max_age_within_2", "3600")
	step := ts.flowExecute(client, map[string]interface{}{"executionId": executionID})

	ts.Equal("COMPLETE", step.FlowStatus, "a session inside the max_age window should still satisfy the request")
	ts.NotEmpty(step.Assertion, "the SSO-satisfied flow should yield an assertion")
	ts.Nil(step.Error, "no assurance error should be reported")
}

// TestMaxAge_ExpiredSessionRequiresInteraction verifies that a session older than max_age fails the
// assurance check: the flow terminates in-band with FET-1082 (interaction required) and mints no
// assertion, so no authorization code can be issued.
//
// Only the flow-level outcome is asserted. Mapping this failure onto an OAuth2 authorize error is an
// open TODO in the product, so the downstream error code is deliberately left unpinned.
func (ts *SSOLogoutTestSuite) TestMaxAge_ExpiredSessionRequiresInteraction() {
	client := ts.newSessionClient()
	ts.login(client, ssoMaxAgeUsername, "max_age_expired_1")
	ts.Require().NotEmpty(ts.ssoCookieNames(client), "an SSO cookie should be set after first login")

	// Age the session past the one second window requested below.
	time.Sleep(3 * time.Second)

	_, executionID := ts.authorizeWithMaxAge(client, "openid", "max_age_expired_2", "1")
	step := ts.flowExecute(client, map[string]interface{}{"executionId": executionID})

	ts.Require().Equal("ERROR", step.FlowStatus, "a session older than max_age must not satisfy the request")
	ts.Require().NotNil(step.Error, "the failed flow should carry a structured error")
	ts.Equal("FET-1082", step.Error.Code, "re-authentication should be reported as interaction required")
	ts.Empty(step.Assertion, "no authentication assertion may be minted for an expired session")
}

// TestMaxAge_MalformedValueIsIgnored verifies that a non-numeric max_age is treated as no constraint
// rather than as a zero-second window, so SSO reuse is unaffected.
func (ts *SSOLogoutTestSuite) TestMaxAge_MalformedValueIsIgnored() {
	client := ts.newSessionClient()
	ts.login(client, ssoMaxAgeUsername, "max_age_malformed_1")
	ts.Require().NotEmpty(ts.ssoCookieNames(client), "an SSO cookie should be set after first login")

	_, executionID := ts.authorizeWithMaxAge(client, "openid", "max_age_malformed_2", "abc")
	step := ts.flowExecute(client, map[string]interface{}{"executionId": executionID})

	ts.Equal("COMPLETE", step.FlowStatus, "an unparseable max_age should impose no constraint")
	ts.NotEmpty(step.Assertion, "the SSO-satisfied flow should yield an assertion")
	ts.Nil(step.Error, "no assurance error should be reported")
}

// TestMaxAge_NegativeValueIsIgnored verifies that a negative max_age is treated as no constraint. A
// literal reading would reject every session, so this pins the lenient behaviour.
func (ts *SSOLogoutTestSuite) TestMaxAge_NegativeValueIsIgnored() {
	client := ts.newSessionClient()
	ts.login(client, ssoMaxAgeUsername, "max_age_negative_1")
	ts.Require().NotEmpty(ts.ssoCookieNames(client), "an SSO cookie should be set after first login")

	_, executionID := ts.authorizeWithMaxAge(client, "openid", "max_age_negative_2", "-5")
	step := ts.flowExecute(client, map[string]interface{}{"executionId": executionID})

	ts.Equal("COMPLETE", step.FlowStatus, "a negative max_age should impose no constraint")
	ts.NotEmpty(step.Assertion, "the SSO-satisfied flow should yield an assertion")
	ts.Nil(step.Error, "no assurance error should be reported")
}
