// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
	invalidScopeTestClientID     = "invalid_scope_test_client"
	invalidScopeTestClientSecret = "invalid_scope_test_secret"
	invalidScopeTestAppName      = "InvalidScopeTestApp"
	invalidScopeTestRedirectURI  = "https://localhost:3000"
	invalidScopeTestUsername     = "invalid_scope_test_user"
	invalidScopeTestPassword     = "InvScopePass123!"
	invalidScopeTestResource     = "https://invalid-scope-rs.example.com"
)

var invalidScopeTestUserType = testutils.UserType{
	Name: "invalid-scope-test-person",
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

var invalidScopeTestAuthFlow = testutils.Flow{
	Name:     "Invalid Scope Test Auth Flow",
	FlowType: "AUTHENTICATION",
	Handle:   "auth_flow_invalid_scope_test",
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

// InvalidScopeTestSuite covers the invalid_scope behavior of the OAuth2 token endpoint across
// the refresh_token, token-exchange, client_credentials grants, and the authorize endpoint.
type InvalidScopeTestSuite struct {
	suite.Suite
	ouID             string
	entityTypeID     string
	authFlowID       string
	userID           string
	resourceServerID string
	roleID           string
	applicationID    string
	client           *http.Client
}

func TestInvalidScopeTestSuite(t *testing.T) {
	suite.Run(t, new(InvalidScopeTestSuite))
}

func (ts *InvalidScopeTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "invalid-scope-test-ou",
		Name:        "Invalid Scope Test OU",
		Description: "Organization unit for invalid scope integration testing",
	})
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

	invalidScopeTestUserType.OUID = ouID
	schemaID, err := testutils.CreateUserType(invalidScopeTestUserType)
	ts.Require().NoError(err, "Failed to create test user type")
	ts.entityTypeID = schemaID

	flowID, err := testutils.CreateFlow(invalidScopeTestAuthFlow)
	ts.Require().NoError(err, "Failed to create test authentication flow")
	ts.authFlowID = flowID

	resourceServerID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:        "Invalid Scope Resource Server",
		Description: "Resource server for invalid scope integration tests",
		Identifier:  invalidScopeTestResource,
		OUID:        ts.ouID,
	}, []testutils.Action{
		{Name: "Read", Handle: "read"},
		{Name: "Write", Handle: "write"},
		{Name: "Admin", Handle: "admin"},
	})
	ts.Require().NoError(err, "Failed to create invalid scope resource server")
	ts.resourceServerID = resourceServerID

	ts.applicationID = ts.createTestApplication()

	user := testutils.User{
		OUID: ouID,
		Type: "invalid-scope-test-person",
		Attributes: json.RawMessage(fmt.Sprintf(`{
			"username": "%s",
			"password": "%s",
			"email": "invalid_scope_test@example.com",
			"given_name": "InvalidScope",
			"family_name": "TestUser"
		}`, invalidScopeTestUsername, invalidScopeTestPassword)),
	}
	userID, err := testutils.CreateUser(user)
	ts.Require().NoError(err, "Failed to create test user")
	ts.userID = userID

	roleID, err := testutils.CreateRole(testutils.Role{
		Name:        "Invalid Scope Test Role",
		Description: "Role granting read and write access for invalid scope tests",
		OUID:        ts.ouID,
		Permissions: []testutils.ResourcePermissions{
			{
				ResourceServerID: ts.resourceServerID,
				Permissions:      []string{"read", "write"},
			},
		},
		Assignments: []testutils.Assignment{
			{ID: ts.userID, Type: "user"},
			{ID: ts.applicationID, Type: "app"},
		},
	})
	ts.Require().NoError(err, "Failed to create test role")
	ts.roleID = roleID
}

