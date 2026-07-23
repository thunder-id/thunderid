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

package token

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	tfidTestClientID     = "tfid_test_client"
	tfidTestClientSecret = "tfid_test_secret"
	tfidTestAppName      = "TfidTestApp"
	tfidTestRedirectURI  = "https://localhost:3000"
	tfidTestUsername     = "tfid_test_user"
	tfidTestPassword     = "testpass123"
	tfidTestResource     = "https://tfid.example.com"
)

var (
	tfidTestOU = testutils.OrganizationUnit{
		Handle:      "tfid-test-ou",
		Name:        "Tfid Test OU",
		Description: "Organization unit for token family id integration testing",
		Parent:      nil,
	}

	tfidTestUserType = testutils.UserType{
		Name: "tfid-test-person",
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
			"email":    map[string]interface{}{"type": "string"},
		},
	}

	// tfidTestAuthFlow is an SSO-enabled authorization-code flow. The SessionExecutor node
	// (session_main) establishes the SSO session and is where the token family id (tfid) is minted,
	// so the issued tokens carry a tfid.
	tfidTestAuthFlow = testutils.Flow{
		Name:     "Tfid Test Auth Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_tfid_test",
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
)

// TfidTestSuite exercises token family id (tfid) propagation and grant-scoped revocation end-to-end
// through the real authorization-code login flow, signed tokens, introspection, and the runtime
// persistent database. Revocation is observed via AS introspection, which reads the deny lists
// directly (unlike RS enforcement, which is eventually consistent through the cache).
type TfidTestSuite struct {
	suite.Suite
	applicationID    string
	entityTypeID     string
	authFlowID       string
	ouID             string
	userID           string
	resourceServerID string
	client           *http.Client
}

func TestTfidTestSuite(t *testing.T) {
	suite.Run(t, new(TfidTestSuite))
}

func (ts *TfidTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	ouID, err := testutils.CreateOrganizationUnit(tfidTestOU)
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

	tfidTestUserType.OUID = ouID
	schemaID, err := testutils.CreateUserType(tfidTestUserType)
	ts.Require().NoError(err, "Failed to create test user type")
	ts.entityTypeID = schemaID

	flowID, err := testutils.CreateFlow(tfidTestAuthFlow)
	ts.Require().NoError(err, "Failed to create test authentication flow")
	ts.authFlowID = flowID

	resourceServerID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:        "Tfid Resource Server",
		Description: "Resource server for tfid integration tests",
		Identifier:  tfidTestResource,
		OUID:        ts.ouID,
	}, []testutils.Action{})
	ts.Require().NoError(err, "Failed to create tfid resource server")
	ts.resourceServerID = resourceServerID

	ts.applicationID = ts.createTestApplication()

	user := testutils.User{
		OUID: ouID,
		Type: "tfid-test-person",
		Attributes: json.RawMessage(fmt.Sprintf(`{
			"username": "%s",
			"password": "%s",
			"email": "tfid_test@example.com"
		}`, tfidTestUsername, tfidTestPassword)),
	}
	userID, err := testutils.CreateUser(user)
	ts.Require().NoError(err, "Failed to create test user")
	ts.userID = userID
}

func (ts *TfidTestSuite) createTestApplication() string {
	app := map[string]interface{}{
		"name":                      tfidTestAppName,
		"description":               "Application for tfid integration tests",
		"ouId":                      ts.ouID,
		"authFlowId":                ts.authFlowID,
		"isRegistrationFlowEnabled": false,
		"allowedUserTypes":          []string{"tfid-test-person"},
		"inboundAuthConfig": []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                tfidTestClientID,
					"clientSecret":            tfidTestClientSecret,
					"redirectUris":            []string{tfidTestRedirectURI},
					"grantTypes":              []string{"authorization_code", "refresh_token"},
					"responseTypes":           []string{"code"},
					"tokenEndpointAuthMethod": "client_secret_basic",
				},
			},
		},
	}

	jsonData, err := json.Marshal(app)
	ts.Require().NoError(err, "Failed to marshal application data")

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/applications", bytes.NewBuffer(jsonData))
	ts.Require().NoError(err, "Failed to create request")
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err, "Failed to create application")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		ts.T().Fatalf("Failed to create application. Status: %d, Response: %s", resp.StatusCode, string(bodyBytes))
	}

	var respData map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&respData)
	ts.Require().NoError(err, "Failed to parse response")
	return respData["id"].(string)
}

