// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package sso

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	lastUsedFlowHandle = "auth_flow_sso_last_used_ou"
	lastUsedUsername   = "sso_last_used_user"
	lastUsedC1Handle   = "sso-last-used-c1"
	lastUsedC2Handle   = "sso-last-used-c2"
	lastUsedC3Handle   = "sso-last-used-c3"
	lastUsedA3ClientID = "sso_last_used_a3"
	lastUsedA1ClientID = "sso_last_used_a1"
	lastUsedA2ClientID = "sso_last_used_a2"
	lastUsedBClientID  = "sso_last_used_b"
)

// lastUsedOUFixture holds the organization units of the last-used OU tests:
//
//	suite OU   ← owns app B, shared with c1 only, and the user
//	├── c1     ← owns app A1; the user is shared in
//	└── c2     ← owns app A2; the user is shared in
//
// Every app signs in with one flow: Check SSO, credentials, Resolve OU (prompt after sign-in), Save/Load Session and
// the assertion. A1 and A2 are not shared, so their sign-ins are fixed to their own OU.
type lastUsedOUFixture struct {
	c1     string
	c2     string
	flowID string
}

// lastUsedOUFlow returns the sign-in flow every app of the fixture uses, with Resolve OU between the sign-in step
// and Save/Load Session. The resolver picks among the user's OUs at or below parentOUID.
func lastUsedOUFlow(parentOUID string) testutils.Flow {
	credentialInputs := []map[string]interface{}{
		{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
		{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
	}
	return testutils.Flow{
		Name:     "SSO Last Used OU Flow",
		FlowType: "AUTHENTICATION",
		Handle:   lastUsedFlowHandle,
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
				"prompts": []map[string]interface{}{
					{
						"inputs": credentialInputs,
						"action": map[string]interface{}{"ref": "action_001", "nextNode": "credentials_auth"},
					},
				},
			},
			{
				"id":           "credentials_auth",
				"type":         "TASK_EXECUTION",
				"executor":     map[string]interface{}{"name": "CredentialsAuthExecutor", "inputs": credentialInputs},
				"onSuccess":    "resolve_ou",
				"onIncomplete": "prompt_credentials",
			},
			{
				"id":           "resolve_ou",
				"type":         "TASK_EXECUTION",
				"executor":     map[string]interface{}{"name": "OUResolverExecutor"},
				"properties":   map[string]interface{}{"resolveFrom": "prompt", "parentOuId": parentOUID},
				"onSuccess":    "session_main",
				"onIncomplete": "prompt_ou",
			},
			{
				"id":   "prompt_ou",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{"ref": "input_ou", "identifier": "ouId", "type": "SELECT", "required": true},
						},
						"action": map[string]interface{}{"ref": "action_ou", "nextNode": "resolve_ou"},
					},
				},
			},
			{
				"id":        "session_main",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "SessionExecutor"},
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
}

// newLastUsedOUFixture creates the fixture and deletes it when the test ends.
func (ts *SSOLogoutTestSuite) newLastUsedOUFixture() *lastUsedOUFixture {
	ts.T().Helper()
	f := &lastUsedOUFixture{c1: ts.createLastUsedOU(lastUsedC1Handle), c2: ts.createLastUsedOU(lastUsedC2Handle)}

	flowID, err := testutils.CreateFlow(lastUsedOUFlow(testOUID))
	ts.Require().NoError(err, "create the last-used OU flow")
	ts.T().Cleanup(func() { ts.deleteOUExchangeFixture("flow", flowID, testutils.DeleteFlow) })
	f.flowID = flowID

	userID := ts.createOUExchangeUser(lastUsedUsername)
	ts.T().Cleanup(func() { ts.deleteOUExchangeFixture("user", userID, testutils.DeleteUser) })
	ts.shareUserInto(userID, f.c1, f.c2)

	ts.createLastUsedApp(lastUsedA1ClientID, f.c1, flowID)
	ts.createLastUsedApp(lastUsedA2ClientID, f.c2, flowID)
	appB := ts.createLastUsedApp(lastUsedBClientID, testOUID, flowID)
	ts.shareAppWith(appB, f.c1)
	return f
}

