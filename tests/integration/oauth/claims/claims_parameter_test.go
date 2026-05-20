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

package claims

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

const (
	testServerURL = "https://localhost:8095"
	clientID      = "claims_test_client_123"
	clientSecret  = "claims_test_secret_123"
	appName       = "ClaimsParameterTestApp"
	redirectURI   = "https://localhost:3000"
)

var (
	testUserType = testutils.UserType{
		Name: "claims-test-person",
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
			"phone_number": map[string]interface{}{
				"type": "string",
			},
			"locale": map[string]interface{}{
				"type": "string",
			},
		},
	}
)

type ClaimsParameterTestSuite struct {
	suite.Suite
	flowID        string
	applicationID string
	entityTypeID  string
	userID        string
	client        *http.Client
	ouID          string
}

func TestClaimsParameterTestSuite(t *testing.T) {
	suite.Run(t, new(ClaimsParameterTestSuite))
}

func (ts *ClaimsParameterTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	// Create test organization unit
	ou := testutils.OrganizationUnit{
		Handle:      "claims-test-ou",
		Name:        "Claims Test OU",
		Description: "Organization unit for Claims Parameter integration testing",
		Parent:      nil,
	}
	ouID, err := testutils.CreateOrganizationUnit(ou)
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

	// Create user type
	testUserType.OUID = ts.ouID
	schemaID, err := testutils.CreateUserType(testUserType)
	ts.Require().NoError(err, "Failed to create test user type")
	ts.entityTypeID = schemaID

	// Create test user
	ts.userID = ts.createTestUser()

	// Create authentication flow
	ts.flowID = ts.createTestAuthenticationFlow()

	// Create OAuth application with claims support
	ts.applicationID = ts.createTestApplication(ts.flowID)
}

func (ts *ClaimsParameterTestSuite) TearDownSuite() {
	// Clean up application
	if ts.applicationID != "" {
		ts.deleteApplication(ts.applicationID)
	}

	// Clean up authentication flow
	if ts.flowID != "" {
		if err := testutils.DeleteFlow(ts.flowID); err != nil {
			ts.T().Logf("Failed to delete authentication flow during teardown: %v", err)
		}
	}

	// Clean up user
	if ts.userID != "" {
		testutils.DeleteUser(ts.userID)
	}

	// Clean up organization unit
	if ts.ouID != "" {
		testutils.DeleteOrganizationUnit(ts.ouID)
	}

	// Clean up user type
	if ts.entityTypeID != "" {
		if err := testutils.DeleteUserType(ts.entityTypeID); err != nil {
			ts.T().Logf("Failed to delete user type during teardown: %v", err)
		}
	}
}

func (ts *ClaimsParameterTestSuite) createTestUser() string {
	attributes := map[string]interface{}{
		"username":     "claims_test_user",
		"password":     "SecurePass123!",
		"email":        "claims_test@example.com",
		"given_name":   "Claims",
		"family_name":  "Test",
		"phone_number": "+1234567890",
		"locale":       "en-US",
	}

	attributesJSON, err := json.Marshal(attributes)
	ts.Require().NoError(err, "Failed to marshal user attributes")

	user := testutils.User{
		Type:       "claims-test-person",
		OUID:       ts.ouID,
		Attributes: json.RawMessage(attributesJSON),
	}

	userID, err := testutils.CreateUser(user)
	ts.Require().NoError(err, "Failed to create test user")
	ts.T().Logf("Created test user with ID: %s", userID)

	return userID
}

func (ts *ClaimsParameterTestSuite) createTestAuthenticationFlow() string {
	flow := testutils.Flow{
		Name:     "Claims Test Auth Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "claims_test_auth_flow",
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

	flowID, err := testutils.CreateFlow(flow)
	ts.Require().NoError(err, "Failed to create test authentication flow")
	ts.T().Logf("Created test authentication flow with ID: %s", flowID)

	return flowID
}

func (ts *ClaimsParameterTestSuite) createTestApplication(authFlowID string) string {
	app := map[string]interface{}{
		"name":                      appName,
		"description":               "Application for Claims Parameter integration tests",
		"ouId":                      ts.ouID,
		"authFlowId":                authFlowID,
		"isRegistrationFlowEnabled": false,
		"allowedUserTypes":          []string{"claims-test-person"},
		"inboundAuthConfig": []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":     clientID,
					"clientSecret": clientSecret,
					"redirectUris": []string{redirectURI},
					"grantTypes": []string{
						"authorization_code",
						"refresh_token",
					},
					"responseTypes":           []string{"code"},
					"tokenEndpointAuthMethod": "client_secret_basic",
					"scopes":                  []string{"openid", "profile", "email", "phone"},
					"token": map[string]interface{}{
						"idToken": map[string]interface{}{
							"userAttributes": []string{
								"email", "given_name", "family_name", "phone_number", "locale",
							},
						},
					},
					"scopeClaims": map[string][]string{
						"profile": {"given_name", "family_name", "locale"},
						"email":   {"email"},
						"phone":   {"phone_number"},
					},
				},
			},
		},
	}

	jsonData, err := json.Marshal(app)
	ts.Require().NoError(err, "Failed to marshal application data")

	req, err := http.NewRequest("POST", testServerURL+"/applications", bytes.NewBuffer(jsonData))
	ts.Require().NoError(err, "Failed to create request")
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err, "Failed to create application")
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusCreated, resp.StatusCode, "Failed to create application")

	var respData map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&respData)
	ts.Require().NoError(err, "Failed to parse response")

	appID := respData["id"].(string)
	ts.T().Logf("Created test application with ID: %s", appID)
	return appID
}

func (ts *ClaimsParameterTestSuite) deleteApplication(appID string) {
	req, err := http.NewRequest(
		"DELETE",
		fmt.Sprintf("%s/applications/%s", testServerURL, appID),
		nil,
	)
	if err != nil {
		ts.T().Errorf("Failed to create delete request: %v", err)
		return
	}

	resp, err := ts.client.Do(req)
	if err != nil {
		ts.T().Errorf("Failed to delete application: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		ts.T().Errorf(
			"Failed to delete application. Status: %d, Response: %s",
			resp.StatusCode, string(bodyBytes),
		)
	} else {
		ts.T().Logf("Successfully deleted test application with ID: %s", appID)
	}
}

// Utility functions for the test suite

// getTokenWithClaims performs the authorization code flow with claims parameter
func (ts *ClaimsParameterTestSuite) getTokenWithClaims(
	scope, claimsParam string,
) (string, string, error) {
	// Step 1: Initiate authorization flow with claims parameter
	authzResp, err := testutils.InitiateAuthorizationFlowWithClaims(
		clientID, redirectURI, "code", scope, "test_state", claimsParam,
	)
	if err != nil {
		return "", "", fmt.Errorf("failed to initiate authorization: %w", err)
	}
	defer authzResp.Body.Close()

	location := authzResp.Header.Get("Location")
	if location == "" {
		bodyBytes, _ := io.ReadAll(authzResp.Body)
		return "", "", fmt.Errorf(
			"no Location header in authorization response. Status: %d, Body: %s",
			authzResp.StatusCode, string(bodyBytes),
		)
	}

	// Step 2: Extract auth ID and flow ID
	authID, flowID, err := testutils.ExtractAuthData(location)
	if err != nil {
		return "", "", fmt.Errorf("failed to extract auth ID: %w", err)
	}

	// Step 3: Initiate authentication flow
	initialStep, err := testutils.ExecuteAuthenticationFlow(flowID, nil, "")
	if err != nil {
		return "", "", fmt.Errorf("failed to initiate authentication flow: %w", err)
	}

	// Step 4: Execute authentication flow
	authInputs := map[string]string{
		"username": "claims_test_user",
		"password": "SecurePass123!",
	}
	flowStep, err := testutils.ExecuteAuthenticationFlow(flowID, authInputs, "action_001", initialStep.ChallengeToken)
	if err != nil {
		return "", "", fmt.Errorf("failed to execute authentication flow: %w", err)
	}

	if flowStep.Assertion == "" {
		return "", "", fmt.Errorf("assertion not found in flow step")
	}

	// Step 5: Complete authorization
	authzResponse, err := testutils.CompleteAuthorization(authID, flowStep.Assertion)
	if err != nil {
		return "", "", fmt.Errorf("failed to complete authorization: %w", err)
	}

	// Step 6: Extract authorization code
	code, err := testutils.ExtractAuthorizationCode(authzResponse.RedirectURI)
	if err != nil {
		return "", "", fmt.Errorf("failed to extract authorization code: %w", err)
	}

	// Step 7: Exchange code for token
	tokenResult, err := testutils.RequestToken(
		clientID, clientSecret, code, redirectURI, "authorization_code",
	)
	if err != nil {
		return "", "", fmt.Errorf("failed to request token: %w", err)
	}

	if tokenResult.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf(
			"token request failed with status %d: %s",
			tokenResult.StatusCode, string(tokenResult.Body),
		)
	}

	if tokenResult.Token == nil || tokenResult.Token.AccessToken == "" {
		return "", "", fmt.Errorf("access token not found in response")
	}

	return tokenResult.Token.AccessToken, tokenResult.Token.IDToken, nil
}

