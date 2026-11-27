// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// logCleanup deletes a fixture, logging rather than failing when the delete fails.
func (ts *SessionAPITestSuite) logCleanup(kind, id string, deleteFn func(string) error) {
	if err := deleteFn(id); err != nil {
		ts.T().Logf("Failed to delete %s %s: %v", kind, id, err)
	}
}

// createApplication creates an application on the sign-in flow and the suite's sign-out flow, in the suite's
// organization unit.
func (ts *SessionAPITestSuite) createApplication(name, cID, cSecret, authFlowID string, grantTypes []string) string {
	appID, err := testutils.CreateApplication(testutils.Application{
		Name:             name,
		Description:      "Application for session management API integration tests",
		OUID:             testOUID,
		AuthFlowID:       authFlowID,
		SignOutFlowID:    ts.signOutFlowID,
		AllowedUserTypes: []string{testUserType.Handle},
		InboundAuthConfig: []map[string]interface{}{{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                cID,
				"clientSecret":            cSecret,
				"redirectUris":            []string{redirectURI},
				"postLogoutRedirectUris":  []string{postLogoutRedirectURI},
				"grantTypes":              grantTypes,
				"responseTypes":           []string{"code"},
				"tokenEndpointAuthMethod": "client_secret_basic",
				"scopes":                  []string{"openid"},
			},
		}},
	})
	ts.Require().NoError(err, "Failed to create application %s", name)
	return appID
}

// createUser creates a user in the suite's organization unit, deleted when the suite ends.
func (ts *SessionAPITestSuite) createUser(username string) string {
	userID, err := testutils.CreateUser(testutils.User{
		OUID: testOUID,
		Type: testUserType.Handle,
		Attributes: json.RawMessage(fmt.Sprintf(`{"username": %q, "password": %q, "email": %q}`,
			username, testPassword, username+"@example.com")),
	})
	ts.Require().NoError(err, "Failed to create user %s", username)
	ts.userIDs = append(ts.userIDs, userID)
	return userID
}

// shareUserInto shares the user into the organization unit.
func (ts *SessionAPITestSuite) shareUserInto(userID, ouID string) {
	body := fmt.Sprintf(`{"childUnits":{"mode":"selected","units":[{"ouId":%q,"reach":"unit"}]}}`, ouID)
	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+"/users/"+userID+"/sharing-policies",
		strings.NewReader(body))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	ts.Require().Equal(http.StatusCreated, resp.StatusCode, "share the user: %s", string(raw))
}

// newSessionClient returns a browser-like client: a cookie jar carries the SSO cookie across requests, and
// redirects are not followed.
func (ts *SessionAPITestSuite) newSessionClient() *http.Client {
	client, err := testutils.NewSessionClient()
	ts.Require().NoError(err, "Failed to create the cookie jar")
	return client
}

// getSessions calls the session management API with client and decodes a 200 response into out.
func (ts *SessionAPITestSuite) getSessions(client *http.Client, path string, out interface{}) (int, string) {
	ts.T().Helper()
	status, raw, err := testutils.GetSessions(client, path, out)
	ts.Require().NoError(err)
	return status, raw
}

// deleteSessions sends a DELETE to the session management API with client and returns the status and body.
func (ts *SessionAPITestSuite) deleteSessions(client *http.Client, path string) (int, string) {
	ts.T().Helper()
	status, raw, err := testutils.DeleteSessions(client, path)
	ts.Require().NoError(err)
	return status, raw
}

// authorize starts an authorization code flow as the client and returns the authId and executionId.
func (ts *SessionAPITestSuite) authorize(client *http.Client, cID, scope, state string) (string, string) {
	params := url.Values{}
	params.Set("client_id", cID)
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	params.Set("scope", scope)
	params.Set("state", state)
	authID, executionID, err := testutils.StartAuthorizationWithSession(client, params)
	ts.Require().NoError(err, "authorize request failed")
	return authID, executionID
}

