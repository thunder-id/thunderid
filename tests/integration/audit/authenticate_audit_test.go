// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package audit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	authenticatePassword = "AuditAuthenticate123!"
	authenticateWrong    = "AuditAuthenticateWrong456!"
	otpAction            = "action_otp"
	accessSection        = "accountAccess"
	lockThreshold        = 3
)

var anonymousRef = testutils.AuditRef{Type: "anonymous"}

func (ts *AuditLogTestSuite) TestAPasswordSignInWritesOneAuthenticateLine() {
	f := ts.createSessionFixture("authn-success")
	adminRef := testutils.AuditRef{Type: "user", ID: ts.adminUserID()}
	userRef := testutils.AuditRef{Type: "user", ID: f.userID}

	prompted, submitted, signedIn := ts.signIn(f)

	written := ts.linesPerCall(prompted, submitted)
	ts.Zero(countEvent(written[0], "user.authenticate"), "presenting the Flow Secret is not a sign-in")
	line := ts.onlyLineOf(written[1], "user.authenticate", "a password sign-in")
	ts.Equal(append(append([]string{}, lineKeys...), "application", "ou", "flow_ref", "details"), ts.keys(line.Raw))
	ts.Equal("authenticate", line.Action)
	ts.Equal(userRef, line.Actor, "the user who signed in is the actor, not the owner of the bearer token")
	ts.NotEqual(adminRef, line.Actor)
	ts.Equal(userRef, line.Subject)
	ts.Equal(userRef, line.Target)
	ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome)
	ts.Equal(map[string]string{"id": f.signInAppID}, line.Application)
	ts.Equal(map[string]string{"id": f.ouID}, line.OU)
	ts.Equal(map[string]string{"authentication_method": "credential"}, ts.details(line))
	ts.NotEmpty(line.FlowRef)
	ts.NotEqual(signedIn.ExecutionID, line.FlowRef, "the flow reference is not the execution ID")
	session := ts.onlyLineOf(written[1], "session.create", "a password sign-in")
	ts.Equal(line.FlowRef, session.FlowRef, "the credential check and the session belong to one flow")
	for _, other := range written[1] {
		ts.NotContains(other.Raw, sessionPassword, "the password is never recorded")
		ts.NotContains(other.Raw, signedIn.ExecutionID, "the execution ID is never recorded")
	}
}

func (ts *AuditLogTestSuite) TestAWrongPasswordWritesOneFailedLineThatNamesNoAccount() {
	f := ts.createSessionFixture("authn-failure")
	unknown := ts.prefix + "authn-unknown"

	cases := []struct {
		name     string
		username string
		reason   string
	}{
		{"a wrong password", f.username, "invalid_credentials"},
		{"an unknown user", unknown, "user_not_found"},
	}
	responses := make([]*http.Response, len(cases))
	for i, tc := range cases {
		_, step := ts.executeSessionStep(f.signInAppID, map[string]interface{}{
			"applicationId": f.signInAppID, "flowType": "AUTHENTICATION",
		})
		responses[i], _ = ts.submitCredentials(f, step, tc.username, authenticateWrong)
	}

	written := ts.linesPerCall(responses...)
	for i, tc := range cases {
		line := ts.onlyLineOf(written[i], "user.authenticate", tc.name)
		ts.Equal([]string{
			"log_type", "schema_version", "id", "time", "event", "trace_id", "actor", "target", "action", "outcome",
			"application", "flow_ref", "details",
		}, ts.keys(line.Raw), tc.name)
		ts.Equal(anonymousRef, line.Actor, "%s: the bearer token the request carries is not the actor", tc.name)
		ts.Equal(testutils.AuditRef{}, line.Subject, "%s: a failed check names no account here", tc.name)
		ts.Equal(testutils.AuditRef{Type: "user"}, line.Target, tc.name)
		ts.Equal(map[string]string{"result": "failed", "reason": tc.reason}, line.Outcome, tc.name)
		ts.Equal(map[string]string{"id": f.signInAppID}, line.Application, tc.name)
		ts.Equal(map[string]string{"authentication_method": "credential"}, ts.details(line), tc.name)
		ts.Nil(line.OU, "%s: a failed check names no account, so no unit", tc.name)
		ts.Zero(countEvent(written[i], "session.create"), "%s: no session starts", tc.name)
		for _, other := range written[i] {
			ts.NotContains(other.Raw, authenticateWrong, "%s: the typed password is never recorded", tc.name)
			ts.NotContains(other.Raw, tc.username, "%s: the typed username is never recorded", tc.name)
		}
	}
}

