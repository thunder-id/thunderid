/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package authz

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

const (
	clientID     = "authz_test_client_123"
	clientSecret = "authz_test_secret_123"
	appName      = "AuthzTestApp"
	redirectURI  = "https://localhost:3000"
)

// TestCase represents a test case for authorization tests
type TestCase struct {
	Name           string
	ClientID       string
	RedirectURI    string
	ResponseType   string
	Scope          string
	State          string
	Username       string
	Password       string
	ExpectedStatus int
	ExpectedError  string
}

var (
	testOUID       string
	testUserType = testutils.UserType{
		Name: "authz-test-person",
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

	testOU = testutils.OrganizationUnit{
		Handle:      "oauth2-authz-test-ou",
		Name:        "OAuth2 Authorization Test OU",
		Description: "Organization unit for OAuth2 authorization testing",
		Parent:      nil,
	}

	testAuthFlow = testutils.Flow{
		Name:     "Authorization Test Auth Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_authz_test",
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

type AuthzTestSuite struct {
	suite.Suite
	applicationID string
	entityTypeID  string
	authFlowID    string
	client        *http.Client
}

func TestAuthzTestSuite(t *testing.T) {
	suite.Run(t, new(AuthzTestSuite))
}

func (ts *AuthzTestSuite) SetupSuite() {

	ts.client = testutils.GetHTTPClient()

	// Create organization unit for tests
	ouID, err := testutils.CreateOrganizationUnit(testOU)
	if err != nil {
		ts.T().Fatalf("Failed to create test organization unit: %v", err)
	}
	testOUID = ouID

	testUserType.OUID = ouID
	schemaID, err := testutils.CreateUserType(testUserType)
	if err != nil {
		ts.T().Fatalf("Failed to create test user type: %v", err)
	}
	ts.entityTypeID = schemaID

	// Create authentication flow
	flowID, err := testutils.CreateFlow(testAuthFlow)
	ts.Require().NoError(err, "Failed to create authorization test flow")
	ts.authFlowID = flowID

	app := map[string]interface{}{
		"name":                      appName,
		"description":               "Application for authorization integration tests",
		"ouId":                      testOUID,
		"authFlowId":                ts.authFlowID,
		"isRegistrationFlowEnabled": false,
		"allowedUserTypes":          []string{"authz-test-person"},
		"inboundAuthConfig": []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":     clientID,
					"clientSecret": clientSecret,
					"redirectUris": []string{redirectURI},
					"grantTypes": []string{
						"client_credentials",
						"authorization_code",
						"refresh_token",
					},
					"responseTypes": []string{
						"code",
					},
					"tokenEndpointAuthMethod": "client_secret_basic",
				},
			},
		},
	}

	// TODO: Use testutils.CreateApplication
	jsonData, err := json.Marshal(app)
	if err != nil {
		ts.T().Fatalf("Failed to marshal application data: %v", err)
	}

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/applications", bytes.NewBuffer(jsonData))
	if err != nil {
		ts.T().Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	if err != nil {
		ts.T().Fatalf("Failed to create application: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		ts.T().Fatalf("Failed to create application. Status: %d, Response: %s", resp.StatusCode, string(bodyBytes))
	}

	var respData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		ts.T().Fatalf("Failed to parse response: %v", err)
	}

	ts.applicationID = respData["id"].(string)
	ts.T().Logf("Created test application with ID: %s", ts.applicationID)

}

func (ts *AuthzTestSuite) TearDownSuite() {
	if ts.applicationID != "" {
		req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/applications/%s", testutils.TestServerURL, ts.applicationID), nil)
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
			ts.T().Errorf("Failed to delete application. Status: %d", resp.StatusCode)
			return
		}

		ts.T().Logf("Successfully deleted test application with ID: %s", ts.applicationID)
	}

	// Delete test authentication flow
	if ts.authFlowID != "" {
		if err := testutils.DeleteFlow(ts.authFlowID); err != nil {
			ts.T().Errorf("Failed to delete test authentication flow: %v", err)
		}
	}

	// Delete test organization unit
	if testOUID != "" {
		if err := testutils.DeleteOrganizationUnit(testOUID); err != nil {
			ts.T().Logf("Failed to delete test organization unit %s: %v", testOUID, err)
		}
	}

	if ts.entityTypeID != "" {
		err := testutils.DeleteUserType(ts.entityTypeID)
		if err != nil {
			ts.T().Errorf("Failed to delete test user type: %v", err)
		}
	}

}

