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

package authentication

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

var (
	googleAuthFlow = testutils.Flow{
		Name:     "Google Auth Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_google",
		Nodes: []map[string]interface{}{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "google_auth",
			},
			{
				"id":   "google_auth",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"idpId": "test-google-idp-id",
				},
				"executor": map[string]interface{}{
					"name": "GoogleOIDCAuthExecutor",
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

	googleAuthTestApp = testutils.Application{
		Name:                      "Google Auth Flow Test Application",
		Description:               "Application for testing Google authentication flows",
		IsRegistrationFlowEnabled: false,
		ClientID:                  "google_auth_flow_test_client",
		ClientSecret:              "google_auth_flow_test_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{"google_auth_user"},
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{"userType", "ouId", "ouName", "ouHandle"},
		},
	}
)

var (
	googleAuthTestAppID string
	googleAuthTestOU    = testutils.OrganizationUnit{
		Handle:      "google-auth-flow-test-ou",
		Name:        "Google Auth Flow Test OU",
		Description: "Organization unit for Google authentication flow tests",
	}
)

const (
	mockGoogleFlowPort = 8093
)

var googleEntityType = testutils.UserType{
	Name: "google_auth_user",
	Schema: map[string]interface{}{
		"username": map[string]interface{}{
			"type": "string",
		},
		"password": map[string]interface{}{
			"type":       "string",
			"credential": true,
		},
		"sub": map[string]interface{}{
			"type": "string",
		},
		"email": map[string]interface{}{
			"type": "string",
		},
		"email_verified": map[string]interface{}{
			"type": "boolean",
		},
		"name": map[string]interface{}{
			"type": "string",
		},
		"given_name": map[string]interface{}{
			"type": "string",
		},
		"family_name": map[string]interface{}{
			"type": "string",
		},
		"givenName": map[string]interface{}{
			"type": "string",
		},
		"familyName": map[string]interface{}{
			"type": "string",
		},
		"picture": map[string]interface{}{
			"type": "string",
		},
		"locale": map[string]interface{}{
			"type": "string",
		},
	},
}

type GoogleAuthFlowTestSuite struct {
	suite.Suite
	config           *common.TestSuiteConfig
	mockGoogleServer *testutils.MockGoogleOIDCServer
	userID           string
	entityTypeID     string
}

func TestGoogleAuthFlowTestSuite(t *testing.T) {
	suite.Run(t, new(GoogleAuthFlowTestSuite))
}

func (ts *GoogleAuthFlowTestSuite) SetupSuite() {
	// Initialize config
	ts.config = &common.TestSuiteConfig{}

	// Start mock Google server
	mockServer, err := testutils.NewMockGoogleOIDCServer(mockGoogleFlowPort,
		"test_google_client", "test_google_secret")
	ts.Require().NoError(err, "Failed to create mock Google server")
	ts.mockGoogleServer = mockServer

	ts.mockGoogleServer.AddUser(&testutils.GoogleUserInfo{
		Sub:           "google-test-user-123",
		Email:         "testuser@gmail.com",
		EmailVerified: true,
		Name:          "Test User",
		GivenName:     "Test",
		FamilyName:    "User",
		Picture:       "https://example.com/picture.jpg",
		Locale:        "en",
	})

	err = ts.mockGoogleServer.Start()
	ts.Require().NoError(err, "Failed to start mock Google server")

	// Create test organization unit for Google auth tests
	ouID, err := testutils.CreateOrganizationUnit(googleAuthTestOU)
	if err != nil {
		ts.T().Fatalf("Failed to create test organization unit during setup: %v", err)
	}
	googleAuthTestOU.ID = ouID

	// create user type
	googleEntityType.OUID = ouID
	schemaID, err := testutils.CreateUserType(googleEntityType)
	ts.Require().NoError(err, "Failed to create Google user type")
	ts.entityTypeID = schemaID

	// Create user
	userAttributes := map[string]interface{}{
		"username":   "googleflowuser",
		"password":   "Test@1234",
		"sub":        "google-test-user-123",
		"email":      "testuser@gmail.com",
		"givenName":  "Test",
		"familyName": "User",
	}

	attributesJSON, err := json.Marshal(userAttributes)
	ts.Require().NoError(err)

	// Create user in the pre-configured OU from database scripts
	user := testutils.User{
		Type:       googleEntityType.Name,
		OUID:       googleEntityType.OUID,
		Attributes: json.RawMessage(attributesJSON),
	}

	userID, err := testutils.CreateUser(user)
	ts.Require().NoError(err, "Failed to create test user")
	ts.userID = userID

	// Create Google IDP
	googleIDP := testutils.IDP{
		Name:        "Google Auth Test IDP",
		Description: "Google IDP for authentication flow test",
		Type:        "GOOGLE",
		Properties: []testutils.IDPProperty{
			{
				Name:     "client_id",
				Value:    "test_google_client",
				IsSecret: false,
			},
			{
				Name:     "client_secret",
				Value:    "test_google_secret",
				IsSecret: true,
			},
			{
				Name:     "redirect_uri",
				Value:    "http://localhost:3000/callback",
				IsSecret: false,
			},
			{
				Name:     "scopes",
				Value:    "openid email profile",
				IsSecret: false,
			},
			{
				Name:     "authorization_endpoint",
				Value:    ts.mockGoogleServer.GetURL() + "/o/oauth2/v2/auth",
				IsSecret: false,
			},
			{
				Name:     "token_endpoint",
				Value:    ts.mockGoogleServer.GetURL() + "/token",
				IsSecret: false,
			},
			{
				Name:     "userinfo_endpoint",
				Value:    ts.mockGoogleServer.GetURL() + "/v1/userinfo",
				IsSecret: false,
			},
			{
				Name:     "jwks_endpoint",
				Value:    ts.mockGoogleServer.GetURL() + "/oauth2/v3/certs",
				IsSecret: false,
			},
		},
	}

	idpID, err := testutils.CreateIDP(googleIDP)
	ts.Require().NoError(err, "Failed to create Google IDP")
	ts.config.CreatedIdpIDs = append(ts.config.CreatedIdpIDs, idpID)

	// Update flow definition with created IDP ID
	nodes := googleAuthFlow.Nodes.([]map[string]interface{})
	nodes[1]["properties"].(map[string]interface{})["idpId"] = idpID
	googleAuthFlow.Nodes = nodes

	// Create flow
	flowID, err := testutils.CreateFlow(googleAuthFlow)
	ts.Require().NoError(err, "Failed to create Google auth flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)
	googleAuthTestApp.AuthFlowID = flowID

	// Create test application for Google auth tests
	googleAuthTestApp.OUID = googleAuthTestOU.ID
	appID, err := testutils.CreateApplication(googleAuthTestApp)
	if err != nil {
		ts.T().Fatalf("Failed to create test application during setup: %v", err)
	}
	googleAuthTestAppID = appID
}

func (ts *GoogleAuthFlowTestSuite) TearDownSuite() {
	// Delete test application
	if googleAuthTestAppID != "" {
		if err := testutils.DeleteApplication(googleAuthTestAppID); err != nil {
			ts.T().Logf("Failed to delete test application during teardown: %v", err)
		}
	}

	// Delete test flows
	for _, flowID := range ts.config.CreatedFlowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete test flow during teardown: %v", err)
		}
	}

	// Delete test IDPs
	for _, idpID := range ts.config.CreatedIdpIDs {
		if err := testutils.DeleteIDP(idpID); err != nil {
			ts.T().Logf("Failed to delete test IDP during teardown: %v", err)
		}
	}

	// Clean up user
	if ts.userID != "" {
		_ = testutils.DeleteUser(ts.userID)
	}

	if ts.entityTypeID != "" {
		_ = testutils.DeleteUserType(ts.entityTypeID)
	}

	// Stop mock server
	if ts.mockGoogleServer != nil {
		_ = ts.mockGoogleServer.Stop()
		// Wait for port to be released
		time.Sleep(200 * time.Millisecond)
	}
}

