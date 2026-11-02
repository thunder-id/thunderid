// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package federated

import (
	"time"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// Claims naming flow control state.
//
// A federated identity's claims are stored in the flow's runtime data, which is also where the
// executors keep the state they act on. A claim that shares a name with that state must not set it: the
// identity provider speaks for its own users, not for which local account the flow acts on.

// otpAfterFederationFlow signs in through the connection and then proves the user with an SMS OTP, the
// shape of the social-plus-SMS sign-in template with the generic OIDC executor in place of Google.
func otpAfterFederationFlow(handle, idpID, senderID string) testutils.Flow {
	return testutils.Flow{
		Name:     "Federated OTP Flow " + handle,
		FlowType: "AUTHENTICATION",
		Handle:   handle,
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "oidc_auth"},
			{
				"id":         "oidc_auth",
				"type":       "TASK_EXECUTION",
				"properties": map[string]interface{}{"idpId": idpID},
				"executor":   map[string]interface{}{"name": "OIDCAuthExecutor"},
				"onSuccess":  "provisioning",
			},
			{
				"id":        "provisioning",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "ProvisioningExecutor"},
				"onSuccess": "generate_otp",
			},
			{
				"id":        "generate_otp",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "OTPExecutor", "mode": "generate"},
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

// startSMSMock starts a notification mock and a sender that delivers to it, both removed when the test
// ends.
func (s *FederatedMappingSuite) startSMSMock() (*testutils.MockNotificationServer, string) {
	s.T().Helper()
	mock := testutils.NewMockNotificationServer(0)
	s.Require().NoError(mock.Start(), "failed to start the notification mock")
	s.T().Cleanup(func() { _ = mock.Stop() })

	senderID, err := testutils.CreateNotificationSender(testutils.NotificationSender{
		Name:     "Federated Claim Injection Sender",
		Provider: "custom",
		Properties: []testutils.SenderProperty{
			{Name: "url", Value: mock.GetSendSMSURL()},
			{Name: "http_method", Value: "POST"},
			{Name: "content_type", Value: "JSON"},
		},
	})
	s.Require().NoError(err, "failed to create the notification sender")
	s.T().Cleanup(func() { _ = testutils.DeleteNotificationSender(senderID) })
	return mock, senderID
}

// awaitSMS returns the first message the mock receives within a short window, or nil when none does.
func awaitSMS(mock *testutils.MockNotificationServer) *testutils.SMSMessage {
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
		if message := mock.GetLastMessage(); message != nil {
			return message
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

// An identity provider asserting another account's id and its own phone number must not be sent that
// account's OTP, and so cannot sign in as it.
func (s *FederatedMappingSuite) TestClaimNamingUserIDCannotTakeOverAccountThroughOTP() {
	victimEmail := s.nextSubject() + "@example.com"
	victimID := s.createLocalUser(map[string]interface{}{"username": victimEmail, "email": victimEmail})

	mock, senderID := s.startSMSMock()
	appID := s.createScenarioApp(
		otpAfterFederationFlow("auth_flow_fed_claim_injection", s.idpID, senderID), "fed_claim_injection")

	attacker := s.baseUser(s.nextSubject())
	attacker.Custom["userID"] = victimID
	attacker.Custom["mobile_number"] = "+15550000123"
	s.applyConfig(mapping(fedPersonType.Handle, pair("email", "email")))
	s.mockOIDC.AddUser(attacker)
	mock.ClearMessages()

	step, err := s.authenticateFlow(appID, attacker.Sub)
	s.Require().NoError(err, "the federated callback should be answered")

	message := awaitSMS(mock)
	if message != nil && step.FlowStatus == "INCOMPLETE" && common.HasInput(step.Data.Inputs, "otp") {
		final, err := common.CompleteFlow(
			step.ExecutionID, map[string]string{"otp": message.OTP}, "action_otp", step.ChallengeToken)
		s.Require().NoError(err, "failed to submit the OTP")
		if final.FlowStatus == "COMPLETE" && final.Assertion != "" {
			claims, err := testutils.DecodeJWT(final.Assertion)
			s.Require().NoError(err, "failed to decode the assertion")
			s.NotEqual(victimID, claims.Sub, "the identity provider signed in as the account it named")
		}
	}
	s.Nil(message, "an OTP for the account the claim named was sent to the number the claim named")
}

// conditionalPromptFlow signs in through the connection and then gates a prompt on provisioning
// eligibility, which the executor withholds because the connection does not allow authentication
// without a local user. The prompt makes the condition's outcome visible in the response.
func conditionalPromptFlow(handle, idpID string) testutils.Flow {
	return testutils.Flow{
		Name:     "Federated Conditional Prompt Flow " + handle,
		FlowType: "AUTHENTICATION",
		Handle:   handle,
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "oidc_auth"},
			{
				"id":         "oidc_auth",
				"type":       "TASK_EXECUTION",
				"properties": map[string]interface{}{"idpId": idpID},
				"executor":   map[string]interface{}{"name": "OIDCAuthExecutor"},
				"onSuccess":  "gated_prompt",
			},
			{
				"id":   "gated_prompt",
				"type": "PROMPT",
				"condition": map[string]interface{}{
					"key":    "{{ctx(userEligibleForProvisioning)}}",
					"value":  "true",
					"onSkip": "auth_assert",
				},
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{"ref": "input_gated", "identifier": "gated", "type": "TEXT_INPUT", "required": true},
						},
						"action": map[string]interface{}{"ref": "action_gated", "nextNode": "auth_assert"},
					},
				},
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

// An identity provider asserting a claim named after flow control state must not satisfy a node's
// condition, and so cannot select a node the flow would otherwise skip.
func (s *FederatedMappingSuite) TestClaimNamingFlowControlStateCannotSelectNode() {
	appID := s.createScenarioApp(
		conditionalPromptFlow("auth_flow_fed_condition_injection", s.idpID), "fed_condition_injection")

	attacker := s.baseUser(s.nextSubject())
	attacker.Custom["userEligibleForProvisioning"] = "true"
	s.applyConfig(mapping(fedPersonType.Handle, pair("email", "email")))
	s.mockOIDC.AddUser(attacker)

	step, err := s.authenticateFlow(appID, attacker.Sub)
	s.Require().NoError(err)
	s.False(common.HasInput(step.Data.Inputs, "gated"), "the claim satisfied the node's condition")
}

// A user without a number of their own is sent the OTP at the number the identity provider asserted,
// rather than being asked again for what the provider already said.
func (s *FederatedMappingSuite) TestOTPIsSentToTheClaimedNumber() {
	mock, senderID := s.startSMSMock()
	appID := s.createScenarioApp(
		otpAfterFederationFlow("auth_flow_fed_claimed_number", s.idpID, senderID), "fed_claimed_number")

	user := s.knownOIDCIdentity()
	user.Custom["mobile_number"] = "+15550000456"
	s.mockOIDC.AddUser(user)
	mock.ClearMessages()

	step, err := s.authenticateFlow(appID, user.Sub)
	s.Require().NoError(err, "the federated callback should be answered")
	s.Require().Equal("INCOMPLETE", step.FlowStatus, "expected the flow to wait for the OTP, got %+v", step)
	s.True(common.HasInput(step.Data.Inputs, "otp"), "expected the flow to ask for the OTP, got %+v", step)
	s.False(common.HasInput(step.Data.Inputs, "mobile_number"), "the claimed number should not be asked for")

	// The claim is the only place the flow holds a number, so a sent OTP went to it.
	s.NotNil(awaitSMS(mock), "expected an OTP to be sent to the claimed number")
}

// An attribute the application asks for that the user does not hold falls back to the identity
// provider's claim of that name.
func (s *FederatedMappingSuite) TestAssertionCarriesARequestedClaimTheUserLacks() {
	appID := s.createScenarioApp(federatedNodeFlow("auth_flow_fed_claim_in_assertion", "OIDCAuthExecutor",
		map[string]interface{}{"idpId": s.idpID}), "fed_claim_in_assertion", "city")
	user := s.knownOIDCIdentity()
	user.Custom["city"] = "Colombo"
	s.mockOIDC.AddUser(user)

	step, err := s.authenticateFlow(appID, user.Sub)
	s.Require().NoError(err, "the federated callback should be answered")
	s.Require().Equal("COMPLETE", step.FlowStatus, "got %+v %+v", step, step.Error)

	claims, err := testutils.DecodeJWTPayloadMap(step.Assertion)
	s.Require().NoError(err, "failed to decode the assertion")
	s.Equal("Colombo", claims["city"], "the assertion should carry the claimed city")
}

// collectThenAssertFlow signs in through the connection and then collects the city into the user's
// profile.
func collectThenAssertFlow(handle, idpID string) testutils.Flow {
	return testutils.Flow{
		Name:     "Federated Collect Flow " + handle,
		FlowType: "AUTHENTICATION",
		Handle:   handle,
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "oidc_auth"},
			{
				"id":         "oidc_auth",
				"type":       "TASK_EXECUTION",
				"properties": map[string]interface{}{"idpId": idpID},
				"executor":   map[string]interface{}{"name": "OIDCAuthExecutor"},
				"onSuccess":  "attribute_collect",
			},
			{
				"id":   "attribute_collect",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "AttributeCollector",
					"inputs": []map[string]interface{}{
						{"ref": "input_city", "identifier": "city", "type": "TEXT_INPUT", "required": true},
					},
				},
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

// An attribute the flow collects that the identity provider already asserted is taken from the
// claim rather than asked for.
func (s *FederatedMappingSuite) TestCollectedAttributeIsTakenFromTheClaim() {
	appID := s.createScenarioApp(collectThenAssertFlow("auth_flow_fed_collect_claim", s.idpID), "fed_collect_claim")
	user := s.knownOIDCIdentity()
	user.Custom["city"] = "Kandy"
	s.mockOIDC.AddUser(user)

	step, err := s.authenticateFlow(appID, user.Sub)

	s.Require().NoError(err, "the federated callback should be answered")
	s.False(common.HasInput(step.Data.Inputs, "city"), "the claimed city should not be asked for")
	s.Equal("COMPLETE", step.FlowStatus, "got %+v %+v", step, step.Error)
}
