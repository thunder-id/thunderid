// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package federated

import (
	"fmt"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

/*
Account linking through a flow.

The linking node withholds a match and forwards to a prompt, so linking is not observable without
driving that prompt, and the unit tests cannot reach what these assert: that the refusal travels as
a forwarded action type across a real node boundary, and that the link the node writes is what
authenticates the *next* sign-in.

The graph puts a provisioning step behind the linking node, since that is where a refused match has
to land.
*/

// fedVerifiedLinkingFlow is the strict authentication graph. The link action calls the verification
// flow, and whoever that flow authenticates is checked against the candidates when it returns to the
// linking node. The refusal points back at the linking node, because a forwarded action type reaches
// only the node its action names.
func fedVerifiedLinkingFlow(idpID, verifyFlowID string) testutils.Flow {
	return testutils.Flow{
		Name:     "Federated Verified Linking Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_federated_linking_verified",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "oidc_auth"},
			{
				"id":   "oidc_auth",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"idpId": idpID,
					// A refused match carries on as a new user, and provisioning only runs on this
					// path when the federated node allows an identity with no local account.
					"allowAuthenticationWithoutLocalUser": true,
				},
				"executor":  map[string]interface{}{"name": "OIDCAuthExecutor"},
				"onSuccess": "linking",
			},
			{
				"id":           "linking",
				"type":         "TASK_EXECUTION",
				"executor":     map[string]interface{}{"name": "AccountLinkingExecutor"},
				"onSuccess":    "provisioning",
				"onIncomplete": "linking_prompt",
			},
			{
				"id":   "linking_prompt",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{"action": map[string]interface{}{
						"ref": common.LinkAccountAction, "type": "CONFIRM", "nextNode": "verify_account"}},
					{"action": map[string]interface{}{
						"ref": common.SeparateAccountAction, "type": "REJECT", "nextNode": "linking"}},
				},
			},
			{
				"id":        "verify_account",
				"type":      "CALL",
				"flow":      map[string]interface{}{"ref": verifyFlowID},
				"onSuccess": "linking",
			},
			{
				"id":        "provisioning",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "ProvisioningExecutor"},
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

// verifiedLinkingApp creates a scenario application on fedVerifiedLinkingFlow, calling verifyFlow to
// prove the account.
func (s *FederatedMappingSuite) verifiedLinkingApp(clientID string, verifyFlow testutils.Flow) string {
	s.T().Helper()
	return s.createScenarioApp(fedVerifiedLinkingFlow(s.idpID, s.createVerifyFlow(verifyFlow)), clientID)
}

// passwordLinkingApp is verifiedLinkingApp proving the account with a username and password.
func (s *FederatedMappingSuite) passwordLinkingApp(clientID string) string {
	s.T().Helper()
	return s.verifiedLinkingApp(clientID, common.VerifyPasswordFlow("verify_flow_"+clientID))
}

const (
	candidatePassword = "Candidate#Secret1"

	// The codes the refusal and a failed verification carry. Asserted rather than "any failure":
	// these paths would otherwise be satisfied by an unrelated fault in the same graph.
	provisioningConflictCode = "FET-1080"
	invalidCredentialsCode   = "FET-1005"
	nonCandidateVerifiedCode = "FET-1093"
)

// linkTarget creates the account a federated identity will match, and returns the email it matches
// on with the account's id. The password is what the scenarios prove ownership with.
func (s *FederatedMappingSuite) linkTarget() (string, string) {
	s.T().Helper()
	email := s.nextSubject() + "@example.com"
	return email, s.createLocalUser(map[string]interface{}{
		"username": email,
		"email":    email,
		"password": candidatePassword,
	})
}

// matchingIdentity is a federated identity carrying email, and a configuration that links on it.
// Provisioning needs the username too: without it a refused match stops to collect one instead of
// failing on the conflict.
func (s *FederatedMappingSuite) matchingIdentity(email string) (
	*testutils.OIDCUserInfo, *testutils.AttributeConfiguration) {
	s.T().Helper()
	user := s.baseUser(s.nextSubject())
	user.Email = email
	return user, linkOn([]string{"email"}, pair("email", "email"), pair("email", "username"))
}

