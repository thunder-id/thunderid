// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sso

import "github.com/thunder-id/thunderid/tests/integration/testutils"

// Deleting a user tears down every SSO session that user holds, not just the record. Without this,
// a deleted user's cookie would keep satisfying SSO_CHECK and authorize requests would continue to
// be served for a subject that no longer exists.
//
// The session is proven live before the deletion by a second authorize that skips the credential
// prompt, so the assertion afterwards is a change in behaviour rather than an absence.
func (ts *SSOLogoutTestSuite) TestTerminateBySubject_UserDeletionEndsSSOSessions() {
	username := "sso_terminate_user"
	userID := ts.createUser(username)

	client := ts.newSessionClient()
	ts.login(client, username, "terminate_state_1")
	ts.Require().NotEmpty(ts.ssoCookieNames(client), "an SSO cookie should be set after login")

	// The session is live: this authorize is satisfied without a credential prompt.
	_, executionID := ts.authorize(client, "openid", "terminate_state_2")
	reused := ts.flowExecute(client, map[string]interface{}{"executionId": executionID})
	ts.Require().Equal("COMPLETE", reused.FlowStatus,
		"the session must be live before the deletion for this test to mean anything")

	ts.Require().NoError(testutils.DeleteUser(userID), "failed to delete the user")

	// The same cookie must no longer satisfy SSO: the flow falls back to asking for credentials.
	_, afterExecutionID := ts.authorize(client, "openid", "terminate_state_3")
	after := ts.flowExecute(client, map[string]interface{}{"executionId": afterExecutionID})
	ts.NotEqual("COMPLETE", after.FlowStatus,
		"a deleted user's SSO session must no longer satisfy an authorize request")
}