func (ts *InvalidScopeTestSuite) createTestApplication() string {
	app := map[string]interface{}{
		"name":                      invalidScopeTestAppName,
		"description":               "Application for invalid scope integration tests",
		"ouId":                      ts.ouID,
		"type":                      "fullstack",
		"authFlowId":                ts.authFlowID,
		"isRegistrationFlowEnabled": false,
		"allowedUserTypes":          []string{"invalid-scope-test-person"},
		"inboundAuthConfig": []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":     invalidScopeTestClientID,
					"clientSecret": invalidScopeTestClientSecret,
					"redirectUris": []string{invalidScopeTestRedirectURI},
					"grantTypes": []string{
						"authorization_code",
						"refresh_token",
						"client_credentials",
						"urn:ietf:params:oauth:grant-type:token-exchange",
					},
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
	ts.T().Logf("Created invalid scope test application with ID: %s", appID)
	return appID
}

func (ts *InvalidScopeTestSuite) TearDownSuite() {
	if ts.roleID != "" {
		if err := testutils.DeleteRole(ts.roleID); err != nil {
			ts.T().Logf("Failed to delete test role: %v", err)
		}
	}

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

	if ts.resourceServerID != "" {
		if err := testutils.DeleteResourceServer(ts.resourceServerID); err != nil {
			ts.T().Logf("Failed to delete resource server: %v", err)
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

// obtainTokensViaAuthCodeFlow performs the complete authorization code flow and returns the token
// response.
func (ts *InvalidScopeTestSuite) obtainTokensViaAuthCodeFlow(scope string) *testutils.TokenResponse {
	resp, err := testutils.InitiateAuthorizationFlowWithResource(
		invalidScopeTestClientID, invalidScopeTestRedirectURI,
		"code", scope, "test-state", invalidScopeTestResource)
	ts.Require().NoError(err, "Failed to initiate authorization flow")
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusFound, resp.StatusCode,
		"Expected redirect status from authorization endpoint")

	location := resp.Header.Get("Location")
	ts.Require().NotEmpty(location, "Expected Location header")

	authID, executionId, err := testutils.ExtractAuthData(location)
	ts.Require().NoError(err, "Failed to extract auth data")

	initialStep, err := testutils.ExecuteAuthenticationFlow(executionId, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")

	flowStep, err := testutils.ExecuteAuthenticationFlow(executionId,
		map[string]string{
			"username": invalidScopeTestUsername,
			"password": invalidScopeTestPassword,
		}, "action_001", initialStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to execute authentication flow")
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus,
		"Authentication flow should complete")
	ts.Require().NotEmpty(flowStep.Assertion, "Assertion should not be empty")

	authzResp, err := testutils.CompleteAuthorization(authID, flowStep.Assertion)
	ts.Require().NoError(err, "Failed to complete authorization")

	code, err := testutils.ExtractAuthorizationCode(authzResp.RedirectURI)
	ts.Require().NoError(err, "Failed to extract authorization code")

	tokenResult, err := testutils.RequestTokenWithResource(
		invalidScopeTestClientID, invalidScopeTestClientSecret,
		code, invalidScopeTestRedirectURI, "authorization_code", invalidScopeTestResource)
	ts.Require().NoError(err, "Failed to request token")
	ts.Require().Equal(http.StatusOK, tokenResult.StatusCode,
		"Token request should succeed. Response: %s", string(tokenResult.Body))
	ts.Require().NotNil(tokenResult.Token, "Token should not be nil")
	ts.Require().NotEmpty(tokenResult.Token.AccessToken, "Access token should not be empty")

	return tokenResult.Token
}

// getUserAssertion authenticates the test user via the Direct Auth API and returns the resulting
// scopeless assertion.
func (ts *InvalidScopeTestSuite) getUserAssertion() string {
	authRequest := map[string]interface{}{
		"identifiers": map[string]interface{}{
			"username": invalidScopeTestUsername,
		},
		"credentials": map[string]interface{}{
			"password": invalidScopeTestPassword,
		},
	}

	requestJSON, err := json.Marshal(authRequest)
	ts.Require().NoError(err, "Failed to marshal auth request")

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/auth/credentials/authenticate",
		bytes.NewReader(requestJSON))
	ts.Require().NoError(err, "Failed to create auth request")
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err, "Failed to authenticate user")
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusOK, resp.StatusCode, "Authentication failed")

	var authResponse testutils.AuthenticationResponse
	err = json.NewDecoder(resp.Body).Decode(&authResponse)
	ts.Require().NoError(err, "Failed to parse auth response")
	ts.Require().NotEmpty(authResponse.Assertion, "Assertion token should not be empty")

	return authResponse.Assertion
}

// exchangeToken submits a raw token request and returns the decoded JSON body along with the
// HTTP status code, for both success and error responses.
func (ts *InvalidScopeTestSuite) exchangeToken(formBody, authHeader string) (map[string]interface{}, int) {
	req, err := http.NewRequest("POST", testutils.TestServerURL+"/oauth2/token", strings.NewReader(formBody))
	ts.Require().NoError(err, "Failed to create token request")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err, "Failed to send token request")
	defer resp.Body.Close()

	var body map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&body)
	ts.Require().NoError(err, "Failed to parse token response")

	return body, resp.StatusCode
}