// requireLinkingPrompt asserts the step is the prompt the linking node forwarded to, showing the
// values the account was matched on. The connection maps the email claim onto both email and
// username, and the account holds the address in both, so both rows show.
func (s *FederatedMappingSuite) requireLinkingPrompt(step *common.FlowStep, email string) {
	s.T().Helper()
	s.Require().Equal("INCOMPLETE", step.FlowStatus, "expected the linking prompt, got %+v", step)
	s.Require().Equal("VIEW", step.Type, "expected the linking prompt to be a view, got %+v", step)
	s.JSONEq(`[{"label":"Email","value":"`+email+`"},{"label":"Username","value":"`+email+`"}]`,
		step.Data.AdditionalData["linkingPromptDetails"],
		"the prompt should name the matched account, got %+v", step.Data.AdditionalData)
}

// BL1: the confirmation calls the verification flow, and the pass that follows links because that
// flow authenticated the candidate. The flow runs in a frame of its own, so the End-User names the
// account as in any other sign-in: the email claim this connection maps onto username does not
// answer it for them.
func (s *FederatedMappingSuite) TestLinksAfterPasswordVerification() {
	email, existingID := s.linkTarget()
	user, config := s.matchingIdentity(email)
	appID := s.passwordLinkingApp("federated-verified")

	step, err := s.authenticate(appID, config, user)
	s.Require().NoError(err, "the matched identity should reach the linking prompt")
	s.requireLinkingPrompt(step, email)

	verification, err := common.CompleteFlow(step.ExecutionID, nil, common.LinkAccountAction, step.ChallengeToken)
	s.Require().NoError(err, "confirming the match should route to the verification step")
	s.Require().Equal("INCOMPLETE", verification.FlowStatus,
		"the candidate should be asked to prove the account, got %+v", verification)
	s.Require().True(common.HasInput(verification.Data.Inputs, "username"),
		"the claim mapped onto username must not name the account, got %+v", verification.Data.Inputs)
	s.Require().True(common.HasInput(verification.Data.Inputs, "password"),
		"the verification step should collect a password, got %+v", verification.Data.Inputs)

	final, err := common.CompleteFlow(verification.ExecutionID,
		map[string]string{"username": email, "password": candidatePassword}, common.VerifyPasswordAction,
		verification.ChallengeToken)
	s.Require().NoError(err, "verifying the account should complete the flow")
	s.Require().Equal("COMPLETE", final.FlowStatus, "the verified match should authenticate, got %+v", final)

	claims, err := testutils.DecodeJWT(final.Assertion)
	s.Require().NoError(err, "failed to decode the assertion")
	s.Equal(existingID, claims.Sub, "the assertion should be issued for the verified account")

	repeat, err := s.authenticate(appID, config, user)
	s.Require().NoError(err, "the linked identity should authenticate again")
	s.Require().Equal("COMPLETE", repeat.FlowStatus,
		"a recorded link should authenticate outright, with no verification, got %+v", repeat)
}

// BL2: a wrong password proves nothing, so the candidate stays unlinked and the flow does not
// authenticate. Confirming the match alone never settles it.
func (s *FederatedMappingSuite) TestLinkingRejectsAWrongPassword() {
	email, _ := s.linkTarget()
	user, config := s.matchingIdentity(email)
	appID := s.passwordLinkingApp("federated-verified-wrong")

	step, err := s.authenticate(appID, config, user)
	s.Require().NoError(err, "the matched identity should reach the linking prompt")

	verification, err := common.CompleteFlow(step.ExecutionID, nil, common.LinkAccountAction, step.ChallengeToken)
	s.Require().NoError(err, "confirming the match should route to the verification step")

	final, err := common.CompleteFlow(verification.ExecutionID,
		map[string]string{"username": email, "password": "not-" + candidatePassword}, common.VerifyPasswordAction,
		verification.ChallengeToken)
	s.Require().NoError(err, "a failed verification is still a 200 carrying its state, got %v", err)
	s.Require().Equal("INCOMPLETE", final.FlowStatus,
		"an unproven account must not authenticate, got %+v", final)
	s.Require().NotNil(final.Error, "the re-prompt should say why, got %+v", final)
	s.Equal(invalidCredentialsCode, final.Error.Code,
		"expected the invalid-credentials error, got %+v", final.Error)
	s.Empty(final.Assertion, "an unproven account must not issue an assertion")
	s.True(common.HasInput(final.Data.Inputs, "password"),
		"the candidate should be asked again rather than dropped, got %+v", final.Data.Inputs)
}

