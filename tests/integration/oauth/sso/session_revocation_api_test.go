// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sso

import (
	"io"
	"net/http"
	"net/url"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// deleteSessions sends a DELETE to the session management API with the given client and returns the
// status code and the raw body.
func (ts *SSOLogoutTestSuite) deleteSessions(client *http.Client, path string) (int, string) {
	ts.T().Helper()

	req, err := http.NewRequest(http.MethodDelete, testutils.TestServerURL+path, nil)
	ts.Require().NoError(err)
	resp, err := client.Do(req)
	ts.Require().NoError(err, "session API request failed")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	return resp.StatusCode, string(body)
}

// listOwnSessions returns the caller's sessions from GET /users/me/sessions.
func (ts *SSOLogoutTestSuite) listOwnSessions(accessToken string) sessionListView {
	ts.T().Helper()

	var list sessionListView
	status, raw := ts.getSessions(testutils.GetHTTPClientWithToken(accessToken), "/users/me/sessions", &list)
	ts.Require().Equal(http.StatusOK, status, raw)
	return list
}

// currentSessionID returns the ID of the session that issued the access token.
func (ts *SSOLogoutTestSuite) currentSessionID(accessToken string) string {
	ts.T().Helper()

	for _, s := range ts.listOwnSessions(accessToken).Sessions {
		if s.IsCurrent != nil && *s.IsCurrent {
			return s.ID
		}
	}
	ts.FailNow("no session is marked as current")
	return ""
}

// An administrator revokes one session: its token stops working, the browser has to sign in again,
// and revoking it again still succeeds.
func (ts *SSOLogoutTestSuite) TestSessionAPI_AdminRevokesSession() {
	ts.createUser("sso_revoke_admin_one_user")
	client := ts.newSessionClient()
	tokens := ts.loginTokens(client, "sso_revoke_admin_one_user", "revoke_admin_one_1")
	other := ts.loginTokens(ts.newSessionClient(), "sso_revoke_admin_one_user", "revoke_admin_one_2")
	sessionID := ts.currentSessionID(tokens.AccessToken)

	status, raw := ts.deleteSessions(testutils.GetHTTPClient(), "/sessions/"+url.PathEscape(sessionID))
	ts.Require().Equal(http.StatusNoContent, status, raw)

	ts.False(ts.introspectActive(client, tokens.AccessToken), "the revoked session's token is no longer active")
	ts.True(ts.introspectActive(client, other.AccessToken), "the user's other session is untouched")
	ts.Equal(1, ts.listOwnSessions(other.AccessToken).TotalResults, "only the other session is left")
	// loginTokens asserts the flow asks for credentials, so the session no longer backs SSO.
	ts.loginTokens(client, "sso_revoke_admin_one_user", "revoke_admin_one_3")

	status, raw = ts.deleteSessions(testutils.GetHTTPClient(), "/sessions/"+url.PathEscape(sessionID))
	ts.Equal(http.StatusNoContent, status, "revoking a session that is already gone still succeeds: %s", raw)
}

// An administrator revokes all of a user's sessions.
func (ts *SSOLogoutTestSuite) TestSessionAPI_AdminRevokesUserSessions() {
	userID := ts.createUser("sso_revoke_admin_all_user")
	first := ts.loginTokens(ts.newSessionClient(), "sso_revoke_admin_all_user", "revoke_admin_all_1")
	second := ts.loginTokens(ts.newSessionClient(), "sso_revoke_admin_all_user", "revoke_admin_all_2")

	status, raw := ts.deleteSessions(testutils.GetHTTPClient(), "/sessions?userId="+url.QueryEscape(userID))
	ts.Require().Equal(http.StatusNoContent, status, raw)

	client := testutils.GetRawHTTPClient()
	ts.False(ts.introspectActive(client, first.AccessToken), "every session's token is revoked")
	ts.False(ts.introspectActive(client, second.AccessToken), "every session's token is revoked")
	var list sessionListView
	status, _ = ts.getSessions(testutils.GetHTTPClient(), "/sessions?userId="+url.QueryEscape(userID), &list)
	ts.Require().Equal(http.StatusOK, status)
	ts.Equal(0, list.TotalResults, "no session is left")

	// Revoking sessions is not a lockout: the user can sign in again and use the new token.
	again := ts.loginTokens(ts.newSessionClient(), "sso_revoke_admin_all_user", "revoke_admin_all_3")
	ts.True(ts.introspectActive(client, again.AccessToken), "a new sign-in issues a working token")
}

// A user revokes one of their own sessions, but gets a 404 for a session that isn't theirs.
func (ts *SSOLogoutTestSuite) TestSessionAPI_UserRevokesOwnSession() {
	ts.createUser("sso_revoke_self_one_user")
	ts.createUser("sso_revoke_self_one_other")
	current := ts.loginTokens(ts.newSessionClient(), "sso_revoke_self_one_user", "revoke_self_one_1")
	stolen := ts.loginTokens(ts.newSessionClient(), "sso_revoke_self_one_user", "revoke_self_one_2")
	someoneElse := ts.loginTokens(ts.newSessionClient(), "sso_revoke_self_one_other", "revoke_self_one_3")
	stolenID := ts.currentSessionID(stolen.AccessToken)
	otherUsersID := ts.currentSessionID(someoneElse.AccessToken)
	self := testutils.GetHTTPClientWithToken(current.AccessToken)

	status, raw := ts.deleteSessions(self, "/users/me/sessions/"+url.PathEscape(otherUsersID))
	ts.Equal(http.StatusNotFound, status, "another user's session looks like a missing one: %s", raw)
	ts.True(ts.introspectActive(testutils.GetRawHTTPClient(), someoneElse.AccessToken),
		"the other user's session is untouched")

	status, raw = ts.deleteSessions(self, "/users/me/sessions/"+url.PathEscape(stolenID))
	ts.Require().Equal(http.StatusNoContent, status, raw)
	ts.False(ts.introspectActive(testutils.GetRawHTTPClient(), stolen.AccessToken),
		"the revoked session's token is no longer active")

	status, _ = ts.deleteSessions(self, "/users/me/sessions/"+url.PathEscape(stolenID))
	ts.Equal(http.StatusNotFound, status, "a session that is gone is not found")
}

// A user signs out everywhere else and keeps the session they are using.
func (ts *SSOLogoutTestSuite) TestSessionAPI_UserRevokesOtherSessions() {
	ts.createUser("sso_revoke_self_rest_user")
	current := ts.loginTokens(ts.newSessionClient(), "sso_revoke_self_rest_user", "revoke_self_rest_1")
	other := ts.loginTokens(ts.newSessionClient(), "sso_revoke_self_rest_user", "revoke_self_rest_2")
	currentID := ts.currentSessionID(current.AccessToken)

	status, raw := ts.deleteSessions(testutils.GetHTTPClientWithToken(current.AccessToken),
		"/users/me/sessions?exceptCurrent=true")
	ts.Require().Equal(http.StatusNoContent, status, raw)

	client := testutils.GetRawHTTPClient()
	ts.True(ts.introspectActive(client, current.AccessToken), "the current session is kept")
	ts.False(ts.introspectActive(client, other.AccessToken), "the other session is revoked")
	list := ts.listOwnSessions(current.AccessToken)
	ts.Require().Len(list.Sessions, 1)
	ts.Equal(currentID, list.Sessions[0].ID)

	status, raw = ts.deleteSessions(testutils.GetHTTPClientWithToken(current.AccessToken), "/users/me/sessions")
	ts.Require().Equal(http.StatusNoContent, status, raw)
	ts.False(ts.introspectActive(client, current.AccessToken), "without exceptCurrent every session goes")
}

// Revoking needs the right caller and valid parameters.
func (ts *SSOLogoutTestSuite) TestSessionAPI_RevokeRejectsInvalidRequests() {
	userID := ts.createUser("sso_revoke_invalid_user")
	tokens := ts.loginTokens(ts.newSessionClient(), "sso_revoke_invalid_user", "revoke_invalid_1")
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
	ts.True(ts.introspectActive(testutils.GetRawHTTPClient(), tokens.AccessToken),
		"rejected requests revoke nothing")
}
