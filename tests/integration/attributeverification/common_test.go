// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package attributeverification checks email and mobile verification through the self-service
// verification flows, and the verification status the user API reports, against a running server.
package attributeverification

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const testServerURL = testutils.TestServerURL

// The shipped self attribute verification flow, its send nodes, and the actions of its code prompts.
// The email node ships without a sender and the SMS node with a placeholder, so each suite runs a copy
// that names its own.
const (
	selfServiceFlowType    = "SELF_SERVICE"
	verificationFlowHandle = "default-self-attribute-verification-flow"
	emailSendNode          = "send_email_otp"
	mobileSendNode         = "send_mobile_otp"

	actionEmailVerify  = "action_email_verify"
	actionEmailResend  = "action_email_resend"
	actionMobileVerify = "action_mobile_verify"
	actionMobileResend = "action_mobile_resend"
	actionConfirm      = "action_confirm"

	// senderPlaceholder is the senderId the shipped SMS node carries until an operator selects a sender.
	senderPlaceholder = "{{SENDER_ID}}"
)

// userSuspendFlowHandle names the shipped administration flow that suspends a user.
const userSuspendFlowHandle = "default-user-suspend-flow"

const (
	emailAttribute  = "email"
	mobileAttribute = "mobile_number"

	flowStatusIncomplete = "INCOMPLETE"
	flowStatusComplete   = "COMPLETE"
	flowStatusError      = "ERROR"
)

// The error codes the tests expect.
const (
	// errCodeInvalidOTP is the flow's answer to a wrong code.
	errCodeInvalidOTP = "FET-1008"
	// errCodeSMSProviderNotConfigured is the SMS step's refusal when its sender has no provider.
	errCodeSMSProviderNotConfigured = "FET-1042"
	// errCodeVerificationRefused is governance's refusal to prepare or record a verification.
	errCodeVerificationRefused = "FET-1112"
	// errCodeVerificationUnavailable is the refusal of a contact or a flow that cannot verify here.
	errCodeVerificationUnavailable = "FET-1119"
	// errCodeExecutionNotFound is the flow's answer to an execution that ended or never existed.
	errCodeExecutionNotFound = "FES-1004"
	// errCodeFlowHandleRequired is the refusal of a self-service start that names no flow.
	errCodeFlowHandleRequired = "FES-1023"
	// errCodeAuthenticationRequired is the refusal of a start whose caller is not the one the flow
	// requires.
	errCodeAuthenticationRequired = "FES-1017"
	// errCodeFlowIDNotPermitted is the flow ID entry point's refusal of a flow it may not start.
	errCodeFlowIDNotPermitted = "FES-1018"
	// errCodeFlowNotFound is flow management's answer to a flow that does not exist.
	errCodeFlowNotFound = "FLM-1003"
	// errCodeInvalidConfigValue is the server configuration API's refusal of a value its section rejects.
	errCodeInvalidConfigValue = "SCF-1003"
)

// deliveryWait bounds the wait for an email or SMS the server sends during a request. A send
// completes before the flow answers the request, so a sent message is normally there at once.
const deliveryWait = 5 * time.Second

// The declarative user the integration resources ship: a Person, whose email is a contact. It is read
// only, never changed.
var declarativeUser = testUser{id: "decl-user-1", username: "decl-user-1", password: "TempPassword123!",
	email: "decl-user-1@example.com"}

// The server-config section and the key that sets the expiry of every self-service flow.
const (
	flowConfigSection  = "flow"
	selfServiceFlowKey = "selfServiceFlow"
)

// verificationUserTypeSchema has two eligible contacts, email and mobile_number, and an attribute no
// verification depends on.
func verificationUserTypeSchema() map[string]interface{} {
	return map[string]interface{}{
		"username":      map[string]interface{}{"type": "string", "required": true, "unique": true},
		"password":      map[string]interface{}{"type": "string", "credential": true},
		"email":         map[string]interface{}{"type": "email"},
		"mobile_number": map[string]interface{}{"type": "mobile"},
		"displayName":   map[string]interface{}{"type": "string"},
	}
}

// verificationFixture is an organization unit and user type for the suite's users, a mock SMTP
// server, and a copy of the shipped verification flow whose email node sends through it.
type verificationFixture struct {
	name       string
	smtp       *testutils.MockSMTPServer
	senderID   string
	flowHandle string
	senders    map[string]string
	ouID       string
	userTypeID string
	userType   string
	userIDs    []string
	suffix     string
	mobileStem int64
	mobiles    int
}