// TestBasicAuthorizationRequest tests the basic authorization request flow
func (ts *AuthzTestSuite) TestBasicAuthorizationRequest() {
	testCases := []TestCase{
		{
			Name:           "Valid Request",
			ClientID:       clientID,
			RedirectURI:    redirectURI,
			ResponseType:   "code",
			Scope:          "openid",
			State:          "test_state_123",
			ExpectedStatus: http.StatusFound,
		},
		{
			Name:           "Invalid Client ID",
			ClientID:       "invalid_client",
			RedirectURI:    redirectURI,
			ResponseType:   "code",
			Scope:          "openid",
			State:          "test_state_456",
			ExpectedStatus: http.StatusFound,
			ExpectedError:  "invalid_request",
		},
		{
			Name:           "Invalid Response Type",
			ClientID:       clientID,
			RedirectURI:    redirectURI,
			ResponseType:   "invalid_type",
			Scope:          "openid",
			State:          "test_state_789",
			ExpectedStatus: http.StatusFound,
			ExpectedError:  "unsupported_response_type",
		},
		{
			Name:           "Missing Client ID",
			ClientID:       "",
			RedirectURI:    redirectURI,
			ResponseType:   "code",
			Scope:          "openid",
			State:          "test_state_missing_client",
			ExpectedStatus: http.StatusFound,
			ExpectedError:  "invalid_request",
		},
		{
			Name:           "Missing Redirect URI",
			ClientID:       clientID,
			RedirectURI:    "",
			ResponseType:   "code",
			Scope:          "openid",
			State:          "test_state_missing_redirect",
			ExpectedStatus: http.StatusFound,
		},
		{
			Name:           "Missing Response Type",
			ClientID:       clientID,
			RedirectURI:    redirectURI,
			ResponseType:   "",
			Scope:          "openid",
			State:          "test_state_missing_response",
			ExpectedStatus: http.StatusFound,
			ExpectedError:  "invalid_request",
		},
		{
			Name:           "Missing State Parameter",
			ClientID:       clientID,
			RedirectURI:    redirectURI,
			ResponseType:   "code",
			Scope:          "openid",
			State:          "",
			ExpectedStatus: http.StatusFound,
		},
	}

	for _, tc := range testCases {
		ts.Run(tc.Name, func() {
			resp, err := testutils.InitiateAuthorizationFlow(tc.ClientID, tc.RedirectURI,
				tc.ResponseType, "openid", tc.State)
			ts.NoError(err, "Failed to initiate authorization flow")
			defer resp.Body.Close()

			ts.Equal(tc.ExpectedStatus, resp.StatusCode, "Expected status code")

			if tc.ExpectedStatus == http.StatusFound {
				location := resp.Header.Get("Location")
				ts.NotEmpty(location, "Expected redirect location header")
				if tc.ExpectedError != "" {
					err := testutils.ValidateOAuth2ErrorRedirect(location, tc.ExpectedError, "")
					ts.NoError(err, "OAuth2 error redirect validation failed")
				} else {
					authId, executionId, err := testutils.ExtractAuthData(location)
					ts.NoError(err, "Failed to extract auth ID")
					ts.NotEmpty(authId, "authId should be present")
					ts.NotEmpty(executionId, "executionId should be present")
				}
			} else {
				bodyBytes, _ := io.ReadAll(resp.Body)
				ts.T().Logf("Error response body: %s", string(bodyBytes))
			}
		})
	}
}

// TestTokenRequestValidation tests the validation of token request parameters
func (ts *AuthzTestSuite) TestTokenRequestValidation() {
	// Create test user and get authorization code
	username := "token_test_user"
	password := "testpass123"

	user := testutils.User{
		OUID: testOUID,
		Type: "authz-test-person",
		Attributes: json.RawMessage(fmt.Sprintf(`{
			"username": "%s",
			"password": "%s",
			"email": "%s@example.com",
			"given_name": "Test",
			"family_name": "User"
		}`, username, password, username)),
	}
	userID, err := testutils.CreateUser(user)
	ts.NoError(err, "Failed to create test user")
	defer func() {
		if err := testutils.DeleteUser(userID); err != nil {
			ts.T().Logf("Warning: Failed to delete test user: %v", err)
		}
	}()

	// Get a valid authorization code first
	validAuthzCode := initiateAuthorizeFlowAndRetrieveAuthzCode(ts, username, password)
	anotherValidAuthzCode := initiateAuthorizeFlowAndRetrieveAuthzCode(ts, username, password)

	testCases := []struct {
		Name           string
		ClientID       string
		ClientSecret   string
		Code           string
		RedirectURI    string
		GrantType      string
		ExpectedStatus int
		ExpectedError  string
	}{
		{
			Name:           "Missing Authorization Code",
			ClientID:       clientID,
			ClientSecret:   clientSecret,
			Code:           "",
			RedirectURI:    redirectURI,
			GrantType:      "authorization_code",
			ExpectedStatus: http.StatusBadRequest,
			ExpectedError:  "invalid_request",
		},
		{
			Name:           "No Client ID",
			ClientID:       "",
			ClientSecret:   clientSecret,
			Code:           validAuthzCode,
			RedirectURI:    redirectURI,
			GrantType:      "authorization_code",
			ExpectedStatus: http.StatusBadRequest,
			ExpectedError:  "invalid_request",
		},
		{
			Name:           "No Client ID and Secret",
			ClientID:       "",
			ClientSecret:   "",
			Code:           validAuthzCode,
			RedirectURI:    redirectURI,
			GrantType:      "authorization_code",
			ExpectedStatus: http.StatusBadRequest,
			ExpectedError:  "invalid_request",
		},
		{
			Name:           "No Client Secret",
			ClientID:       clientID,
			ClientSecret:   "",
			Code:           validAuthzCode,
			RedirectURI:    redirectURI,
			GrantType:      "authorization_code",
			ExpectedStatus: http.StatusUnauthorized,
			ExpectedError:  "invalid_client",
		},
		{
			Name:           "Invalid Client Credentials",
			ClientID:       clientID,
			ClientSecret:   "wrong_secret",
			Code:           validAuthzCode,
			RedirectURI:    redirectURI,
			GrantType:      "authorization_code",
			ExpectedStatus: http.StatusUnauthorized,
			ExpectedError:  "invalid_client",
		},
		{
			Name:           "Missing Grant Type",
			ClientID:       clientID,
			ClientSecret:   clientSecret,
			Code:           validAuthzCode,
			RedirectURI:    redirectURI,
			GrantType:      "",
			ExpectedStatus: http.StatusBadRequest,
			ExpectedError:  "invalid_request",
		},
		{
			Name:           "Invalid Authorization Code",
			ClientID:       clientID,
			ClientSecret:   clientSecret,
			Code:           "invalid_code_12345",
			RedirectURI:    redirectURI,
			GrantType:      "authorization_code",
			ExpectedStatus: http.StatusBadRequest,
			ExpectedError:  "invalid_grant",
		},
		{
			Name:           "Invalid Client ID",
			ClientID:       "invalid_client_id",
			ClientSecret:   clientSecret,
			Code:           validAuthzCode,
			RedirectURI:    redirectURI,
			GrantType:      "authorization_code",
			ExpectedStatus: http.StatusUnauthorized,
			ExpectedError:  "invalid_client",
		},
		{
			Name:           "Mismatched Redirect URI",
			ClientID:       clientID,
			ClientSecret:   clientSecret,
			Code:           anotherValidAuthzCode,
			RedirectURI:    "https://localhost:3001",
			GrantType:      "authorization_code",
			ExpectedStatus: http.StatusBadRequest,
			ExpectedError:  "invalid_grant",
		},
		{
			Name:           "Used unsuccessful Authz Code",
			ClientID:       clientID,
			ClientSecret:   clientSecret,
			Code:           anotherValidAuthzCode,
			RedirectURI:    redirectURI,
			GrantType:      "authorization_code",
			ExpectedStatus: http.StatusBadRequest,
			ExpectedError:  "invalid_grant",
		},
		{
			Name:           "Invalid Grant Type Format",
			ClientID:       clientID,
			ClientSecret:   clientSecret,
			Code:           validAuthzCode,
			RedirectURI:    redirectURI,
			GrantType:      "invalid_grant_type",
			ExpectedStatus: http.StatusBadRequest,
			ExpectedError:  "unsupported_grant_type",
		},
		{
			Name:           "Valid Token Request",
			ClientID:       clientID,
			ClientSecret:   clientSecret,
			Code:           validAuthzCode,
			RedirectURI:    redirectURI,
			GrantType:      "authorization_code",
			ExpectedStatus: http.StatusOK,
			ExpectedError:  "",
		},
		{
			Name:           "Used successful Authz Code",
			ClientID:       clientID,
			ClientSecret:   clientSecret,
			Code:           validAuthzCode,
			RedirectURI:    redirectURI,
			GrantType:      "authorization_code",
			ExpectedStatus: http.StatusBadRequest,
			ExpectedError:  "invalid_grant",
		},
	}

	for _, tc := range testCases {
		ts.Run(tc.Name, func() {
			result, err := testutils.RequestToken(tc.ClientID, tc.ClientSecret, tc.Code, tc.RedirectURI, tc.GrantType)
			ts.NoError(err, "Token request should not error at transport level")
			ts.Equal(tc.ExpectedStatus, result.StatusCode, "Expected status code")

			if tc.ExpectedStatus == http.StatusOK {
				ts.NotNil(result.Token, "Token payload should be present on success")

				tokenResponse := result.Token
				ts.NotEmpty(tokenResponse.AccessToken, "Access token should be present")
				ts.Equal("Bearer", tokenResponse.TokenType, "Token type should be Bearer")
				ts.True(tokenResponse.ExpiresIn > 0, "Expires in should be greater than 0")

				parts := strings.Split(tokenResponse.AccessToken, ".")
				ts.Len(parts, 3, "Access token should be a JWT with 3 parts")

				payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
				ts.NoError(err, "Failed to decode JWT payload")

				var claims map[string]interface{}
				err = json.Unmarshal(payloadBytes, &claims)
				ts.NoError(err, "Failed to unmarshal JWT claims")

				ts.Equal(tc.ClientID, claims["aud"], "Audience claim should match client_id")
				ts.Equal("openid", claims["scope"], "Scope claim should match requested scope")
				ts.Equal(userID, claims["sub"], "Subject claim should match authenticated user ID")
			} else if tc.ExpectedError != "" {
				var errorResponse map[string]interface{}
				err := json.Unmarshal(result.Body, &errorResponse)
				ts.NoError(err, "Failed to unmarshal error response")
				ts.Contains(errorResponse, "error", "Error response should contain error field")
				ts.Equal(tc.ExpectedError, errorResponse["error"], "Expected error should match")
			}
		})
	}
}