// createLastUsedOU creates a child of the suite's OU, deleted when the test ends.
func (ts *SSOLogoutTestSuite) createLastUsedOU(handle string) string {
	ts.T().Helper()
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: handle, Name: handle, Parent: &testOUID,
	})
	ts.Require().NoError(err, "create OU %s", handle)
	ts.T().Cleanup(func() { ts.deleteOUExchangeFixture("OU", ouID, testutils.DeleteOrganizationUnit) })
	return ouID
}

// createLastUsedApp creates an app in ouID on the given flow whose access tokens carry the sign-in's OU, deleted
// when the test ends.
func (ts *SSOLogoutTestSuite) createLastUsedApp(appClientID, ouID, flowID string) string {
	ts.T().Helper()
	appID, err := testutils.CreateApplication(testutils.Application{
		Name:             "SSOLastUsedOU " + appClientID,
		OUID:             ouID,
		AuthFlowID:       flowID,
		AllowedUserTypes: []string{testUserType.Handle},
		InboundAuthConfig: []map[string]interface{}{{"type": "oauth2", "config": map[string]interface{}{
			"clientId":                appClientID,
			"clientSecret":            clientSecret,
			"redirectUris":            []string{redirectURI},
			"grantTypes":              []string{"authorization_code", tokenExchangeGrant, "refresh_token"},
			"responseTypes":           []string{"code"},
			"scopes":                  []string{"openid"},
			"tokenEndpointAuthMethod": "client_secret_basic",
			"token": map[string]interface{}{
				"accessToken": map[string]interface{}{
					"userConfig": map[string]interface{}{"attributes": []string{"ouId"}},
				},
			},
		}}},
	})
	ts.Require().NoError(err, "create application %s", appClientID)
	ts.T().Cleanup(func() { ts.deleteAppByID(appID) })
	return appID
}

// shareAppWith shares an application with the named child units only.
func (ts *SSOLogoutTestSuite) shareAppWith(appID string, ouIDs ...string) {
	ts.T().Helper()
	units := make([]map[string]string, 0, len(ouIDs))
	for _, ouID := range ouIDs {
		units = append(units, map[string]string{"ouId": ouID, "reach": "unit"})
	}
	ts.postJSON("/applications/"+appID+"/sharing-policies",
		map[string]interface{}{"childUnits": map[string]interface{}{"mode": "selected", "units": units}})
}

// lastUsedAuthorize sends an authorization request as the client, to the OU path when ouID is set.
func (ts *SSOLogoutTestSuite) lastUsedAuthorize(client *http.Client, ouID, appClientID, scope, state string,
	extra map[string]string) *http.Response {
	ts.T().Helper()
	params := url.Values{}
	params.Set("client_id", appClientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	params.Set("scope", scope)
	params.Set("state", state)
	for k, v := range extra {
		params.Set(k, v)
	}
	path := "/oauth2/authorize"
	if ouID != "" {
		path = "/ou/" + ouID + path
	}
	req, err := http.NewRequest(http.MethodGet, testutils.TestServerURL+path+"?"+params.Encode(), nil)
	ts.Require().NoError(err)
	resp, err := client.Do(req)
	ts.Require().NoError(err, "authorize request failed")
	return resp
}

// lastUsedStart authorizes as the client and returns the authId and executionId the gate redirect carries.
func (ts *SSOLogoutTestSuite) lastUsedStart(client *http.Client, ouID, appClientID, scope, state string) (
	string, string) {
	ts.T().Helper()
	resp := ts.lastUsedAuthorize(client, ouID, appClientID, scope, state, nil)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusFound, resp.StatusCode, "authorize should redirect to the gate")
	authID, executionID, err := testutils.ExtractAuthData(resp.Header.Get("Location"))
	ts.Require().NoError(err, "the gate redirect carries the flow: %s", resp.Header.Get("Location"))
	return authID, executionID
}

