// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package devicegrant

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	deviceSecretClientID     = "device_grant_device_secret_client"
	deviceSecretClientSecret = "device_grant_device_secret_secret"
	deviceSSOScope           = "openid device_sso"
	tokenExchangeGrantType   = "urn:ietf:params:oauth:grant-type:token-exchange"
	jwtTokenType             = "urn:ietf:params:oauth:token-type:jwt"
)

// TestDeviceGrant_ReturnsDeviceSecret verifies that a device that asked for device_sso gets a device secret
// carrying the session its approval started, for the approving user, and that the secret signs that user in
// again with no browser.
func (ts *DeviceGrantTestSuite) TestDeviceGrant_ReturnsDeviceSecret() {
	tokens, sessions := ts.approveWithSessions(deviceSSOScope, true)

	ts.Equal(http.StatusOK, tokens.statusCode, tokens.errorCode)
	ts.Require().Len(sessions, 1, "the approval started one session")
	header, err := testutils.DecodeJWTHeaderMap(tokens.deviceSecret)
	ts.Require().NoError(err, "the token response should carry a device secret")
	ts.Equal("sso-session+jwt", header["typ"])
	claims, err := testutils.DecodeJWT(tokens.deviceSecret)
	ts.Require().NoError(err)
	ts.Equal(ts.userID, claims.Sub, "the device secret names the user who approved the device")
	entries, _ := claims.Additional["session_data"].([]interface{})
	ts.Require().Len(entries, 1)
	entry, _ := entries[0].(map[string]interface{})
	ts.Equal(sessions[0].FlowIdentifier, entry["flow_identifier"])
	ts.Equal(sessions[0].SessionHandle, entry["session_handle"])

	status, body := ts.exchangeDeviceSecret(tokens.deviceSecret)
	ts.Require().Equal(http.StatusOK, status, "the device secret should be exchangeable: %v", body)
	accessToken, _ := body["access_token"].(string)
	accessClaims, err := testutils.DecodeJWT(accessToken)
	ts.Require().NoError(err)
	ts.Equal(ts.userID, accessClaims.Sub)
}

// TestDeviceGrant_NoDeviceSecretWithoutDeviceSSO verifies that a device that did not ask for device_sso gets
// no device secret, even when the callback posted the session its approval started.
func (ts *DeviceGrantTestSuite) TestDeviceGrant_NoDeviceSecretWithoutDeviceSSO() {
	tokens, sessions := ts.approveWithSessions("openid", true)

	ts.Equal(http.StatusOK, tokens.statusCode, tokens.errorCode)
	ts.Require().Len(sessions, 1, "the approval started one session")
	ts.Empty(tokens.deviceSecret)
}

// TestDeviceGrant_NoDeviceSecretWithoutPostedSessions verifies that a device that asked for device_sso gets its
// tokens but no device secret when the callback posted no session, as a browser SDK that does not collect them does.
func (ts *DeviceGrantTestSuite) TestDeviceGrant_NoDeviceSecretWithoutPostedSessions() {
	tokens, sessions := ts.approveWithSessions(deviceSSOScope, false)

	ts.Equal(http.StatusOK, tokens.statusCode, tokens.errorCode)
	ts.Require().Len(sessions, 1, "the approval started one session")
	ts.NotEmpty(tokens.accessToken)
	ts.NotEmpty(tokens.idToken)
	ts.Empty(tokens.deviceSecret)
}