func (ts *TfidTestSuite) TearDownSuite() {
	if ts.userID != "" {
		_ = testutils.DeleteUser(ts.userID)
	}
	if ts.applicationID != "" {
		_ = testutils.DeleteApplication(ts.applicationID)
	}
	if ts.authFlowID != "" {
		_ = testutils.DeleteFlow(ts.authFlowID)
	}
	if ts.resourceServerID != "" {
		_ = testutils.DeleteResourceServer(ts.resourceServerID)
	}
	if ts.entityTypeID != "" {
		_ = testutils.DeleteUserType(ts.entityTypeID)
	}
	if ts.ouID != "" {
		_ = testutils.DeleteOrganizationUnit(ts.ouID)
	}
}

// obtainCodeAndTokens drives the full authorization-code login flow and returns the authorization
// code (for replay tests) together with the issued tokens.
func (ts *TfidTestSuite) obtainCodeAndTokens() (string, *testutils.TokenResponse) {
	resp, err := testutils.InitiateAuthorizationFlow(
		tfidTestClientID, tfidTestRedirectURI, "code", "openid", "test-state")
	ts.Require().NoError(err, "Failed to initiate authorization flow")
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusFound, resp.StatusCode, "Expected redirect from authorization endpoint")

	location := resp.Header.Get("Location")
	ts.Require().NotEmpty(location, "Expected Location header")

	authID, executionID, err := testutils.ExtractAuthData(location)
	ts.Require().NoError(err, "Failed to extract auth data")

	initialStep, err := testutils.ExecuteAuthenticationFlow(executionID, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")

	flowStep, err := testutils.ExecuteAuthenticationFlow(executionID, map[string]string{
		"username": tfidTestUsername,
		"password": tfidTestPassword,
	}, "action_001", initialStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to execute authentication flow")
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus, "Authentication flow should complete")

	authzResp, err := testutils.CompleteAuthorization(authID, flowStep.Assertion)
	ts.Require().NoError(err, "Failed to complete authorization")

	code, err := testutils.ExtractAuthorizationCode(authzResp.RedirectURI)
	ts.Require().NoError(err, "Failed to extract authorization code")

	tokenResult, err := testutils.RequestTokenWithResource(
		tfidTestClientID, tfidTestClientSecret, code, tfidTestRedirectURI, "authorization_code", tfidTestResource)
	ts.Require().NoError(err, "Failed to request token")
	ts.Require().Equal(http.StatusOK, tokenResult.StatusCode, "Token request should succeed: %s", string(tokenResult.Body))
	ts.Require().NotNil(tokenResult.Token, "Token should not be nil")
	ts.Require().NotEmpty(tokenResult.Token.AccessToken, "Access token should not be empty")
	ts.Require().NotEmpty(tokenResult.Token.RefreshToken, "Refresh token should not be empty")

	return code, tokenResult.Token
}

// obtainTokens is obtainCodeAndTokens without the code.
func (ts *TfidTestSuite) obtainTokens() *testutils.TokenResponse {
	_, tokens := ts.obtainCodeAndTokens()
	return tokens
}

// tfidClaim returns the tfid claim of a signed token, or "" when absent.
func (ts *TfidTestSuite) tfidClaim(token string) string {
	claims, err := testutils.DecodeJWTPayloadMap(token)
	ts.Require().NoError(err, "Failed to decode token payload")
	tfid, _ := claims["tfid"].(string)
	return tfid
}

// introspectActive reports whether the access token is active per the AS introspection endpoint.
func (ts *TfidTestSuite) introspectActive(token string) bool {
	form := url.Values{}
	form.Set("token", token)
	req, err := http.NewRequest("POST", testutils.TestServerURL+"/oauth2/introspect",
		strings.NewReader(form.Encode()))
	ts.Require().NoError(err, "Failed to build introspection request")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(tfidTestClientID, tfidTestClientSecret)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err, "Introspection request failed")
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode, "Introspection should return 200")

	var result struct {
		Active bool `json:"active"`
	}
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&result), "Failed to parse introspection response")
	return result.Active
}

