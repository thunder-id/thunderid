// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package session holds integration tests for the session management API: listing the live sessions of a user
// or an application as an administrator, and listing your own.
package session

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	clientID     = "session_api_test_client"
	clientSecret = "session_api_test_secret" //nolint:gosec // test credential
	appName      = "SessionAPITestApp"
	redirectURI  = "https://localhost:3000"
	// postLogoutRedirectURI must be registered on the client for the RP-initiated logout redirect to be honoured.
	postLogoutRedirectURI = "https://localhost:3000/logged-out"
	testPassword          = "testpass123"

	// joinClientID is a further application on the suite's flow, which joins a session over SSO.
	joinClientID     = "session_api_join_client"
	joinClientSecret = "session_api_join_secret" //nolint:gosec // test credential

	// ouClientID is an application shared into child organization units, which joins a session under one of
	// them by exchanging a device secret.
	ouClientID = "session_api_ou_client"
	// ouDirectClientID is an application on a flow without a session step, shared into a child unit.
	ouDirectClientID = "session_api_ou_direct_client"

	// directClientID is an application on a flow without a session step.
	directClientID = "session_api_direct_client"
	// refreshClientID is an application on a flow without a session step that refreshes its tokens.
	refreshClientID = "session_api_refresh_client"
	// nativeClientID is an app-native application on a flow without a session step.
	nativeClientID = "session_api_native_client"

	// ouRevokeClientID is an application shared into a child organization unit, for the revocation tests.
	ouRevokeClientID = "session_api_ou_revoke_client"

	// delegateClientID is an application that exchanges a user's token for a delegated one.
	delegateClientID = "session_api_delegate_client"

	deviceSSOScope     = "openid device_sso"
	tokenExchangeGrant = "urn:ietf:params:oauth:grant-type:token-exchange"
	jwtTokenType       = "urn:ietf:params:oauth:token-type:jwt"
	accessTokenType    = "urn:ietf:params:oauth:token-type:access_token"
)

var (
	testOUID string

	testUserType = testutils.UserType{
		Handle:           "session-api-person",
		DisplayName:      "Session API Person",
		SystemAttributes: &testutils.UserTypeSystemAttributes{Display: "username"},
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
			"email":    map[string]interface{}{"type": "string"},
		},
	}

	testOU = testutils.OrganizationUnit{
		Handle:      "session-api-test-ou",
		Name:        "Session API Test OU",
		Description: "Organization unit for session management API integration testing",
	}

	// credentialInputs are the inputs of the credential prompt and of the executor that checks them.
	credentialInputs = []map[string]interface{}{
		{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
		{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
	}

	// ssoAuthFlow signs in with credentials and records an SSO session, which a later sign-in in the same
	// browser reuses without a prompt.
	ssoAuthFlow = testutils.Flow{
		Name:     "Session API Test Auth Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_session_api_test",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "sso_check"},
			{
				"id":         "sso_check",
				"type":       "TASK_EXECUTION",
				"executor":   map[string]interface{}{"name": "SSOCheckExecutor"},
				"properties": map[string]interface{}{"checkpointRef": "session_main"},
				"onSuccess":  "session_main",
				"onFailure":  "prompt_credentials",
			},
			{
				"id":   "prompt_credentials",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{{
					"inputs": credentialInputs,
					"action": map[string]interface{}{"ref": "action_001", "nextNode": "credentials_auth"},
				}},
			},
			{
				"id":   "credentials_auth",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name":   "CredentialsAuthExecutor",
					"inputs": credentialInputs,
				},
				"onSuccess":    "session_main",
				"onIncomplete": "prompt_credentials",
			},
			{
				"id":        "session_main",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "SessionExecutor"},
				"onSuccess": "authorization_check",
			},
			{
				"id":        "authorization_check",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "AuthorizationExecutor"},
				"onSuccess": "auth_assert",
			},
			{
				"id":        "auth_assert",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "AuthAssertExecutor"},
				"onSuccess": "end",
			},
			{"id": "end", "type": "END"},
		},
	}

	// directAuthFlow signs a user in without a session step, so each sign-in records a direct session.
	directAuthFlow = testutils.Flow{
		Name:     "Session API Test Direct Sign-In Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "direct_auth_flow_session_api_test",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "prompt_credentials"},
			{
				"id":   "prompt_credentials",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{{
					"inputs": credentialInputs,
					"action": map[string]interface{}{"ref": "action_001", "nextNode": "credentials_auth"},
				}},
			},
			{
				"id":   "credentials_auth",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name":   "CredentialsAuthExecutor",
					"inputs": credentialInputs,
				},
				"onSuccess":    "authorization_check",
				"onIncomplete": "prompt_credentials",
			},
			{
				"id":        "authorization_check",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "AuthorizationExecutor"},
				"onSuccess": "auth_assert",
			},
			{
				"id":        "auth_assert",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "AuthAssertExecutor"},
				"onSuccess": "end",
			},
			{"id": "end", "type": "END"},
		},
	}

	// signOutFlow ends the SSO session the sign-in flow recorded.
	signOutFlow = testutils.Flow{
		Name:     "Session API Test Sign-Out Flow",
		FlowType: "SIGNOUT",
		Handle:   "signout_flow_session_api_test",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "session_signout"},
			{
				"id":        "session_signout",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "SessionSignOutExecutor"},
				"onSuccess": "end",
			},
			{"id": "end", "type": "END"},
		},
	}
)

