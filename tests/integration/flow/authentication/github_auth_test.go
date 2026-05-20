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
	githubAuthFlow = testutils.Flow{
		Name:     "GitHub Auth Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_github",
		Nodes: []map[string]interface{}{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "github_auth",
			},
			{
				"id":   "github_auth",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"idpId": "test-github-idp-id",
				},
				"executor": map[string]interface{}{
					"name": "GithubOAuthExecutor",
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

	githubAuthTestApp = testutils.Application{
		Name:                      "GitHub Auth Flow Test Application",
		Description:               "Application for testing GitHub authentication flows",
		IsRegistrationFlowEnabled: false,
		ClientID:                  "github_auth_flow_test_client",
		ClientSecret:              "github_auth_flow_test_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{"github_auth_user"},
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{"userType", "ouId", "ouName", "ouHandle"},
		},
	}
)

var (
	githubAuthTestAppID string
	githubAuthTestOU    = testutils.OrganizationUnit{
		Handle:      "github-auth-flow-test-ou",
		Name:        "GitHub Auth Flow Test OU",
		Description: "Organization unit for GitHub authentication flow tests",
	}
)

const (
	mockGithubFlowPort = 8092
)

var githubEntityType = testutils.UserType{
	Name: "github_auth_user",
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
		"givenName": map[string]interface{}{
			"type": "string",
		},
		"familyName": map[string]interface{}{
			"type": "string",
		},
	},
}

type GithubAuthFlowTestSuite struct {
	suite.Suite
	config           *common.TestSuiteConfig
	mockGithubServer *testutils.MockGithubOAuthServer
	userID           string
	entityTypeID     string
}

func TestGithubAuthFlowTestSuite(t *testing.T) {
	suite.Run(t, new(GithubAuthFlowTestSuite))
}