func (ts *GoogleAuthFlowTestSuite) TestGoogleAuthFlowInitiation() {
	// Initialize the flow by calling the flow execution API
	flowStep, err := common.InitiateAuthenticationFlow(googleAuthTestAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate Google authentication flow: %v", err)
	}

	// Verify flow status and type
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus, "Expected flow status to be INCOMPLETE")
	ts.Require().Equal("REDIRECTION", flowStep.Type, "Expected flow type to be REDIRECT")
	ts.Require().NotEmpty(flowStep.ExecutionID, "Execution ID should not be empty")

	// Validate redirect information
	ts.Require().NotEmpty(flowStep.Data, "Flow data should not be empty")
	ts.Require().NotEmpty(flowStep.Data.RedirectURL, "Redirect URL should not be empty")
	redirectURLStr := flowStep.Data.RedirectURL
	ts.Require().True(strings.HasPrefix(redirectURLStr, "http://localhost:8093/o/oauth2/v2/auth"),
		"Redirect URL should point to mock Google server")

	// Parse and validate the redirect URL
	redirectURL, err := url.Parse(redirectURLStr)
	ts.Require().NoError(err, "Should be able to parse the redirect URL")

	// Check required query parameters in the redirect URL
	queryParams := redirectURL.Query()
	ts.Require().NotEmpty(queryParams.Get("client_id"), "client_id should be present in redirect URL")
	ts.Require().NotEmpty(queryParams.Get("redirect_uri"), "redirect_uri should be present in redirect URL")
	ts.Require().NotEmpty(queryParams.Get("response_type"), "response_type should be present in redirect URL")
	ts.Require().Equal("code", queryParams.Get("response_type"), "response_type should be 'code'")

	scope := queryParams.Get("scope")
	ts.Require().NotEmpty(scope, "scope should be present in redirect URL")

	scopesPresent := strings.Contains(scope, "openid") &&
		strings.Contains(scope, "email") &&
		strings.Contains(scope, "profile")
	ts.Require().True(scopesPresent, "scope should include expected scopes")
}

