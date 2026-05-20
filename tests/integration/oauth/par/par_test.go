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

package par

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

const (
	clientID     = "par_test_client_123"
	clientSecret = "par_test_secret_123"
	appName      = "PARTestApp"
	redirectURI  = "https://localhost:3000"
)

var (
	testOUID string

	testUserType = testutils.UserType{
		Name: "par-test-person",
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
		},
	}

	testOU = testutils.OrganizationUnit{
		Handle:      "oauth2-par-test-ou",
		Name:        "OAuth2 PAR Test OU",
		Description: "Organization unit for OAuth2 PAR testing",
		Parent:      nil,
	}

	testAuthFlow = testutils.Flow{
		Name:     "PAR Test Auth Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_par_test",
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

type PARTestSuite struct {
	suite.Suite
	applicationID string
	entityTypeID  string
	authFlowID    string
	testUserID    string
	client        *http.Client
}

func TestPARTestSuite(t *testing.T) {
	suite.Run(t, new(PARTestSuite))
}

func (ts *PARTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	// Create organization unit.
	ouID, err := testutils.CreateOrganizationUnit(testOU)
	if err != nil {
		ts.T().Fatalf("Failed to create test organization unit: %v", err)
	}
	testOUID = ouID

	// Create user type.
	testUserType.OUID = ouID
	schemaID, err := testutils.CreateUserType(testUserType)
	if err != nil {
		ts.T().Fatalf("Failed to create test user type: %v", err)
	}
	ts.entityTypeID = schemaID

	// Create authentication flow.
	flowID, err := testutils.CreateFlow(testAuthFlow)
	ts.Require().NoError(err, "Failed to create PAR test flow")
	ts.authFlowID = flowID

	// Create application with OAuth2 config.
	app := map[string]interface{}{
		"name":                      appName,
		"description":               "Application for PAR integration tests",
		"ouId":                      testOUID,
		"authFlowId":                ts.authFlowID,
		"isRegistrationFlowEnabled": false,
		"allowedUserTypes":          []string{"par-test-person"},
		"inboundAuthConfig": []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                clientID,
					"clientSecret":            clientSecret,
					"redirectUris":            []string{redirectURI},
					"grantTypes":              []string{"authorization_code"},
					"responseTypes":           []string{"code"},
					"tokenEndpointAuthMethod": "client_secret_post",
				},
			},
		},
	}

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

	// Create test user.
	user := testutils.User{
		OUID: testOUID,
		Type: "par-test-person",
		Attributes: json.RawMessage(`{
			"username": "par_test_user",
			"password": "testpass123",
			"email": "par_test_user@example.com"
		}`),
	}
	userID, err := testutils.CreateUser(user)
	ts.Require().NoError(err, "Failed to create test user")
	ts.testUserID = userID
}

func (ts *PARTestSuite) TearDownSuite() {
	if ts.testUserID != "" {
		if err := testutils.DeleteUser(ts.testUserID); err != nil {
			ts.T().Logf("Warning: Failed to delete test user: %v", err)
		}
	}

	if ts.applicationID != "" {
		req, err := http.NewRequest("DELETE",
			fmt.Sprintf("%s/applications/%s", testutils.TestServerURL, ts.applicationID), nil)
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
		}
	}

	if ts.authFlowID != "" {
		if err := testutils.DeleteFlow(ts.authFlowID); err != nil {
			ts.T().Errorf("Failed to delete test authentication flow: %v", err)
		}
	}

	if testOUID != "" {
		if err := testutils.DeleteOrganizationUnit(testOUID); err != nil {
			ts.T().Logf("Failed to delete test organization unit: %v", err)
		}
	}

	if ts.entityTypeID != "" {
		if err := testutils.DeleteUserType(ts.entityTypeID); err != nil {
			ts.T().Errorf("Failed to delete test user type: %v", err)
		}
	}
}

// validPARParams returns a base set of valid PAR parameters.
func validPARParams() map[string]string {
	return map[string]string{
		"response_type":         "code",
		"redirect_uri":          redirectURI,
		"scope":                 "openid",
		"state":                 "par_test_state",
		"code_challenge":        testutils.GenerateCodeChallenge("test-verifier-that-is-at-least-43-characters-long-enough"),
		"code_challenge_method": "S256",
	}
}