// callUserInfo calls the UserInfo endpoint with the given access token
func (ts *ClaimsParameterTestSuite) callUserInfo(accessToken string) (map[string]interface{}, error) {
	req, err := http.NewRequest("GET", testServerURL+"/oauth2/userinfo", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := ts.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf(
			"userinfo request failed with status %d: %s",
			resp.StatusCode, string(bodyBytes),
		)
	}

	var userInfo map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&userInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to decode userinfo response: %w", err)
	}

	return userInfo, nil
}

// decodeIDToken decodes the JWT ID token and returns the payload claims
func (ts *ClaimsParameterTestSuite) decodeIDToken(idToken string) (map[string]interface{}, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format")
	}

	// Decode payload (second part)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWT payload: %w", err)
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JWT payload: %w", err)
	}

	return claims, nil
}

// getTokensWithClaims performs authorization code flow and returns access token and refresh token
func (ts *ClaimsParameterTestSuite) getTokensWithClaims(
	scope, claimsParam string,
) (accessToken, refreshToken string, err error) {
	// Step 1: Initiate authorization flow with claims parameter
	authzResp, err := testutils.InitiateAuthorizationFlowWithClaims(
		clientID, redirectURI, "code", scope, "test_state", claimsParam,
	)
	if err != nil {
		return "", "", fmt.Errorf("failed to initiate authorization: %w", err)
	}
	defer authzResp.Body.Close()

	location := authzResp.Header.Get("Location")
	if location == "" {
		return "", "", fmt.Errorf("no Location header in authorization response")
	}

	// Step 2: Extract auth ID and flow ID
	authID, flowID, err := testutils.ExtractAuthData(location)
	if err != nil {
		return "", "", fmt.Errorf("failed to extract auth ID: %w", err)
	}

	// Step 3: Initiate authentication flow
	initialStep, err := testutils.ExecuteAuthenticationFlow(flowID, nil, "")
	if err != nil {
		return "", "", fmt.Errorf("failed to initiate authentication flow: %w", err)
	}

	// Step 4: Execute authentication flow
	authInputs := map[string]string{
		"username": "claims_test_user",
		"password": "SecurePass123!",
	}
	flowStep, err := testutils.ExecuteAuthenticationFlow(flowID, authInputs, "action_001", initialStep.ChallengeToken)
	if err != nil {
		return "", "", fmt.Errorf("failed to execute authentication flow: %w", err)
	}

	// Step 5: Complete authorization
	authzResponse, err := testutils.CompleteAuthorization(authID, flowStep.Assertion)
	if err != nil {
		return "", "", fmt.Errorf("failed to complete authorization: %w", err)
	}

	// Step 6: Extract authorization code
	code, err := testutils.ExtractAuthorizationCode(authzResponse.RedirectURI)
	if err != nil {
		return "", "", fmt.Errorf("failed to extract authorization code: %w", err)
	}

	// Step 7: Exchange code for tokens
	tokenResult, err := testutils.RequestToken(
		clientID, clientSecret, code, redirectURI, "authorization_code",
	)
	if err != nil {
		return "", "", fmt.Errorf("failed to request token: %w", err)
	}

	if tokenResult.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("token request failed with status %d", tokenResult.StatusCode)
	}

	if tokenResult.Token == nil {
		return "", "", fmt.Errorf("token not found in response")
	}

	return tokenResult.Token.AccessToken, tokenResult.Token.RefreshToken, nil
}

