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

package registration

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
	githubRegistrationFlow = testutils.Flow{
		Name:     "GitHub Registration Test Flow",
		FlowType: "REGISTRATION",
		Handle:   "registration_flow_github_test",
		Nodes: []map[string]interface{}{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "user_type_resolver",
			},
			{
				"id":   "user_type_resolver",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "UserTypeResolver",
				},
				"onSuccess":    "github_auth",
				"onIncomplete": "prompt_usertype",
			},
			{
				"id":   "prompt_usertype",
				"type": "PROMPT",
				"meta": map[string]interface{}{
					"components": []map[string]interface{}{
						{
							"type":    "TEXT",
							"id":      "heading_usertype",
							"label":   "Sign Up",
							"variant": "HEADING_2",
						},
						{
							"type": "BLOCK",
							"id":   "block_usertype",
							"components": []map[string]interface{}{
								{
									"type":        "SELECT",
									"id":          "usertype_input",
									"ref":         "userType",
									"label":       "User Type",
									"placeholder": "Select your user type",
									"required":    true,
									"options":     []interface{}{},
								},
								{
									"type":      "ACTION",
									"id":        "action_usertype",
									"label":     "Continue",
									"variant":   "PRIMARY",
									"eventType": "SUBMIT",
								},
							},
						},
					},
				},
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{
								"ref":        "usertype_input",
								"identifier": "userType",
								"type":       "SELECT",
								"required":   true,
							},
						},
						"action": map[string]interface{}{
							"ref":      "action_usertype",
							"nextNode": "user_type_resolver",
						},
					},
				},
			},
			{
				"id":   "github_auth",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"idpId": "placeholder-idp-id",
				},
				"executor": map[string]interface{}{
					"name": "GithubOAuthExecutor",
				},
				"onSuccess": "provisioning",
			},
			{
				"id":   "provisioning",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "ProvisioningExecutor",
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

	githubRegTestOU = testutils.OrganizationUnit{
		Handle:      "github-reg-flow-test-ou",
		Name:        "GitHub Registration Flow Test Organization Unit",
		Description: "Organization unit for GitHub registration flow testing",
		Parent:      nil,
	}

	githubRegEntityType = testutils.UserType{
		Name: "github_reg_flow_user",
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
			"login": map[string]interface{}{
				"type": "string",
			},
			"node_id": map[string]interface{}{
				"type": "string",
			},
			"avatar_url": map[string]interface{}{
				"type": "string",
			},
			"gravatar_id": map[string]interface{}{
				"type": "string",
			},
			"url": map[string]interface{}{
				"type": "string",
			},
			"html_url": map[string]interface{}{
				"type": "string",
			},
			"followers_url": map[string]interface{}{
				"type": "string",
			},
			"following_url": map[string]interface{}{
				"type": "string",
			},
			"gists_url": map[string]interface{}{
				"type": "string",
			},
			"starred_url": map[string]interface{}{
				"type": "string",
			},
			"subscriptions_url": map[string]interface{}{
				"type": "string",
			},
			"organizations_url": map[string]interface{}{
				"type": "string",
			},
			"repos_url": map[string]interface{}{
				"type": "string",
			},
			"events_url": map[string]interface{}{
				"type": "string",
			},
			"received_events_url": map[string]interface{}{
				"type": "string",
			},
			"type": map[string]interface{}{
				"type": "string",
			},
			"site_admin": map[string]interface{}{
				"type": "string",
			},
			"name": map[string]interface{}{
				"type": "string",
			},
			"company": map[string]interface{}{
				"type": "string",
			},
			"blog": map[string]interface{}{
				"type": "string",
			},
			"location": map[string]interface{}{
				"type": "string",
			},
			"hireable": map[string]interface{}{
				"type": "string",
			},
			"bio": map[string]interface{}{
				"type": "string",
			},
			"twitter_username": map[string]interface{}{
				"type": "string",
			},
			"public_repos": map[string]interface{}{
				"type": "string",
			},
			"public_gists": map[string]interface{}{
				"type": "string",
			},
			"followers": map[string]interface{}{
				"type": "string",
			},
			"following": map[string]interface{}{
				"type": "string",
			},
			"created_at": map[string]interface{}{
				"type": "string",
			},
			"updated_at": map[string]interface{}{
				"type": "string",
			},
		},
	}

	githubRegTestApp = testutils.Application{
		Name:                      "GitHub Registration Flow Test Application",
		Description:               "Application for testing GitHub registration flows",
		IsRegistrationFlowEnabled: true,
		ClientID:                  "github_reg_flow_test_client",
		ClientSecret:              "github_reg_flow_test_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{githubRegEntityType.Name},
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{"userType", "ouId", "ouName", "ouHandle"},
		},
	}
)

var (
	githubRegTestAppID string
	githubRegTestOUID  string
)

const (
	mockGithubRegFlowPort = 8092
)

type GithubRegistrationFlowTestSuite struct {
	suite.Suite
	mockGithubServer *testutils.MockGithubOAuthServer
	idpID            string
	entityTypeID     string
	config           *common.TestSuiteConfig
}