// SessionAPITestSuite exercises the session management API against the live server.
type SessionAPITestSuite struct {
	suite.Suite
	userTypeID    string
	authFlowID    string
	directFlowID  string
	signOutFlowID string
	applicationID string
	userIDs       []string
	// childOUIDs are deleted after the users, which can be shared into them, and before the suite's own unit.
	childOUIDs []string
}

func TestSessionAPITestSuite(t *testing.T) {
	suite.Run(t, new(SessionAPITestSuite))
}

func (ts *SessionAPITestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(testOU)
	ts.Require().NoError(err, "Failed to create the test organization unit")
	testOUID = ouID

	testUserType.OUID = ouID
	ts.userTypeID, err = testutils.CreateUserType(testUserType)
	ts.Require().NoError(err, "Failed to create the test user type")

	ts.authFlowID, err = testutils.CreateFlow(ssoAuthFlow)
	ts.Require().NoError(err, "Failed to create the SSO sign-in flow")
	ts.directFlowID, err = testutils.CreateFlow(directAuthFlow)
	ts.Require().NoError(err, "Failed to create the sign-in flow without a session step")
	ts.signOutFlowID, err = testutils.CreateFlow(signOutFlow)
	ts.Require().NoError(err, "Failed to create the sign-out flow")

	ts.applicationID = ts.createApplication(appName, clientID, clientSecret, ts.authFlowID,
		[]string{"authorization_code", "refresh_token"})
}

func (ts *SessionAPITestSuite) TearDownSuite() {
	if ts.applicationID != "" {
		ts.logCleanup("application", ts.applicationID, testutils.DeleteApplication)
	}
	for _, userID := range ts.userIDs {
		ts.logCleanup("user", userID, testutils.DeleteUser)
	}
	for _, ouID := range ts.childOUIDs {
		ts.logCleanup("organization unit", ouID, testutils.DeleteOrganizationUnit)
	}
	for kind, id := range map[string]string{
		"sign-in flow": ts.authFlowID, "direct sign-in flow": ts.directFlowID, "sign-out flow": ts.signOutFlowID,
	} {
		if id != "" {
			ts.logCleanup(kind, id, testutils.DeleteFlow)
		}
	}
	if ts.userTypeID != "" {
		ts.logCleanup("user type", ts.userTypeID, testutils.DeleteUserType)
	}
	if testOUID != "" {
		ts.logCleanup("organization unit", testOUID, testutils.DeleteOrganizationUnit)
	}
}

// An administrator can list a user's live sessions, with the device, the IP and the participating
// application recorded at sign-in, and the response carries no credential.
func (ts *SessionAPITestSuite) TestAdminListsUserSessions() {
	userID := ts.createUser("session_api_admin_user")
	ts.signIn(ts.newSessionClient(), "session_api_admin_user", "openid", "admin_1")

	var list testutils.SessionListView
	status, raw := ts.getSessions(testutils.GetHTTPClient(), "/sessions?userId="+url.QueryEscape(userID), &list)

	ts.Require().Equal(http.StatusOK, status, raw)
	ts.Require().Equal(1, list.TotalResults, "one sign-in makes one session")
	ts.Require().Len(list.Sessions, 1)
	got := list.Sessions[0]
	ts.Equal("SSO", got.Type, "a sign-in through the session step records an SSO session")
	ts.Equal(userID, got.UserID)
	ts.Equal("session_api_admin_user", got.UserDisplayName, "the user's display attribute is resolved")
	ts.NotEmpty(got.UserAgent, "the sign-in's User-Agent is recorded")
	ts.NotEmpty(got.CreationIP, "the IP the session was created from is recorded")
	ts.NotEmpty(got.LastActiveIP, "the sign-in's IP is recorded")
	ts.Nil(got.IsCurrent, "admin listings don't mark a current session")
	ts.Require().Len(got.Participants, 1)
	ts.Equal(ts.applicationID, got.Participants[0].AppID)
	ts.Equal(appName, got.Participants[0].AppName)
	for _, secret := range []string{"handle", "tfid", "tokenFamily", "context"} {
		ts.NotContains(strings.ToLower(raw), strings.ToLower(secret), "the response must not carry %s", secret)
	}
}

