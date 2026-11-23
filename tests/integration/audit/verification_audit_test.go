// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	verificationRequestEvent    = "user.verification.request"
	verificationVerifyEvent     = "user.verification.verify"
	verificationInvalidateEvent = "user.verification.invalidate"

	adminVerificationFlowHandle = "default-admin-attribute-verification-flow"
	adminVerificationSendNode   = "send_link"

	// errCodeVerificationRefused is the flow's refusal to prepare or record a verification.
	errCodeVerificationRefused = "FET-1112"

	// typedAttributeMarker is an attribute name a caller types, which no line may carry.
	typedAttributeMarker = "AuditTypedAttributeMarker"
)

var (
	anonymousRef           = testutils.AuditRef{Type: "anonymous"}
	verifiedEmailDetails   = map[string]string{"attribute": "email"}
	notVerifiableOutcome   = map[string]string{"result": "failed", "reason": "avs_1001"}
	succeededOutcome       = map[string]string{"result": "succeeded"}
	verificationEmailInput = map[string]interface{}{"identifier": "email", "type": "EMAIL_INPUT", "required": true}
)

// verificationRecovery is a recovery flow that verifies the identified user's email with an emailed code,
// the application that runs it, and the user type and mock SMTP server it works with.
type verificationRecovery struct {
	ouID     string
	userType string
	appID    string
	senderID string
	smtp     *testutils.MockSMTPServer
}

// A recovery runs on the server's authority on a public path that accepts any caller's bearer token. Every
// request here carries the admin's token, so a request line that names no one and a verify line that names
// the user show the actor never comes from the token.
func (ts *AuditLogTestSuite) TestAVerificationThroughRecoveryNamesNeitherTheBearerTokenNorTheContact() {
	r := ts.newVerificationRecovery("verify-recovery")
	userID, username, email := ts.createVerificationUser(r, "verify-recovery", true)
	adminToken, err := testutils.GetAccessToken()
	ts.Require().NoError(err)

	requested, step, code := ts.requestVerificationCode(r, adminToken, username, email)
	verified, step := ts.executeRecovery(adminToken, r.appID, map[string]interface{}{
		"executionId": step.ExecutionID, "challengeToken": step.ChallengeToken,
		"inputs": map[string]string{"otp": code}, "action": "action_submit_otp",
	})
	ts.Require().Equal("COMPLETE", step.FlowStatus, "the recovery must complete once the code is submitted")
	ts.Require().Contains(ts.verifiedAttributes(userID), "email", "the recovery must have verified the email")

	userRef := testutils.AuditRef{Type: "user", ID: userID}
	perCall := ts.linesPerCall(requested, verified)
	request := ts.onlyLineOf(perCall[0], verificationRequestEvent, "a request through recovery")
	ts.Equal("request", request.Action)
	ts.Equal(anonymousRef, request.Actor, "a request on the server's authority names no one, not the token owner")
	ts.Equal(testutils.AuditRef{}, request.OnBehalfOf, "the token owner is not recorded as the principal either")
	ts.Equal(userRef, request.Target)
	ts.Equal(userRef, request.Subject, "a succeeded request names the user")
	ts.Equal(succeededOutcome, request.Outcome)
	ts.Equal(verifiedEmailDetails, ts.details(request), "a succeeded request names the attribute it pinned")
	ts.Zero(countEvent(perCall[0], verificationVerifyEvent), "sending the code verifies nothing")

	verify := ts.onlyLineOf(perCall[1], verificationVerifyEvent, "a verification through recovery")
	ts.Equal("verify", verify.Action)
	ts.Equal(userRef, verify.Actor, "the user who passed the challenge is the actor, not the token owner")
	ts.Equal(testutils.AuditRef{}, verify.OnBehalfOf, "the token owner is not recorded as the principal")
	ts.Equal(userRef, verify.Target)
	ts.Equal(userRef, verify.Subject)
	ts.Equal(succeededOutcome, verify.Outcome)
	ts.Equal(verifiedEmailDetails, ts.details(verify), "a verification names the attribute it recorded")
	ts.Zero(countEvent(perCall[1], verificationRequestEvent), "submitting the code requests nothing")

	ts.requireNoContact(perCall, email, username)
}

