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
	sidSharedUsername   = "sso_sid_shared_user"
	sidRelogUsername    = "sso_sid_relogin_user"
	sidNoSessionUser    = "sso_sid_nosession_user"
	sidPartnerClientID  = "sso_sid_partner_client"
	sidPartnerSecret    = "sso_sid_partner_secret" //nolint:gosec // test credential
	sidPartnerAppName   = "SSOSidPartnerApp"
	sidNoSessClientID   = "sso_sid_nosession_client"
	sidNoSessSecret     = "sso_sid_nosession_secret" //nolint:gosec // test credential
	sidNoSessAppName    = "SSOSidNoSessionApp"
	sidNoSessFlowHandle = "auth_flow_sso_sid_no_session"
)

// noSessionAuthFlow is the suite's authentication flow with the SSO nodes removed: credentials are
// always prompted and no session is established, so the flow has nothing to name as a sid.
var noSessionAuthFlow = testutils.Flow{
	Name:     "SSO Sid No Session Auth Flow",
	FlowType: "AUTHENTICATION",
	Handle:   sidNoSessFlowHandle,
	Nodes: []map[string]interface{}{
		{"id": "start", "type": "START", "onSuccess": "prompt_credentials"},
		{
			"id":   "prompt_credentials",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{
				{
					"inputs": []map[string]interface{}{
						{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
						{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
					},
					"action": map[string]interface{}{"ref": "action_001", "nextNode": "credentials_auth"},
				},
			},
		},
		{
			"id":   "credentials_auth",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{
				"name": "CredentialsAuthExecutor",
				"inputs": []map[string]interface{}{
					{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
					{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
				},
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

// TestSid_SharedAcrossApplicationsAndRefresh verifies the OIDC sid claim names the SSO session: two
// applications that join one session receive the same sid, and an ID token issued on refresh keeps it.
func (ts *SSOLogoutTestSuite) TestSid_SharedAcrossApplicationsAndRefresh() {
	ts.createUser(sidSharedUsername)
	partnerAppID := ts.createParticipantApplication(sidPartnerAppName, sidPartnerClientID, sidPartnerSecret)
	ts.Require().NotEmpty(partnerAppID)

	client := ts.newSessionClient()

	// The first login establishes the session; its ID token carries the session id as sid.
	first := ts.loginTokens(client, sidSharedUsername, "sid_state_1")
	sid, ok := ts.sidClaim(first.IDToken)
	ts.Require().True(ok, "the ID token of an SSO login must carry a sid claim")
	ts.Require().NotEmpty(sid)

	// The partner application joins the same session through SSO reuse and sees the same sid.
	authID, executionID := ts.authorizeAsClient(client, sidPartnerClientID, "openid", "sid_state_2")
	step := ts.flowExecute(client, map[string]interface{}{"executionId": executionID})
	ts.Require().Equal("COMPLETE", step.FlowStatus, "the partner authorize should be satisfied by SSO")
	clientRedirect := ts.completeAuthorization(client, authID, step.Assertion)
	code, err := testutils.ExtractAuthorizationCode(clientRedirect)
	ts.Require().NoError(err, "failed to extract the partner authorization code")
	partner := ts.exchangeCodeAsClient(client, code, sidPartnerClientID, sidPartnerSecret)
	partnerSid, ok := ts.sidClaim(partner.IDToken)
	ts.Require().True(ok, "the partner ID token must carry a sid claim")
	ts.Equal(sid, partnerSid, "applications sharing one SSO session must see one sid")

	// A refreshed ID token still names the session the grant belongs to.
	ts.Require().NotEmpty(first.RefreshToken, "a refresh token should be issued")
	refreshed, err := testutils.RefreshAccessToken(clientID, clientSecret, first.RefreshToken)
	ts.Require().NoError(err, "refresh should succeed")
	ts.Require().NotEmpty(refreshed.IDToken, "refresh with openid scope should issue an ID token")
	refreshedSid, ok := ts.sidClaim(refreshed.IDToken)
	ts.Require().True(ok, "the refreshed ID token must carry a sid claim")
	ts.Equal(sid, refreshedSid, "the sid must survive refresh")

	// Access tokens never carry sid: the session identifier stays off resource-server-facing tokens.
	_, hasAccessSid := ts.sidClaim(first.AccessToken)
	ts.False(hasAccessSid, "the access token must not carry a sid claim")
}

// TestSid_ChangesAfterSignOutAndReLogin verifies a sid is bound to one session: signing out and
// logging in again establishes a new session with a different sid.
func (ts *SSOLogoutTestSuite) TestSid_ChangesAfterSignOutAndReLogin() {
	ts.createUser(sidRelogUsername)
	client := ts.newSessionClient()

	first := ts.loginTokens(client, sidRelogUsername, "sid_relogin_1")
	firstSid, ok := ts.sidClaim(first.IDToken)
	ts.Require().True(ok)
	ts.Require().NotEmpty(firstSid)

	executionID, logoutID := ts.initiateLogout(client, first.IDToken, postLogoutRedirectURI, "sid_relogin_2")
	step := ts.flowExecute(client, map[string]interface{}{"executionId": executionID})
	ts.Require().Equal("COMPLETE", step.FlowStatus, "the sign-out flow should complete")
	ts.completeLogout(client, logoutID)
	ts.Require().Empty(ts.ssoCookieNames(client), "the SSO cookie should be cleared after sign-out")

	second := ts.loginTokens(client, sidRelogUsername, "sid_relogin_3")
	secondSid, ok := ts.sidClaim(second.IDToken)
	ts.Require().True(ok)
	ts.Require().NotEmpty(secondSid)
	ts.NotEqual(firstSid, secondSid, "a new session must be named by a new sid")
}

// TestSid_AbsentWithoutSessionNode verifies no sid is synthesised: a flow that establishes no SSO
// session issues an ID token without the claim, because no termination could ever reference it.
func (ts *SSOLogoutTestSuite) TestSid_AbsentWithoutSessionNode() {
	ts.createUser(sidNoSessionUser)

	flowID, err := testutils.CreateFlow(noSessionAuthFlow)
	ts.Require().NoError(err, "failed to create the no-session authentication flow")
	ts.T().Cleanup(func() {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete no-session flow: %v", err)
		}
	})

	appID, err := testutils.CreateApplication(testutils.Application{
		Name:             sidNoSessAppName,
		Description:      "Application on an authentication flow without a Session node",
		OUID:             testOUID,
		Type:             "fullstack",
		AuthFlowID:       flowID,
		AllowedUserTypes: []string{testUserType.Name},
		InboundAuthConfig: []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                sidNoSessClientID,
					"clientSecret":            sidNoSessSecret,
					"redirectUris":            []string{redirectURI},
					"grantTypes":              []string{"authorization_code"},
					"responseTypes":           []string{"code"},
					"tokenEndpointAuthMethod": "client_secret_basic",
					"scopes":                  []string{"openid"},
				},
			},
		},
	})
	ts.Require().NoError(err, "failed to create the no-session application")
	ts.T().Cleanup(func() {
		if err := testutils.DeleteApplication(appID); err != nil {
			ts.T().Logf("Failed to delete no-session application: %v", err)
		}
	})

	client := ts.newSessionClient()
	authID, executionID := ts.authorizeAsClient(client, sidNoSessClientID, "openid", "sid_nosession_1")
	initial := ts.flowExecute(client, map[string]interface{}{"executionId": executionID})
	ts.Require().NotEqual("COMPLETE", initial.FlowStatus, "the flow must prompt for credentials")
	step := ts.flowExecute(client, map[string]interface{}{
		"executionId":    executionID,
		"inputs":         map[string]string{"username": sidNoSessionUser, "password": testPassword},
		"action":         "action_001",
		"challengeToken": initial.ChallengeToken,
	})
	ts.Require().Equal("COMPLETE", step.FlowStatus, "credential login should complete the flow")
	clientRedirect := ts.completeAuthorization(client, authID, step.Assertion)
	code, err := testutils.ExtractAuthorizationCode(clientRedirect)
	ts.Require().NoError(err)
	tokens := ts.exchangeCodeAsClient(client, code, sidNoSessClientID, sidNoSessSecret)
	ts.Require().NotEmpty(tokens.IDToken, "an ID token should still be issued for openid scope")

	_, has := ts.sidClaim(tokens.IDToken)
	ts.False(has, "a flow without a Session node must not issue a sid claim")
	ts.Empty(ts.ssoCookieNames(client), "no SSO cookie should be set by a flow without a Session node")
}

// sidClaim decodes a JWT and returns its sid claim, reporting whether the claim is present.
func (ts *SSOLogoutTestSuite) sidClaim(token string) (string, bool) {
	ts.T().Helper()

	claims, err := testutils.DecodeJWTPayloadMap(token)
	ts.Require().NoError(err, "failed to decode token payload")
	sid, ok := claims["sid"].(string)
	return sid, ok
}

// exchangeCodeAsClient swaps an authorization code for tokens on behalf of the given client. It is the
// multi-application counterpart of exchangeCode, which is fixed to the suite's main client.
func (ts *SSOLogoutTestSuite) exchangeCodeAsClient(client *http.Client, code, cID, cSecret string,
) *testutils.TokenResponse {
	ts.T().Helper()

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)

	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+"/oauth2/token",
		strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(cID, cSecret)

	resp, err := client.Do(req)
	ts.Require().NoError(err, "token request failed")
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	ts.Require().Equal(http.StatusOK, resp.StatusCode, "token request failed: %s", string(respBody))

	var token testutils.TokenResponse
	ts.Require().NoError(json.Unmarshal(respBody, &token), "failed to decode token response")
	return &token
}