// lastUsedSignIn signs the fixture's user in to the client with credentials, posting the sessions the flow
// started, and returns the tokens.
func (ts *SSOLogoutTestSuite) lastUsedSignIn(client *http.Client, appClientID, scope,
	state string) *testutils.TokenResponse {
	ts.T().Helper()
	authID, executionID := ts.lastUsedStart(client, "", appClientID, scope, state)
	step, collected := ts.runCredentialLogin(client, executionID, lastUsedUsername)
	return ts.lastUsedRedeem(client, authID, step.Assertion, collected, appClientID)
}

// lastUsedJoin opens the client through the live session, expecting no interaction, and returns the tokens.
func (ts *SSOLogoutTestSuite) lastUsedJoin(client *http.Client, ouID, appClientID,
	state string) *testutils.TokenResponse {
	ts.T().Helper()
	authID, executionID := ts.lastUsedStart(client, ouID, appClientID, "openid", state)
	step := ts.flowExecute(client, map[string]interface{}{"executionId": executionID})
	ts.Require().Equal("COMPLETE", step.FlowStatus, "%s is signed in from the session without being asked: %+v",
		appClientID, step.Data)
	return ts.lastUsedRedeem(client, authID, step.Assertion, nil, appClientID)
}

// lastUsedRedeem posts the assertion and sessions to the callback and exchanges the code as the client.
func (ts *SSOLogoutTestSuite) lastUsedRedeem(client *http.Client, authID, assertion string,
	sessionData []testutils.FlowSessionData, appClientID string) *testutils.TokenResponse {
	ts.T().Helper()
	authzResponse, err := testutils.CompleteAuthorizationWithSessionData(client, authID, assertion, sessionData)
	ts.Require().NoError(err, "auth callback failed")
	code, err := testutils.ExtractAuthorizationCode(authzResponse.RedirectURI)
	ts.Require().NoError(err, "failed to extract authorization code")
	return ts.exchangeCodeAsClient(client, code, appClientID, clientSecret)
}

// tokenOUID returns the ouId claim of an access token.
func (ts *SSOLogoutTestSuite) tokenOUID(accessToken string) string {
	ts.T().Helper()
	claims, err := testutils.DecodeJWT(accessToken)
	ts.Require().NoError(err, "decode the access token")
	return claims.OUID
}

// TestLastUsedOU_AnotherAppFollowsIt verifies that a shared app opened with no OU named follows the OU the user
// last used: signed in to A1 under c1, B completes in c1 from the session, without a prompt.
func (ts *SSOLogoutTestSuite) TestLastUsedOU_AnotherAppFollowsIt() {
	f := ts.newLastUsedOUFixture()
	client := ts.newSessionClient()

	first := ts.lastUsedSignIn(client, lastUsedA1ClientID, "openid", "last_used_first")
	ts.Require().Equal(f.c1, ts.tokenOUID(first.AccessToken), "A1 signs in under its own OU")

	joined := ts.lastUsedJoin(client, "", lastUsedBClientID, "last_used_join")
	ts.Equal(f.c1, ts.tokenOUID(joined.AccessToken), "B follows the OU the user last used")
}

