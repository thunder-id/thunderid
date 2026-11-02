// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sso

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	// The shipped administration flow that deletes a user, its flow type, and the input its
	// executors read the target user from.
	userDeletionFlowHandle = "default-user-deletion-flow"
	administrationFlowType = "ADMINISTRATION"
	deletionSubjectInput   = "subject"
)

// deletionFlowID resolves the shipped user deletion flow by handle.
func (ts *SSOLogoutTestSuite) deletionFlowID() string {
	ts.T().Helper()

	flowID, err := testutils.GetFlowIDByHandle(userDeletionFlowHandle, administrationFlowType)
	ts.Require().NoError(err, "failed to resolve the shipped user deletion flow")

	return flowID
}

// deleteUserThroughFlow runs the shipped deletion flow against the given subject.
//
// The bearer token is set on a raw client because the shared test clients treat /flow/execute as a
// public endpoint and skip token injection, which would make the request anonymous and be refused
// at the administration entry point.
func (ts *SSOLogoutTestSuite) deleteUserThroughFlow(flowID, subjectID string) {
	ts.T().Helper()

	token, err := testutils.GetAccessToken()
	ts.Require().NoError(err, "failed to obtain admin access token")

	reqBody, err := json.Marshal(map[string]interface{}{
		"flowId": flowID,
		"inputs": map[string]string{deletionSubjectInput: subjectID},
	})
	ts.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+"/flow/execute", bytes.NewReader(reqBody))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err, "failed to execute the deletion flow")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, resp.StatusCode, "deletion flow should run: %s", string(body))

	var step testutils.FlowStep
	ts.Require().NoError(json.Unmarshal(body, &step))
	ts.Require().Equal("COMPLETE", step.FlowStatus, "deletion flow should complete: %s", string(body))
}

// Deleting a user tears down every SSO session that user holds, not just the record. Without this,
// a deleted user's cookie would keep satisfying SSO_CHECK and authorize requests would continue to
// be served for a subject that no longer exists.
//
// The session is proven live before the deletion by a second authorize that skips the credential
// prompt, so the assertion afterwards is a change in behaviour rather than an absence.
func (ts *SSOLogoutTestSuite) TestTerminateBySubject_UserDeletionEndsSSOSessions() {
	flowID := ts.deletionFlowID()
	ts.Require().NotEmpty(flowID, "the shipped user deletion flow should be present")

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

	ts.deleteUserThroughFlow(flowID, userID)

	// The same cookie must no longer satisfy SSO: the flow falls back to asking for credentials.
	_, afterExecutionID := ts.authorize(client, "openid", "terminate_state_3")
	after := ts.flowExecute(client, map[string]interface{}{"executionId": afterExecutionID})
	ts.NotEqual("COMPLETE", after.FlowStatus,
		"a deleted user's SSO session must no longer satisfy an authorize request")
}