// The pin holds the address the code went to, so an address changed before the code is submitted is not
// verified. The failed verification still names the attribute, which the server pinned, but not the user.
func (ts *AuditLogTestSuite) TestAContactChangedBeforeTheCodeIsRecordedAsAFailedVerification() {
	r := ts.newVerificationRecovery("verify-changed")
	userID, username, email := ts.createVerificationUser(r, "verify-changed", true)
	adminToken, err := testutils.GetAccessToken()
	ts.Require().NoError(err)

	_, step, code := ts.requestVerificationCode(r, adminToken, username, email)
	changedEmail := "changed." + email
	changed, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPut, "/users/"+userID,
		ts.verificationUserBody(r, username, changedEmail, "before the code"))
	ts.Require().Equal(http.StatusOK, changed.StatusCode, string(body))
	refused, step := ts.executeRecoveryStep(adminToken, r.appID, map[string]interface{}{
		"executionId": step.ExecutionID, "challengeToken": step.ChallengeToken,
		"inputs": map[string]string{"otp": code}, "action": "action_submit_otp",
	})
	ts.Require().Equal("ERROR", step.FlowStatus, "recording an address the code was not sent to must fail")
	ts.Require().NotNil(step.Error)
	ts.Require().Equal(errCodeVerificationRefused, step.Error.Code)
	ts.Require().NotContains(ts.verifiedAttributes(userID), "email", "the changed address must not be verified")

	perCall := ts.linesPerCall(changed, refused)
	ts.Zero(countEvent(perCall[0], verificationInvalidateEvent), "an unverified address has nothing to invalidate")
	verify := ts.onlyLineOf(perCall[1], verificationVerifyEvent, "a failed verification through recovery")
	ts.Equal("verify", verify.Action)
	ts.Equal(anonymousRef, verify.Actor, "a failed verification names no one, not the token owner")
	ts.Equal(testutils.AuditRef{}, verify.OnBehalfOf)
	ts.Equal(testutils.AuditRef{Type: "user", ID: userID}, verify.Target)
	ts.Equal(testutils.AuditRef{}, verify.Subject, "a failed verification has no subject")
	ts.Equal(notVerifiableOutcome, verify.Outcome)
	ts.Equal(verifiedEmailDetails, ts.details(verify), "the attribute was pinned by the server, so it is named")

	ts.requireNoContact(perCall, email, username)
	ts.requireNoContact(perCall, changedEmail, username)
}

// A recovery for a user without the contact it pins is refused at the request, which names no attribute
// and no user.
func (ts *AuditLogTestSuite) TestARequestForAUserWithoutTheContactNamesNoAttribute() {
	r := ts.newVerificationRecovery("verify-no-contact")
	userID, username, _ := ts.createVerificationUser(r, "verify-no-contact", false)
	adminToken, err := testutils.GetAccessToken()
	ts.Require().NoError(err)

	_, step := ts.executeRecovery(adminToken, r.appID, map[string]interface{}{
		"applicationId": r.appID, "flowType": "RECOVERY",
	})
	refused, step := ts.executeRecoveryStep(adminToken, r.appID, map[string]interface{}{
		"executionId": step.ExecutionID, "challengeToken": step.ChallengeToken,
		"inputs": map[string]string{"username": username}, "action": "action_submit_username",
	})
	ts.Require().Equal("ERROR", step.FlowStatus, "a user without an email must not be sent a code")
	ts.Require().NotNil(step.Error)
	ts.Require().Equal(errCodeVerificationRefused, step.Error.Code)
	ts.Empty(r.smtp.GetEmails(), "nothing may be sent")

	request := ts.onlyLineOf(ts.linesPerCall(refused)[0], verificationRequestEvent, "a refused request")
	ts.Equal(anonymousRef, request.Actor, "a refused request on the server's authority names no one")
	ts.Equal(testutils.AuditRef{Type: "user", ID: userID}, request.Target)
	ts.Equal(testutils.AuditRef{}, request.Subject, "a failed request has no subject")
	ts.Equal(notVerifiableOutcome, request.Outcome)
	ts.Nil(ts.details(request), "a refused request names no attribute")
}

