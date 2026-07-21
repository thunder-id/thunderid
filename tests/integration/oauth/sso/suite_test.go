/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package sso holds end-to-end integration tests for the SSO session lifecycle:
// establishing a per-flow SSO session and reusing it on a subsequent authorize
// (login skip), and ending it via the OIDC RP-Initiated Logout end_session_endpoint.
//
// These tests run against the live test server (see testutils.TestServerURL) and
// exercise the same observable HTTP behaviour a browser would: they carry the
// per-flow SSO cookie across requests with a cookie jar, so a second authorize is
// short-circuited by the live session and a sign-out clears that cookie.
package sso

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	clientID     = "sso_logout_test_client"
	clientSecret = "sso_logout_test_secret" //nolint:gosec // test credential
	appName      = "SSOLogoutTestApp"
	redirectURI  = "https://localhost:3000"
	// postLogoutRedirectURI must be registered on the client for the RP-initiated logout redirect
	// to be honoured.
	postLogoutRedirectURI = "https://localhost:3000/logged-out"
	resourceIdentifier    = "https://sso-logout.example.com"

	testPassword     = "testpass123"
	ssoReuseUsername = "sso_reuse_user"
	logoutUsername   = "sso_logout_user"

	// ssoCookiePrefix is the per-flow SSO handle cookie prefix minted by the session transport.
	ssoCookiePrefix = "tid_sso_"
)

var (
	testOUID string

	testUserType = testutils.UserType{
		Name: "sso-logout-person",
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
			"email":    map[string]interface{}{"type": "string"},
		},
	}

	testOU = testutils.OrganizationUnit{
		Handle:      "sso-logout-test-ou",
		Name:        "SSO Logout Test OU",
		Description: "Organization unit for SSO session and logout integration testing",
		Parent:      nil,
	}

	// ssoAuthFlow is an SSO-enabled authentication flow: SSO_CHECK short-circuits to the session
	// node when a live session exists, otherwise it prompts for credentials. The SessionExecutor
	// node (session_main) both establishes the session on first login and is the checkpoint the
	// SSO_CHECK looks for (checkpointRef) on reuse.
	ssoAuthFlow = testutils.Flow{
		Name:     "SSO Logout Test Auth Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_sso_logout_test",
		Nodes: []map[string]interface{}{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "sso_check",
			},
			{
				"id":   "sso_check",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "SSOCheckExecutor",
				},
				"properties": map[string]interface{}{
					"checkpointRef": "session_main",
				},
				"onSuccess": "session_main",
				"onFailure": "prompt_credentials",
			},
			{
				"id":   "prompt_credentials",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
							{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
						},
						"action": map[string]interface{}{
							"ref":      "action_001",
							"nextNode": "credentials_auth",
						},
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
				"onSuccess":    "session_main",
				"onIncomplete": "prompt_credentials",
			},
			{
				"id":   "session_main",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "SessionExecutor",
				},
				"onSuccess": "authorization_check",
			},
			{
				"id":   "authorization_check",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "AuthorizationExecutor",
				},
				"onSuccess": "auth_assert",
			},
			{
				"id":   "auth_assert",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "AuthAssertExecutor",
				},
				"onSuccess": "end",
			},
			{
				"id":   "end",
				"type": "END",
			},
		},
	}

	// signOutFlow terminates the SSO session the login flow established and clears its per-flow cookie.
	signOutFlow = testutils.Flow{
		Name:     "SSO Logout Test Sign-Out Flow",
		FlowType: "SIGNOUT",
		Handle:   "signout_flow_sso_logout_test",
		Nodes: []map[string]interface{}{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "session_signout",
			},
			{
				"id":   "session_signout",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "SessionSignOutExecutor",
				},
				"onSuccess": "end",
			},
			{
				"id":   "end",
				"type": "END",
			},
		},
	}
)

// SSOLogoutTestSuite exercises the SSO session lifecycle against the live server.
type SSOLogoutTestSuite struct {
	suite.Suite
	applicationID    string
	entityTypeID     string
	authFlowID       string
	signOutFlowID    string
	resourceServerID string
	userIDs          []string
}

func TestSSOLogoutTestSuite(t *testing.T) {
	suite.Run(t, new(SSOLogoutTestSuite))
}

func (ts *SSOLogoutTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(testOU)
	ts.Require().NoError(err, "Failed to create test organization unit")
	testOUID = ouID

	testUserType.OUID = ouID
	schemaID, err := testutils.CreateUserType(testUserType)
	ts.Require().NoError(err, "Failed to create test user type")
	ts.entityTypeID = schemaID

	authFlowID, err := testutils.CreateFlow(ssoAuthFlow)
	ts.Require().NoError(err, "Failed to create SSO authentication flow")
	ts.authFlowID = authFlowID

	signOutFlowID, err := testutils.CreateFlow(signOutFlow)
	ts.Require().NoError(err, "Failed to create sign-out flow")
	ts.signOutFlowID = signOutFlowID

	resourceServerID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:        "SSO Logout Resource Server",
		Description: "Resource server for SSO session and logout integration tests",
		Identifier:  resourceIdentifier,
		OUID:        testOUID,
	}, []testutils.Action{})
	ts.Require().NoError(err, "Failed to create resource server")
	ts.resourceServerID = resourceServerID

	ts.applicationID = ts.createApplication()

	for _, username := range []string{ssoReuseUsername, logoutUsername} {
		ts.createUser(username)
	}
}