func (ts *AuditLogTestSuite) TestAttemptsAtALockedAccountAreDenied() {
	ts.applyLockoutThreshold(lockThreshold)
	f := ts.createSessionFixture("authn-locked")
	_, step := ts.executeSessionStep(f.signInAppID, map[string]interface{}{
		"applicationId": f.signInAppID, "flowType": "AUTHENTICATION",
	})
	responses := make([]*http.Response, 0, lockThreshold+1)
	for range lockThreshold {
		var response *http.Response
		response, step = ts.submitCredentials(f, step, f.username, authenticateWrong)
		responses = append(responses, response)
	}
	locked, _ := ts.submitCredentials(f, step, f.username, sessionPassword)
	responses = append(responses, locked)

	written := ts.linesPerCall(responses...)
	for i := range lockThreshold + 1 {
		line := ts.onlyLineOf(written[i], "user.authenticate", fmt.Sprintf("attempt %d", i+1))
		ts.Equal(anonymousRef, line.Actor)
		ts.Equal(testutils.AuditRef{}, line.Subject)
		ts.Equal(testutils.AuditRef{Type: "user"}, line.Target)
		ts.Equal(map[string]string{"authentication_method": "credential"}, ts.details(line))
		ts.Zero(countEvent(written[i], "session.create"))
		expected := map[string]string{"result": "failed", "reason": "invalid_credentials"}
		if i >= lockThreshold {
			expected = map[string]string{"result": "denied", "reason": "account_locked"}
		}
		ts.Equal(expected, line.Outcome, "attempt %d", i+1)
	}
}

func (ts *AuditLogTestSuite) TestTheCredentialsAPICheckIsRecordedWithoutTheBearerTokenAsActor() {
	ouID, userType := ts.createUserType("authn-api")
	username := ts.prefix + "authn-api"
	userID := ts.createUser(ouID, userType, username, authenticatePassword)
	userRef := testutils.AuditRef{Type: "user", ID: userID}
	adminRef := testutils.AuditRef{Type: "user", ID: ts.adminUserID()}
	token, err := testutils.GetAccessToken()
	ts.Require().NoError(err)
	check := func(password string) *http.Response {
		payload, err := json.Marshal(map[string]interface{}{
			"identifiers": map[string]string{"username": username},
			"credentials": map[string]string{"password": password},
		})
		ts.Require().NoError(err)
		response, body := ts.post(testutils.GetRawHTTPClient(), "/auth/credentials/authenticate", payload, http.Header{
			"Authorization":                {"Bearer " + token},
			testutils.DirectAuthHeaderName: {testutils.DirectAuthHeaderValue},
		})
		ts.Require().Contains([]int{http.StatusOK, http.StatusUnauthorized}, response.StatusCode, string(body))
		return response
	}

	right, wrong := check(authenticatePassword), check(authenticateWrong)

	written := ts.linesPerCall(right, wrong)
	succeeded := ts.onlyLineOf(written[0], "user.authenticate", "a right password")
	ts.Equal(userRef, succeeded.Actor)
	ts.NotEqual(adminRef, succeeded.Actor)
	ts.Equal(userRef, succeeded.Subject)
	ts.Equal(map[string]string{"result": "succeeded"}, succeeded.Outcome)
	ts.Equal(map[string]string{"id": ouID}, succeeded.OU)
	ts.Nil(succeeded.Application, "a credentials API check runs in no flow")
	ts.Empty(succeeded.FlowRef)
	failed := ts.onlyLineOf(written[1], "user.authenticate", "a wrong password")
	ts.Equal(anonymousRef, failed.Actor)
	ts.NotEqual(adminRef, failed.Actor)
	ts.Equal(testutils.AuditRef{}, failed.Subject)
	ts.Equal(map[string]string{"result": "failed", "reason": "invalid_credentials"}, failed.Outcome)
	for _, line := range []testutils.AuditLine{succeeded, failed} {
		ts.Equal(map[string]string{"authentication_method": "credential"}, ts.details(line))
		ts.NotContains(line.Raw, authenticatePassword)
		ts.NotContains(line.Raw, authenticateWrong)
	}
}

