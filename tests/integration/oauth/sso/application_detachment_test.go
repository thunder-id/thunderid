// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sso

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	// The shipped administration flow that deletes an application, and the input its executors read
	// the target from. The name is deliberately not "applicationId": that is already taken at the top
	// level of the execution request, where it names the application the flow runs *for*.
	applicationDeletionFlowHandle = "default-application-deletion-flow"
	applicationTargetInput        = "targetApplicationId"

	// Two further applications bound to the same SSO authentication flow as the suite's main one, so
	// all three share a per-flow session and can participate in it together.
	detachClientID     = "sso_detach_test_client"
	detachClientSecret = "sso_detach_test_secret" //nolint:gosec // test credential
	soleClientID       = "sso_sole_participant_client"
	soleClientSecret   = "sso_sole_participant_secret" //nolint:gosec // test credential

	detachUsername = "sso_detach_user"
	soleUsername   = "sso_sole_user"
)

// applicationDeletionFlowID resolves the shipped application deletion flow by handle. Resolving by
// handle keeps the test independent of the seeded flow ids, which are a bootstrap detail.
func (ts *SSOLogoutTestSuite) applicationDeletionFlowID() string {
	ts.T().Helper()

	flowID, err := testutils.GetFlowIDByHandle(applicationDeletionFlowHandle, administrationFlowType)
	ts.Require().NoError(err, "failed to resolve the shipped application deletion flow")

	return flowID
}

// createParticipantApplication registers a further OAuth application on the suite's SSO authentication
// flow and returns its id. Sharing the flow is what puts it in the same per-flow SSO session as the
// suite's main application, which is the arrangement these tests need.
//
// The application is registered for best-effort cleanup: the tests here expect the deletion flow to
// remove it, so the cleanup only matters when a test fails before that happens.
func (ts *SSOLogoutTestSuite) createParticipantApplication(name, cID, cSecret string) string {
	ts.T().Helper()

	app := map[string]interface{}{
		"name":                      name,
		"description":               "Application participating in an SSO session for detachment tests",
		"ouId":                      testOUID,
		"type":                      "fullstack",
		"authFlowId":                ts.authFlowID,
		"isRegistrationFlowEnabled": false,
		"allowedUserTypes":          []string{testUserType.Name},
		"inboundAuthConfig": []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                cID,
					"clientSecret":            cSecret,
					"redirectUris":            []string{redirectURI},
					"grantTypes":              []string{"authorization_code", "refresh_token"},
					"responseTypes":           []string{"code"},
					"tokenEndpointAuthMethod": "client_secret_basic",
					"scopes":                  []string{"openid"},
				},
			},
		},
	}

	jsonData, err := json.Marshal(app)
	ts.Require().NoError(err, "failed to marshal participant application")

	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+"/applications", bytes.NewBuffer(jsonData))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err, "failed to create participant application")
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	ts.Require().Equal(http.StatusCreated, resp.StatusCode,
		"failed to create participant application: %s", string(body))

	var respData map[string]interface{}
	ts.Require().NoError(json.Unmarshal(body, &respData))
	id, ok := respData["id"].(string)
	ts.Require().True(ok, "participant application response missing id")

	ts.T().Cleanup(func() { ts.deleteAppIfPresent(id) })

	return id
}

// deleteAppIfPresent removes an application, tolerating one that a flow has already deleted. It is the
// teardown counterpart of createParticipantApplication, so a failed test does not leak a fixed client id
// into the next run.
func (ts *SSOLogoutTestSuite) deleteAppIfPresent(id string) {
	req, err := http.NewRequest(http.MethodDelete,
		fmt.Sprintf("%s/applications/%s", testutils.TestServerURL, id), nil)
	if err != nil {
		ts.T().Logf("Failed to build delete request for application %s: %v", id, err)
		return
	}
	resp, err := testutils.GetHTTPClient().Do(req)
	if err != nil {
		ts.T().Logf("Failed to delete application %s: %v", id, err)
		return
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
}

// authorizeAsClient starts an authorization code flow for a given client and returns the authId and
// executionId issued at the gate redirect. It is the multi-application counterpart of authorize, which
// is fixed to the suite's main client.
func (ts *SSOLogoutTestSuite) authorizeAsClient(client *http.Client, cID, scope, state string) (
	string, string) {
	ts.T().Helper()

	params := url.Values{}
	params.Set("client_id", cID)
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	params.Set("scope", scope)
	params.Set("state", state)

	req, err := http.NewRequest(http.MethodGet,
		testutils.TestServerURL+"/oauth2/authorize?"+params.Encode(), nil)
	ts.Require().NoError(err)

	resp, err := client.Do(req)
	ts.Require().NoError(err, "authorize request failed")
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusFound, resp.StatusCode, "authorize should redirect to the gate")
	authID, executionID, err := testutils.ExtractAuthData(resp.Header.Get("Location"))
	ts.Require().NoError(err, "failed to extract auth data from authorize redirect")

	return authID, executionID
}