func initiateAuthorizeFlowAndRetrieveAuthzCode(ts *AuthzTestSuite, username string, password string) string {
	resp, err := testutils.InitiateAuthorizationFlow(clientID, redirectURI, "code", "openid", "token_test_state")
	ts.NoError(err, "Failed to initiate authorization flow")
	defer resp.Body.Close()

	ts.Equal(http.StatusFound, resp.StatusCode, "Expected redirect status")
	location := resp.Header.Get("Location")
	authId, executionId, err := testutils.ExtractAuthData(location)
	ts.NoError(err, "Failed to extract auth ID")

	// Initiate authentication flow
	initialStep, err := testutils.ExecuteAuthenticationFlow(executionId, nil, "")
	ts.NoError(err, "Failed to initiate authentication flow")

	// Execute authentication flow
	flowStep, err := testutils.ExecuteAuthenticationFlow(executionId, map[string]string{
		"username": username,
		"password": password,
	}, "action_001", initialStep.ChallengeToken)
	ts.NoError(err, "Failed to execute authentication flow")
	ts.Equal("COMPLETE", flowStep.FlowStatus, "Flow should complete successfully")

	// Complete authorization
	authzResponse, err := testutils.CompleteAuthorization(authId, flowStep.Assertion)
	ts.NoError(err, "Failed to complete authorization")
	validAuthzCode, err := testutils.ExtractAuthorizationCode(authzResponse.RedirectURI)
	ts.NoError(err, "Failed to extract authorization code")
	return validAuthzCode
}

