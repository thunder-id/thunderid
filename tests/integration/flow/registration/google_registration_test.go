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
	googleRegistrationFlow = testutils.Flow{
		Name:     "Google Registration Test Flow",
		FlowType: "REGISTRATION",
		Handle:   "registration_flow_google_test",
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
				"onSuccess":    "google_auth",
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
				"id":   "google_auth",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"idpId": "placeholder-idp-id",
				},
				"executor": map[string]interface{}{
					"name": "GoogleOIDCAuthExecutor",
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

	googleRegistrationFlowWithExistingUser = testutils.Flow{
		Name:     "Google Registration Test Flow With Existing User",
		FlowType: "REGISTRATION",
		Handle:   "registration_flow_google_with_existing_user_test",
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
				"onSuccess": "google_auth",
			},
			{
				"id":   "google_auth",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"idpId":                             "placeholder-idp-id",
					"allowRegistrationWithExistingUser": true,
				},
				"executor": map[string]interface{}{
					"name": "GoogleOIDCAuthExecutor",
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

	googleRegTestOU = testutils.OrganizationUnit{
		Handle:      "google-reg-flow-test-ou",
		Name:        "Google Registration Flow Test Organization Unit",
		Description: "Organization unit for Google registration flow testing",
		Parent:      nil,
	}

	googleRegEntityType = testutils.UserType{
		Name: "google_reg_flow_user",
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
				"type": "string",
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

	googleRegTestApp = testutils.Application{
		Name:                      "Google Registration Flow Test Application",
		Description:               "Application for testing Google registration flows",
		IsRegistrationFlowEnabled: true,
		ClientID:                  "google_reg_flow_test_client",
		ClientSecret:              "google_reg_flow_test_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{googleRegEntityType.Name},
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{"userType", "ouId", "ouName", "ouHandle"},
		},
	}
)

var (
	googleRegTestAppID string
	googleRegTestOUID  string
)

const (
	mockGoogleRegFlowPort = 8093
)

type GoogleRegistrationFlowTestSuite struct {
	suite.Suite
	mockGoogleServer *testutils.MockGoogleOIDCServer
	idpID            string
	entityTypeID     string
	config           *common.TestSuiteConfig
}

func TestGoogleRegistrationFlowTestSuite(t *testing.T) {
	suite.Run(t, new(GoogleRegistrationFlowTestSuite))
}

func (ts *GoogleRegistrationFlowTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}

	// Start mock Google server
	mockServer, err := testutils.NewMockGoogleOIDCServer(mockGoogleRegFlowPort,
		"test_google_client", "test_google_secret")
	ts.Require().NoError(err, "Failed to create mock Google server")
	ts.mockGoogleServer = mockServer

	ts.mockGoogleServer.AddUser(&testutils.GoogleUserInfo{
		Sub:           "google-reg-user-456",
		Email:         "reguser@gmail.com",
		EmailVerified: true,
		Name:          "Registration User",
		GivenName:     "Registration",
		FamilyName:    "User",
		Picture:       "https://example.com/regpicture.jpg",
		Locale:        "en",
	})

	err = ts.mockGoogleServer.Start()
	ts.Require().NoError(err, "Failed to start mock Google server")

	// Create test organization unit for Google registration tests
	ouID, err := testutils.CreateOrganizationUnit(googleRegTestOU)
	if err != nil {
		ts.T().Fatalf("Failed to create test organization unit during setup: %v", err)
	}
	googleRegTestOUID = ouID

	// create user type
	googleRegEntityType.OUID = googleRegTestOUID
	googleRegEntityType.AllowSelfRegistration = true
	schemaID, err := testutils.CreateUserType(googleRegEntityType)
	ts.Require().NoError(err, "Failed to create Google user type")
	ts.entityTypeID = schemaID

	// Create Google IDP
	googleIDP := testutils.IDP{
		Name:        "Google Registration Test IDP",
		Description: "Google IDP for registration flow test",
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
	ts.idpID = idpID
	ts.config.CreatedIdpIDs = append(ts.config.CreatedIdpIDs, idpID)

	// Update flow definitions with created IDP ID
	nodes := googleRegistrationFlow.Nodes.([]map[string]interface{})
	nodes[3]["properties"].(map[string]interface{})["idpId"] = idpID
	googleRegistrationFlow.Nodes = nodes

	nodesWithExisting := googleRegistrationFlowWithExistingUser.Nodes.([]map[string]interface{})
	nodesWithExisting[2]["properties"].(map[string]interface{})["idpId"] = idpID
	googleRegistrationFlowWithExistingUser.Nodes = nodesWithExisting

	// Create registration flows
	flowID, err := testutils.CreateFlow(googleRegistrationFlow)
	ts.Require().NoError(err, "Failed to create Google registration flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)
	googleRegTestApp.RegistrationFlowID = flowID

	flowIDWithExisting, err := testutils.CreateFlow(googleRegistrationFlowWithExistingUser)
	ts.Require().NoError(err, "Failed to create Google registration flow with existing user")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowIDWithExisting)

	// Create test application with the first flow
	googleRegTestApp.OUID = googleRegTestOUID
	appID, err := testutils.CreateApplication(googleRegTestApp)
	if err != nil {
		ts.T().Fatalf("Failed to create test application during setup: %v", err)
	}
	googleRegTestAppID = appID
}

func (ts *GoogleRegistrationFlowTestSuite) TearDownTest() {
	// Clean up users created during each test
	if len(ts.config.CreatedUserIDs) > 0 {
		if err := testutils.CleanupUsers(ts.config.CreatedUserIDs); err != nil {
			ts.T().Logf("Failed to cleanup users after test: %v", err)
		}
		// Reset the list for the next test
		ts.config.CreatedUserIDs = []string{}
	}
}

func (ts *GoogleRegistrationFlowTestSuite) TearDownSuite() {
	// Delete test application
	if googleRegTestAppID != "" {
		if err := testutils.DeleteApplication(googleRegTestAppID); err != nil {
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
	if googleRegTestOUID != "" {
		if err := testutils.DeleteOrganizationUnit(googleRegTestOUID); err != nil {
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
	if ts.mockGoogleServer != nil {
		_ = ts.mockGoogleServer.Stop()
		// Wait for port to be released
		time.Sleep(200 * time.Millisecond)
	}
}

func (ts *GoogleRegistrationFlowTestSuite) TestGoogleRegistrationFlowInitiation() {
	// Initialize the flow by calling the flow execution API
	flowStep, err := common.InitiateRegistrationFlow(googleRegTestAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate Google registration flow: %v", err)
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

func (ts *GoogleRegistrationFlowTestSuite) TestGoogleRegistrationFlowCompleteSuccess() {
	// Step 1: Initialize the flow by calling the flow execution API
	flowStep, err := common.InitiateRegistrationFlow(googleRegTestAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate Google registration flow: %v", err)
	}

	// Verify flow status and type
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus, "Expected flow status to be INCOMPLETE")
	ts.Require().Equal("REDIRECTION", flowStep.Type, "Expected flow type to be REDIRECT")
	ts.Require().NotEmpty(flowStep.ExecutionID, "Execution ID should not be empty")

	flowID := flowStep.ExecutionID
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

	completeFlowStep, err := common.CompleteFlow(flowID, inputs, "", flowStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete Google registration flow: %v", err)
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
	ts.Require().Equal(googleRegEntityType.Name, jwtClaims.UserType, "Expected userType to match created schema")
	ts.Require().NotEmpty(jwtClaims.OUID, "Expected ouId to be present")
	ts.Require().Equal(googleRegTestAppID, jwtClaims.Aud, "Expected aud to match the application ID")
	ts.Require().NotEmpty(jwtClaims.Sub, "JWT subject should not be empty")

	// Verify the user was created by searching via the user API
	user, err := testutils.FindUserByAttribute("sub", "google-reg-user-456")
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
		ts.Require().Equal("google-reg-user-456", attributes["sub"], "User sub should match")
	}
}

func (ts *GoogleRegistrationFlowTestSuite) TestGoogleRegistrationFlowCompleteWithInvalidCode() {
	// Step 1: Initialize the flow
	flowStep, err := common.InitiateRegistrationFlow(googleRegTestAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate Google registration flow: %v", err)
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

func (ts *GoogleRegistrationFlowTestSuite) TestGoogleRegistrationFlowCompleteWithMissingCode() {
	// Step 1: Initialize the flow
	flowStep, err := common.InitiateRegistrationFlow(googleRegTestAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate Google registration flow: %v", err)
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

func (ts *GoogleRegistrationFlowTestSuite) TestGoogleRegistrationFlowDuplicateUser() {
	// Step 1: First, create a user through registration
	flowStep, err := common.InitiateRegistrationFlow(googleRegTestAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate first Google registration flow: %v", err)
	}

	redirectURLStr := flowStep.Data.RedirectURL
	authCode, state, err := testutils.SimulateFederatedOAuthFlow(redirectURLStr)
	if err != nil {
		ts.T().Fatalf("Failed to simulate first Google authorization: %v", err)
	}

	inputs := map[string]string{
		"code":  authCode,
		"state": state,
	}

	completeFlowStep, err := common.CompleteFlow(flowStep.ExecutionID, inputs, "", flowStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete first Google registration flow: %v", err)
	}

	ts.Require().Equal("COMPLETE", completeFlowStep.FlowStatus, "First registration should complete successfully")

	// Store created user for cleanup
	user, err := testutils.FindUserByAttribute("sub", "google-reg-user-456")
	if err == nil && user != nil {
		ts.config.CreatedUserIDs = append(ts.config.CreatedUserIDs, user.ID)
	}

	// Step 2: Try to register again with the same Google user
	flowStep2, err := common.InitiateRegistrationFlow(googleRegTestAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate second Google registration flow: %v", err)
	}

	redirectURLStr2 := flowStep2.Data.RedirectURL
	authCode2, state2, err := testutils.SimulateFederatedOAuthFlow(redirectURLStr2)
	if err != nil {
		ts.T().Fatalf("Failed to simulate second Google authorization: %v", err)
	}

	inputs2 := map[string]string{
		"code":  authCode2,
		"state": state2,
	}

	completeFlowStep2, err := common.CompleteFlow(flowStep2.ExecutionID, inputs2, "", flowStep2.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete second Google registration flow: %v", err)
	}

	// Step 3: Verify registration failure due to duplicate user
	ts.Require().Equal("ERROR", completeFlowStep2.FlowStatus, "Expected flow status to be ERROR for duplicate user")
	ts.Require().Empty(completeFlowStep2.Assertion, "No JWT assertion should be returned for failed registration")
	ts.Require().NotEmpty(completeFlowStep2.FailureReason, "Failure reason should be provided for duplicate user")
}

func (ts *GoogleRegistrationFlowTestSuite) TestGoogleRegistrationFlowWithExistingUserAllowed() {
	// Step 1: First, create a user through registration with the default flow
	flowStep, err := common.InitiateRegistrationFlow(googleRegTestAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate first Google registration flow: %v", err)
	}

	redirectURLStr := flowStep.Data.RedirectURL
	authCode, state, err := testutils.SimulateFederatedOAuthFlow(redirectURLStr)
	if err != nil {
		ts.T().Fatalf("Failed to simulate first Google authorization: %v", err)
	}

	inputs := map[string]string{
		"code":  authCode,
		"state": state,
	}

	completeFlowStep, err := common.CompleteFlow(flowStep.ExecutionID, inputs, "", flowStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete first Google registration flow: %v", err)
	}

	ts.Require().Equal("COMPLETE", completeFlowStep.FlowStatus, "First registration should complete successfully")

	// Store created user for cleanup
	user, err := testutils.FindUserByAttribute("sub", "google-reg-user-456")
	if err == nil && user != nil {
		ts.config.CreatedUserIDs = append(ts.config.CreatedUserIDs, user.ID)
	}
	ts.Require().NotNil(user, "User should be created after first registration")
	firstUserID := user.ID

	// Decode first JWT to verify initial registration
	firstJWT, err := testutils.DecodeJWT(completeFlowStep.Assertion)
	ts.Require().NoError(err, "Failed to decode first JWT assertion")
	ts.Require().Equal(firstUserID, firstJWT.Sub, "First JWT subject should match the created user ID")

	// Step 2: Update application config to use the flow that allows registration with existing users
	flowIDWithExisting := ts.config.CreatedFlowIDs[1]
	err = common.UpdateAppConfig(googleRegTestAppID, "", flowIDWithExisting)
	ts.Require().NoError(err, "Failed to update app config with custom registration flow")

	// Step 3: Try to register again with the same Google user
	flowStep2, err := common.InitiateRegistrationFlow(googleRegTestAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate second Google registration flow: %v", err)
	}

	redirectURLStr2 := flowStep2.Data.RedirectURL
	authCode2, state2, err := testutils.SimulateFederatedOAuthFlow(redirectURLStr2)
	if err != nil {
		ts.T().Fatalf("Failed to simulate second Google authorization: %v", err)
	}

	inputs2 := map[string]string{
		"code":  authCode2,
		"state": state2,
	}

	completeFlowStep2, err := common.CompleteFlow(flowStep2.ExecutionID, inputs2, "", flowStep2.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete second Google registration flow: %v", err)
	}

	// Step 4: Verify that the flow completes successfully with the existing user
	ts.Require().Equal("COMPLETE", completeFlowStep2.FlowStatus,
		"Registration should complete successfully with existing user")
	ts.Require().NotEmpty(completeFlowStep2.Assertion, "JWT assertion should be returned for existing user")
	ts.Require().Empty(completeFlowStep2.FailureReason, "No failure reason should be present")

	// Decode and validate JWT claims
	jwtClaims, err := testutils.DecodeJWT(completeFlowStep2.Assertion)
	ts.Require().NoError(err, "Failed to decode JWT assertion")
	ts.Require().NotNil(jwtClaims, "JWT claims should not be nil")

	// Verify that the JWT is for the same user (existing user ID should match)
	ts.Require().Equal(firstUserID, jwtClaims.Sub, "JWT subject should match the existing user ID")
	ts.Require().Equal(googleRegEntityType.Name, jwtClaims.UserType, "User type should match")
	ts.Require().Equal(googleRegTestAppID, jwtClaims.Aud, "Audience should match the application ID")

	// Verify that no new user was created - should still be the same user
	userAfter, err := testutils.FindUserByAttribute("sub", "google-reg-user-456")
	ts.Require().NoError(err, "Should be able to find the user")
	ts.Require().NotNil(userAfter, "User should still exist")
	ts.Require().Equal(firstUserID, userAfter.ID, "User ID should be the same (no new user created)")

	// Step 5: Restore original app config
	originalFlowID := ts.config.CreatedFlowIDs[0]
	err = common.UpdateAppConfig(googleRegTestAppID, "", originalFlowID)
	if err != nil {
		ts.T().Logf("Warning: Failed to restore original app config: %v", err)
	}
}