// BL3: a refusal skips verification, drops the candidate and carries on as a new user. Provisioning
// then creates a user holding the attribute that matched, and the uniqueness rule on that attribute
// is what stops a second account on the same address. There is no separate "you refused" error:
// honouring the refusal and refusing the duplicate are the same outcome.
func (s *FederatedMappingSuite) TestLinkingRefusalIsStoppedByUniqueness() {
	email, existingID := s.linkTarget()
	user, config := s.matchingIdentity(email)
	appID := s.passwordLinkingApp("federated-verified-refused")

	step, err := s.authenticate(appID, config, user)
	s.Require().NoError(err, "the matched identity should reach the linking prompt")
	s.requireLinkingPrompt(step, email)

	final, err := common.CompleteFlow(step.ExecutionID, nil, common.SeparateAccountAction, step.ChallengeToken)
	s.Require().NoError(err, "a failed flow is still a 200 carrying its error, got %v", err)
	s.Require().Equal("ERROR", final.FlowStatus,
		"the refusal should skip verification and land on the uniqueness rule, got %+v", final)
	s.Require().NotNil(final.Error, "the terminal step should carry its error, got %+v", final)
	s.Equal(provisioningConflictCode, final.Error.Code,
		"the duplicate should be refused by the uniqueness rule, got %+v", final.Error)
	s.Empty(final.Assertion, "a refused match must not issue an assertion")

	// The refusal must not have linked the identity either: the account it declined is untouched,
	// so a repeat sign-in is offered the same choice rather than resolving straight through.
	repeat, err := s.authenticate(appID, config, user)
	s.Require().NoError(err, "the refused identity should reach the prompt again")
	s.requireLinkingPrompt(repeat, email)

	found, err := testutils.FindUserByAttribute("email", email)
	s.Require().NoError(err, "failed to look up the account holding the matched address")
	s.Require().NotNil(found, "the pre-existing account should still hold the address")
	s.Equal(existingID, found.ID, "the address should still belong to the account that had it")
}

// BL4: the shape Google and OIDC connections are seeded with, linking on email while email is mapped
// onto username. The account's username is not its address, so only its email can match: mappings
// copy the claim there, and linking follows it. Proving the account links it, and the link is what
// the next sign-in resolves on.
func (s *FederatedMappingSuite) TestLinksOnEmailWhenEmailIsMappedToUsername() {
	email := s.nextSubject() + "@example.com"
	username := s.nextSubject()
	existingID := s.createLocalUser(map[string]interface{}{
		"username": username,
		"email":    email,
		"password": candidatePassword,
	})
	user := s.baseUser(s.nextSubject())
	user.Email = email
	config := linkOn([]string{"email"}, pair("email", "username"))
	appID := s.passwordLinkingApp("federated-verified-seeded")

	step, err := s.authenticate(appID, config, user)
	s.Require().NoError(err, "the matched identity should reach the linking prompt")
	s.Require().Equal("INCOMPLETE", step.FlowStatus, "expected the linking prompt, got %+v", step)
	s.JSONEq(`[{"label":"Email","value":"`+email+`"}]`, step.Data.AdditionalData["linkingPromptDetails"],
		"the account should match on its email alone, got %+v", step.Data.AdditionalData)

	verification, err := common.CompleteFlow(step.ExecutionID, nil, common.LinkAccountAction, step.ChallengeToken)
	s.Require().NoError(err, "confirming the match should route to the verification step")
	final, err := common.CompleteFlow(verification.ExecutionID,
		map[string]string{"username": username, "password": candidatePassword}, common.VerifyPasswordAction,
		verification.ChallengeToken)
	s.Require().NoError(err, "verifying the account should complete the flow")
	s.Require().Equal("COMPLETE", final.FlowStatus, "the verified match should authenticate, got %+v", final)

	claims, err := testutils.DecodeJWT(final.Assertion)
	s.Require().NoError(err, "failed to decode the assertion")
	s.Equal(existingID, claims.Sub, "the assertion should be issued for the verified account")

	repeat, err := s.authenticate(appID, config, user)
	s.Require().NoError(err, "the linked identity should authenticate again")
	s.Require().Equal("COMPLETE", repeat.FlowStatus,
		"a recorded link should authenticate outright, with no verification, got %+v", repeat)
}