// flowExecute posts a step to /flow/execute with the session client and returns the next step.
func (ts *SessionAPITestSuite) flowExecute(client *http.Client, body map[string]interface{}) *testutils.FlowStep {
	status, respBody, err := testutils.ExecuteFlowRaw(client, "", body)
	ts.Require().NoError(err, "flow execute request failed")
	ts.Require().Equal(http.StatusOK, status, "flow execute failed: %s", string(respBody))
	var step testutils.FlowStep
	ts.Require().NoError(json.Unmarshal(respBody, &step), "failed to decode the flow step")
	return &step
}

// signIn runs a first sign-in as the suite's application, prompting for the user's credentials, and returns
// the issued tokens.
func (ts *SessionAPITestSuite) signIn(client *http.Client, username, scope, state string) *testutils.TokenResponse {
	return ts.signInAs(client, clientID, clientSecret, username, scope, state)
}

// signInAs runs a first sign-in as the client, prompting for the user's credentials, and returns the issued
// tokens.
func (ts *SessionAPITestSuite) signInAs(client *http.Client, cID, cSecret, username, scope,
	state string) *testutils.TokenResponse {
	authID, executionID := ts.authorize(client, cID, scope, state)
	code, err := testutils.ExtractAuthorizationCode(ts.completeSignIn(client, authID, executionID, username))
	ts.Require().NoError(err, "failed to extract the authorization code")
	return ts.exchangeCode(client, cID, cSecret, code)
}

// signInUnder signs the user in as the client through the organization unit's authorization endpoint, so the
// sign-in is recorded under that unit, and returns the client redirect carrying the code.
func (ts *SessionAPITestSuite) signInUnder(client *http.Client, ouID, cID, username, state string) string {
	params := url.Values{}
	params.Set("client_id", cID)
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	params.Set("scope", "openid")
	params.Set("state", state)
	resp, err := client.Get(testutils.TestServerURL + "/ou/" + ouID + "/oauth2/authorize?" + params.Encode())
	ts.Require().NoError(err, "authorize request failed")
	defer func() { _ = resp.Body.Close() }()
	ts.Require().Equal(http.StatusFound, resp.StatusCode, "authorize should redirect to the sign-in")
	authID, executionID, err := testutils.ExtractAuthData(resp.Header.Get("Location"))
	ts.Require().NoError(err, "the redirect should carry the flow")
	return ts.completeSignIn(client, authID, executionID, username)
}

// completeSignIn answers a first sign-in's credential prompt and the authorize callback, and returns the client
// redirect carrying the code.
func (ts *SessionAPITestSuite) completeSignIn(client *http.Client, authID, executionID, username string) string {
	initial := ts.flowExecute(client, map[string]interface{}{"executionId": executionID})
	ts.Require().NotEqual("COMPLETE", initial.FlowStatus, "a first sign-in prompts for credentials")

	step := ts.flowExecute(client, map[string]interface{}{
		"executionId":    executionID,
		"inputs":         map[string]string{"username": username, "password": testPassword},
		"action":         "action_001",
		"challengeToken": initial.ChallengeToken,
	})
	ts.Require().Equal("COMPLETE", step.FlowStatus, "the credential sign-in should complete")
	sessionData := make([]testutils.FlowSessionData, 0, len(initial.SessionData)+len(step.SessionData))
	for _, issued := range append(initial.SessionData, step.SessionData...) {
		sessionData = append(sessionData, issued.FlowSessionData)
	}

	authzResponse, err := testutils.CompleteAuthorizationWithSessionData(client, authID, step.Assertion, sessionData)
	ts.Require().NoError(err, "the authorize callback failed")
	return authzResponse.RedirectURI
}

// exchangeCode swaps an authorization code for the client's tokens.
func (ts *SessionAPITestSuite) exchangeCode(client *http.Client, cID, cSecret, code string) *testutils.TokenResponse {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	status, raw := ts.tokenRequest(client, "/oauth2/token", cID, cSecret, form)
	ts.Require().Equal(http.StatusOK, status, "the code exchange failed: %s", raw)
	var token testutils.TokenResponse
	ts.Require().NoError(json.Unmarshal([]byte(raw), &token), "failed to decode the token response")
	return &token
}

