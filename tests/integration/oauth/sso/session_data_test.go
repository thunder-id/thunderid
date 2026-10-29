// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sso

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// TestAppNativeFlow_ReturnsSessionData verifies that a flow the application initiates itself with its
// Flow Secret returns the minted SSO session as sessionData, keyed as the per-flow cookie is, and still
// sets that cookie with the same handle.
func (ts *SSOLogoutTestSuite) TestAppNativeFlow_ReturnsSessionData() {
	appID := ts.createAppNativeApp()
	defer ts.deleteAppByID(appID)
	client := ts.newSessionClient()

	held := ts.appNativeLogin(client, appID)

	ts.Equal(map[string]string{testutils.SSOCookiePrefix + held.FlowIdentifier: held.SessionHandle},
		ts.ssoCookies(client), "the per-flow SSO cookie carries the same identifier and handle")
}

// TestAppNativeFlow_ReusesSessionDataFromRequestBody verifies that an app-native caller with no cookie
// reuses the session by sending every session it holds: the server picks the one for its flow, and the
// flow completes on its initial step without prompting for credentials.
func (ts *SSOLogoutTestSuite) TestAppNativeFlow_ReusesSessionDataFromRequestBody() {
	appID := ts.createAppNativeApp()
	defer ts.deleteAppByID(appID)
	held := ts.appNativeLogin(ts.newSessionClient(), appID)

	step := ts.appNativeExecute(ts.newSessionClient(), appID, map[string]interface{}{
		"applicationId": appID,
		"flowType":      "AUTHENTICATION",
		"sessionData": []testutils.FlowSessionData{
			{FlowIdentifier: "0000000000000000", SessionHandle: "a-session-of-another-flow"},
			*held,
		},
	})

	ts.Equal("COMPLETE", step.FlowStatus, "the held session should skip authentication via SSO")
	ts.NotEmpty(step.Assertion, "the SSO-skipped flow should still yield an assertion")
}

// TestAppNativeSignOut_ClearsSession verifies that an app-native sign-out with the held session ends it
// and lists it in clearedSessions, after which the same session no longer skips authentication.
func (ts *SSOLogoutTestSuite) TestAppNativeSignOut_ClearsSession() {
	appID := ts.createAppNativeApp()
	defer ts.deleteAppByID(appID)
	held := ts.appNativeLogin(ts.newSessionClient(), appID)
	sessionData := []testutils.FlowSessionData{*held}

	signOut := ts.appNativeExecute(ts.newSessionClient(), appID, map[string]interface{}{
		"applicationId": appID,
		"flowType":      "SIGNOUT",
		"sessionData":   sessionData,
	})
	ts.Require().Equal("COMPLETE", signOut.FlowStatus, "sign-out should complete")
	ts.Equal([]string{held.FlowIdentifier}, signOut.ClearedSessions, "the ended session is listed as cleared")

	step := ts.appNativeExecute(ts.newSessionClient(), appID, map[string]interface{}{
		"applicationId": appID,
		"flowType":      "AUTHENTICATION",
		"sessionData":   sessionData,
	})
	ts.NotEqual("COMPLETE", step.FlowStatus, "a signed-out session must not skip authentication")
}

// TestBrowserFlow_ReturnsSessionData verifies that the same flow initiated through authorize, as a browser
// does, returns the session as sessionData too, with the same identifier and handle as the cookie.
func (ts *SSOLogoutTestSuite) TestBrowserFlow_ReturnsSessionData() {
	client := ts.newSessionClient()
	_, executionID := ts.authorize(client, "openid", "browser_handle_state")

	initial := ts.flowExecute(client, map[string]interface{}{"executionId": executionID})
	ts.Require().NotEqual("COMPLETE", initial.FlowStatus, "first login must prompt for credentials")
	step := ts.flowExecute(client, map[string]interface{}{
		"executionId":    executionID,
		"inputs":         map[string]string{"username": ssoReuseUsername, "password": testPassword},
		"action":         "action_001",
		"challengeToken": initial.ChallengeToken,
	})

	ts.Require().Equal("COMPLETE", step.FlowStatus, "credential login should complete the flow")
	ts.Require().Len(step.SessionData, 1, "a browser-initiated flow returns the session too")
	issued := step.SessionData[0]
	ts.Equal(map[string]string{testutils.SSOCookiePrefix + issued.FlowIdentifier: issued.SessionHandle},
		ts.ssoCookies(client), "the per-flow SSO cookie carries the same identifier and handle")
}