// The administrator-initiated flow takes the attribute as an input, so its caller can type any text there.
// Only a request that resolved the text to a contact names it; a failed or denied one names none. The flow
// acts for its caller, so unlike a recovery its lines name the caller.
func (ts *AuditLogTestSuite) TestAnAttributeTheCallerTypesIsNeverRecorded() {
	r := ts.newVerificationContacts("verify-typed")
	userID, username, email := ts.createVerificationUser(r, "verify-typed", true)
	manager := ts.createScopedManager("verify-typed-scoped", []string{"user"}, []string{"org:user"})
	adminToken, err := testutils.GetAccessToken()
	ts.Require().NoError(err)
	adminHandle := ts.copyAdminVerificationFlow(r, "verify-typed-admin", "")
	managerHandle := ts.copyAdminVerificationFlow(r, "verify-typed-manager", manager.ouID)

	adminRef := testutils.AuditRef{Type: "user", ID: ts.adminUserID()}
	userRef := testutils.AuditRef{Type: "user", ID: userID}
	cases := []struct {
		name      string
		token     string
		handle    string
		attribute string
		status    string
		actor     testutils.AuditRef
		subject   testutils.AuditRef
		outcome   map[string]string
		details   map[string]string
	}{
		{
			name: "the default attribute", token: adminToken, handle: adminHandle, status: "INCOMPLETE",
			actor: adminRef, subject: userRef, outcome: succeededOutcome, details: verifiedEmailDetails,
		},
		{
			name: "a typed attribute that is not a contact", token: adminToken, handle: adminHandle,
			attribute: typedAttributeMarker, status: "ERROR", actor: adminRef, outcome: notVerifiableOutcome,
		},
		{
			name: "a typed attribute of a user outside the caller's unit", token: manager.token,
			handle: managerHandle, attribute: typedAttributeMarker, status: "ERROR",
			actor: testutils.AuditRef{Type: "user", ID: manager.id}, subject: userRef,
			outcome: map[string]string{"result": "denied", "reason": "forbidden"},
		},
	}

	responses := make([]*http.Response, len(cases))
	for i, tc := range cases {
		inputs := map[string]string{"subject": userID}
		if tc.attribute != "" {
			inputs["attribute"] = tc.attribute
		}
		payload, err := json.Marshal(map[string]interface{}{
			"flowType": "ADMIN_INITIATED", "handle": tc.handle, "inputs": inputs,
		})
		ts.Require().NoError(err)
		response, body := ts.post(testutils.GetRawHTTPClient(), "/flow/execute", payload,
			http.Header{"Authorization": {"Bearer " + tc.token}})
		ts.Require().Equal(http.StatusOK, response.StatusCode, "%s: %s", tc.name, string(body))
		var step common.FlowStep
		ts.Require().NoError(json.Unmarshal(body, &step))
		ts.Require().Equal(tc.status, step.FlowStatus, "%s: %s", tc.name, string(body))
		if tc.status == "ERROR" {
			ts.Require().NotNil(step.Error, tc.name)
			ts.Require().Equal(errCodeVerificationRefused, step.Error.Code, tc.name)
		}
		responses[i] = response
	}

	perCall := ts.linesPerCall(responses...)
	for i, tc := range cases {
		line := ts.onlyLineOf(perCall[i], verificationRequestEvent, tc.name)
		ts.Equal("request", line.Action, tc.name)
		ts.Equal(tc.actor, line.Actor, "%s: a flow that acts for its caller names the caller", tc.name)
		ts.Equal(userRef, line.Target, tc.name)
		ts.Equal(tc.subject, line.Subject, tc.name)
		ts.Equal(tc.outcome, line.Outcome, tc.name)
		ts.Equal(tc.details, ts.details(line), "%s: only a resolved attribute is named", tc.name)
		for _, other := range perCall[i] {
			ts.NotContains(other.Raw, typedAttributeMarker, "%s: typed text is never recorded", tc.name)
		}
	}
	ts.requireNoContact(perCall, email, username)
}