// A second application that signs in over SSO joins the existing session rather than starting one, so the
// session is listed once with both applications, in the order they joined, and appears under either one.
func (ts *SessionAPITestSuite) TestSSOJoinListsBothApplications() {
	userID := ts.createUser("session_api_join_user")
	joinAppID := ts.createApplication("SessionAPIJoinApp", joinClientID, joinClientSecret, ts.authFlowID,
		[]string{"authorization_code", "refresh_token"})
	defer ts.logCleanup("application", joinAppID, testutils.DeleteApplication)

	client := ts.newSessionClient()
	ts.signIn(client, "session_api_join_user", "openid", "join_1")
	ts.Require().True(ts.ssoSatisfies(client, joinClientID, "join_2"),
		"the second application must be signed in over SSO for this test to mean anything")

	var list testutils.SessionListView
	status, raw := ts.getSessions(testutils.GetHTTPClient(), "/sessions?userId="+url.QueryEscape(userID), &list)

	ts.Require().Equal(http.StatusOK, status, raw)
	ts.Require().Equal(1, list.TotalResults, "an SSO sign-in joins the session instead of starting one")
	got := list.Sessions[0]
	ts.Require().Len(got.Participants, 2)
	ts.Equal(ts.applicationID, got.Participants[0].AppID, "the application that started the session is first")
	ts.Equal(joinAppID, got.Participants[1].AppID)
	ts.Equal("SessionAPIJoinApp", got.Participants[1].AppName)

	var byApp testutils.SessionListView
	status, raw = ts.getSessions(testutils.GetHTTPClient(), "/sessions?appId="+url.QueryEscape(joinAppID), &byApp)

	ts.Require().Equal(http.StatusOK, status, raw)
	ts.Require().Len(byApp.Sessions, 1)
	ts.Equal(got.ID, byApp.Sessions[0].ID, "the joining application lists the same session")
}

// Listing by application finds the sessions the application has joined, and paging splits them with the
// filter carried into the links.
func (ts *SessionAPITestSuite) TestListByApplicationPages() {
	ts.createUser("session_api_app_user")
	ts.signIn(ts.newSessionClient(), "session_api_app_user", "openid", "app_1")
	ts.signIn(ts.newSessionClient(), "session_api_app_user", "openid", "app_2")

	var list testutils.SessionListView
	status, raw := ts.getSessions(testutils.GetHTTPClient(),
		"/sessions?appId="+url.QueryEscape(ts.applicationID)+"&limit=1", &list)

	ts.Require().Equal(http.StatusOK, status, raw)
	ts.GreaterOrEqual(list.TotalResults, 2, "both sign-ins joined the application")
	ts.Equal(1, list.Count, "limit caps the page")
	ts.Require().Len(list.Sessions, 1)
	ts.Require().Len(list.Sessions[0].Participants, 1)
	ts.Equal(ts.applicationID, list.Sessions[0].Participants[0].AppID)
	var next string
	for _, l := range list.Links {
		if l.Rel == "next" {
			next = l.Href
		}
	}
	ts.Contains(next, "appId="+url.QueryEscape(ts.applicationID), "the next link keeps the filter")
}

