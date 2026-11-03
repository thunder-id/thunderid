// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sso

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	tokenExchangeGrant  = "urn:ietf:params:oauth:grant-type:token-exchange"
	jwtTokenType        = "urn:ietf:params:oauth:token-type:jwt"
	exchangeAppClientID = "sso_exchange_app_client"
	publicAppClientID   = "sso_exchange_public_client"
	otherFlowClientID   = "sso_exchange_other_flow_client"
)

// TestTokenExchange_DeviceSecretSignsInAnotherApp verifies the back-channel SSO: the device secret one app
// got from its code exchange, exchanged by another app on the same login flow, signs the user in to that
// app silently, with access, ID and refresh tokens that belong to the same session.
func (ts *SSOLogoutTestSuite) TestTokenExchange_DeviceSecretSignsInAnotherApp() {
	appID := ts.createTokenExchangeApp(exchangeAppClientID, ts.authFlowID, false)
	defer ts.deleteAppByID(appID)
	first, _ := ts.loginDeviceSecret(ts.newSessionClient(), deviceSSOScope, "exchange_state", true)

	status, token := ts.exchangeDeviceSecret(exchangeAppClientID, clientSecret, first.DeviceSecret, "openid")

	ts.Require().Equal(http.StatusOK, status, "the exchange should succeed: %v", token)
	ts.NotEmpty(token["access_token"], "an access token is issued")
	ts.NotEmpty(token["refresh_token"], "a refresh token is issued when the app allows that grant")
	idToken, _ := token["id_token"].(string)
	ts.Require().NotEmpty(idToken, "an ID token is issued for openid")
	ts.Equal(ts.idTokenClaim(first.IDToken, "sid"), ts.idTokenClaim(idToken, "sid"),
		"the second app joins the same session")
	ts.Equal(exchangeAppClientID, ts.idTokenAudience(idToken), "the ID token is for the exchanging app")
}

// TestTokenExchange_NoIDTokenWithoutOpenID verifies that an exchange that does not request openid signs the
// user in to the app with an access token but returns no ID token.
func (ts *SSOLogoutTestSuite) TestTokenExchange_NoIDTokenWithoutOpenID() {
	appID := ts.createTokenExchangeApp(exchangeAppClientID, ts.authFlowID, false)
	defer ts.deleteAppByID(appID)
	first, _ := ts.loginDeviceSecret(ts.newSessionClient(), deviceSSOScope, "exchange_no_openid_state", true)

	status, token := ts.exchangeDeviceSecret(exchangeAppClientID, clientSecret, first.DeviceSecret, "")

	ts.Require().Equal(http.StatusOK, status, "the exchange should succeed: %v", token)
	ts.NotEmpty(token["access_token"], "an access token is issued")
	ts.NotContains(token, "id_token", "no ID token is issued without openid")
}

// TestTokenExchange_DeviceSecretPublicClient verifies that a public client may exchange the device secret too.
func (ts *SSOLogoutTestSuite) TestTokenExchange_DeviceSecretPublicClient() {
	appID := ts.createTokenExchangeApp(publicAppClientID, ts.authFlowID, true)
	defer ts.deleteAppByID(appID)
	first, _ := ts.loginDeviceSecret(ts.newSessionClient(), deviceSSOScope, "exchange_public_state", true)

	status, token := ts.exchangeDeviceSecret(publicAppClientID, "", first.DeviceSecret, "openid")

	ts.Require().Equal(http.StatusOK, status, "the exchange should succeed: %v", token)
	ts.NotEmpty(token["access_token"])
}

