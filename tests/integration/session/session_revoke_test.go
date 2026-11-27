// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

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

// An administrator revokes a direct session, one recorded by a sign-in through a flow without a session step,
// by its ID and with the rest of the user's sessions.
func (ts *SessionAPITestSuite) TestAdminRevokesDirectSessions() {
	const username = "session_api_revoke_direct_admin_user"
	userID := ts.createUser(username)
	sso := ts.signIn(ts.newSessionClient(), username, "openid", "revoke_direct_admin_1")
	directAppID := ts.createApplication("SessionAPIDirectApp", directClientID, clientSecret, ts.directFlowID,
		[]string{"authorization_code"})
	defer ts.logCleanup("application", directAppID, testutils.DeleteApplication)
	direct := ts.signInAs(ts.newSessionClient(), directClientID, clientSecret, username, "openid",
		"revoke_direct_admin_2")
	directID := ts.assertDirectSessionListed(directAppID, userID, direct.AccessToken)

	status, raw := ts.deleteSessions(testutils.GetHTTPClient(), "/sessions/"+url.PathEscape(directID))
	ts.Require().Equal(http.StatusNoContent, status, raw)
	ts.False(ts.introspectActive(direct.AccessToken), "the revoked direct session's token is no longer active")
	ts.True(ts.introspectActive(sso.AccessToken), "the user's SSO session is untouched")

	direct = ts.signInAs(ts.newSessionClient(), directClientID, clientSecret, username, "openid",
		"revoke_direct_admin_3")
	status, raw = ts.deleteSessions(testutils.GetHTTPClient(), "/sessions?userId="+url.QueryEscape(userID))
	ts.Require().Equal(http.StatusNoContent, status, raw)
	ts.False(ts.introspectActive(direct.AccessToken), "the direct session is revoked with the user's sessions")
	ts.False(ts.introspectActive(sso.AccessToken), "the SSO session is revoked with the user's sessions")
}

// org:session:delete revokes another user's sessions under the organization unit the token names, and
// org:session:view alone does not.
func (ts *SessionAPITestSuite) TestSessionDeletePermissionRevokesOthers() {
	const identifier = "https://session-api-revoke.example.com/system"
	userID := ts.createUser("session_api_revoke_narrow_user")
	tokens := ts.signIn(ts.newSessionClient(), "session_api_revoke_narrow_user", "openid", "revoke_narrow_1")
	sessionID := ts.currentSessionID(tokens.AccessToken)

	rsID := ts.createSessionResourceServer(testOUID, identifier)
	var signInApps testutils.OUSignInApps
	defer func() { ts.NoError(signInApps.DeleteAll()) }()
	viewer := ts.orgSessionToken(&signInApps, testOUID, rsID, identifier, "session_api_revoke_viewer",
		"org:session:view")
	revoker := ts.orgSessionToken(&signInApps, testOUID, rsID, identifier, "session_api_revoke_revoker",
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

// Revoking is confined to the caller's organization unit. A child administrator, or the user's token from the
// child, leaves alone a session with no application signed in under the child, and ends whole a session that has
// one, which signs the user out of the parent unit too.
func (ts *SessionAPITestSuite) TestRevokeIsConfinedToTheCallersUnit() {
	const (
		username   = "session_api_revoke_ou_user"
		identifier = "https://session-api-revoke-ou.example.com/system"
	)
	childOUID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "session-api-revoke-child", Name: "session-api-revoke-child", Parent: &testOUID,
	})
	ts.Require().NoError(err, "create the child OU")
	ts.childOUIDs = append(ts.childOUIDs, childOUID)

	userID := ts.createUser(username)
	ts.shareUserInto(userID, childOUID)
	appID := ts.createApplication("SessionAPIOURevokeApp", ouRevokeClientID, clientSecret, ts.authFlowID,
		[]string{"authorization_code", tokenExchangeGrant})
	defer ts.logCleanup("application", appID, testutils.DeleteApplication)
	ts.Require().NoError(testutils.ShareApplicationWithChildUnits(appID), "share the app into the child")

	// signInToChild signs the user in under the parent and joins the same session from the child, returning
	// the parent's and the child's access tokens.
	signInToChild := func(state string) (string, string) {
		client := ts.newSessionClient()
		first := ts.signIn(client, username, deviceSSOScope, state+"_1")
		ts.Require().True(ts.ssoSatisfies(client, ouRevokeClientID, state+"_2"), "the app joins under the parent")
		return first.AccessToken, ts.exchangeDeviceSecret("/ou/"+childOUID+"/oauth2/token", ouRevokeClientID,
			clientSecret, first.DeviceSecret)
	}
	parentOnly := ts.signIn(ts.newSessionClient(), username, "openid", "revoke_ou_parent")
	parentOnlyID := ts.currentSessionID(parentOnly.AccessToken)

	rsID := ts.createSessionResourceServer(childOUID, identifier)
	var signInApps testutils.OUSignInApps
	defer func() { ts.NoError(signInApps.DeleteAll()) }()
	childAdmin := testutils.GetHTTPClientWithToken(ts.orgSessionToken(&signInApps, childOUID, rsID, identifier,
		"session_api_revoke_ou_admin", "org:session:delete"))

	ts.Run("administrator", func() {
		parentToken, _ := signInToChild("revoke_ou_admin")

		status, raw := ts.deleteSessions(childAdmin, "/sessions/"+url.PathEscape(parentOnlyID))
		ts.Require().Equal(http.StatusNoContent, status, raw)
		ts.True(ts.introspectActive(parentOnly.AccessToken), "a session out of the unit is left alone")

		status, raw = ts.deleteSessions(childAdmin, "/sessions?userId="+url.QueryEscape(userID))
		ts.Require().Equal(http.StatusNoContent, status, raw)
		ts.True(ts.introspectActive(parentOnly.AccessToken), "a session out of the unit is left alone")
		ts.False(ts.introspectActive(parentToken), "the session with the child's app ends whole")
	})
	ts.Run("self-service", func() {
		parentToken, childToken := signInToChild("revoke_ou_self")
		self := testutils.GetHTTPClientWithToken(childToken)

		status, raw := ts.deleteSessions(self, "/users/me/sessions/"+url.PathEscape(parentOnlyID))
		ts.Require().Equal(http.StatusNoContent, status, raw)
		ts.True(ts.introspectActive(parentOnly.AccessToken), "a session the child's list doesn't show is left alone")

		status, raw = ts.deleteSessions(self, "/users/me/sessions")
		ts.Require().Equal(http.StatusNoContent, status, raw)
		ts.True(ts.introspectActive(parentOnly.AccessToken), "a session the child's list doesn't show is left alone")
		ts.False(ts.introspectActive(parentToken), "the session with the child's app ends whole")
	})
}