// testUser is an account a test created, with what it signs in with.
type testUser struct {
	id       string
	username string
	password string
	email    string
	mobile   string
}

// newVerificationFixture stands the fixture up and removes it when t ends, a failed setup included.
// name keeps the suite's resources apart from other suites'.
func newVerificationFixture(t *testing.T, name string) *verificationFixture {
	t.Helper()

	now := time.Now().UnixNano()
	f := &verificationFixture{name: name, suffix: fmt.Sprintf("%d", now), mobileStem: now % 10000,
		senders: map[string]string{}}

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "verification-" + name + "-ou",
		Name:        "Attribute Verification " + name + " OU",
		Description: "Organization unit for the attribute verification tests",
	})
	require.NoError(t, err, "failed to create the organization unit")
	f.ouID = ouID
	t.Cleanup(func() {
		if err := testutils.DeleteOrganizationUnit(ouID); err != nil {
			t.Logf("cleanup: failed to delete the organization unit: %v", err)
		}
	})

	f.userType = "verification_" + name + "_user"
	f.userTypeID, err = testutils.CreateUserType(testutils.UserType{
		Handle:      f.userType,
		DisplayName: "Attribute Verification " + name + " User",
		OUID:        ouID,
		Schema:      verificationUserTypeSchema(),
	})
	require.NoError(t, err, "failed to create the user type")
	t.Cleanup(func() {
		if err := testutils.DeleteUserType(f.userTypeID); err != nil {
			t.Logf("cleanup: failed to delete the user type: %v", err)
		}
	})
	// Users go before the type they belong to.
	t.Cleanup(func() { f.deleteUsers(t) })

	f.smtp = testutils.NewMockSMTPServer(0)
	require.NoError(t, f.smtp.Start(), "failed to start the mock SMTP server")
	t.Cleanup(func() { _ = f.smtp.Stop() })
	f.senderID, err = testutils.CreateSMTPEmailProvider("Attribute Verification "+name+" Provider",
		"localhost", f.smtp.GetPort(), "noreply@thunderid.test")
	require.NoError(t, err, "failed to create the SMTP email provider")
	t.Cleanup(func() {
		if err := testutils.DeleteNotificationSender(f.senderID); err != nil {
			t.Logf("cleanup: failed to delete the sender: %v", err)
		}
	})

	f.nameSender(t, emailSendNode, f.senderID)
	return f
}

// nameSender names senderID on one send node of the fixture's copy of the shipped verification flow,
// with the senders named before it, by making a new copy the fixture starts from then on.
func (f *verificationFixture) nameSender(t *testing.T, nodeID, senderID string) {
	t.Helper()

	f.senders[nodeID] = senderID
	f.flowHandle = copyShippedFlow(t, fmt.Sprintf("%s-%d-%s", f.name, len(f.senders), f.suffix), f.senders)
}

// copyShippedFlow creates a copy of the shipped verification flow under a handle made from name, with
// the senders named on their nodes, and deletes it when t ends. Cleanups run newest first, so the copy
// is deleted before the senders it names. It returns the copy's handle.
func copyShippedFlow(t *testing.T, name string, senders map[string]string) string {
	t.Helper()

	handle := "verification-" + strings.ReplaceAll(name, "_", "-")
	flowID, err := testutils.CopyFlowWithSenders(verificationFlowHandle, selfServiceFlowType, handle, senders, nil)
	require.NoError(t, err, "failed to copy the shipped verification flow")
	t.Cleanup(func() {
		if err := testutils.DeleteFlow(flowID); err != nil {
			t.Errorf("cleanup: failed to delete flow %s: %v", flowID, err)
		}
	})
	return handle
}

// deleteUsers deletes the users the fixture created or was given, and forgets them.
func (f *verificationFixture) deleteUsers(t *testing.T) {
	for _, userID := range f.userIDs {
		if err := testutils.DeleteUser(userID); err != nil {
			t.Logf("cleanup: failed to delete user %s: %v", userID, err)
		}
	}
	f.userIDs = nil
}