// TestRedirectURIValidation tests the redirect URI validation in OAuth2 flows
func (ts *AuthzTestSuite) TestRedirectURIValidation() {
	testCases := []struct {
		Name           string
		ClientID       string
		RedirectURI    string
		ResponseType   string
		Scope          string
		State          string
		ExpectedStatus int
		ExpectedError  string
		Description    string
	}{
		{
			Name:           "Valid HTTPS Redirect URI",
			ClientID:       clientID,
			RedirectURI:    redirectURI,
			ResponseType:   "code",
			Scope:          "openid",
			State:          "redirect_test_valid_https",
			ExpectedStatus: http.StatusFound,
			Description:    "Standard HTTPS localhost should be valid",
		},
		{
			Name:           "Valid HTTPS with Path",
			ClientID:       clientID,
			RedirectURI:    "https://localhost:3000/callback",
			ResponseType:   "code",
			Scope:          "openid",
			State:          "redirect_test_valid_path",
			ExpectedStatus: http.StatusFound,
			ExpectedError:  "invalid_request",
			Description:    "HTTPS with callback path should be rejected (not registered)",
		},
		{
			Name:           "HTTP Redirect URI",
			ClientID:       clientID,
			RedirectURI:    "http://localhost:3000",
			ResponseType:   "code",
			Scope:          "openid",
			State:          "redirect_test_http",
			ExpectedStatus: http.StatusFound,
			ExpectedError:  "invalid_request",
			Description:    "HTTP should be rejected for security",
		},
		{
			Name:           "Invalid Protocol",
			ClientID:       clientID,
			RedirectURI:    "invalid://localhost:3000",
			ResponseType:   "code",
			Scope:          "openid",
			State:          "redirect_test_invalid_protocol",
			ExpectedStatus: http.StatusFound,
			ExpectedError:  "invalid_request",
			Description:    "Invalid protocol should be rejected",
		},
		{
			Name:           "External Domain",
			ClientID:       clientID,
			RedirectURI:    "https://malicious.com/callback",
			ResponseType:   "code",
			Scope:          "openid",
			State:          "redirect_test_malicious_domain",
			ExpectedStatus: http.StatusFound,
			ExpectedError:  "invalid_request",
			Description:    "External malicious domain should be rejected",
		},
	}

	for _, tc := range testCases {
		ts.Run(tc.Name, func() {
			resp, err := testutils.InitiateAuthorizationFlow(tc.ClientID, tc.RedirectURI, tc.ResponseType,
				tc.Scope, tc.State)
			ts.NoError(err, "Failed to initiate authorization flow")
			defer resp.Body.Close()

			ts.Equal(tc.ExpectedStatus, resp.StatusCode, "Expected status code")

			if tc.ExpectedStatus == http.StatusFound {
				location := resp.Header.Get("Location")
				ts.NotEmpty(location, "Expected redirect location header")

				if tc.ExpectedError != "" {
					if tc.RedirectURI != redirectURI {
						parsedLocation, parseErr := url.Parse(location)
						ts.NoError(parseErr, "Failed to parse redirect location")

						parsedTestURI, parseErr := url.Parse(tc.RedirectURI)
						ts.NoError(parseErr, "Failed to parse test case redirect URI")

						ts.NotEqual(parsedTestURI.Host, parsedLocation.Host,
							"System redirected to invalid domain '%s' instead of authorization server",
							parsedTestURI.Host)
					}

					err := testutils.ValidateOAuth2ErrorRedirect(location, tc.ExpectedError, "")
					ts.NoError(err, "OAuth2 error redirect validation failed")

				} else {
					authId, executionId, err := testutils.ExtractAuthData(location)
					ts.NoError(err, "Failed to extract auth ID")
					ts.NotEmpty(authId, "authId should be present")
					ts.NotEmpty(executionId, "executionId should be present")
				}
			}
		})
	}
}

func (ts *AuthzTestSuite) TestCompleteAuthorizationCodeFlow() {
	testCases := []TestCase{
		{
			Name:         "Successful Flow",
			ClientID:     clientID,
			RedirectURI:  "https://localhost:3000",
			ResponseType: "code",
			Scope:        "openid",
			State:        "test_state_456",
			Username:     "testuser",
			Password:     "testpass123",
		},
	}

	for _, tc := range testCases {
		ts.Run(tc.Name, func() {
			// Create test user with credentials
			user := testutils.User{
				OUID: testOUID,
				Type: "authz-test-person",
				Attributes: json.RawMessage(fmt.Sprintf(`{
					"username": "%s",
					"password": "%s",
					"email": "%s@example.com",
					"given_name": "Test",
					"family_name": "User"
				}`, tc.Username, tc.Password, tc.Username)),
			}
			userID, err := testutils.CreateUser(user)
			if err != nil {
				ts.T().Fatalf("Failed to create test user: %v", err)
			}
			if userID == "" {
				ts.T().Fatalf("Expected user ID, got empty string")
			}

			defer func() {
				if err := testutils.DeleteUser(userID); err != nil {
					ts.T().Logf("Warning: Failed to delete test user: %v", err)
				}
			}()

			// Start authorization flow
			resp, err := testutils.InitiateAuthorizationFlow(tc.ClientID, tc.RedirectURI,
				tc.ResponseType, tc.Scope, tc.State)
			ts.NoError(err, "Failed to initiate authorization flow")
			defer resp.Body.Close()

			ts.Equal(http.StatusFound, resp.StatusCode, "Expected redirect status")

			location := resp.Header.Get("Location")
			ts.NotEmpty(location, "Expected redirect location header")

			// Extract auth ID and execution ID
			authId, executionId, err := testutils.ExtractAuthData(location)
			if err != nil {
				ts.T().Fatalf("Failed to extract auth ID: %v", err)
			}
			if authId == "" {
				ts.T().Fatalf("Expected authId, got empty string")
			}

			// Initiate authentication flow
			initialStep, err := testutils.ExecuteAuthenticationFlow(executionId, nil, "")
			if err != nil {
				ts.T().Fatalf("Failed to initiate authentication flow: %v", err)
			}

			// Execute authentication flow
			flowStep, err := testutils.ExecuteAuthenticationFlow(executionId, map[string]string{
				"username": tc.Username,
				"password": tc.Password,
			}, "action_001", initialStep.ChallengeToken)
			if err != nil {
				ts.T().Fatalf("Failed to execute authentication flow: %v", err)
			}
			if flowStep == nil {
				ts.T().Fatalf("Expected flow step, got nil")
			}

			if flowStep.ExecutionID == "" {
				ts.T().Fatalf("Expected execution ID, got empty string")
			}
			if flowStep.FlowStatus != "COMPLETE" {
				ts.T().Fatalf("Expected flow status COMPLETE, got %s", flowStep.FlowStatus)
			}

			if flowStep.Assertion == "" {
				ts.T().Fatalf("Expected assertion, got empty string")
			}

			// Complete authorization
			authzResponse, err := testutils.CompleteAuthorization(authId, flowStep.Assertion)
			ts.NoError(err, "Failed to complete authorization")
			ts.NotEmpty(authzResponse.RedirectURI, "Redirect URI should be present")

			authzCode, err := testutils.ExtractAuthorizationCode(authzResponse.RedirectURI)
			ts.NoError(err, "Failed to extract authorization code")
			ts.NotEmpty(authzCode, "Authorization code should be present")

			// Exchange authorization code for access token
			result, err := testutils.RequestToken(clientID, clientSecret, authzCode, tc.RedirectURI,
				"authorization_code")
			ts.NoError(err, "Failed to exchange code for token")
			ts.Equal(http.StatusOK, result.StatusCode, "Token request should succeed")
			tokenResponse := result.Token

			// Verify token response
			ts.NotEmpty(tokenResponse.AccessToken, "Access token should be present")
			ts.Equal("Bearer", tokenResponse.TokenType, "Token type should be Bearer")
			ts.True(tokenResponse.ExpiresIn > 0, "Expires in should be greater than 0")

			parts := strings.Split(tokenResponse.AccessToken, ".")
			ts.Len(parts, 3, "Access token should be a JWT with 3 parts")

			payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
			ts.NoError(err, "Failed to decode JWT payload")

			var claims map[string]interface{}
			err = json.Unmarshal(payloadBytes, &claims)
			ts.NoError(err, "Failed to unmarshal JWT claims")

			ts.Equal(tc.ClientID, claims["aud"], "Audience claim should match client_id")
			ts.Equal(tc.Scope, claims["scope"], "Scope claim should match requested scope")
			ts.Equal(userID, claims["sub"], "Subject claim should match authenticated user ID")
		})
	}
}

