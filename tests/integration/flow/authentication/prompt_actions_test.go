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
	"time"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

const (
	mockPromptActionsNotificationServerPort = 8098
)

var (
	promptActionsFlow = testutils.Flow{
		Name:     "Prompt Actions Auth Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_decision_and_mfa_test_1",
		Nodes: []map[string]interface{}{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "choose_auth",
			},
			{
				"id":   "choose_auth",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"action": map[string]interface{}{
							"ref":      "basic_auth",
							"nextNode": "basic_auth",
						},
					},
					{
						"action": map[string]interface{}{
							"ref":      "prompt_mobile",
							"nextNode": "prompt_mobile",
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
				"onSuccess": "attr_collector",
			},
			{
				"id":   "attr_collector",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "AttributeCollector",
					"inputs": []map[string]interface{}{
						{
							"ref":        "input_003",
							"identifier": "mobileNumber",
							"type":       "string",
							"required":   true,
						},
					},
				},
				"onSuccess": "sms_otp_send",
			},
			{
				"id":   "prompt_mobile",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{
								"ref":        "input_004",
								"identifier": "mobileNumber",
								"type":       "string",
								"required":   true,
							},
						},
						"action": map[string]interface{}{
							"ref":      "action_mobile",
							"nextNode": "sms_otp_send",
						},
					},
				},
			},
			{
				"id":   "sms_otp_send",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"senderId": "placeholder-sender-id",
				},
				"executor": map[string]interface{}{
					"name": "SMSOTPAuthExecutor",
					"mode": "send",
				},
				"onSuccess": "prompt_otp",
			},
			{
				"id":   "prompt_otp",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{
								"ref":        "input_005",
								"identifier": "otp",
								"type":       "number",
								"required":   true,
							},
						},
						"action": map[string]interface{}{
							"ref":      "action_otp",
							"nextNode": "sms_otp_verify",
						},
					},
				},
			},
			{
				"id":   "sms_otp_verify",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"senderId": "placeholder-sender-id",
				},
				"executor": map[string]interface{}{
					"name": "SMSOTPAuthExecutor",
					"mode": "verify",
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

	promptActionsTestApp = testutils.Application{
		Name:                      "Prompt Actions Flow Test Application",
		Description:               "Application for testing prompt with multiple actions flows",
		IsRegistrationFlowEnabled: false,
		ClientID:                  "prompt_actions_flow_test_client",
		ClientSecret:              "prompt_actions_flow_test_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{"prompt_actions_test_person"},
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{"userType", "ouId", "ouName", "ouHandle"},
		},
	}

	promptActionsTestOU = testutils.OrganizationUnit{
		Handle:      "prompt-actions-flow-test-ou",
		Name:        "Prompt Actions Flow Test Organization Unit",
		Description: "Organization unit for prompt actions flow testing",
		Parent:      nil,
	}

	promptActionsEntityType = testutils.UserType{
		Name: "prompt_actions_test_person",
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
			"mobileNumber": map[string]interface{}{
				"type": "string",
			},
		},
	}

	testUserWithMobilePromptActions = testutils.User{
		Type: promptActionsEntityType.Name,
		Attributes: json.RawMessage(`{
			"username": "promptactionsuser1",
			"password": "testpassword",
			"email": "promptactionsuser1@example.com",
			"given_name": "PromptActions",
			"family_name": "User1",
			"mobileNumber": "+1234567890"
		}`),
	}

	testUserWithoutMobilePromptActions = testutils.User{
		Type: promptActionsEntityType.Name,
		Attributes: json.RawMessage(`{
			"username": "promptactionsuser2",
			"password": "testpassword",
			"email": "promptactionsuser2@example.com",
			"given_name": "PromptActions",
			"family_name": "User2"
		}`),
	}
)

var (
	promptActionsTestAppID    string
	promptActionsTestOUID     string
	promptActionsEntityTypeID string
	promptActionsTestSenderID string
)

type PromptActionsAndMFAFlowTestSuite struct {
	suite.Suite
	config     *common.TestSuiteConfig
	mockServer *testutils.MockNotificationServer
}

func TestPromptActionsAndMFAFlowTestSuite(t *testing.T) {
	suite.Run(t, new(PromptActionsAndMFAFlowTestSuite))
}

