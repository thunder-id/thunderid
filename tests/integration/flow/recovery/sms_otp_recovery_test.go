/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package recovery

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	smsRecoveryMockNotificationPort = 8099
)

var (
	smsRecoveryOU = testutils.OrganizationUnit{
		Handle:      "sms-recovery-test-ou",
		Name:        "SMS OTP Recovery Flow Test OU",
		Description: "Organization unit for SMS OTP recovery flow testing",
	}

	smsRecoveryUserSchema = testutils.UserType{
		Name: "sms-recovery-user-type",
		Schema: map[string]interface{}{
			"username": map[string]interface{}{
				"type": "string",
			},
			"password": map[string]interface{}{
				"type":       "string",
				"credential": true,
			},
			"mobile_number": map[string]interface{}{
				"type": "string",
			},
		},
		AllowSelfRegistration: false,
	}
)

// SMSOTPRecoveryFlowTestSuite tests the SMS OTP password recovery flow.
type SMSOTPRecoveryFlowTestSuite struct {
	suite.Suite
	config        *common.TestSuiteConfig
	mockSMSServer *testutils.MockNotificationServer
	userSchemaID  string
	testOUID      string
	testAppID     string
	smsFlowID     string
	authFlowID    string
	testUserID    string
	testUsername  string
	testMobile    string
	testPassword  string
	smsSenderID   string
}

func TestSMSOTPRecoveryFlowTestSuite(t *testing.T) {
	suite.Run(t, new(SMSOTPRecoveryFlowTestSuite))
}

func (ts *SMSOTPRecoveryFlowTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}
	ts.testPassword = "OldSMSPassword123!"
	ts.testUsername = common.GenerateUniqueUsername("smsrecoveryuser")
	ts.testMobile = fmt.Sprintf("+1555%07d", time.Now().UnixNano()%10000000)

	// Create OU
	ouID, err := testutils.CreateOrganizationUnit(smsRecoveryOU)
	ts.Require().NoError(err, "Failed to create test OU")
	ts.testOUID = ouID

	// Create user schema
	smsRecoveryUserSchema.OUID = ts.testOUID
	schemaID, err := testutils.CreateUserType(smsRecoveryUserSchema)
	ts.Require().NoError(err, "Failed to create SMS recovery user schema")
	ts.userSchemaID = schemaID

	// Create a test user with known credentials and a mobile number
	userIDs, err := testutils.CreateMultipleUsers(testutils.User{
		OUID: ts.testOUID,
		Type: smsRecoveryUserSchema.Name,
		Attributes: json.RawMessage(`{
			"username":     "` + ts.testUsername + `",
			"password":     "` + ts.testPassword + `",
			"mobile_number": "` + ts.testMobile + `"
		}`),
	})
	ts.Require().NoError(err, "Failed to create test user")
	ts.testUserID = userIDs[0]

	// Start mock SMS notification server
	ts.mockSMSServer = testutils.NewMockNotificationServer(smsRecoveryMockNotificationPort)
	ts.Require().NoError(ts.mockSMSServer.Start(), "Failed to start mock SMS notification server")
	time.Sleep(100 * time.Millisecond)

	// Create notification sender pointing at the mock server
	senderID, err := testutils.CreateNotificationSender(testutils.NotificationSender{
		Name:        "SMS Recovery Test Sender",
		Description: "Notification sender for SMS OTP recovery flow testing",
		Provider:    "custom",
		Properties: []testutils.SenderProperty{
			{Name: "url", Value: ts.mockSMSServer.GetSendSMSURL(), IsSecret: false},
			{Name: "http_method", Value: "POST", IsSecret: false},
			{Name: "content_type", Value: "JSON", IsSecret: false},
		},
	})
	ts.Require().NoError(err, "Failed to create SMS notification sender")
	ts.smsSenderID = senderID
	ts.config.CreatedSenderIDs = append(ts.config.CreatedSenderIDs, senderID)

	// Patch the sms_send node in the flow to use the created sender ID.
	customFlow := buildSMSOTPRecoveryFlow(senderID)
	customFlowID, err := testutils.CreateFlow(customFlow)
	ts.Require().NoError(err, "Failed to create custom SMS OTP recovery flow")
	ts.smsFlowID = customFlowID
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, customFlowID)

	authFlowID, err := testutils.CreateIsolatedAuthFlow("sms-otp-recovery-isolated-auth")
	ts.Require().NoError(err, "Failed to create isolated auth flow")
	ts.authFlowID = authFlowID
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, authFlowID)

	// Create test application with SMS OTP recovery flow
	appID, err := testutils.CreateApplication(testutils.Application{
		OUID:                      ts.testOUID,
		Name:                      "SMS OTP Recovery Flow Test App",
		Description:               "Application for testing SMS OTP recovery",
		IsRegistrationFlowEnabled: false,
		IsRecoveryFlowEnabled:     true,
		ClientID:                  "sms_recovery_test_client",
		ClientSecret:              "sms_recovery_test_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{smsRecoveryUserSchema.Name},
		AuthFlowID:                ts.authFlowID,
		RecoveryFlowID:            ts.smsFlowID,
	})
	ts.Require().NoError(err, "Failed to create test application")
	ts.testAppID = appID
}