func (ts *AuthzTestSuite) TestAuthorizationCodeErrorScenarios() {
	testCases := []TestCase{
		{
			Name:           "Reused Authorization Code",
			ClientID:       clientID,
			RedirectURI:    "https://localhost:3000",
			ResponseType:   "code",
			Scope:          "openid",
			State:          "test_state_error",
			Username:       "testuser_error",
			Password:       "testpass123",
			ExpectedStatus: http.StatusBadRequest,
			ExpectedError:  "invalid_grant",
		}}

	for _, tc := range testCases {
		ts.Run(tc.Name, func() {
			// Create test user
			user := testutils.User{
				OUID: testOUID,
				Type: "authz-test-person",
				Attributes: json.RawMessage(fmt.Sprintf(`{
					"username": "%s",
					"password": "%s",
					"email": "%s@example.com",
					"given_name": "Test",
					"family_name": "User"
				}`, tc.Username, tc.Password, tc.Username)),
			}
			userID, err := testutils.CreateUser(user)
			ts.NoError(err, "Failed to create test user")
			defer func() {
				if err := testutils.DeleteUser(userID); err != nil {
					ts.T().Logf("Warning: Failed to delete test user: %v", err)
				}
			}()

			// Start authorization flow
			resp, err := testutils.InitiateAuthorizationFlow(tc.ClientID, tc.RedirectURI, tc.ResponseType, tc.Scope, tc.State)
			ts.NoError(err, "Failed to initiate authorization flow")
			defer resp.Body.Close()

			ts.Equal(http.StatusFound, resp.StatusCode, "Expected redirect status")

			location := resp.Header.Get("Location")
			ts.NotEmpty(location, "Expected redirect location header")

			authId, executionId, err := testutils.ExtractAuthData(location)
			ts.NoError(err, "Failed to extract auth ID")

			// Initiate authentication flow
			initialStep, err := testutils.ExecuteAuthenticationFlow(executionId, nil, "")
			if err != nil {
				ts.T().Fatalf("Failed to initiate authentication flow: %v", err)
			}

			// Execute authentication flow
			flowStep, err := testutils.ExecuteAuthenticationFlow(executionId, map[string]string{
				"username": tc.Username,
				"password": tc.Password,
			}, "action_001", initialStep.ChallengeToken)
			if err != nil {
				ts.T().Fatalf("Failed to execute authentication flow: %v", err)
			}
			if flowStep.FlowStatus != "COMPLETE" {
				ts.T().Fatalf("Expected flow status COMPLETE, got %s", flowStep.FlowStatus)
			}

			authzResponse, err := testutils.CompleteAuthorization(authId, flowStep.Assertion)
			if err != nil {
				ts.T().Fatalf("Failed to complete authorization: %v", err)
			}

			// Extract authorization code
			authzCode, err := testutils.ExtractAuthorizationCode(authzResponse.RedirectURI)
			ts.NoError(err, "Failed to extract authorization code")

			if tc.Name == "Reused Authorization Code" {
				result, err := testutils.RequestToken(clientID, clientSecret, authzCode, tc.RedirectURI, "authorization_code")
				ts.NoError(err, "First token exchange should succeed")
				ts.Equal(http.StatusOK, result.StatusCode, "First token exchange should succeed")

				// Second attempt should fail with invalid_grant
				result2, err := testutils.RequestToken(clientID, clientSecret, authzCode, tc.RedirectURI, "authorization_code")
				ts.NoError(err, "Second token exchange should not error at transport level")
				ts.Equal(http.StatusBadRequest, result2.StatusCode, "Second token exchange should fail with bad request")

				// Check error response
				var errorResponse map[string]interface{}
				err = json.Unmarshal(result2.Body, &errorResponse)
				ts.NoError(err, "Failed to unmarshal error response")
				ts.Equal("invalid_grant", errorResponse["error"], "Expected invalid_grant error")
			}
		})
	}
}