// Changing a verified email clears its verification, which the management write records with the caller
// as actor. A write that keeps the email is the control: it clears and records nothing.
func (ts *AuditLogTestSuite) TestChangingAVerifiedEmailRecordsAnInvalidation() {
	r := ts.newVerificationRecovery("verify-invalidate")
	userID, username, email := ts.createVerificationUser(r, "verify-invalidate", true)
	adminToken, err := testutils.GetAccessToken()
	ts.Require().NoError(err)
	_, step, code := ts.requestVerificationCode(r, adminToken, username, email)
	_, step = ts.executeRecovery(adminToken, r.appID, map[string]interface{}{
		"executionId": step.ExecutionID, "challengeToken": step.ChallengeToken,
		"inputs": map[string]string{"otp": code}, "action": "action_submit_otp",
	})
	ts.Require().Equal("COMPLETE", step.FlowStatus, "precondition: the user verifies their email")
	ts.Require().Contains(ts.verifiedAttributes(userID), "email", "precondition: the email is verified")

	kept, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPut, "/users/"+userID,
		ts.verificationUserBody(r, username, email, "renamed"))
	ts.Require().Equal(http.StatusOK, kept.StatusCode, string(body))
	ts.Require().Contains(ts.verifiedAttributes(userID), "email", "a write that keeps the email keeps it verified")
	changedEmail := "changed." + email
	changed, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPut, "/users/"+userID,
		ts.verificationUserBody(r, username, changedEmail, "renamed"))
	ts.Require().Equal(http.StatusOK, changed.StatusCode, string(body))
	ts.Require().NotContains(ts.verifiedAttributes(userID), "email", "changing the email must clear its verification")

	userRef := testutils.AuditRef{Type: "user", ID: userID}
	perCall := ts.linesPerCall(kept, changed)
	ts.Zero(countEvent(perCall[0], verificationInvalidateEvent), "a write that keeps the email invalidates nothing")
	line := ts.onlyLineOf(perCall[1], verificationInvalidateEvent, "a changed verified email")
	ts.Equal("invalidate", line.Action)
	ts.Equal(testutils.AuditRef{Type: "user", ID: ts.adminUserID()}, line.Actor, "the management caller is the actor")
	ts.Equal(userRef, line.Target)
	ts.Equal(userRef, line.Subject)
	ts.Equal(succeededOutcome, line.Outcome)
	ts.Equal(verifiedEmailDetails, ts.details(line), "an invalidation names the attribute, not why it was cleared")
	ts.Equal(1, countEvent(perCall[1], "user.update"), "the write itself is recorded as well")

	ts.requireNoContact(perCall, email, username)
	ts.requireNoContact(perCall, changedEmail, username)
}

// newVerificationContacts creates an organization unit, a user type in it whose email is a contact, and a
// mock SMTP server with a sender, all removed when the test ends.
func (ts *AuditLogTestSuite) newVerificationContacts(name string) verificationRecovery {
	ts.T().Helper()
	r := verificationRecovery{ouID: ts.createOrganizationUnit(name)}
	userType := testutils.UserType{
		Handle: ts.prefix + name + "-person", DisplayName: "Audit Verification Person " + name, OUID: r.ouID,
		Schema: map[string]interface{}{
			"username":    map[string]interface{}{"type": "string"},
			"email":       map[string]interface{}{"type": "email"},
			"displayName": map[string]interface{}{"type": "string"},
		},
	}
	userTypeID, err := testutils.CreateUserType(userType)
	ts.Require().NoError(err, "failed to create the user type whose email is verified")
	ts.T().Cleanup(func() { _ = testutils.DeleteUserType(userTypeID) })
	r.userType = userType.Handle

	r.smtp = testutils.NewMockSMTPServer(0)
	ts.Require().NoError(r.smtp.Start(), "failed to start the mock SMTP server")
	ts.T().Cleanup(func() { _ = r.smtp.Stop() })
	r.senderID, err = testutils.CreateSMTPEmailProvider(ts.prefix+name+"-sender", "localhost", r.smtp.GetPort(),
		"noreply@thunderid.test")
	ts.Require().NoError(err, "failed to create the SMTP email provider")
	ts.T().Cleanup(func() { _ = testutils.DeleteNotificationSender(r.senderID) })
	return r
}

