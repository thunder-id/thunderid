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
	"testing"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

const (
	refreshTokenTestClientID     = "refresh_token_test_client"
	refreshTokenTestClientSecret = "refresh_token_test_secret"
	refreshTokenTestAppName      = "RefreshTokenTestApp"
	refreshTokenTestRedirectURI  = "https://localhost:3000"
	refreshTokenTestUsername     = "refresh_token_test_user"
	refreshTokenTestPassword     = "testpass123"
)

var (
	refreshTokenTestOU = testutils.OrganizationUnit{
		Handle:      "refresh-token-test-ou",
		Name:        "Refresh Token Test OU",
		Description: "Organization unit for refresh token integration testing",
		Parent:      nil,
	}

	refreshTokenTestUserType = testutils.UserType{
		Name: "refresh-token-test-person",
		Schema: map[string]interface{}{
			"username": map[string]interface{}{
				"type": "string",
			},
			"password": map[string]interface{}{
				"type":       "string",
				"credential": true,
			},
			"email": map[string]interface{}{
				"type": "string",
			},
			"given_name": map[string]interface{}{
				"type": "string",
			},
			"family_name": map[string]interface{}{
				"type": "string",
			},
		},
	}

	refreshTokenTestAuthFlow = testutils.Flow{
		Name:     "Refresh Token Test Auth Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_refresh_token_test",
		Nodes: []map[string]interface{}{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "prompt_credentials",
			},
			{
				"id":   "prompt_credentials",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{
								"ref":        "input_001",
								"identifier": "username",
								"type":       "TEXT_INPUT",
								"required":   true,
							},
							{
								"ref":        "input_002",
								"identifier": "password",
								"type":       "PASSWORD_INPUT",
								"required":   true,
							},
						},
						"action": map[string]interface{}{
							"ref":      "action_001",
							"nextNode": "basic_auth",
						},
					},
				},
			},
			{
				"id":   "basic_auth",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "BasicAuthExecutor",
					"inputs": []map[string]interface{}{
						{
							"ref":        "input_001",
							"identifier": "username",
							"type":       "TEXT_INPUT",
							"required":   true,
						},
						{
							"ref":        "input_002",
							"identifier": "password",
							"type":       "PASSWORD_INPUT",
							"required":   true,
						},
					},
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
)

// RefreshTokenTestSuite tests the refresh token grant flow,
// specifically verifying ID token behavior.
type RefreshTokenTestSuite struct {
	suite.Suite
	applicationID string
	entityTypeID  string
	authFlowID    string
	ouID          string
	userID        string
	client        *http.Client
}

func TestRefreshTokenTestSuite(t *testing.T) {
	suite.Run(t, new(RefreshTokenTestSuite))
}

func (ts *RefreshTokenTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	// Create organization unit.
	ouID, err := testutils.CreateOrganizationUnit(refreshTokenTestOU)
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

	// Create user type.
	refreshTokenTestUserType.OUID = ouID
	schemaID, err := testutils.CreateUserType(refreshTokenTestUserType)
	ts.Require().NoError(err, "Failed to create test user type")
	ts.entityTypeID = schemaID

	// Create authentication flow.
	flowID, err := testutils.CreateFlow(refreshTokenTestAuthFlow)
	ts.Require().NoError(err, "Failed to create test authentication flow")
	ts.authFlowID = flowID

	// Create application with authorization_code and refresh_token grants.
	ts.applicationID = ts.createTestApplication()

	// Create test user.
	user := testutils.User{
		OUID: ouID,
		Type: "refresh-token-test-person",
		Attributes: json.RawMessage(fmt.Sprintf(`{
			"username": "%s",
			"password": "%s",
			"email": "refresh_token_test@example.com",
			"given_name": "Refresh",
			"family_name": "TokenTest"
		}`, refreshTokenTestUsername, refreshTokenTestPassword)),
	}
	userID, err := testutils.CreateUser(user)
	ts.Require().NoError(err, "Failed to create test user")
	ts.userID = userID
}

func (ts *RefreshTokenTestSuite) createTestApplication() string {
	app := map[string]interface{}{
		"name":                      refreshTokenTestAppName,
		"description":               "Application for refresh token integration tests",
		"ouId":                      ts.ouID,
		"authFlowId":                ts.authFlowID,
		"isRegistrationFlowEnabled": false,
		"allowedUserTypes":          []string{"refresh-token-test-person"},
		"inboundAuthConfig": []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                refreshTokenTestClientID,
					"clientSecret":            refreshTokenTestClientSecret,
					"redirectUris":            []string{refreshTokenTestRedirectURI},
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
		ts.T().Fatalf("Failed to create application. Status: %d, Response: %s",
			resp.StatusCode, string(bodyBytes))
	}

	var respData map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&respData)
	ts.Require().NoError(err, "Failed to parse response")

	appID := respData["id"].(string)
	ts.T().Logf("Created refresh token test application with ID: %s", appID)
	return appID
}