// createApp creates an application and deletes it when t ends.
func createApp(t *testing.T, app testutils.Application) string {
	t.Helper()

	appID, err := testutils.CreateApplication(app)
	require.NoError(t, err, "failed to create the %s application", app.Name)
	t.Cleanup(func() {
		if err := testutils.DeleteApplication(appID); err != nil {
			t.Errorf("cleanup: failed to delete application %s: %v", appID, err)
		}
	})
	return appID
}

// createFlow creates a flow and deletes it when t ends.
func createFlow(t *testing.T, flow testutils.Flow) string {
	t.Helper()

	flowID, err := testutils.CreateFlow(flow)
	require.NoError(t, err, "the flow management API must accept %s", flow.Handle)
	t.Cleanup(func() {
		if err := testutils.DeleteFlow(flowID); err != nil {
			t.Errorf("cleanup: failed to delete flow %s: %v", flowID, err)
		}
	})
	return flowID
}

// newUser creates a user with an unverified email and an unverified E.164 mobile number that no other
// test touches.
func (f *verificationFixture) newUser(t *testing.T, name string) testUser {
	t.Helper()

	return f.newUserOfType(t, name, f.userType)
}

// newUserOfType creates a user of userType as newUser does.
func (f *verificationFixture) newUserOfType(t *testing.T, name, userType string) testUser {
	t.Helper()

	u := testUser{
		username: fmt.Sprintf("verify_%s_%s", name, f.suffix),
		password: "Correct-Horse-" + name + "1!",
		email:    fmt.Sprintf("%s.%s@example.com", name, f.suffix),
		mobile:   f.uniqueMobile(),
	}
	attributes, err := json.Marshal(map[string]interface{}{
		"username":      u.username,
		"password":      u.password,
		"email":         u.email,
		"mobile_number": u.mobile,
	})
	require.NoError(t, err)

	u.id, err = testutils.CreateUser(testutils.User{OUID: f.ouID, Type: userType, Attributes: attributes})
	require.NoError(t, err, "failed to create the test user")
	f.userIDs = append(f.userIDs, u.id)
	return u
}

// newUserType creates a user type with schema in the fixture's organization unit for a test that
// changes it, and returns its ID and handle. It is deleted when t ends, after the fixture's users, so a
// user of the type does not keep it in place.
func (f *verificationFixture) newUserType(t *testing.T, name string,
	schema map[string]interface{}) (string, string) {
	t.Helper()

	handle := "verification_" + name + "_user"
	typeID, err := testutils.CreateUserType(testutils.UserType{
		Handle:      handle,
		DisplayName: "Attribute Verification " + name + " User",
		OUID:        f.ouID,
		Schema:      schema,
	})
	require.NoError(t, err, "failed to create the %s user type", name)
	t.Cleanup(func() {
		f.deleteUsers(t)
		if err := testutils.DeleteUserType(typeID); err != nil {
			t.Logf("cleanup: failed to delete the %s user type: %v", name, err)
		}
	})
	return typeID, handle
}

// updateUserTypeSchema replaces the schema of a user type newUserType created.
func (f *verificationFixture) updateUserTypeSchema(t *testing.T, typeID, handle string,
	schema map[string]interface{}) {
	t.Helper()

	status, body := sendJSON(t, testutils.GetHTTPClient(), http.MethodPut, "/user-types/"+typeID,
		testutils.UserType{Handle: handle, DisplayName: "Attribute Verification Changed User", OUID: f.ouID,
			AllowSelfRegistration: true, Schema: schema})
	require.Equal(t, http.StatusOK, status, "precondition: the user type's schema changes: %s", body)
}

// uniqueMobile returns an E.164 mobile number no other user or test has.
func (f *verificationFixture) uniqueMobile() string {
	f.mobiles++
	return fmt.Sprintf("+1555%04d%03d", f.mobileStem, f.mobiles)
}

// token signs the user in and returns their access token, which carries no permission.
func (f *verificationFixture) token(t *testing.T, u testUser) string {
	t.Helper()

	return signIn(t, u).AccessToken
}

// signIn signs the user in through the Console application and returns the tokens.
func signIn(t *testing.T, u testUser) *testutils.TokenResponse {
	t.Helper()

	tokenResp, err := testutils.ObtainAccessTokenWithPassword("CONSOLE", "https://localhost:8095/console",
		"openid", u.username, u.password, true)
	require.NoError(t, err, "failed to sign %s in", u.username)
	require.NotEmpty(t, tokenResp.AccessToken)
	return tokenResp
}

