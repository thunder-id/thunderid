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
	mockNotificationServerPort = 8098
)

var (
	smsAuthFlowWithMobile = testutils.Flow{
		Name:     "SMS Auth Flow with Mobile Test",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_sms_test",
		Nodes: []map[string]interface{}{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "prompt_mobile",
			},
			{
				"id":   "prompt_mobile",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{
								"ref":        "input_001",
								"identifier": "mobileNumber",
								"type":       "string",
								"required":   true,
							},
						},
						"action": map[string]interface{}{
							"ref":      "action_001",
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
								"ref":        "input_002",
								"identifier": "otp",
								"type":       "string",
								"required":   true,
							},
						},
						"action": map[string]interface{}{
							"ref":      "action_002",
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

	smsAuthFlowWithUsername = testutils.Flow{
		Name:     "SMS Auth Flow with Username Test",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_sms_test_with_username",
		Nodes: []map[string]interface{}{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "mobile_prompt_username",
			},
			{
				"id":   "mobile_prompt_username",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{
								"ref":        "input_001",
								"identifier": "username",
								"type":       "string",
								"required":   true,
							},
						},
						"action": map[string]interface{}{
							"ref":      "action_001",
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
								"ref":        "input_002",
								"identifier": "otp",
								"type":       "string",
								"required":   true,
							},
						},
						"action": map[string]interface{}{
							"ref":      "action_002",
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
)

var (
	smsAuthTestApp = testutils.Application{
		Name:                      "SMS Auth Flow Test Application",
		Description:               "Application for testing SMS authentication flows",
		IsRegistrationFlowEnabled: false,
		ClientID:                  "sms_auth_flow_test_client",
		ClientSecret:              "sms_auth_flow_test_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{"sms_auth_user"},
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{"userType", "ouId", "ouName", "ouHandle"},
		},
	}

	smsAuthEntityType = testutils.UserType{
		Name: "sms_auth_user",
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

	testUserWithMobile = testutils.User{
		Type: smsAuthEntityType.Name,
		Attributes: json.RawMessage(`{
			"username": "smsuser",
			"password": "testpassword",
			"email": "smsuser@example.com",
			"given_name": "SMS",
			"family_name": "User",
			"mobileNumber": "+1234567890"
		}`),
	}
)

var (
	smsAuthTestAppID      string
	smsAuthEntityTypeID   string
	smsAuthTestSenderID   string
	smsAuthFlowMobileID   string
	smsAuthFlowUsernameID string
	smsAuthTestOU         = testutils.OrganizationUnit{
		Handle:      "sms-auth-flow-test-ou",
		Name:        "SMS Auth Flow Test OU",
		Description: "Organization unit for SMS authentication flow tests",
	}
)

type SMSAuthFlowTestSuite struct {
	suite.Suite
	config     *common.TestSuiteConfig
	mockServer *testutils.MockNotificationServer
}

func TestSMSAuthFlowTestSuite(t *testing.T) {
	suite.Run(t, new(SMSAuthFlowTestSuite))
}

func (ts *SMSAuthFlowTestSuite) SetupSuite() {
	// Initialize config
	ts.config = &common.TestSuiteConfig{}

	// Create test organization unit for SMS auth tests
	ouID, err := testutils.CreateOrganizationUnit(smsAuthTestOU)
	if err != nil {
		ts.T().Fatalf("Failed to create test organization unit during setup: %v", err)
	}
	smsAuthTestOU.ID = ouID

	// Create test user type within the OU
	smsAuthEntityType.OUID = ouID
	schemaID, err := testutils.CreateUserType(smsAuthEntityType)
	if err != nil {
		ts.T().Fatalf("Failed to create test user type during setup: %v", err)
	}
	smsAuthEntityTypeID = schemaID

	// Start mock notification server
	ts.mockServer = testutils.NewMockNotificationServer(mockNotificationServerPort)
	err = ts.mockServer.Start()
	if err != nil {
		ts.T().Fatalf("Failed to start mock notification server: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	ts.T().Log("Mock notification server started successfully")

	// Create test user with mobile number using the created OU
	testUserWithMobile := testUserWithMobile
	testUserWithMobile.OUID = smsAuthTestOU.ID
	userIDs, err := testutils.CreateMultipleUsers(testUserWithMobile)
	if err != nil {
		ts.T().Fatalf("Failed to create test user during setup: %v", err)
	}
	ts.config.CreatedUserIDs = userIDs
	ts.T().Logf("Test user created with ID: %s", ts.config.CreatedUserIDs[0])

	// Create notification sender
	customSender := testutils.NotificationSender{
		Name:        "SMS Auth Test Sender",
		Description: "Sender for SMS authentication flow testing",
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
	smsAuthTestSenderID = senderID
	ts.config.CreatedSenderIDs = append(ts.config.CreatedSenderIDs, senderID)

	// Update flow definitions with created sender ID
	nodesSendSMS := smsAuthFlowWithMobile.Nodes.([]map[string]interface{})
	nodesSendSMS[2]["properties"].(map[string]interface{})["senderId"] = senderID // sms_otp_send node
	nodesSendSMS[4]["properties"].(map[string]interface{})["senderId"] = senderID // sms_otp_verify node
	smsAuthFlowWithMobile.Nodes = nodesSendSMS

	nodesUNSendSMS := smsAuthFlowWithUsername.Nodes.([]map[string]interface{})
	nodesUNSendSMS[2]["properties"].(map[string]interface{})["senderId"] = senderID // sms_otp_send node
	nodesUNSendSMS[4]["properties"].(map[string]interface{})["senderId"] = senderID // sms_otp_verify node
	smsAuthFlowWithUsername.Nodes = nodesUNSendSMS

	// Create flows
	flowMobileID, err := testutils.CreateFlow(smsAuthFlowWithMobile)
	ts.Require().NoError(err, "Failed to create SMS auth flow with mobile")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowMobileID)
	smsAuthFlowMobileID = flowMobileID
	smsAuthTestApp.AuthFlowID = flowMobileID

	flowUsernameID, err := testutils.CreateFlow(smsAuthFlowWithUsername)
	ts.Require().NoError(err, "Failed to create SMS auth flow with username")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowUsernameID)
	smsAuthFlowUsernameID = flowUsernameID

	// Create test application
	smsAuthTestApp.AuthFlowID = flowMobileID
	smsAuthTestApp.OUID = smsAuthTestOU.ID
	appID, err := testutils.CreateApplication(smsAuthTestApp)
	if err != nil {
		ts.T().Fatalf("Failed to create test application during setup: %v", err)
	}
	smsAuthTestAppID = appID
}

func (ts *SMSAuthFlowTestSuite) TearDownSuite() {
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
	if smsAuthTestAppID != "" {
		if err := testutils.DeleteApplication(smsAuthTestAppID); err != nil {
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
	if smsAuthTestOU.ID != "" {
		if err := testutils.DeleteOrganizationUnit(smsAuthTestOU.ID); err != nil {
			ts.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}

	// Delete test user type
	if smsAuthEntityTypeID != "" {
		if err := testutils.DeleteUserType(smsAuthEntityTypeID); err != nil {
			ts.T().Logf("Failed to delete test user type during teardown: %v", err)
		}
	}
}

func (ts *SMSAuthFlowTestSuite) TestSMSAuthFlowWithMobileNumber() {
	// Step 1: Initialize the flow by calling the flow execution API
	flowStep, err := common.InitiateAuthenticationFlow(smsAuthTestAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate authentication flow: %v", err)
	}

	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus, "Expected flow status to be INCOMPLETE")
	ts.Require().Equal("VIEW", flowStep.Type, "Expected flow type to be VIEW")
	ts.Require().NotEmpty(flowStep.ExecutionID, "Execution ID should not be empty")

	// Validate that mobile number input is required
	ts.Require().NotEmpty(flowStep.Data, "Flow data should not be empty")
	ts.Require().NotEmpty(flowStep.Data.Inputs, "Flow should require inputs")

	ts.Require().True(common.HasInput(flowStep.Data.Inputs, "mobileNumber"),
		"Mobile number input should be required")

	// Clear any previous messages
	ts.mockServer.ClearMessages()

	// Step 2: Continue the flow with mobile number
	userAttrs, err := testutils.GetUserAttributes(testUserWithMobile)
	ts.Require().NoError(err, "Failed to get user attributes")

	inputs := map[string]string{
		"mobileNumber": userAttrs["mobileNumber"].(string),
	}

	otpFlowStep, err := common.CompleteFlow(flowStep.ExecutionID, inputs, "action_001",
		flowStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete authentication flow with mobile number: %v", err)
	}

	// Verify OTP input is now required
	ts.Require().Equal("INCOMPLETE", otpFlowStep.FlowStatus, "Expected flow status to be INCOMPLETE")
	ts.Require().Equal("VIEW", otpFlowStep.Type, "Expected flow type to be VIEW")
	ts.Require().NotEmpty(otpFlowStep.Data, "Flow data should not be empty")
	ts.Require().NotEmpty(otpFlowStep.Data.Inputs, "Flow should require inputs")

	ts.Require().True(common.HasInput(otpFlowStep.Data.Inputs, "otp"),
		"OTP input should be required")

	// Wait for SMS to be sent
	time.Sleep(500 * time.Millisecond)

	// Verify SMS was sent
	lastMessage := ts.mockServer.GetLastMessage()
	ts.Require().NotNil(lastMessage, "Last message should not be nil")
	ts.Require().NotEmpty(lastMessage.OTP, "OTP should be extracted from message")

	// Step 3: Complete authentication with OTP
	otpInputs := map[string]string{
		"otp": lastMessage.OTP,
	}

	completeFlowStep, err := common.CompleteFlow(flowStep.ExecutionID, otpInputs, "action_002",
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
		smsAuthTestAppID,
		smsAuthEntityType.Name,
		smsAuthTestOU.ID,
		smsAuthTestOU.Name,
		smsAuthTestOU.Handle,
	)
	ts.Require().NoError(err, "Failed to validate JWT assertion fields")
	ts.Require().NotNil(jwtClaims, "JWT claims should not be nil")
}

func (ts *SMSAuthFlowTestSuite) TestSMSAuthFlowWithUsername() {
	// Update app to use SMS flow with username
	err := common.UpdateAppConfig(smsAuthTestAppID, smsAuthFlowUsernameID, "")
	if err != nil {
		ts.T().Fatalf("Failed to update app config for SMS flow with username: %v", err)
	}
	defer func() {
		// Restore to mobile flow for other tests
		_ = common.UpdateAppConfig(smsAuthTestAppID, smsAuthFlowMobileID, "")
	}()

	// Step 1: Initialize the flow
	flowStep, err := common.InitiateAuthenticationFlow(smsAuthTestAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate authentication flow: %v", err)
	}

	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus, "Expected flow status to be INCOMPLETE")
	ts.Require().Equal("VIEW", flowStep.Type, "Expected flow type to be VIEW")
	ts.Require().NotEmpty(flowStep.ExecutionID, "Execution ID should not be empty")

	// Validate that username input is required
	ts.Require().NotEmpty(flowStep.Data, "Flow data should not be empty")
	ts.Require().NotEmpty(flowStep.Data.Inputs, "Flow should require inputs")

	var hasUsername bool
	for _, input := range flowStep.Data.Inputs {
		if input.Identifier == "username" {
			hasUsername = true
		}
	}
	ts.Require().True(hasUsername, "Username input should be required")

	// Clear any previous messages
	ts.mockServer.ClearMessages()

	// Step 2: Continue the flow with username
	var userAttrs map[string]interface{}
	err = json.Unmarshal(testUserWithMobile.Attributes, &userAttrs)
	ts.Require().NoError(err, "Failed to unmarshal user attributes")

	inputs := map[string]string{
		"username": userAttrs["username"].(string),
	}

	otpFlowStep, err := common.CompleteFlow(flowStep.ExecutionID, inputs, "action_001",
		flowStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete authentication flow with username: %v", err)
	}

	// Verify OTP input is now required
	ts.Require().Equal("INCOMPLETE", otpFlowStep.FlowStatus, "Expected flow status to be INCOMPLETE")
	ts.Require().Equal("VIEW", otpFlowStep.Type, "Expected flow type to be VIEW")
	ts.Require().NotEmpty(otpFlowStep.Data, "Flow data should not be empty")
	ts.Require().NotEmpty(otpFlowStep.Data.Inputs, "Flow should require inputs")

	var hasOTP bool
	for _, input := range otpFlowStep.Data.Inputs {
		if input.Identifier == "otp" {
			hasOTP = true
		}
	}
	ts.Require().True(hasOTP, "OTP input should be required")

	// Wait for SMS to be sent
	time.Sleep(500 * time.Millisecond)

	// Verify SMS was sent
	lastMessage := ts.mockServer.GetLastMessage()
	ts.Require().NotNil(lastMessage, "Last message should not be nil")
	ts.Require().NotEmpty(lastMessage.OTP, "OTP should be extracted from message")

	// Step 3: Complete authentication with OTP
	otpInputs := map[string]string{
		"otp": lastMessage.OTP,
	}

	completeFlowStep, err := common.CompleteFlow(flowStep.ExecutionID, otpInputs, "action_002",
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
		smsAuthTestAppID,
		smsAuthEntityType.Name,
		smsAuthTestOU.ID,
		smsAuthTestOU.Name,
		smsAuthTestOU.Handle,
	)
	ts.Require().NoError(err, "Failed to validate JWT assertion fields")
	ts.Require().NotNil(jwtClaims, "JWT claims should not be nil")
}

func (ts *SMSAuthFlowTestSuite) TestSMSAuthFlowInvalidOTP() {
	// Step 1: Initialize the flow and provide mobile number
	var userAttrs map[string]interface{}
	err := json.Unmarshal(testUserWithMobile.Attributes, &userAttrs)
	ts.Require().NoError(err, "Failed to unmarshal user attributes")

	inputs := map[string]string{
		"mobileNumber": userAttrs["mobileNumber"].(string),
	}

	flowStep, err := common.InitiateAuthenticationFlow(smsAuthTestAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate authentication flow: %v", err)
	}

	// Clear any previous messages
	ts.mockServer.ClearMessages()

	// Continue flow to trigger OTP sending
	otpFlowStep, err := common.CompleteFlow(flowStep.ExecutionID, inputs, "action_001",
		flowStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete authentication flow with mobile number: %v", err)
	}

	ts.Require().Equal("INCOMPLETE", otpFlowStep.FlowStatus, "Expected flow status to be INCOMPLETE")

	// Wait for SMS to be sent
	time.Sleep(500 * time.Millisecond)

	// Step 2: Try with invalid OTP
	invalidOTPInputs := map[string]string{
		"otp": "000000", // Invalid OTP
	}

	completeFlowStep, err := common.CompleteFlow(otpFlowStep.ExecutionID, invalidOTPInputs, "action_002",
		otpFlowStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete authentication flow with invalid OTP: %v", err)
	}

	// Verify authentication failure returns INCOMPLETE (retryable)
	ts.Require().Equal("INCOMPLETE", completeFlowStep.FlowStatus,
		"Expected flow status to be INCOMPLETE for invalid OTP")
	ts.Require().Equal("VIEW", completeFlowStep.Type, "Expected type to be VIEW for prompt re-display")
	ts.Require().Empty(completeFlowStep.Assertion, "No JWT assertion should be returned for failed authentication")
	ts.Require().NotEmpty(completeFlowStep.FailureReason, "Failure reason should be provided for invalid OTP")

	// Verify OTP input is re-prompted
	ts.Require().NotEmpty(completeFlowStep.Data, "Flow data should not be empty after re-prompt")
	ts.Require().True(common.HasInput(completeFlowStep.Data.Inputs, "otp"),
		"OTP input should be re-prompted after invalid OTP")
}

func (ts *SMSAuthFlowTestSuite) TestSMSAuthFlowRetryAfterInvalidOTP() {
	// Step 1: Initialize the flow and provide mobile number
	var userAttrs map[string]interface{}
	err := json.Unmarshal(testUserWithMobile.Attributes, &userAttrs)
	ts.Require().NoError(err, "Failed to unmarshal user attributes")

	inputs := map[string]string{
		"mobileNumber": userAttrs["mobileNumber"].(string),
	}

	flowStep, err := common.InitiateAuthenticationFlow(smsAuthTestAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate authentication flow: %v", err)
	}
	ts.Require().NotEmpty(flowStep.ExecutionID, "Execution ID should not be empty")

	// Clear any previous messages
	ts.mockServer.ClearMessages()

	// Step 2: Continue flow to trigger OTP sending
	otpFlowStep, err := common.CompleteFlow(flowStep.ExecutionID, inputs, "action_001",
		flowStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete authentication flow with mobile number: %v", err)
	}

	ts.Require().Equal("INCOMPLETE", otpFlowStep.FlowStatus, "Expected flow status to be INCOMPLETE after OTP send")
	ts.Require().NotEmpty(otpFlowStep.ExecutionID, "Execution ID should not be empty")

	// Wait for SMS to be sent
	time.Sleep(500 * time.Millisecond)

	// Capture the valid OTP before submitting an invalid one
	lastMessage := ts.mockServer.GetLastMessage()
	ts.Require().NotNil(lastMessage, "SMS should have been sent")
	ts.Require().NotEmpty(lastMessage.OTP, "OTP should be available from mock server")
	validOTP := lastMessage.OTP

	// Step 3: Submit invalid OTP
	invalidOTPInputs := map[string]string{
		"otp": "000000", // Invalid OTP
	}

	retryFlowStep, err := common.CompleteFlow(otpFlowStep.ExecutionID, invalidOTPInputs, "action_002",
		otpFlowStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete authentication flow with invalid OTP: %v", err)
	}

	// Verify we get INCOMPLETE (retryable) not ERROR
	ts.Require().Equal("INCOMPLETE", retryFlowStep.FlowStatus, "Expected INCOMPLETE after invalid OTP")
	ts.Require().NotEmpty(retryFlowStep.FailureReason, "Failure reason should be present for invalid OTP")

	// Verify OTP input is re-prompted
	ts.Require().NotEmpty(retryFlowStep.Data, "Flow data should not be empty after re-prompt")
	ts.Require().True(common.HasInput(retryFlowStep.Data.Inputs, "otp"),
		"OTP input should be re-prompted for retry")

	// Step 4: Retry with the valid OTP
	validOTPInputs := map[string]string{
		"otp": validOTP,
	}

	successFlowStep, err := common.CompleteFlow(otpFlowStep.ExecutionID, validOTPInputs, "action_002",
		retryFlowStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete authentication flow after retry with valid OTP: %v", err)
	}

	// Verify successful authentication
	ts.Require().Equal("COMPLETE", successFlowStep.FlowStatus,
		"Expected COMPLETE after retry with valid OTP")
	ts.Require().NotEmpty(successFlowStep.Assertion, "JWT assertion should be returned on successful retry")
	ts.Require().Empty(successFlowStep.FailureReason, "No failure reason on success")
}

func (ts *SMSAuthFlowTestSuite) TestSMSAuthFlowSingleRequestWithMobileNumber() {
	// Clear any previous messages
	ts.mockServer.ClearMessages()

	// Get user attributes
	var userAttrs map[string]interface{}
	err := json.Unmarshal(testUserWithMobile.Attributes, &userAttrs)
	ts.Require().NoError(err, "Failed to unmarshal user attributes")

	// Step 1: Initialize the flow with mobile number - single action should auto-select
	inputs := map[string]string{
		"mobileNumber": userAttrs["mobileNumber"].(string),
	}

	flowStep, err := common.InitiateAuthenticationFlow(smsAuthTestAppID, false, inputs, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate authentication flow: %v", err)
	}

	// Should require OTP input now (single action was auto-selected)
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus, "Expected flow status to be INCOMPLETE")
	ts.Require().Equal("VIEW", flowStep.Type, "Expected flow type to be VIEW")
	ts.Require().True(common.HasInput(flowStep.Data.Inputs, "otp"), "OTP input should be required")

	// Wait for SMS to be sent
	time.Sleep(500 * time.Millisecond)

	// Get the OTP from mock server
	lastMessage := ts.mockServer.GetLastMessage()
	ts.Require().NotNil(lastMessage, "SMS should have been sent")
	ts.Require().NotEmpty(lastMessage.OTP, "OTP should be available")

	// Step 2: Complete with OTP - single action should auto-select
	otpInputs := map[string]string{
		"otp": lastMessage.OTP,
	}

	completeFlowStep, err := common.CompleteFlow(flowStep.ExecutionID, otpInputs, "", flowStep.ChallengeToken)
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
		smsAuthTestAppID,
		smsAuthEntityType.Name,
		smsAuthTestOU.ID,
		smsAuthTestOU.Name,
		smsAuthTestOU.Handle,
	)
	ts.Require().NoError(err, "Failed to validate JWT assertion fields")
	ts.Require().NotNil(jwtClaims, "JWT claims should not be nil")
}
