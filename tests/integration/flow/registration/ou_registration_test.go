/*
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	mockNotificationServerPortOU = 8098
)

var (
	basicRegistrationFlowWithOU = testutils.Flow{
		Name:     "Basic Registration Flow with OU Test",
		FlowType: "REGISTRATION",
		Handle:   "registration_flow_basic_with_ou_test",
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
				"onSuccess":    "credentials_auth",
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
				"id":   "credentials_auth",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "CredentialsAuthExecutor",
				},
				"onSuccess": "ou_creation",
			},
			{
				"id":   "ou_creation",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "OUExecutor",
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
							"ref":        "input_001",
							"identifier": "username",
							"type":       "string",
							"required":   true,
						},
						{
							"ref":        "input_002",
							"identifier": "password",
							"type":       "string",
							"required":   true,
						},
						{
							"ref":        "input_003",
							"identifier": "given_name",
							"type":       "string",
							"required":   true,
						},
						{
							"ref":        "input_004",
							"identifier": "family_name",
							"type":       "string",
							"required":   true,
						},
						{
							"ref":        "input_005",
							"identifier": "email",
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

	smsRegistrationFlowWithOU = testutils.Flow{
		Name:     "SMS Registration Flow with OU Test",
		FlowType: "REGISTRATION",
		Handle:   "registration_flow_sms_with_ou_test",
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
								"identifier": "mobile_number",
								"type":       "TEXT_INPUT",
								"required":   true,
							},
						},
						"action": map[string]interface{}{
							"ref":      "action_001",
							"nextNode": "generate_otp",
						},
					},
				},
			},
			{
				"id":   "generate_otp",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "OTPExecutor",
					"mode": "generate",
					"inputs": []map[string]interface{}{
						{
							"ref":        "input_001",
							"identifier": "mobile_number",
							"type":       "PHONE_INPUT",
							"required":   true,
						},
					},
				},
				"onSuccess": "sms_send",
			},
			{
				"id":   "sms_send",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"senderId":    "placeholder-sender-id",
					"smsTemplate": "OTP",
				},
				"executor": map[string]interface{}{
					"name": "SMSExecutor",
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
								"type":       "TEXT_INPUT",
								"required":   true,
							},
						},
						"action": map[string]interface{}{
							"ref":      "action_002",
							"nextNode": "verify_otp",
						},
					},
				},
			},
			{
				"id":   "verify_otp",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "OTPExecutor",
					"mode": "verify",
				},
				"onSuccess": "ou_creation",
			},
			{
				"id":   "ou_creation",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "OUExecutor",
					"inputs": []map[string]interface{}{
						{
							"ref":        "input_003",
							"identifier": "ouName",
							"type":       "TEXT_INPUT",
							"required":   true,
						},
						{
							"ref":        "input_004",
							"identifier": "ouHandle",
							"type":       "TEXT_INPUT",
							"required":   true,
						},
						{
							"ref":        "input_005",
							"identifier": "ouDescription",
							"type":       "TEXT_INPUT",
							"required":   false,
						},
					},
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
							"ref":        "input_006",
							"identifier": "mobile_number",
							"type":       "TEXT_INPUT",
							"required":   true,
						},
						{
							"ref":        "input_007",
							"identifier": "given_name",
							"type":       "TEXT_INPUT",
							"required":   true,
						},
						{
							"ref":        "input_008",
							"identifier": "family_name",
							"type":       "TEXT_INPUT",
							"required":   true,
						},
						{
							"ref":        "input_009",
							"identifier": "email",
							"type":       "TEXT_INPUT",
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

	ouRegTestApp = testutils.Application{
		Name:                      "OU Registration Flow Test Application",
		Description:               "Application for testing OU registration flows",
		IsRegistrationFlowEnabled: true,
		ClientID:                  "ou_reg_flow_test_client",
		ClientSecret:              "ou_reg_flow_test_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{dynamicEntityType.Name},
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{"userType", "ouId", "ouName", "ouHandle"},
		},
	}

	smsApp = testutils.Application{
		Name:                      "OU SMS Registration Flow Test Application",
		Description:               "Application for testing OU SMS registration flows",
		IsRegistrationFlowEnabled: true,
		ClientID:                  "ou_sms_reg_flow_test_client",
		ClientSecret:              "ou_sms_reg_flow_test_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{dynamicEntityType.Name},
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{"userType", "ouId", "ouName", "ouHandle"},
		},
	}

	ouRegTestOU = testutils.OrganizationUnit{
		Handle:      "ou-reg-flow-test-ou",
		Name:        "OU Registration Flow Test Organization Unit",
		Description: "Organization unit for OU registration flow testing",
		Parent:      nil,
	}

	smsOU = testutils.OrganizationUnit{
		Handle:      "ou-sms-reg-flow-test-ou",
		Name:        "OU SMS Registration Flow Test Organization Unit",
		Description: "Organization unit for OU SMS registration flow testing",
		Parent:      nil,
	}

	dynamicEntityType = testutils.UserType{
		Name: "dynamic-user-type",
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
			"mobile_number": map[string]interface{}{
				"type": "string",
			},
		},
	}
)

type OURegistrationFlowTestSuite struct {
	suite.Suite
	config             *common.TestSuiteConfig
	mockServer         *testutils.MockNotificationServer
	createdOUs         []string
	basicFlowTestAppID string
	basicFlowTestOUID  string
	smsFlowTestAppID   string
	smsFlowTestOUID    string
	entityTypeID       string
}

func TestOURegistrationFlowTestSuite(t *testing.T) {
	suite.Run(t, new(OURegistrationFlowTestSuite))
}

func (ts *OURegistrationFlowTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}
	ts.createdOUs = []string{}

	// Create organization units
	ouID, err := testutils.CreateOrganizationUnit(ouRegTestOU)
	if err != nil {
		ts.T().Fatalf("Failed to create test organization unit during setup: %v", err)
	}
	ts.basicFlowTestOUID = ouID

	smsOUID, err := testutils.CreateOrganizationUnit(smsOU)
	if err != nil {
		ts.T().Fatalf("Failed to create SMS test organization unit during setup: %v", err)
	}
	ts.smsFlowTestOUID = smsOUID

	// Create dynamic user type
	dynamicEntityType.OUID = ts.basicFlowTestOUID
	dynamicEntityType.AllowSelfRegistration = true
	schemaID, err := testutils.CreateUserType(dynamicEntityType)
	if err != nil {
		ts.T().Fatalf("Failed to create dynamic user type during setup: %v", err)
	}
	ts.entityTypeID = schemaID

	// Start mock notification server for SMS flow
	ts.mockServer = testutils.NewMockNotificationServer(mockNotificationServerPortOU)
	err = ts.mockServer.Start()
	if err != nil {
		ts.T().Fatalf("Failed to start mock notification server: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	// Create notification sender for SMS flow
	customSender := testutils.NotificationSender{
		Name:        "OU SMS Test Notification Sender",
		Description: "Notification sender for OU SMS registration flow test",
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

	// Create basic registration flow with OU
	basicFlowID, err := testutils.CreateFlow(basicRegistrationFlowWithOU)
	ts.Require().NoError(err, "Failed to create basic registration flow with OU")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, basicFlowID)
	ouRegTestApp.RegistrationFlowID = basicFlowID

	// Update SMS flow definition with created sender ID
	smsNodes := smsRegistrationFlowWithOU.Nodes.([]map[string]interface{})
	smsNodes[4]["properties"].(map[string]interface{})["senderId"] = senderID
	smsRegistrationFlowWithOU.Nodes = smsNodes

	// Create SMS registration flow with OU
	smsFlowID, err := testutils.CreateFlow(smsRegistrationFlowWithOU)
	ts.Require().NoError(err, "Failed to create SMS registration flow with OU")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, smsFlowID)
	smsApp.RegistrationFlowID = smsFlowID

	// Create test applications with allowed user types
	ouRegTestApp.OUID = ts.basicFlowTestOUID
	appID, err := testutils.CreateApplication(ouRegTestApp)
	if err != nil {
		ts.T().Fatalf("Failed to create test application during setup: %v", err)
	}
	ts.basicFlowTestAppID = appID

	smsApp.OUID = ts.smsFlowTestOUID
	smsAppID, err := testutils.CreateApplication(smsApp)
	if err != nil {
		ts.T().Fatalf("Failed to create SMS test application during setup: %v", err)
	}
	ts.smsFlowTestAppID = smsAppID
}

func (ts *OURegistrationFlowTestSuite) TearDownSuite() {
	// Stop mock notification server
	if ts.mockServer != nil {
		err := ts.mockServer.Stop()
		if err != nil {
			ts.T().Logf("Failed to stop mock notification server during teardown: %v", err)
		}
	}

	// Delete users
	if err := testutils.CleanupUsers(ts.config.CreatedUserIDs); err != nil {
		ts.T().Logf("Failed to cleanup users during teardown: %v", err)
	}

	// Delete dynamically created OUs
	for _, ouID := range ts.createdOUs {
		if err := testutils.DeleteOrganizationUnit(ouID); err != nil {
			ts.T().Logf("Failed to delete created OU %s during teardown: %v", ouID, err)
		}
	}

	// Delete test applications
	if ts.basicFlowTestAppID != "" {
		if err := testutils.DeleteApplication(ts.basicFlowTestAppID); err != nil {
			ts.T().Logf("Failed to delete test application during teardown: %v", err)
		}
	}
	if ts.smsFlowTestAppID != "" {
		if err := testutils.DeleteApplication(ts.smsFlowTestAppID); err != nil {
			ts.T().Logf("Failed to delete SMS test application during teardown: %v", err)
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

	// Delete test organization units
	if ts.smsFlowTestOUID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.smsFlowTestOUID); err != nil {
			ts.T().Logf("Failed to delete SMS test organization unit during teardown: %v", err)
		}
	}
	if ts.basicFlowTestOUID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.basicFlowTestOUID); err != nil {
			ts.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}

	// delete user type
	if ts.entityTypeID != "" {
		if err := testutils.DeleteUserType(ts.entityTypeID); err != nil {
			ts.T().Logf("Failed to delete dynamic user type during teardown: %v", err)
		}
	}
}

func (ts *OURegistrationFlowTestSuite) TestBasicRegistrationFlowWithOU() {
	testCases := []struct {
		name          string
		ouName        string
		ouHandle      string
		ouDescription string
	}{
		{
			name:          "SuccessWithDescription",
			ouName:        "Test OU With Desc",
			ouHandle:      generateUniqueHandle("ou-desc"),
			ouDescription: "Test OU created with description",
		},
		{
			name:          "SuccessWithoutDescription",
			ouName:        "Test OU No Desc",
			ouHandle:      generateUniqueHandle("ou-nodesc"),
			ouDescription: "",
		},
	}

	for _, tc := range testCases {
		ts.Run(tc.name, func() {
			username := common.GenerateUniqueUsername("ouuser")
			inputs := map[string]string{
				"username":      username,
				"password":      "testpassword123",
				"ouName":        tc.ouName,
				"ouHandle":      tc.ouHandle,
				"ouDescription": tc.ouDescription,
				"given_name":    "Test",
				"family_name":   "User",
				"email":         username + "@example.com",
			}

			flowStep, err := common.InitiateRegistrationFlow(ts.basicFlowTestAppID, false, inputs, "")
			ts.Require().NoError(err)
			ts.Require().Equal("COMPLETE", flowStep.FlowStatus)
			ts.Require().NotEmpty(flowStep.Assertion)

			jwtClaims, err := testutils.DecodeJWT(flowStep.Assertion)
			ts.Require().NoError(err)
			ts.Require().Equal(dynamicEntityType.Name, jwtClaims.UserType)
			ts.Require().NotEmpty(jwtClaims.OUID)

			user, err := testutils.FindUserByAttribute("username", username)
			ts.Require().NoError(err)
			ts.Require().NotNil(user)

			if user != nil {
				ts.Require().Equal(jwtClaims.OUID, user.OUID)
				ts.config.CreatedUserIDs = append(ts.config.CreatedUserIDs, user.ID)
			}

			ou, err := testutils.GetOrganizationUnit(jwtClaims.OUID)
			ts.Require().NoError(err)
			ts.Require().Equal(tc.ouName, ou.Name)
			ts.Require().Equal(tc.ouHandle, ou.Handle)
			ts.Require().NotNil(ou.Parent, "Created OU should have a parent")
			ts.Require().Equal(ts.basicFlowTestOUID, *ou.Parent,
				"Created OU should be a child of the user type's OU")

			if tc.ouDescription != "" {
				ts.Require().Equal(tc.ouDescription, ou.Description)
			}

			ts.createdOUs = append(ts.createdOUs, jwtClaims.OUID)
		})
	}
}

func (ts *OURegistrationFlowTestSuite) TestBasicRegistrationFlowWithOUCreationDuplicateError() {
	testCases := []struct {
		name                string
		existingOUName      string
		existingOUHandle    string
		newOUName           string
		newOUHandle         string
		expectedErrorSubstr string
	}{
		{
			name:                "DuplicateOUName",
			existingOUName:      "Duplicate OU Name",
			existingOUHandle:    generateUniqueHandle("duplicate-name"),
			newOUName:           "Duplicate OU Name",
			newOUHandle:         generateUniqueHandle("new-handle"),
			expectedErrorSubstr: "An organization unit with the same name already exists",
		},
		{
			name:                "DuplicateOUHandle",
			existingOUName:      "Existing OU Handle",
			existingOUHandle:    generateUniqueHandle("duplicate-handle"),
			newOUName:           "New OU Name",
			newOUHandle:         "",
			expectedErrorSubstr: "An organization unit with the same handle already exists",
		},
	}

	for _, tc := range testCases {
		ts.Run(tc.name, func() {
			existingOU := testutils.OrganizationUnit{
				Handle:      tc.existingOUHandle,
				Name:        tc.existingOUName,
				Description: "Existing OU",
				Parent:      &ts.basicFlowTestOUID,
			}

			existingOUID, err := testutils.CreateOrganizationUnit(existingOU)
			ts.Require().NoError(err)
			ts.createdOUs = append(ts.createdOUs, existingOUID)

			username := common.GenerateUniqueUsername("dupou")
			newHandle := tc.newOUHandle
			if newHandle == "" {
				newHandle = tc.existingOUHandle
			}

			inputs := map[string]string{
				"username":      username,
				"password":      "testpassword123",
				"ouName":        tc.newOUName,
				"ouHandle":      newHandle,
				"ouDescription": "Should fail due to duplicate",
				"given_name":    "Test",
				"family_name":   "User",
				"email":         username + "@example.com",
			}

			flowStep, err := common.InitiateRegistrationFlow(ts.basicFlowTestAppID, false, inputs, "")
			ts.Require().NoError(err)
			ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)
			ts.Require().Empty(flowStep.Assertion)
			ts.Require().NotNil(flowStep.Error)
			ts.Require().Contains(flowStep.Error.Description.DefaultValue, tc.expectedErrorSubstr)
		})
	}
}

func (ts *OURegistrationFlowTestSuite) TestSMSRegistrationFlowWithOUCreation() {
	testCases := []struct {
		name          string
		ouName        string
		ouHandle      string
		ouDescription string
	}{
		{
			name:          "SuccessWithDescription",
			ouName:        "Test SMS OU With Desc",
			ouHandle:      generateUniqueHandle("sms-ou-desc"),
			ouDescription: "Test SMS OU created with description",
		},
		{
			name:          "SuccessWithoutDescription",
			ouName:        "Test SMS OU No Desc",
			ouHandle:      generateUniqueHandle("sms-ou-nodesc"),
			ouDescription: "",
		},
	}

	for _, tc := range testCases {
		ts.Run(tc.name, func() {
			ts.mockServer.ClearMessages()

			mobileNumber := generateUniqueMobileNumber()

			// Step 1: Initiate flow
			flowStep, err := common.InitiateRegistrationFlow(ts.smsFlowTestAppID, false, nil, "")
			ts.Require().NoError(err)
			ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)

			// Step 2: Submit mobile number with action to trigger SMS send
			inputs := map[string]string{
				"mobile_number": mobileNumber,
			}

			flowStep, err = common.CompleteFlow(flowStep.ExecutionID, inputs, "action_001",
				flowStep.ChallengeToken)
			ts.Require().NoError(err)
			ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)

			// Wait for OTP to be sent
			time.Sleep(1 * time.Second)

			lastMessage := ts.mockServer.GetLastMessage()
			ts.Require().NotNil(lastMessage)
			ts.Require().NotEmpty(lastMessage.OTP)

			// Step 3: Submit OTP with action
			inputs = map[string]string{
				"otp": lastMessage.OTP,
			}

			flowStep, err = common.CompleteFlow(flowStep.ExecutionID, inputs, "action_002",
				flowStep.ChallengeToken)
			ts.Require().NoError(err)
			ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)

			// Step 4: Submit OU details
			inputs = map[string]string{
				"ouName":        tc.ouName,
				"ouHandle":      tc.ouHandle,
				"ouDescription": tc.ouDescription,
			}

			flowStep, err = common.CompleteFlow(flowStep.ExecutionID, inputs, "", flowStep.ChallengeToken)
			ts.Require().NoError(err)
			ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)

			// Step 5: Submit user details
			inputs = map[string]string{
				"mobile_number": mobileNumber,
				"given_name":    "Test",
				"family_name":   "User",
				"email":         mobileNumber + "@example.com",
			}

			flowStep, err = common.CompleteFlow(flowStep.ExecutionID, inputs, "", flowStep.ChallengeToken)
			ts.Require().NoError(err)
			ts.Require().Equal("COMPLETE", flowStep.FlowStatus)
			ts.Require().NotEmpty(flowStep.Assertion)

			jwtClaims, err := testutils.DecodeJWT(flowStep.Assertion)
			ts.Require().NoError(err)
			ts.Require().Equal(dynamicEntityType.Name, jwtClaims.UserType)
			ts.Require().NotEmpty(jwtClaims.OUID)

			user, err := testutils.FindUserByAttribute("mobile_number", mobileNumber)
			ts.Require().NoError(err)
			ts.Require().NotNil(user)

			if user != nil {
				ts.Require().Equal(jwtClaims.OUID, user.OUID)
				ts.config.CreatedUserIDs = append(ts.config.CreatedUserIDs, user.ID)
			}

			ou, err := testutils.GetOrganizationUnit(jwtClaims.OUID)
			ts.Require().NoError(err)
			ts.Require().Equal(tc.ouName, ou.Name)
			ts.Require().Equal(tc.ouHandle, ou.Handle)
			ts.Require().NotNil(ou.Parent, "Created OU should have a parent")
			ts.Require().Equal(ts.basicFlowTestOUID, *ou.Parent,
				"Created OU should be a child of the user type's OU")

			if tc.ouDescription != "" {
				ts.Require().Equal(tc.ouDescription, ou.Description)
			}

			ts.createdOUs = append(ts.createdOUs, jwtClaims.OUID)
		})
	}
}

func (ts *OURegistrationFlowTestSuite) TestSMSRegistrationFlowWithOUCreationDuplicateError() {
	testCases := []struct {
		name                string
		existingOUName      string
		existingOUHandle    string
		newOUName           string
		newOUHandle         string
		expectedErrorSubstr string
	}{
		{
			name:                "DuplicateOUName",
			existingOUName:      "SMS Duplicate OU Name",
			existingOUHandle:    generateUniqueHandle("sms-duplicate-name"),
			newOUName:           "SMS Duplicate OU Name",
			newOUHandle:         generateUniqueHandle("new-sms-handle"),
			expectedErrorSubstr: "An organization unit with the same name already exists",
		},
		{
			name:                "DuplicateOUHandle",
			existingOUName:      "SMS Existing OU Handle",
			existingOUHandle:    generateUniqueHandle("sms-duplicate-handle"),
			newOUName:           "SMS New OU Name",
			newOUHandle:         "",
			expectedErrorSubstr: "An organization unit with the same handle already exists",
		},
	}

	for _, tc := range testCases {
		ts.Run(tc.name, func() {
			existingOU := testutils.OrganizationUnit{
				Handle:      tc.existingOUHandle,
				Name:        tc.existingOUName,
				Description: "Existing OU",
				Parent:      &ts.basicFlowTestOUID,
			}

			existingOUID, err := testutils.CreateOrganizationUnit(existingOU)
			ts.Require().NoError(err)
			ts.createdOUs = append(ts.createdOUs, existingOUID)

			ts.mockServer.ClearMessages()

			mobileNumber := generateUniqueMobileNumber()

			// Step 1: Initiate flow
			flowStep, err := common.InitiateRegistrationFlow(ts.smsFlowTestAppID, false, nil, "")
			ts.Require().NoError(err)

			// Step 2: Submit mobile number with action to trigger SMS send
			inputs := map[string]string{
				"mobile_number": mobileNumber,
			}
			// Wait for OTP to be sent
			time.Sleep(1 * time.Second)

			flowStep, err =
				common.CompleteFlow(flowStep.ExecutionID, inputs, "action_001", flowStep.ChallengeToken)
			ts.Require().NoError(err)
			ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)

			lastMessage := ts.mockServer.GetLastMessage()
			ts.Require().NotNil(lastMessage)

			// Step 3: Submit OTP with action
			inputs = map[string]string{
				"otp": lastMessage.OTP,
			}

			flowStep, err = common.CompleteFlow(flowStep.ExecutionID, inputs, "action_002",
				flowStep.ChallengeToken)
			ts.Require().NoError(err)
			ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)

			// Step 4: Submit OU details (should fail with duplicate error)
			newHandle := tc.newOUHandle
			if newHandle == "" {
				newHandle = tc.existingOUHandle
			}

			inputs = map[string]string{
				"ouName":        tc.newOUName,
				"ouHandle":      newHandle,
				"ouDescription": "Should fail due to duplicate",
			}

			flowStep, err = common.CompleteFlow(flowStep.ExecutionID, inputs, "", flowStep.ChallengeToken)
			ts.Require().NoError(err)
			ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)
			ts.Require().Empty(flowStep.Assertion)
			ts.Require().NotNil(flowStep.Error)
			ts.Require().Contains(flowStep.Error.Description.DefaultValue, tc.expectedErrorSubstr)
		})
	}
}

func generateUniqueHandle(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano()%1000000)
}

// Helper function to generate unique mobile numbers
func generateUniqueMobileNumber() string {
	return fmt.Sprintf("+1%d", time.Now().UnixNano())
}