// refresh redeems a refresh token as the client and returns the new tokens.
func (ts *SessionAPITestSuite) refresh(cID, cSecret, refreshToken string) *testutils.TokenResponse {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	status, raw := ts.tokenRequest(testutils.GetRawHTTPClient(), "/oauth2/token", cID, cSecret, form)
	ts.Require().Equal(http.StatusOK, status, "the refresh failed: %s", raw)
	var token testutils.TokenResponse
	ts.Require().NoError(json.Unmarshal([]byte(raw), &token), "failed to decode the token response")
	return &token
}

// createAppNativeApplication creates a confidential client with only the token-exchange grant on the flow, so it is
// issued a Flow Secret, runs the flow itself and exchanges the assertion for tokens.
func (ts *SessionAPITestSuite) createAppNativeApplication(name, cID, cSecret, authFlowID string) string {
	appID, err := testutils.CreateApplication(testutils.Application{
		Name:             name,
		Description:      "App-native application for session management API integration tests",
		OUID:             testOUID,
		AuthFlowID:       authFlowID,
		SignOutFlowID:    ts.signOutFlowID,
		AllowedUserTypes: []string{testUserType.Handle},
		InboundAuthConfig: []map[string]interface{}{{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                cID,
				"clientSecret":            cSecret,
				"grantTypes":              []string{tokenExchangeGrant},
				"tokenEndpointAuthMethod": "client_secret_basic",
			},
		}},
	})
	ts.Require().NoError(err, "Failed to create application %s", name)
	ts.Require().NotEmpty(testutils.GetFlowSecret(appID), "an app-native application is issued a Flow Secret")
	return appID
}

// signInAppNative signs the user in through the application's flow with its Flow Secret, as an app-native
// application does, and exchanges the assertion for the client's tokens.
func (ts *SessionAPITestSuite) signInAppNative(appID, cID, cSecret, username string) *testutils.TokenResponse {
	initial, err := common.InitiateAuthenticationFlow(appID, false, nil, "")
	ts.Require().NoError(err, "the app-native sign-in should start")
	ts.Require().Equal("INCOMPLETE", initial.FlowStatus, "an app-native sign-in prompts for credentials")
	step, err := testutils.ExecuteAuthenticationFlow(initial.ExecutionID,
		map[string]string{"username": username, "password": testPassword}, "action_001", initial.ChallengeToken)
	ts.Require().NoError(err, "the credential step failed")
	ts.Require().Equal("COMPLETE", step.FlowStatus, "the credential sign-in should complete")
	ts.Require().NotEmpty(step.Assertion, "the sign-in should return an assertion")

	form := url.Values{}
	form.Set("grant_type", tokenExchangeGrant)
	form.Set("subject_token", step.Assertion)
	form.Set("subject_token_type", jwtTokenType)
	status, raw := ts.tokenRequest(testutils.GetRawHTTPClient(), "/oauth2/token", cID, cSecret, form)
	ts.Require().Equal(http.StatusOK, status, "the assertion exchange failed: %s", raw)
	var token testutils.TokenResponse
	ts.Require().NoError(json.Unmarshal([]byte(raw), &token), "failed to decode the token response")
	return &token
}

// assertDirectSessionListed checks that the application's one sign-in recorded a direct session that is listed by
// the application and as the user's, and that the user lists it with the access token, as the current session. It
// returns the session's ID.
func (ts *SessionAPITestSuite) assertDirectSessionListed(appID, userID, accessToken string) string {
	ts.T().Helper()
	var byApp testutils.SessionListView
	status, raw := ts.getSessions(testutils.GetHTTPClient(), "/sessions?appId="+url.QueryEscape(appID), &byApp)
	ts.Require().Equal(http.StatusOK, status, raw)
	ts.Require().Len(byApp.Sessions, 1, "the application's one sign-in recorded one session: %s", raw)
	direct := byApp.Sessions[0]
	ts.Equal("DIRECT", direct.Type)
	ts.Equal(userID, direct.UserID)
	ts.Require().Len(direct.Participants, 1, "a direct session holds only the application that signed in")
	ts.Equal(appID, direct.Participants[0].AppID)

	listed, err := testutils.ListedSession(testutils.GetHTTPClient(), "/sessions?userId="+url.QueryEscape(userID),
		direct.ID)
	ts.Require().NoError(err)
	ts.Require().NotNil(listed, "the user's sessions include the direct one")
	ts.Equal("DIRECT", listed.Type)

	var mine testutils.SessionListView
	status, raw = ts.getSessions(testutils.GetHTTPClientWithToken(accessToken), "/users/me/sessions", &mine)
	ts.Require().Equal(http.StatusOK, status, raw)
	current := ""
	for _, s := range mine.Sessions {
		ts.Equal(userID, s.UserID, "own sessions are the user's")
		ts.Require().NotNil(s.IsCurrent, "own sessions always say whether they are current")
		if *s.IsCurrent {
			ts.Require().Empty(current, "only one session is current: %s", raw)
			current = s.ID
		}
	}
	ts.Equal(direct.ID, current, "the access token's own session is current: %s", raw)
	return direct.ID
}