// A signed-in user lists only their own sessions with GET /users/me/sessions, and the session that issued the
// access token is marked as current.
func (ts *SessionAPITestSuite) TestSelfListMarksCurrentSession() {
	userID := ts.createUser("session_api_self_user")
	ts.signIn(ts.newSessionClient(), "session_api_self_user", "openid", "self_1")
	tokens := ts.signIn(ts.newSessionClient(), "session_api_self_user", "openid", "self_2")

	var list testutils.SessionListView
	status, raw := ts.getSessions(testutils.GetHTTPClientWithToken(tokens.AccessToken),
		"/users/me/sessions?userId=someone-else", &list)

	ts.Require().Equal(http.StatusOK, status, raw)
	ts.Require().Equal(2, list.TotalResults, "each browser holds its own session")
	current := 0
	for _, s := range list.Sessions {
		ts.Equal(userID, s.UserID, "a userId parameter can't widen /users/me/sessions")
		ts.Empty(s.UserDisplayName, "own sessions don't resolve the user's name")
		ts.Require().NotNil(s.IsCurrent, "own sessions always say whether they are current")
		if *s.IsCurrent {
			current++
		}
	}
	ts.Equal(1, current, "exactly the session behind the token is current")
}

// An authorization code sign-in through a flow without a session step records a direct session, which is listed
// with the user's SSO session, each with its type. The code grant's token finds its own session, so the user's list,
// which holds both sessions, marks the direct one as current.
func (ts *SessionAPITestSuite) TestDirectSessionIsListed() {
	const username = "session_api_direct_user"
	userID := ts.createUser(username)
	ssoTokens := ts.signIn(ts.newSessionClient(), username, "openid", "direct_1")
	ssoID := ts.tokenClaim(ssoTokens.IDToken, "sid")
	ts.Require().NotEmpty(ssoID, "the SSO sign-in's ID token names its session")
	directAppID := ts.createApplication("SessionAPIDirectApp", directClientID, clientSecret, ts.directFlowID,
		[]string{"authorization_code"})
	defer ts.logCleanup("application", directAppID, testutils.DeleteApplication)

	tokens := ts.signInAs(ts.newSessionClient(), directClientID, clientSecret, username, "openid", "direct_2")
	ts.Require().NotEmpty(ts.tokenClaim(tokens.AccessToken, "tfid"), "the token must carry a token family")

	directID := ts.assertDirectSessionListed(directAppID, userID, tokens.AccessToken)
	for path, client := range map[string]*http.Client{
		"/sessions?userId=" + url.QueryEscape(userID): testutils.GetHTTPClient(),
		"/users/me/sessions":                          testutils.GetHTTPClientWithToken(tokens.AccessToken),
	} {
		var list testutils.SessionListView
		status, raw := ts.getSessions(client, path, &list)
		ts.Require().Equal(http.StatusOK, status, raw)
		ts.Require().Len(list.Sessions, 2, "%s lists two sessions: %s", path, raw)
		typeByID := map[string]string{}
		for _, s := range list.Sessions {
			ts.Equal(userID, s.UserID, "%s lists only the user's sessions", path)
			typeByID[s.ID] = s.Type
		}
		ts.Equal(map[string]string{ssoID: "SSO", directID: "DIRECT"}, typeByID,
			"%s lists the SSO and the direct session, each with its type: %s", path, raw)
	}
}

// A refresh keeps the grant in its direct session: the session is still listed once, and the refreshed access
// token finds it as the user's current session.
func (ts *SessionAPITestSuite) TestDirectSessionIsListedAfterRefresh() {
	const username = "session_api_refresh_user"
	userID := ts.createUser(username)
	appID := ts.createApplication("SessionAPIRefreshApp", refreshClientID, clientSecret, ts.directFlowID,
		[]string{"authorization_code", "refresh_token"})
	defer ts.logCleanup("application", appID, testutils.DeleteApplication)
	tokens := ts.signInAs(ts.newSessionClient(), refreshClientID, clientSecret, username, "openid", "refresh_1")
	ts.Require().NotEmpty(tokens.RefreshToken, "the sign-in should issue a refresh token")
	before := ts.assertDirectSessionListed(appID, userID, tokens.AccessToken)

	refreshed := ts.refresh(refreshClientID, clientSecret, tokens.RefreshToken)

	ts.Equal(before, ts.assertDirectSessionListed(appID, userID, refreshed.AccessToken),
		"the refreshed grant stays in the same session")
}