// TestAuthorizationCodeFlowWithResourceParameter tests RFC 8707 resource parameter implementation
func (ts *AuthzTestSuite) TestAuthorizationCodeFlowWithResourceParameter() {
	// Test that resource parameter is properly stored and used as audience in access token
	resourceURL := "https://mcp.example.com/mcp"

	// Create a Resource Server with the matching identifier so the resource parameter is valid
	rs := testutils.ResourceServer{
		Name:       "MCP Resource Server",
		Handle:     "mcp-server",
		Identifier: resourceURL,
		OUID:       testOUID,
	}
	rsID, err := testutils.CreateResourceServerWithActions(rs, nil)
	ts.NoError(err, "Failed to create resource server")
	defer func() {
		if err := testutils.DeleteResourceServer(rsID); err != nil {
			ts.T().Logf("Warning: Failed to delete resource server: %v", err)
		}
	}()

	// Create test user
	user := testutils.User{
		OUID: testOUID,
		Type: "authz-test-person",
		Attributes: json.RawMessage(`{
			"username": "resourcetest",
			"password": "testpass123",
			"email": "resourcetest@example.com",
			"given_name": "Resource",
			"family_name": "Test"
		}`),
	}
	userID, err := testutils.CreateUser(user)
	ts.NoError(err, "Failed to create test user")
	defer func() {
		if err := testutils.DeleteUser(userID); err != nil {
			ts.T().Logf("Warning: Failed to delete test user: %v", err)
		}
	}()

	// Start authorization flow with resource parameter
	resp, err := testutils.InitiateAuthorizationFlowWithResource(
		clientID,
		redirectURI,
		"code",
		"openid",
		"test_resource_state",
		resourceURL,
	)
	ts.NoError(err, "Failed to initiate authorization flow with resource")
	defer resp.Body.Close()

	ts.Equal(http.StatusFound, resp.StatusCode, "Expected redirect status")
	location := resp.Header.Get("Location")
	ts.NotEmpty(location, "Expected redirect location header")

	authId, executionId, err := testutils.ExtractAuthData(location)
	ts.NoError(err, "Failed to extract auth ID")

	// Initiate authentication flow
	initialStep, err := testutils.ExecuteAuthenticationFlow(executionId, nil, "")
	ts.NoError(err, "Failed to initiate authentication flow")

	// Execute authentication flow
	flowStep, err := testutils.ExecuteAuthenticationFlow(executionId, map[string]string{
		"username": "resourcetest",
		"password": "testpass123",
	}, "action_001", initialStep.ChallengeToken)
	ts.NoError(err, "Failed to execute authentication flow")
	ts.Equal("COMPLETE", flowStep.FlowStatus, "Expected flow status COMPLETE")

	// Complete authorization
	authzResponse, err := testutils.CompleteAuthorization(authId, flowStep.Assertion)
	ts.NoError(err, "Failed to complete authorization")

	// Extract authorization code
	authzCode, err := testutils.ExtractAuthorizationCode(authzResponse.RedirectURI)
	ts.NoError(err, "Failed to extract authorization code")

	// Request token with resource parameter
	tokenReq := url.Values{}
	tokenReq.Set("grant_type", "authorization_code")
	tokenReq.Set("code", authzCode)
	tokenReq.Set("redirect_uri", redirectURI)
	tokenReq.Set("resource", resourceURL)

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/oauth2/token",
		bytes.NewBufferString(tokenReq.Encode()))
	ts.NoError(err, "Failed to create token request")

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)

	tokenResp, err := ts.client.Do(req)
	ts.NoError(err, "Failed to send token request")
	defer tokenResp.Body.Close()

	ts.Equal(http.StatusOK, tokenResp.StatusCode, "Token request should succeed")

	var tokenResponse map[string]interface{}
	err = json.NewDecoder(tokenResp.Body).Decode(&tokenResponse)
	ts.NoError(err, "Failed to decode token response")

	// Extract and decode the access token
	accessToken, ok := tokenResponse["access_token"].(string)
	ts.True(ok, "Access token should be present")

	// Decode JWT to verify audience claim
	parts := strings.Split(accessToken, ".")
	ts.Len(parts, 3, "Access token should be a JWT")

	// Decode the payload (second part)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	ts.NoError(err, "Failed to decode JWT payload")

	var claims map[string]interface{}
	err = json.Unmarshal(payload, &claims)
	ts.NoError(err, "Failed to unmarshal JWT claims")

	// Verify the audience claim matches the resource parameter
	aud, ok := claims["aud"]
	ts.True(ok, "Audience claim should be present in access token")
	switch audVal := aud.(type) {
	case string:
		ts.Equal(resourceURL, audVal, "Audience should match the resource parameter")
	case []interface{}:
		found := false
		for _, a := range audVal {
			if a == resourceURL {
				found = true
				break
			}
		}
		ts.True(found, "Audience array should contain the resource URL")
	default:
		ts.Fail("Unexpected audience type")
	}
}

// TestAuthorizationCodeFlowWithClaimsLocales tests that claims_locales parameter is accepted and stored
func (ts *AuthzTestSuite) TestAuthorizationCodeFlowWithClaimsLocales() {
	// Test that claims_locales parameter is properly handled in authorization flow
	claimsLocales := "en-US fr-CA ja"

	// Create test user
	user := testutils.User{
		OUID: testOUID,
		Type: "authz-test-person",
		Attributes: json.RawMessage(`{
			"username": "localestest",
			"password": "testpass123",
			"email": "localestest@example.com",
			"given_name": "Locales",
			"family_name": "Test"
		}`),
	}
	userID, err := testutils.CreateUser(user)
	ts.NoError(err, "Failed to create test user")
	defer func() {
		if err := testutils.DeleteUser(userID); err != nil {
			ts.T().Logf("Warning: Failed to delete test user: %v", err)
		}
	}()

	// Start authorization flow with claims_locales parameter
	resp, err := testutils.InitiateAuthorizationFlowWithClaimsLocales(
		clientID, redirectURI, "code", "openid", "test_locales_state", claimsLocales,
	)
	ts.NoError(err, "Failed to initiate authorization flow with claims_locales")
	if resp == nil {
		return
	}
	defer resp.Body.Close()

	ts.Equal(http.StatusFound, resp.StatusCode, "Expected redirect status")
	location := resp.Header.Get("Location")
	ts.NotEmpty(location, "Expected redirect location header")

	authId, executionId, err := testutils.ExtractAuthData(location)
	ts.NoError(err, "Failed to extract auth ID")

	// Initiate authentication flow
	initialStep, err := testutils.ExecuteAuthenticationFlow(executionId, nil, "")
	ts.NoError(err, "Failed to initiate authentication flow")

	// Execute authentication flow
	flowStep, err := testutils.ExecuteAuthenticationFlow(executionId, map[string]string{
		"username": "localestest",
		"password": "testpass123",
	}, "action_001", initialStep.ChallengeToken)
	ts.NoError(err, "Failed to execute authentication flow")
	ts.Equal("COMPLETE", flowStep.FlowStatus, "Expected flow status COMPLETE")

	// Complete authorization
	authzResponse, err := testutils.CompleteAuthorization(authId, flowStep.Assertion)
	ts.NoError(err, "Failed to complete authorization")

	// Extract authorization code
	authzCode, err := testutils.ExtractAuthorizationCode(authzResponse.RedirectURI)
	ts.NoError(err, "Failed to extract authorization code")
	ts.NotEmpty(authzCode, "Authorization code should be present")

	// Exchange authorization code for access token
	result, err := testutils.RequestToken(clientID, clientSecret, authzCode, redirectURI, "authorization_code")
	ts.NoError(err, "Failed to exchange code for token")
	ts.Equal(http.StatusOK, result.StatusCode, "Token request should succeed")

	// Verify token response
	tokenResponse := result.Token
	ts.NotEmpty(tokenResponse.AccessToken, "Access token should be present")
	ts.Equal("Bearer", tokenResponse.TokenType, "Token type should be Bearer")
	ts.True(tokenResponse.ExpiresIn > 0, "Expires in should be greater than 0")

	// Decode JWT to verify basic claims
	parts := strings.Split(tokenResponse.AccessToken, ".")
	ts.Len(parts, 3, "Access token should be a JWT with 3 parts")

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	ts.NoError(err, "Failed to decode JWT payload")

	var claims map[string]interface{}
	err = json.Unmarshal(payloadBytes, &claims)
	ts.NoError(err, "Failed to unmarshal JWT claims")

	ts.Equal(clientID, claims["aud"], "Audience claim should match client_id")
	ts.Equal("openid", claims["scope"], "Scope claim should match requested scope")
	ts.Equal(userID, claims["sub"], "Subject claim should match authenticated user ID")
	ts.Equal(claimsLocales, claims["claims_locales"], "claims_locales claim should match requested value")
}