func (ts *SMSOTPRecoveryFlowTestSuite) TearDownSuite() {
	if ts.testAppID != "" {
		if err := testutils.DeleteApplication(ts.testAppID); err != nil {
			ts.T().Logf("teardown: failed to delete test app: %v", err)
		}
	}
	if ts.testUserID != "" {
		if err := testutils.CleanupUsers([]string{ts.testUserID}); err != nil {
			ts.T().Logf("teardown: failed to delete test user: %v", err)
		}
	}
	for _, flowID := range ts.config.CreatedFlowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("teardown: failed to delete flow %s: %v", flowID, err)
		}
	}
	for _, senderID := range ts.config.CreatedSenderIDs {
		if err := testutils.DeleteNotificationSender(senderID); err != nil {
			ts.T().Logf("teardown: failed to delete notification sender %s: %v", senderID, err)
		}
	}
	if ts.mockSMSServer != nil {
		if err := ts.mockSMSServer.Stop(); err != nil {
			ts.T().Logf("teardown: failed to stop mock SMS server: %v", err)
		}
	}
	if ts.userSchemaID != "" {
		if err := testutils.DeleteUserType(ts.userSchemaID); err != nil {
			ts.T().Logf("teardown: failed to delete user schema: %v", err)
		}
	}
	if ts.testOUID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.testOUID); err != nil {
			ts.T().Logf("teardown: failed to delete OU: %v", err)
		}
	}
}

// TestSMSOTPRecoveryFlow_Success tests the full happy-path SMS OTP recovery flow.
// The flow: prompt_username → identify_user → generate_otp → sms_send → otp_sent_status →
//
//	verify_otp → prompt_new_password → set_credential → complete
func (ts *SMSOTPRecoveryFlowTestSuite) TestSMSOTPRecoveryFlow_Success() {
	ts.mockSMSServer.ClearMessages()
	newPassword := "NewSMSPassword456!"

	// Step 1: Initiate recovery flow — stops at prompt_username.
	flowStep, err := common.InitiateRecoveryFlow(ts.testAppID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate SMS OTP recovery flow")
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)
	ts.Require().Equal("VIEW", flowStep.Type)
	ts.Require().True(common.HasInput(flowStep.Data.Inputs, "username"),
		"Expected username input at prompt_username")

	// Step 2: Submit username — engine runs identify_user, generate_otp, sms_send, then stops at otp_sent_status.
	flowStep, err = common.CompleteFlow(
		flowStep.ExecutionID,
		map[string]string{"username": ts.testUsername},
		"action_submit_username",
		flowStep.ChallengeToken,
	)
	ts.Require().NoError(err, "Failed to submit username")
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus,
		"Expected INCOMPLETE after username submission (at otp_sent_status)")
	ts.Require().True(common.HasInput(flowStep.Data.Inputs, "otp"),
		"Expected OTP input at otp_sent_status")

	// Wait for mock SMS server to receive the OTP message.
	time.Sleep(1 * time.Second)

	smsMessage := ts.mockSMSServer.GetLastMessage()
	ts.Require().NotNil(smsMessage, "Expected OTP SMS to be captured by mock notification server")
	ts.Require().NotEmpty(smsMessage.OTP, "Expected OTP to be extracted from SMS message")

	// Step 3: Submit OTP — engine runs verify_otp, then stops at prompt_new_password.
	flowStep, err = common.CompleteFlow(
		flowStep.ExecutionID,
		map[string]string{"otp": smsMessage.OTP},
		"action_submit_otp",
		flowStep.ChallengeToken,
	)
	ts.Require().NoError(err, "Failed to submit OTP")
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus,
		"Expected INCOMPLETE after OTP verification (at prompt_new_password)")
	ts.Require().True(common.HasInput(flowStep.Data.Inputs, "password"),
		"Expected password input at prompt_new_password")

	// Step 4: Submit new password — engine runs set_credential, recovery_complete → COMPLETE.
	flowStep, err = common.CompleteFlow(
		flowStep.ExecutionID,
		map[string]string{"password": newPassword},
		"action_submit_password",
		flowStep.ChallengeToken,
	)
	ts.Require().NoError(err, "Failed to submit new password")
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus, "Expected COMPLETE after setting new password")

	// Step 5: Verify old password no longer works.
	ok, err := testutils.AuthenticateWithCredential("username", ts.testUsername, "password", ts.testPassword)
	ts.Require().NoError(err)
	ts.Require().False(ok, "Old password should be rejected after SMS OTP recovery")

	// Step 6: Verify new password works.
	ok, err = testutils.AuthenticateWithCredential("username", ts.testUsername, "password", newPassword)
	ts.Require().NoError(err)
	ts.Require().True(ok, "New password should authenticate successfully after SMS OTP recovery")

	// Restore for subsequent tests.
	ts.testPassword = newPassword
}