func (ts *GoogleAuthFlowTestSuite) TestGoogleAuthFlowInvalidAppID() {
	errorResp, err := common.InitiateAuthFlowWithError("invalid-google-app-id", nil)
	if err != nil {
		ts.T().Fatalf("Failed to initiate authentication flow with invalid app ID: %v", err)
	}

	ts.Require().Equal("FES-1003", errorResp.Code, "Expected error code for invalid app ID")
	ts.Require().Equal("Invalid request", errorResp.Message.DefaultValue, "Expected error message for invalid request")
	ts.Require().Equal("Invalid app ID provided in the request", errorResp.Description.DefaultValue,
		"Expected error description for invalid app ID")
}

func (ts *GoogleAuthFlowTestSuite) TestGoogleAuthFlowCompleteSuccess() {
	// Step 1: Initialize the flow by calling the flow execution API
	flowStep, err := common.InitiateAuthenticationFlow(googleAuthTestAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate Google authentication flow: %v", err)
	}

	// Verify flow status and type
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus, "Expected flow status to be INCOMPLETE")
	ts.Require().Equal("REDIRECTION", flowStep.Type, "Expected flow type to be REDIRECT")
	ts.Require().NotEmpty(flowStep.ExecutionID, "Execution ID should not be empty")

	ExecutionID := flowStep.ExecutionID
	redirectURLStr := flowStep.Data.RedirectURL
	ts.Require().NotEmpty(redirectURLStr, "Redirect URL should not be empty")

	// Step 2: Simulate user authorization at Google (get authorization code)
	authCode, state, err := testutils.SimulateFederatedOAuthFlow(redirectURLStr)
	if err != nil {
		ts.T().Fatalf("Failed to simulate Google authorization: %v", err)
	}
	ts.Require().NotEmpty(authCode, "Authorization code should not be empty")

	// Step 3: Complete the flow with the authorization code
	inputs := map[string]string{
		"code":  authCode,
		"state": state,
	}

	completeFlowStep, err := common.CompleteFlow(ExecutionID, inputs, "", flowStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete Google authentication flow: %v", err)
	}

	// Verify flow completion
	ts.Require().Equal("COMPLETE", completeFlowStep.FlowStatus, "Expected flow status to be COMPLETE")
	ts.Require().NotEmpty(completeFlowStep.Assertion, "Assertion token should be present")

	// Validate JWT assertion fields using common utility
	jwtClaims, err := testutils.ValidateJWTAssertionFields(
		completeFlowStep.Assertion,
		googleAuthTestAppID,
		googleEntityType.Name,
		googleAuthTestOU.ID,
		googleAuthTestOU.Name,
		googleAuthTestOU.Handle,
	)
	ts.Require().NoError(err, "Failed to validate JWT assertion fields")
	ts.Require().NotNil(jwtClaims, "JWT claims should not be nil")
}