func TestGithubRegistrationFlowTestSuite(t *testing.T) {
	suite.Run(t, new(GithubRegistrationFlowTestSuite))
}

func (ts *GithubRegistrationFlowTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}

	// Start mock GitHub server
	ts.mockGithubServer = testutils.NewMockGithubOAuthServer(mockGithubRegFlowPort,
		"test_github_client", "test_github_secret")

	email := "reguser@github.com"
	ts.mockGithubServer.AddUser(&testutils.GithubUserInfo{
		Login:     "reguser",
		ID:        67890,
		NodeID:    "MDQ6VXNlcjY3ODkw",
		Email:     &email,
		Name:      "Registration User",
		AvatarURL: "https://avatars.githubusercontent.com/u/67890",
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

	// Create test organization unit for GitHub registration tests
	ouID, err := testutils.CreateOrganizationUnit(githubRegTestOU)
	if err != nil {
		ts.T().Fatalf("Failed to create test organization unit during setup: %v", err)
	}
	githubRegTestOUID = ouID

	// create user type
	githubRegEntityType.OUID = githubRegTestOUID
	githubRegEntityType.AllowSelfRegistration = true
	schemaID, err := testutils.CreateUserType(githubRegEntityType)
	ts.Require().NoError(err, "Failed to create GitHub user type")
	ts.entityTypeID = schemaID

	// Create GitHub IDP
	githubIDP := testutils.IDP{
		Name:        "GitHub Registration Test IDP",
		Description: "GitHub IDP for registration flow test",
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
	ts.idpID = idpID
	ts.config.CreatedIdpIDs = append(ts.config.CreatedIdpIDs, idpID)

	// Update flow definition with created IDP ID
	nodes := githubRegistrationFlow.Nodes.([]map[string]interface{})
	nodes[3]["properties"].(map[string]interface{})["idpId"] = idpID
	githubRegistrationFlow.Nodes = nodes

	// Create registration flow
	flowID, err := testutils.CreateFlow(githubRegistrationFlow)
	ts.Require().NoError(err, "Failed to create GitHub registration flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)
	githubRegTestApp.RegistrationFlowID = flowID

	// Create test application
	githubRegTestApp.OUID = githubRegTestOUID
	appID, err := testutils.CreateApplication(githubRegTestApp)
	if err != nil {
		ts.T().Fatalf("Failed to create test application during setup: %v", err)
	}
	githubRegTestAppID = appID
}

func (ts *GithubRegistrationFlowTestSuite) TearDownTest() {
	// Clean up users created during each test
	if len(ts.config.CreatedUserIDs) > 0 {
		if err := testutils.CleanupUsers(ts.config.CreatedUserIDs); err != nil {
			ts.T().Logf("Failed to cleanup users after test: %v", err)
		}
		// Reset the list for the next test
		ts.config.CreatedUserIDs = []string{}
	}
}

func (ts *GithubRegistrationFlowTestSuite) TearDownSuite() {
	// Delete test application
	if githubRegTestAppID != "" {
		if err := testutils.DeleteApplication(githubRegTestAppID); err != nil {
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

	// Delete test organization unit
	if githubRegTestOUID != "" {
		if err := testutils.DeleteOrganizationUnit(githubRegTestOUID); err != nil {
			ts.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}

	// Clean up any remaining users
	if len(ts.config.CreatedUserIDs) > 0 {
		if err := testutils.CleanupUsers(ts.config.CreatedUserIDs); err != nil {
			ts.T().Logf("Failed to cleanup users during teardown: %v", err)
		}
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

func (ts *GithubRegistrationFlowTestSuite) TestGithubRegistrationFlowInitiation() {
	// Initialize the flow by calling the flow execution API
	flowStep, err := common.InitiateRegistrationFlow(githubRegTestAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate GitHub registration flow: %v", err)
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

func (ts *GithubRegistrationFlowTestSuite) TestGithubRegistrationFlowCompleteSuccess() {
	// Step 1: Initialize the flow by calling the flow execution API
	flowStep, err := common.InitiateRegistrationFlow(githubRegTestAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate GitHub registration flow: %v", err)
	}

	// Verify flow status and type
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus, "Expected flow status to be INCOMPLETE")
	ts.Require().Equal("REDIRECTION", flowStep.Type, "Expected flow type to be REDIRECT")
	ts.Require().NotEmpty(flowStep.ExecutionID, "Execution ID should not be empty")

	flowID := flowStep.ExecutionID
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

	completeFlowStep, err := common.CompleteFlow(flowID, inputs, "", flowStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete GitHub registration flow: %v", err)
	}

	// Verify flow completion
	ts.Require().Equal("COMPLETE", completeFlowStep.FlowStatus, "Expected flow status to be COMPLETE")
	ts.Require().NotEmpty(completeFlowStep.Assertion, "Assertion token should be present")

	// Verify the assertion token contains expected information
	ts.Require().Contains(completeFlowStep.Assertion, ".", "Assertion should be a JWT token")

	// Decode and validate JWT claims
	jwtClaims, err := testutils.DecodeJWT(completeFlowStep.Assertion)
	ts.Require().NoError(err, "Failed to decode JWT assertion")
	ts.Require().NotNil(jwtClaims, "JWT claims should not be nil")

	// Validate JWT contains expected user type and OU ID
	ts.Require().Equal(githubRegEntityType.Name, jwtClaims.UserType, "Expected userType to match created schema")
	ts.Require().NotEmpty(jwtClaims.OUID, "Expected ouId to be present")
	ts.Require().Equal(githubRegTestAppID, jwtClaims.Aud, "Expected aud to match the application ID")
	ts.Require().NotEmpty(jwtClaims.Sub, "JWT subject should not be empty")

	// Verify the user was created by searching via the user API
	user, err := testutils.FindUserByAttribute("sub", "67890")
	if err != nil {
		ts.T().Fatalf("Failed to retrieve user by sub: %v", err)
	}
	ts.Require().NotNil(user, "User should be found in user list after registration")

	// Store the created user for cleanup
	if user != nil {
		ts.config.CreatedUserIDs = append(ts.config.CreatedUserIDs, user.ID)

		// Verify user attributes
		var attributes map[string]interface{}
		err = json.Unmarshal(user.Attributes, &attributes)
		ts.Require().NoError(err, "Should be able to unmarshal user attributes")
		ts.Require().Equal("67890", attributes["sub"], "User sub should match")
	}
}

func (ts *GithubRegistrationFlowTestSuite) TestGithubRegistrationFlowCompleteWithInvalidCode() {
	// Step 1: Initialize the flow
	flowStep, err := common.InitiateRegistrationFlow(githubRegTestAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate GitHub registration flow: %v", err)
	}

	flowID := flowStep.ExecutionID

	// Step 2: Try to complete with invalid authorization code
	state := testutils.ExtractStateFromRedirectURL(flowStep.Data.RedirectURL)
	inputs := map[string]string{
		"code":  "invalid-reg-auth-code-12345",
		"state": state,
	}

	_, err = common.CompleteFlow(flowID, inputs, "", flowStep.ChallengeToken)
	ts.Require().Error(err, "Should fail with invalid authorization code")
}

func (ts *GithubRegistrationFlowTestSuite) TestGithubRegistrationFlowCompleteWithMissingCode() {
	// Step 1: Initialize the flow
	flowStep, err := common.InitiateRegistrationFlow(githubRegTestAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate GitHub registration flow: %v", err)
	}

	flowID := flowStep.ExecutionID

	// Step 2: Try to complete without providing authorization code
	inputs := map[string]string{}

	// When required inputs are missing, the flow returns INCOMPLETE status (not an error)
	// and asks for the missing inputs again
	flowStep, err = common.CompleteFlow(flowID, inputs, "", flowStep.ChallengeToken)
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

func (ts *GithubRegistrationFlowTestSuite) TestGithubRegistrationFlowDuplicateUser() {
	// Step 1: First, create a user through registration
	flowStep, err := common.InitiateRegistrationFlow(githubRegTestAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate first GitHub registration flow: %v", err)
	}

	redirectURLStr := flowStep.Data.RedirectURL
	authCode, state, err := testutils.SimulateFederatedOAuthFlow(redirectURLStr)
	if err != nil {
		ts.T().Fatalf("Failed to simulate first GitHub authorization: %v", err)
	}

	inputs := map[string]string{
		"code":  authCode,
		"state": state,
	}

	completeFlowStep, err := common.CompleteFlow(flowStep.ExecutionID, inputs, "", flowStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete first GitHub registration flow: %v", err)
	}

	ts.Require().Equal("COMPLETE", completeFlowStep.FlowStatus, "First registration should complete successfully")

	// Store created user for cleanup
	user, err := testutils.FindUserByAttribute("sub", "67890")
	if err == nil && user != nil {
		ts.config.CreatedUserIDs = append(ts.config.CreatedUserIDs, user.ID)
	}

	// Step 2: Try to register again with the same GitHub user
	flowStep2, err := common.InitiateRegistrationFlow(githubRegTestAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate second GitHub registration flow: %v", err)
	}

	redirectURLStr2 := flowStep2.Data.RedirectURL
	authCode2, state2, err := testutils.SimulateFederatedOAuthFlow(redirectURLStr2)
	if err != nil {
		ts.T().Fatalf("Failed to simulate second GitHub authorization: %v", err)
	}

	inputs2 := map[string]string{
		"code":  authCode2,
		"state": state2,
	}

	completeFlowStep2, err := common.CompleteFlow(flowStep2.ExecutionID, inputs2, "", flowStep2.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete second GitHub registration flow: %v", err)
	}

	// Step 3: Verify registration failure due to duplicate user
	ts.Require().Equal("ERROR", completeFlowStep2.FlowStatus, "Expected flow status to be ERROR for duplicate user")
	ts.Require().Empty(completeFlowStep2.Assertion, "No JWT assertion should be returned for failed registration")
	ts.Require().NotEmpty(completeFlowStep2.FailureReason, "Failure reason should be provided for duplicate user")
}