// TestSMSOTPRecoveryFlow_UnknownUsername tests anti-enumeration: an unknown username
// must still reach otp_sent_status without revealing whether the user exists.
func (ts *SMSOTPRecoveryFlowTestSuite) TestSMSOTPRecoveryFlow_UnknownUsername() {
	ts.mockSMSServer.ClearMessages()

	// Initiate
	flowStep, err := common.InitiateRecoveryFlow(ts.testAppID, false, nil, "")
	ts.Require().NoError(err)
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)

	// Submit non-existent username
	flowStep, err = common.CompleteFlow(
		flowStep.ExecutionID,
		map[string]string{"username": "ghost_user_that_does_not_exist"},
		"action_submit_username",
		flowStep.ChallengeToken,
	)
	ts.Require().NoError(err)

	// Must still reach otp_sent_status (anti-enumeration).
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus,
		"Unknown username must show otp_sent_status (anti-enumeration)")

	// No SMS should be sent.
	time.Sleep(500 * time.Millisecond)
	ts.Require().Nil(ts.mockSMSServer.GetLastMessage(),
		"No SMS should be sent for a non-existent username")
}

// TestSMSOTPRecoveryFlow_InvalidOTP tests that a wrong OTP causes verify_otp to return
// incomplete and the flow redirects to otp_sent_status (as configured in onIncomplete).
func (ts *SMSOTPRecoveryFlowTestSuite) TestSMSOTPRecoveryFlow_InvalidOTP() {
	ts.mockSMSServer.ClearMessages()

	// Initiate + submit valid username to reach otp_sent_status.
	flowStep, err := common.InitiateRecoveryFlow(ts.testAppID, false, nil, "")
	ts.Require().NoError(err)

	flowStep, err = common.CompleteFlow(
		flowStep.ExecutionID,
		map[string]string{"username": ts.testUsername},
		"action_submit_username",
		flowStep.ChallengeToken,
	)
	ts.Require().NoError(err)
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)

	// Wait for real OTP to be sent (we won't use it).
	time.Sleep(1 * time.Second)
	ts.mockSMSServer.ClearMessages()

	// Submit an incorrect OTP — verify_otp onIncomplete → otp_sent_status.
	flowStep, err = common.CompleteFlow(
		flowStep.ExecutionID,
		map[string]string{"otp": "000000"},
		"action_submit_otp",
		flowStep.ChallengeToken,
	)
	ts.Require().NoError(err)

	// The flow must be INCOMPLETE (restarted to otp_sent_status), not COMPLETE.
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus,
		"Invalid OTP must not complete the recovery flow")
	ts.Require().NotEqual("COMPLETE", flowStep.FlowStatus)

	// The error must be present.
	ts.Require().NotNil(flowStep.Error,
		"Expected an error for invalid OTP")

	// The flow should loop back to OTP prompt.
	ts.Require().True(common.HasInput(flowStep.Data.Inputs, "otp"),
		"After invalid OTP, flow must return to otp_sent_status")
}