func (ts *SSOLogoutTestSuite) TearDownSuite() {
	if ts.applicationID != "" {
		ts.deleteAppByID(ts.applicationID)
	}
	if ts.authFlowID != "" {
		if err := testutils.DeleteFlow(ts.authFlowID); err != nil {
			ts.T().Errorf("Failed to delete SSO authentication flow: %v", err)
		}
	}
	if ts.signOutFlowID != "" {
		if err := testutils.DeleteFlow(ts.signOutFlowID); err != nil {
			ts.T().Errorf("Failed to delete sign-out flow: %v", err)
		}
	}
	if ts.resourceServerID != "" {
		if err := testutils.DeleteResourceServer(ts.resourceServerID); err != nil {
			ts.T().Errorf("Failed to delete resource server: %v", err)
		}
	}
	// Delete the OU's children (users, then the user type) before the OU itself, otherwise the OU
	// delete fails with "organization unit has children" and leaks the fixed-handle OU across runs.
	for _, userID := range ts.userIDs {
		if err := testutils.DeleteUser(userID); err != nil {
			ts.T().Logf("Failed to delete test user %s: %v", userID, err)
		}
	}
	if ts.entityTypeID != "" {
		if err := testutils.DeleteUserType(ts.entityTypeID); err != nil {
			ts.T().Errorf("Failed to delete test user type: %v", err)
		}
	}
	if testOUID != "" {
		if err := testutils.DeleteOrganizationUnit(testOUID); err != nil {
			ts.T().Logf("Failed to delete test organization unit %s: %v", testOUID, err)
		}
	}
}

// createApplication registers the OAuth application linked to both the SSO login flow and the
// sign-out flow, with the post-logout redirect URI registered on the client.
func (ts *SSOLogoutTestSuite) createApplication() string {
	app := map[string]interface{}{
		"name":                      appName,
		"description":               "Application for SSO session and logout integration tests",
		"ouId":                      testOUID,
		"authFlowId":                ts.authFlowID,
		"isRegistrationFlowEnabled": false,
		"signOutFlowId":             ts.signOutFlowID,
		"isSignOutFlowEnabled":      true,
		"allowedUserTypes":          []string{testUserType.Name},
		"inboundAuthConfig": []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                clientID,
					"clientSecret":            clientSecret,
					"redirectUris":            []string{redirectURI},
					"postLogoutRedirectUris":  []string{postLogoutRedirectURI},
					"grantTypes":              []string{"authorization_code", "refresh_token"},
					"responseTypes":           []string{"code"},
					"tokenEndpointAuthMethod": "client_secret_basic",
					"scopes":                  []string{"openid"},
				},
			},
		},
	}

	jsonData, err := json.Marshal(app)
	ts.Require().NoError(err, "Failed to marshal application data")

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/applications", bytes.NewBuffer(jsonData))
	ts.Require().NoError(err, "Failed to create application request")
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err, "Failed to create application")
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	ts.Require().Equal(http.StatusCreated, resp.StatusCode, "Failed to create application: %s", string(body))

	var respData map[string]interface{}
	ts.Require().NoError(json.Unmarshal(body, &respData), "Failed to parse application response")
	id, ok := respData["id"].(string)
	ts.Require().True(ok, "Application response missing id")
	return id
}

