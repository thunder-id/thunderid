// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authentication

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

var (
	mfaRecipientTestOU = testutils.OrganizationUnit{
		Handle:      "mfa-recipient-test-ou",
		Name:        "MFA Recipient Test OU",
		Description: "Organization unit for MFA second factor recipient tests",
	}

	mfaRecipientUserType = testutils.UserType{
		Name: "mfa_recipient_user",
		Schema: map[string]interface{}{
			"username":      map[string]interface{}{"type": "string"},
			"password":      map[string]interface{}{"type": "string", "credential": true},
			"email":         map[string]interface{}{"type": "string"},
			"mobile_number": map[string]interface{}{"type": "string"},
		},
	}
)

// MFARecipientTestSuite covers the password plus SMS OTP flow.
type MFARecipientTestSuite struct {
	suite.Suite
	config        *common.TestSuiteConfig
	mockSMSServer *testutils.MockNotificationServer
	userTypeID    string
	smsSenderID   string
	smsAppID      string
	enrollAppID   string
}

func TestMFARecipientTestSuite(t *testing.T) {
	suite.Run(t, new(MFARecipientTestSuite))
}

func (ts *MFARecipientTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}

	ouID, err := testutils.CreateOrganizationUnit(mfaRecipientTestOU)
	ts.Require().NoError(err, "Failed to create test organization unit")
	mfaRecipientTestOU.ID = ouID

	mfaRecipientUserType.OUID = ouID
	userTypeID, err := testutils.CreateUserType(mfaRecipientUserType)
	ts.Require().NoError(err, "Failed to create test user type")
	ts.userTypeID = userTypeID

	ts.mockSMSServer = testutils.NewMockNotificationServer(mockNotificationServerPort)
	ts.Require().NoError(ts.mockSMSServer.Start(), "Failed to start mock notification server")
	smsSenderID, err := testutils.CreateNotificationSender(testutils.NotificationSender{
		Name:        "MFA Recipient Test SMS Sender",
		Description: "Sender for MFA recipient tests",
		Provider:    "custom",
		Properties: []testutils.SenderProperty{
			{Name: "url", Value: ts.mockSMSServer.GetSendSMSURL()},
			{Name: "http_method", Value: "POST"},
			{Name: "content_type", Value: "JSON"},
		},
	})
	ts.Require().NoError(err, "Failed to create the SMS sender")
	ts.smsSenderID = smsSenderID

	ts.smsAppID = ts.createApp("mfa_recipient_sms_client", buildPasswordSMSOTPFlow(smsSenderID))
	ts.enrollAppID = ts.createApp("mfa_recipient_sms_enroll_client", buildPasswordSMSOTPEnrollFlow(smsSenderID))
}

func (ts *MFARecipientTestSuite) TearDownSuite() {
	for _, appID := range []string{ts.smsAppID, ts.enrollAppID} {
		if appID == "" {
			continue
		}
		if err := testutils.DeleteApplication(appID); err != nil {
			ts.T().Logf("Failed to delete application during teardown: %v", err)
		}
	}
	for _, flowID := range ts.config.CreatedFlowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete flow %s during teardown: %v", flowID, err)
		}
	}
	if ts.smsSenderID != "" {
		if err := testutils.DeleteNotificationSender(ts.smsSenderID); err != nil {
			ts.T().Logf("Failed to delete sender during teardown: %v", err)
		}
	}
	if ts.userTypeID != "" {
		if err := testutils.DeleteUserType(ts.userTypeID); err != nil {
			ts.T().Logf("Failed to delete user type during teardown: %v", err)
		}
	}
	if mfaRecipientTestOU.ID != "" {
		if err := testutils.DeleteOrganizationUnit(mfaRecipientTestOU.ID); err != nil {
			ts.T().Logf("Failed to delete organization unit during teardown: %v", err)
		}
	}
	if ts.mockSMSServer != nil {
		if err := ts.mockSMSServer.Stop(); err != nil {
			ts.T().Logf("Failed to stop mock notification server during teardown: %v", err)
		}
	}
}