// splitTargets creates two accounts that one address names through different attributes: the first
// holds it as a username, the second as an email. It returns the address, and each account's
// username and id, in that order. Both prove ownership with candidatePassword.
func (s *FederatedMappingSuite) splitTargets() (string, [2]string, [2]string) {
	s.T().Helper()
	email := s.nextSubject() + "@example.com"
	secondUsername := s.nextSubject()
	firstID := s.createLocalUser(map[string]interface{}{
		"username": email, "email": s.nextSubject() + "@example.com", "password": candidatePassword,
	})
	secondID := s.createLocalUser(map[string]interface{}{
		"username": secondUsername, "email": email, "password": candidatePassword,
	})
	return email, [2]string{email, secondUsername}, [2]string{firstID, secondID}
}

// splitIdentity is a federated identity whose email names both split targets, and a configuration
// that links on that email.
func (s *FederatedMappingSuite) splitIdentity(email string) (
	*testutils.OIDCUserInfo, *testutils.AttributeConfiguration) {
	s.T().Helper()
	user := s.baseUser(s.nextSubject())
	user.Email = email
	return user, linkOn([]string{"email"}, pair("email", "email"), pair("email", "username"))
}

// requireSplitVerification drives the split identity to the password step and asserts it asks who
// the End-User is as well.
func (s *FederatedMappingSuite) requireSplitVerification(appID string,
	config *testutils.AttributeConfiguration, user *testutils.OIDCUserInfo) *common.FlowStep {
	s.T().Helper()
	step, err := s.authenticate(appID, config, user)
	s.Require().NoError(err, "the split identity should reach the linking prompt")
	s.requireLinkingPrompt(step, user.Email)

	verification, err := common.CompleteFlow(step.ExecutionID, nil, common.LinkAccountAction, step.ChallengeToken)
	s.Require().NoError(err, "confirming the match should route to the verification step")
	s.Require().Equal("INCOMPLETE", verification.FlowStatus,
		"the End-User should be asked to prove an account, got %+v", verification)
	s.Require().True(common.HasInput(verification.Data.Inputs, "username"),
		"two candidates need the End-User to say which account, got %+v", verification.Data.Inputs)
	s.Require().True(common.HasInput(verification.Data.Inputs, "password"),
		"the verification step should collect a password, got %+v", verification.Data.Inputs)
	return verification
}

// BL5: two accounts match through different attributes. Whichever one the End-User proves is the
// one linked, and the link is what the next sign-in resolves on.
func (s *FederatedMappingSuite) TestLinksWhicheverCandidateIsVerified() {
	email, usernames, ids := s.splitTargets()
	user, config := s.splitIdentity(email)
	appID := s.passwordLinkingApp("federated-verified-split")

	verification := s.requireSplitVerification(appID, config, user)
	final, err := common.CompleteFlow(verification.ExecutionID,
		map[string]string{"username": usernames[1], "password": candidatePassword}, common.VerifyPasswordAction,
		verification.ChallengeToken)
	s.Require().NoError(err, "verifying the account should complete the flow")
	s.Require().Equal("COMPLETE", final.FlowStatus, "the verified candidate should authenticate, got %+v", final)

	claims, err := testutils.DecodeJWT(final.Assertion)
	s.Require().NoError(err, "failed to decode the assertion")
	s.Equal(ids[1], claims.Sub, "the assertion should be issued for the account that was verified")

	repeat, err := s.authenticate(appID, config, user)
	s.Require().NoError(err, "the linked identity should authenticate again")
	s.Require().Equal("COMPLETE", repeat.FlowStatus,
		"a recorded link should authenticate outright, with no verification, got %+v", repeat)
	claims, err = testutils.DecodeJWT(repeat.Assertion)
	s.Require().NoError(err, "failed to decode the repeat assertion")
	s.Equal(ids[1], claims.Sub, "the link should resolve to the account that was verified")
}

