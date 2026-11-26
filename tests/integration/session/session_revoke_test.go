// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"io"
	"net/http"
	"net/url"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// deleteSessions sends a DELETE to the session management API with the given client and returns the
// status code and the raw body.
func (ts *SessionAPITestSuite) deleteSessions(client *http.Client, path string) (int, string) {
	ts.T().Helper()

	req, err := http.NewRequest(http.MethodDelete, testutils.TestServerURL+path, nil)
	ts.Require().NoError(err)
	resp, err := client.Do(req)
	ts.Require().NoError(err, "session API request failed")
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	return resp.StatusCode, string(body)
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

// An administrator revokes one session: its token stops working, the browser has to sign in again,
// and revoking it again still succeeds.
func (ts *SessionAPITestSuite) TestAdminRevokesSession() {
	ts.createUser("session_api_revoke_one_user")
	client := ts.newSessionClient()
	tokens := ts.signIn(client, "session_api_revoke_one_user", "openid", "revoke_one_1")
	other := ts.signIn(ts.newSessionClient(), "session_api_revoke_one_user", "openid", "revoke_one_2")
	sessionID := ts.currentSessionID(tokens.AccessToken)

	status, raw := ts.deleteSessions(testutils.GetHTTPClient(), "/sessions/"+url.PathEscape(sessionID))
	ts.Require().Equal(http.StatusNoContent, status, raw)

	ts.False(ts.introspectActive(tokens.AccessToken), "the revoked session's token is no longer active")
	ts.True(ts.introspectActive(other.AccessToken), "the user's other session is untouched")
	ts.Equal(1, ts.listOwnSessions(other.AccessToken).TotalResults, "only the other session is left")
	// signIn asserts the flow asks for credentials, so the session no longer backs SSO.
	ts.signIn(client, "session_api_revoke_one_user", "openid", "revoke_one_3")

	status, raw = ts.deleteSessions(testutils.GetHTTPClient(), "/sessions/"+url.PathEscape(sessionID))
	ts.Equal(http.StatusNoContent, status, "revoking a session that is already gone still succeeds: %s", raw)
}

// An administrator revokes all of a user's sessions.
func (ts *SessionAPITestSuite) TestAdminRevokesUserSessions() {
	userID := ts.createUser("session_api_revoke_all_user")
	first := ts.signIn(ts.newSessionClient(), "session_api_revoke_all_user", "openid", "revoke_all_1")
	second := ts.signIn(ts.newSessionClient(), "session_api_revoke_all_user", "openid", "revoke_all_2")

	status, raw := ts.deleteSessions(testutils.GetHTTPClient(), "/sessions?userId="+url.QueryEscape(userID))
	ts.Require().Equal(http.StatusNoContent, status, raw)

	ts.False(ts.introspectActive(first.AccessToken), "every session's token is revoked")
	ts.False(ts.introspectActive(second.AccessToken), "every session's token is revoked")
	var list testutils.SessionListView
	status, raw = ts.getSessions(testutils.GetHTTPClient(), "/sessions?userId="+url.QueryEscape(userID), &list)
	ts.Require().Equal(http.StatusOK, status, raw)
	ts.Zero(list.TotalResults, "no session is left")

	// Revoking sessions is not a lockout: the user can sign in again and use the new token.
	again := ts.signIn(ts.newSessionClient(), "session_api_revoke_all_user", "openid", "revoke_all_3")
	ts.True(ts.introspectActive(again.AccessToken), "a new sign-in issues a working token")
}

// org:session:delete revokes another user's sessions under the organization unit the token names, and
// org:session:view alone does not.
func (ts *SessionAPITestSuite) TestSessionDeletePermissionRevokesOthers() {
	const identifier = "https://session-api-revoke.example.com/system"
	userID := ts.createUser("session_api_revoke_narrow_user")
	tokens := ts.signIn(ts.newSessionClient(), "session_api_revoke_narrow_user", "openid", "revoke_narrow_1")
	sessionID := ts.currentSessionID(tokens.AccessToken)

	rsID := ts.createSessionResourceServer(identifier)
	var signInApps testutils.OUSignInApps
	defer func() { ts.NoError(signInApps.DeleteAll()) }()
	viewer := ts.orgSessionToken(&signInApps, rsID, identifier, "session_api_revoke_viewer", "org:session:view")
	revoker := ts.orgSessionToken(&signInApps, rsID, identifier, "session_api_revoke_revoker",
		"org:session:delete")

	paths := []string{"/sessions/" + url.PathEscape(sessionID), "/sessions?userId=" + url.QueryEscape(userID)}
	for _, path := range paths {
		status, raw := ts.deleteSessions(testutils.GetHTTPClientWithToken(viewer), path)
		ts.Equal(http.StatusForbidden, status, "%s: %s", path, raw)
	}
	ts.True(ts.introspectActive(tokens.AccessToken), "the viewer revoked nothing")

	status, raw := ts.deleteSessions(testutils.GetHTTPClientWithToken(revoker), "/sessions/"+url.PathEscape(sessionID))
	ts.Require().Equal(http.StatusNoContent, status, raw)
	ts.False(ts.introspectActive(tokens.AccessToken), "the session's token is revoked")

	status, raw = ts.deleteSessions(testutils.GetHTTPClientWithToken(revoker),
		"/sessions?userId="+url.QueryEscape(userID))
	ts.Equal(http.StatusNoContent, status, raw)
}

// createSessionResourceServer creates a resource server in the suite's organization unit that reproduces the
// Data Plane's session permissions, org:session:view and org:session:delete.
func (ts *SessionAPITestSuite) createSessionResourceServer(identifier string) string {
	rsID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name: "Session API Revoke RS", Identifier: identifier, OUID: testOUID,
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

// orgSessionToken signs in a new user of the suite's organization unit who holds only permission, and returns
// their access token, which names the unit.
func (ts *SessionAPITestSuite) orgSessionToken(signInApps *testutils.OUSignInApps, rsID, identifier, username,
	permission string) string {
	userID := ts.createUser(username)
	roleID, err := testutils.CreateRole(testutils.Role{
		Name: "SessionAPI_" + username,
		OUID: testOUID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: rsID, Permissions: []string{permission}},
		},
		Assignments: []testutils.Assignment{{ID: userID, Type: "user"}},
	})
	ts.Require().NoError(err, "create the %s role", permission)
	ts.T().Cleanup(func() { ts.logCleanup("role", roleID, testutils.DeleteRole) })

	token, err := signInApps.ObtainAccessToken(testOUID, permission, username, testPassword, identifier)
	ts.Require().NoError(err, "obtain the %s token", permission)
	ts.Require().Equal(testOUID, ts.tokenClaim(token.AccessToken, "ouId"), "the token names the suite's unit")
	return token.AccessToken
}