// TestLastUsedOU_FollowsAnOrgSwitch verifies that a device-secret exchange on an OU's token route is the
// user's last-used OU from then on: B, which landed in c1, lands in the OU switched to.
func (ts *SSOLogoutTestSuite) TestLastUsedOU_FollowsAnOrgSwitch() {
	f := ts.newLastUsedOUFixture()
	client := ts.newSessionClient()

	first := ts.lastUsedSignIn(client, lastUsedA1ClientID, deviceSSOScope, "last_used_switch_first")
	ts.Require().NotEmpty(first.DeviceSecret, "the sign-in should return a device secret")
	before := ts.lastUsedJoin(client, "", lastUsedBClientID, "last_used_before_switch")
	ts.Require().Equal(f.c1, ts.tokenOUID(before.AccessToken), "before the switch B lands in c1")

	status, switched := ts.exchangeDeviceSecretAt("/ou/"+testOUID+"/oauth2/token", lastUsedBClientID,
		clientSecret, first.DeviceSecret, "openid")
	ts.Require().Equal(http.StatusOK, status, "the switch should succeed: %v", switched)
	switchedToken, _ := switched["access_token"].(string)
	ts.Require().Equal(testOUID, ts.tokenOUID(switchedToken), "the switch signs B in to the OU on the route")

	after := ts.lastUsedJoin(client, "", lastUsedBClientID, "last_used_after_switch")
	ts.Equal(testOUID, ts.tokenOUID(after.AccessToken), "after the switch B lands in the OU switched to")
}

// TestLastUsedOU_UnusableForTheAppTakesTheFreshPath verifies that a last-used OU the app is not shared into is
// not followed: the user signs in again and picks among the OUs the app is used from, and prompt=none is
// answered with login_required.
func (ts *SSOLogoutTestSuite) TestLastUsedOU_UnusableForTheAppTakesTheFreshPath() {
	f := ts.newLastUsedOUFixture()

	ts.Run("the user signs in and picks", func() {
		client := ts.newSessionClient()
		first := ts.lastUsedSignIn(client, lastUsedA2ClientID, "openid", "last_used_fresh_first")
		ts.Require().Equal(f.c2, ts.tokenOUID(first.AccessToken), "A2 signs in under its own OU")

		authID, executionID := ts.lastUsedStart(client, "", lastUsedBClientID, "openid", "last_used_fresh")
		prompt := ts.flowExecute(client, map[string]interface{}{"executionId": executionID})
		ts.Require().NotEqual("COMPLETE", prompt.FlowStatus, "B is not shared into c2, so the session is not reused")
		ts.Empty(prompt.Assertion)
		ts.Require().NotNil(prompt.Data)
		ts.Require().True(hasFlowInput(prompt.Data.Inputs, "username"), "the user signs in again")

		status, raw, err := testutils.ExecuteFlowRaw(client, "", map[string]interface{}{
			"executionId":    executionID,
			"inputs":         map[string]string{"username": lastUsedUsername, "password": testPassword},
			"action":         "action_001",
			"challengeToken": prompt.ChallengeToken,
		})
		ts.Require().NoError(err)
		ts.Require().Equal(http.StatusOK, status, string(raw))
		var choice struct {
			FlowStatus     string `json:"flowStatus"`
			ChallengeToken string `json:"challengeToken"`
			Data           struct {
				Inputs []struct {
					Identifier string   `json:"identifier"`
					Options    []string `json:"options"`
				} `json:"inputs"`
			} `json:"data"`
		}
		ts.Require().NoError(json.Unmarshal(raw, &choice))
		ts.Require().Equal("INCOMPLETE", choice.FlowStatus, "the user picks the OU: %s", string(raw))
		ts.Require().Len(choice.Data.Inputs, 1, string(raw))
		c1Path := testOU.Handle + "/" + lastUsedC1Handle
		ts.Equal([]string{testOU.Handle, c1Path}, choice.Data.Inputs[0].Options,
			"the OUs B is used from that the user is eligible in")

		chosen := ts.flowExecute(client, map[string]interface{}{
			"executionId":    executionID,
			"inputs":         map[string]string{"ouId": c1Path},
			"action":         "action_ou",
			"challengeToken": choice.ChallengeToken,
		})
		ts.Require().Equal("COMPLETE", chosen.FlowStatus, "the pick completes the sign-in")
		token := ts.lastUsedRedeem(client, authID, chosen.Assertion, nil, lastUsedBClientID)
		ts.Equal(f.c1, ts.tokenOUID(token.AccessToken), "B signs in to the OU picked")
	})

	ts.Run("prompt=none needs the user", func() {
		client := ts.newSessionClient()
		ts.lastUsedSignIn(client, lastUsedA2ClientID, "openid", "last_used_silent_first")

		resp := ts.lastUsedAuthorize(client, "", lastUsedBClientID, "openid", "last_used_silent",
			map[string]string{"prompt": "none"})
		defer resp.Body.Close()
		query := ts.redirectQuery(resp)
		ts.Equal("login_required", query.Get("error"))
		ts.Empty(query.Get("code"), "no code is issued without the user")
		ts.Equal("last_used_silent", query.Get("state"))
	})
}