// The flow context is stored between the two requests, so one flow_ref across them shows it survives a request boundary.
func (ts *AuditLogTestSuite) TestATwoFactorSignInSharesOneFlowRefAcrossItsRequests() {
	ouID := ts.createOrganizationUnit("authn-mfa")
	mobile := fmt.Sprintf("+1555%07d", time.Now().UnixNano()%10000000)
	userType := testutils.UserType{
		Handle: ts.prefix + "authn-mfa-person", DisplayName: "Audit MFA Person", OUID: ouID,
		Schema: map[string]interface{}{
			"username":      map[string]interface{}{"type": "string"},
			"password":      map[string]interface{}{"type": "string", "credential": true},
			"mobile_number": map[string]interface{}{"type": "string"},
		},
	}
	userTypeID, err := testutils.CreateUserType(userType)
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteUserType(userTypeID) })
	username := ts.prefix + "authn-mfa"
	attributes, err := json.Marshal(map[string]interface{}{
		"username": username, "password": authenticatePassword, "mobile_number": mobile,
	})
	ts.Require().NoError(err)
	userID, err := testutils.CreateUser(testutils.User{OUID: ouID, Type: userType.Handle, Attributes: attributes})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteUser(userID) })

	sms, err := testutils.NewMockNotificationServer()
	ts.Require().NoError(err, "failed to start the mock SMS provider")
	ts.T().Cleanup(func() { _ = sms.Stop() })
	senderID, err := testutils.CreateNotificationSender(testutils.NotificationSender{
		Name: "Audit MFA SMS sender " + ts.prefix, Provider: "custom",
		Properties: []testutils.SenderProperty{
			{Name: "url", Value: sms.GetSendSMSURL()},
			{Name: "http_method", Value: "POST"},
			{Name: "content_type", Value: "JSON"},
		},
	})
	ts.Require().NoError(err, "failed to create the SMS sender")
	ts.T().Cleanup(func() { _ = testutils.DeleteNotificationSender(senderID) })
	flowID, err := testutils.CreateFlow(twoFactorFlow(ts.prefix+"authn-mfa-flow", senderID))
	ts.Require().NoError(err, "failed to create the two-factor flow")
	ts.T().Cleanup(func() { _ = testutils.DeleteFlow(flowID) })
	appID, err := testutils.CreateApplication(testutils.Application{
		Name: ts.prefix + "authn-mfa-app", OUID: ouID, AuthFlowID: flowID, AllowedUserTypes: []string{userType.Handle},
	})
	ts.Require().NoError(err, "failed to create the two-factor application")
	ts.T().Cleanup(func() { _ = testutils.DeleteApplication(appID) })
	userRef := testutils.AuditRef{Type: "user", ID: userID}

	_, step := ts.executeSessionStep(appID, map[string]interface{}{
		"applicationId": appID, "flowType": "AUTHENTICATION",
	})
	passed, step := ts.executeSessionStep(appID, map[string]interface{}{
		"executionId": step.ExecutionID, "challengeToken": step.ChallengeToken,
		"inputs": map[string]string{"username": username, "password": authenticatePassword},
		"action": sessionCredentialsAction,
	})
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "the password must pass and prompt for the passcode")
	var code string
	ts.Require().Eventually(func() bool {
		message := sms.GetLastMessage()
		if message == nil {
			return false
		}
		code = message.OTP
		return code != ""
	}, 10*time.Second, 100*time.Millisecond, "the passcode was not sent")
	executionID := step.ExecutionID
	completed, step := ts.executeSessionStep(appID, map[string]interface{}{
		"executionId": step.ExecutionID, "challengeToken": step.ChallengeToken,
		"inputs": map[string]string{"otp": code}, "action": otpAction,
	})
	ts.Require().Equal("COMPLETE", step.FlowStatus, "the passcode must complete the sign-in")

	written := ts.linesPerCall(passed, completed)
	first := ts.onlyLineOf(written[0], "user.authenticate", "the password")
	second := ts.onlyLineOf(written[1], "user.authenticate", "the passcode")
	session := ts.onlyLineOf(written[1], "session.create", "the passcode")
	ts.Equal(map[string]string{"authentication_method": "credential"}, ts.details(first))
	ts.Equal(map[string]string{"authentication_method": "otp"}, ts.details(second))
	for _, line := range []testutils.AuditLine{first, second} {
		ts.Equal(userRef, line.Actor)
		ts.Equal(userRef, line.Subject)
		ts.Equal(map[string]string{"result": "succeeded"}, line.Outcome)
		ts.Equal(map[string]string{"id": appID}, line.Application)
		ts.Equal(map[string]string{"id": ouID}, line.OU)
		ts.requireNoStringValue(line, code, "the passcode is never recorded")
		ts.NotContains(line.Raw, executionID, "the execution ID is never recorded")
	}
	ts.NotEmpty(first.FlowRef)
	ts.Equal(first.FlowRef, second.FlowRef, "both checks belong to one login")
	ts.Equal(first.FlowRef, session.FlowRef, "the session belongs to the same login")
	ts.NotEqual(first.TraceID, second.TraceID, "the two checks were made by different requests")
	ts.NotEqual(executionID, first.FlowRef, "the flow reference is not the execution ID")
}

