// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const mfaMockSMSPort = 8106

// mfaFixture is a password then SMS one-time-password sign-in flow, a mock SMS provider and an
// application using the flow.
type mfaFixture struct {
	server   *testutils.MockNotificationServer
	senderID string
	flowID   string
	appID    string
}

func newMFAFixture(t *testing.T, name, ouID, userType string) *mfaFixture {
	t.Helper()

	f := &mfaFixture{server: testutils.NewMockNotificationServer(mfaMockSMSPort)}
	require.NoError(t, f.server.Start(), "failed to start the mock SMS server")
	time.Sleep(100 * time.Millisecond)

	var err error
	f.senderID, err = testutils.CreateNotificationSender(testutils.NotificationSender{
		Name:     "Governance " + name + " MFA sender",
		Provider: "custom",
		Properties: []testutils.SenderProperty{
			{Name: "url", Value: f.server.GetSendSMSURL()},
			{Name: "http_method", Value: "POST"},
			{Name: "content_type", Value: "JSON"},
		},
	})
	require.NoError(t, err, "failed to create the SMS sender")

	f.flowID, err = testutils.CreateFlow(mfaFlow(name, f.senderID))
	require.NoError(t, err, "failed to create the MFA flow")

	f.appID, err = testutils.CreateApplication(testutils.Application{
		Name:             "Governance " + name + " MFA app",
		ClientID:         "governance_" + name + "_mfa_client",
		ClientSecret:     "governance_" + name + "_mfa_secret",
		RedirectURIs:     []string{"http://localhost:3000/callback"},
		AllowedUserTypes: []string{userType},
		AuthFlowID:       f.flowID,
		OUID:             ouID,
	})
	require.NoError(t, err, "failed to create the MFA application")
	return f
}

// passPassword starts the flow and passes the first factor. It returns the step prompting for the
// one-time password.
func (f *mfaFixture) passPassword(t *testing.T, username, password string) *common.FlowStep {
	t.Helper()

	step := f.submitPassword(t, username, password)
	require.Equal(t, "INCOMPLETE", step.FlowStatus, "the password must pass and prompt for the code")
	return step
}

// submitPassword starts the MFA flow and submits the password, returning the next step whatever it is.
func (f *mfaFixture) submitPassword(t *testing.T, username, password string) *common.FlowStep {
	t.Helper()

	step, err := common.InitiateAuthenticationFlow(f.appID, false, nil, "")
	require.NoError(t, err, "failed to start the MFA flow")
	f.server.ClearMessages()

	step, err = common.CompleteFlow(step.ExecutionID,
		map[string]string{"username": username, "password": password}, "action_credentials", step.ChallengeToken)
	require.NoError(t, err, "failed to submit the password")
	return step
}

// sentCode returns the code the mock provider last received.
func (f *mfaFixture) sentCode(t *testing.T) string {
	t.Helper()
	message := awaitSMS(f.server)
	require.NotNil(t, message, "a code must have been sent")
	require.NotEmpty(t, message.OTP)
	return message.OTP
}

// requireNoCodeSent requires that the step just taken sent nothing. Dispatch is synchronous, so a
// sent code is already at the mock when the step returns.
func (f *mfaFixture) requireNoCodeSent(t *testing.T, msg string) {
	t.Helper()
	require.Equal(t, 0, smsCount(t, f.server), msg)
}

// submitCode submits a one-time password at the prompt.
func (f *mfaFixture) submitCode(t *testing.T, step *common.FlowStep, code string) *common.FlowStep {
	t.Helper()
	next, err := common.CompleteFlow(step.ExecutionID, map[string]string{"otp": code}, "action_otp",
		step.ChallengeToken)
	require.NoError(t, err, "failed to submit the code")
	return next
}

func (f *mfaFixture) tearDown(t *testing.T) {
	t.Helper()
	if f == nil {
		return
	}
	if f.appID != "" {
		if err := testutils.DeleteApplication(f.appID); err != nil {
			t.Logf("teardown: failed to delete the MFA app: %v", err)
		}
	}
	if f.flowID != "" {
		if err := testutils.DeleteFlow(f.flowID); err != nil {
			t.Logf("teardown: failed to delete the MFA flow: %v", err)
		}
	}
	if f.senderID != "" {
		if err := testutils.DeleteNotificationSender(f.senderID); err != nil {
			t.Logf("teardown: failed to delete the SMS sender: %v", err)
		}
	}
	if f.server != nil {
		_ = f.server.Stop()
	}
}

func mfaFlow(name, senderID string) testutils.Flow {
	credentialInputs := []map[string]interface{}{
		{"ref": "input_username", "identifier": "username", "type": "TEXT_INPUT", "required": true},
		{"ref": "input_password", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
	}
	otpInputs := []map[string]interface{}{
		{"ref": "input_otp", "identifier": "otp", "type": "OTP_INPUT", "required": true},
	}
	return testutils.Flow{
		Name:     "Governance " + name + " MFA flow",
		FlowType: "AUTHENTICATION",
		Handle:   "governance_" + name + "_mfa_flow",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "prompt_credentials"},
			{
				"id":   "prompt_credentials",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{{
					"inputs": credentialInputs,
					"action": map[string]interface{}{"ref": "action_credentials", "nextNode": "credentials_auth"},
				}},
			},
			{
				"id":           "credentials_auth",
				"type":         "TASK_EXECUTION",
				"executor":     map[string]interface{}{"name": "CredentialsAuthExecutor", "inputs": credentialInputs},
				"onSuccess":    "generate_otp",
				"onIncomplete": "prompt_credentials",
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
				"properties": map[string]interface{}{"senderId": senderID, "smsTemplate": "otp"},
				"executor":   map[string]interface{}{"name": "SMSExecutor"},
				"onSuccess":  "prompt_otp",
			},
			{
				"id":   "prompt_otp",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{{
					"inputs": otpInputs,
					"action": map[string]interface{}{"ref": "action_otp", "nextNode": "verify_otp"},
				}},
			},
			{
				"id":           "verify_otp",
				"type":         "TASK_EXECUTION",
				"executor":     map[string]interface{}{"name": "OTPExecutor", "mode": "verify", "inputs": otpInputs},
				"onSuccess":    "auth_assert",
				"onIncomplete": "prompt_otp",
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
