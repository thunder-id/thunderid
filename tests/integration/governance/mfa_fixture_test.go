// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// secondFactorAction is the action the MFA flow's code prompt submits.
const secondFactorAction = "action_otp"

// mfaFixture is a password then SMS one-time-password sign-in flow, a mock SMS provider and an
// application using the flow.
type mfaFixture struct {
	server   *testutils.MockNotificationServer
	senderID string
	flowID   string
	appID    string
}

// newMFAFixture creates an SMS sender, the MFA flow and an application of userType in ouID using the
// flow.
func newMFAFixture(t *testing.T, name, ouID, userType string) *mfaFixture {
	t.Helper()

	f := &mfaFixture{}
	f.server, f.senderID = newSMSSender(t, name+" MFA")
	t.Cleanup(func() { f.tearDown(t) })

	var err error
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
		map[string]string{"username": username, "password": password}, signInAction, step.ChallengeToken)
	require.NoError(t, err, "failed to submit the password")
	return step
}

// sentCode returns the code the mock provider last received.
func (f *mfaFixture) sentCode(t *testing.T) string {
	t.Helper()
	message := awaitDelivered(t, f.server.GetLastMessage, "a code must have been sent")
	require.NotEmpty(t, message.OTP)
	return message.OTP
}

// submitCode submits a one-time password at the prompt.
func (f *mfaFixture) submitCode(t *testing.T, step *common.FlowStep, code string) *common.FlowStep {
	t.Helper()
	next, err := common.CompleteFlow(step.ExecutionID, map[string]string{"otp": code}, secondFactorAction,
		step.ChallengeToken)
	require.NoError(t, err, "failed to submit the code")
	return next
}

// tearDown deletes the application and the flow, logging rather than failing. The SMS sender goes
// after them.
func (f *mfaFixture) tearDown(t *testing.T) {
	t.Helper()
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
}

// mfaFlow is a sign-in flow that checks the password and then a one-time password sent by SMS
// through senderID.
func mfaFlow(name, senderID string) testutils.Flow {
	return testutils.Flow{
		Name:     "Governance " + name + " MFA flow",
		FlowType: "AUTHENTICATION",
		Handle:   "governance_" + name + "_mfa_flow",
		Nodes: []map[string]interface{}{
			startNode("prompt_credentials"),
			promptNode("prompt_credentials", signInAction, "credentials_auth", usernameInput, passwordInput),
			withExecutorInputs(taskNode("credentials_auth", "CredentialsAuthExecutor", "", "generate_otp",
				map[string]interface{}{"onIncomplete": "prompt_credentials"}), usernameInput, passwordInput),
			withExecutorInputs(taskNode("generate_otp", "OTPExecutor", "generate", "sms_send", nil),
				flowInput("input_mobile", "mobile_number", "PHONE_INPUT")),
			smsSendNode(senderID, "prompt_otp", nil),
			promptNode("prompt_otp", secondFactorAction, "verify_otp", otpInput),
			withExecutorInputs(taskNode("verify_otp", "OTPExecutor", "verify", "auth_assert",
				map[string]interface{}{"onIncomplete": "prompt_otp"}), otpInput),
			taskNode("auth_assert", "AuthAssertExecutor", "", "end", nil),
			endNode(),
		},
	}
}