// revokeRefreshToken revokes a refresh token via the RFC 7009 endpoint under the owning client.
func (ts *TfidTestSuite) revokeRefreshToken(refreshToken string) {
	form := url.Values{}
	form.Set("token", refreshToken)
	form.Set("token_type_hint", "refresh_token")
	req, err := http.NewRequest("POST", testutils.TestServerURL+"/oauth2/revoke",
		strings.NewReader(form.Encode()))
	ts.Require().NoError(err, "Failed to build revocation request")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(tfidTestClientID, tfidTestClientSecret)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err, "Revocation request failed")
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode, "Revocation should return 200")
}

// The access and refresh tokens of one login share a single, non-empty tfid.
func (ts *TfidTestSuite) TestAccessAndRefreshTokensShareTfid() {
	tokens := ts.obtainTokens()

	atTfid := ts.tfidClaim(tokens.AccessToken)
	rtTfid := ts.tfidClaim(tokens.RefreshToken)

	ts.NotEmpty(atTfid, "Access token should carry a tfid")
	ts.NotEmpty(rtTfid, "Refresh token should carry a tfid")
	ts.Equal(atTfid, rtTfid, "Access and refresh tokens of one grant share a tfid")
}

// The tfid is copied onto both tokens minted during a refresh: the new access token and, since
// refresh-token rotation is enabled by default, the rotated refresh token.
func (ts *TfidTestSuite) TestTfidPreservedOnRefresh() {
	tokens := ts.obtainTokens()
	originalTfid := ts.tfidClaim(tokens.AccessToken)
	ts.Require().NotEmpty(originalTfid)

	refreshed, err := testutils.RefreshAccessToken(tfidTestClientID, tfidTestClientSecret, tokens.RefreshToken)
	ts.Require().NoError(err, "Refresh should succeed")
	ts.Require().NotEmpty(refreshed.AccessToken, "Refreshed access token should not be empty")
	ts.Require().NotEmpty(refreshed.RefreshToken, "Rotation is enabled, so a new refresh token should be issued")

	ts.Equal(originalTfid, ts.tfidClaim(refreshed.AccessToken),
		"The refreshed access token keeps the grant's tfid")
	ts.Equal(originalTfid, ts.tfidClaim(refreshed.RefreshToken),
		"The rotated refresh token keeps the grant's tfid")
}

// Explicitly revoking a login's refresh token also drops its access token (grant-scoped revocation).
func (ts *TfidTestSuite) TestExplicitRefreshRevokeDropsAccessToken() {
	tokens := ts.obtainTokens()
	ts.Require().True(ts.introspectActive(tokens.AccessToken), "Access token should start active")

	ts.revokeRefreshToken(tokens.RefreshToken)

	ts.False(ts.introspectActive(tokens.AccessToken),
		"Revoking the refresh token must drop the login's access token via its tfid")
}

// Redeeming an authorization code twice (replay) revokes the whole grant issued from the first redemption.
func (ts *TfidTestSuite) TestAuthCodeReplayRevokesGrant() {
	code, tokens := ts.obtainCodeAndTokens()
	ts.Require().True(ts.introspectActive(tokens.AccessToken), "Access token should start active")

	// Replay the already-consumed code.
	replay, err := testutils.RequestTokenWithResource(
		tfidTestClientID, tfidTestClientSecret, code, tfidTestRedirectURI, "authorization_code", tfidTestResource)
	ts.Require().NoError(err, "Replay request should complete")
	ts.NotEqual(http.StatusOK, replay.StatusCode, "Replaying a consumed code must not issue tokens")

	ts.False(ts.introspectActive(tokens.AccessToken),
		"An authorization-code replay must revoke the grant issued from the first redemption")
}

// Independent logins get distinct tfids, and revoking one family leaves the other untouched.
func (ts *TfidTestSuite) TestIndependentGrantsAreIsolated() {
	first := ts.obtainTokens()
	second := ts.obtainTokens()

	ts.NotEqual(ts.tfidClaim(first.AccessToken), ts.tfidClaim(second.AccessToken),
		"Two independent logins must mint different tfids")

	// Revoke the first login's family; the second must remain active.
	ts.revokeRefreshToken(first.RefreshToken)

	ts.False(ts.introspectActive(first.AccessToken), "The revoked login's access token is inactive")
	ts.True(ts.introspectActive(second.AccessToken),
		"An independent login must be unaffected by another login's revocation")
}