// exceptCurrent=true with a token no session holds is refused with SSM-1006, so a caller who asked to stay
// signed in is never signed out. The token's participant row is removed so it still works but holds no session.
func (ts *SessionAPITestSuite) TestExceptCurrentNeedsTheCurrentSession() {
	const username = "session_api_revoke_unknown_user"
	ts.createUser(username)
	orphan := ts.signIn(ts.newSessionClient(), username, "openid", "revoke_unknown_1")
	other := ts.signIn(ts.newSessionClient(), username, "openid", "revoke_unknown_2")
	_, err := testutils.QueryRuntimeDB(fmt.Sprintf(`DELETE FROM "SESSION_PARTICIPANT" WHERE TFID = '%s';`,
		ts.tokenClaim(orphan.AccessToken, "tfid")))
	ts.Require().NoError(err, "detach the token from its session")

	status, raw := ts.deleteSessions(testutils.GetHTTPClientWithToken(orphan.AccessToken),
		"/users/me/sessions?exceptCurrent=true")

	ts.Equal(http.StatusBadRequest, status, raw)
	ts.Contains(raw, "SSM-1006")
	ts.True(ts.introspectActive(other.AccessToken), "the user's other session is untouched")
}

// The self-service revocations refuse a delegated token, one issued to a client acting for the user, with
// SSM-1007.
func (ts *SessionAPITestSuite) TestSelfRevokeRefusesADelegatedToken() {
	ts.createUser("session_api_revoke_delegate_user")
	ts.createUser("session_api_revoke_delegate_actor")
	subject := ts.signIn(ts.newSessionClient(), "session_api_revoke_delegate_user", "openid", "revoke_delegate_1")
	actor := ts.signIn(ts.newSessionClient(), "session_api_revoke_delegate_actor", "openid", "revoke_delegate_2")
	appID := ts.createApplication("SessionAPIDelegateApp", delegateClientID, clientSecret, ts.authFlowID,
		[]string{"authorization_code", tokenExchangeGrant})
	defer ts.logCleanup("application", appID, testutils.DeleteApplication)

	form := url.Values{}
	form.Set("grant_type", tokenExchangeGrant)
	form.Set("subject_token", subject.AccessToken)
	form.Set("subject_token_type", accessTokenType)
	form.Set("actor_token", actor.AccessToken)
	form.Set("actor_token_type", accessTokenType)
	status, raw := ts.tokenRequest(testutils.GetRawHTTPClient(), "/oauth2/token", delegateClientID, clientSecret,
		form)
	ts.Require().Equal(http.StatusOK, status, raw)
	var delegated testutils.TokenResponse
	ts.Require().NoError(json.Unmarshal([]byte(raw), &delegated))
	claims, err := testutils.DecodeJWT(delegated.AccessToken)
	ts.Require().NoError(err)
	ts.Require().NotNil(claims.Additional["act"], "the exchanged token is delegated")

	client := testutils.GetHTTPClientWithToken(delegated.AccessToken)
	sessionID := ts.currentSessionID(subject.AccessToken)
	for _, path := range []string{"/users/me/sessions/" + url.PathEscape(sessionID), "/users/me/sessions"} {
		status, raw = ts.deleteSessions(client, path)
		ts.Equal(http.StatusForbidden, status, "%s: %s", path, raw)
		ts.Contains(raw, "SSM-1007", path)
	}
	ts.True(ts.introspectActive(subject.AccessToken), "a refused request revokes nothing")
}