// TestAuthorizationCodeFlowWithNonce verifies that when a nonce is sent in the
// authorization request, the same nonce value is included in the issued ID token.
// This ensures compliance with OIDC replay protection requirements.
func (ts *AuthzTestSuite) TestAuthorizationCodeFlowWithNonce() {
	nonce := "test-nonce-123"

	// Create test user
	user := testutils.User{
		OUID: testOUID,
		Type: "authz-test-person",
		Attributes: json.RawMessage(`{
			"username": "noncetest",
			"password": "testpass123",
			"email": "noncetest@example.com",
			"given_name": "Nonce",
			"family_name": "Test"
		}`),
	}

	userID, err := testutils.CreateUser(user)
	ts.NoError(err, "Failed to create test user")
	defer func() {
		_ = testutils.DeleteUser(userID)
	}()

	// Start authorization flow WITH nonce
	resp, err := testutils.InitiateAuthorizationFlowWithNonce(
		clientID,
		redirectURI,
		"code",
		"openid",
		"test_nonce_state",
		nonce,
	)
	ts.NoError(err)
	defer resp.Body.Close()

	ts.Equal(http.StatusFound, resp.StatusCode)

	location := resp.Header.Get("Location")
	authId, executionId, err := testutils.ExtractAuthData(location)
	ts.NoError(err)

	// Initiate auth flow
	initialStep, err := testutils.ExecuteAuthenticationFlow(executionId, nil, "")
	ts.NoError(err)

	// Execute credentials
	flowStep, err := testutils.ExecuteAuthenticationFlow(executionId, map[string]string{
		"username": "noncetest",
		"password": "testpass123",
	}, "action_001", initialStep.ChallengeToken)
	ts.NoError(err)
	ts.Equal("COMPLETE", flowStep.FlowStatus)

	// Complete authorization
	authzResponse, err := testutils.CompleteAuthorization(authId, flowStep.Assertion)
	ts.NoError(err)

	authzCode, err := testutils.ExtractAuthorizationCode(authzResponse.RedirectURI)
	ts.NoError(err)

	// Exchange code for token
	result, err := testutils.RequestToken(
		clientID,
		clientSecret,
		authzCode,
		redirectURI,
		"authorization_code",
	)
	ts.NoError(err)
	ts.Equal(http.StatusOK, result.StatusCode)

	tokenResponse := result.Token

	ts.NotEmpty(tokenResponse.IDToken, "ID token must be present")

	// Decode ID token
	parts := strings.Split(tokenResponse.IDToken, ".")
	ts.Len(parts, 3)

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	ts.NoError(err)

	var claims map[string]interface{}
	err = json.Unmarshal(payloadBytes, &claims)
	ts.NoError(err)

	ts.Equal(nonce, claims["nonce"], "Nonce claim must match requested nonce")
}

// TestNonceIgnoredWithoutOpenIDScope verifies that nonce is ignored
// when openid scope is NOT requested
func (ts *AuthzTestSuite) TestNonceIgnoredWithoutOpenIDScope() {

	nonce := "should-not-be-present"

	// Create test user
	user := testutils.User{
		OUID: testOUID,
		Type: "authz-test-person",
		Attributes: json.RawMessage(`{
			"username": "nonopeniduser",
			"password": "testpass123",
			"email": "nonopeniduser@example.com",
			"given_name": "No",
			"family_name": "OpenID"
		}`),
	}

	userID, err := testutils.CreateUser(user)
	ts.NoError(err)
	defer func() {
		_ = testutils.DeleteUser(userID)
	}()

	// Start authorization flow WITHOUT openid scope
	resp, err := testutils.InitiateAuthorizationFlowWithNonce(
		clientID,
		redirectURI,
		"code",
		"profile", // No openid scope
		"test_state_no_openid",
		nonce,
	)
	ts.NoError(err)
	defer resp.Body.Close()

	ts.Equal(http.StatusFound, resp.StatusCode)

	location := resp.Header.Get("Location")
	authId, executionId, err := testutils.ExtractAuthData(location)
	ts.NoError(err)

	// Execute authentication flow
	initialStep, err := testutils.ExecuteAuthenticationFlow(executionId, nil, "")
	ts.NoError(err)

	flowStep, err := testutils.ExecuteAuthenticationFlow(executionId, map[string]string{
		"username": "nonopeniduser",
		"password": "testpass123",
	}, "action_001", initialStep.ChallengeToken)
	ts.NoError(err)
	ts.Equal("COMPLETE", flowStep.FlowStatus)

	// Complete authorization
	authzResponse, err := testutils.CompleteAuthorization(authId, flowStep.Assertion)
	ts.NoError(err)

	authzCode, err := testutils.ExtractAuthorizationCode(authzResponse.RedirectURI)
	ts.NoError(err)

	// Exchange code for token
	result, err := testutils.RequestToken(
		clientID,
		clientSecret,
		authzCode,
		redirectURI,
		"authorization_code",
	)
	ts.NoError(err)
	ts.Equal(http.StatusOK, result.StatusCode)

	tokenResponse := result.Token

	// 🔎 If ID token exists, ensure nonce is NOT present
	if tokenResponse.IDToken != "" {
		parts := strings.Split(tokenResponse.IDToken, ".")
		ts.Len(parts, 3)

		payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
		ts.NoError(err)

		var claims map[string]interface{}
		err = json.Unmarshal(payloadBytes, &claims)
		ts.NoError(err)

		_, exists := claims["nonce"]
		ts.False(exists, "Nonce must not be included when openid scope is absent")
	}
}