// An app-native sign-in through a flow without a session step records a direct session, and the access token the
// application gets for its assertion finds that session as the user's current one.
func (ts *SessionAPITestSuite) TestAppNativeDirectSessionIsListed() {
	const username = "session_api_native_user"
	userID := ts.createUser(username)
	appID := ts.createAppNativeApplication("SessionAPINativeApp", nativeClientID, clientSecret, ts.directFlowID)
	defer ts.logCleanup("application", appID, testutils.DeleteApplication)

	tokens := ts.signInAppNative(appID, nativeClientID, clientSecret, username)

	ts.assertDirectSessionListed(appID, userID, tokens.AccessToken)
}

// A session that has been signed out no longer appears in either listing.
func (ts *SessionAPITestSuite) TestSignedOutSessionIsNotListed() {
	userID := ts.createUser("session_api_signout_user")
	client := ts.newSessionClient()
	signedOut := ts.signIn(client, "session_api_signout_user", "openid", "signout_1")
	kept := ts.signIn(ts.newSessionClient(), "session_api_signout_user", "openid", "signout_2")
	path := "/sessions?userId=" + url.QueryEscape(userID)

	var before testutils.SessionListView
	status, raw := ts.getSessions(testutils.GetHTTPClient(), path, &before)
	ts.Require().Equal(http.StatusOK, status, raw)
	ts.Require().Equal(2, before.TotalResults)

	ts.signOut(client, signedOut.IDToken, "signout_3")

	var after testutils.SessionListView
	status, raw = ts.getSessions(testutils.GetHTTPClient(), path, &after)
	ts.Require().Equal(http.StatusOK, status, raw)
	ts.Equal(1, after.TotalResults, "the signed-out session is gone from the admin list")

	var mine testutils.SessionListView
	status, raw = ts.getSessions(testutils.GetHTTPClientWithToken(kept.AccessToken), "/users/me/sessions", &mine)
	ts.Require().Equal(http.StatusOK, status, raw)
	ts.Equal(1, mine.TotalResults, "the signed-out session is gone from the user's own list")
}

// Listing another user's sessions needs org:session:view or the system permission, which a regular user lacks.
func (ts *SessionAPITestSuite) TestRegularUserCannotListOthers() {
	userID := ts.createUser("session_api_forbidden_user")
	tokens := ts.signIn(ts.newSessionClient(), "session_api_forbidden_user", "openid", "forbidden_1")

	status, _ := ts.getSessions(testutils.GetHTTPClientWithToken(tokens.AccessToken),
		"/sessions?userId="+url.QueryEscape(userID), nil)

	ts.Equal(http.StatusForbidden, status)
}