func (ts *MFARecipientTestSuite) TestPasswordSMSOTP_StoredMobileIsKept() {
	username := ts.createUser(`"mobile_number": "+1222333444"`)
	ts.mockSMSServer.ClearMessages()

	step := ts.submitPassword(ts.smsAppID, username, map[string]string{"mobile_number": "+1999888777"})

	ts.Require().Equal("INCOMPLETE", step.FlowStatus)
	ts.Require().True(common.HasInput(step.Data.Inputs, "otp"), "The OTP should be requested")
	ts.Equal("+1222333444", ts.storedAttribute(username, "mobile_number"),
		"The stored mobile number is kept")
}

func (ts *MFARecipientTestSuite) TestPasswordSMSOTP_StoredMobileIsKeptWhenSentOnStart() {
	username := ts.createUser(`"mobile_number": "+1222333444"`)
	ts.mockSMSServer.ClearMessages()

	step, err := common.InitiateAuthenticationFlow(ts.smsAppID, false,
		map[string]string{"username": username, "password": "testpassword", "mobile_number": "+1999888777"}, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")

	ts.Require().Equal("INCOMPLETE", step.FlowStatus)
	ts.Require().True(common.HasInput(step.Data.Inputs, "otp"), "The OTP should be requested")
	ts.Equal("+1222333444", ts.storedAttribute(username, "mobile_number"),
		"The stored mobile number is kept")
}

func (ts *MFARecipientTestSuite) TestPasswordSMSOTP_UserWithoutMobileEnrollsOne() {
	username := ts.createUser("")
	ts.mockSMSServer.ClearMessages()

	step := ts.submitPassword(ts.smsAppID, username, nil)
	ts.Require().Equal("INCOMPLETE", step.FlowStatus)
	ts.Require().True(common.HasInput(step.Data.Inputs, "mobile_number"), "The mobile number should be collected")

	step, err := common.CompleteFlow(step.ExecutionID, map[string]string{"mobile_number": "+1555666777"}, "",
		step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the mobile number")
	ts.Require().True(common.HasInput(step.Data.Inputs, "otp"), "The OTP should be requested")

	var message *testutils.SMSMessage
	ts.Require().Eventually(func() bool {
		message = ts.mockSMSServer.GetLastMessage()
		return message != nil
	}, 5*time.Second, 100*time.Millisecond, "The OTP should be sent")

	finalStep, err := common.CompleteFlow(step.ExecutionID, map[string]string{"otp": message.OTP}, "action_otp",
		step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the OTP")
	ts.Require().Equal("COMPLETE", finalStep.FlowStatus)
	ts.Equal("+1555666777", ts.storedAttribute(username, "mobile_number"), "The collected mobile number is stored")
}

func (ts *MFARecipientTestSuite) TestPasswordSMSOTP_MobileIsStoredOnlyAfterVerification() {
	username := ts.createUser("")

	step := ts.submitMobile(ts.submitPassword(ts.enrollAppID, username, nil), "+1555000111")
	ts.Require().True(common.HasInput(step.Data.Inputs, "otp"), "The OTP should be requested")
	ts.Nil(ts.storedAttribute(username, "mobile_number"), "An unverified mobile number isn't stored")

	step = ts.submitMobile(ts.submitPassword(ts.enrollAppID, username, nil), "+1555666777")
	ts.Require().True(common.HasInput(step.Data.Inputs, "otp"), "The OTP should be requested")

	var message *testutils.SMSMessage
	ts.Require().Eventually(func() bool {
		message = ts.mockSMSServer.GetLastMessage()
		return message != nil
	}, 5*time.Second, 100*time.Millisecond, "The OTP should be sent")

	finalStep, err := common.CompleteFlow(step.ExecutionID, map[string]string{"otp": message.OTP}, "action_otp",
		step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the OTP")
	ts.Require().Equal("COMPLETE", finalStep.FlowStatus)
	ts.Equal("+1555666777", ts.storedAttribute(username, "mobile_number"), "The verified mobile number is stored")
}

func (ts *MFARecipientTestSuite) TestPasswordSMSOTP_StoredMobileSkipsTheMobilePrompt() {
	username := ts.createUser(`"mobile_number": "+1222333444"`)

	step := ts.submitPassword(ts.enrollAppID, username, nil)

	ts.Require().Equal("INCOMPLETE", step.FlowStatus)
	ts.True(common.HasInput(step.Data.Inputs, "otp"), "The OTP should be requested")
}

// createApp creates the flow and an application that signs in with it, and returns the application ID.
func (ts *MFARecipientTestSuite) createApp(clientID string, flow testutils.Flow) string {
	flowID, err := testutils.CreateFlow(flow)
	ts.Require().NoError(err, "Failed to create flow %s", flow.Handle)
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)

	appID, err := testutils.CreateApplication(testutils.Application{
		OUID:             mfaRecipientTestOU.ID,
		Name:             flow.Name + " Application",
		ClientID:         clientID,
		ClientSecret:     clientID + "_secret",
		RedirectURIs:     []string{"http://localhost:3000/callback"},
		AllowedUserTypes: []string{mfaRecipientUserType.Name},
		AuthFlowID:       flowID,
	})
	ts.Require().NoError(err, "Failed to create the application for %s", flow.Handle)
	return appID
}

// createUser creates a user with a password and the given extra attributes, deleted when the test ends.
func (ts *MFARecipientTestSuite) createUser(extraAttributes string) string {
	username := common.GenerateUniqueUsername("mfarecipient")
	attributes := `{"username": "` + username + `", "password": "testpassword"`
	if extraAttributes != "" {
		attributes += ", " + extraAttributes
	}
	userIDs, err := testutils.CreateMultipleUsers(testutils.User{
		OUID:       mfaRecipientTestOU.ID,
		Type:       mfaRecipientUserType.Name,
		Attributes: json.RawMessage(attributes + "}"),
	})
	ts.Require().NoError(err, "Failed to create the test user")
	ts.T().Cleanup(func() {
		if err := testutils.CleanupUsers(userIDs); err != nil {
			ts.T().Logf("Failed to delete the test user: %v", err)
		}
	})
	return username
}

// submitPassword starts a sign-in and submits the user's password along with any extra inputs.
func (ts *MFARecipientTestSuite) submitPassword(appID, username string,
	extraInputs map[string]string) *common.FlowStep {
	step, err := common.InitiateAuthenticationFlow(appID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")

	inputs := map[string]string{"username": username, "password": "testpassword"}
	for key, value := range extraInputs {
		inputs[key] = value
	}
	step, err = common.CompleteFlow(step.ExecutionID, inputs, "", step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the password")
	return step
}

// submitMobile checks that the step asks for a mobile number, submits the given one and returns the next step.
func (ts *MFARecipientTestSuite) submitMobile(step *common.FlowStep, mobile string) *common.FlowStep {
	ts.Require().Equal("INCOMPLETE", step.FlowStatus)
	ts.Require().True(common.HasInput(step.Data.Inputs, "mobile_number"), "The mobile number should be collected")
	ts.mockSMSServer.ClearMessages()

	step, err := common.CompleteFlow(step.ExecutionID, map[string]string{"mobile_number": mobile}, "action_mobile",
		step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the mobile number")
	ts.Require().Equal("INCOMPLETE", step.FlowStatus)
	return step
}

// storedAttribute reads the attribute on the user's profile.
func (ts *MFARecipientTestSuite) storedAttribute(username, attribute string) interface{} {
	user, err := testutils.FindUserByAttribute("username", username)
	ts.Require().NoError(err, "Failed to read the user back")
	ts.Require().NotNil(user, "The user should exist")
	attrs, err := testutils.GetUserAttributes(*user)
	ts.Require().NoError(err, "Failed to read the user's attributes")
	return attrs[attribute]
}

// buildPasswordSMSOTPFlow builds a password then SMS OTP flow, collecting a mobile number from a user who has none.
func buildPasswordSMSOTPFlow(senderID string) testutils.Flow {
	return testutils.Flow{
		Name:     "MFA Recipient Password SMS OTP",
		FlowType: "AUTHENTICATION",
		Handle:   "mfa_recipient_password_sms_otp",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "credentials_auth"},
			{
				"id":        "credentials_auth",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "CredentialsAuthExecutor"},
				"onSuccess": "attribute_collect",
			},
			{
				"id":   "attribute_collect",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "AttributeCollector",
					"inputs": []map[string]interface{}{
						{"ref": "input_mobile", "identifier": "mobile_number", "type": "PHONE_INPUT", "required": true},
					},
				},
				"onSuccess": "generate_otp",
			},
			{
				"id":   "generate_otp",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "OTPExecutor",
					"mode": "generate",
					"inputs": []map[string]interface{}{
						{"ref": "input_mobile", "identifier": "mobile_number", "type": "PHONE_INPUT", "required": true},
					},
				},
				"onSuccess": "sms_send",
			},
			{
				"id":         "sms_send",
				"type":       "TASK_EXECUTION",
				"properties": map[string]interface{}{"senderId": senderID, "smsTemplate": "OTP"},
				"executor":   map[string]interface{}{"name": "SMSExecutor"},
				"onSuccess":  "prompt_otp",
			},
			{
				"id":   "prompt_otp",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{"ref": "input_otp", "identifier": "otp", "type": "OTP_INPUT", "required": true},
						},
						"action": map[string]interface{}{"ref": "action_otp", "nextNode": "verify_otp"},
					},
				},
			},
			{
				"id":        "verify_otp",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "OTPExecutor", "mode": "verify"},
				"onSuccess": "auth_assert",
			},
			{
				"id":        "auth_assert",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "AuthAssertExecutor"},
				"onSuccess": "end",
			},
			{"id": "end", "type": "END"},
		},
	}
}

