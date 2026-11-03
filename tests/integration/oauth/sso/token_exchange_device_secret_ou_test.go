// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sso

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	ouExchangeClientID     = "sso_exchange_ou_client"
	ouExchangeChildHandle  = "sso-exchange-child"
	ouExchangeSharedUser   = "sso_exchange_shared_user"
	ouExchangeUnsharedUser = "sso_exchange_unshared_user"
	ouExchangeChildRole    = "SSO Exchange Child Role"
	ouExchangeParentRole   = "SSO Exchange Parent Role"
	accessTokenType        = "urn:ietf:params:oauth:token-type:access_token"
	ouScopedGrantRefusal   = "The grant type is not supported on the organization unit scoped token endpoint"
)

// TestTokenExchange_DeviceSecretSignsInToAChildOU verifies the back-channel SSO into an organization unit: the
// device secret the suite's app got in the parent, exchanged at /ou/{child}/oauth2/token by an app shared with
// the child, signs a user shared into the child in to it, and asks a user who is not to sign in.
func (ts *SSOLogoutTestSuite) TestTokenExchange_DeviceSecretSignsInToAChildOU() {
	childOUID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: ouExchangeChildHandle, Name: ouExchangeChildHandle, Parent: &testOUID,
	})
	ts.Require().NoError(err, "create the child OU")
	defer ts.deleteOUExchangeFixture("OU", childOUID, testutils.DeleteOrganizationUnit)

	sharedUserID := ts.createOUExchangeUser(ouExchangeSharedUser)
	defer ts.deleteOUExchangeFixture("user", sharedUserID, testutils.DeleteUser)
	unsharedUserID := ts.createOUExchangeUser(ouExchangeUnsharedUser)
	defer ts.deleteOUExchangeFixture("user", unsharedUserID, testutils.DeleteUser)
	ts.shareUserInto(sharedUserID, childOUID)

	childRoleID := ts.createOUExchangeRole(ouExchangeChildRole, childOUID, sharedUserID)
	defer ts.deleteOUExchangeFixture("role", childRoleID, testutils.DeleteRole)
	parentRoleID := ts.createOUExchangeRole(ouExchangeParentRole, testOUID, sharedUserID)
	defer ts.deleteOUExchangeFixture("role", parentRoleID, testutils.DeleteRole)

	appID := ts.createOUTokenExchangeApp()
	defer ts.deleteAppByID(appID)
	ts.Require().NoError(testutils.ShareApplicationWithChildUnits(appID), "share the exchanging app")
	childTokenPath := "/ou/" + childOUID + "/oauth2/token"

	ts.Run("a user shared into the child is signed in to it", func() {
		first := ts.loginDeviceSecretAs(ouExchangeSharedUser, "ou_exchange_shared_state")

		status, token := ts.exchangeDeviceSecretAt(childTokenPath, ouExchangeClientID, clientSecret,
			first.DeviceSecret, "openid")

		ts.Require().Equal(http.StatusOK, status, "the exchange should succeed: %v", token)
		accessToken, _ := token["access_token"].(string)
		claims, err := testutils.DecodeJWT(accessToken)
		ts.Require().NoError(err, "decode the access token")
		ts.Equal(sharedUserID, claims.Sub)
		ts.Equal(childOUID, claims.OUID, "the token names the OU in the path")
		ts.Equal([]interface{}{ouExchangeChildRole}, claims.Additional["roles"],
			"only the role assigned in the child is listed")
	})

	ts.Run("a refresh at the plain token endpoint keeps the child and its roles", func() {
		first := ts.loginDeviceSecretAs(ouExchangeSharedUser, "ou_exchange_refresh_state")
		status, token := ts.exchangeDeviceSecretAt(childTokenPath, ouExchangeClientID, clientSecret,
			first.DeviceSecret, "openid")
		ts.Require().Equal(http.StatusOK, status, "the exchange should succeed: %v", token)
		refreshToken, _ := token["refresh_token"].(string)
		ts.Require().NotEmpty(refreshToken, "the exchange issues a refresh token")

		refreshed, err := testutils.RefreshAccessTokenRaw(ouExchangeClientID, clientSecret, refreshToken, "")

		ts.Require().NoError(err)
		ts.Require().Equal(http.StatusOK, refreshed.StatusCode, "the refresh should succeed: %s",
			string(refreshed.Body))
		claims, err := testutils.DecodeJWT(refreshed.Token.AccessToken)
		ts.Require().NoError(err, "decode the refreshed access token")
		ts.Equal(sharedUserID, claims.Sub)
		ts.Equal(childOUID, claims.OUID, "the refreshed token still names the child")
		ts.Equal([]interface{}{ouExchangeChildRole}, claims.Additional["roles"],
			"the refreshed token still lists only the role assigned in the child")
	})

	ts.Run("a user not in the child must sign in", func() {
		first := ts.loginDeviceSecretAs(ouExchangeUnsharedUser, "ou_exchange_unshared_state")

		status, token := ts.exchangeDeviceSecretAt(childTokenPath, ouExchangeClientID, clientSecret,
			first.DeviceSecret, "openid")

		ts.Equal(http.StatusBadRequest, status)
		ts.Equal("interaction_required", token["error"], "the session cannot be used in the child: %v", token)
		ts.NotContains(token, "access_token")
	})

	ts.Run("another token exchange is refused on the OU endpoint", func() {
		first := ts.loginDeviceSecretAs(ouExchangeSharedUser, "ou_exchange_other_state")

		status, token := ts.requestTokenExchange(childTokenPath, ouExchangeClientID, clientSecret,
			first.AccessToken, accessTokenType, "")

		ts.Equal(http.StatusBadRequest, status)
		ts.Equal("invalid_request", token["error"])
		ts.Equal(ouScopedGrantRefusal, token["error_description"])
	})
}