// exchangeDeviceSecret exchanges a device secret at the token endpoint at path as the client, signing the user
// in to that application over the session the secret carries, and returns the access token.
func (ts *SessionAPITestSuite) exchangeDeviceSecret(path, cID, cSecret, deviceSecret string) string {
	ts.Require().NotEmpty(deviceSecret, "the first sign-in should return a device secret")
	form := url.Values{}
	form.Set("grant_type", tokenExchangeGrant)
	form.Set("subject_token", deviceSecret)
	form.Set("subject_token_type", jwtTokenType)
	form.Set("scope", "openid")
	status, raw := ts.tokenRequest(testutils.GetRawHTTPClient(), path, cID, cSecret, form)
	ts.Require().Equal(http.StatusOK, status, "the exchange at %s failed: %s", path, raw)
	var token testutils.TokenResponse
	ts.Require().NoError(json.Unmarshal([]byte(raw), &token), "failed to decode the token response")
	ts.Require().NotEmpty(token.AccessToken, "the exchange should issue an access token")
	return token.AccessToken
}

// tokenRequest posts a form to the token endpoint at path as the client and returns the status and body.
func (ts *SessionAPITestSuite) tokenRequest(client *http.Client, path, cID, cSecret string,
	form url.Values) (int, string) {
	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+path, strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(cID, cSecret)
	resp, err := client.Do(req)
	ts.Require().NoError(err, "token request failed")
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(raw)
}

// ssoSatisfies reports whether an authorize as the client completes from the browser's SSO session alone.
func (ts *SessionAPITestSuite) ssoSatisfies(client *http.Client, cID, state string) bool {
	_, executionID := ts.authorize(client, cID, "openid", state)
	return ts.flowExecute(client, map[string]interface{}{"executionId": executionID}).FlowStatus == "COMPLETE"
}

// signOut runs the RP-initiated sign-out for the session behind the ID token.
func (ts *SessionAPITestSuite) signOut(client *http.Client, idToken, state string) {
	form := url.Values{}
	form.Set("id_token_hint", idToken)
	form.Set("post_logout_redirect_uri", postLogoutRedirectURI)
	form.Set("state", state)
	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+"/oauth2/logout",
		strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	ts.Require().NoError(err, "logout request failed")
	defer func() { _ = resp.Body.Close() }()
	ts.Require().Equal(http.StatusFound, resp.StatusCode, "logout should redirect to the sign-out page")

	location, err := url.Parse(resp.Header.Get("Location"))
	ts.Require().NoError(err, "failed to parse the sign-out redirect")
	step := ts.flowExecute(client, map[string]interface{}{"executionId": location.Query().Get("executionId")})
	ts.Require().Equal("COMPLETE", step.FlowStatus, "the sign-out flow should complete")
}

// tokenClaim returns a string claim of a token.
func (ts *SessionAPITestSuite) tokenClaim(token, claim string) string {
	claims, err := testutils.DecodeJWT(token)
	ts.Require().NoError(err, "decode the token")
	if claim == "ouId" {
		return claims.OUID
	}
	value, _ := claims.Additional[claim].(string)
	return value
}