// buildPasswordSMSOTPEnrollFlow builds a password then SMS OTP flow that stores a collected mobile number only
// after the OTP is verified.
func buildPasswordSMSOTPEnrollFlow(senderID string) testutils.Flow {
	mobileInput := []map[string]interface{}{
		{"ref": "input_mobile", "identifier": "mobile_number", "type": "PHONE_INPUT", "required": true},
	}
	return testutils.Flow{
		Name:     "MFA Recipient Password SMS OTP Enrollment",
		FlowType: "AUTHENTICATION",
		Handle:   "mfa_recipient_password_sms_otp_enroll",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "credentials_auth"},
			{
				"id":        "credentials_auth",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "CredentialsAuthExecutor"},
				"onSuccess": "attribute_collect",
			},
			{
				"id":           "attribute_collect",
				"type":         "TASK_EXECUTION",
				"executor":     map[string]interface{}{"name": "AttributeCollector", "inputs": mobileInput},
				"onSuccess":    "generate_otp",
				"onIncomplete": "prompt_mobile",
			},
			{
				"id":   "prompt_mobile",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": mobileInput,
						"action": map[string]interface{}{"ref": "action_mobile", "nextNode": "generate_otp"},
					},
				},
			},
			{
				"id":        "generate_otp",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "OTPExecutor", "mode": "generate", "inputs": mobileInput},
				"onSuccess": "sms_send",
			},
			{
				"id":         "sms_send",
				"type":       "TASK_EXECUTION",
				"properties": map[string]interface{}{"senderId": senderID, "smsTemplate": "OTP"},
				"executor":   map[string]interface{}{"name": "SMSExecutor"},
				"onSuccess":  "prompt_otp",
			},
			{
				"id":   "prompt_otp",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{"ref": "input_otp", "identifier": "otp", "type": "OTP_INPUT", "required": true},
						},
						"action": map[string]interface{}{"ref": "action_otp", "nextNode": "verify_otp"},
					},
				},
			},
			{
				"id":        "verify_otp",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "OTPExecutor", "mode": "verify"},
				"onSuccess": "save_mobile",
			},
			{
				"id":        "save_mobile",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "AttributeCollector", "inputs": mobileInput},
				"onSuccess": "auth_assert",
			},
			{
				"id":        "auth_assert",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "AuthAssertExecutor"},
				"onSuccess": "end",
			},
			{"id": "end", "type": "END"},
		},
	}
}