// loginDeviceSecretAs signs the user in to the suite's app, requesting device_sso, and returns its tokens.
func (ts *SSOLogoutTestSuite) loginDeviceSecretAs(username, state string) *testutils.TokenResponse {
	client := ts.newSessionClient()
	authID, executionID := ts.authorize(client, deviceSSOScope, state)
	step, collected := ts.runCredentialLogin(client, executionID, username)
	token := ts.completeAndExchange(client, authID, step.Assertion, collected)
	ts.Require().NotEmpty(token.DeviceSecret, "the sign-in should return a device secret")
	return token
}

// createOUTokenExchangeApp creates an app on the suite's login flow that holds the token exchange and refresh
// token grants and whose access tokens carry the sign-in's OU and roles.
func (ts *SSOLogoutTestSuite) createOUTokenExchangeApp() string {
	appID, err := testutils.CreateApplication(testutils.Application{
		Name:             "SSOExchangeOUApp",
		Description:      "Application that signs in to an OU by exchanging an SSO session",
		OUID:             testOUID,
		AuthFlowID:       ts.authFlowID,
		AllowedUserTypes: []string{testUserType.Handle},
		InboundAuthConfig: []map[string]interface{}{{"type": "oauth2", "config": map[string]interface{}{
			"clientId":                ouExchangeClientID,
			"clientSecret":            clientSecret,
			"redirectUris":            []string{redirectURI},
			"grantTypes":              []string{"authorization_code", tokenExchangeGrant, "refresh_token"},
			"responseTypes":           []string{"code"},
			"scopes":                  []string{"openid"},
			"tokenEndpointAuthMethod": "client_secret_basic",
			"token": map[string]interface{}{
				"accessToken": map[string]interface{}{
					"userConfig": map[string]interface{}{"attributes": []string{"ouId", "roles"}},
				},
			},
		}}},
	})
	ts.Require().NoError(err, "Failed to create the OU token exchange application")
	return appID
}

// createOUExchangeUser creates a user living in the suite's OU and returns its ID.
func (ts *SSOLogoutTestSuite) createOUExchangeUser(username string) string {
	userID, err := testutils.CreateUser(testutils.User{
		OUID: testOUID,
		Type: testUserType.Handle,
		Attributes: json.RawMessage(fmt.Sprintf(`{"username": %q, "password": %q, "email": %q}`,
			username, testPassword, username+"@example.com")),
	})
	ts.Require().NoError(err, "Failed to create user %s", username)
	return userID
}

// shareUserInto shares the user into the named organization units.
func (ts *SSOLogoutTestSuite) shareUserInto(userID string, ouIDs ...string) {
	units := make([]map[string]string, 0, len(ouIDs))
	for _, ouID := range ouIDs {
		units = append(units, map[string]string{"ouId": ouID, "reach": "unit"})
	}
	ts.postJSON("/users/"+userID+"/sharing-policies",
		map[string]interface{}{"childUnits": map[string]interface{}{"mode": "selected", "units": units}})
}

// postJSON posts a body as the admin and requires 201 Created.
func (ts *SSOLogoutTestSuite) postJSON(path string, payload interface{}) {
	body, err := json.Marshal(payload)
	ts.Require().NoError(err)
	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+path, bytes.NewReader(body))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	ts.Require().Equal(http.StatusCreated, resp.StatusCode, "POST %s: %s", path, string(raw))
}

// createOUExchangeRole creates a role without permissions in ouID, assigned to the user.
func (ts *SSOLogoutTestSuite) createOUExchangeRole(name, ouID, userID string) string {
	roleID, err := testutils.CreateRole(testutils.Role{
		Name:        name,
		OUID:        ouID,
		Assignments: []testutils.Assignment{{ID: userID, Type: "user"}},
	})
	ts.Require().NoError(err, "Failed to create role %s", name)
	return roleID
}

// deleteOUExchangeFixture deletes one fixture of the OU exchange test, logging a failure.
func (ts *SSOLogoutTestSuite) deleteOUExchangeFixture(kind, id string, deleteFn func(string) error) {
	if err := deleteFn(id); err != nil {
		ts.T().Logf("Failed to delete %s %s: %v", kind, id, err)
	}
}