// requireNoStringValue checks that no field of the line, at any depth, holds value as a complete string. A
// numeric value such as a passcode is not looked for inside the line's text, where it can match the digits of a
// timestamp or an identifier without having been recorded.
func (ts *AuditLogTestSuite) requireNoStringValue(line testutils.AuditLine, value, message string) {
	ts.T().Helper()
	var decoded interface{}
	ts.Require().NoError(json.Unmarshal([]byte(line.Raw), &decoded))
	var walk func(node interface{})
	walk = func(node interface{}) {
		switch typed := node.(type) {
		case string:
			ts.NotEqual(value, typed, message)
		case map[string]interface{}:
			for _, child := range typed {
				walk(child)
			}
		case []interface{}:
			for _, child := range typed {
				walk(child)
			}
		}
	}
	walk(decoded)
}

func (ts *AuditLogTestSuite) TestAMachineCredentialWritesNoAuthenticateLine() {
	client := ts.createClient("authn-machine", auditClientSecret, clientCredentialsGrant)

	issued, body := ts.oauthCall("/oauth2/token", url.Values{"grant_type": {clientCredentialsGrant}}, client, nil)
	ts.Require().Equal(http.StatusOK, issued.StatusCode, string(body))

	written := ts.linesPerCall(issued)[0]
	ts.Equal(1, countEvent(written, "token.issue"))
	ts.Zero(countEvent(written, "user.authenticate"), "a client secret check is not a sign-in")
}

// submitCredentials submits a username and a password at the credential prompt of the session sign-in flow
// that step is at, with the admin's bearer token, and returns the response and the step after it.
func (ts *AuditLogTestSuite) submitCredentials(f sessionFixture, step *testutils.FlowStep, username,
	password string) (*http.Response, *testutils.FlowStep) {
	ts.T().Helper()
	return ts.executeSessionStep(f.signInAppID, map[string]interface{}{
		"executionId": step.ExecutionID, "challengeToken": step.ChallengeToken,
		"inputs": map[string]string{"username": username, "password": password},
		"action": sessionCredentialsAction,
	})
}