// requestClientCredentialsToken requests a client_credentials token bound to the invalid scope
// test resource server, with the given scope.
func (ts *InvalidScopeTestSuite) requestClientCredentialsToken(scope string) (map[string]interface{}, int) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	if scope != "" {
		form.Set("scope", scope)
	}
	form.Set("resource", invalidScopeTestResource)

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/oauth2/token", strings.NewReader(form.Encode()))
	ts.Require().NoError(err, "Failed to create token request")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(invalidScopeTestClientID, invalidScopeTestClientSecret)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err, "Failed to send token request")
	defer resp.Body.Close()

	var body map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&body)
	ts.Require().NoError(err, "Failed to parse token response")

	return body, resp.StatusCode
}

// -----------------------------------------------------------------------------------------------
// Group A: Refresh Token invalid_scope
// -----------------------------------------------------------------------------------------------

func (ts *InvalidScopeTestSuite) TestRefreshToken_ScopeEscalation_ReturnsInvalidScope() {
	tokenResponse := ts.obtainTokensViaAuthCodeFlow("openid read write")
	ts.Require().NotEmpty(tokenResponse.RefreshToken, "Refresh token should not be empty")

	result, err := testutils.RefreshAccessTokenRaw(
		invalidScopeTestClientID, invalidScopeTestClientSecret, tokenResponse.RefreshToken, "read write admin")
	ts.Require().NoError(err, "Refresh token request should not error")
	ts.Require().Equal(http.StatusBadRequest, result.StatusCode)

	var body map[string]interface{}
	ts.Require().NoError(json.Unmarshal(result.Body, &body), "Failed to parse error response")
	ts.Equal("invalid_scope", body["error"])
	ts.Contains(body["error_description"], "exceeds the scope granted")
}

func (ts *InvalidScopeTestSuite) TestRefreshToken_SingleUnauthorizedScope_ReturnsInvalidScope() {
	tokenResponse := ts.obtainTokensViaAuthCodeFlow("openid read write")
	ts.Require().NotEmpty(tokenResponse.RefreshToken, "Refresh token should not be empty")

	result, err := testutils.RefreshAccessTokenRaw(
		invalidScopeTestClientID, invalidScopeTestClientSecret, tokenResponse.RefreshToken, "admin")
	ts.Require().NoError(err, "Refresh token request should not error")
	ts.Require().Equal(http.StatusBadRequest, result.StatusCode)

	var body map[string]interface{}
	ts.Require().NoError(json.Unmarshal(result.Body, &body), "Failed to parse error response")
	ts.Equal("invalid_scope", body["error"])
	ts.Contains(body["error_description"], "exceeds the scope granted")
}

func (ts *InvalidScopeTestSuite) TestRefreshToken_CompletelyUnknownScope_ReturnsInvalidScope() {
	tokenResponse := ts.obtainTokensViaAuthCodeFlow("openid read write")
	ts.Require().NotEmpty(tokenResponse.RefreshToken, "Refresh token should not be empty")

	result, err := testutils.RefreshAccessTokenRaw(
		invalidScopeTestClientID, invalidScopeTestClientSecret, tokenResponse.RefreshToken, "nonexistent_scope")
	ts.Require().NoError(err, "Refresh token request should not error")
	ts.Require().Equal(http.StatusBadRequest, result.StatusCode)

	var body map[string]interface{}
	ts.Require().NoError(json.Unmarshal(result.Body, &body), "Failed to parse error response")
	ts.Equal("invalid_scope", body["error"])
	ts.Contains(body["error_description"], "exceeds the scope granted")
}

