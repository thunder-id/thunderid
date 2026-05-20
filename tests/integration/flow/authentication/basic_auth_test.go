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
	"testing"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

var (
	testOU = testutils.OrganizationUnit{
		Handle:      "basicauth_flow_test_ou",
		Name:        "Test Organization Unit for BasicAuth Flow",
		Description: "Organization unit created for BasicAuth flow testing",
		Parent:      nil,
	}

	basicAuthTestFlow = testutils.Flow{
		Name:     "Basic Auth Test Auth Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_basic_auth_test",
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
				"onSuccess":    "auth_assert",
				"onIncomplete": "prompt_credentials",
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

	basicAuthWithoutPromptFlow = testutils.Flow{
		Name:     "Basic Auth Without Prompt Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_basic_auth_without_prompt",
		Nodes: []map[string]interface{}{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "basic_auth",
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

	testApp = testutils.Application{
		Name:                      "Flow Test Application",
		Description:               "Application for testing authentication flows",
		IsRegistrationFlowEnabled: false,
		ClientID:                  "flow_test_client",
		ClientSecret:              "flow_test_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{"basic_auth_user"},
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{"userType", "ouId", "ouName", "ouHandle"},
		},
	}

	testUserType = testutils.UserType{
		Name: "basic_auth_user",
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

	testUser = testutils.User{
		Type: testUserType.Name,
		Attributes: json.RawMessage(`{
			"username": "testuser",
			"password": "testpassword",
			"email": "test@example.com",
			"given_name": "Test",
			"family_name": "User"
		}`),
	}
)

var (
	testAppID    string
	entityTypeID string
)

type BasicAuthFlowTestSuite struct {
	suite.Suite
	config *common.TestSuiteConfig
	ouID   string
}

func TestBasicAuthFlowTestSuite(t *testing.T) {
	suite.Run(t, new(BasicAuthFlowTestSuite))
}

func (ts *BasicAuthFlowTestSuite) SetupSuite() {
	// Initialize config
	ts.config = &common.TestSuiteConfig{}

	// Create test organization unit
	ouID, err := testutils.CreateOrganizationUnit(testOU)
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

	// Create test user type
	testUserType.OUID = ts.ouID
	schemaID, err := testutils.CreateUserType(testUserType)
	if err != nil {
		ts.T().Fatalf("Failed to create test user type during setup: %v", err)
	}
	entityTypeID = schemaID

	// Create flows
	flowID, err := testutils.CreateFlow(basicAuthTestFlow)
	ts.Require().NoError(err, "Failed to create basic auth test flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)
	testApp.AuthFlowID = flowID

	withoutPromptFlow, err := testutils.CreateFlow(basicAuthWithoutPromptFlow)
	ts.Require().NoError(err, "Failed to create basic auth without prompt flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, withoutPromptFlow)

	// Create test application
	testApp.OUID = ts.ouID
	appID, err := testutils.CreateApplication(testApp)
	if err != nil {
		ts.T().Fatalf("Failed to create test application during setup: %v", err)
	}
	testAppID = appID

	// Create test user with the created OU
	testUser := testUser
	testUser.OUID = ts.ouID
	userIDs, err := testutils.CreateMultipleUsers(testUser)
	if err != nil {
		ts.T().Fatalf("Failed to create test user during setup: %v", err)
	}
	ts.config.CreatedUserIDs = userIDs
}

func (ts *BasicAuthFlowTestSuite) TearDownSuite() {
	// Delete all created users
	if err := testutils.CleanupUsers(ts.config.CreatedUserIDs); err != nil {
		ts.T().Logf("Failed to cleanup users during teardown: %v", err)
	}

	// Delete test application
	if testAppID != "" {
		if err := testutils.DeleteApplication(testAppID); err != nil {
			ts.T().Logf("Failed to delete test application during teardown: %v", err)
		}
	}

	// Delete test flows
	for _, flowID := range ts.config.CreatedFlowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete test flow %s during teardown: %v", flowID, err)
		}
	}

	if entityTypeID != "" {
		if err := testutils.DeleteUserType(entityTypeID); err != nil {
			ts.T().Logf("Failed to delete test user type during teardown: %v", err)
		}
	}

	// Delete the test organization unit
	if ts.ouID != "" {
		err := testutils.DeleteOrganizationUnit(ts.ouID)
		if err != nil {
			ts.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}

}

func (ts *BasicAuthFlowTestSuite) TestBasicAuthFlowSuccess() {
	// Update application
	err := common.UpdateAppConfig(testAppID, ts.config.CreatedFlowIDs[0], "")
	ts.NoError(err, "App config update should succeed")

	// Step 1: Initialize the flow by calling the flow execution API
	flowStep, err := common.InitiateAuthenticationFlow(testAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate authentication flow: %v", err)
	}

	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus, "Expected flow status to be INCOMPLETE")
	ts.Require().Equal("VIEW", flowStep.Type, "Expected flow type to be VIEW")
	ts.Require().NotEmpty(flowStep.ExecutionID, "Execution ID should not be empty")

	// Validate that the required inputs are returned
	ts.Require().NotEmpty(flowStep.Data, "Flow data should not be empty")
	ts.Require().NotEmpty(flowStep.Data.Inputs, "Flow should require inputs")

	// Verify username and password are required inputs using utility function
	ts.Require().True(common.ValidateRequiredInputs(flowStep.Data.Inputs, []string{"username", "password"}),
		"Username and password inputs should be required")
	ts.Require().True(common.HasInput(flowStep.Data.Inputs, "username"), "Username input should be present")
	ts.Require().True(common.HasInput(flowStep.Data.Inputs, "password"), "Password input should be present")

	// Step 2: Continue the flow with valid credentials
	var userAttrs map[string]interface{}
	err = json.Unmarshal(testUser.Attributes, &userAttrs)
	ts.Require().NoError(err, "Failed to unmarshal user attributes")

	inputs := map[string]string{
		"username": userAttrs["username"].(string),
		"password": userAttrs["password"].(string),
	}

	completeFlowStep, err := common.CompleteFlow(flowStep.ExecutionID, inputs, "action_001",
		flowStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete authentication flow: %v", err)
	}

	// Verify successful authentication
	ts.Require().Equal("COMPLETE", completeFlowStep.FlowStatus, "Expected flow status to be COMPLETE")
	ts.Require().NotEmpty(completeFlowStep.Assertion,
		"JWT assertion should be returned after successful authentication")
	ts.Require().Empty(completeFlowStep.FailureReason, "Failure reason should be empty for successful authentication")

	// Validate JWT assertion fields using common utility
	jwtClaims, err := testutils.ValidateJWTAssertionFields(
		completeFlowStep.Assertion,
		testAppID,
		testUserType.Name,
		ts.ouID,
		testOU.Name,
		testOU.Handle,
	)
	ts.Require().NoError(err, "Failed to validate JWT assertion fields")
	ts.Require().NotNil(jwtClaims, "JWT claims should not be nil")
}

func (ts *BasicAuthFlowTestSuite) TestBasicAuthFlowSuccessWithSingleRequest() {
	// Update application
	err := common.UpdateAppConfig(testAppID, ts.config.CreatedFlowIDs[1], "")
	ts.NoError(err, "App config update should succeed")

	// Step 1: Initialize the flow by calling the flow execution API with user credentials
	var userAttrs map[string]interface{}
	err = json.Unmarshal(testUser.Attributes, &userAttrs)
	ts.Require().NoError(err, "Failed to unmarshal user attributes")

	inputs := map[string]string{
		"username": userAttrs["username"].(string),
		"password": userAttrs["password"].(string),
	}

	flowStep, err := common.InitiateAuthenticationFlow(testAppID, false, inputs, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate authentication flow: %v", err)
	}

	// Verify successful authentication
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus, "Expected flow status to be COMPLETE")
	ts.Require().Empty(flowStep.Data, "Flow should not require additional data after successful authentication")
	ts.Require().NotEmpty(flowStep.Assertion,
		"JWT assertion should be returned after successful authentication")
	ts.Require().Empty(flowStep.FailureReason, "Failure reason should be empty for successful authentication")

	// Validate JWT assertion fields using common utility
	jwtClaims, err := testutils.ValidateJWTAssertionFields(
		flowStep.Assertion,
		testAppID,
		testUserType.Name,
		ts.ouID,
		testOU.Name,
		testOU.Handle,
	)
	ts.Require().NoError(err, "Failed to validate JWT assertion fields")
	ts.Require().NotNil(jwtClaims, "JWT claims should not be nil")
}

func (ts *BasicAuthFlowTestSuite) TestBasicAuthFlowWithTwoStepInput() {
	// Update application
	err := common.UpdateAppConfig(testAppID, ts.config.CreatedFlowIDs[0], "")
	ts.NoError(err, "App config update should succeed")

	// Step 1: Initialize the flow
	flowStep, err := common.InitiateAuthenticationFlow(testAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate authentication flow: %v", err)
	}

	ts.Require().NotEmpty(flowStep.ExecutionID, "Execution ID should not be empty")

	var userAttrs map[string]interface{}
	err = json.Unmarshal(testUser.Attributes, &userAttrs)
	ts.Require().NoError(err, "Failed to unmarshal user attributes")

	// Step 2: Continue with missing password
	inputs := map[string]string{
		"username": userAttrs["username"].(string),
	}

	intermediateFlowStep, err := common.CompleteFlow(flowStep.ExecutionID, inputs, "action_001",
		flowStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete authentication flow with missing credentials: %v", err)
	}

	ts.Require().Equal("INCOMPLETE", intermediateFlowStep.FlowStatus, "Expected flow status to be INCOMPLETE")
	ts.Require().Equal("VIEW", intermediateFlowStep.Type, "Expected flow type to be VIEW")
	ts.Require().NotEmpty(intermediateFlowStep.ExecutionID, "Execution ID should not be empty")

	// Validate that the required inputs are returned
	ts.Require().NotEmpty(intermediateFlowStep.Data, "Flow data should not be empty")
	ts.Require().NotEmpty(intermediateFlowStep.Data.Inputs, "Flow should require inputs")

	// Verify password is required input using utility function
	ts.Require().True(common.HasInput(flowStep.Data.Inputs, "password"), "Password input should be required")

	// Step 3: Continue the flow with the password
	inputs = map[string]string{
		"password": userAttrs["password"].(string),
	}

	completeFlowStep, err := common.CompleteFlow(flowStep.ExecutionID, inputs, "action_001",
		intermediateFlowStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete authentication flow: %v", err)
	}

	// Verify successful authentication
	ts.Require().Equal("COMPLETE", completeFlowStep.FlowStatus, "Expected flow status to be COMPLETE")
	ts.Require().NotEmpty(completeFlowStep.Assertion,
		"JWT assertion should be returned after successful authentication")
	ts.Require().Empty(completeFlowStep.FailureReason, "Failure reason should be empty for successful authentication")

	// Validate JWT assertion fields using common utility
	jwtClaims, err := testutils.ValidateJWTAssertionFields(
		completeFlowStep.Assertion,
		testAppID,
		testUserType.Name,
		ts.ouID,
		testOU.Name,
		testOU.Handle,
	)
	ts.Require().NoError(err, "Failed to validate JWT assertion fields")
	ts.Require().NotNil(jwtClaims, "JWT claims should not be nil")
}

func (ts *BasicAuthFlowTestSuite) TestBasicAuthFlowInvalidCredentials() {
	// Update application
	err := common.UpdateAppConfig(testAppID, ts.config.CreatedFlowIDs[0], "")
	ts.NoError(err, "App config update should succeed")

	// Step 1: Initialize the flow
	flowStep, err := common.InitiateAuthenticationFlow(testAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate authentication flow: %v", err)
	}

	ts.Require().NotEmpty(flowStep.ExecutionID, "Execution ID should not be empty")

	// Step 2: Continue with invalid credentials
	inputs := map[string]string{
		"username": "invalid_user",
		"password": "wrong_password",
	}

	completeFlowStep, err := common.CompleteFlow(flowStep.ExecutionID, inputs, "action_001",
		flowStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete authentication flow with invalid credentials: %v", err)
	}

	// Verify authentication failure returns INCOMPLETE
	ts.Require().Equal("INCOMPLETE", completeFlowStep.FlowStatus,
		"Expected flow status to be INCOMPLETE for invalid credentials")
	ts.Require().Equal("VIEW", completeFlowStep.Type, "Expected type to be VIEW for prompt re-display")
	ts.Require().Empty(completeFlowStep.Assertion, "No JWT assertion should be returned for failed authentication")
	ts.Require().NotEmpty(completeFlowStep.FailureReason,
		"Failure reason should be provided for invalid credentials")

	// Verify both inputs are re-prompted (cleared after failure)
	ts.Require().NotEmpty(completeFlowStep.Data, "Flow data should not be empty after re-prompt")
	ts.Require().NotEmpty(completeFlowStep.Data.Inputs, "Inputs should be re-prompted after failure")
	ts.Require().Len(completeFlowStep.Data.Inputs, 2, "Both username and password should be re-prompted")
}

func (ts *BasicAuthFlowTestSuite) TestBasicAuthFlowRetryAfterInvalidCredentials() {
	// Update application
	err := common.UpdateAppConfig(testAppID, ts.config.CreatedFlowIDs[0], "")
	ts.NoError(err, "App config update should succeed")

	// Step 1: Initialize the flow
	flowStep, err := common.InitiateAuthenticationFlow(testAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate authentication flow: %v", err)
	}
	ts.Require().NotEmpty(flowStep.ExecutionID, "Execution ID should not be empty")

	// Step 2: Submit invalid credentials
	invalidInputs := map[string]string{
		"username": "invalid_user",
		"password": "wrong_password",
	}

	retryFlowStep, err := common.CompleteFlow(flowStep.ExecutionID, invalidInputs, "action_001",
		flowStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete authentication flow with invalid credentials: %v", err)
	}

	// Verify we get INCOMPLETE (retryable) not ERROR
	ts.Require().Equal("INCOMPLETE", retryFlowStep.FlowStatus, "Expected INCOMPLETE after invalid credentials")
	ts.Require().NotEmpty(retryFlowStep.FailureReason, "Failure reason should be present")

	// Step 3: Retry with valid credentials
	validInputs := map[string]string{
		"username": "testuser",
		"password": "testpassword",
	}

	successFlowStep, err := common.CompleteFlow(flowStep.ExecutionID, validInputs, "action_001",
		retryFlowStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete authentication flow after retry: %v", err)
	}

	// Verify successful authentication
	ts.Require().Equal("COMPLETE", successFlowStep.FlowStatus,
		"Expected COMPLETE after retry with valid credentials")
	ts.Require().NotEmpty(successFlowStep.Assertion, "JWT assertion should be returned on successful retry")
	ts.Require().Empty(successFlowStep.FailureReason, "No failure reason on success")
}

func (ts *BasicAuthFlowTestSuite) TestBasicAuthFlowInvalidAppID() {
	// Try to initialize the flow with an invalid app ID
	errorResp, err := common.InitiateAuthFlowWithError("invalid-app-id", nil)
	if err != nil {
		ts.T().Fatalf("Failed to initiate authentication flow with invalid app ID: %v", err)
	}

	// Verify the error response
	ts.Require().Equal("FES-1003", errorResp.Code, "Expected error code for invalid app ID")
	ts.Require().Equal("Invalid request", errorResp.Message.DefaultValue, "Expected error message for invalid request")
	ts.Require().Equal("Invalid app ID provided in the request", errorResp.Description.DefaultValue,
		"Expected error description for invalid app ID")
}

func (ts *BasicAuthFlowTestSuite) TestBasicAuthFlowInvalidFlowID() {
	// Step 1: Initialize the flow by calling the flow execution API
	flowStep, err := common.InitiateAuthenticationFlow(testAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate authentication flow: %v", err)
	}

	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus, "Expected flow status to be INCOMPLETE")
	ts.Require().Equal("VIEW", flowStep.Type, "Expected flow type to be VIEW")
	ts.Require().NotEmpty(flowStep.ExecutionID, "Execution ID should not be empty")
	ts.Require().NotEmpty(flowStep.Data, "Flow data should not be empty")
	ts.Require().NotEmpty(flowStep.Data.Inputs, "Flow should require inputs")

	// Step 2: Attempt to complete a flow with an invalid flow ID
	inputs := map[string]string{
		"username": "someuser",
		"password": "somepassword",
	}

	errorResp, err := common.CompleteAuthFlowWithError("invalid-flow-id", inputs, flowStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete authentication flow: %v", err)
	}

	// Verify the error response
	ts.Require().Equal("FES-1004", errorResp.Code, "Expected error code for invalid flow ID")
	ts.Require().Equal("Invalid request", errorResp.Message.DefaultValue, "Expected error message for invalid request")
	ts.Require().Equal("Invalid flow execution ID provided in the request", errorResp.Description.DefaultValue,
		"Expected error description for invalid flow ID")
}

// TestBasicAuthFlow_WithoutTokenConfig tests that userType and OU attributes are NOT included
// in JWT assertion when TokenConfig is not specified.
func (ts *BasicAuthFlowTestSuite) TestBasicAuthFlow_WithoutTokenConfig() {
	// Create a new application without TokenConfig
	appWithoutTokenConfig := testutils.Application{
		Name:                      "Flow Test Application Without Token Config",
		OUID:                      ts.ouID,
		Description:               "Application for testing default behavior without token config",
		IsRegistrationFlowEnabled: false,
		ClientID:                  "flow_test_client_no_token_config",
		ClientSecret:              "flow_test_secret_no_token_config",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{"basic_auth_user"},
		// TokenConfig is nil - not specified
	}

	appID, err := testutils.CreateApplication(appWithoutTokenConfig)
	ts.Require().NoError(err, "Failed to create application without token config")
	defer func() {
		if err := testutils.DeleteApplication(appID); err != nil {
			ts.T().Logf("Failed to delete test application: %v", err)
		}
	}()

	// Update application with flow
	err = common.UpdateAppConfig(appID, ts.config.CreatedFlowIDs[0], "")
	ts.Require().NoError(err, "App config update should succeed")

	// Execute authentication flow
	var userAttrs map[string]interface{}
	err = json.Unmarshal(testUser.Attributes, &userAttrs)
	ts.Require().NoError(err, "Failed to unmarshal user attributes")

	inputs := map[string]string{
		"username": userAttrs["username"].(string),
		"password": userAttrs["password"].(string),
	}

	flowStep, err := common.InitiateAuthenticationFlow(appID, false, inputs, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")

	ts.Require().Equal("COMPLETE", flowStep.FlowStatus, "Expected flow status to be COMPLETE")
	ts.Require().NotEmpty(flowStep.Assertion, "JWT assertion should be returned")

	// Decode and validate JWT claims
	jwtClaims, err := testutils.DecodeJWT(flowStep.Assertion)
	ts.Require().NoError(err, "Failed to decode JWT assertion")
	ts.Require().NotNil(jwtClaims, "JWT claims should not be nil")

	// Verify standard claims are present
	ts.Require().Equal(appID, jwtClaims.Aud, "JWT aud should match app ID")
	ts.Require().NotEmpty(jwtClaims.Sub, "JWT subject should not be empty")

	// Verify userType and OU attributes are NOT present (since TokenConfig is not specified)
	ts.Require().Empty(jwtClaims.UserType, "userType should NOT be present when TokenConfig is not specified")
	ts.Require().Empty(jwtClaims.OUID, "ouId should NOT be present when TokenConfig is not specified")
	ts.Require().Empty(jwtClaims.OuName, "ouName should NOT be present when TokenConfig is not specified")
	ts.Require().Empty(jwtClaims.OuHandle, "ouHandle should NOT be present when TokenConfig is not specified")
}

// TestBasicAuthFlow_WithEmptyUserAttributes tests that userType and OU attributes are NOT included
// in JWT assertion when user_attributes is an empty array.
func (ts *BasicAuthFlowTestSuite) TestBasicAuthFlow_WithEmptyUserAttributes() {
	// Create a new application with empty user_attributes
	appWithEmptyAttrs := testutils.Application{
		Name:                      "Flow Test Application With Empty User Attributes",
		OUID:                      ts.ouID,
		Description:               "Application for testing behavior with empty user_attributes",
		IsRegistrationFlowEnabled: false,
		ClientID:                  "flow_test_client_empty_attrs",
		ClientSecret:              "flow_test_secret_empty_attrs",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{"basic_auth_user"},
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{}, // Empty array
		},
	}

	appID, err := testutils.CreateApplication(appWithEmptyAttrs)
	ts.Require().NoError(err, "Failed to create application with empty user attributes")
	defer func() {
		if err := testutils.DeleteApplication(appID); err != nil {
			ts.T().Logf("Failed to delete test application: %v", err)
		}
	}()

	// Update application with flow
	err = common.UpdateAppConfig(appID, ts.config.CreatedFlowIDs[0], "")
	ts.Require().NoError(err, "App config update should succeed")

	// Execute authentication flow
	var userAttrs map[string]interface{}
	err = json.Unmarshal(testUser.Attributes, &userAttrs)
	ts.Require().NoError(err, "Failed to unmarshal user attributes")

	inputs := map[string]string{
		"username": userAttrs["username"].(string),
		"password": userAttrs["password"].(string),
	}

	flowStep, err := common.InitiateAuthenticationFlow(appID, false, inputs, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")

	ts.Require().Equal("COMPLETE", flowStep.FlowStatus, "Expected flow status to be COMPLETE")
	ts.Require().NotEmpty(flowStep.Assertion, "JWT assertion should be returned")

	// Decode and validate JWT claims
	jwtClaims, err := testutils.DecodeJWT(flowStep.Assertion)
	ts.Require().NoError(err, "Failed to decode JWT assertion")
	ts.Require().NotNil(jwtClaims, "JWT claims should not be nil")

	// Verify standard claims are present
	ts.Require().Equal(appID, jwtClaims.Aud, "JWT aud should match app ID")
	ts.Require().NotEmpty(jwtClaims.Sub, "JWT subject should not be empty")

	// Verify userType and OU attributes are NOT present (since user_attributes is empty)
	ts.Require().Empty(jwtClaims.UserType, "userType should NOT be present when user_attributes is empty")
	ts.Require().Empty(jwtClaims.OUID, "ouId should NOT be present when user_attributes is empty")
	ts.Require().Empty(jwtClaims.OuName, "ouName should NOT be present when user_attributes is empty")
	ts.Require().Empty(jwtClaims.OuHandle, "ouHandle should NOT be present when user_attributes is empty")
}
