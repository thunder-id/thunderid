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
	"fmt"
	"testing"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

var (
	smsRegistrationFlow = testutils.Flow{
		Name:     "SMS Test Registration Flow",
		FlowType: "REGISTRATION",
		Handle:   "registration_flow_sms_test",
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
				"onSuccess":    "prompt_mobile",
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
								"ref":        "input_otp",
								"identifier": "otp",
								"type":       "string",
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
				"onSuccess": "provisioning",
			},
			{
				"id":   "provisioning",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "ProvisioningExecutor",
					"inputs": []map[string]interface{}{
						{
							"ref":        "input_002",
							"identifier": "given_name",
							"type":       "string",
							"required":   false,
						},
						{
							"ref":        "input_003",
							"identifier": "family_name",
							"type":       "string",
							"required":   false,
						},
						{
							"ref":        "input_004",
							"identifier": "email",
							"type":       "string",
							"required":   true,
						},
						{
							"ref":        "input_005",
							"identifier": "mobileNumber",
							"type":       "string",
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

	smsRegTestOU = testutils.OrganizationUnit{
		Handle:      "sms-reg-flow-test-ou",
		Name:        "SMS Registration Flow Test Organization Unit",
		Description: "Organization unit for SMS registration flow testing",
		Parent:      nil,
	}

	smsRegTestEntityType = testutils.UserType{
		Name: "sms-test-user-type",
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
		AllowSelfRegistration: true,
	}

	smsRegTestApp = testutils.Application{
		Name:                      "SMS Registration Flow Test Application",
		Description:               "Application for testing SMS registration flows",
		IsRegistrationFlowEnabled: true,
		ClientID:                  "sms_reg_flow_test_client",
		ClientSecret:              "sms_reg_flow_test_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{smsRegTestEntityType.Name},
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{"userType", "ouId", "ouName", "ouHandle"},
		},
	}
)

const (
	mockNotificationServerPort = 8098
)

type SMSRegistrationFlowTestSuite struct {
	suite.Suite
	config       *common.TestSuiteConfig
	mockServer   *testutils.MockNotificationServer
	entityTypeID string
	testAppID    string
	testOUID     string
}

func TestSMSRegistrationFlowTestSuite(t *testing.T) {
	suite.Run(t, new(SMSRegistrationFlowTestSuite))
}

func (ts *SMSRegistrationFlowTestSuite) SetupSuite() {
	// Initialize config
	ts.config = &common.TestSuiteConfig{}

	// Create test organization unit for SMS tests
	ouID, err := testutils.CreateOrganizationUnit(smsRegTestOU)
	if err != nil {
		ts.T().Fatalf("Failed to create test organization unit during setup: %v", err)
	}
	ts.testOUID = ouID

	// Create test user type for SMS tests
	smsRegTestEntityType.OUID = ts.testOUID
	schemaID, err := testutils.CreateUserType(smsRegTestEntityType)
	if err != nil {
		ts.T().Fatalf("Failed to create test user type during setup: %v", err)
	}
	ts.entityTypeID = schemaID

	// Start mock notification server
	ts.mockServer = testutils.NewMockNotificationServer(mockNotificationServerPort)
	err = ts.mockServer.Start()
	if err != nil {
		ts.T().Fatalf("Failed to start mock notification server: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	ts.T().Log("Mock notification server started successfully")

	// Create notification sender for SMS flows
	customSender := testutils.NotificationSender{
		Name:        "SMS Registration Test Notification Sender",
		Description: "Notification sender for SMS registration flow testing",
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
	ts.config.CreatedSenderIDs = append(ts.config.CreatedSenderIDs, senderID)

	// Update registration flow with created sender ID
	smsNodes := smsRegistrationFlow.Nodes.([]map[string]interface{})
	smsNodes[4]["properties"].(map[string]interface{})["senderId"] = senderID // sms_otp_send node
	smsNodes[6]["properties"].(map[string]interface{})["senderId"] = senderID // sms_otp_verify node
	smsRegistrationFlow.Nodes = smsNodes

	// Create the SMS registration flow
	flowID, err := testutils.CreateFlow(smsRegistrationFlow)
	ts.Require().NoError(err, "Failed to create SMS registration flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)
	smsRegTestApp.RegistrationFlowID = flowID

	// Create test application with allowed user types
	smsRegTestApp.OUID = ts.testOUID
	appID, err := testutils.CreateApplication(smsRegTestApp)
	if err != nil {
		ts.T().Fatalf("Failed to create test application during setup: %v", err)
	}
	ts.testAppID = appID
}

func (ts *SMSRegistrationFlowTestSuite) TearDownSuite() {
	// Delete test users created during SMS registration tests
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
	if ts.testAppID != "" {
		if err := testutils.DeleteApplication(ts.testAppID); err != nil {
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
	if ts.testOUID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.testOUID); err != nil {
			ts.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}

	if ts.entityTypeID != "" {
		if err := testutils.DeleteUserType(ts.entityTypeID); err != nil {
			ts.T().Logf("Failed to delete test user type during teardown: %v", err)
		}
	}

}

func (ts *SMSRegistrationFlowTestSuite) TestSMSRegistrationFlow() {
	// Generate unique mobile number for registration
	mobileNumber := generateUniqueMobileNumber()

	// Step 1: Initialize the registration flow by calling the flow execution API
	flowStep, err := common.InitiateRegistrationFlow(ts.testAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate registration flow: %v", err)
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
	inputs := map[string]string{
		"mobileNumber": mobileNumber,
	}

	otpFlowStep, err := common.CompleteFlow(flowStep.ExecutionID, inputs, "action_001",
		flowStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete registration flow with mobile number: %v", err)
	}

	// Verify OTP input is now required
	ts.Require().Equal("INCOMPLETE", otpFlowStep.FlowStatus, "Expected flow status to be INCOMPLETE")
	ts.Require().Equal("VIEW", otpFlowStep.Type, "Expected flow type to be VIEW")
	ts.Require().NotEmpty(otpFlowStep.Data, "Flow data should not be empty")
	ts.Require().NotEmpty(otpFlowStep.Data.Inputs, "Flow should require inputs")
	ts.Require().True(common.HasInput(otpFlowStep.Data.Inputs, "otp"), "OTP input should be required")

	// Wait for SMS to be sent
	time.Sleep(1 * time.Second)

	// Verify SMS was sent
	lastMessage := ts.mockServer.GetLastMessage()
	ts.Require().NotNil(lastMessage, "Last message should not be nil")
	ts.Require().NotEmpty(lastMessage.OTP, "OTP should be extracted from message")

	// Step 3: Complete registration with OTP
	otpInputs := map[string]string{
		"otp": lastMessage.OTP,
	}

	completeFlowStep, err := common.CompleteFlow(flowStep.ExecutionID, otpInputs, "action_otp",
		otpFlowStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete registration flow with OTP: %v", err)
	}

	ts.Require().Equal("INCOMPLETE", completeFlowStep.FlowStatus,
		"Expected flow status to be INCOMPLETE after OTP input")
	ts.Require().Equal("VIEW", completeFlowStep.Type, "Expected flow type to be VIEW after OTP input")
	ts.Require().NotEmpty(completeFlowStep.Data, "Flow data should not be empty after OTP input")
	ts.Require().NotEmpty(completeFlowStep.Data.Inputs, "Flow should require inputs after OTP input")
	ts.Require().True(common.ValidateRequiredInputs(completeFlowStep.Data.Inputs, []string{"email"}),
		"Email should be a required inputs after first step")

	// Step 4: Provide additional attributes
	fillInputs := []common.Inputs{
		{
			Identifier: "given_name",
			Type:       "string",
			Required:   true,
		},
		{
			Identifier: "family_name",
			Type:       "string",
			Required:   true,
		},
	}
	fillInputs = append(fillInputs, completeFlowStep.Data.Inputs...)
	attrInputs := fillRequiredRegistrationAttributes(fillInputs, mobileNumber)
	completeFlowStep, err = common.CompleteFlow(flowStep.ExecutionID, attrInputs, "", completeFlowStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete registration flow with attributes: %v", err)
	}

	// Verify successful registration
	ts.Require().Equal("COMPLETE", completeFlowStep.FlowStatus, "Expected flow status to be COMPLETE")
	ts.Require().NotEmpty(completeFlowStep.Assertion,
		"JWT assertion should be returned after successful registration")
	ts.Require().Empty(completeFlowStep.FailureReason,
		"Failure reason should be empty for successful registration")

	// Decode and validate JWT claims
	jwtClaims, err := testutils.DecodeJWT(completeFlowStep.Assertion)
	ts.Require().NoError(err, "Failed to decode JWT assertion")
	ts.Require().NotNil(jwtClaims, "JWT claims should not be nil")

	// Validate JWT contains expected user type and OU ID
	ts.Require().Equal(smsRegTestEntityType.Name, jwtClaims.UserType, "Expected userType to match created schema")
	ts.Require().Equal(ts.testOUID, jwtClaims.OUID, "Expected ouId to match the created organization unit")
	ts.Require().Equal(ts.testAppID, jwtClaims.Aud, "Expected aud to match the application ID")
	ts.Require().NotEmpty(jwtClaims.Sub, "JWT subject should not be empty")

	// Step 5: Verify the user was created by searching via the user API
	user, err := testutils.FindUserByAttribute("mobileNumber", mobileNumber)
	if err != nil {
		ts.T().Fatalf("Failed to retrieve user by mobile number: %v", err)
	}
	ts.Require().NotNil(user, "User should be found in user list after registration")

	// Store the created user for cleanup
	if user != nil {
		ts.config.CreatedUserIDs = append(ts.config.CreatedUserIDs, user.ID)
	}
}

func (ts *SMSRegistrationFlowTestSuite) TestSMSRegistrationFlowInvalidOTP() {
	// Switch back to SMS flow with mobile number (first flow)
	if len(ts.config.CreatedFlowIDs) < 1 {
		ts.T().Fatalf("Expected at least 1 flow to be created during setup")
	}
	err := common.UpdateAppConfig(ts.testAppID, "", ts.config.CreatedFlowIDs[0])
	if err != nil {
		ts.T().Fatalf("Failed to update app config for SMS flow: %v", err)
	}

	// Generate unique mobile number
	mobileNumber := generateUniqueMobileNumber()

	// Step 1: Initialize the registration flow and provide mobile number
	inputs := map[string]string{
		"mobileNumber": mobileNumber,
	}

	flowStep, err := common.InitiateRegistrationFlow(ts.testAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate registration flow: %v", err)
	}

	// Clear any previous messages
	ts.mockServer.ClearMessages()

	// Continue flow to trigger OTP sending
	otpFlowStep, err := common.CompleteFlow(flowStep.ExecutionID, inputs, "action_001",
		flowStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete registration flow with mobile number: %v", err)
	}

	ts.Require().Equal("INCOMPLETE", otpFlowStep.FlowStatus, "Expected flow status to be INCOMPLETE")

	// Wait for SMS to be sent
	time.Sleep(1 * time.Second)

	// Step 2: Try with invalid OTP
	invalidOTPInputs := map[string]string{
		"otp": "000000",
	}

	completeFlowStep, err := common.CompleteFlow(flowStep.ExecutionID, invalidOTPInputs, "action_otp",
		otpFlowStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete registration flow with invalid OTP: %v", err)
	}

	// Verify registration is incomplete (invalid OTP triggers retry)
	ts.Require().Equal("INCOMPLETE", completeFlowStep.FlowStatus, "Expected flow status to be INCOMPLETE for invalid OTP")
	ts.Require().Empty(completeFlowStep.Assertion, "No JWT assertion should be returned for failed OTP")
	ts.Require().NotEmpty(completeFlowStep.FailureReason, "Failure reason should be provided for invalid OTP")
	ts.Equal("invalid OTP provided", completeFlowStep.FailureReason,
		"Expected failure reason to indicate invalid OTP")
}

func (ts *SMSRegistrationFlowTestSuite) TestSMSRegistrationFlowSingleRequestWithMobileNumber() {
	// Switch back to SMS flow with mobile number (first flow)
	if len(ts.config.CreatedFlowIDs) < 1 {
		ts.T().Fatalf("Expected at least 1 flow to be created during setup")
	}
	err := common.UpdateAppConfig(ts.testAppID, "", ts.config.CreatedFlowIDs[0])
	if err != nil {
		ts.T().Fatalf("Failed to update app config for SMS flow: %v", err)
	}

	// Clear any previous messages
	ts.mockServer.ClearMessages()

	// Generate unique mobile number
	mobileNumber := generateUniqueMobileNumber()

	// Step 1: Initialize the registration flow
	flowStep, err := common.InitiateRegistrationFlow(ts.testAppID, false, nil, "")
	if err != nil {
		ts.T().Fatalf("Failed to initiate registration flow: %v", err)
	}

	// Step 2: Provide mobile number with action to trigger SMS
	ts.mockServer.ClearMessages()
	inputs := map[string]string{
		"mobileNumber": mobileNumber,
	}

	otpStep, err := common.CompleteFlow(flowStep.ExecutionID, inputs, "action_001",
		flowStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to provide mobile number: %v", err)
	}

	// Should require OTP input now
	ts.Require().Equal("INCOMPLETE", otpStep.FlowStatus, "Expected flow status to be INCOMPLETE")
	ts.Require().Equal("VIEW", otpStep.Type, "Expected flow type to be VIEW")

	// Wait for SMS to be sent
	time.Sleep(1 * time.Second)

	// Get the OTP from mock server
	lastMessage := ts.mockServer.GetLastMessage()
	ts.Require().NotNil(lastMessage, "SMS should have been sent")
	ts.Require().NotEmpty(lastMessage.OTP, "OTP should be available")

	// Step 3: Complete with OTP
	otpInputs := map[string]string{
		"otp": lastMessage.OTP,
	}

	provisionStep, err := common.CompleteFlow(otpStep.ExecutionID, otpInputs, "action_otp",
		otpStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete registration flow with OTP: %v", err)
	}

	// Should now require user attributes
	ts.Require().Equal("INCOMPLETE", provisionStep.FlowStatus, "Expected flow status to be INCOMPLETE for provisioning")

	// Step 4: Provide user attributes
	userInputs := map[string]string{
		"given_name":   "Test",
		"family_name":  "User",
		"email":        fmt.Sprintf("%s@example.com", mobileNumber),
		"mobileNumber": mobileNumber,
	}

	completeFlowStep, err := common.CompleteFlow(provisionStep.ExecutionID, userInputs, "",
		provisionStep.ChallengeToken)
	if err != nil {
		ts.T().Fatalf("Failed to complete registration with user attributes: %v", err)
	}

	// Verify successful registration
	ts.Require().Equal("COMPLETE", completeFlowStep.FlowStatus, "Expected flow status to be COMPLETE")
	ts.Require().NotEmpty(completeFlowStep.Assertion,
		"JWT assertion should be returned after successful registration")
	ts.Require().Empty(completeFlowStep.FailureReason,
		"Failure reason should be empty for successful registration")

	// Decode and validate JWT claims
	jwtClaims, err := testutils.DecodeJWT(completeFlowStep.Assertion)
	ts.Require().NoError(err, "Failed to decode JWT assertion")
	ts.Require().NotNil(jwtClaims, "JWT claims should not be nil")

	// Validate JWT contains expected user type and OU ID
	ts.Require().Equal(smsRegTestEntityType.Name, jwtClaims.UserType, "Expected userType to match created schema")
	ts.Require().Equal(ts.testOUID, jwtClaims.OUID, "Expected ouId to match the created organization unit")
	ts.Require().Equal(ts.testAppID, jwtClaims.Aud, "Expected aud to match the application ID")
	ts.Require().NotEmpty(jwtClaims.Sub, "JWT subject should not be empty")

	// Step 3: Verify the user was created by searching via the user API
	user, err := testutils.FindUserByAttribute("mobileNumber", mobileNumber)
	if err != nil {
		ts.T().Fatalf("Failed to retrieve user by mobile number: %v", err)
	}
	ts.Require().NotNil(user, "User should be found in user list after registration")

	// Store the created user for cleanup
	if user != nil {
		ts.config.CreatedUserIDs = append(ts.config.CreatedUserIDs, user.ID)
	}
}

// Helper to fill required attributes for registration
func fillRequiredRegistrationAttributes(inputs []common.Inputs, mobile string) map[string]string {
	attrInputs := map[string]string{}
	for _, input := range inputs {
		if input.Required {
			switch input.Identifier {
			case "given_name":
				attrInputs["given_name"] = "Test"
			case "family_name":
				attrInputs["family_name"] = "User"
			case "email":
				attrInputs["email"] = fmt.Sprintf("%s@example.com", mobile)
			default:
				attrInputs[input.Identifier] = "dummy"
			}
		}
	}
	return attrInputs
}