// newVerificationRecovery is newVerificationContacts with a recovery flow that verifies the identified
// user's email and an application that runs it.
func (ts *AuditLogTestSuite) newVerificationRecovery(name string) verificationRecovery {
	ts.T().Helper()
	r := ts.newVerificationContacts(name)
	flowID, err := testutils.CreateFlow(emailCodeVerificationRecoveryFlow(ts.prefix+name+"-flow", r.senderID))
	ts.Require().NoError(err, "failed to create the recovery flow that verifies the email")
	ts.T().Cleanup(func() { _ = testutils.DeleteFlow(flowID) })
	// The default authentication flow names a recovery flow of its own, which the application must match.
	authFlowID, err := testutils.CreateIsolatedAuthFlow(ts.prefix + name + "-auth")
	ts.Require().NoError(err, "failed to create the authentication flow of the application")
	ts.T().Cleanup(func() { _ = testutils.DeleteFlow(authFlowID) })
	r.appID, err = testutils.CreateApplication(testutils.Application{
		Name: ts.prefix + name + "-app", OUID: r.ouID, AuthFlowID: authFlowID,
		IsRecoveryFlowEnabled: true, RecoveryFlowID: flowID,
		ClientID: ts.prefix + name + "-client", ClientSecret: credentialMarker + "ClientSecret1!",
		RedirectURIs: []string{"http://localhost:3000/callback"}, AllowedUserTypes: []string{r.userType},
	})
	ts.Require().NoError(err, "failed to create the application that runs the recovery flow")
	ts.T().Cleanup(func() { _ = testutils.DeleteApplication(r.appID) })
	return r
}

// createVerificationUser creates a user of r's type, with an email when withEmail is set, removed when the
// test ends, and returns its ID, username and email.
func (ts *AuditLogTestSuite) createVerificationUser(r verificationRecovery, name string, withEmail bool) (
	string, string, string) {
	ts.T().Helper()
	username := ts.prefix + name
	attributes := map[string]interface{}{"username": username}
	var email string
	if withEmail {
		email = username + "@example.com"
		attributes["email"] = email
	}
	encoded, err := json.Marshal(attributes)
	ts.Require().NoError(err)
	userID, err := testutils.CreateUser(testutils.User{OUID: r.ouID, Type: r.userType, Attributes: encoded})
	ts.Require().NoError(err, "failed to create the user whose email is verified")
	ts.T().Cleanup(func() { _ = testutils.DeleteUser(userID) })
	return userID, username, email
}

// verificationUserBody is the body of a management update of a user of r's type.
func (ts *AuditLogTestSuite) verificationUserBody(r verificationRecovery, username, email,
	displayName string) string {
	ts.T().Helper()
	body, err := json.Marshal(map[string]interface{}{
		"ouId": r.ouID, "type": r.userType,
		"attributes": map[string]string{"username": username, "email": email, "displayName": displayName},
	})
	ts.Require().NoError(err)
	return string(body)
}

// requestVerificationCode starts r's recovery and submits the username with the token, and returns the
// response of the submission, the step that asks for the code and the code emailed to email.
func (ts *AuditLogTestSuite) requestVerificationCode(r verificationRecovery, token, username, email string) (
	*http.Response, *common.FlowStep, string) {
	ts.T().Helper()
	r.smtp.ClearEmails()
	_, step := ts.executeRecovery(token, r.appID, map[string]interface{}{
		"applicationId": r.appID, "flowType": "RECOVERY",
	})
	requested, step := ts.executeRecovery(token, r.appID, map[string]interface{}{
		"executionId": step.ExecutionID, "challengeToken": step.ChallengeToken,
		"inputs": map[string]string{"username": username}, "action": "action_submit_username",
	})
	ts.Require().True(common.HasInput(step.Data.Inputs, "otp"), "the flow must ask for the emailed code")
	var message *testutils.EmailMessage
	ts.Require().Eventually(func() bool {
		message = r.smtp.GetLastEmail()
		return message != nil
	}, 10*time.Second, 100*time.Millisecond, "the code was not emailed")
	ts.Require().Equal([]string{email}, message.To, "the code must go to the stored address")
	code := message.ExtractOTP()
	ts.Require().NotEmpty(code, "the email carries no code")
	return requested, step, code
}

// copyAdminVerificationFlow copies the shipped administrator verification flow into ouID, or the default
// unit when it is empty, sending through r's sender, and returns the copy's handle.
func (ts *AuditLogTestSuite) copyAdminVerificationFlow(r verificationRecovery, name, ouID string) string {
	ts.T().Helper()
	handle := ts.prefix + name
	flowID, err := testutils.CopyFlowWithSendersInOU(adminVerificationFlowHandle, "ADMIN_INITIATED", handle, ouID,
		map[string]string{adminVerificationSendNode: r.senderID}, nil)
	ts.Require().NoError(err, "failed to copy the shipped administrator verification flow")
	ts.T().Cleanup(func() { _ = testutils.DeleteFlow(flowID) })
	return handle
}