// TestTokenExchange_DeviceSecretOfAnotherFlowNeedsInteraction verifies that an app whose login flow is not
// the session's cannot use it: its flow would need the user, so interaction_required is returned.
func (ts *SSOLogoutTestSuite) TestTokenExchange_DeviceSecretOfAnotherFlowNeedsInteraction() {
	appID := ts.createTokenExchangeApp(otherFlowClientID, ts.promptStepFlowID, false)
	defer ts.deleteAppByID(appID)
	first, _ := ts.loginDeviceSecret(ts.newSessionClient(), deviceSSOScope, "exchange_other_flow_state", true)

	status, token := ts.exchangeDeviceSecret(otherFlowClientID, clientSecret, first.DeviceSecret, "openid")

	ts.Equal(http.StatusBadRequest, status)
	ts.Equal("interaction_required", token["error"])
}

// TestTokenExchange_EndedSessionNeedsInteraction verifies that once the session is signed out, its
// device secret no longer signs anyone in.
func (ts *SSOLogoutTestSuite) TestTokenExchange_EndedSessionNeedsInteraction() {
	appID := ts.createTokenExchangeApp(exchangeAppClientID, ts.authFlowID, false)
	defer ts.deleteAppByID(appID)
	nativeAppID := ts.createAppNativeApp()
	defer ts.deleteAppByID(nativeAppID)
	client := ts.newSessionClient()
	first, _ := ts.loginDeviceSecret(client, deviceSSOScope, "exchange_ended_state", true)

	cookies := ts.ssoCookies(client)
	ts.Require().Len(cookies, 1)
	var held testutils.FlowSessionData
	for name, handle := range cookies {
		held = testutils.FlowSessionData{
			FlowIdentifier: strings.TrimPrefix(name, testutils.SSOCookiePrefix), SessionHandle: handle,
		}
	}
	signOut := ts.appNativeExecute(ts.newSessionClient(), nativeAppID, map[string]interface{}{
		"applicationId": nativeAppID, "flowType": "SIGNOUT", "sessionData": []testutils.FlowSessionData{held},
	})
	ts.Require().Equal("COMPLETE", signOut.FlowStatus, "sign-out should complete")

	status, token := ts.exchangeDeviceSecret(exchangeAppClientID, clientSecret, first.DeviceSecret, "openid")

	ts.Equal(http.StatusBadRequest, status)
	ts.Equal("interaction_required", token["error"])
}

// TestTokenExchange_PicksTheSessionOfTheAppsFlow verifies that of the sessions a device secret carries,
// the exchange uses the one for the exchanging app's login flow.
func (ts *SSOLogoutTestSuite) TestTokenExchange_PicksTheSessionOfTheAppsFlow() {
	appID := ts.createTokenExchangeApp(exchangeAppClientID, ts.authFlowID, false)
	defer ts.deleteAppByID(appID)
	client := ts.newSessionClient()
	authID, executionID := ts.authorize(client, deviceSSOScope, "exchange_pick_state")
	step, collected := ts.runCredentialLogin(client, executionID, ssoReuseUsername)
	first := ts.completeAndExchange(client, authID, step.Assertion,
		append([]testutils.FlowSessionData{unrelatedSession}, collected...))

	status, token := ts.exchangeDeviceSecret(exchangeAppClientID, clientSecret, first.DeviceSecret, "openid")

	ts.Require().Equal(http.StatusOK, status, "the exchange should succeed: %v", token)
	ts.NotEmpty(token["access_token"])
}

// TestTokenExchange_SessionOfAnotherUserRefused verifies the device secret cannot sign someone in as
// another user: a callback that posts another user's session in place of the signed-in user's gets a
// device secret whose exchange is refused, since the session the flow runs on is not its subject's.
func (ts *SSOLogoutTestSuite) TestTokenExchange_SessionOfAnotherUserRefused() {
	appID := ts.createTokenExchangeApp(exchangeAppClientID, ts.authFlowID, false)
	defer ts.deleteAppByID(appID)
	otherClient := ts.newSessionClient()
	_, otherExecutionID := ts.authorize(otherClient, deviceSSOScope, "exchange_other_user_state")
	_, otherUsersSessions := ts.runCredentialLogin(otherClient, otherExecutionID, logoutUsername)
	client := ts.newSessionClient()
	authID, executionID := ts.authorize(client, deviceSSOScope, "exchange_swapped_state")
	step, _ := ts.runCredentialLogin(client, executionID, ssoReuseUsername)
	swapped := ts.completeAndExchange(client, authID, step.Assertion, otherUsersSessions)

	status, token := ts.exchangeDeviceSecret(exchangeAppClientID, clientSecret, swapped.DeviceSecret, "openid")

	ts.Equal(http.StatusBadRequest, status)
	ts.Equal("invalid_grant", token["error"])
}