func (ts *PromptActionsAndMFAFlowTestSuite) SetupSuite() {
	// Initialize config
	ts.config = &common.TestSuiteConfig{}

	// Create test organization unit for prompt actions tests
	ouID, err := testutils.CreateOrganizationUnit(promptActionsTestOU)
	if err != nil {
		ts.T().Fatalf("Failed to create test organization unit during setup: %v", err)
	}
	promptActionsTestOUID = ouID

	// Create test user type within the OU
	promptActionsEntityType.OUID = promptActionsTestOUID
	schemaID, err := testutils.CreateUserType(promptActionsEntityType)
	if err != nil {
		ts.T().Fatalf("Failed to create test user type during setup: %v", err)
	}
	promptActionsEntityTypeID = schemaID

	// Start mock notification server
	ts.mockServer = testutils.NewMockNotificationServer(mockPromptActionsNotificationServerPort)
	err = ts.mockServer.Start()
	if err != nil {
		ts.T().Fatalf("Failed to start mock notification server: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	ts.T().Log("Mock notification server started successfully")

	// Create test users with the created OU
	userWithMobile := testUserWithMobilePromptActions
	userWithMobile.OUID = promptActionsTestOUID
	userWithoutMobile := testUserWithoutMobilePromptActions
	userWithoutMobile.OUID = promptActionsTestOUID

	userIDs, err := testutils.CreateMultipleUsers(userWithMobile, userWithoutMobile)
	if err != nil {
		ts.T().Fatalf("Failed to create test users during setup: %v", err)
	}
	ts.config.CreatedUserIDs = userIDs
	ts.T().Logf("Test users created with IDs: %v", ts.config.CreatedUserIDs)

	// Create notification sender
	customSender := testutils.NotificationSender{
		Name:        "Prompt Actions Test SMS Sender",
		Description: "Sender for prompt actions flow testing",
		Provider:    "custom",
		Properties: []testutils.SenderProperty{
			{
				Name:     "url",
				Value:    ts.mockServer.GetSendSMSURL(),
				IsSecret: false,
			},
			{
				Name:     "http_method",
				Value:    "POST",
				IsSecret: false,
			},
			{
				Name:     "content_type",
				Value:    "JSON",
				IsSecret: false,
			},
		},
	}

	senderID, err := testutils.CreateNotificationSender(customSender)
	ts.Require().NoError(err, "Failed to create notification sender")
	promptActionsTestSenderID = senderID
	ts.config.CreatedSenderIDs = append(ts.config.CreatedSenderIDs, senderID)

	// Update flow definition with created sender ID
	nodes := promptActionsFlow.Nodes.([]map[string]interface{})
	nodes[5]["properties"].(map[string]interface{})["senderId"] = senderID // sms_otp_send node
	nodes[7]["properties"].(map[string]interface{})["senderId"] = senderID // sms_otp_verify node
	promptActionsFlow.Nodes = nodes

	// Create flow
	flowID, err := testutils.CreateFlow(promptActionsFlow)
	ts.Require().NoError(err, "Failed to create prompt actions flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)
	promptActionsTestApp.AuthFlowID = flowID

	// Create test application
	promptActionsTestApp.OUID = promptActionsTestOUID
	appID, err := testutils.CreateApplication(promptActionsTestApp)
	if err != nil {
		ts.T().Fatalf("Failed to create test application during setup: %v", err)
	}
	promptActionsTestAppID = appID
}

func (ts *PromptActionsAndMFAFlowTestSuite) TearDownSuite() {
	// Delete test users
	if err := testutils.CleanupUsers(ts.config.CreatedUserIDs); err != nil {
		ts.T().Logf("Failed to cleanup users during teardown: %v", err)
	}

	// Stop mock server
	if ts.mockServer != nil {
		err := ts.mockServer.Stop()
		if err != nil {
			ts.T().Logf("Failed to stop mock notification server during teardown: %v", err)
		}
	}

	// Delete test application
	if promptActionsTestAppID != "" {
		if err := testutils.DeleteApplication(promptActionsTestAppID); err != nil {
			ts.T().Logf("Failed to delete test application during teardown: %v", err)
		}
	}

	// Delete test flows
	for _, flowID := range ts.config.CreatedFlowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete test flow during teardown: %v", err)
		}
	}

	// Delete notification senders
	for _, senderID := range ts.config.CreatedSenderIDs {
		if err := testutils.DeleteNotificationSender(senderID); err != nil {
			ts.T().Logf("Failed to delete notification sender during teardown: %v", err)
		}
	}

	// Delete test organization unit
	if promptActionsTestOUID != "" {
		if err := testutils.DeleteOrganizationUnit(promptActionsTestOUID); err != nil {
			ts.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}

	if promptActionsEntityTypeID != "" {
		if err := testutils.DeleteUserType(promptActionsEntityTypeID); err != nil {
			ts.T().Logf("Failed to delete test user type during teardown: %v", err)
		}
	}
}

func (ts *PromptActionsAndMFAFlowTestSuite) TestBasicAuthWithMobileUserSMSOTP() {
	// Step 1: Initialize the flow - should present prompt with action choices
	flowStep, err := common.InitiateAuthenticationFlow(promptActionsTestAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate authentication flow: %v", err)
	}

	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus, "Expected flow status to be INCOMPLETE")
	ts.Require().Equal("VIEW", flowStep.Type, "Expected flow type to be VIEW")
	ts.Require().NotEmpty(flowStep.ExecutionID, "Execution ID should not be empty")

	// Validate that decision input is required
	ts.Require().NotEmpty(flowStep.Data, "Flow data should not be empty")
	ts.Require().NotEmpty(flowStep.Data.Actions, "Flow should require actions")

	// Check if expected actions are present
	expectedActions := []string{"basic_auth", "prompt_mobile"}
	ts.Require().True(common.ValidateRequiredActions(flowStep.Data.Actions, expectedActions),
		"Expected actions basic_auth and prompt_mobile should be present")

	// Step 2: Choose basic auth
	basicAuthStep, err := common.CompleteFlow(flowStep.ExecutionID, map[string]string{}, "basic_auth",
		flowStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete authentication flow with decision: %v", err)
	}

	// Should now require username and password
	ts.Require().Equal("INCOMPLETE", basicAuthStep.FlowStatus, "Expected flow status to be INCOMPLETE")
	ts.Require().Equal("VIEW", basicAuthStep.Type, "Expected flow type to be VIEW")

	// Validate required inputs using utility function
	expectedInputs := []string{"username", "password"}
	ts.Require().True(common.ValidateRequiredInputs(basicAuthStep.Data.Inputs, expectedInputs),
		"Username and password inputs should be required")

	// Step 3: Provide username and password
	userAttrs, err := testutils.GetUserAttributes(testUserWithMobilePromptActions)
	ts.Require().NoError(err, "Failed to get user attributes")

	basicInputs := map[string]string{
		"username": userAttrs["username"].(string),
		"password": userAttrs["password"].(string),
	}

	// Clear any previous messages before SMS flow
	ts.mockServer.ClearMessages()

	otpFlowStep, err := common.CompleteFlow(flowStep.ExecutionID, basicInputs, "",
		basicAuthStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete authentication flow with credentials: %v", err)
	}

	// Should now require OTP since user has mobile number
	ts.Require().Equal("INCOMPLETE", otpFlowStep.FlowStatus, "Expected flow status to be INCOMPLETE")
	ts.Require().Equal("VIEW", otpFlowStep.Type, "Expected flow type to be VIEW")

	var hasOTP bool
	for _, input := range otpFlowStep.Data.Inputs {
		if input.Identifier == "otp" {
			hasOTP = true
			break
		}
	}
	ts.Require().True(hasOTP, "OTP input should be required")

	// Wait for SMS to be sent
	time.Sleep(500 * time.Millisecond)

	// Verify SMS was sent
	lastMessage := ts.mockServer.GetLastMessage()
	ts.Require().NotNil(lastMessage, "Last message should not be nil")
	ts.Require().NotEmpty(lastMessage.OTP, "OTP should be extracted from message")

	// Step 4: Complete authentication with OTP
	otpInputs := map[string]string{
		"otp": lastMessage.OTP,
	}

	completeFlowStep, err := common.CompleteFlow(flowStep.ExecutionID, otpInputs, "action_otp",
		otpFlowStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete authentication flow with OTP: %v", err)
	}

	// Verify successful authentication
	ts.Require().Equal("COMPLETE", completeFlowStep.FlowStatus, "Expected flow status to be COMPLETE")
	ts.Require().NotEmpty(completeFlowStep.Assertion,
		"JWT assertion should be returned after successful authentication")
	ts.Require().Empty(completeFlowStep.FailureReason, "Failure reason should be empty for successful authentication")

	// Validate JWT assertion fields using common utility
	jwtClaims, err := testutils.ValidateJWTAssertionFields(
		completeFlowStep.Assertion,
		promptActionsTestAppID,
		promptActionsEntityType.Name,
		promptActionsTestOUID,
		promptActionsTestOU.Name,
		promptActionsTestOU.Handle,
	)
	ts.Require().NoError(err, "Failed to validate JWT assertion fields")
	ts.Require().NotNil(jwtClaims, "JWT claims should not be nil")
}

func (ts *PromptActionsAndMFAFlowTestSuite) TestBasicAuthWithoutMobileUserSMSOTP() {
	// Test case 1: Authentication with basic auth with user not having mobile, provide mobile, then SMS OTP
	ts.Run("TestBasicAuthWithoutMobileUserSMSOTP_ProvideMobile", func() {
		// Step 1: Initialize the flow - should present prompt with action choices
		flowStep, err := common.InitiateAuthenticationFlow(promptActionsTestAppID, false, nil, "")
		if err != nil {
			ts.T().Fatalf("Failed to initiate authentication flow: %v", err)
		}

		// Validate that decision input is required
		ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus, "Expected flow status to be INCOMPLETE")
		ts.Require().NotEmpty(flowStep.Data, "Flow data should not be empty")
		ts.Require().NotEmpty(flowStep.Data.Actions, "Flow should require actions")

		// Check if expected actions are present
		for _, action := range flowStep.Data.Actions {
			if action.Ref != "basic_auth" && action.Ref != "prompt_mobile" {
				ts.T().Fatalf("Expected action ref to be 'basic_auth' or 'prompt_mobile', but got %s", action.Ref)
			}
		}

		// Step 2: Choose basic auth
		basicAuthStep, err := common.CompleteFlow(flowStep.ExecutionID, map[string]string{}, "basic_auth",
			flowStep.ChallengeToken)
		if err != nil {
			ts.T().Fatalf("Failed to complete authentication flow with decision: %v", err)
		}

		// Step 3: Provide username and password
		var userAttrs map[string]interface{}
		err = json.Unmarshal(testUserWithoutMobilePromptActions.Attributes, &userAttrs)
		ts.Require().NoError(err, "Failed to unmarshal user attributes")

		basicInputs := map[string]string{
			"username": userAttrs["username"].(string),
			"password": userAttrs["password"].(string),
		}

		mobilePromptStep, err := common.CompleteFlow(flowStep.ExecutionID, basicInputs, "",
			basicAuthStep.ChallengeToken)
		if err != nil {
			ts.T().Fatalf("Failed to complete authentication flow with credentials: %v", err)
		}

		// Should now ask for mobile number since user doesn't have one
		ts.Require().Equal("INCOMPLETE", mobilePromptStep.FlowStatus, "Expected flow status to be INCOMPLETE")
		ts.Require().Equal("VIEW", mobilePromptStep.Type, "Expected flow type to be VIEW")

		var hasMobileNumber bool
		for _, input := range mobilePromptStep.Data.Inputs {
			if input.Identifier == "mobileNumber" {
				hasMobileNumber = true
				break
			}
		}
		ts.Require().True(hasMobileNumber, "Mobile number input should be required")

		// Clear any previous messages before SMS flow
		ts.mockServer.ClearMessages()

		// Step 4: Provide mobile number
		mobileInputs := map[string]string{
			"mobileNumber": "+1987654321",
		}

		otpFlowStep, err := common.CompleteFlow(flowStep.ExecutionID, mobileInputs, "",
			mobilePromptStep.ChallengeToken)
		if err != nil {
			ts.T().Fatalf("Failed to complete authentication flow with mobile number: %v", err)
		}

		// Should now require OTP
		ts.Require().Equal("INCOMPLETE", otpFlowStep.FlowStatus, "Expected flow status to be INCOMPLETE")
		ts.Require().Equal("VIEW", otpFlowStep.Type, "Expected flow type to be VIEW")

		var hasOTP bool
		for _, input := range otpFlowStep.Data.Inputs {
			if input.Identifier == "otp" {
				hasOTP = true
				break
			}
		}
		ts.Require().True(hasOTP, "OTP input should be required")

		// Wait for SMS to be sent
		time.Sleep(500 * time.Millisecond)

		// Verify SMS was sent
		lastMessage := ts.mockServer.GetLastMessage()
		ts.Require().NotNil(lastMessage, "Last message should not be nil")
		ts.Require().NotEmpty(lastMessage.OTP, "OTP should be extracted from message")

		// Step 5: Complete authentication with OTP
		otpInputs := map[string]string{
			"otp": lastMessage.OTP,
		}

		completeFlowStep, err := common.CompleteFlow(flowStep.ExecutionID, otpInputs, "action_otp",
			otpFlowStep.ChallengeToken)
		if err != nil {
			ts.T().Fatalf("Failed to complete authentication flow with OTP: %v", err)
		}

		// Verify successful authentication
		ts.Require().Equal("COMPLETE", completeFlowStep.FlowStatus, "Expected flow status to be COMPLETE")
		ts.Require().NotEmpty(completeFlowStep.Assertion,
			"JWT assertion should be returned after successful authentication")
		ts.Require().Empty(completeFlowStep.FailureReason,
			"Failure reason should be empty for successful authentication")

		// Validate JWT assertion fields using common utility
		jwtClaims, err := testutils.ValidateJWTAssertionFields(
			completeFlowStep.Assertion,
			promptActionsTestAppID,
			promptActionsEntityType.Name,
			promptActionsTestOUID,
			promptActionsTestOU.Name,
			promptActionsTestOU.Handle,
		)
		ts.Require().NoError(err, "Failed to validate JWT assertion fields")
		ts.Require().NotNil(jwtClaims, "JWT claims should not be nil")
	})

	// Test case 2: Retry auth flow for same user - should not prompt for mobile again
	ts.Run("TestBasicAuthWithoutMobileUserSMSOTP_RetryAuth", func() {
		// Step 1: Initialize the flow - should present prompt with action choices
		flowStep, err := common.InitiateAuthenticationFlow(promptActionsTestAppID, false, nil, "")
		if err != nil {
			ts.T().Fatalf("Failed to initiate authentication flow: %v", err)
		}

		ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus, "Expected flow status to be INCOMPLETE")
		ts.Require().NotEmpty(flowStep.Data, "Flow data should not be empty")
		ts.Require().NotEmpty(flowStep.Data.Actions, "Flow should require actions")

		// Check if expected actions are present
		for _, action := range flowStep.Data.Actions {
			if action.Ref != "basic_auth" && action.Ref != "prompt_mobile" {
				ts.T().Fatalf("Expected action ref to be 'basic_auth' or 'prompt_mobile', but got %s", action.Ref)
			}
		}

		// Step 2: Choose basic auth
		basicAuthStep, err := common.CompleteFlow(flowStep.ExecutionID, map[string]string{}, "basic_auth",
			flowStep.ChallengeToken)
		if err != nil {
			ts.T().Fatalf("Failed to complete authentication flow with decision: %v", err)
		}

		// Should now require username and password
		ts.Require().Equal("INCOMPLETE", basicAuthStep.FlowStatus, "Expected flow status to be INCOMPLETE")
		ts.Require().Equal("VIEW", basicAuthStep.Type, "Expected flow type to be VIEW")

		var hasUsername, hasPassword bool
		for _, input := range basicAuthStep.Data.Inputs {
			if input.Identifier == "username" {
				hasUsername = true
			}
			if input.Identifier == "password" {
				hasPassword = true
			}
		}
		ts.Require().True(hasUsername, "Username input should be required")
		ts.Require().True(hasPassword, "Password input should be required")

		// Step 3: Provide username and password
		var userAttrs map[string]interface{}
		err = json.Unmarshal(testUserWithoutMobilePromptActions.Attributes, &userAttrs)
		ts.Require().NoError(err, "Failed to unmarshal user attributes")

		basicInputs := map[string]string{
			"username": userAttrs["username"].(string),
			"password": userAttrs["password"].(string),
		}

		otpFlowStep, err := common.CompleteFlow(flowStep.ExecutionID, basicInputs, "",
			basicAuthStep.ChallengeToken)
		if err != nil {
			ts.T().Fatalf("Failed to complete authentication flow with mobile number: %v", err)
		}

		// Should now require OTP
		ts.Require().Equal("INCOMPLETE", otpFlowStep.FlowStatus, "Expected flow status to be INCOMPLETE")
		ts.Require().Equal("VIEW", otpFlowStep.Type, "Expected flow type to be VIEW")

		var hasOTP bool
		for _, input := range otpFlowStep.Data.Inputs {
			if input.Identifier == "otp" {
				hasOTP = true
				break
			}
		}
		ts.Require().True(hasOTP, "OTP input should be required")

		// Wait for SMS to be sent
		time.Sleep(500 * time.Millisecond)

		// Verify SMS was sent
		lastMessage := ts.mockServer.GetLastMessage()
		ts.Require().NotNil(lastMessage, "Last message should not be nil")
		ts.Require().NotEmpty(lastMessage.OTP, "OTP should be extracted from message")

		// Step 5: Complete authentication with OTP
		otpInputs := map[string]string{
			"otp": lastMessage.OTP,
		}

		completeFlowStep, err := common.CompleteFlow(flowStep.ExecutionID, otpInputs, "action_otp",
			otpFlowStep.ChallengeToken)
		if err != nil {
			ts.T().Fatalf("Failed to complete authentication flow with OTP: %v", err)
		}

		// Verify successful authentication
		ts.Require().Equal("COMPLETE", completeFlowStep.FlowStatus, "Expected flow status to be COMPLETE")
		ts.Require().NotEmpty(completeFlowStep.Assertion,
			"JWT assertion should be returned after successful authentication")
		ts.Require().Empty(completeFlowStep.FailureReason,
			"Failure reason should be empty for successful authentication")

		// Validate JWT assertion fields using common utility
		jwtClaims, err := testutils.ValidateJWTAssertionFields(
			completeFlowStep.Assertion,
			promptActionsTestAppID,
			promptActionsEntityType.Name,
			promptActionsTestOUID,
			promptActionsTestOU.Name,
			promptActionsTestOU.Handle,
		)
		ts.Require().NoError(err, "Failed to validate JWT assertion fields")
		ts.Require().NotNil(jwtClaims, "JWT claims should not be nil")
	})
}