func (ts *InvalidScopeTestSuite) TestRefreshToken_SubsetScope_Succeeds() {
	tokenResponse := ts.obtainTokensViaAuthCodeFlow("openid read write")
	ts.Require().NotEmpty(tokenResponse.RefreshToken, "Refresh token should not be empty")

	result, err := testutils.RefreshAccessTokenRaw(
		invalidScopeTestClientID, invalidScopeTestClientSecret, tokenResponse.RefreshToken, "read")
	ts.Require().NoError(err, "Refresh token request should not error")
	ts.Require().Equal(http.StatusOK, result.StatusCode, "Response: %s", string(result.Body))
	ts.Require().NotNil(result.Token, "Token should not be nil")

	ts.ElementsMatch([]string{"read"}, strings.Fields(result.Token.Scope))
}

func (ts *InvalidScopeTestSuite) TestRefreshToken_SameScopes_Succeeds() {
	tokenResponse := ts.obtainTokensViaAuthCodeFlow("openid read write")
	ts.Require().NotEmpty(tokenResponse.RefreshToken, "Refresh token should not be empty")

	result, err := testutils.RefreshAccessTokenRaw(
		invalidScopeTestClientID, invalidScopeTestClientSecret, tokenResponse.RefreshToken, "openid read write")
	ts.Require().NoError(err, "Refresh token request should not error")
	ts.Require().Equal(http.StatusOK, result.StatusCode, "Response: %s", string(result.Body))
	ts.Require().NotNil(result.Token, "Token should not be nil")
	ts.ElementsMatch([]string{"openid", "read", "write"}, strings.Fields(result.Token.Scope))
}

func (ts *InvalidScopeTestSuite) TestRefreshToken_NoScopeParam_GrantsOriginalScopes() {
	tokenResponse := ts.obtainTokensViaAuthCodeFlow("openid read write")
	ts.Require().NotEmpty(tokenResponse.RefreshToken, "Refresh token should not be empty")

	result, err := testutils.RefreshAccessTokenRaw(
		invalidScopeTestClientID, invalidScopeTestClientSecret, tokenResponse.RefreshToken, "")
	ts.Require().NoError(err, "Refresh token request should not error")
	ts.Require().Equal(http.StatusOK, result.StatusCode, "Response: %s", string(result.Body))
	ts.Require().NotNil(result.Token, "Token should not be nil")
	ts.ElementsMatch([]string{"openid", "read", "write"}, strings.Fields(result.Token.Scope))
}

// -----------------------------------------------------------------------------------------------
// Group B: Token Exchange invalid_scope
// -----------------------------------------------------------------------------------------------

func (ts *InvalidScopeTestSuite) TestTokenExchange_ScopelessSubjectToken_WithScope_ReturnsInvalidScope() {
	assertion := ts.getUserAssertion()

	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	formData.Set("subject_token", assertion)
	formData.Set("subject_token_type", "urn:ietf:params:oauth:token-type:jwt")
	formData.Set("scope", "read")
	formData.Set("resource", invalidScopeTestResource)

	authHeader := "Basic " + basicAuth(invalidScopeTestClientID, invalidScopeTestClientSecret)

	body, statusCode := ts.exchangeToken(formData.Encode(), authHeader)
	ts.Equal(http.StatusBadRequest, statusCode)
	ts.Equal("invalid_scope", body["error"])
	ts.Contains(body["error_description"], "Cannot request scopes when the subject token has no scopes")
}

func (ts *InvalidScopeTestSuite) TestTokenExchange_ScopelessSubjectToken_NoScope_Succeeds() {
	assertion := ts.getUserAssertion()

	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	formData.Set("subject_token", assertion)
	formData.Set("subject_token_type", "urn:ietf:params:oauth:token-type:jwt")
	formData.Set("resource", invalidScopeTestResource)

	authHeader := "Basic " + basicAuth(invalidScopeTestClientID, invalidScopeTestClientSecret)

	body, statusCode := ts.exchangeToken(formData.Encode(), authHeader)
	ts.Equal(http.StatusOK, statusCode, "Response: %v", body)
	ts.NotEmpty(body["access_token"])
}