// TestPAREndpointSuccess tests a successful PAR request returns 201 with request_uri and expires_in.
func (ts *PARTestSuite) TestPAREndpointSuccess() {
	result, err := testutils.SubmitPARRequest(clientID, clientSecret, validPARParams())
	ts.Require().NoError(err)

	ts.Equal(http.StatusCreated, result.StatusCode, "PAR should return 201 Created")
	ts.Require().NotNil(result.PAR, "PAR response should be present")
	ts.True(strings.HasPrefix(result.PAR.RequestURI, "urn:ietf:params:oauth:request_uri:"),
		"request_uri should have the RFC 9126 prefix")
	ts.Greater(result.PAR.ExpiresIn, int64(0), "expires_in should be positive")
}

// TestPAREndpointWithoutAuth tests that PAR rejects requests without client authentication.
func (ts *PARTestSuite) TestPAREndpointWithoutAuth() {
	result, err := testutils.SubmitPARRequestWithoutAuth(validPARParams())
	ts.Require().NoError(err)

	ts.Equal(http.StatusBadRequest, result.StatusCode,
		"PAR should reject unauthenticated requests")
}

// TestPAREndpointInvalidClientCredentials tests that PAR rejects wrong client credentials.
func (ts *PARTestSuite) TestPAREndpointInvalidClientCredentials() {
	result, err := testutils.SubmitPARRequest(clientID, "wrong_secret", validPARParams())
	ts.Require().NoError(err)

	ts.Equal(http.StatusUnauthorized, result.StatusCode,
		"PAR should reject invalid client credentials")
}

// TestPAREndpointValidation tests PAR parameter validation.
func (ts *PARTestSuite) TestPAREndpointValidation() {
	testCases := []struct {
		Name          string
		Params        map[string]string
		ExpectedError string
	}{
		{
			Name: "Missing Response Type",
			Params: map[string]string{
				"redirect_uri":          redirectURI,
				"scope":                 "openid",
				"state":                 "test",
				"code_challenge":        testutils.GenerateCodeChallenge("test-verifier-that-is-at-least-43-characters-long-enough"),
				"code_challenge_method": "S256",
			},
			ExpectedError: "invalid_request",
		},
		{
			Name: "Unsupported Response Type",
			Params: map[string]string{
				"response_type":         "token",
				"redirect_uri":          redirectURI,
				"scope":                 "openid",
				"state":                 "test",
				"code_challenge":        testutils.GenerateCodeChallenge("test-verifier-that-is-at-least-43-characters-long-enough"),
				"code_challenge_method": "S256",
			},
			ExpectedError: "unsupported_response_type",
		},
		{
			Name: "Invalid Redirect URI",
			Params: map[string]string{
				"response_type":         "code",
				"redirect_uri":          "https://evil.example.com/callback",
				"scope":                 "openid",
				"state":                 "test",
				"code_challenge":        testutils.GenerateCodeChallenge("test-verifier-that-is-at-least-43-characters-long-enough"),
				"code_challenge_method": "S256",
			},
			ExpectedError: "invalid_request",
		},
		{
			Name: "Request URI Not Allowed In PAR Body",
			Params: map[string]string{
				"response_type":         "code",
				"redirect_uri":          redirectURI,
				"scope":                 "openid",
				"state":                 "test",
				"request_uri":           "urn:ietf:params:oauth:request_uri:test",
				"code_challenge":        testutils.GenerateCodeChallenge("test-verifier-that-is-at-least-43-characters-long-enough"),
				"code_challenge_method": "S256",
			},
			ExpectedError: "invalid_request",
		},
	}

	for _, tc := range testCases {
		ts.Run(tc.Name, func() {
			result, err := testutils.SubmitPARRequest(clientID, clientSecret, tc.Params)
			ts.Require().NoError(err)

			ts.Equal(http.StatusBadRequest, result.StatusCode)
			ts.Require().NotNil(result.Error, "Error response should be present")
			ts.Equal(tc.ExpectedError, result.Error.Error)
		})
	}
}

// TestPARAuthorizationFlowWithPKCE tests the full PAR + authorization code flow with PKCE.
func (ts *PARTestSuite) TestPARAuthorizationFlowWithPKCE() {
	token, err := testutils.ObtainAccessTokenWithPAR(clientID, redirectURI, "openid",
		"par_test_user", "testpass123", true, clientSecret)
	ts.Require().NoError(err, "PAR authorization flow should succeed")
	ts.Require().NotNil(token, "Token response should be present")
	ts.NotEmpty(token.AccessToken)
}