// TestBrowserFlow_AcceptsSessionDataInRequestBody verifies that a flow initiated through authorize also
// accepts the session from the request body: with no cookie, it completes without prompting.
func (ts *SSOLogoutTestSuite) TestBrowserFlow_AcceptsSessionDataInRequestBody() {
	appID := ts.createAppNativeApp()
	defer ts.deleteAppByID(appID)
	held := ts.appNativeLogin(ts.newSessionClient(), appID)

	client := ts.newSessionClient()
	_, executionID := ts.authorize(client, "openid", "browser_body_handle_state")
	step := ts.flowExecute(client, map[string]interface{}{
		"executionId": executionID,
		"sessionData": []testutils.FlowSessionData{*held},
	})

	ts.Equal("COMPLETE", step.FlowStatus, "the session in the body should skip authentication via SSO")
}

// appNativeLogin drives a first-time app-native login to completion and returns the session data the
// final step returns.
func (ts *SSOLogoutTestSuite) appNativeLogin(client *http.Client, appID string) *testutils.FlowSessionData {
	initial := ts.appNativeExecute(client, appID, map[string]interface{}{
		"applicationId": appID,
		"flowType":      "AUTHENTICATION",
	})
	ts.Require().Equal("INCOMPLETE", initial.FlowStatus, "first login must prompt for credentials")
	ts.Require().Empty(initial.SessionData, "no session exists before the credentials are verified")

	step := ts.appNativeExecute(client, appID, map[string]interface{}{
		"executionId":    initial.ExecutionID,
		"inputs":         map[string]string{"username": ssoReuseUsername, "password": testPassword},
		"action":         "action_001",
		"challengeToken": initial.ChallengeToken,
	})
	ts.Require().Equal("COMPLETE", step.FlowStatus, "credential login should complete the flow")
	ts.Require().Len(step.SessionData, 1, "an app-native caller gets the minted session")
	issued := step.SessionData[0]
	ts.Require().NotEmpty(issued.FlowIdentifier)
	ts.Require().NotEmpty(issued.SessionHandle)
	ts.Require().Positive(issued.ExpiresIn, "the session reports how long its handle stays usable")
	return &issued.FlowSessionData
}

// createAppNativeApp creates an application on the SSO sign-in and sign-out flows with no OAuth
// configuration, so it gets a Flow Secret and initiates flows itself.
func (ts *SSOLogoutTestSuite) createAppNativeApp() string {
	appID, err := testutils.CreateApplication(testutils.Application{
		Name:             "SSOAppNativeApp",
		Description:      "Application that runs the SSO flows through the flow API",
		OUID:             testOUID,
		AuthFlowID:       ts.authFlowID,
		SignOutFlowID:    ts.signOutFlowID,
		AllowedUserTypes: []string{testUserType.Handle},
	})
	ts.Require().NoError(err, "Failed to create the app-native application")
	ts.Require().NotEmpty(testutils.GetFlowSecret(appID), "the app-native application should get a Flow Secret")
	return appID
}

// appNativeExecute posts a step to /flow/execute with the application's Flow Secret, using the given
// client so any SSO cookie is carried and updated, and returns the resulting step.
func (ts *SSOLogoutTestSuite) appNativeExecute(client *http.Client, appID string,
	body map[string]interface{}) *testutils.FlowStep {
	payload, err := json.Marshal(body)
	ts.Require().NoError(err)
	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+"/flow/execute", bytes.NewReader(payload))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(testutils.FlowSecretHeaderName, testutils.GetFlowSecret(appID))

	resp, err := client.Do(req)
	ts.Require().NoError(err, "flow execute request failed")
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode, "flow execute failed")

	var step testutils.FlowStep
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&step), "failed to decode flow step")
	return &step
}

// ssoCookies returns the per-flow SSO cookies the client's jar holds, by name.
func (ts *SSOLogoutTestSuite) ssoCookies(client *http.Client) map[string]string {
	cookies, err := testutils.SSOCookies(client)
	ts.Require().NoError(err)
	return cookies
}