// selfServiceStart is the request that starts the self-service flow the handle names.
func selfServiceStart(handle string, inputs map[string]string) map[string]interface{} {
	return map[string]interface{}{
		"flowType": selfServiceFlowType,
		"handle":   handle,
		"inputs":   inputs,
		"verbose":  true,
	}
}

// execute posts one flow request as the caller the token names, or with no token when it is empty.
func execute(t *testing.T, token string, request map[string]interface{}) *common.FlowExecutionResult {
	t.Helper()

	result, err := common.ExecuteFlowWithToken(token, request)
	require.NoError(t, err)
	return result
}

// startVerification starts the fixture's copy of the shipped verification flow as the caller the token
// names.
func (f *verificationFixture) startVerification(t *testing.T, token string,
	inputs map[string]string) *common.FlowExecutionResult {
	t.Helper()

	return execute(t, token, selfServiceStart(f.flowHandle, inputs))
}

// adminToken returns the administrator's access token, which carries the system permission.
func adminToken(t *testing.T) string {
	t.Helper()

	token, err := testutils.GetAccessToken()
	require.NoError(t, err)
	return token
}

// requireRefused requires the flow to answer the request with an error step carrying code.
func requireRefused(t *testing.T, result *common.FlowExecutionResult, code, reason string) {
	t.Helper()

	require.NotNil(t, result.Step, "the request must be answered in the flow: %s", result.Body)
	require.Equal(t, flowStatusError, result.Step.FlowStatus, "%s: %s", reason, result.Body)
	require.NotNil(t, result.Step.Error, "the refusal must be reported: %s", result.Body)
	require.Equal(t, code, result.Step.Error.Code, "%s: %s", reason, result.Body)
}

// requireStartRefused requires the request to be refused before any flow runs, with the HTTP status
// and error code given.
func requireStartRefused(t *testing.T, result *common.FlowExecutionResult, status int, code, reason string) {
	t.Helper()

	require.Nil(t, result.Step, "%s before any flow runs: %s", reason, result.Body)
	require.Equal(t, status, result.StatusCode, "%s: %s", reason, result.Body)
	require.NotNil(t, result.Error, "the refusal must be reported: %s", result.Body)
	require.Equal(t, code, result.Error.Code, "%s: %s", reason, result.Body)
}

// requireNothingDisclosed requires that the response carries none of the values anywhere and none of
// the codes in its additional data. A code is checked in the additional data only, so that a numeric
// code cannot match part of an execution ID or a challenge token.
func requireNothingDisclosed(t *testing.T, name string, result *common.FlowExecutionResult,
	values, codes []string) {
	t.Helper()

	for _, value := range values {
		require.NotContains(t, string(result.Body), value, "the %s response must not carry a contact value or link",
			name)
	}
	for key, data := range result.Step.Data.AdditionalData {
		for _, code := range codes {
			require.NotContains(t, data, code, "the %s response must not carry a code in %s", name, key)
		}
	}
}

// requireCodePrompt requires the flow to be waiting for a code, on the prompt that submits it with
// verifyAction.
func requireCodePrompt(t *testing.T, result *common.FlowExecutionResult, verifyAction string) *common.FlowStep {
	t.Helper()

	require.Equal(t, http.StatusOK, result.StatusCode, "the flow must accept the request: %s", result.Body)
	require.Equal(t, flowStatusIncomplete, result.Step.FlowStatus,
		"the flow must wait for the code: %s", result.Body)
	require.True(t, common.HasInput(result.Step.Data.Inputs, "otp"), "the flow must ask for the code: %s",
		result.Body)
	require.True(t, common.HasAction(result.Step.Data.Actions, verifyAction),
		"the code prompt of the expected branch must be returned: %s", result.Body)
	return result.Step
}

// submitCode continues the flow with a code through verifyAction, without the bearer token.
func submitCode(t *testing.T, step *common.FlowStep, verifyAction, code string) *common.FlowStep {
	t.Helper()

	next, err := common.CompleteFlow(step.ExecutionID, map[string]string{"otp": code}, verifyAction,
		step.ChallengeToken)
	require.NoError(t, err, "the code submission must reach the flow")
	return next
}