// BL6: proving an account the attributes did not name links nothing, even with several candidates
// on offer. The End-User owns that account, but it is not one the connection matched. Picking the
// wrong account is the End-User's to retry, so the linking prompt comes back with the error, still
// naming the match after the called flow returned, and proving a candidate then links it.
func (s *FederatedMappingSuite) TestLinkingRetriesAfterAVerifiedNonCandidate() {
	email, usernames, ids := s.splitTargets()
	outsiderEmail, _ := s.linkTarget()
	user, config := s.splitIdentity(email)
	appID := s.passwordLinkingApp("federated-verified-outsider")

	verification := s.requireSplitVerification(appID, config, user)
	retry, err := common.CompleteFlow(verification.ExecutionID,
		map[string]string{"username": outsiderEmail, "password": candidatePassword}, common.VerifyPasswordAction,
		verification.ChallengeToken)
	s.Require().NoError(err, "verifying a non-candidate should return the linking prompt")
	s.requireLinkingPrompt(retry, user.Email)
	s.Require().NotNil(retry.Error, "the prompt should carry the not-a-candidate error, got %+v", retry)
	s.Equal(nonCandidateVerifiedCode, retry.Error.Code, "expected the not-a-candidate error, got %+v", retry.Error)
	s.Empty(retry.Assertion, "a non-candidate must not issue an assertion")

	verification, err = common.CompleteFlow(retry.ExecutionID, nil, common.LinkAccountAction, retry.ChallengeToken)
	s.Require().NoError(err, "confirming again should route to the verification step")
	s.Require().True(common.HasInput(verification.Data.Inputs, "username"),
		"the retry should ask for the account again, got %+v", verification.Data.Inputs)
	final, err := common.CompleteFlow(verification.ExecutionID,
		map[string]string{"username": usernames[0], "password": candidatePassword}, common.VerifyPasswordAction,
		verification.ChallengeToken)
	s.Require().NoError(err, "verifying a candidate should complete the flow")
	s.Require().Equal("COMPLETE", final.FlowStatus, "the verified candidate should authenticate, got %+v", final)

	claims, err := testutils.DecodeJWT(final.Assertion)
	s.Require().NoError(err, "failed to decode the assertion")
	s.Equal(ids[0], claims.Sub, "the assertion should be issued for the candidate, not the account tried first")
}

// BL7: two accounts share a non-unique linking value, so one lookup names both. Proving either one
// links it. costCenter is used because it is the only non-unique attribute.
func (s *FederatedMappingSuite) TestLinksAVerifiedAccountAmongSharedMatches() {
	costCenter := "CC-" + s.nextSubject()
	usernames := [2]string{s.nextSubject(), s.nextSubject()}
	var ids [2]string
	for i, username := range usernames {
		ids[i] = s.createLocalUser(map[string]interface{}{
			"username":   username,
			"email":      username + "@example.com",
			"costCenter": costCenter,
			"password":   candidatePassword,
		})
	}
	user := s.baseUser(s.nextSubject())
	user.Custom["cost_centre"] = costCenter
	config := linkOn([]string{"costCenter"}, pair("cost_centre", "costCenter"))
	appID := s.passwordLinkingApp("federated-verified-shared")

	step, err := s.authenticate(appID, config, user)
	s.Require().NoError(err, "the shared value should reach the linking prompt")
	s.Require().Equal("INCOMPLETE", step.FlowStatus, "expected the linking prompt, got %+v", step)
	s.JSONEq(`[{"label":"Cost center","value":"`+costCenter+`"}]`,
		step.Data.AdditionalData["linkingPromptDetails"],
		"the prompt should show the shared value once, got %+v", step.Data.AdditionalData)

	verification, err := common.CompleteFlow(step.ExecutionID, nil, common.LinkAccountAction, step.ChallengeToken)
	s.Require().NoError(err, "confirming the match should route to the verification step")
	s.Require().True(common.HasInput(verification.Data.Inputs, "username"),
		"two candidates need the End-User to say which account, got %+v", verification.Data.Inputs)

	final, err := common.CompleteFlow(verification.ExecutionID,
		map[string]string{"username": usernames[0], "password": candidatePassword}, common.VerifyPasswordAction,
		verification.ChallengeToken)
	s.Require().NoError(err, "verifying the account should complete the flow")
	s.Require().Equal("COMPLETE", final.FlowStatus, "the verified candidate should authenticate, got %+v", final)

	claims, err := testutils.DecodeJWT(final.Assertion)
	s.Require().NoError(err, "failed to decode the assertion")
	s.Equal(ids[0], claims.Sub, "the assertion should be issued for the account that was verified")
}

