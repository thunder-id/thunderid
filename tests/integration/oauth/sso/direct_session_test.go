// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sso

import (
	"fmt"
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// directSessionApp is an application on an authentication flow without a Session node, so every login through
// it records a direct session.
type directSessionApp struct {
	id           string
	clientID     string
	clientSecret string
}

// createDirectSessionApp creates an application, optionally notified through back-channel logout, on its own
// authentication flow without a Session node. Both are removed when the test ends.
func (ts *SSOLogoutTestSuite) createDirectSessionApp(name, logoutURI string, extraGrants ...string) directSessionApp {
	ts.T().Helper()

	flow := noSessionAuthFlow
	flow.Name = "SSO Direct Session Flow " + name
	flow.Handle = "auth_flow_sso_direct_" + name
	flowID, err := testutils.CreateFlow(flow)
	ts.Require().NoError(err, "failed to create the direct session flow")
	ts.T().Cleanup(func() {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete the direct session flow: %v", err)
		}
	})

	app := directSessionApp{clientID: "sso_direct_" + name + "_client", clientSecret: "sso_direct_" + name + "_secret"}
	config := map[string]interface{}{
		"clientId":                app.clientID,
		"clientSecret":            app.clientSecret,
		"redirectUris":            []string{redirectURI},
		"grantTypes":              append([]string{"authorization_code"}, extraGrants...),
		"responseTypes":           []string{"code"},
		"tokenEndpointAuthMethod": "client_secret_basic",
		"scopes":                  []string{"openid"},
	}
	if logoutURI != "" {
		config["backchannelLogoutUri"] = logoutURI
	}
	appID, err := testutils.CreateApplication(testutils.Application{
		Name:              "SSODirectSession_" + name,
		Description:       "Application on an authentication flow without a Session node",
		OUID:              testOUID,
		Type:              "fullstack",
		AuthFlowID:        flowID,
		AllowedUserTypes:  []string{testUserType.Handle},
		InboundAuthConfig: []map[string]interface{}{{"type": "oauth2", "config": config}},
	})
	ts.Require().NoError(err, "failed to create the direct session application")
	ts.T().Cleanup(func() {
		if err := testutils.DeleteApplication(appID); err != nil {
			ts.T().Logf("Failed to delete the direct session application: %v", err)
		}
	})
	app.id = appID
	return app
}

// TestDirectSession_UserDeletionEndsItAndNotifiesTheApp verifies deleting the user ends their direct session like
// any other, and the application is told through back-channel logout with the session's sid.
func (ts *SSOLogoutTestSuite) TestDirectSession_UserDeletionEndsItAndNotifiesTheApp() {
	username := "sso_direct_deletion_user"
	userID := ts.createUser(username)
	receiver := ts.newLogoutReceiver(http.StatusOK)
	app := ts.createDirectSessionApp("deletion", receiver.srv.URL+"/bcl")
	client := ts.newSessionClient()
	tokens := ts.loginTokensAsClient(client, app.clientID, app.clientSecret, username, "direct_deletion_1")

	ts.Require().NoError(testutils.DeleteUser(userID), "failed to delete the user")

	got := receiver.await(ts, 1)
	ts.assertLogoutToken(got[0], app.clientID, tokens.IDToken)
}

// TestDirectSession_ClientCredentialsRecordsNone verifies a client credentials token records no session: no flow
// runs for it, so it carries no sid or tfid.
func (ts *SSOLogoutTestSuite) TestDirectSession_ClientCredentialsRecordsNone() {
	app := ts.createDirectSessionApp("client_credentials", "", "client_credentials")

	result, err := testutils.RequestClientCredentialsToken(app.clientID, app.clientSecret, "", "")
	ts.Require().NoError(err, "failed to request the token")
	ts.Require().Equal(http.StatusOK, result.StatusCode, "the token request failed: %s", result.Body)
	claims, err := testutils.DecodeJWTPayloadMap(result.Token.AccessToken)
	ts.Require().NoError(err, "failed to decode the token")
	ts.NotContains(claims, "sid", "the token must carry no sid")
	ts.NotContains(claims, "tfid", "the token must carry no tfid")

	sessions, err := testutils.QueryRuntimeDB(fmt.Sprintf(
		`SELECT COUNT(*) FROM "SESSION" WHERE SUBJECT_ID = '%s';`, app.id))
	ts.Require().NoError(err, "failed to count the app's sessions")
	ts.Equal("0", sessions, "no session may name the app as its subject")
	participants, err := testutils.QueryRuntimeDB(fmt.Sprintf(
		`SELECT COUNT(*) FROM "SESSION_PARTICIPANT" WHERE APP_ID = '%s';`, app.id))
	ts.Require().NoError(err, "failed to count the app's participants")
	ts.Equal("0", participants, "the app must join no session")
}