// refreshTokens is a consolidated helper for all refresh token operations
// scope: optional scope for downscoping (empty string for no scope change)
// returnRefreshToken: if true, returns both new access and refresh tokens; if false, returns only access token
func (ts *ClaimsParameterTestSuite) refreshTokens(refreshToken, scope string, returnRefreshToken bool) (string, string, error) {
	formData := url.Values{}
	formData.Set("grant_type", "refresh_token")
	formData.Set("refresh_token", refreshToken)
	if scope != "" {
		formData.Set("scope", scope)
	}

	req, err := http.NewRequest("POST", testServerURL+"/oauth2/token", bytes.NewBufferString(formData.Encode()))
	if err != nil {
		return "", "", fmt.Errorf("failed to create refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)

	resp, err := ts.client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to send refresh request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("refresh request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var tokenResp map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &tokenResp); err != nil {
		return "", "", fmt.Errorf("failed to decode refresh response: %w", err)
	}

	accessToken, _ := tokenResp["access_token"].(string)
	if accessToken == "" {
		return "", "", fmt.Errorf("access_token not found in refresh response")
	}

	if returnRefreshToken {
		newRefreshToken, _ := tokenResp["refresh_token"].(string)
		// If refresh_token is not returned, use the original one
		if newRefreshToken == "" {
			newRefreshToken = refreshToken
		}
		return accessToken, newRefreshToken, nil
	}

	return accessToken, "", nil
}

// Convenience wrappers for common refresh scenarios

// refreshAccessToken uses refresh token to get new access token
func (ts *ClaimsParameterTestSuite) refreshAccessToken(refreshToken string) (string, error) {
	accessToken, _, err := ts.refreshTokens(refreshToken, "", false)
	return accessToken, err
}

// refreshAccessTokenWithScopes uses refresh token with specific scopes (for downscoping tests)
func (ts *ClaimsParameterTestSuite) refreshAccessTokenWithScopes(refreshToken, scope string) (string, error) {
	accessToken, _, err := ts.refreshTokens(refreshToken, scope, false)
	return accessToken, err
}

// refreshAccessTokenComplete returns all tokens including new refresh token (for renew_on_grant scenarios)
func (ts *ClaimsParameterTestSuite) refreshAccessTokenComplete(refreshToken string) (string, string, error) {
	return ts.refreshTokens(refreshToken, "", true)
}

// TestClaimsParameter_UserInfo_BasicClaim tests basic claims parameter for UserInfo
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_UserInfo_BasicClaim() {
	// Request email claim explicitly via claims parameter
	claimsParam := `{"userinfo":{"email":null}}`

	accessToken, _, err := ts.getTokenWithClaims("openid", claimsParam)
	ts.Require().NoError(err, "Failed to get token with claims parameter")
	ts.Require().NotEmpty(accessToken, "Access token should not be empty")

	// Call UserInfo endpoint
	userInfo, err := ts.callUserInfo(accessToken)
	ts.Require().NoError(err, "Failed to call UserInfo endpoint")

	// Verify response contains sub claim
	assert.Contains(ts.T(), userInfo, "sub", "Response should contain sub claim")

	// Verify response contains the requested email claim
	assert.Contains(ts.T(), userInfo, "email", "Response should contain email claim")
	assert.Equal(ts.T(), "claims_test@example.com", userInfo["email"], "Email should match")

	ts.T().Logf("UserInfo response: %+v", userInfo)
}

// TestClaimsParameter_IDToken_BasicClaim tests basic claims parameter for ID Token
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_IDToken_BasicClaim() {
	// Request given_name claim explicitly via claims parameter for ID Token
	claimsParam := `{"id_token":{"given_name":null}}`

	_, idToken, err := ts.getTokenWithClaims("openid", claimsParam)
	ts.Require().NoError(err, "Failed to get token with claims parameter")
	ts.Require().NotEmpty(idToken, "ID token should not be empty")

	// Decode ID token
	idTokenClaims, err := ts.decodeIDToken(idToken)
	ts.Require().NoError(err, "Failed to decode ID token")

	// Verify ID token contains the requested given_name claim
	assert.Contains(ts.T(), idTokenClaims, "given_name", "ID Token should contain given_name claim")
	assert.Equal(ts.T(), "Claims", idTokenClaims["given_name"], "given_name should match")

	ts.T().Logf("ID Token claims: %+v", idTokenClaims)
}

// TestClaimsParameter_MultipleClaims tests requesting multiple claims
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_MultipleClaims() {
	// Request multiple claims for both userinfo and id_token
	claimsParam := `{
		"userinfo": {
			"email": null,
			"phone_number": null
		},
		"id_token": {
			"given_name": null,
			"family_name": null
		}
	}`

	accessToken, idToken, err := ts.getTokenWithClaims("openid", claimsParam)
	ts.Require().NoError(err, "Failed to get token with claims parameter")

	// Verify UserInfo claims
	userInfo, err := ts.callUserInfo(accessToken)
	ts.Require().NoError(err, "Failed to call UserInfo endpoint")

	assert.Contains(ts.T(), userInfo, "email", "UserInfo should contain email claim")
	assert.Contains(ts.T(), userInfo, "phone_number", "UserInfo should contain phone_number claim")

	// Verify ID token claims
	idTokenClaims, err := ts.decodeIDToken(idToken)
	ts.Require().NoError(err, "Failed to decode ID token")

	assert.Contains(ts.T(), idTokenClaims, "given_name", "ID Token should contain given_name claim")
	assert.Contains(ts.T(), idTokenClaims, "family_name", "ID Token should contain family_name claim")

	ts.T().Logf("UserInfo response: %+v", userInfo)
	ts.T().Logf("ID Token claims: %+v", idTokenClaims)
}

// TestClaimsParameter_WithScopes tests that claims from scopes and claims parameter are merged
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_WithScopes() {
	// Request phone_number via claims parameter, profile scope will add given_name, family_name
	claimsParam := `{"userinfo":{"phone_number":null}}`

	accessToken, _, err := ts.getTokenWithClaims("openid profile", claimsParam)
	ts.Require().NoError(err, "Failed to get token with claims parameter")

	// Call UserInfo endpoint
	userInfo, err := ts.callUserInfo(accessToken)
	ts.Require().NoError(err, "Failed to call UserInfo endpoint")

	// Verify claims from profile scope
	assert.Contains(ts.T(), userInfo, "given_name", "UserInfo should contain given_name from profile scope")
	assert.Contains(ts.T(), userInfo, "family_name", "UserInfo should contain family_name from profile scope")

	// Verify claim from claims parameter
	assert.Contains(ts.T(), userInfo, "phone_number", "UserInfo should contain phone_number from claims param")

	ts.T().Logf("UserInfo response: %+v", userInfo)
}

// TestClaimsParameter_EssentialClaim tests essential claim marker
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_EssentialClaim() {
	// Request email as essential claim
	claimsParam := `{"userinfo":{"email":{"essential":true}}}`

	accessToken, _, err := ts.getTokenWithClaims("openid", claimsParam)
	ts.Require().NoError(err, "Failed to get token with claims parameter")

	// Call UserInfo endpoint
	userInfo, err := ts.callUserInfo(accessToken)
	ts.Require().NoError(err, "Failed to call UserInfo endpoint")

	// Essential claims should still be returned (even though user has the value)
	assert.Contains(ts.T(), userInfo, "email", "UserInfo should contain essential email claim")

	ts.T().Logf("UserInfo response: %+v", userInfo)
}

// TestClaimsParameter_ValueConstraint tests value constraint in claims parameter
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_ValueConstraint() {
	// Request email with specific value constraint that matches user's email
	claimsParam := `{"userinfo":{"email":{"value":"claims_test@example.com"}}}`

	accessToken, _, err := ts.getTokenWithClaims("openid", claimsParam)
	ts.Require().NoError(err, "Failed to get token with claims parameter")

	// Call UserInfo endpoint
	userInfo, err := ts.callUserInfo(accessToken)
	ts.Require().NoError(err, "Failed to call UserInfo endpoint")

	// Email should be returned since value matches
	assert.Contains(ts.T(), userInfo, "email", "UserInfo should contain email when value matches")
	assert.Equal(ts.T(), "claims_test@example.com", userInfo["email"], "Email value should match")

	ts.T().Logf("UserInfo response: %+v", userInfo)
}

// TestClaimsParameter_ValueConstraint_Mismatch tests that value constraint mismatch excludes claim
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_ValueConstraint_Mismatch() {
	// Request email with value constraint that doesn't match
	claimsParam := `{"userinfo":{"email":{"value":"wrong@example.com"}}}`

	accessToken, _, err := ts.getTokenWithClaims("openid", claimsParam)
	ts.Require().NoError(err, "Failed to get token with claims parameter")

	// Call UserInfo endpoint
	userInfo, err := ts.callUserInfo(accessToken)
	ts.Require().NoError(err, "Failed to call UserInfo endpoint")

	// Email should NOT be returned since value doesn't match
	assert.NotContains(
		ts.T(), userInfo, "email",
		"UserInfo should NOT contain email when value doesn't match",
	)

	ts.T().Logf("UserInfo response: %+v", userInfo)
}

// TestClaimsParameter_ValuesConstraint tests values array constraint
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_ValuesConstraint() {
	// Request locale with multiple allowed values
	claimsParam := `{"userinfo":{"locale":{"values":["en-US","en-GB","de-DE"]}}}`

	accessToken, _, err := ts.getTokenWithClaims("openid", claimsParam)
	ts.Require().NoError(err, "Failed to get token with claims parameter")

	// Call UserInfo endpoint
	userInfo, err := ts.callUserInfo(accessToken)
	ts.Require().NoError(err, "Failed to call UserInfo endpoint")

	// Locale should be returned since "en-US" is in the allowed values
	assert.Contains(ts.T(), userInfo, "locale", "UserInfo should contain locale when value is in values array")
	assert.Equal(ts.T(), "en-US", userInfo["locale"], "Locale value should match")

	ts.T().Logf("UserInfo response: %+v", userInfo)
}

// TestClaimsParameter_InvalidJSON tests that invalid claims parameter is rejected
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_InvalidJSON() {
	// Invalid JSON in claims parameter
	claimsParam := `{"userinfo":{"email":invalid}}`

	// Initiate authorization flow with invalid claims parameter
	authzResp, err := testutils.InitiateAuthorizationFlowWithClaims(
		clientID, redirectURI, "code", "openid", "test_state", claimsParam,
	)
	ts.Require().NoError(err, "Failed to initiate authorization")
	defer authzResp.Body.Close()

	location := authzResp.Header.Get("Location")

	// Check if we get an error redirect or error response
	if location != "" {
		// If redirected, check for error parameters in the URL
		err := testutils.ValidateOAuth2ErrorRedirect(location, "invalid_request", "")
		assert.NoError(ts.T(), err, "Should receive invalid_request error for invalid JSON")
	} else {
		// If not redirected, the status should indicate an error
		assert.True(
			ts.T(),
			authzResp.StatusCode >= 400,
			"Should receive error status for invalid JSON claims parameter",
		)
	}
}

// TestClaimsParameter_EmptyClaimsRequest tests behavior with empty claims object
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_EmptyClaimsRequest() {
	// Empty claims request
	claimsParam := `{}`

	accessToken, _, err := ts.getTokenWithClaims("openid profile", claimsParam)
	ts.Require().NoError(err, "Failed to get token with empty claims parameter")

	// Call UserInfo endpoint
	userInfo, err := ts.callUserInfo(accessToken)
	ts.Require().NoError(err, "Failed to call UserInfo endpoint")

	// Should still get claims from scopes
	assert.Contains(ts.T(), userInfo, "sub", "Response should contain sub claim")
	// Profile scope claims should still be present
	assert.Contains(ts.T(), userInfo, "given_name", "UserInfo should contain given_name from profile scope")

	ts.T().Logf("UserInfo response: %+v", userInfo)
}

// TestClaimsParameter_NonConfiguredClaim tests that non-configured claims are not returned
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_NonConfiguredClaim() {
	// Request a claim that's not in the app's user_attributes configuration
	claimsParam := `{"userinfo":{"non_existent_claim":null}}`

	accessToken, _, err := ts.getTokenWithClaims("openid", claimsParam)
	ts.Require().NoError(err, "Failed to get token with claims parameter")

	// Call UserInfo endpoint
	userInfo, err := ts.callUserInfo(accessToken)
	ts.Require().NoError(err, "Failed to call UserInfo endpoint")

	// non_existent_claim should NOT be returned
	assert.NotContains(
		ts.T(), userInfo, "non_existent_claim",
		"UserInfo should NOT contain non-configured claim",
	)

	ts.T().Logf("UserInfo response: %+v", userInfo)
}

// TestClaimsParameter_IDToken_EssentialClaim tests essential claim marker in ID Token
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_IDToken_EssentialClaim() {
	// Request given_name as essential claim in ID Token
	claimsParam := `{"id_token":{"given_name":{"essential":true}}}`

	_, idToken, err := ts.getTokenWithClaims("openid", claimsParam)
	ts.Require().NoError(err, "Failed to get token with claims parameter")

	// Decode ID token
	idTokenClaims, err := ts.decodeIDToken(idToken)
	ts.Require().NoError(err, "Failed to decode ID token")

	// Essential claims should be returned in ID Token
	assert.Contains(ts.T(), idTokenClaims, "given_name", "ID Token should contain essential given_name claim")
	assert.Equal(ts.T(), "Claims", idTokenClaims["given_name"], "given_name should match")

	ts.T().Logf("ID Token claims: %+v", idTokenClaims)
}

// TestClaimsParameter_IDToken_ValueConstraint tests value constraint in ID Token claims
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_IDToken_ValueConstraint() {
	// Request email with specific value constraint that matches in ID Token
	claimsParam := `{"id_token":{"email":{"value":"claims_test@example.com"}}}`

	_, idToken, err := ts.getTokenWithClaims("openid", claimsParam)
	ts.Require().NoError(err, "Failed to get token with claims parameter")

	// Decode ID token
	idTokenClaims, err := ts.decodeIDToken(idToken)
	ts.Require().NoError(err, "Failed to decode ID token")

	// Email should be returned since value matches
	assert.Contains(ts.T(), idTokenClaims, "email", "ID Token should contain email when value matches")
	assert.Equal(ts.T(), "claims_test@example.com", idTokenClaims["email"], "Email value should match")

	ts.T().Logf("ID Token claims: %+v", idTokenClaims)
}

// TestClaimsParameter_IDToken_ValueConstraint_Mismatch tests value mismatch excludes claim from ID Token
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_IDToken_ValueConstraint_Mismatch() {
	// Request email with value constraint that doesn't match in ID Token
	claimsParam := `{"id_token":{"email":{"value":"wrong@example.com"}}}`

	_, idToken, err := ts.getTokenWithClaims("openid", claimsParam)
	ts.Require().NoError(err, "Failed to get token with claims parameter")

	// Decode ID token
	idTokenClaims, err := ts.decodeIDToken(idToken)
	ts.Require().NoError(err, "Failed to decode ID token")

	// Email should NOT be returned since value doesn't match
	assert.NotContains(ts.T(), idTokenClaims, "email",
		"ID Token should NOT contain email when value doesn't match")

	ts.T().Logf("ID Token claims: %+v", idTokenClaims)
}

// TestClaimsParameter_IDToken_ValuesConstraint tests values array constraint in ID Token
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_IDToken_ValuesConstraint() {
	// Request locale with multiple allowed values in ID Token
	claimsParam := `{"id_token":{"locale":{"values":["en-US","en-GB","de-DE"]}}}`

	_, idToken, err := ts.getTokenWithClaims("openid", claimsParam)
	ts.Require().NoError(err, "Failed to get token with claims parameter")

	// Decode ID token
	idTokenClaims, err := ts.decodeIDToken(idToken)
	ts.Require().NoError(err, "Failed to decode ID token")

	// Locale should be returned since "en-US" is in the allowed values
	assert.Contains(ts.T(), idTokenClaims, "locale",
		"ID Token should contain locale when value is in values array")
	assert.Equal(ts.T(), "en-US", idTokenClaims["locale"], "Locale value should match")

	ts.T().Logf("ID Token claims: %+v", idTokenClaims)
}

// TestClaimsParameter_IDToken_ValuesConstraint_Mismatch tests values array mismatch in ID Token
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_IDToken_ValuesConstraint_Mismatch() {
	// Request locale with values array that doesn't include user's locale
	claimsParam := `{"id_token":{"locale":{"values":["fr-FR","de-DE","es-ES"]}}}`

	_, idToken, err := ts.getTokenWithClaims("openid", claimsParam)
	ts.Require().NoError(err, "Failed to get token with claims parameter")

	// Decode ID token
	idTokenClaims, err := ts.decodeIDToken(idToken)
	ts.Require().NoError(err, "Failed to decode ID token")

	// Locale should NOT be returned since "en-US" is not in the allowed values
	assert.NotContains(ts.T(), idTokenClaims, "locale",
		"ID Token should NOT contain locale when value is not in values array")

	ts.T().Logf("ID Token claims: %+v", idTokenClaims)
}

// TestClaimsParameter_BothIDTokenAndUserInfo_SameClaim tests same claim requested for both endpoints
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_BothIDTokenAndUserInfo_SameClaim() {
	// Request email for both id_token and userinfo
	claimsParam := `{
		"id_token": {"email": null},
		"userinfo": {"email": null}
	}`

	accessToken, idToken, err := ts.getTokenWithClaims("openid", claimsParam)
	ts.Require().NoError(err, "Failed to get token with claims parameter")

	// Verify ID Token contains email
	idTokenClaims, err := ts.decodeIDToken(idToken)
	ts.Require().NoError(err, "Failed to decode ID token")
	assert.Contains(ts.T(), idTokenClaims, "email", "ID Token should contain email claim")
	assert.Equal(ts.T(), "claims_test@example.com", idTokenClaims["email"], "ID Token email should match")

	// Verify UserInfo contains email
	userInfo, err := ts.callUserInfo(accessToken)
	ts.Require().NoError(err, "Failed to call UserInfo endpoint")
	assert.Contains(ts.T(), userInfo, "email", "UserInfo should contain email claim")
	assert.Equal(ts.T(), "claims_test@example.com", userInfo["email"], "UserInfo email should match")

	ts.T().Logf("ID Token claims: %+v", idTokenClaims)
	ts.T().Logf("UserInfo response: %+v", userInfo)
}

// TestClaimsParameter_DifferentClaimsForIDTokenAndUserInfo tests different claims for each endpoint
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_DifferentClaimsForIDTokenAndUserInfo() {
	// Request different claims for id_token and userinfo
	claimsParam := `{
		"id_token": {"given_name": null, "family_name": null},
		"userinfo": {"email": null, "phone_number": null}
	}`

	accessToken, idToken, err := ts.getTokenWithClaims("openid", claimsParam)
	ts.Require().NoError(err, "Failed to get token with claims parameter")

	// Verify ID Token contains only requested claims (not email/phone)
	idTokenClaims, err := ts.decodeIDToken(idToken)
	ts.Require().NoError(err, "Failed to decode ID token")
	assert.Contains(ts.T(), idTokenClaims, "given_name", "ID Token should contain given_name")
	assert.Contains(ts.T(), idTokenClaims, "family_name", "ID Token should contain family_name")
	// Note: email/phone might not be in ID token since they weren't requested for id_token

	// Verify UserInfo contains only requested claims (not given_name/family_name unless from scope)
	userInfo, err := ts.callUserInfo(accessToken)
	ts.Require().NoError(err, "Failed to call UserInfo endpoint")
	assert.Contains(ts.T(), userInfo, "email", "UserInfo should contain email")
	assert.Contains(ts.T(), userInfo, "phone_number", "UserInfo should contain phone_number")

	ts.T().Logf("ID Token claims: %+v", idTokenClaims)
	ts.T().Logf("UserInfo response: %+v", userInfo)
}

// TestClaimsParameter_AllClaimsWithEssential tests essential flag with multiple claims
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_AllClaimsWithEssential() {
	// Request multiple claims with essential flag
	claimsParam := `{
		"userinfo": {
			"email": {"essential": true},
			"phone_number": {"essential": true},
			"locale": {"essential": false}
		}
	}`

	accessToken, _, err := ts.getTokenWithClaims("openid", claimsParam)
	ts.Require().NoError(err, "Failed to get token with claims parameter")

	// Call UserInfo endpoint
	userInfo, err := ts.callUserInfo(accessToken)
	ts.Require().NoError(err, "Failed to call UserInfo endpoint")

	// All claims should be returned (user has all values)
	assert.Contains(ts.T(), userInfo, "email", "UserInfo should contain email")
	assert.Contains(ts.T(), userInfo, "phone_number", "UserInfo should contain phone_number")
	assert.Contains(ts.T(), userInfo, "locale", "UserInfo should contain locale")

	ts.T().Logf("UserInfo response: %+v", userInfo)
}

// TestClaimsParameter_MixedConstraints tests combining essential, value, and values constraints
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_MixedConstraints() {
	// Mix different constraint types
	claimsParam := `{
		"userinfo": {
			"email": {"essential": true, "value": "claims_test@example.com"},
			"locale": {"values": ["en-US", "en-GB"]},
			"phone_number": null
		}
	}`

	accessToken, _, err := ts.getTokenWithClaims("openid", claimsParam)
	ts.Require().NoError(err, "Failed to get token with claims parameter")

	// Call UserInfo endpoint
	userInfo, err := ts.callUserInfo(accessToken)
	ts.Require().NoError(err, "Failed to call UserInfo endpoint")

	// All claims should be returned since all constraints are satisfied
	assert.Contains(ts.T(), userInfo, "email", "UserInfo should contain email (essential + value match)")
	assert.Equal(ts.T(), "claims_test@example.com", userInfo["email"])
	assert.Contains(ts.T(), userInfo, "locale", "UserInfo should contain locale (values match)")
	assert.Equal(ts.T(), "en-US", userInfo["locale"])
	assert.Contains(ts.T(), userInfo, "phone_number", "UserInfo should contain phone_number (no constraint)")

	ts.T().Logf("UserInfo response: %+v", userInfo)
}

// TestClaimsParameter_OnlyIDTokenClaims tests requesting claims only for ID Token
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_OnlyIDTokenClaims() {
	// Request claims only for id_token, userinfo should be empty in claims param
	claimsParam := `{"id_token":{"email":null,"given_name":null}}`

	accessToken, idToken, err := ts.getTokenWithClaims("openid", claimsParam)
	ts.Require().NoError(err, "Failed to get token with claims parameter")

	// Verify ID Token contains requested claims
	idTokenClaims, err := ts.decodeIDToken(idToken)
	ts.Require().NoError(err, "Failed to decode ID token")
	assert.Contains(ts.T(), idTokenClaims, "email", "ID Token should contain email")
	assert.Contains(ts.T(), idTokenClaims, "given_name", "ID Token should contain given_name")

	// UserInfo should only return sub (no explicit userinfo claims requested)
	userInfo, err := ts.callUserInfo(accessToken)
	ts.Require().NoError(err, "Failed to call UserInfo endpoint")
	assert.Contains(ts.T(), userInfo, "sub", "UserInfo should contain sub")
	// Without scope claims, only sub should be present
	// Note: This depends on whether scope adds claims

	ts.T().Logf("ID Token claims: %+v", idTokenClaims)
	ts.T().Logf("UserInfo response: %+v", userInfo)
}

// TestClaimsParameter_OnlyUserInfoClaims tests requesting claims only for UserInfo
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_OnlyUserInfoClaims() {
	// Request claims only for userinfo, id_token should have no explicit claims
	claimsParam := `{"userinfo":{"email":null,"phone_number":null}}`

	accessToken, idToken, err := ts.getTokenWithClaims("openid", claimsParam)
	ts.Require().NoError(err, "Failed to get token with claims parameter")

	// Verify ID Token contains only standard claims (no explicit claims)
	idTokenClaims, err := ts.decodeIDToken(idToken)
	ts.Require().NoError(err, "Failed to decode ID token")
	// ID token should have standard claims but not the userinfo-specific ones
	assert.Contains(ts.T(), idTokenClaims, "sub", "ID Token should contain sub")
	assert.Contains(ts.T(), idTokenClaims, "iss", "ID Token should contain iss")
	assert.Contains(ts.T(), idTokenClaims, "aud", "ID Token should contain aud")

	// UserInfo should contain requested claims
	userInfo, err := ts.callUserInfo(accessToken)
	ts.Require().NoError(err, "Failed to call UserInfo endpoint")
	assert.Contains(ts.T(), userInfo, "email", "UserInfo should contain email")
	assert.Contains(ts.T(), userInfo, "phone_number", "UserInfo should contain phone_number")

	ts.T().Logf("ID Token claims: %+v", idTokenClaims)
	ts.T().Logf("UserInfo response: %+v", userInfo)
}

// TestClaimsParameter_ScopeAndClaimsParamCombination tests that explicit claims parameter
// takes precedence over scope-based claims when value constraints don't match
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_ScopeAndClaimsParamCombination() {
	// Request email via both scope and claims parameter with value constraint that doesn't match
	// The explicit claims parameter should take precedence over scope
	// Value constraint filters the claim
	claimsParam := `{"userinfo":{"email":{"value":"wrong@example.com"}}}`

	accessToken, _, err := ts.getTokenWithClaims("openid email", claimsParam)
	ts.Require().NoError(err, "Failed to get token with claims parameter")

	// Call UserInfo endpoint
	userInfo, err := ts.callUserInfo(accessToken)
	ts.Require().NoError(err, "Failed to call UserInfo endpoint")

	// Email should NOT be returned because explicit value constraint doesn't match
	// Explicit claims parameter takes precedence over scope-based claims
	assert.NotContains(ts.T(), userInfo, "email",
		"UserInfo should NOT contain email when explicit value constraint doesn't match, even with email scope")

	ts.T().Logf("UserInfo response: %+v", userInfo)
}

// TestClaimsParameter_IDToken_ScopeAndClaimsParamCombination tests that explicit claims parameter
// takes precedence over scope-based claims in ID Token when value constraints don't match
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_IDToken_ScopeAndClaimsParamCombination() {
	// Request email via both scope and claims parameter with value constraint that doesn't match
	// The explicit claims parameter should take precedence over scope for ID Token
	// Value constraint filters the claim
	claimsParam := `{"id_token":{"email":{"value":"wrong@example.com"}}}`

	_, idToken, err := ts.getTokenWithClaims("openid email", claimsParam)
	ts.Require().NoError(err, "Failed to get token with claims parameter")

	// Decode ID token
	idTokenClaims, err := ts.decodeIDToken(idToken)
	ts.Require().NoError(err, "Failed to decode ID token")

	// Email should NOT be returned because explicit value constraint doesn't match
	// Explicit claims parameter takes precedence over scope-based claims
	assert.NotContains(ts.T(), idTokenClaims, "email",
		"ID Token should NOT contain email when explicit value constraint doesn't match, even with email scope")

	ts.T().Logf("ID Token claims: %+v", idTokenClaims)
}

// TestClaimsParameter_EmptyUserInfoAndIDToken tests empty objects for both sections
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_EmptyUserInfoAndIDToken() {
	// Empty claims for both id_token and userinfo
	claimsParam := `{"id_token":{},"userinfo":{}}`

	accessToken, idToken, err := ts.getTokenWithClaims("openid profile", claimsParam)
	ts.Require().NoError(err, "Failed to get token with empty claims objects")

	// ID Token should still have standard claims
	idTokenClaims, err := ts.decodeIDToken(idToken)
	ts.Require().NoError(err, "Failed to decode ID token")
	assert.Contains(ts.T(), idTokenClaims, "sub", "ID Token should contain sub")

	// UserInfo should have claims from profile scope
	userInfo, err := ts.callUserInfo(accessToken)
	ts.Require().NoError(err, "Failed to call UserInfo endpoint")
	assert.Contains(ts.T(), userInfo, "sub", "UserInfo should contain sub")
	assert.Contains(ts.T(), userInfo, "given_name", "UserInfo should contain given_name from profile scope")

	ts.T().Logf("ID Token claims: %+v", idTokenClaims)
	ts.T().Logf("UserInfo response: %+v", userInfo)
}

// TestClaimsParameter_AllConfiguredClaims tests requesting all configured claims at once
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_AllConfiguredClaims() {
	// Request all claims that are configured in the application
	claimsParam := `{
		"userinfo": {
			"email": null,
			"given_name": null,
			"family_name": null,
			"phone_number": null,
			"locale": null
		},
		"id_token": {
			"email": null,
			"given_name": null,
			"family_name": null,
			"phone_number": null,
			"locale": null
		}
	}`

	accessToken, idToken, err := ts.getTokenWithClaims("openid", claimsParam)
	ts.Require().NoError(err, "Failed to get token with all claims")

	// Verify all claims in ID Token
	idTokenClaims, err := ts.decodeIDToken(idToken)
	ts.Require().NoError(err, "Failed to decode ID token")
	assert.Contains(ts.T(), idTokenClaims, "email")
	assert.Contains(ts.T(), idTokenClaims, "given_name")
	assert.Contains(ts.T(), idTokenClaims, "family_name")
	assert.Contains(ts.T(), idTokenClaims, "phone_number")
	assert.Contains(ts.T(), idTokenClaims, "locale")

	// Verify all claims in UserInfo
	userInfo, err := ts.callUserInfo(accessToken)
	ts.Require().NoError(err, "Failed to call UserInfo endpoint")
	assert.Contains(ts.T(), userInfo, "email")
	assert.Contains(ts.T(), userInfo, "given_name")
	assert.Contains(ts.T(), userInfo, "family_name")
	assert.Contains(ts.T(), userInfo, "phone_number")
	assert.Contains(ts.T(), userInfo, "locale")

	// Verify values are correct
	assert.Equal(ts.T(), "claims_test@example.com", userInfo["email"])
	assert.Equal(ts.T(), "Claims", userInfo["given_name"])
	assert.Equal(ts.T(), "Test", userInfo["family_name"])
	assert.Equal(ts.T(), "+1234567890", userInfo["phone_number"])
	assert.Equal(ts.T(), "en-US", userInfo["locale"])

	ts.T().Logf("ID Token claims: %+v", idTokenClaims)
	ts.T().Logf("UserInfo response: %+v", userInfo)
}

// TestClaimsParameter_UserInfoValuesConstraint_Mismatch tests values array mismatch in UserInfo
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_UserInfoValuesConstraint_Mismatch() {
	// Request locale with values array that doesn't include user's locale
	claimsParam := `{"userinfo":{"locale":{"values":["fr-FR","de-DE","es-ES"]}}}`

	accessToken, _, err := ts.getTokenWithClaims("openid", claimsParam)
	ts.Require().NoError(err, "Failed to get token with claims parameter")

	// Call UserInfo endpoint
	userInfo, err := ts.callUserInfo(accessToken)
	ts.Require().NoError(err, "Failed to call UserInfo endpoint")

	// Locale should NOT be returned since "en-US" is not in the allowed values
	assert.NotContains(ts.T(), userInfo, "locale",
		"UserInfo should NOT contain locale when value is not in values array")

	ts.T().Logf("UserInfo response: %+v", userInfo)
}

// TestClaimsParameter_IDToken_NonConfiguredClaim tests non-configured claim in ID Token
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_IDToken_NonConfiguredClaim() {
	// Request a claim that's not in the app's user_attributes configuration
	claimsParam := `{"id_token":{"non_existent_claim":null}}`

	_, idToken, err := ts.getTokenWithClaims("openid", claimsParam)
	ts.Require().NoError(err, "Failed to get token with claims parameter")

	// Decode ID token
	idTokenClaims, err := ts.decodeIDToken(idToken)
	ts.Require().NoError(err, "Failed to decode ID token")

	// non_existent_claim should NOT be in ID Token
	assert.NotContains(ts.T(), idTokenClaims, "non_existent_claim",
		"ID Token should NOT contain non-configured claim")

	ts.T().Logf("ID Token claims: %+v", idTokenClaims)
}

// TestClaimsParameter_EssentialFalse tests essential:false behavior
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_EssentialFalse() {
	// Request claim with essential:false (claim is voluntary)
	claimsParam := `{"userinfo":{"email":{"essential":false}}}`

	accessToken, _, err := ts.getTokenWithClaims("openid", claimsParam)
	ts.Require().NoError(err, "Failed to get token with claims parameter")

	// Call UserInfo endpoint
	userInfo, err := ts.callUserInfo(accessToken)
	ts.Require().NoError(err, "Failed to call UserInfo endpoint")

	// Claim should still be returned if available (essential:false just means voluntary)
	assert.Contains(ts.T(), userInfo, "email", "UserInfo should contain email even with essential:false")

	ts.T().Logf("UserInfo response: %+v", userInfo)
}

// TestClaimsParameter_IDToken_WithProfileScope tests ID Token claims with profile scope
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_IDToken_WithProfileScope() {
	// Request phone_number via claims parameter, profile scope adds given_name, family_name
	claimsParam := `{"id_token":{"phone_number":null}}`

	_, idToken, err := ts.getTokenWithClaims("openid profile", claimsParam)
	ts.Require().NoError(err, "Failed to get token with claims parameter")

	// Decode ID token
	idTokenClaims, err := ts.decodeIDToken(idToken)
	ts.Require().NoError(err, "Failed to decode ID token")

	// Verify claims from profile scope in ID Token
	assert.Contains(ts.T(), idTokenClaims, "given_name", "ID Token should contain given_name from profile scope")
	assert.Contains(ts.T(), idTokenClaims, "family_name", "ID Token should contain family_name from profile scope")

	// Verify claim from claims parameter
	assert.Contains(ts.T(), idTokenClaims, "phone_number",
		"ID Token should contain phone_number from claims param")

	ts.T().Logf("ID Token claims: %+v", idTokenClaims)
}

// TestClaimsParameter_ValueAndEssentialCombined tests value with essential constraint
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_ValueAndEssentialCombined() {
	// Request claim with both essential and value constraint (value matches)
	claimsParam := `{"userinfo":{"email":{"essential":true,"value":"claims_test@example.com"}}}`

	accessToken, _, err := ts.getTokenWithClaims("openid", claimsParam)
	ts.Require().NoError(err, "Failed to get token with claims parameter")

	// Call UserInfo endpoint
	userInfo, err := ts.callUserInfo(accessToken)
	ts.Require().NoError(err, "Failed to call UserInfo endpoint")

	// Email should be returned (essential + value matches)
	assert.Contains(ts.T(), userInfo, "email", "UserInfo should contain email")
	assert.Equal(ts.T(), "claims_test@example.com", userInfo["email"])

	ts.T().Logf("UserInfo response: %+v", userInfo)
}

// TestClaimsParameter_ValuesAndEssentialCombined tests values with essential constraint
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_ValuesAndEssentialCombined() {
	// Request claim with both essential and values constraint
	claimsParam := `{"userinfo":{"locale":{"essential":true,"values":["en-US","en-GB"]}}}`

	accessToken, _, err := ts.getTokenWithClaims("openid", claimsParam)
	ts.Require().NoError(err, "Failed to get token with claims parameter")

	// Call UserInfo endpoint
	userInfo, err := ts.callUserInfo(accessToken)
	ts.Require().NoError(err, "Failed to call UserInfo endpoint")

	// Locale should be returned (essential + values contains user's locale)
	assert.Contains(ts.T(), userInfo, "locale", "UserInfo should contain locale")
	assert.Equal(ts.T(), "en-US", userInfo["locale"])

	ts.T().Logf("UserInfo response: %+v", userInfo)
}

// TestClaimsParameter_RefreshToken_PreservesClaimsRequest tests that claims parameter is preserved through refresh token flow
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_RefreshToken_PreservesClaimsRequest() {
	// Request specific claims via claims parameter
	claimsParam := `{"userinfo":{"email":{"essential":true},"phone_number":null},"id_token":{"given_name":null}}`

	accessToken, refreshToken, err := ts.getTokensWithClaims("openid profile", claimsParam)
	ts.Require().NoError(err, "Failed to get initial tokens with claims parameter")
	ts.Require().NotEmpty(refreshToken, "Refresh token should be returned")

	// Verify initial UserInfo contains requested claims
	initialUserInfo, err := ts.callUserInfo(accessToken)
	ts.Require().NoError(err, "Failed to call initial UserInfo endpoint")
	assert.Contains(ts.T(), initialUserInfo, "email", "Initial UserInfo should contain email")
	assert.Contains(ts.T(), initialUserInfo, "phone_number", "Initial UserInfo should contain phone_number")

	// Use refresh token to get new tokens
	newAccessToken, err := ts.refreshAccessToken(refreshToken)
	ts.Require().NoError(err, "Failed to refresh access token")
	ts.Require().NotEmpty(newAccessToken, "New access token should be returned")

	// Verify refreshed UserInfo endpoint still respects original claims parameter
	refreshedUserInfo, err := ts.callUserInfo(newAccessToken)
	ts.Require().NoError(err, "Failed to call refreshed UserInfo endpoint")
	assert.Contains(ts.T(), refreshedUserInfo, "email",
		"Refreshed UserInfo should preserve email from original claims parameter")
	assert.Contains(ts.T(), refreshedUserInfo, "phone_number",
		"Refreshed UserInfo should preserve phone_number from original claims parameter")

	ts.T().Logf("Initial UserInfo: %+v", initialUserInfo)
	ts.T().Logf("Refreshed UserInfo: %+v", refreshedUserInfo)
}

// TestClaimsParameter_RefreshToken_WithoutClaimsParameter tests refresh token without claims parameter
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_RefreshToken_WithoutClaimsParameter() {
	// Get tokens without claims parameter (profile scope adds given_name, family_name)
	accessToken, refreshToken, err := ts.getTokensWithClaims("openid profile", "")
	ts.Require().NoError(err, "Failed to get initial tokens without claims parameter")
	ts.Require().NotEmpty(refreshToken, "Refresh token should be returned")

	// Verify initial UserInfo
	initialUserInfo, err := ts.callUserInfo(accessToken)
	ts.Require().NoError(err, "Failed to call initial UserInfo endpoint")

	// Use refresh token to get new tokens
	newAccessToken, err := ts.refreshAccessToken(refreshToken)
	ts.Require().NoError(err, "Failed to refresh access token")

	refreshedUserInfo, err := ts.callUserInfo(newAccessToken)
	ts.Require().NoError(err, "Failed to call refreshed UserInfo endpoint")

	ts.T().Logf("Initial UserInfo: %+v", initialUserInfo)
	ts.T().Logf("Refreshed UserInfo: %+v", refreshedUserInfo)
}

// TestClaimsParameter_RefreshToken_EssentialClaimPreserved tests essential claims preservation
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_RefreshToken_EssentialClaimPreserved() {
	// Request essential claims
	claimsParam := `{"userinfo":{"email":{"essential":true}},"id_token":{"given_name":{"essential":true}}}`

	_, refreshToken, err := ts.getTokensWithClaims("openid", claimsParam)
	ts.Require().NoError(err, "Failed to get tokens with essential claims")

	// Refresh and verify essential claims are still present
	newAccessToken, err := ts.refreshAccessToken(refreshToken)
	ts.Require().NoError(err, "Failed to refresh access token")

	// Verify UserInfo has essential claim
	userInfo, err := ts.callUserInfo(newAccessToken)
	ts.Require().NoError(err, "Failed to call UserInfo endpoint")
	assert.Contains(ts.T(), userInfo, "email",
		"Refreshed UserInfo should contain essential email claim")

	ts.T().Logf("Refreshed UserInfo: %+v", userInfo)
}

// TestClaimsParameter_RefreshToken_ValueConstraintPreserved tests value constraint preservation
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_RefreshToken_ValueConstraintPreserved() {
	// Request claim with value constraint
	claimsParam := `{"userinfo":{"email":{"value":"claims_test@example.com"}}}`

	_, refreshToken, err := ts.getTokensWithClaims("openid", claimsParam)
	ts.Require().NoError(err, "Failed to get tokens with value constraint")

	// Refresh and verify value constraint is still enforced
	newAccessToken, err := ts.refreshAccessToken(refreshToken)
	ts.Require().NoError(err, "Failed to refresh access token")

	// Verify UserInfo respects value constraint
	userInfo, err := ts.callUserInfo(newAccessToken)
	ts.Require().NoError(err, "Failed to call UserInfo endpoint")
	assert.Contains(ts.T(), userInfo, "email", "UserInfo should contain email")
	assert.Equal(ts.T(), "claims_test@example.com", userInfo["email"],
		"Email should match the value constraint from original claims parameter")

	ts.T().Logf("Refreshed UserInfo: %+v", userInfo)
}

// TestClaimsParameter_RefreshToken_ScopeDownscoping tests refresh with reduced scopes
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_RefreshToken_ScopeDownscoping() {
	// Get tokens with multiple scopes and claims parameter
	claimsParam := `{"id_token":{"given_name":null,"family_name":null},"userinfo":{"email":null,"phone_number":null}}`

	_, refreshToken, err := ts.getTokensWithClaims("openid profile email", claimsParam)
	ts.Require().NoError(err, "Failed to get tokens")

	// Refresh with reduced scopes (remove profile scope, keep openid and email)
	newAccessToken, err := ts.refreshAccessTokenWithScopes(refreshToken, "openid email")
	ts.Require().NoError(err, "Failed to refresh with downscoped scopes")

	// UserInfo should respect claims parameter and downscoped scopes
	userInfo, err := ts.callUserInfo(newAccessToken)
	ts.Require().NoError(err, "Failed to call UserInfo endpoint")
	assert.Contains(ts.T(), userInfo, "email",
		"UserInfo should contain email (from both claims param and email scope)")

	ts.T().Logf("Refreshed UserInfo (downscoped): %+v", userInfo)
}

// TestClaimsParameter_RefreshToken_MultipleRefreshCycles tests claims preservation across multiple refresh cycles
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_RefreshToken_MultipleRefreshCycles() {
	// Request specific claims
	claimsParam := `{"userinfo":{"email":null},"id_token":{"given_name":null}}`

	_, refreshToken1, err := ts.getTokensWithClaims("openid", claimsParam)
	ts.Require().NoError(err, "Failed to get initial tokens")

	// First refresh cycle
	accessToken2, refreshToken2, err := ts.refreshAccessTokenComplete(refreshToken1)
	ts.Require().NoError(err, "Failed first refresh")
	ts.Require().NotEmpty(refreshToken2, "Second refresh token should be returned")

	// Verify first refresh preserves claims
	userInfo2, err := ts.callUserInfo(accessToken2)
	ts.Require().NoError(err, "Failed to call UserInfo after first refresh")
	assert.Contains(ts.T(), userInfo2, "email", "First refresh UserInfo should contain email")

	// Second refresh cycle
	accessToken3, err := ts.refreshAccessToken(refreshToken2)
	ts.Require().NoError(err, "Failed second refresh")

	// Verify second refresh still preserves claims
	userInfo3, err := ts.callUserInfo(accessToken3)
	ts.Require().NoError(err, "Failed to call UserInfo after second refresh")
	assert.Contains(ts.T(), userInfo3, "email",
		"Second refresh UserInfo should still contain email from original claims parameter")

	ts.T().Logf("After first refresh - UserInfo: %+v", userInfo2)
	ts.T().Logf("After second refresh - UserInfo: %+v", userInfo3)
}

// TestClaimsParameter_WithoutOpenIDScope_ShouldIgnore tests claims parameter without openid scope
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_WithoutOpenIDScope_ShouldIgnore() {
	// UserInfo endpoint requires openid scope.
	// Without openid scope, the endpoint returns HTTP 403 with insufficient_scope error.
	claimsParam := `{"userinfo":{"email":null}}`

	// Get access token with scope "profile" (no openid scope)
	accessToken, _, err := ts.getTokenWithClaims("profile", claimsParam)
	ts.Require().NoError(err, "Authorization should succeed without openid scope (OAuth 2.0)")

	// Call UserInfo endpoint - should fail with insufficient_scope
	userInfo, err := ts.callUserInfo(accessToken)
	ts.Require().Error(err, "UserInfo should reject requests without openid scope")
	ts.Require().Nil(userInfo, "UserInfo should not return claims without openid scope")

	// Verify the error indicates HTTP 403 Forbidden
	assert.Contains(ts.T(), err.Error(), "403",
		"UserInfo should return HTTP 403 for missing openid scope")
	assert.Contains(ts.T(), err.Error(), "insufficient_scope",
		"Error should indicate insufficient_scope")

	ts.T().Logf("UserInfo correctly rejected request without openid scope: %v", err)
}

// TestClaimsParameter_VoluntaryClaim_MissingAttribute tests voluntary claim when user doesn't have it
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_VoluntaryClaim_MissingAttribute() {
	// Request claim that user doesn't have (without essential flag)
	// Should be silently omitted if not available
	claimsParam := `{"userinfo":{"missing_voluntary_claim":null}}`

	accessToken, _, err := ts.getTokenWithClaims("openid", claimsParam)
	ts.Require().NoError(err, "Failed to get token with claims parameter")

	// Call UserInfo endpoint
	userInfo, err := ts.callUserInfo(accessToken)
	ts.Require().NoError(err, "Failed to call UserInfo endpoint")

	// Should not contain the missing claim
	assert.NotContains(ts.T(), userInfo, "missing_voluntary_claim",
		"Voluntary claim should be omitted if user doesn't have it")

	// Should still have sub claim
	assert.Contains(ts.T(), userInfo, "sub", "Response should contain sub claim")

	ts.T().Logf("UserInfo response: %+v", userInfo)
}

// TestClaimsParameter_EssentialClaim_MissingAttribute tests essential claim when user doesn't have it
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_EssentialClaim_MissingAttribute() {
	// Request essential claim that user doesn't have
	// Should be omitted (authorization server should act as though not requested)
	claimsParam := `{"userinfo":{"non_existent_essential":{"essential":true}}}`

	accessToken, _, err := ts.getTokenWithClaims("openid", claimsParam)
	ts.Require().NoError(err, "Failed to get token with essential claim request")

	// Call UserInfo endpoint
	userInfo, err := ts.callUserInfo(accessToken)
	ts.Require().NoError(err, "Failed to call UserInfo endpoint")

	// Essential claim should NOT be present if user doesn't have it
	assert.NotContains(ts.T(), userInfo, "non_existent_essential",
		"Essential claim should be omitted if user doesn't have it")

	// Should still have sub claim
	assert.Contains(ts.T(), userInfo, "sub", "Response should contain sub claim")

	ts.T().Logf("UserInfo response: %+v", userInfo)
}

// TestClaimsParameter_BothValueAndValues_InvalidRequest tests invalid claim with both value and values
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_BothValueAndValues_InvalidRequest() {
	// Request with both value and values specified (mutually exclusive per spec)
	// This is invalid and should be rejected
	claimsParam := `{"userinfo":{"email":{"value":"test@example.com","values":["test@example.com","other@example.com"]}}}`

	// Initiate authorization flow with invalid claims parameter
	authzResp, err := testutils.InitiateAuthorizationFlowWithClaims(
		clientID, redirectURI, "code", "openid", "test_state", claimsParam,
	)
	ts.Require().NoError(err, "Failed to initiate authorization")
	defer authzResp.Body.Close()

	// The server SHOULD either:
	// 1. Reject with invalid_request error
	// 2. Accept but ignore one of the constraints (implementation-dependent)

	// Check if we get an error redirect or if it proceeds
	location := authzResp.Header.Get("Location")

	if location != "" && strings.Contains(location, "error=") {
		// If server rejects, verify it's invalid_request
		err := testutils.ValidateOAuth2ErrorRedirect(location, "invalid_request", "")
		assert.NoError(ts.T(), err, "Should receive invalid_request for both value and values")
		ts.T().Log("Server correctly rejected both value and values as invalid_request")
	} else {
		// If server accepts, document that it processed the request
		// (implementation chose to handle gracefully by prioritizing one constraint)
		ts.T().Log("Server accepted both value and values (implementation-specific handling)")
	}
}

// TestClaimsParameter_EmptyValuesArray tests claim with empty values array
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_EmptyValuesArray() {
	// Empty values array is semantically invalid
	// (cannot match "one of zero values") and should be rejected during parsing
	claimsParam := `{"userinfo":{"email":{"values":[]}}}`

	// Initiate authorization - should redirect to error page due to invalid claims parameter
	authzResp, err := testutils.InitiateAuthorizationFlowWithClaims(
		clientID, redirectURI, "code", "openid", "test_state", claimsParam,
	)
	ts.Require().NoError(err, "HTTP request should succeed")
	defer authzResp.Body.Close()

	// Check the redirect location - should be error page, not auth flow
	location := authzResp.Header.Get("Location")
	ts.Require().NotEmpty(location, "Should have Location header")

	// Should redirect to error page (not contain authId)
	ts.Require().NotContains(location, "authId", "Should redirect to error page, not continue auth flow")

	// Error page URL should contain error information
	ts.T().Logf("Redirect location for empty values array: %s", location)
}

// TestClaimsParameter_MutuallyExclusiveValueAndValues tests mutual exclusivity of value and values
func (ts *ClaimsParameterTestSuite) TestClaimsParameter_MutuallyExclusiveValueAndValues() {
	// value and values are mutually exclusive
	claimsParam := `{"userinfo":{"email":{"value":"user@example.com","values":["user1@example.com","user2@example.com"]}}}`

	// Initiate authorization - should redirect to error page due to invalid claims parameter
	authzResp, err := testutils.InitiateAuthorizationFlowWithClaims(
		clientID, redirectURI, "code", "openid", "test_state", claimsParam,
	)
	ts.Require().NoError(err, "HTTP request should succeed")
	defer authzResp.Body.Close()

	// Check the redirect location - should be error page, not auth flow
	location := authzResp.Header.Get("Location")
	ts.Require().NotEmpty(location, "Should have Location header")

	// Should redirect to error page (not contain authId)
	ts.Require().NotContains(location, "authId", "Should redirect to error page, not continue auth flow")

	// Error page URL should contain error information
	ts.T().Logf("Redirect location for mutually exclusive value/values: %s", location)
}