// twoFactorFlow is a sign-in flow that checks the password, sends a passcode by SMS through senderID, checks
// it and starts a session.
func twoFactorFlow(handle, senderID string) testutils.Flow {
	credentials := []map[string]interface{}{
		{"ref": "input_username", "identifier": "username", "type": "TEXT_INPUT", "required": true},
		{"ref": "input_password", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
	}
	passcode := []map[string]interface{}{
		{"ref": "input_otp", "identifier": "otp", "type": "OTP_INPUT", "required": true},
	}
	return testutils.Flow{
		Name: "Audit Two Factor Sign In " + handle, Handle: handle, FlowType: "AUTHENTICATION",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "sso_check"},
			{"id": "sso_check", "type": "TASK_EXECUTION",
				"executor":   map[string]interface{}{"name": "SSOCheckExecutor"},
				"properties": map[string]interface{}{"checkpointRef": "session_main"},
				"onSuccess":  "session_main", "onFailure": "prompt_credentials"},
			{"id": "prompt_credentials", "type": "PROMPT", "prompts": []map[string]interface{}{{
				"inputs": credentials,
				"action": map[string]interface{}{"ref": sessionCredentialsAction, "nextNode": "credentials_auth"},
			}}},
			{"id": "credentials_auth", "type": "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "CredentialsAuthExecutor", "inputs": credentials},
				"onSuccess": "generate_otp", "onIncomplete": "prompt_credentials"},
			{"id": "generate_otp", "type": "TASK_EXECUTION",
				"executor": map[string]interface{}{"name": "OTPExecutor", "mode": "generate",
					"inputs": []map[string]interface{}{
						{"ref": "input_mobile", "identifier": "mobile_number", "type": "PHONE_INPUT", "required": true},
					}},
				"onSuccess": "sms_send"},
			{"id": "sms_send", "type": "TASK_EXECUTION",
				"properties": map[string]interface{}{"senderId": senderID, "smsTemplate": "otp"},
				"executor":   map[string]interface{}{"name": "SMSExecutor"}, "onSuccess": "prompt_otp"},
			{"id": "prompt_otp", "type": "PROMPT", "prompts": []map[string]interface{}{{
				"inputs": passcode,
				"action": map[string]interface{}{"ref": otpAction, "nextNode": "verify_otp"},
			}}},
			{"id": "verify_otp", "type": "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "OTPExecutor", "mode": "verify", "inputs": passcode},
				"onSuccess": "session_main", "onIncomplete": "prompt_otp"},
			{"id": "session_main", "type": "TASK_EXECUTION",
				"executor": map[string]interface{}{"name": "SessionExecutor"}, "onSuccess": "auth_assert"},
			{"id": "auth_assert", "type": "TASK_EXECUTION",
				"executor": map[string]interface{}{"name": "AuthAssertExecutor"}, "onSuccess": "end"},
			{"id": "end", "type": "END"},
		},
	}
}

// applyLockoutThreshold sets the user lock threshold through the writable accountAccess section, which applies
// without a restart, and puts the section back when the test ends.
func (ts *AuditLogTestSuite) applyLockoutThreshold(threshold int) {
	ts.T().Helper()
	original, err := testutils.GetWritableServerConfig(accessSection)
	ts.Require().NoError(err, "failed to read the accountAccess section")
	ts.T().Cleanup(func() {
		ts.NoError(testutils.PutWritableServerConfig(accessSection, original), "failed to restore the accountAccess section")
	})
	var layer map[string]interface{}
	if len(original) > 0 {
		ts.Require().NoError(json.Unmarshal(original, &layer))
	}
	if layer == nil {
		layer = map[string]interface{}{}
	}
	user, _ := layer["user"].(map[string]interface{})
	if user == nil {
		user = map[string]interface{}{}
	}
	defaults, _ := user["default"].(map[string]interface{})
	if defaults == nil {
		defaults = map[string]interface{}{}
	}
	defaults["enabled"], defaults["threshold"] = true, threshold
	defaults["lockDurationsSeconds"] = []int{300}
	user["default"], layer["user"] = defaults, user
	body, err := json.Marshal(layer)
	ts.Require().NoError(err)
	ts.Require().NoError(testutils.PutWritableServerConfig(accessSection, body), "failed to apply the lockout policy")
}
