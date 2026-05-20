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
	assuranceMockNotificationServerPort = 8099
)

// Authenticator names used in assurance
const (
	AuthenticatorCredentials = "CredentialsAuthenticator"
	AuthenticatorSMSOTP      = "SMSOTPAuthenticator"
)

var (
	// Flow for SMS OTP only authentication (AAL1)
	assuranceSMSOnlyFlow = testutils.Flow{
		Name:     "SMS Only Auth Flow for Assurance Test",
		FlowType: "AUTHENTICATION",
		Handle:   "assurance_test_sms_only_flow",
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

	// Flow for Credentials + SMS OTP authentication (AAL2)
	assuranceMFAFlow = testutils.Flow{
		Name:     "MFA Auth Flow for Assurance Test",
		FlowType: "AUTHENTICATION",
		Handle:   "assurance_test_mfa_flow",
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
								"type":       "string",
								"required":   true,
							},
							{
								"ref":        "input_002",
								"identifier": "password",
								"type":       "string",
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
				},
				"onSuccess": "sms_otp_send",
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
								"ref":        "input_003",
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

	// Flow for Basic Auth only (AAL1)
	assuranceBasicAuthFlow = testutils.Flow{
		Name:     "Basic Auth Only Flow for Assurance Test",
		FlowType: "AUTHENTICATION",
		Handle:   "assurance_test_basic_auth_flow",
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
								"type":       "string",
								"required":   true,
							},
							{
								"ref":        "input_002",
								"identifier": "password",
								"type":       "string",
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
	assuranceTestApp = testutils.Application{
		Name:                      "Assurance Test Application",
		Description:               "Application for testing authentication assurance levels",
		IsRegistrationFlowEnabled: false,
		ClientID:                  "assurance_test_client",
		ClientSecret:              "assurance_test_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{"assurance_test_user"},
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{"userType", "ouId", "ouName", "ouHandle"},
		},
	}

	assuranceEntityType = testutils.UserType{
		Name: "assurance_test_user",
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
			"mobileNumber": map[string]interface{}{
				"type": "string",
			},
		},
	}

	assuranceTestUser = testutils.User{
		Type: assuranceEntityType.Name,
		Attributes: json.RawMessage(`{
			"username": "assurance_user",
			"password": "testpassword123",
			"email": "assurance@example.com",
			"mobileNumber": "+1987654321"
		}`),
	}
)

var (
	assuranceTestAppID       string
	assuranceEntityTypeID    string
	assuranceTestSenderID    string
	assuranceSMSOnlyFlowID   string
	assuranceMFAFlowID       string
	assuranceBasicAuthFlowID string
	assuranceTestOU          = testutils.OrganizationUnit{
		Handle:      "assurance-test-ou",
		Name:        "Assurance Test OU",
		Description: "Organization unit for assurance testing",
	}
)

type AssuranceTestSuite struct {
	suite.Suite
	config     *common.TestSuiteConfig
	mockServer *testutils.MockNotificationServer
}

func TestAssuranceTestSuite(t *testing.T) {
	suite.Run(t, new(AssuranceTestSuite))
}

