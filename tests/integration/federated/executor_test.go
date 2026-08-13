// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package federated

import (
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

/*
The generic executors as flow nodes, and the state parameter they guard the callback with.

Until this file, OAuthExecutor and OIDCAuthExecutor appeared in no flow graph anywhere: only the Google
and GitHub specialisations did, and those are five-statement wrappers around this code. The state
scenarios matter more than they look — ErrInvalidOAuthState had no coverage at any level, and the
executor accepts a callback that returns no state at all, which is a deliberate product decision rather
than an oversight (G3).
*/

// startFederatedFlow initiates an authentication flow and returns the step plus the authorization code
// and state the provider issued.
func (s *FederatedMappingSuite) startFederatedFlow(
	appID string, user *testutils.OIDCUserInfo) (*common.FlowStep, string, string) {
	s.T().Helper()
	s.activeSub = user.Sub

	step, err := common.InitiateAuthenticationFlow(appID, false, nil, "")
	s.Require().NoError(err, "failed to initiate the authentication flow")
	s.Require().Equal("REDIRECTION", step.Type, "expected a redirection, got %+v", step)

	code, state, err := testutils.SimulateFederatedOAuthFlow(step.Data.RedirectURL)
	s.Require().NoError(err, "failed to simulate authorization at the identity provider")
	return step, code, state
}

// knownOIDCIdentity registers an identity on the OIDC mock with a local user to resolve to.
func (s *FederatedMappingSuite) knownOIDCIdentity() *testutils.OIDCUserInfo {
	s.T().Helper()
	user := s.baseUser(s.nextSubject())
	email := user.Sub + "@example.com"
	s.createLocalUser(map[string]interface{}{"username": email, "email": email, "sub": user.Sub})
	s.applyConfig(mapping(fedPersonType.Name, pair("email", "email")))
	s.mockOIDC.AddUser(user)
	return user
}

// knownOAuthIdentity is the same for the generic OAuth connection.
func (s *FederatedMappingSuite) knownOAuthIdentity() *testutils.OIDCUserInfo {
	s.T().Helper()
	sub := s.nextSubject()
	email := sub + "@example.com"
	s.mockOAuth.AddUser(&testutils.OAuthUserInfo{Sub: sub, Email: email, Name: "OAuth User"})
	s.createLocalUser(map[string]interface{}{"username": email, "email": email, "sub": sub})
	s.applyConfigTo("oauth", s.oauthIDPID, mapping(fedPersonType.Name, pair("email", "email")))
	return &testutils.OIDCUserInfo{Sub: sub, Email: email}
}

// B17: the generic OIDC executor drives an authentication flow to completion.
func (s *FederatedMappingSuite) TestGenericOIDCExecutorAsFlowNode() {
	user := s.knownOIDCIdentity()
	step, code, state := s.startFederatedFlow(s.strictAuthAppID, user)

	completed, err := common.CompleteFlow(
		step.ExecutionID, map[string]string{"code": code, "state": state}, "", step.ChallengeToken)
	s.Require().NoError(err, "the generic OIDC executor should complete the flow")
	s.Equal("COMPLETE", completed.FlowStatus, "got %+v", completed)
	s.NotEmpty(completed.Assertion, "a completed authentication should carry an assertion")
}

// B18: the same for the generic OAuth executor, which has no ID token and reads the profile from the
// userinfo endpoint.
func (s *FederatedMappingSuite) TestGenericOAuthExecutorAsFlowNode() {
	user := s.knownOAuthIdentity()
	step, code, state := s.startFederatedFlow(s.oauthAppID, user)

	completed, err := common.CompleteFlow(
		step.ExecutionID, map[string]string{"code": code, "state": state}, "", step.ChallengeToken)
	s.Require().NoError(err, "the generic OAuth executor should complete the flow")
	s.Equal("COMPLETE", completed.FlowStatus, "got %+v", completed)
	s.NotEmpty(completed.Assertion, "a completed authentication should carry an assertion")
}

// B19: a callback returning a state the server did not issue is refused. This closed the last branch of
// ErrInvalidOAuthState, which had no coverage at any level.
func (s *FederatedMappingSuite) TestOIDCStateMismatchRefused() {
	user := s.knownOIDCIdentity()
	step, code, _ := s.startFederatedFlow(s.strictAuthAppID, user)

	completed, err := common.CompleteFlow(step.ExecutionID,
		map[string]string{"code": code, "state": "a-state-the-server-never-issued"}, "", step.ChallengeToken)

	s.assertNotAuthenticated(completed, err, "a mismatched state must not authenticate")
}

// B20: the same on the generic OAuth executor, which carries its own copy of the state check.
func (s *FederatedMappingSuite) TestOAuthStateMismatchRefused() {
	user := s.knownOAuthIdentity()
	step, code, _ := s.startFederatedFlow(s.oauthAppID, user)

	completed, err := common.CompleteFlow(step.ExecutionID,
		map[string]string{"code": code, "state": "a-state-the-server-never-issued"}, "", step.ChallengeToken)

	s.assertNotAuthenticated(completed, err, "a mismatched state must not authenticate")
}

// B20a: a callback that returns no state at all is accepted. The check is gated on the client returning
// one, which the code documents as deliberate for clients handling CSRF themselves. This pins that
// decision rather than asserting it is a defect; the residual risk is G3.
func (s *FederatedMappingSuite) TestOmittedStateIsAccepted() {
	user := s.knownOIDCIdentity()
	step, code, _ := s.startFederatedFlow(s.strictAuthAppID, user)

	completed, err := common.CompleteFlow(
		step.ExecutionID, map[string]string{"code": code}, "", step.ChallengeToken)

	s.Require().NoError(err, "omitting the state should not fail the exchange")
	s.Equal("COMPLETE", completed.FlowStatus,
		"a callback with no state proceeds; the server does not require one back: %+v", completed)
}

// B20b: submitting the code on the very first interaction skips the authorize step entirely, so no state
// was ever stored. A returned state then matches nothing and is refused.
func (s *FederatedMappingSuite) TestStateWithoutStoredStateRefused() {
	s.knownOIDCIdentity()

	// A first request already carrying a code: HasRequiredInputs is satisfied, so the executor processes
	// the callback without ever having built an authorize URL.
	step, err := common.InitiateAuthenticationFlow(s.strictAuthAppID, false,
		map[string]string{"code": "a-code-from-nowhere", "state": "a-state-from-nowhere"}, "")

	s.assertNotAuthenticated(step, err, "a state with nothing stored to compare against must be refused")
}

// B20c: replaying a completed callback. A validated state is deleted once used, so the replay has
// nothing to match — but the flow execution is also finished by then, and whichever check refuses first,
// the replay must not authenticate a second time.
func (s *FederatedMappingSuite) TestCallbackReplayRefused() {
	user := s.knownOIDCIdentity()
	step, code, state := s.startFederatedFlow(s.strictAuthAppID, user)
	inputs := map[string]string{"code": code, "state": state}

	completed, err := common.CompleteFlow(step.ExecutionID, inputs, "", step.ChallengeToken)
	s.Require().NoError(err, "the first callback should succeed")
	s.Require().Equal("COMPLETE", completed.FlowStatus)

	replayed, err := common.CompleteFlow(step.ExecutionID, inputs, "", step.ChallengeToken)
	s.assertNotAuthenticated(replayed, err, "replaying a completed callback must not authenticate again")
}

// assertNotAuthenticated accepts either an error from the exchange or a step that did not complete,
// since a refusal can surface as either depending on where it is caught.
func (s *FederatedMappingSuite) assertNotAuthenticated(
	step *common.FlowStep, err error, message string) {
	s.T().Helper()
	if err != nil {
		return
	}
	s.Require().NotNil(step, message)
	s.NotEqual("COMPLETE", step.FlowStatus, "%s, got %+v", message, step)
	s.Empty(step.Assertion, "%s: no assertion should be issued", message)
}