// loginAsClient drives a first-time login through the given client, establishing the per-flow SSO
// session with that application recorded as its participant.
func (ts *SSOLogoutTestSuite) loginAsClient(client *http.Client, cID, username, state string) {
	ts.T().Helper()

	_, executionID := ts.authorizeAsClient(client, cID, "openid", state)

	initial := ts.flowExecute(client, map[string]interface{}{"executionId": executionID})
	ts.Require().NotEqual("COMPLETE", initial.FlowStatus, "first login must prompt for credentials")

	step := ts.flowExecute(client, map[string]interface{}{
		"executionId":    executionID,
		"inputs":         map[string]string{"username": username, "password": testPassword},
		"action":         "action_001",
		"challengeToken": initial.ChallengeToken,
	})
	ts.Require().Equal("COMPLETE", step.FlowStatus, "credential login should complete the flow")
}

// ssoSatisfies reports whether the client's live session short-circuits an authorize for the given
// client id. A completed initial step means SSO_CHECK found the session; anything else means the flow
// fell back to asking for credentials.
func (ts *SSOLogoutTestSuite) ssoSatisfies(client *http.Client, cID, state string) bool {
	ts.T().Helper()

	_, executionID := ts.authorizeAsClient(client, cID, "openid", state)
	step := ts.flowExecute(client, map[string]interface{}{"executionId": executionID})

	return step.FlowStatus == "COMPLETE"
}

// deleteApplicationThroughFlow runs the shipped application deletion flow against the given application.
//
// The bearer token is set on a raw client because the shared test clients treat /flow/execute as a
// public endpoint and skip token injection, which would make the request anonymous and be refused at
// the administration entry point.
func (ts *SSOLogoutTestSuite) deleteApplicationThroughFlow(flowID, appID string) {
	ts.T().Helper()

	token, err := testutils.GetAccessToken()
	ts.Require().NoError(err, "failed to obtain admin access token")

	reqBody, err := json.Marshal(map[string]interface{}{
		"flowId": flowID,
		"inputs": map[string]string{applicationTargetInput: appID},
	})
	ts.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+"/flow/execute",
		bytes.NewReader(reqBody))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err, "failed to execute the application deletion flow")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, resp.StatusCode,
		"application deletion flow should run: %s", string(body))

	var step testutils.FlowStep
	ts.Require().NoError(json.Unmarshal(body, &step))
	ts.Require().Equal("COMPLETE", step.FlowStatus,
		"application deletion flow should complete: %s", string(body))
}

// Deleting an application detaches it from the SSO sessions it participates in, and detaches nothing
// else: a session shared with other applications survives, because it can still back SSO for them.
//
// The session is proven live for both applications before the deletion, so what is asserted afterwards
// is a change in behavior for one and its absence for the other, rather than an absence on its own.
func (ts *SSOLogoutTestSuite) TestApplicationDeletion_DetachesOnlyThatApplication() {
	flowID := ts.applicationDeletionFlowID()
	ts.Require().NotEmpty(flowID, "the shipped application deletion flow should be present")

	ts.createUser(detachUsername)
	appID := ts.createParticipantApplication("SSODetachTestApp", detachClientID, detachClientSecret)

	client := ts.newSessionClient()

	// The main application establishes the session; the second joins it over SSO. Both are now
	// recorded as participants of the one session.
	ts.loginAsClient(client, clientID, detachUsername, "detach_state_1")
	ts.Require().NotEmpty(ts.ssoCookieNames(client), "an SSO cookie should be set after login")
	ts.Require().True(ts.ssoSatisfies(client, detachClientID, "detach_state_2"),
		"the second application must join the session for this test to mean anything")

	ts.deleteApplicationThroughFlow(flowID, appID)

	// The session outlives the deleted participant: the application that still exists keeps being
	// served from it without a credential prompt.
	ts.True(ts.ssoSatisfies(client, clientID, "detach_state_3"),
		"deleting one participant must not end a session another application still participates in")
}

// A session exists to back SSO for the applications in it. When the deleted application was the only
// participant, nothing is left to participate, so the session goes with it rather than lingering as a
// live credential that no application can use.
func (ts *SSOLogoutTestSuite) TestApplicationDeletion_EndsTheSessionWhenItsLastParticipantGoes() {
	flowID := ts.applicationDeletionFlowID()
	ts.Require().NotEmpty(flowID, "the shipped application deletion flow should be present")

	ts.createUser(soleUsername)
	appID := ts.createParticipantApplication("SSOSoleParticipantApp", soleClientID, soleClientSecret)

	// A fresh client, so this session's only participant is the application about to be deleted.
	client := ts.newSessionClient()
	ts.loginAsClient(client, soleClientID, soleUsername, "sole_state_1")
	ts.Require().NotEmpty(ts.ssoCookieNames(client), "an SSO cookie should be set after login")
	ts.Require().True(ts.ssoSatisfies(client, soleClientID, "sole_state_2"),
		"the session must be live before the deletion for this test to mean anything")

	ts.deleteApplicationThroughFlow(flowID, appID)

	// The same cookie must no longer satisfy SSO on this flow, for any application on it.
	ts.False(ts.ssoSatisfies(client, clientID, "sole_state_3"),
		"a session whose last participant was deleted must no longer satisfy an authorize request")
}