func (ts *SSOLogoutTestSuite) deleteAppByID(id string) {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/applications/%s", testutils.TestServerURL, id), nil)
	if err != nil {
		ts.T().Errorf("Failed to create delete request: %v", err)
		return
	}
	resp, err := testutils.GetHTTPClient().Do(req)
	if err != nil {
		ts.T().Errorf("Failed to delete application: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		ts.T().Errorf("Failed to delete application. Status: %d", resp.StatusCode)
	}
}

func (ts *SSOLogoutTestSuite) createUser(username string) {
	user := testutils.User{
		OUID: testOUID,
		Type: testUserType.Name,
		Attributes: json.RawMessage(fmt.Sprintf(`{
			"username": "%s",
			"password": "%s",
			"email": "%s@example.com"
		}`, username, testPassword, username)),
	}
	userID, err := testutils.CreateUser(user)
	ts.Require().NoError(err, "Failed to create test user %s", username)
	ts.userIDs = append(ts.userIDs, userID)
}

// newSessionClient returns a browser-like HTTP client: a cookie jar carries the per-flow SSO
// cookie across requests, and redirects are not followed so Location headers can be inspected.
func (ts *SSOLogoutTestSuite) newSessionClient() *http.Client {
	jar, err := cookiejar.New(nil)
	ts.Require().NoError(err, "Failed to create cookie jar")
	return &http.Client{
		Jar:       jar,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// ssoCookieNames returns the per-flow SSO cookie names currently held by the client's jar.
func (ts *SSOLogoutTestSuite) ssoCookieNames(client *http.Client) []string {
	u, err := url.Parse(testutils.TestServerURL)
	ts.Require().NoError(err)
	var names []string
	for _, c := range client.Jar.Cookies(u) {
		if strings.HasPrefix(c.Name, ssoCookiePrefix) {
			names = append(names, c.Name)
		}
	}
	return names
}

// authorize starts an authorization code flow and returns the authId and executionId issued at the
// gate redirect.
func (ts *SSOLogoutTestSuite) authorize(client *http.Client, scope, state string) (string, string) {
	params := url.Values{}
	params.Set("client_id", clientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("response_type", "code")
	params.Set("scope", scope)
	params.Set("state", state)

	req, err := http.NewRequest("GET", testutils.TestServerURL+"/oauth2/authorize?"+params.Encode(), nil)
	ts.Require().NoError(err)

	resp, err := client.Do(req)
	ts.Require().NoError(err, "authorize request failed")
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusFound, resp.StatusCode, "authorize should redirect to the gate")
	authID, executionID, err := testutils.ExtractAuthData(resp.Header.Get("Location"))
	ts.Require().NoError(err, "failed to extract auth data from authorize redirect")
	return authID, executionID
}

// flowExecute posts a step to /flow/execute using the session client (so SSO cookies are carried and
// updated) and returns the resulting step.
func (ts *SSOLogoutTestSuite) flowExecute(client *http.Client, body map[string]interface{}) *testutils.FlowStep {
	data, err := json.Marshal(body)
	ts.Require().NoError(err)

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/flow/execute", bytes.NewReader(data))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	ts.Require().NoError(err, "flow execute request failed")
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	ts.Require().Equal(http.StatusOK, resp.StatusCode, "flow execute failed: %s", string(respBody))

	var step testutils.FlowStep
	ts.Require().NoError(json.Unmarshal(respBody, &step), "failed to decode flow step")
	return &step
}

// completeAuthorization submits the assertion to the authorize callback and returns the client redirect URI.
func (ts *SSOLogoutTestSuite) completeAuthorization(client *http.Client, authID, assertion string) string {
	data, err := json.Marshal(map[string]string{"authId": authID, "assertion": assertion})
	ts.Require().NoError(err)

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/oauth2/auth/callback", bytes.NewReader(data))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	ts.Require().NoError(err, "auth callback request failed")
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	ts.Require().Equal(http.StatusOK, resp.StatusCode, "auth callback failed: %s", string(respBody))

	var authzResponse testutils.AuthorizationResponse
	ts.Require().NoError(json.Unmarshal(respBody, &authzResponse), "failed to decode authorization response")
	return authzResponse.RedirectURI
}

// exchangeCode swaps an authorization code for tokens (including the id_token used as a logout hint).
func (ts *SSOLogoutTestSuite) exchangeCode(client *http.Client, code string) *testutils.TokenResponse {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("resource", resourceIdentifier)

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/oauth2/token", strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)

	resp, err := client.Do(req)
	ts.Require().NoError(err, "token request failed")
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	ts.Require().Equal(http.StatusOK, resp.StatusCode, "token request failed: %s", string(respBody))

	var token testutils.TokenResponse
	ts.Require().NoError(json.Unmarshal(respBody, &token), "failed to decode token response")
	return &token
}

// login drives a first-time SSO login to completion (prompting for credentials, establishing the
// session), and returns the issued id_token. It asserts the initial step prompts for credentials,
// proving the session did not already exist.
func (ts *SSOLogoutTestSuite) login(client *http.Client, username, state string) string {
	authID, executionID := ts.authorize(client, "openid", state)

	initial := ts.flowExecute(client, map[string]interface{}{"executionId": executionID})
	ts.Require().NotEqual("COMPLETE", initial.FlowStatus, "first login must prompt for credentials")

	step := ts.flowExecute(client, map[string]interface{}{
		"executionId":    executionID,
		"inputs":         map[string]string{"username": username, "password": testPassword},
		"action":         "action_001",
		"challengeToken": initial.ChallengeToken,
	})
	ts.Require().Equal("COMPLETE", step.FlowStatus, "credential login should complete the flow")
	ts.Require().NotEmpty(step.Assertion, "login should yield an assertion")

	clientRedirect := ts.completeAuthorization(client, authID, step.Assertion)
	code, err := testutils.ExtractAuthorizationCode(clientRedirect)
	ts.Require().NoError(err, "failed to extract authorization code")

	token := ts.exchangeCode(client, code)
	ts.Require().NotEmpty(token.IDToken, "id_token should be issued for openid scope")
	return token.IDToken
}