// resend asks the flow for a new code through resendAction, without the bearer token.
func resend(t *testing.T, step *common.FlowStep, resendAction string) *common.FlowStep {
	t.Helper()

	next, err := common.CompleteFlow(step.ExecutionID, nil, resendAction, step.ChallengeToken)
	require.NoError(t, err, "the resend must reach the flow")
	return next
}

// continueRaw continues the flow at step through action with inputs, without the bearer token, and
// returns the whole response.
func continueRaw(t *testing.T, step *common.FlowStep, action string,
	inputs map[string]string) *common.FlowExecutionResult {
	t.Helper()

	request := map[string]interface{}{"executionId": step.ExecutionID, "challengeToken": step.ChallengeToken}
	if action != "" {
		request["action"] = action
	}
	if len(inputs) > 0 {
		request["inputs"] = inputs
	}
	result := execute(t, "", request)
	require.NotNil(t, result.Step, "the continuation must be answered in the flow: %s", result.Body)
	return result
}

// awaitEmailTo waits for an email to recipient and returns it.
func (f *verificationFixture) awaitEmailTo(t *testing.T, recipient string) *testutils.EmailMessage {
	t.Helper()

	var email *testutils.EmailMessage
	require.Eventually(t, func() bool {
		email = f.smtp.GetLastEmail()
		return email != nil
	}, deliveryWait, 50*time.Millisecond, "no email arrived for %s", recipient)
	require.Equal(t, []string{recipient}, email.To, "the email must go to the stored address only")
	return email
}

// awaitCode waits for the code emailed to recipient.
func (f *verificationFixture) awaitCode(t *testing.T, recipient string) string {
	t.Helper()

	code := f.awaitEmailTo(t, recipient).ExtractOTP()
	require.NotEmpty(t, code, "the email must carry a code")
	return code
}

// requireNoEmail requires that the requests made so far sent no email. It needs no wait: executors
// send before the flow answers, and the mock server records a message before acknowledging it, so the
// answer to the request is the barrier.
func (f *verificationFixture) requireNoEmail(t *testing.T, msgAndArgs ...interface{}) {
	t.Helper()

	require.Empty(t, f.smtp.GetEmails(), msgAndArgs...)
}

// verifyEmail runs the shipped verification flow for the user's email to completion. Use it where
// verification is a precondition rather than the thing under test.
func (f *verificationFixture) verifyEmail(t *testing.T, u testUser) {
	t.Helper()

	f.smtp.ClearEmails()
	step := requireCodePrompt(t,
		f.startVerification(t, f.token(t, u), map[string]string{"attribute": emailAttribute}), actionEmailVerify)
	done := submitCode(t, step, actionEmailVerify, f.awaitCode(t, u.email))
	require.Equal(t, flowStatusComplete, done.FlowStatus, "precondition: the user verifies their email")
	require.NotEmpty(t, verifiedAt(t, u.id, emailAttribute), "precondition: the email is verified")
	f.smtp.ClearEmails()
}

// verifyMobile runs the shipped verification flow for the user's mobile number to completion, with
// the SMS node sending through p. Use it where verification is a precondition rather than the thing
// under test.
func (f *verificationFixture) verifyMobile(t *testing.T, p *smsProvider, u testUser) {
	t.Helper()

	p.server.ClearMessages()
	step := requireCodePrompt(t,
		f.startVerification(t, f.token(t, u), map[string]string{"attribute": mobileAttribute}), actionMobileVerify)
	done := submitCode(t, step, actionMobileVerify, p.awaitCode(t))
	require.Equal(t, flowStatusComplete, done.FlowStatus, "precondition: the user verifies their mobile number")
	require.NotEmpty(t, verifiedAt(t, u.id, mobileAttribute), "precondition: the mobile number is verified")
}

// suspend suspends the user through the shipped suspension flow.
func suspend(t *testing.T, u testUser) {
	t.Helper()

	flowID, err := testutils.GetFlowIDByHandle(userSuspendFlowHandle, "ADMINISTRATION")
	require.NoError(t, err)
	result, err := common.ExecuteFlowWithToken(adminToken(t),
		map[string]interface{}{"flowId": flowID, "inputs": map[string]string{"subject": u.id}})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, result.StatusCode, "precondition: the user is suspended: %s", result.Body)
	require.Equal(t, flowStatusComplete, result.Step.FlowStatus, "precondition: the user is suspended")
}