// createTokenExchangeApp creates an app on the given login flow that signs in with the authorization code
// grant and also holds the token exchange and refresh token grants, as a confidential or a public client.
// The application rules require the authorization code grant of a public client, and of any client given
// the refresh token grant.
func (ts *SSOLogoutTestSuite) createTokenExchangeApp(appClientID, flowID string, public bool) string {
	oauthConfig := map[string]interface{}{
		"clientId":      appClientID,
		"redirectUris":  []string{redirectURI},
		"grantTypes":    []string{"authorization_code", tokenExchangeGrant, "refresh_token"},
		"responseTypes": []string{"code"},
		"scopes":        []string{"openid"},
	}
	if public {
		oauthConfig["publicClient"] = true
		oauthConfig["pkceRequired"] = true
		oauthConfig["tokenEndpointAuthMethod"] = "none"
	} else {
		oauthConfig["clientSecret"] = clientSecret
		oauthConfig["tokenEndpointAuthMethod"] = "client_secret_basic"
	}
	appID, err := testutils.CreateApplication(testutils.Application{
		Name:              "SSOExchangeApp " + appClientID,
		Description:       "Application that signs in by exchanging an SSO session",
		OUID:              testOUID,
		AuthFlowID:        flowID,
		AllowedUserTypes:  []string{testUserType.Handle},
		InboundAuthConfig: []map[string]interface{}{{"type": "oauth2", "config": oauthConfig}},
	})
	ts.Require().NoError(err, "Failed to create the token exchange application")
	return appID
}

// exchangeDeviceSecret exchanges a device secret at the token endpoint as the given client, requesting the
// given scope (none when empty), and returns the status and the decoded response body.
func (ts *SSOLogoutTestSuite) exchangeDeviceSecret(appClientID, secret, deviceSecret, scope string) (
	int, map[string]interface{}) {
	ts.Require().NotEmpty(deviceSecret, "the first login should have returned a device secret")
	form := url.Values{}
	form.Set("grant_type", tokenExchangeGrant)
	form.Set("subject_token", deviceSecret)
	form.Set("subject_token_type", jwtTokenType)
	if scope != "" {
		form.Set("scope", scope)
	}
	if secret == "" {
		form.Set("client_id", appClientID)
	}
	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+"/oauth2/token",
		strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if secret != "" {
		req.SetBasicAuth(appClientID, secret)
	}

	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err, "token request failed")
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var body map[string]interface{}
	ts.Require().NoError(json.Unmarshal(raw, &body), "failed to decode the token response: %s", string(raw))
	return resp.StatusCode, body
}

// idTokenClaim returns a string claim of an ID token.
func (ts *SSOLogoutTestSuite) idTokenClaim(idToken, claim string) string {
	claims, err := testutils.DecodeJWT(idToken)
	ts.Require().NoError(err, "failed to decode the ID token")
	value, _ := claims.Additional[claim].(string)
	ts.Require().NotEmpty(value, "the ID token should carry %s", claim)
	return value
}

// idTokenAudience returns the audience of an ID token.
func (ts *SSOLogoutTestSuite) idTokenAudience(idToken string) string {
	claims, err := testutils.DecodeJWT(idToken)
	ts.Require().NoError(err, "failed to decode the ID token")
	return claims.Aud
}