// A user revokes one of their own sessions. Another user's session, or one that is already gone, is left as it
// is with a 204 alike, so session IDs can't be probed.
func (ts *SessionAPITestSuite) TestUserRevokesOwnSession() {
	ts.createUser("session_api_revoke_self_user")
	ts.createUser("session_api_revoke_self_other")
	current := ts.signIn(ts.newSessionClient(), "session_api_revoke_self_user", "openid", "revoke_self_1")
	stolen := ts.signIn(ts.newSessionClient(), "session_api_revoke_self_user", "openid", "revoke_self_2")
	someoneElse := ts.signIn(ts.newSessionClient(), "session_api_revoke_self_other", "openid", "revoke_self_3")
	stolenID := ts.currentSessionID(stolen.AccessToken)
	otherUsersID := ts.currentSessionID(someoneElse.AccessToken)
	self := testutils.GetHTTPClientWithToken(current.AccessToken)

	status, raw := ts.deleteSessions(self, "/users/me/sessions/"+url.PathEscape(otherUsersID))
	ts.Equal(http.StatusNoContent, status, "another user's session looks like a missing one: %s", raw)
	ts.True(ts.introspectActive(someoneElse.AccessToken), "the other user's session is untouched")

	status, raw = ts.deleteSessions(self, "/users/me/sessions/"+url.PathEscape(stolenID))
	ts.Require().Equal(http.StatusNoContent, status, raw)
	ts.False(ts.introspectActive(stolen.AccessToken), "the revoked session's token is no longer active")

	status, raw = ts.deleteSessions(self, "/users/me/sessions/"+url.PathEscape(stolenID))
	ts.Equal(http.StatusNoContent, status, "revoking a session that is already gone still succeeds: %s", raw)
	ts.True(ts.introspectActive(current.AccessToken), "the caller's own session is untouched")
}

// A user signs out everywhere else and keeps the session they are using.
func (ts *SessionAPITestSuite) TestUserRevokesOtherSessions() {
	ts.createUser("session_api_revoke_rest_user")
	current := ts.signIn(ts.newSessionClient(), "session_api_revoke_rest_user", "openid", "revoke_rest_1")
	other := ts.signIn(ts.newSessionClient(), "session_api_revoke_rest_user", "openid", "revoke_rest_2")
	currentID := ts.currentSessionID(current.AccessToken)

	status, raw := ts.deleteSessions(testutils.GetHTTPClientWithToken(current.AccessToken),
		"/users/me/sessions?exceptCurrent=true")
	ts.Require().Equal(http.StatusNoContent, status, raw)

	ts.True(ts.introspectActive(current.AccessToken), "the current session is kept")
	ts.False(ts.introspectActive(other.AccessToken), "the other session is revoked")
	list := ts.listOwnSessions(current.AccessToken)
	ts.Require().Len(list.Sessions, 1)
	ts.Equal(currentID, list.Sessions[0].ID)

	status, raw = ts.deleteSessions(testutils.GetHTTPClientWithToken(current.AccessToken), "/users/me/sessions")
	ts.Require().Equal(http.StatusNoContent, status, raw)
	ts.False(ts.introspectActive(current.AccessToken), "without exceptCurrent every session goes")
}

// Revoking needs the right caller and valid parameters.
func (ts *SessionAPITestSuite) TestRevokeRejectsInvalidRequests() {
	userID := ts.createUser("session_api_revoke_invalid_user")
	tokens := ts.signIn(ts.newSessionClient(), "session_api_revoke_invalid_user", "openid", "revoke_invalid_1")
	user := testutils.GetHTTPClientWithToken(tokens.AccessToken)

	tests := []struct {
		name   string
		client *http.Client
		path   string
		want   int
	}{
		{"regular user revokes a session", user, "/sessions/any", http.StatusForbidden},
		{"regular user revokes a user's sessions", user, "/sessions?userId=" + url.QueryEscape(userID),
			http.StatusForbidden},
		{"no userId", testutils.GetHTTPClient(), "/sessions", http.StatusBadRequest},
		{"invalid exceptCurrent", user, "/users/me/sessions?exceptCurrent=maybe", http.StatusBadRequest},
		{"no token on the admin path", testutils.GetRawHTTPClient(), "/sessions/any", http.StatusUnauthorized},
		{"no token on the self path", testutils.GetRawHTTPClient(), "/users/me/sessions", http.StatusUnauthorized},
	}
	for _, tc := range tests {
		ts.Run(tc.name, func() {
			status, raw := ts.deleteSessions(tc.client, tc.path)
			ts.Equal(tc.want, status, raw)
		})
	}
	ts.True(ts.introspectActive(tokens.AccessToken), "rejected requests revoke nothing")
}