// approveWithSessions runs a device authorization for the device secret application's client with the given
// scope, approves it as a browser would (collecting the sessions every flow response returns and, when post is
// set, posting them with the callback), then polls once, and returns the token response and the sessions
// collected.
func (ts *DeviceGrantTestSuite) approveWithSessions(scope string, post bool) (deviceTokenResult,
	[]testutils.FlowSessionData) {
	authStatus, authResponse, _ := ts.deviceAuthorizeAs(scope, deviceSecretClientID, deviceSecretClientSecret)
	ts.Require().Equal(http.StatusOK, authStatus, "device authorization error: %s", authResponse.Error)

	var sessions []testutils.FlowSessionData
	collect := func(step *testutils.FlowStep) {
		for _, issued := range step.SessionData {
			sessions = append(sessions, issued.FlowSessionData)
		}
	}
	codePrompt := ts.startDeviceGrantFlowFor(ts.deviceSecretAppID)
	collect(codePrompt)
	authPrompt, err := testutils.ExecuteAuthenticationFlow(codePrompt.ExecutionID,
		map[string]string{"user_code": authResponse.UserCode}, "device-code-submit", codePrompt.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("INCOMPLETE", authPrompt.FlowStatus, "with no session, the sign-in asks for credentials")
	collect(authPrompt)
	completed, err := testutils.ExecuteAuthenticationFlow(authPrompt.ExecutionID, map[string]string{
		"username": deviceGrantUsername,
		"password": deviceGrantPassword,
	}, "action_credentials", authPrompt.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("COMPLETE", completed.FlowStatus)
	collect(completed)

	assertionClaims, err := testutils.DecodeJWT(completed.Assertion)
	ts.Require().NoError(err)
	authRequestID, _ := assertionClaims.Additional["authorization_request_id"].(string)
	var posted []testutils.FlowSessionData
	if post {
		posted = sessions
	}
	status, callbackError := ts.postCallbackWithSessions(authRequestID, completed.Assertion, posted)
	ts.Require().Equal(http.StatusOK, status, callbackError)

	return ts.pollTokenAs(authResponse.DeviceCode, deviceSecretClientID, deviceSecretClientSecret), sessions
}

// exchangeDeviceSecret exchanges a device secret at the token endpoint as the device secret client, and returns
// the status and the decoded response body.
func (ts *DeviceGrantTestSuite) exchangeDeviceSecret(deviceSecret string) (int, map[string]interface{}) {
	form := url.Values{
		"grant_type":         []string{tokenExchangeGrantType},
		"subject_token":      []string{deviceSecret},
		"subject_token_type": []string{jwtTokenType},
	}
	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+deviceTokenEndpoint,
		strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(deviceSecretClientID, deviceSecretClientSecret)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	var body map[string]interface{}
	ts.Require().NoError(json.Unmarshal(raw, &body), string(raw))
	return resp.StatusCode, body
}

// deviceGrantSSOAuthenticationFlow is a credential sign-in flow that keeps an SSO session, so its responses
// return the session it starts.
func deviceGrantSSOAuthenticationFlow(ouID string) testutils.Flow {
	inputs := []map[string]interface{}{
		{"ref": "input_username", "identifier": "username", "type": "TEXT_INPUT", "required": true},
		{"ref": "input_password", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
	}
	return testutils.Flow{
		Name:     "Device Grant SSO Authentication Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "device_grant_sso_auth_flow",
		OUID:     ouID,
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "sso_check"},
			{
				"id": "sso_check", "type": "TASK_EXECUTION",
				"executor":   map[string]interface{}{"name": "SSOCheckExecutor"},
				"properties": map[string]interface{}{"checkpointRef": "session_main"},
				"onSuccess":  "session_main", "onFailure": "prompt_credentials",
			},
			{
				"id":   "prompt_credentials",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{{
					"inputs": inputs,
					"action": map[string]interface{}{"ref": "action_credentials", "nextNode": "credentials_auth"},
				}},
			},
			{
				"id": "credentials_auth", "type": "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "CredentialsAuthExecutor", "inputs": inputs},
				"onSuccess": "session_main", "onIncomplete": "prompt_credentials",
			},
			{
				"id": "session_main", "type": "TASK_EXECUTION",
				"executor": map[string]interface{}{"name": "SessionExecutor"}, "onSuccess": "auth_assert",
			},
			{
				"id": "auth_assert", "type": "TASK_EXECUTION",
				"executor": map[string]interface{}{"name": "AuthAssertExecutor"}, "onSuccess": "end",
			},
			{"id": "end", "type": "END"},
		},
	}
}