// TestAuthorizationCodeFlow_IDToken_JWE verifies that when an application is configured with
// ID token encryption (RSA-OAEP-256 / A256GCM), the id_token returned from the token endpoint
// is a JWE compact serialisation (five dot-separated parts) rather than a plain JWT.
func (ts *AuthzTestSuite) TestAuthorizationCodeFlow_IDToken_JWE() {
	// Generate RSA key pair and build inline JWKS with use=enc.
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	ts.Require().NoError(err)

	eBytes := big.NewInt(int64(privKey.PublicKey.E)).Bytes()
	jwksKey := map[string]interface{}{
		"kty": "RSA",
		"use": "enc",
		"alg": "RSA-OAEP-256",
		"n":   base64.RawURLEncoding.EncodeToString(privKey.PublicKey.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(eBytes),
	}
	jwksBytes, err := json.Marshal(map[string]interface{}{"keys": []interface{}{jwksKey}})
	ts.Require().NoError(err)
	jwksJSON := string(jwksBytes)

	const (
		jweClientID     = "authz_idtoken_jwe_client"
		jweClientSecret = "authz_idtoken_jwe_secret" //nolint:gosec // test credential
	)

	// Create an application with ID token encryption configured.
	app := map[string]interface{}{
		"name":                      "AuthzIDTokenJWETestApp",
		"description":               "Test app for ID token JWE integration",
		"ouId":                      testOUID,
		"authFlowId":                ts.authFlowID,
		"isRegistrationFlowEnabled": false,
		"allowedUserTypes":          []string{"authz-test-person"},
		"inboundAuthConfig": []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                jweClientID,
					"clientSecret":            jweClientSecret,
					"redirectUris":            []string{redirectURI},
					"grantTypes":              []string{"authorization_code"},
					"responseTypes":           []string{"code"},
					"tokenEndpointAuthMethod": "client_secret_basic",
					"scopes":                  []string{"openid"},
					"token": map[string]interface{}{
						"idToken": map[string]interface{}{
							"responseType":  "JWE",
							"encryptionAlg": "RSA-OAEP-256",
							"encryptionEnc": "A256GCM",
						},
					},
					"certificate": map[string]interface{}{
						"type":  "JWKS",
						"value": jwksJSON,
					},
				},
			},
		},
	}

	jsonData, err := json.Marshal(app)
	ts.Require().NoError(err)

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/applications", bytes.NewBuffer(jsonData))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusCreated, resp.StatusCode, "Failed to create JWE test application")

	var appRespData map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&appRespData)
	ts.Require().NoError(err)
	jweAppID := appRespData["id"].(string)
	defer func() {
		delReq, _ := http.NewRequest("DELETE",
			fmt.Sprintf("%s/applications/%s", testutils.TestServerURL, jweAppID), nil)
		delResp, _ := ts.client.Do(delReq)
		if delResp != nil {
			_ = delResp.Body.Close()
		}
	}()

	// Create a test user.
	user := testutils.User{
		OUID: testOUID,
		Type: "authz-test-person",
		Attributes: json.RawMessage(`{
			"username": "idtoken_jwe_user",
			"password": "testpass123",
			"email": "idtoken_jwe_user@example.com",
			"given_name": "JWE",
			"family_name": "User"
		}`),
	}
	userID, err := testutils.CreateUser(user)
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteUser(userID) }()

	// Run the full authorization code flow.
	authzResp, err := testutils.InitiateAuthorizationFlow(jweClientID, redirectURI, "code", "openid", "jwe_state")
	ts.Require().NoError(err)
	defer authzResp.Body.Close()
	ts.Require().Equal(http.StatusFound, authzResp.StatusCode)

	authID, executionID, err := testutils.ExtractAuthData(authzResp.Header.Get("Location"))
	ts.Require().NoError(err)

	initialStep, err := testutils.ExecuteAuthenticationFlow(executionID, nil, "")
	ts.Require().NoError(err)

	flowStep, err := testutils.ExecuteAuthenticationFlow(executionID, map[string]string{
		"username": "idtoken_jwe_user",
		"password": "testpass123",
	}, "action_001", initialStep.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus)

	authzResponse, err := testutils.CompleteAuthorization(authID, flowStep.Assertion)
	ts.Require().NoError(err)

	authzCode, err := testutils.ExtractAuthorizationCode(authzResponse.RedirectURI)
	ts.Require().NoError(err)

	result, err := testutils.RequestToken(jweClientID, jweClientSecret, authzCode, redirectURI, "authorization_code")
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, result.StatusCode)

	idToken := result.Token.IDToken
	ts.Require().NotEmpty(idToken, "ID token must be present")

	// A JWE compact serialisation has exactly 5 dot-separated parts.
	parts := strings.Split(idToken, ".")
	ts.Len(parts, 5, "Encrypted ID token must be a JWE compact serialisation (5 parts)")
}