// TestLastUsedOU_OUPathWins verifies that an OU named on the OU path wins over the last-used OU.
func (ts *SSOLogoutTestSuite) TestLastUsedOU_OUPathWins() {
	f := ts.newLastUsedOUFixture()
	client := ts.newSessionClient()

	first := ts.lastUsedSignIn(client, lastUsedA1ClientID, "openid", "last_used_path_first")
	ts.Require().Equal(f.c1, ts.tokenOUID(first.AccessToken))

	joined := ts.lastUsedJoin(client, testOUID, lastUsedBClientID, "last_used_path_join")
	ts.Equal(testOUID, ts.tokenOUID(joined.AccessToken), "the OU in the path is used, not the last-used c1")
}

// TestLastUsedOU_PlainRouteDeviceSecretExchange verifies the device-secret exchange on the plain token route,
// which names no OU. For a shared app it follows the last-used OU when the user is eligible there for the app.
// When not, the flow needs the user to sign in and pick, so the exchange is refused with interaction_required.
// For an unshared app in an OU the user is not in, the boundary refuses it with invalid_grant.
func (ts *SSOLogoutTestSuite) TestLastUsedOU_PlainRouteDeviceSecretExchange() {
	f := ts.newLastUsedOUFixture()

	ts.Run("eligible in the last-used OU", func() {
		first := ts.lastUsedSignIn(ts.newSessionClient(), lastUsedA1ClientID, deviceSSOScope,
			"last_used_plain_eligible")

		status, token := ts.exchangeDeviceSecret(lastUsedBClientID, clientSecret, first.DeviceSecret, "openid")

		ts.Require().Equal(http.StatusOK, status, "the exchange should succeed: %v", token)
		accessToken, _ := token["access_token"].(string)
		ts.Equal(f.c1, ts.tokenOUID(accessToken), "the exchange follows the last-used OU")
	})

	ts.Run("not eligible in the last-used OU", func() {
		first := ts.lastUsedSignIn(ts.newSessionClient(), lastUsedA2ClientID, deviceSSOScope,
			"last_used_plain_ineligible")

		status, token := ts.exchangeDeviceSecret(lastUsedBClientID, clientSecret, first.DeviceSecret, "openid")

		ts.Equal(http.StatusBadRequest, status)
		ts.Equal("interaction_required", token["error"], "B is not shared into c2: %v", token)
		ts.NotContains(token, "access_token")
	})

	ts.Run("an unshared app in an OU the user is not in", func() {
		ts.createLastUsedApp(lastUsedA3ClientID, ts.createLastUsedOU(lastUsedC3Handle), f.flowID)
		first := ts.lastUsedSignIn(ts.newSessionClient(), lastUsedA1ClientID, deviceSSOScope,
			"last_used_plain_unshared")

		status, token := ts.exchangeDeviceSecret(lastUsedA3ClientID, clientSecret, first.DeviceSecret, "openid")

		ts.Equal(http.StatusBadRequest, status)
		ts.Equal("interaction_required", token["error"], "the user is not in c3, so a sign-in is needed: %v", token)
		ts.NotContains(token, "access_token")
	})
}

// hasFlowInput reports whether a step asks for the input.
func hasFlowInput(inputs []testutils.FlowInput, identifier string) bool {
	for _, input := range inputs {
		if input.Identifier == identifier {
			return true
		}
	}
	return false
}