// verifiedAttributes returns the verifiedAttributes a management read of the user reports, requiring the
// field, since it is present even when nothing is verified.
func verifiedAttributes(t *testing.T, userID string) map[string]testutils.VerifiedAttribute {
	t.Helper()

	body := readUserRaw(t, userID)
	var read struct {
		VerifiedAttributes map[string]testutils.VerifiedAttribute `json:"verifiedAttributes"`
	}
	require.NoError(t, json.Unmarshal(body, &read))
	require.NotNil(t, read.VerifiedAttributes, "a user read must carry verifiedAttributes: %s", body)
	return read.VerifiedAttributes
}

// verifiedAt returns when the user's attribute was verified, or "" when it is not.
func verifiedAt(t *testing.T, userID, attribute string) string {
	t.Helper()

	return verifiedAttributes(t, userID)[attribute].VerifiedAt
}

// requireVerifiedSince requires the attribute to be verified at a server time no earlier than since,
// allowing for clock granularity, and returns that time.
func requireVerifiedSince(t *testing.T, userID, attribute string, since time.Time) time.Time {
	t.Helper()

	raw := verifiedAt(t, userID, attribute)
	require.NotEmpty(t, raw, "%s must be verified", attribute)
	at, err := time.Parse(time.RFC3339Nano, raw)
	require.NoError(t, err, "verifiedAt must be an RFC 3339 time")
	require.False(t, at.Before(since.Add(-time.Second)), "verifiedAt %s must be the time of this verification", raw)
	require.False(t, at.After(time.Now().Add(time.Second)), "verifiedAt %s must not be in the future", raw)
	return at
}

// storedRows returns the user's stored profile and runtime rows: the modification time and system
// attributes of the profile, and the revision and modification time of the runtime row, which every
// write to them moves or may change.
func storedRows(t *testing.T, userID string) string {
	t.Helper()

	profile, err := testutils.QueryEntityDB(fmt.Sprintf(
		`SELECT UPDATED_AT, SYSTEM_ATTRIBUTES FROM "ENTITY" WHERE ID = '%s';`, userID))
	require.NoError(t, err, "the user's row must be readable")
	require.NotEmpty(t, profile, "the user must have a stored row")
	runtime, err := testutils.QueryRuntimeDB(fmt.Sprintf(
		`SELECT REVISION, UPDATED_AT FROM "ENTITY_RUNTIME_DATA" WHERE ENTITY_ID = '%s';`, userID))
	require.NoError(t, err, "the user's runtime row must be readable")
	return profile + "\n" + runtime
}

// readUserRaw reads one user through the management API and returns the body.
func readUserRaw(t *testing.T, userID string) []byte {
	t.Helper()

	resp, err := testutils.GetHTTPClient().Get(testServerURL + "/users/" + userID)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "user read failed: %s", body)
	return body
}

// readSelfRaw reads GET /users/me as the caller the token names and returns the body.
func readSelfRaw(t *testing.T, token string) []byte {
	t.Helper()

	status, body := sendJSON(t, testutils.GetHTTPClientWithToken(token), http.MethodGet, "/users/me", nil)
	require.Equal(t, http.StatusOK, status, "self read failed: %s", body)
	return body
}

// sendJSON sends one management request through client and returns the status and body.
func sendJSON(t *testing.T, client *http.Client, method, path string, payload interface{}) (int, []byte) {
	t.Helper()

	var reader io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		require.NoError(t, err)
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, testServerURL+path, reader)
	require.NoError(t, err)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, body
}

// adminUpdate replaces the user's attributes, and optionally type, through the management API.
func (f *verificationFixture) adminUpdate(t *testing.T, u testUser, userType string,
	attributes map[string]interface{}) {
	t.Helper()

	status, body := sendJSON(t, testutils.GetHTTPClient(), http.MethodPut, "/users/"+u.id,
		map[string]interface{}{"ouId": f.ouID, "type": userType, "attributes": attributes})
	require.Equal(t, http.StatusOK, status, "the administrator's update must succeed: %s", body)
}

// profile is the attribute set a write sends, with the given email.
func profile(u testUser, email string) map[string]interface{} {
	return map[string]interface{}{"username": u.username, "email": email}
}