func (ts *PromptActionsAndMFAFlowTestSuite) TestSMSOTPAuthWithValidMobile() {
	// Step 1: Initialize the flow - should present prompt with action choices
	flowStep, err := common.InitiateAuthenticationFlow(promptActionsTestAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate authentication flow: %v", err)
	}

	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus, "Expected flow status to be INCOMPLETE")
	ts.Require().NotEmpty(flowStep.Data, "Flow data should not be empty")
	ts.Require().NotEmpty(flowStep.Data.Actions, "Flow should require actions")

	// Check if expected actions are present
	for _, action := range flowStep.Data.Actions {
		if action.Ref != "basic_auth" && action.Ref != "prompt_mobile" {
			ts.T().Fatalf("Expected action ref to be 'basic_auth' or 'prompt_mobile', but got %s", action.Ref)
		}
	}

	// Step 2: Choose sms OTP auth
	smsAuthStep, err := common.CompleteFlow(flowStep.ExecutionID, map[string]string{}, "prompt_mobile",
		flowStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete authentication flow with decision: %v", err)
	}

	// Should ask for mobile number
	ts.Require().Equal("INCOMPLETE", smsAuthStep.FlowStatus, "Expected flow status to be INCOMPLETE")
	ts.Require().Equal("VIEW", smsAuthStep.Type, "Expected flow type to be VIEW")

	var hasMobileNumber bool
	for _, input := range smsAuthStep.Data.Inputs {
		if input.Identifier == "mobileNumber" {
			hasMobileNumber = true
			break
		}
	}
	ts.Require().True(hasMobileNumber, "Mobile number input should be required")

	// Clear any previous messages before SMS flow
	ts.mockServer.ClearMessages()

	// Step 3: Provide valid mobile number from user profile
	var userAttrs map[string]interface{}
	err = json.Unmarshal(testUserWithMobilePromptActions.Attributes, &userAttrs)
	ts.Require().NoError(err, "Failed to unmarshal user attributes")

	mobileInputs := map[string]string{
		"mobileNumber": userAttrs["mobileNumber"].(string),
	}

	otpFlowStep, err := common.CompleteFlow(flowStep.ExecutionID, mobileInputs, "action_mobile",
		smsAuthStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete authentication flow with mobile number: %v", err)
	}

	// Should now require OTP
	ts.Require().Equal("INCOMPLETE", otpFlowStep.FlowStatus, "Expected flow status to be INCOMPLETE")
	ts.Require().Equal("VIEW", otpFlowStep.Type, "Expected flow type to be VIEW")

	var hasOTP bool
	for _, input := range otpFlowStep.Data.Inputs {
		if input.Identifier == "otp" {
			hasOTP = true
			break
		}
	}
	ts.Require().True(hasOTP, "OTP input should be required")

	// Wait for SMS to be sent
	time.Sleep(500 * time.Millisecond)

	// Verify SMS was sent
	lastMessage := ts.mockServer.GetLastMessage()
	ts.Require().NotNil(lastMessage, "Last message should not be nil")
	ts.Require().NotEmpty(lastMessage.OTP, "OTP should be extracted from message")

	// Step 4: Complete authentication with OTP
	otpInputs := map[string]string{
		"otp": lastMessage.OTP,
	}

	completeFlowStep, err := common.CompleteFlow(flowStep.ExecutionID, otpInputs, "action_otp",
		otpFlowStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete authentication flow with OTP: %v", err)
	}

	// Verify successful authentication
	ts.Require().Equal("COMPLETE", completeFlowStep.FlowStatus, "Expected flow status to be COMPLETE")
	ts.Require().NotEmpty(completeFlowStep.Assertion,
		"JWT assertion should be returned after successful authentication")
	ts.Require().Empty(completeFlowStep.FailureReason, "Failure reason should be empty for successful authentication")

	// Validate JWT assertion fields using common utility
	jwtClaims, err := testutils.ValidateJWTAssertionFields(
		completeFlowStep.Assertion,
		promptActionsTestAppID,
		promptActionsEntityType.Name,
		promptActionsTestOUID,
		promptActionsTestOU.Name,
		promptActionsTestOU.Handle,
	)
	ts.Require().NoError(err, "Failed to validate JWT assertion fields")
	ts.Require().NotNil(jwtClaims, "JWT claims should not be nil")
}