func (ts *GithubAuthFlowTestSuite) SetupSuite() {
	// Initialize config
	ts.config = &common.TestSuiteConfig{}

	// Start mock GitHub server
	ts.mockGithubServer = testutils.NewMockGithubOAuthServer(mockGithubFlowPort,
		"test_github_client", "test_github_secret")

	email := "testuser@github.com"
	ts.mockGithubServer.AddUser(&testutils.GithubUserInfo{
		Login:     "testuser",
		ID:        12345,
		NodeID:    "MDQ6VXNlcjEyMzQ1",
		Email:     &email,
		Name:      "Test User",
		AvatarURL: "https://avatars.githubusercontent.com/u/12345",
		Type:      "User",
		CreatedAt: "2020-01-01T00:00:00Z",
		UpdatedAt: "2024-01-01T00:00:00Z",
	}, []*testutils.GithubEmail{
		{
			Email:    email,
			Primary:  true,
			Verified: true,
		},
	})

	err := ts.mockGithubServer.Start()
	ts.Require().NoError(err, "Failed to start mock GitHub server")

	// Create test organization unit for GitHub auth tests
	ouID, err := testutils.CreateOrganizationUnit(githubAuthTestOU)
	if err != nil {
		ts.T().Fatalf("Failed to create test organization unit during setup: %v", err)
	}
	githubAuthTestOU.ID = ouID

	// create user type
	githubEntityType.OUID = ouID
	schemaID, err := testutils.CreateUserType(githubEntityType)
	ts.Require().NoError(err, "Failed to create GitHub user type")
	ts.entityTypeID = schemaID

	// Create user
	userAttributes := map[string]interface{}{
		"username":   "githubflowuser",
		"password":   "Test@1234",
		"sub":        "12345",
		"email":      "testuser@github.com",
		"givenName":  "Test",
		"familyName": "User",
	}

	attributesJSON, err := json.Marshal(userAttributes)
	ts.Require().NoError(err)

	// Create user in the pre-configured OU from database scripts
	user := testutils.User{
		Type:       githubEntityType.Name,
		OUID:       githubEntityType.OUID,
		Attributes: json.RawMessage(attributesJSON),
	}

	userID, err := testutils.CreateUser(user)
	ts.Require().NoError(err, "Failed to create test user")
	ts.userID = userID

	// Create GitHub IDP
	githubIDP := testutils.IDP{
		Name:        "GitHub Auth Test IDP",
		Description: "GitHub IDP for authentication flow test",
		Type:        "GITHUB",
		Properties: []testutils.IDPProperty{
			{
				Name:     "client_id",
				Value:    "test_github_client",
				IsSecret: false,
			},
			{
				Name:     "client_secret",
				Value:    "test_github_secret",
				IsSecret: true,
			},
			{
				Name:     "redirect_uri",
				Value:    "http://localhost:3000/callback",
				IsSecret: false,
			},
			{
				Name:     "scopes",
				Value:    "user:email,read:user",
				IsSecret: false,
			},
			{
				Name:     "authorization_endpoint",
				Value:    ts.mockGithubServer.GetURL() + "/login/oauth/authorize",
				IsSecret: false,
			},
			{
				Name:     "token_endpoint",
				Value:    ts.mockGithubServer.GetURL() + "/login/oauth/access_token",
				IsSecret: false,
			},
			{
				Name:     "userinfo_endpoint",
				Value:    ts.mockGithubServer.GetURL() + "/user",
				IsSecret: false,
			},
			{
				Name:     "user_email_endpoint",
				Value:    ts.mockGithubServer.GetURL() + "/user/emails",
				IsSecret: false,
			},
		},
	}

	idpID, err := testutils.CreateIDP(githubIDP)
	ts.Require().NoError(err, "Failed to create GitHub IDP")
	ts.config.CreatedIdpIDs = append(ts.config.CreatedIdpIDs, idpID)

	// Update flow definition with created IDP ID
	nodes := githubAuthFlow.Nodes.([]map[string]interface{})
	nodes[1]["properties"].(map[string]interface{})["idpId"] = idpID
	githubAuthFlow.Nodes = nodes

	// Create flow
	flowID, err := testutils.CreateFlow(githubAuthFlow)
	ts.Require().NoError(err, "Failed to create GitHub auth flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)
	githubAuthTestApp.AuthFlowID = flowID

	// Create test application for GitHub auth tests
	githubAuthTestApp.OUID = githubAuthTestOU.ID
	appID, err := testutils.CreateApplication(githubAuthTestApp)
	if err != nil {
		ts.T().Fatalf("Failed to create test application during setup: %v", err)
	}
	githubAuthTestAppID = appID
}

func (ts *GithubAuthFlowTestSuite) TearDownSuite() {
	// Delete test application
	if githubAuthTestAppID != "" {
		if err := testutils.DeleteApplication(githubAuthTestAppID); err != nil {
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
	if ts.mockGithubServer != nil {
		_ = ts.mockGithubServer.Stop()
		// Wait for port to be released
		time.Sleep(200 * time.Millisecond)
	}
}

func (ts *GithubAuthFlowTestSuite) TestGithubAuthFlowInitiation() {
	// Initialize the flow by calling the flow execution API
	flowStep, err := common.InitiateAuthenticationFlow(githubAuthTestAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate GitHub authentication flow: %v", err)
	}

	// Verify flow status and type
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus, "Expected flow status to be INCOMPLETE")
	ts.Require().Equal("REDIRECTION", flowStep.Type, "Expected flow type to be REDIRECT")
	ts.Require().NotEmpty(flowStep.ExecutionID, "Execution ID should not be empty")

	// Validate redirect information
	ts.Require().NotEmpty(flowStep.Data, "Flow data should not be empty")
	ts.Require().NotEmpty(flowStep.Data.RedirectURL, "Redirect URL should not be empty")
	redirectURLStr := flowStep.Data.RedirectURL
	ts.Require().True(strings.HasPrefix(redirectURLStr, "http://localhost:8092/login/oauth/authorize"),
		"Redirect URL should point to mock GitHub server")

	// Parse and validate the redirect URL
	redirectURL, err := url.Parse(redirectURLStr)
	ts.Require().NoError(err, "Should be able to parse the redirect URL")

	// Check required query parameters in the redirect URL
	queryParams := redirectURL.Query()
	ts.Require().NotEmpty(queryParams.Get("client_id"), "client_id should be present in redirect URL")
	ts.Require().NotEmpty(queryParams.Get("redirect_uri"), "redirect_uri should be present in redirect URL")

	scope := queryParams.Get("scope")
	ts.Require().NotEmpty(scope, "scope should be present in redirect URL")

	scopesPresent := strings.Contains(scope, "read:user") &&
		strings.Contains(scope, "user:email")
	ts.Require().True(scopesPresent, "scope should include expected scopes")
}

func (ts *GithubAuthFlowTestSuite) TestGithubAuthFlowInvalidAppID() {
	errorResp, err := common.InitiateAuthFlowWithError("invalid-github-app-id", nil)
	if err != nil {
		ts.T().Fatalf("Failed to initiate authentication flow with invalid app ID: %v", err)
	}

	ts.Require().Equal("FES-1003", errorResp.Code, "Expected error code for invalid app ID")
	ts.Require().Equal("Invalid request", errorResp.Message.DefaultValue, "Expected error message for invalid request")
	ts.Require().Equal("Invalid app ID provided in the request", errorResp.Description.DefaultValue,
		"Expected error description for invalid app ID")
}

func (ts *GithubAuthFlowTestSuite) TestGithubAuthFlowCompleteSuccess() {
	// Step 1: Initialize the flow by calling the flow execution API
	flowStep, err := common.InitiateAuthenticationFlow(githubAuthTestAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate GitHub authentication flow: %v", err)
	}

	// Verify flow status and type
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus, "Expected flow status to be INCOMPLETE")
	ts.Require().Equal("REDIRECTION", flowStep.Type, "Expected flow type to be REDIRECT")
	ts.Require().NotEmpty(flowStep.ExecutionID, "Execution ID should not be empty")

	ExecutionID := flowStep.ExecutionID
	redirectURLStr := flowStep.Data.RedirectURL
	ts.Require().NotEmpty(redirectURLStr, "Redirect URL should not be empty")

	// Step 2: Simulate user authorization at GitHub (get authorization code)
	authCode, state, err := testutils.SimulateFederatedOAuthFlow(redirectURLStr)
	if err != nil {
		ts.T().Fatalf("Failed to simulate GitHub authorization: %v", err)
	}
	ts.Require().NotEmpty(authCode, "Authorization code should not be empty")

	// Step 3: Complete the flow with the authorization code
	inputs := map[string]string{
		"code":  authCode,
		"state": state,
	}

	completeFlowStep, err := common.CompleteFlow(ExecutionID, inputs, "", flowStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete GitHub authentication flow: %v", err)
	}

	// Verify flow completion
	ts.Require().Equal("COMPLETE", completeFlowStep.FlowStatus, "Expected flow status to be COMPLETE")
	ts.Require().NotEmpty(completeFlowStep.Assertion, "Assertion token should be present")

	// Validate JWT assertion fields using common utility
	jwtClaims, err := testutils.ValidateJWTAssertionFields(
		completeFlowStep.Assertion,
		githubAuthTestAppID,
		githubEntityType.Name,
		githubAuthTestOU.ID,
		githubAuthTestOU.Name,
		githubAuthTestOU.Handle,
	)
	ts.Require().NoError(err, "Failed to validate JWT assertion fields")
	ts.Require().NotNil(jwtClaims, "JWT claims should not be nil")
}

func (ts *GithubAuthFlowTestSuite) TestGithubAuthFlowCompleteWithInvalidCode() {
	// Step 1: Initialize the flow
	flowStep, err := common.InitiateAuthenticationFlow(githubAuthTestAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate GitHub authentication flow: %v", err)
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

func (ts *GithubAuthFlowTestSuite) TestGithubAuthFlowCompleteWithMissingCode() {
	// Step 1: Initialize the flow
	flowStep, err := common.InitiateAuthenticationFlow(githubAuthTestAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate GitHub authentication flow: %v", err)
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