func (ts *RefreshTokenTestSuite) TearDownSuite() {
	if ts.userID != "" {
		if err := testutils.DeleteUser(ts.userID); err != nil {
			ts.T().Logf("Failed to delete test user: %v", err)
		}
	}

	if ts.applicationID != "" {
		if err := testutils.DeleteApplication(ts.applicationID); err != nil {
			ts.T().Logf("Failed to delete test application: %v", err)
		}
	}

	if ts.authFlowID != "" {
		if err := testutils.DeleteFlow(ts.authFlowID); err != nil {
			ts.T().Logf("Failed to delete test auth flow: %v", err)
		}
	}

	if ts.entityTypeID != "" {
		if err := testutils.DeleteUserType(ts.entityTypeID); err != nil {
			ts.T().Logf("Failed to delete test user type: %v", err)
		}
	}

	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete test OU: %v", err)
		}
	}
}

// obtainTokensViaAuthCodeFlow performs the complete authorization code flow
// and returns the token response.
func (ts *RefreshTokenTestSuite) obtainTokensViaAuthCodeFlow(
	scope string) *testutils.TokenResponse {

	// Step 1: Initiate authorization flow.
	resp, err := testutils.InitiateAuthorizationFlow(
		refreshTokenTestClientID, refreshTokenTestRedirectURI,
		"code", scope, "test-state")
	ts.Require().NoError(err, "Failed to initiate authorization flow")
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusFound, resp.StatusCode,
		"Expected redirect status from authorization endpoint")

	location := resp.Header.Get("Location")
	ts.Require().NotEmpty(location, "Expected Location header")

	authID, executionId, err := testutils.ExtractAuthData(location)
	ts.Require().NoError(err, "Failed to extract auth data")

	// Step 2: Execute authentication flow.
	initialStep, err := testutils.ExecuteAuthenticationFlow(executionId, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")

	flowStep, err := testutils.ExecuteAuthenticationFlow(executionId,
		map[string]string{
			"username": refreshTokenTestUsername,
			"password": refreshTokenTestPassword,
		}, "action_001", initialStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to execute authentication flow")
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus,
		"Authentication flow should complete")
	ts.Require().NotEmpty(flowStep.Assertion, "Assertion should not be empty")

	// Step 3: Complete authorization.
	authzResp, err := testutils.CompleteAuthorization(
		authID, flowStep.Assertion)
	ts.Require().NoError(err, "Failed to complete authorization")

	code, err := testutils.ExtractAuthorizationCode(authzResp.RedirectURI)
	ts.Require().NoError(err, "Failed to extract authorization code")

	// Step 4: Exchange code for tokens using Basic Auth.
	tokenResult, err := testutils.RequestToken(
		refreshTokenTestClientID, refreshTokenTestClientSecret,
		code, refreshTokenTestRedirectURI, "authorization_code")
	ts.Require().NoError(err, "Failed to request token")
	ts.Require().Equal(http.StatusOK, tokenResult.StatusCode,
		"Token request should succeed. Response: %s",
		string(tokenResult.Body))
	ts.Require().NotNil(tokenResult.Token, "Token should not be nil")
	ts.Require().NotEmpty(tokenResult.Token.AccessToken,
		"Access token should not be empty")
	ts.Require().NotEmpty(tokenResult.Token.RefreshToken,
		"Refresh token should not be empty")

	return tokenResult.Token
}

// TestRefreshTokenGrantReturnsIDToken verifies that when the original
// authorization code flow includes the "openid" scope, the refresh token
// grant also returns a new ID token.
func (ts *RefreshTokenTestSuite) TestRefreshTokenGrantReturnsIDToken() {
	// Step 1: Obtain tokens via auth code flow with openid scope.
	tokenResponse := ts.obtainTokensViaAuthCodeFlow("openid")
	ts.NotEmpty(tokenResponse.IDToken,
		"Auth code response should contain an ID token with openid scope")

	// Step 2: Use the refresh token to get new tokens.
	refreshResponse, err := testutils.RefreshAccessToken(
		refreshTokenTestClientID, refreshTokenTestClientSecret,
		tokenResponse.RefreshToken)
	ts.Require().NoError(err, "Refresh token request should succeed")
	ts.Require().NotNil(refreshResponse,
		"Refresh token response should not be nil")

	// Step 3: Validate the refresh token response contains an ID token.
	ts.NotEmpty(refreshResponse.AccessToken,
		"Refresh response should contain an access token")
	ts.NotEmpty(refreshResponse.IDToken,
		"Refresh response should contain an ID token with openid scope")
}

// TestRefreshTokenGrantWithoutOpenIDScope verifies that when the original
// authorization code flow does not include the "openid" scope, the refresh
// token grant does not return an ID token.
func (ts *RefreshTokenTestSuite) TestRefreshTokenGrantWithoutOpenIDScope() {
	// Step 1: Obtain tokens via auth code flow without openid scope.
	tokenResponse := ts.obtainTokensViaAuthCodeFlow("internal_user_mgt_view")
	ts.Empty(tokenResponse.IDToken,
		"Auth code response should not contain an ID token without openid scope")

	// Step 2: Use the refresh token to get new tokens.
	refreshResponse, err := testutils.RefreshAccessToken(
		refreshTokenTestClientID, refreshTokenTestClientSecret,
		tokenResponse.RefreshToken)
	ts.Require().NoError(err, "Refresh token request should succeed")
	ts.Require().NotNil(refreshResponse,
		"Refresh token response should not be nil")

	// Step 3: Validate the refresh token response has no ID token.
	ts.NotEmpty(refreshResponse.AccessToken,
		"Refresh response should contain an access token")
	ts.Empty(refreshResponse.IDToken,
		"Refresh response should not contain an ID token without openid scope")
}