func (ts *PromptActionsAndMFAFlowTestSuite) TestSMSOTPAuthWithInvalidMobile() {
	ts.T().Log("Test Case 5: Authentication with SMS OTP and prompt actions - invalid mobile should fail")

	// Step 1: Initialize the flow - should present prompt with action choices
	flowStep, err := common.InitiateAuthenticationFlow(promptActionsTestAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate authentication flow: %v", err)
	}

	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus, "Expected flow status to be INCOMPLETE")
	ts.Require().NotEmpty(flowStep.Data, "Flow data should not be empty")
	ts.Require().NotEmpty(flowStep.Data.Actions, "Flow should require actions")

	// Check if expected actions are present
	for _, action := range flowStep.Data.Actions {
		if action.Ref != "basic_auth" && action.Ref != "prompt_mobile" {
			ts.T().Fatalf("Expected action ref to be 'basic_auth' or 'prompt_mobile', but got %s", action.Ref)
		}
	}

	// Step 2: Choose sms OTP auth
	smsAuthStep, err := common.CompleteFlow(flowStep.ExecutionID, map[string]string{}, "prompt_mobile",
		flowStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete authentication flow with decision: %v", err)
	}

	// Should ask for mobile number
	ts.Require().Equal("INCOMPLETE", smsAuthStep.FlowStatus, "Expected flow status to be INCOMPLETE")

	// Step 3: Provide invalid mobile number (not in any user profile)
	mobileInputs := map[string]string{
		"mobileNumber": "+9999999999", // Invalid mobile not associated with any user
	}

	// This should result in failure or error
	errorResp, err := common.CompleteAuthFlowWithError(flowStep.ExecutionID, mobileInputs, flowStep.ChallengeToken)
	if err != nil {
		// If the API returned an error response, that's expected
		ts.T().Logf("Expected error occurred: %v", err)
		return
	}

	if errorResp != nil {
		// If we get an error response back, that's expected
		ts.Require().NotEmpty(errorResp.Message.DefaultValue, "Error message should be provided")
		ts.T().Logf("Authentication failed as expected: %s", errorResp.Message.DefaultValue)
	} else {
		ts.T().Fatalf("Expected authentication to fail with invalid mobile number")
	}
}