// An application shared into a child organization unit joins the user's session under the parent over SSO and
// under the child, as an organization switch does. An administrator with org:session:view in the child sees the
// session with only the child's participant, and the system permission sees every participant, with the
// application listed once by application. The user's token in each unit sees only the participants of the unit
// its own sign-in was recorded under, which the server reads from the participant holding the token's family,
// not from the token's claims. Another user's direct sign-in under the child is listed for the child's
// administrator by application too. Revoking the session from the child ends it in both units.
func (ts *SessionAPITestSuite) TestOrgAdminSeesOnlyItsOU() {
	const (
		listedUser = "session_api_ou_user"
		directUser = "session_api_ou_direct_user"
		orgAdmin   = "session_api_ou_admin"
		identifier = "https://session-api-ou.example.com/system"
	)
	childOUID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "session-api-child", Name: "session-api-child", Parent: &testOUID,
	})
	ts.Require().NoError(err, "create the child OU")
	ts.childOUIDs = append(ts.childOUIDs, childOUID)

	userID := ts.createUser(listedUser)
	ts.shareUserInto(userID, childOUID)
	appID := ts.createApplication("SessionAPIOUApp", ouClientID, clientSecret, ts.authFlowID,
		[]string{"authorization_code", tokenExchangeGrant})
	defer ts.logCleanup("application", appID, testutils.DeleteApplication)
	ts.Require().NoError(testutils.ShareApplicationWithChildUnits(appID), "share the app into the child")

	client := ts.newSessionClient()
	first := ts.signIn(client, listedUser, deviceSSOScope, "ou_1")
	ts.Require().True(ts.ssoSatisfies(client, ouClientID, "ou_2"), "the app joins the session under the parent")
	childToken := ts.exchangeDeviceSecret("/ou/"+childOUID+"/oauth2/token", ouClientID, clientSecret,
		first.DeviceSecret)

	directUserID := ts.createUser(directUser)
	ts.shareUserInto(directUserID, childOUID)
	directAppID := ts.createApplication("SessionAPIOUDirectApp", ouDirectClientID, clientSecret, ts.directFlowID,
		[]string{"authorization_code"})
	defer ts.logCleanup("application", directAppID, testutils.DeleteApplication)
	ts.Require().NoError(testutils.ShareApplicationWithChildUnits(directAppID), "share the direct app into the child")
	ts.signInUnder(ts.newSessionClient(), childOUID, ouDirectClientID, directUser, "ou_direct")

	adminID := ts.createUser(orgAdmin)
	ts.shareUserInto(adminID, childOUID)
	rsID, err := testutils.CreateOrgScopedResourceServer(childOUID, "Session API RS", identifier, "session")
	ts.Require().NoError(err, "create the child's resource server")
	defer ts.logCleanup("resource server", rsID, testutils.DeleteResourceServerWithChildren)
	roleID, err := testutils.CreateRole(testutils.Role{
		Name: "SessionAPI_OUSessionViewer",
		OUID: childOUID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: rsID, Permissions: []string{"org:session:view"}},
		},
		Assignments: []testutils.Assignment{{ID: adminID, Type: "user"}},
	})
	ts.Require().NoError(err, "create the child's session viewer role")
	defer ts.logCleanup("role", roleID, testutils.DeleteRole)
	var signInApps testutils.OUSignInApps
	defer func() { ts.NoError(signInApps.DeleteAll()) }()
	token, err := signInApps.ObtainAccessToken(childOUID, "org:session:view", orgAdmin, testPassword, identifier)
	ts.Require().NoError(err, "obtain the child administrator's token")
	ts.Require().Equal(childOUID, ts.tokenClaim(token.AccessToken, "ouId"), "the token names the child")
	orgClient := testutils.GetHTTPClientWithToken(token.AccessToken)

	var byUser testutils.SessionListView
	status, raw := ts.getSessions(orgClient, "/sessions?userId="+url.QueryEscape(userID), &byUser)
	ts.Require().Equal(http.StatusOK, status, raw)
	ts.Require().Equal(1, byUser.TotalResults, raw)
	ts.Require().Len(byUser.Sessions[0].Participants, 1, "only the child's participant is shown")
	ts.Equal(appID, byUser.Sessions[0].Participants[0].AppID)
	ts.Equal(childOUID, byUser.Sessions[0].Participants[0].OUID)

	var bySuiteApp testutils.SessionListView
	status, raw = ts.getSessions(orgClient, "/sessions?appId="+url.QueryEscape(ts.applicationID), &bySuiteApp)
	ts.Require().Equal(http.StatusOK, status, raw)
	ts.Zero(bySuiteApp.TotalResults, "the suite's app signed in under the parent only")

	var byApp testutils.SessionListView
	status, raw = ts.getSessions(orgClient, "/sessions?appId="+url.QueryEscape(appID), &byApp)
	ts.Require().Equal(http.StatusOK, status, raw)
	ts.Require().Equal(1, byApp.TotalResults, raw)
	ts.Require().Len(byApp.Sessions[0].Participants, 1, "only the app's participant in the child is shown")
	ts.Equal(childOUID, byApp.Sessions[0].Participants[0].OUID)

	var byDirectApp testutils.SessionListView
	status, raw = ts.getSessions(orgClient, "/sessions?appId="+url.QueryEscape(directAppID), &byDirectApp)
	ts.Require().Equal(http.StatusOK, status, raw)
	ts.Require().Equal(1, byDirectApp.TotalResults, raw)
	ts.Require().Len(byDirectApp.Sessions, 1, raw)
	ts.Equal("DIRECT", byDirectApp.Sessions[0].Type, "the direct sign-in under the child is listed")
	ts.Equal(directUserID, byDirectApp.Sessions[0].UserID)
	ts.Require().Len(byDirectApp.Sessions[0].Participants, 1)
	ts.Equal(childOUID, byDirectApp.Sessions[0].Participants[0].OUID)

	var all testutils.SessionListView
	status, raw = ts.getSessions(testutils.GetHTTPClient(), "/sessions?userId="+url.QueryEscape(userID), &all)
	ts.Require().Equal(http.StatusOK, status, raw)
	ts.Require().Equal(1, all.TotalResults, raw)
	ts.Len(all.Sessions[0].Participants, 3, "the system administrator sees the participants of both units")

	recordedOU := ts.participantOUs(all.Sessions[0].ID)
	ts.Require().Equal(childOUID, recordedOU[ts.tokenClaim(childToken, "tfid")],
		"the exchange is recorded under the child")
	parentOUID := recordedOU[ts.tokenClaim(first.AccessToken, "tfid")]
	ts.Require().NotEqual(childOUID, parentOUID, "the first sign-in is recorded under the parent")

	var allByApp testutils.SessionListView
	status, raw = ts.getSessions(testutils.GetHTTPClient(), "/sessions?appId="+url.QueryEscape(appID), &allByApp)
	ts.Require().Equal(http.StatusOK, status, raw)
	ts.Require().Equal(1, allByApp.TotalResults, "an app signed in under two units is one session: %s", raw)
	ts.Require().Len(allByApp.Sessions, 1, raw)
	appOUs := make([]string, 0, 2)
	for _, p := range allByApp.Sessions[0].Participants {
		if p.AppID == appID {
			appOUs = append(appOUs, p.OUID)
		}
	}
	ts.ElementsMatch([]string{parentOUID, childOUID}, appOUs, "the app's participant in each unit is shown")

	for _, tc := range []struct {
		name, token, ouID string
		participants      int
	}{
		{"parent", first.AccessToken, parentOUID, 2},
		{"child", childToken, childOUID, 1},
	} {
		var own testutils.SessionListView
		status, raw = ts.getSessions(testutils.GetHTTPClientWithToken(tc.token), "/users/me/sessions", &own)
		ts.Require().Equal(http.StatusOK, status, raw)
		ts.Require().Equal(1, own.TotalResults, raw)
		ts.Require().Len(own.Sessions[0].Participants, tc.participants, "the user's %s token sees only its unit",
			tc.name)
		for _, p := range own.Sessions[0].Participants {
			ts.Equal(tc.ouID, p.OUID, tc.name)
		}
		ts.Require().NotNil(own.Sessions[0].IsCurrent)
		ts.True(*own.Sessions[0].IsCurrent, "the %s token's session is the current one", tc.name)
	}
	// Revoking from one unit ends the whole session, signing the user out of the other unit too.
	status, raw = ts.deleteSessions(testutils.GetHTTPClientWithToken(childToken),
		"/users/me/sessions/"+url.PathEscape(all.Sessions[0].ID))
	ts.Require().Equal(http.StatusNoContent, status, raw)
	var after testutils.SessionListView
	status, raw = ts.getSessions(testutils.GetHTTPClient(), "/sessions?userId="+url.QueryEscape(userID), &after)
	ts.Require().Equal(http.StatusOK, status, raw)
	ts.Zero(after.TotalResults, raw)
}