// TestPARRequestURISingleUse tests that a request_uri can only be used once.
func (ts *PARTestSuite) TestPARRequestURISingleUse() {
	// Submit a PAR request.
	parResult, err := testutils.SubmitPARRequest(clientID, clientSecret, validPARParams())
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusCreated, parResult.StatusCode)
	ts.Require().NotNil(parResult.PAR)

	requestURI := parResult.PAR.RequestURI

	// First use should succeed.
	resp1, err := testutils.InitiateAuthorizationFlowWithRequestURI(clientID, requestURI)
	ts.Require().NoError(err)
	defer resp1.Body.Close()

	ts.Equal(http.StatusFound, resp1.StatusCode, "First use should succeed")
	location1 := resp1.Header.Get("Location")
	// A successful first use should not redirect with an OAuth2 error and should expose authId/flowId.
	_, _, extractErr := testutils.ExtractAuthData(location1)
	ts.NoError(extractErr, "First use should produce valid auth data")

	// Second use should fail with an error redirect.
	resp2, err := testutils.InitiateAuthorizationFlowWithRequestURI(clientID, requestURI)
	ts.Require().NoError(err)
	defer resp2.Body.Close()

	ts.Equal(http.StatusFound, resp2.StatusCode, "Second use should still redirect")
	location2 := resp2.Header.Get("Location")
	err = testutils.ValidateOAuth2ErrorRedirect(location2, "invalid_request", "")
	ts.NoError(err, "Second use should produce an error redirect")
}

// TestPARInvalidRequestURI tests that an invalid request_uri is rejected by the authorize endpoint.
func (ts *PARTestSuite) TestPARInvalidRequestURI() {
	testCases := []struct {
		Name       string
		RequestURI string
	}{
		{
			Name:       "Completely Invalid URI",
			RequestURI: "not-a-valid-uri",
		},
		{
			Name:       "Valid Prefix But Non-existent",
			RequestURI: "urn:ietf:params:oauth:request_uri:nonexistent-request-123",
		},
	}

	for _, tc := range testCases {
		ts.Run(tc.Name, func() {
			resp, err := testutils.InitiateAuthorizationFlowWithRequestURI(clientID, tc.RequestURI)
			ts.Require().NoError(err)
			defer resp.Body.Close()

			ts.Equal(http.StatusFound, resp.StatusCode, "Should redirect with error")
			location := resp.Header.Get("Location")
			err = testutils.ValidateOAuth2ErrorRedirect(location, "invalid_request", "")
			ts.NoError(err, "Should produce an invalid_request error")
		})
	}
}

// TestPARClientIDBinding tests that a request_uri cannot be used by a different client.
func (ts *PARTestSuite) TestPARClientIDBinding() {
	// Submit a PAR request with the test client.
	parResult, err := testutils.SubmitPARRequest(clientID, clientSecret, validPARParams())
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusCreated, parResult.StatusCode)
	ts.Require().NotNil(parResult.PAR)

	// Try to use the request_uri with a different client_id at the authorize endpoint.
	resp, err := testutils.InitiateAuthorizationFlowWithRequestURI("different_client_id", parResult.PAR.RequestURI)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Equal(http.StatusFound, resp.StatusCode, "Should redirect")
	location := resp.Header.Get("Location")
	err = testutils.ValidateOAuth2ErrorRedirect(location, "invalid_request", "")
	ts.NoError(err, "Different client should be rejected")
}

// TestPARUniqueRequestURIs tests that each PAR request generates a unique request_uri.
func (ts *PARTestSuite) TestPARUniqueRequestURIs() {
	params := validPARParams()

	result1, err := testutils.SubmitPARRequest(clientID, clientSecret, params)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusCreated, result1.StatusCode)
	ts.Require().NotNil(result1.PAR)

	result2, err := testutils.SubmitPARRequest(clientID, clientSecret, params)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusCreated, result2.StatusCode)
	ts.Require().NotNil(result2.PAR)

	ts.NotEqual(result1.PAR.RequestURI, result2.PAR.RequestURI,
		"Each PAR request should produce a unique request_uri")
}