func (ts *AssuranceTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}

	// Create test organization unit
	ouID, err := testutils.CreateOrganizationUnit(assuranceTestOU)
	if err != nil {
		ts.T().Fatalf("Failed to create test organization unit: %v", err)
	}
	assuranceTestOU.ID = ouID

	// Create test user type
	assuranceEntityType.OUID = ouID
	schemaID, err := testutils.CreateUserType(assuranceEntityType)
	if err != nil {
		ts.T().Fatalf("Failed to create test user type: %v", err)
	}
	assuranceEntityTypeID = schemaID

	// Start mock notification server
	ts.mockServer = testutils.NewMockNotificationServer(assuranceMockNotificationServerPort)
	err = ts.mockServer.Start()
	if err != nil {
		ts.T().Fatalf("Failed to start mock notification server: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	// Create test user
	testUser := assuranceTestUser
	testUser.OUID = assuranceTestOU.ID
	userIDs, err := testutils.CreateMultipleUsers(testUser)
	if err != nil {
		ts.T().Fatalf("Failed to create test user: %v", err)
	}
	ts.config.CreatedUserIDs = userIDs

	// Create notification sender
	customSender := testutils.NotificationSender{
		Name:        "Assurance Test Sender",
		Description: "Sender for assurance testing",
		Provider:    "custom",
		Properties: []testutils.SenderProperty{
			{Name: "url", Value: ts.mockServer.GetSendSMSURL(), IsSecret: false},
			{Name: "http_method", Value: "POST", IsSecret: false},
			{Name: "content_type", Value: "JSON", IsSecret: false},
		},
	}
	senderID, err := testutils.CreateNotificationSender(customSender)
	ts.Require().NoError(err, "Failed to create notification sender")
	assuranceTestSenderID = senderID
	ts.config.CreatedSenderIDs = append(ts.config.CreatedSenderIDs, senderID)

	// Update flow definitions with sender ID
	smsOnlyNodes := assuranceSMSOnlyFlow.Nodes.([]map[string]interface{})
	smsOnlyNodes[2]["properties"].(map[string]interface{})["senderId"] = senderID
	smsOnlyNodes[4]["properties"].(map[string]interface{})["senderId"] = senderID
	assuranceSMSOnlyFlow.Nodes = smsOnlyNodes

	mfaNodes := assuranceMFAFlow.Nodes.([]map[string]interface{})
	mfaNodes[3]["properties"].(map[string]interface{})["senderId"] = senderID
	mfaNodes[5]["properties"].(map[string]interface{})["senderId"] = senderID
	assuranceMFAFlow.Nodes = mfaNodes

	// Create flows
	smsOnlyFlowID, err := testutils.CreateFlow(assuranceSMSOnlyFlow)
	ts.Require().NoError(err, "Failed to create SMS only flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, smsOnlyFlowID)
	assuranceSMSOnlyFlowID = smsOnlyFlowID

	mfaFlowID, err := testutils.CreateFlow(assuranceMFAFlow)
	ts.Require().NoError(err, "Failed to create MFA flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, mfaFlowID)
	assuranceMFAFlowID = mfaFlowID

	basicAuthFlowID, err := testutils.CreateFlow(assuranceBasicAuthFlow)
	ts.Require().NoError(err, "Failed to create basic auth flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, basicAuthFlowID)
	assuranceBasicAuthFlowID = basicAuthFlowID

	// Create test application
	assuranceTestApp.AuthFlowID = smsOnlyFlowID
	assuranceTestApp.OUID = assuranceTestOU.ID
	appID, err := testutils.CreateApplication(assuranceTestApp)
	if err != nil {
		ts.T().Fatalf("Failed to create test application: %v", err)
	}
	assuranceTestAppID = appID
}

func (ts *AssuranceTestSuite) TearDownSuite() {
	if err := testutils.CleanupUsers(ts.config.CreatedUserIDs); err != nil {
		ts.T().Logf("Failed to cleanup users: %v", err)
	}

	if ts.mockServer != nil {
		_ = ts.mockServer.Stop()
	}

	if assuranceTestAppID != "" {
		_ = testutils.DeleteApplication(assuranceTestAppID)
	}

	for _, flowID := range ts.config.CreatedFlowIDs {
		_ = testutils.DeleteFlow(flowID)
	}

	for _, senderID := range ts.config.CreatedSenderIDs {
		_ = testutils.DeleteNotificationSender(senderID)
	}

	if assuranceTestOU.ID != "" {
		_ = testutils.DeleteOrganizationUnit(assuranceTestOU.ID)
	}

	if assuranceEntityTypeID != "" {
		_ = testutils.DeleteUserType(assuranceEntityTypeID)
	}
}

func (ts *AssuranceTestSuite) TestAssurance_SMSOTPOnly() {
	// Ensure app is configured to use SMS only flow
	err := common.UpdateAppConfig(assuranceTestAppID, assuranceSMSOnlyFlowID, "")
	ts.Require().NoError(err)

	// Step 1: Initiate flow
	flowStep, err := common.InitiateAuthenticationFlow(assuranceTestAppID, false, nil, "")
	ts.Require().NoError(err)
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)

	// Clear messages
	ts.mockServer.ClearMessages()

	// Step 2: Submit mobile number
	userAttrs, err := testutils.GetUserAttributes(assuranceTestUser)
	ts.Require().NoError(err)

	inputs := map[string]string{
		"mobileNumber": userAttrs["mobileNumber"].(string),
	}

	otpFlowStep, err := common.CompleteFlow(flowStep.ExecutionID, inputs, "action_001",
		flowStep.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("INCOMPLETE", otpFlowStep.FlowStatus)

	// Wait for SMS
	time.Sleep(500 * time.Millisecond)

	// Get OTP
	lastMessage := ts.mockServer.GetLastMessage()
	ts.Require().NotNil(lastMessage)
	ts.Require().NotEmpty(lastMessage.OTP)

	// Step 3: Submit OTP
	otpInputs := map[string]string{"otp": lastMessage.OTP}
	completeFlowStep, err := common.CompleteFlow(flowStep.ExecutionID, otpInputs, "action_002",
		otpFlowStep.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("COMPLETE", completeFlowStep.FlowStatus)
	ts.Require().NotEmpty(completeFlowStep.Assertion)

	// Validate JWT claims
	jwtClaims, err := testutils.DecodeJWT(completeFlowStep.Assertion)
	ts.Require().NoError(err)

	// Validate assurance block
	err = testutils.ValidateAssurance(jwtClaims, testutils.AssuranceExpectation{
		AAL:                    "AAL1",
		IAL:                    "IAL1",
		ExpectedAuthenticators: []string{AuthenticatorSMSOTP},
	})
	ts.Require().NoError(err, "Assurance validation failed")
}

func (ts *AssuranceTestSuite) TestAssurance_CredentialsPlusSMSOTP() {
	// Update app to use MFA flow
	err := common.UpdateAppConfig(assuranceTestAppID, assuranceMFAFlowID, "")
	ts.Require().NoError(err)

	// Step 1: Initiate flow
	flowStep, err := common.InitiateAuthenticationFlow(assuranceTestAppID, false, nil, "")
	ts.Require().NoError(err)
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)

	// Clear messages
	ts.mockServer.ClearMessages()

	// Step 2: Submit credentials
	userAttrs, err := testutils.GetUserAttributes(assuranceTestUser)
	ts.Require().NoError(err)

	credInputs := map[string]string{
		"username": userAttrs["username"].(string),
		"password": userAttrs["password"].(string),
	}

	otpFlowStep, err := common.CompleteFlow(flowStep.ExecutionID, credInputs, "action_001",
		flowStep.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("INCOMPLETE", otpFlowStep.FlowStatus)

	// Wait for SMS
	time.Sleep(500 * time.Millisecond)

	// Get OTP
	lastMessage := ts.mockServer.GetLastMessage()
	ts.Require().NotNil(lastMessage)
	ts.Require().NotEmpty(lastMessage.OTP)

	// Step 3: Submit OTP
	otpInputs := map[string]string{"otp": lastMessage.OTP}
	completeFlowStep, err := common.CompleteFlow(flowStep.ExecutionID, otpInputs, "action_002",
		otpFlowStep.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("COMPLETE", completeFlowStep.FlowStatus)
	ts.Require().NotEmpty(completeFlowStep.Assertion)

	// Validate JWT claims
	jwtClaims, err := testutils.DecodeJWT(completeFlowStep.Assertion)
	ts.Require().NoError(err)

	// Validate assurance block
	err = testutils.ValidateAssurance(jwtClaims, testutils.AssuranceExpectation{
		AAL:                    "AAL2",
		IAL:                    "IAL1",
		ExpectedAuthenticators: []string{AuthenticatorCredentials, AuthenticatorSMSOTP},
	})
	ts.Require().NoError(err, "Assurance validation failed")
}

func (ts *AssuranceTestSuite) TestAssurance_BasicAuthOnly() {
	// Update app to use basic auth flow
	err := common.UpdateAppConfig(assuranceTestAppID, assuranceBasicAuthFlowID, "")
	ts.Require().NoError(err)

	// Step 1: Initiate flow
	flowStep, err := common.InitiateAuthenticationFlow(assuranceTestAppID, false, nil, "")
	ts.Require().NoError(err)
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)

	// Step 2: Submit credentials
	userAttrs, err := testutils.GetUserAttributes(assuranceTestUser)
	ts.Require().NoError(err)

	credInputs := map[string]string{
		"username": userAttrs["username"].(string),
		"password": userAttrs["password"].(string),
	}

	completeFlowStep, err := common.CompleteFlow(flowStep.ExecutionID, credInputs, "action_001",
		flowStep.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("COMPLETE", completeFlowStep.FlowStatus)
	ts.Require().NotEmpty(completeFlowStep.Assertion)

	// Validate JWT claims
	jwtClaims, err := testutils.DecodeJWT(completeFlowStep.Assertion)
	ts.Require().NoError(err)

	// Validate assurance block
	err = testutils.ValidateAssurance(jwtClaims, testutils.AssuranceExpectation{
		AAL:                    "AAL1",
		IAL:                    "IAL1",
		ExpectedAuthenticators: []string{AuthenticatorCredentials},
	})
	ts.Require().NoError(err, "Assurance validation failed")
}
