// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sso

import "time"

// The OIDC max_age parameter is answered by the SSO-Check node, which declines to reuse a session
// whose authentication is older than the requested window and routes the flow to full
// authentication instead. On a first-time login the subject authenticates during the execution
// itself, so the constraint is trivially met. These tests therefore always establish a session
// first and then re-authorize over it, which is the only path where max_age can actually bite.
// The AuthAssertExecutor's assurance check still guards the same constraint at the end of the flow,
// but re-authenticating at the branch point means it is no longer the first thing to notice.
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

// TestMaxAge_ExpiredSessionRequiresInteraction verifies that a session older than max_age is not
// reused: the SSO-Check node declines it and the flow prompts for credentials instead, which is
// what OIDC Core requires of max_age. Erroring out would leave the client unable to proceed even
// though the End-User is present and able to authenticate.
func (ts *SSOLogoutTestSuite) TestMaxAge_ExpiredSessionRequiresInteraction() {
	client := ts.newSessionClient()
	ts.login(client, ssoMaxAgeUsername, "max_age_expired_1")
	ts.Require().NotEmpty(ts.ssoCookieNames(client), "an SSO cookie should be set after first login")

	// Age the session past the one second window requested below.
	time.Sleep(3 * time.Second)

	_, executionID := ts.authorizeWithMaxAge(client, "openid", "max_age_expired_2", "1")
	step := ts.flowExecute(client, map[string]interface{}{"executionId": executionID})

	ts.Require().NotEqual("COMPLETE", step.FlowStatus,
		"a session older than max_age must not satisfy the request without re-authentication")
	ts.Empty(step.Assertion, "no assertion may be minted before the subject re-authenticates")
	// The SSO-Check node reports FET-1081 as it routes to the full-authentication path, so the step
	// carries that alongside the prompt. What matters is that the flow asks for credentials rather
	// than failing: it must still offer a challenge the subject can answer.
	ts.NotEmpty(step.ChallengeToken,
		"declining the session must yield a credential prompt, not a dead end")
	if step.Error != nil {
		ts.Equal("FET-1081", step.Error.Code,
			"the only error accompanying the prompt should be the SSO-Check routing outcome")
	}
}

// TestMaxAge_ExpiredSessionReauthenticationSucceeds verifies the other half: after the prompt, the
// subject can authenticate again and the flow completes, so max_age costs a login rather than
// failing the request outright.
func (ts *SSOLogoutTestSuite) TestMaxAge_ExpiredSessionReauthenticationSucceeds() {
	client := ts.newSessionClient()
	ts.login(client, ssoMaxAgeUsername, "max_age_reauth_1")
	ts.Require().NotEmpty(ts.ssoCookieNames(client), "an SSO cookie should be set after first login")

	time.Sleep(3 * time.Second)

	_, executionID := ts.authorizeWithMaxAge(client, "openid", "max_age_reauth_2", "1")
	initial := ts.flowExecute(client, map[string]interface{}{"executionId": executionID})
	ts.Require().NotEqual("COMPLETE", initial.FlowStatus, "an expired session must prompt for credentials")

	step := ts.flowExecute(client, map[string]interface{}{
		"executionId":    executionID,
		"inputs":         map[string]string{"username": ssoMaxAgeUsername, "password": testPassword},
		"action":         "action_001",
		"challengeToken": initial.ChallengeToken,
	})

	ts.Equal("COMPLETE", step.FlowStatus, "re-authentication should complete the flow")
	ts.NotEmpty(step.Assertion, "the re-authenticated flow should yield an assertion")
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

// TestMaxAge_ReauthenticationRefreshesSessionAuthTime covers the sequence a two-step test misses: a
// forced re-authentication attaches to the existing session, so that session's authentication time
// has to move forward with it. When it does not, the session goes on claiming the original login and
// the next request is refused for a max_age it actually satisfies.
func (ts *SSOLogoutTestSuite) TestMaxAge_ReauthenticationRefreshesSessionAuthTime() {
	client := ts.newSessionClient()
	ts.login(client, ssoMaxAgeUsername, "max_age_refresh_1")
	ts.Require().NotEmpty(ts.ssoCookieNames(client), "an SSO cookie should be set after first login")

	// Age the session past the window requested below so the next authorize re-authenticates.
	time.Sleep(3 * time.Second)

	_, executionID := ts.authorizeWithMaxAge(client, "openid", "max_age_refresh_2", "1")
	initial := ts.flowExecute(client, map[string]interface{}{"executionId": executionID})
	ts.Require().NotEqual("COMPLETE", initial.FlowStatus, "an expired session must prompt for credentials")

	reauth := ts.flowExecute(client, map[string]interface{}{
		"executionId":    executionID,
		"inputs":         map[string]string{"username": ssoMaxAgeUsername, "password": testPassword},
		"action":         "action_001",
		"challengeToken": initial.ChallengeToken,
	})
	ts.Require().Equal("COMPLETE", reauth.FlowStatus, "re-authentication should complete the flow")

	// The subject authenticated moments ago, so a generous window must now be satisfiable from the
	// session without another prompt.
	_, thirdExecutionID := ts.authorizeWithMaxAge(client, "openid", "max_age_refresh_3", "10000")
	third := ts.flowExecute(client, map[string]interface{}{"executionId": thirdExecutionID})

	ts.Equal("COMPLETE", third.FlowStatus,
		"the re-authentication must refresh the session's auth time, so a later max_age is satisfied")
	ts.NotEmpty(third.Assertion, "the reused session should yield an assertion")
	ts.Nil(third.Error, "no interaction_required error should be reported after a fresh re-authentication")
}