// participantOUs returns the organization unit each participant of the session was recorded under, keyed by
// the token family the participant holds.
func (ts *SessionAPITestSuite) participantOUs(sessionID string) map[string]string {
	out, err := testutils.QueryRuntimeDB(fmt.Sprintf(
		`SELECT COALESCE(TFID, '') || '|' || OU_ID FROM "SESSION_PARTICIPANT" WHERE SESSION_ID = '%s';`, sessionID))
	ts.Require().NoError(err, "read the session's participants")
	ous := make(map[string]string)
	for _, row := range strings.Split(out, "\n") {
		if tfid, ouID, ok := strings.Cut(row, "|"); ok {
			ous[tfid] = ouID
		}
	}
	return ous
}

// listOwnSessions returns the caller's sessions from GET /users/me/sessions.
func (ts *SessionAPITestSuite) listOwnSessions(accessToken string) testutils.SessionListView {
	ts.T().Helper()

	var list testutils.SessionListView
	status, raw := ts.getSessions(testutils.GetHTTPClientWithToken(accessToken), "/users/me/sessions", &list)
	ts.Require().Equal(http.StatusOK, status, raw)
	return list
}

// currentSessionID returns the ID of the session that issued the access token.
func (ts *SessionAPITestSuite) currentSessionID(accessToken string) string {
	ts.T().Helper()

	for _, s := range ts.listOwnSessions(accessToken).Sessions {
		if s.IsCurrent != nil && *s.IsCurrent {
			return s.ID
		}
	}
	ts.FailNow("no session is marked as current")
	return ""
}

// introspectActive reports whether the access token is still active.
func (ts *SessionAPITestSuite) introspectActive(accessToken string) bool {
	ts.T().Helper()

	active, err := testutils.IntrospectToken(accessToken, clientID, clientSecret)
	ts.Require().NoError(err, "introspection request failed")
	return active
}

// createSessionResourceServer creates a resource server in the organization unit that reproduces the Data
// Plane's session permissions, org:session:view and org:session:delete.
func (ts *SessionAPITestSuite) createSessionResourceServer(ouID, identifier string) string {
	rsID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name: "Session API Revoke RS", Identifier: identifier, OUID: ouID,
	}, nil)
	ts.Require().NoError(err, "create the resource server")
	ts.T().Cleanup(func() { ts.logCleanup("resource server", rsID, testutils.DeleteResourceServerWithChildren) })
	_, err = testutils.CreateResource(rsID, "System", "system", "")
	ts.Require().NoError(err, "create the system resource")
	orgID, err := testutils.CreateResource(rsID, "Organization", "org", "")
	ts.Require().NoError(err, "create the org resource")
	sessionID, err := testutils.CreateResource(rsID, "session", "session", orgID)
	ts.Require().NoError(err, "create the session resource")
	for _, action := range []testutils.Action{{Name: "View", Handle: "view"}, {Name: "Delete", Handle: "delete"}} {
		_, err = testutils.CreateResourceAction(rsID, sessionID, action)
		ts.Require().NoError(err, "create the %s action", action.Handle)
	}
	return rsID
}

// orgSessionToken signs a new user in to the organization unit, holding only permission there, and returns their
// access token, which names the unit. A user of the suite's unit is shared into any other unit first.
func (ts *SessionAPITestSuite) orgSessionToken(signInApps *testutils.OUSignInApps, ouID, rsID, identifier,
	username, permission string) string {
	userID := ts.createUser(username)
	if ouID != testOUID {
		ts.shareUserInto(userID, ouID)
	}
	roleID, err := testutils.CreateRole(testutils.Role{
		Name: "SessionAPI_" + username,
		OUID: ouID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: rsID, Permissions: []string{permission}},
		},
		Assignments: []testutils.Assignment{{ID: userID, Type: "user"}},
	})
	ts.Require().NoError(err, "create the %s role", permission)
	ts.T().Cleanup(func() { ts.logCleanup("role", roleID, testutils.DeleteRole) })

	token, err := signInApps.ObtainAccessToken(ouID, permission, username, testPassword, identifier)
	ts.Require().NoError(err, "obtain the %s token", permission)
	ts.Require().Equal(ouID, ts.tokenClaim(token.AccessToken, "ouId"), "the token names the unit")
	return token.AccessToken
}