// otherAddress is an address distinct from every other test's.
func otherAddress(u testUser) string {
	return fmt.Sprintf("changed.%s", u.email)
}

// maskedEmail is the destination the flows show for an address.
func maskedEmail(address string) string {
	local, domain, _ := strings.Cut(address, "@")
	return string([]rune(local)[:1]) + "•••@" + domain
}

// maskedMobile is the destination the flows show for a mobile number.
func maskedMobile(number string) string {
	return "•••" + number[len(number)-2:]
}

// wrongCode returns a code of the same shape that is definitely not the right one.
func wrongCode(code string) string {
	wrong := []rune(code)
	for i, r := range wrong {
		if r == '0' {
			wrong[i] = '1'
			continue
		}
		wrong[i] = '0'
	}
	return string(wrong)
}

// smsProvider is a mock SMS provider and the sender that posts to it. The sender posts only the message
// text, so a test cannot see the number an SMS went to.
type smsProvider struct {
	server   *testutils.MockNotificationServer
	senderID string
}

// newSMSProvider starts a mock SMS provider and creates a sender for it, both removed when t ends.
// name keeps the sender apart from other suites'.
func newSMSProvider(t *testing.T, name string) *smsProvider {
	t.Helper()

	p := &smsProvider{server: testutils.NewMockNotificationServer(0)}
	require.NoError(t, p.server.Start(), "failed to start the mock SMS server")
	t.Cleanup(func() { _ = p.server.Stop() })
	senderID, err := testutils.CreateNotificationSender(testutils.NotificationSender{
		Name:        "Attribute Verification " + name + " SMS Sender",
		Description: "Sender a mobile verification flow sends through",
		Provider:    "custom",
		Properties: []testutils.SenderProperty{
			{Name: "url", Value: p.server.GetSendSMSURL()},
			{Name: "http_method", Value: "POST"},
			{Name: "content_type", Value: "JSON"},
		},
	})
	require.NoError(t, err, "failed to create the SMS sender")
	p.senderID = senderID
	t.Cleanup(func() {
		if err := testutils.DeleteNotificationSender(senderID); err != nil {
			t.Logf("cleanup: failed to delete the SMS sender: %v", err)
		}
	})
	return p
}

// awaitMessage waits for an SMS and returns it, requiring it to be the only one sent.
func (p *smsProvider) awaitMessage(t *testing.T) *testutils.SMSMessage {
	t.Helper()

	var message *testutils.SMSMessage
	require.Eventually(t, func() bool {
		message = p.server.GetLastMessage()
		return message != nil
	}, deliveryWait, 50*time.Millisecond, "no SMS arrived")
	require.Nil(t, p.server.GetLastMessage(), "exactly one SMS must be sent")
	return message
}

// awaitCode waits for the SMS and returns its code.
func (p *smsProvider) awaitCode(t *testing.T) string {
	t.Helper()

	code := p.awaitMessage(t).OTP
	require.NotEmpty(t, code, "the SMS must carry a code")
	return code
}

// requireNone requires that the requests made so far sent no SMS. As for requireNoEmail, the answer
// to the request is the barrier.
func (p *smsProvider) requireNone(t *testing.T, msgAndArgs ...interface{}) {
	t.Helper()

	require.Empty(t, p.server.GetMessages(), msgAndArgs...)
}

// openLink continues a flow from its link as Gate does, with the execution ID and invite token and no
// bearer token.
func openLink(t *testing.T, link, inviteToken string) *common.FlowStep {
	t.Helper()

	step, err := common.CompleteFlow(testutils.ExtractQueryParam(link, "executionId"),
		map[string]string{"inviteToken": inviteToken}, "", "")
	require.NoError(t, err, "the link must reach the flow")
	return step
}

// useFlowConfigWith writes value under key in the flow section until the test ends, keeping the
// section's other keys.
func useFlowConfigWith(t *testing.T, key string, value map[string]interface{}) {
	t.Helper()

	original, err := testutils.MergeWritableServerConfig(flowConfigSection, map[string]interface{}{key: value})
	require.NoError(t, err, "failed to write %s", key)
	t.Cleanup(func() {
		if err := testutils.PutWritableServerConfig(flowConfigSection, original); err != nil {
			t.Errorf("cleanup: failed to restore the flow configuration: %v", err)
		}
	})
}