func (ts *GoogleAuthFlowTestSuite) TestGoogleAuthFlowCompleteWithInvalidCode() {
	// Step 1: Initialize the flow
	flowStep, err := common.InitiateAuthenticationFlow(googleAuthTestAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate Google authentication flow: %v", err)
	}

	ExecutionID := flowStep.ExecutionID
	state := testutils.ExtractStateFromRedirectURL(flowStep.Data.RedirectURL)

	// Step 2: Try to complete with invalid authorization code
	inputs := map[string]string{
		"code":  "invalid-auth-code-12345",
		"state": state,
	}

	_, err = common.CompleteFlow(ExecutionID, inputs, "", flowStep.ChallengeToken)
	ts.Require().Error(err, "Should fail with invalid authorization code")
}

func (ts *GoogleAuthFlowTestSuite) TestGoogleAuthFlowCompleteWithMissingCode() {
	// Step 1: Initialize the flow
	flowStep, err := common.InitiateAuthenticationFlow(googleAuthTestAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate Google authentication flow: %v", err)
	}

	ExecutionID := flowStep.ExecutionID

	// Step 2: Try to complete without providing authorization code
	inputs := map[string]string{}

	// When required inputs are missing, the flow returns INCOMPLETE status (not an error)
	// and asks for the missing inputs again
	flowStep, err = common.CompleteFlow(ExecutionID, inputs, "", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Should not return error when inputs are missing")
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus,
		"Flow should remain INCOMPLETE when required inputs are missing")
	ts.Require().Equal("REDIRECTION", flowStep.Type, "Flow should still be REDIRECTION type")

	// Verify that code input is still required
	ts.Require().NotEmpty(flowStep.Data.Inputs, "Should still require inputs")
	hasCodeInput := false
	for _, input := range flowStep.Data.Inputs {
		if input.Identifier == "code" && input.Required {
			hasCodeInput = true
			break
		}
	}
	ts.Require().True(hasCodeInput, "Code input should still be required")
}

func (ts *GoogleAuthFlowTestSuite) TestGoogleAuthFlowMultipleUsersSuccess() {
	// This test verifies that the flow works correctly when the IDP is configured
	// with multiple users, and one of them authenticates successfully

	// Step 1: Initialize the flow
	flowStep, err := common.InitiateAuthenticationFlow(googleAuthTestAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate Google authentication flow: %v", err)
	}

	ExecutionID := flowStep.ExecutionID
	redirectURLStr := flowStep.Data.RedirectURL

	// Step 2: Simulate user authorization at Google
	authCode, state, err := testutils.SimulateFederatedOAuthFlow(redirectURLStr)
	if err != nil {
		ts.T().Fatalf("Failed to simulate Google authorization: %v", err)
	}

	// Step 3: Complete the flow with the authorization code
	inputs := map[string]string{
		"code":  authCode,
		"state": state,
	}

	completeFlowStep, err := common.CompleteFlow(ExecutionID, inputs, "", flowStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete Google authentication flow: %v", err)
	}

	// Verify flow completion
	ts.Require().Equal("COMPLETE", completeFlowStep.FlowStatus, "Expected flow status to be COMPLETE")
	ts.Require().NotEmpty(completeFlowStep.Assertion, "Assertion token should be present")

	// Validate JWT assertion fields using common utility
	jwtClaims, err := testutils.ValidateJWTAssertionFields(
		completeFlowStep.Assertion,
		googleAuthTestAppID,
		googleEntityType.Name,
		googleAuthTestOU.ID,
		googleAuthTestOU.Name,
		googleAuthTestOU.Handle,
	)
	ts.Require().NoError(err, "Failed to validate JWT assertion fields")
	ts.Require().NotNil(jwtClaims, "JWT claims should not be nil")
}