// verifiedAttributes returns the attributes a management read of the user reports as verified.
func (ts *AuditLogTestSuite) verifiedAttributes(userID string) map[string]testutils.VerifiedAttribute {
	ts.T().Helper()
	resp, body := ts.exchange(testutils.GetHTTPClient(), http.MethodGet, "/users/"+userID, "")
	ts.Require().Equal(http.StatusOK, resp.StatusCode, string(body))
	var read struct {
		VerifiedAttributes map[string]testutils.VerifiedAttribute `json:"verifiedAttributes"`
	}
	ts.Require().NoError(json.Unmarshal(body, &read))
	return read.VerifiedAttributes
}

// requireNoContact checks that no line the calls wrote carries the contact value or the username.
func (ts *AuditLogTestSuite) requireNoContact(perCall [][]testutils.AuditLine, email, username string) {
	ts.T().Helper()
	for _, written := range perCall {
		for _, line := range written {
			ts.NotContains(line.Raw, email, "%s: a contact value is never recorded", line.Event)
			ts.NotContains(line.Raw, username, "%s: no identifier of the user is recorded", line.Event)
		}
	}
}

// emailCodeVerificationRecoveryFlow is a recovery flow that identifies the user by username, pins their
// email, emails a code to it and records the email as verified once the code is submitted.
func emailCodeVerificationRecoveryFlow(handle, senderID string) testutils.Flow {
	return testutils.Flow{
		Name: "Audit Email Verification Recovery " + handle, Handle: handle, FlowType: "RECOVERY",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "prompt_username"},
			{"id": "prompt_username", "type": "PROMPT", "prompts": []map[string]interface{}{{
				"inputs": []map[string]interface{}{
					{"ref": "input_username", "identifier": "username", "type": "TEXT_INPUT", "required": true},
				},
				"action": map[string]interface{}{"ref": "action_submit_username", "nextNode": "identify_user"},
			}}},
			{"id": "identify_user", "type": "TASK_EXECUTION", "executor": map[string]interface{}{
				"name": "IdentifyingExecutor", "mode": "identify", "inputs": []map[string]interface{}{
					{"ref": "input_username", "identifier": "username", "type": "TEXT_INPUT", "required": true},
				},
			}, "onSuccess": "prepare_verification"},
			{"id": "prepare_verification", "type": "TASK_EXECUTION", "executor": map[string]interface{}{
				"name": "AttributeVerificationExecutor", "mode": "prepare", "inputs": []map[string]interface{}{
					{"identifier": "attribute", "type": "TEXT_INPUT", "required": true, "defaultValue": "email",
						"constant": true},
				},
			}, "onSuccess": "generate_otp"},
			{"id": "generate_otp", "type": "TASK_EXECUTION", "executor": map[string]interface{}{
				"name": "OTPExecutor", "mode": "generate", "inputs": []map[string]interface{}{verificationEmailInput},
			}, "onSuccess": "send_otp"},
			{"id": "send_otp", "type": "TASK_EXECUTION",
				"properties": map[string]interface{}{"senderId": senderID, "emailTemplate": "otp"},
				"executor": map[string]interface{}{"name": "EmailExecutor", "mode": "send",
					"inputs": []map[string]interface{}{verificationEmailInput}},
				"onSuccess": "prompt_otp"},
			{"id": "prompt_otp", "type": "PROMPT", "prompts": []map[string]interface{}{{
				"inputs": []map[string]interface{}{
					{"ref": "input_otp", "identifier": "otp", "type": "OTP_INPUT", "required": true},
				},
				"action": map[string]interface{}{"ref": "action_submit_otp", "nextNode": "verify_otp"},
			}}},
			{"id": "verify_otp", "type": "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "OTPExecutor", "mode": "verify"},
				"onSuccess": "record_verification"},
			{"id": "record_verification", "type": "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "AttributeVerificationExecutor", "mode": "record"},
				"onSuccess": "end"},
			{"id": "end", "type": "END"},
		},
	}
}