// Both listings need an access token.
func (ts *SessionAPITestSuite) TestRequiresAuthentication() {
	for _, path := range []string{"/sessions?userId=any", "/users/me/sessions"} {
		status, _ := ts.getSessions(testutils.GetRawHTTPClient(), path, nil)
		ts.Equal(http.StatusUnauthorized, status, path)
	}
}

// The admin list needs exactly one filter.
func (ts *SessionAPITestSuite) TestListNeedsOneFilter() {
	for _, path := range []string{"/sessions", "/sessions?userId=a&appId=b"} {
		status, _ := ts.getSessions(testutils.GetHTTPClient(), path, nil)
		ts.Equal(http.StatusBadRequest, status, path)
	}
}

// Both listings reject a limit outside 1 to 100 or a negative offset, and accept a valid page.
func (ts *SessionAPITestSuite) TestValidatesPagination() {
	for path, wantCode := range map[string]string{
		"/sessions?userId=any&limit=0":   "SSM-1002",
		"/sessions?userId=any&limit=101": "SSM-1002",
		"/sessions?userId=any&limit=x":   "SSM-1002",
		"/sessions?userId=any&offset=-1": "SSM-1003",
		"/sessions?userId=any&offset=x":  "SSM-1003",
		"/users/me/sessions?limit=0":     "SSM-1002",
	} {
		status, raw := ts.getSessions(testutils.GetHTTPClient(), path, nil)
		ts.Equal(http.StatusBadRequest, status, path)
		ts.Contains(raw, wantCode, path)
	}

	var page testutils.SessionListView
	status, raw := ts.getSessions(testutils.GetHTTPClient(), "/sessions?userId=any&limit=5&offset=10", &page)
	ts.Require().Equal(http.StatusOK, status, raw)
	ts.Zero(page.Count)
}