// otpVerifyFlow is a verification flow that proves an account with an SMS OTP sent to the number the
// End-User gives, the shape of the SMS sign-in template. Nothing in it knows about linking.
func otpVerifyFlow(handle, senderID string) testutils.Flow {
	mobileInput := map[string]interface{}{
		"identifier": "mobile_number", "type": "PHONE_INPUT", "required": true,
	}
	return testutils.Flow{
		Name:     "Verify OTP Flow " + handle,
		FlowType: "AUTHENTICATION",
		Handle:   handle,
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "prompt_mobile"},
			{
				"id":   "prompt_mobile",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{mobileInput},
						"action": map[string]interface{}{"ref": "action_mobile", "nextNode": "generate_otp"},
					},
				},
			},
			{
				"id":   "generate_otp",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "OTPExecutor", "mode": "generate",
					"inputs": []map[string]interface{}{mobileInput},
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
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{"identifier": "otp", "type": "OTP_INPUT", "required": true},
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

// BL8: two accounts share a linking value and the End-User proves one with an SMS OTP. The identity
// also asserts the other account's mobile number. The verification flow runs in a frame of its own,
// so that claim neither answers the number prompt nor picks where the OTP goes: the End-User names
// the account, and the one they prove is the one linked.
func (s *FederatedMappingSuite) TestLinksAnAccountVerifiedByOTP() {
	costCenter := "CC-" + s.nextSubject()
	// Unique per run: the OTP step identifies the account by its number.
	s.nextSubject()
	run := fmt.Sprintf("%03d%03d", testutils.GetServerPID()%1000, s.subCounter%1000)
	mobiles := [2]string{"+15550" + run, "+15551" + run}
	var ids [2]string
	for i, mobile := range mobiles {
		username := s.nextSubject()
		ids[i] = s.createLocalUser(map[string]interface{}{
			"username":      username,
			"email":         username + "@example.com",
			"costCenter":    costCenter,
			"mobile_number": mobile,
		})
	}
	user := s.baseUser(s.nextSubject())
	user.Custom["cost_centre"] = costCenter
	user.Custom["mobile_number"] = mobiles[0]
	config := linkOn([]string{"costCenter"}, pair("cost_centre", "costCenter"),
		pair("mobile_number", "mobile_number"))

	mock, senderID := s.startSMSMock()
	appID := s.verifiedLinkingApp("federated-verified-otp", otpVerifyFlow("verify_flow_fed_otp", senderID))
	mock.ClearMessages()

	step, err := s.authenticate(appID, config, user)
	s.Require().NoError(err, "the shared value should reach the linking prompt")
	s.Require().Equal("INCOMPLETE", step.FlowStatus, "expected the linking prompt, got %+v", step)

	verification, err := common.CompleteFlow(step.ExecutionID, nil, common.LinkAccountAction, step.ChallengeToken)
	s.Require().NoError(err, "confirming the match should call the verification flow")
	s.Require().Equal("INCOMPLETE", verification.FlowStatus,
		"the End-User should be asked for a number, got %+v", verification)
	s.Require().True(common.HasInput(verification.Data.Inputs, "mobile_number"),
		"the asserted mobile number must not answer the prompt, got %+v", verification.Data.Inputs)
	s.Nil(mock.GetLastMessage(), "no OTP should be sent before the End-User names a number")

	otpStep, err := common.CompleteFlow(verification.ExecutionID,
		map[string]string{"mobile_number": mobiles[1]}, "action_mobile", verification.ChallengeToken)
	s.Require().NoError(err, "submitting the number should send an OTP")
	s.Require().True(common.HasInput(otpStep.Data.Inputs, "otp"), "expected the OTP prompt, got %+v", otpStep)
	message := awaitSMS(mock)
	s.Require().NotNil(message, "an OTP should be sent to the number the End-User gave")

	final, err := common.CompleteFlow(otpStep.ExecutionID, map[string]string{"otp": message.OTP},
		"action_otp", otpStep.ChallengeToken)
	s.Require().NoError(err, "verifying the OTP should complete the flow")
	s.Require().Equal("COMPLETE", final.FlowStatus, "the verified candidate should authenticate, got %+v", final)

	claims, err := testutils.DecodeJWT(final.Assertion)
	s.Require().NoError(err, "failed to decode the assertion")
	s.Equal(ids[1], claims.Sub, "the assertion should be issued for the account the OTP proved")
}

// oauthVerifyFlow is a verification flow that proves an account by signing in at a federated
// connection, which only resolves an account through a link already recorded there.
func oauthVerifyFlow(handle, idpID string) testutils.Flow {
	return testutils.Flow{
		Name:     "Verify OAuth Flow " + handle,
		FlowType: "AUTHENTICATION",
		Handle:   handle,
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "oauth_auth"},
			{
				"id":         "oauth_auth",
				"type":       "TASK_EXECUTION",
				"properties": map[string]interface{}{"idpId": idpID},
				"executor":   map[string]interface{}{"name": "OAuthExecutor"},
				"onSuccess":  "auth_assert",
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

// BL9: the End-User proves the account by signing in at a second connection it is already linked
// at. That sign-in sets its own connection and subject, but it runs in the called flow's frame,
// so the link recorded on return is the identity the flow started with, which then authenticates
// outright.
func (s *FederatedMappingSuite) TestLinksThePendingIdentityWhenVerifiedAtAnotherConnection() {
	email := s.nextSubject() + "@example.com"
	existingID := s.createLocalUser(map[string]interface{}{
		"username": email, "email": email, "password": linkPassword,
	})
	oauthSub := s.nextSubject()
	s.mockOAuth.AddUser(&testutils.OAuthUserInfo{Sub: oauthSub, Email: email, Name: "OAuth User"})
	s.applyConfigTo("oauth", s.oauthIDPID, linkOn([]string{"email"}, pair("email", "email")))
	s.recordLink("OAuthExecutor", s.oauthIDPID, oauthSub, email)

	user, config := s.matchingIdentity(email)
	appID := s.verifiedLinkingApp("federated-verified-oauth",
		oauthVerifyFlow("verify_flow_fed_oauth", s.oauthIDPID))

	step, err := s.authenticate(appID, config, user)
	s.Require().NoError(err, "the matched identity should reach the linking prompt")
	s.requireLinkingPrompt(step, email)

	redirect, err := common.CompleteFlow(step.ExecutionID, nil, common.LinkAccountAction, step.ChallengeToken)
	s.Require().NoError(err, "confirming the match should call the verification flow")
	s.Require().Equal("REDIRECTION", redirect.Type,
		"the verification flow should redirect to the second connection, got %+v", redirect)

	s.activeSub = oauthSub
	code, state, err := testutils.SimulateFederatedOAuthFlow(redirect.Data.RedirectURL)
	s.Require().NoError(err, "failed to simulate authorization at the second connection")
	final, err := common.CompleteFlow(redirect.ExecutionID, map[string]string{"code": code, "state": state},
		"", redirect.ChallengeToken)
	s.Require().NoError(err, "verifying at the second connection should complete the flow")
	s.Require().Equal("COMPLETE", final.FlowStatus, "the verified match should authenticate, got %+v", final)

	claims, err := testutils.DecodeJWT(final.Assertion)
	s.Require().NoError(err, "failed to decode the assertion")
	s.Equal(existingID, claims.Sub, "the assertion should be issued for the verified account")

	repeat, err := s.authenticate(appID, config, user)
	s.Require().NoError(err, "the linked identity should authenticate again")
	s.Require().Equal("COMPLETE", repeat.FlowStatus,
		"the identity the flow started with should be the one linked, got %+v", repeat)
}