// TestSMSOTPRecoveryFlow_MissingMobileNumber tests that a user without a mobileNumber
// attribute causes the sms_send step to fail gracefully.
func (ts *SMSOTPRecoveryFlowTestSuite) TestSMSOTPRecoveryFlow_MissingMobileNumber() {
	// Create a user without mobile_number.
	usernameNoMobile := common.GenerateUniqueUsername("nomobile")
	userIDs, err := testutils.CreateMultipleUsers(testutils.User{
		OUID: ts.testOUID,
		Type: smsRecoveryUserSchema.Name,
		Attributes: json.RawMessage(`{
			"username": "` + usernameNoMobile + `",
			"password": "TestPassword123!"
		}`),
	})
	ts.Require().NoError(err, "Failed to create user without mobile")
	defer func() {
		if err := testutils.CleanupUsers(userIDs); err != nil {
			ts.T().Logf("Failed to delete user without mobile: %v", err)
		}
	}()

	ts.mockSMSServer.ClearMessages()

	// Initiate + submit username of the user with no mobile.
	flowStep, err := common.InitiateRecoveryFlow(ts.testAppID, false, nil, "")
	ts.Require().NoError(err)

	flowStep, err = common.CompleteFlow(
		flowStep.ExecutionID,
		map[string]string{"username": usernameNoMobile},
		"action_submit_username",
		flowStep.ChallengeToken,
	)
	ts.Require().NoError(err)

	// The flow should still reach otp_sent_status (sms_send onFailure → otp_sent_status).
	// Users without a mobile number cause sms_send to fail silently and advance to the status node.
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus,
		"Even when mobile is missing, flow should advance to otp_sent_status via onFailure")
}

// buildSMSOTPRecoveryFlow returns a custom SMS OTP recovery flow definition with
// the given senderID wired into the sms_send node.
func buildSMSOTPRecoveryFlow(senderID string) testutils.Flow {
	return testutils.Flow{
		Name:     "SMS OTP Recovery Test Flow",
		Handle:   "test-sms-otp-recovery-" + senderID[:8],
		FlowType: "RECOVERY",
		Nodes: []map[string]interface{}{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "prompt_username",
			},
			{
				"id":   "prompt_username",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{
								"ref":        "input_username",
								"identifier": "username",
								"type":       "TEXT_INPUT",
								"required":   true,
							},
						},
						"action": map[string]interface{}{
							"ref":      "action_submit_username",
							"nextNode": "identify_user",
						},
					},
				},
			},
			{
				"id":   "identify_user",
				"type": "TASK_EXECUTION",
				"inputs": []map[string]interface{}{
					{
						"ref":        "input_username",
						"identifier": "username",
						"type":       "TEXT_INPUT",
						"required":   true,
					},
				},
				"executor": map[string]interface{}{
					"name": "IdentifyingExecutor",
					"mode": "identify",
				},
				"onSuccess": "generate_otp",
				"onFailure": "otp_sent_status",
			},
			{
				"id":   "generate_otp",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "OTPExecutor",
					"mode": "generate",
					"inputs": []map[string]interface{}{
						{
							"ref":        "input_username",
							"identifier": "username",
							"type":       "TEXT_INPUT",
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
					"senderId":    senderID,
					"smsTemplate": "OTP",
				},
				"executor": map[string]interface{}{
					"name": "SMSExecutor",
				},
				"onSuccess": "otp_sent_status",
				"onFailure": "otp_sent_status",
			},
			{
				"id":   "otp_sent_status",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{
								"ref":        "input_otp",
								"identifier": "otp",
								"type":       "OTP_INPUT",
								"required":   true,
							},
						},
						"action": map[string]interface{}{
							"ref":      "action_submit_otp",
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
				"onSuccess":    "prompt_new_password",
				"onFailure":    "prompt_username",
				"onIncomplete": "otp_sent_status",
			},
			{
				"id":   "prompt_new_password",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{
								"ref":        "input_new_password",
								"identifier": "password",
								"type":       "PASSWORD_INPUT",
								"required":   true,
							},
						},
						"action": map[string]interface{}{
							"ref":      "action_submit_password",
							"nextNode": "set_credential",
						},
					},
				},
			},
			{
				"id":   "set_credential",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "CredentialSetter",
				},
				"onSuccess": "recovery_complete",
			},
			{
				"id":   "recovery_complete",
				"type": "PROMPT",
				"next": "end",
			},
			{
				"id":   "end",
				"type": "END",
			},
		},
	}
}