func (ts *InvalidScopeTestSuite) TestTokenExchange_ScopedSubjectToken_ExcessScope_IsSilentlyDropped() {
	tokenResponse := ts.obtainTokensViaAuthCodeFlow("read write")

	formData := url.Values{}
	formData.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	formData.Set("subject_token", tokenResponse.AccessToken)
	formData.Set("subject_token_type", "urn:ietf:params:oauth:token-type:access_token")
	formData.Set("scope", "read write admin")
	formData.Set("resource", invalidScopeTestResource)

	authHeader := "Basic " + basicAuth(invalidScopeTestClientID, invalidScopeTestClientSecret)

	body, statusCode := ts.exchangeToken(formData.Encode(), authHeader)
	ts.Equal(http.StatusOK, statusCode, "Response: %v", body)

	scopeStr, _ := body["scope"].(string)
	ts.ElementsMatch([]string{"read", "write"}, strings.Fields(scopeStr))
}

// -----------------------------------------------------------------------------------------------
// Group C: Client Credentials Silent Drop
// -----------------------------------------------------------------------------------------------

func (ts *InvalidScopeTestSuite) TestClientCredentials_UndefinedScopeOnRS_SilentlyDropped() {
	body, statusCode := ts.requestClientCredentialsToken("read nonexistent_action")
	ts.Equal(http.StatusOK, statusCode, "Response: %v", body)

	scopeStr, _ := body["scope"].(string)
	ts.ElementsMatch([]string{"read"}, strings.Fields(scopeStr))
}

func (ts *InvalidScopeTestSuite) TestClientCredentials_AllUndefinedScopes_DroppedNoError() {
	body, statusCode := ts.requestClientCredentialsToken("fake_one fake_two")
	ts.Equal(http.StatusOK, statusCode, "Response: %v", body)
	ts.NotEmpty(body["access_token"])

	scopeStr, _ := body["scope"].(string)
	ts.Empty(strings.Fields(scopeStr))
}

func (ts *InvalidScopeTestSuite) TestClientCredentials_UnauthorizedScope_SilentlyDropped() {
	body, statusCode := ts.requestClientCredentialsToken("read admin")
	ts.Equal(http.StatusOK, statusCode, "Response: %v", body)

	scopeStr, _ := body["scope"].(string)
	ts.ElementsMatch([]string{"read"}, strings.Fields(scopeStr))
}

// -----------------------------------------------------------------------------------------------
// Group D: Authorize Endpoint Silent Drop
// -----------------------------------------------------------------------------------------------

func (ts *InvalidScopeTestSuite) TestAuthorize_UndefinedScopeOnRS_SilentlyDropped() {
	tokenResponse := ts.obtainTokensViaAuthCodeFlow("openid read nonexistent_action")

	claims, err := testutils.DecodeJWT(tokenResponse.AccessToken)
	ts.Require().NoError(err, "Access token should be a valid JWT")

	scopeStr, _ := claims.Additional["scope"].(string)
	ts.ElementsMatch([]string{"openid", "read"}, strings.Fields(scopeStr))
}

func (ts *InvalidScopeTestSuite) TestAuthorize_AllUndefinedPermissionScopes_OnlyOIDCSurvives() {
	tokenResponse := ts.obtainTokensViaAuthCodeFlow("openid fake_one fake_two")

	claims, err := testutils.DecodeJWT(tokenResponse.AccessToken)
	ts.Require().NoError(err, "Access token should be a valid JWT")

	scopeStr, _ := claims.Additional["scope"].(string)
	ts.ElementsMatch([]string{"openid"}, strings.Fields(scopeStr))
}

func (ts *InvalidScopeTestSuite) TestAuthorize_UnauthorizedScope_SilentlyDropped() {
	tokenResponse := ts.obtainTokensViaAuthCodeFlow("openid read admin")

	claims, err := testutils.DecodeJWT(tokenResponse.AccessToken)
	ts.Require().NoError(err, "Access token should be a valid JWT")

	scopeStr, _ := claims.Additional["scope"].(string)
	ts.ElementsMatch([]string{"openid", "read"}, strings.Fields(scopeStr))
}
