// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// AuthenticationMethodScopeTestSuite checks that a lock stops at the authentication method it was
// earned at, using password and OTP, and that a held method is never sent a fresh challenge.
// Counter cases use the atomic endpoints; the dispatch cases drive flows.
type AuthenticationMethodScopeTestSuite struct {
	suite.Suite
	mockServer     *testutils.MockNotificationServer
	ouID           string
	entityTypeID   string
	senderID       string
	authFlowID     string
	recoveryFlowID string
	recoveryAppID  string
	otpAuthFlowID  string
	otpAuthAppID   string
	mfa            *mfaFixture
	userIDs        []string
	suffix         string
}

// scopeLockSeconds is this suite's only rung. No case here waits for a lock to lapse, so a rung far
// longer than any case keeps every "still locked" assertion independent of how fast the run is.
const scopeLockSeconds = 300

// authenticationMethodScopePolicy is the package policy with one long rung. Each method counts under
// its own key with this policy.
func authenticationMethodScopePolicy() map[string]interface{} {
	return map[string]interface{}{
		"user": map[string]interface{}{
			"lockGranularity": "authentication_method",
			"default": map[string]interface{}{
				"enabled":              true,
				"threshold":            lockThreshold,
				"failureWindowSeconds": failureWindowSecs,
				"lockDurationsSeconds": []int{scopeLockSeconds},
				"lockDecaySeconds":     lockDecaySeconds,
			},
		},
	}
}

func TestAuthenticationMethodScopeTestSuite(t *testing.T) {
	suite.Run(t, new(AuthenticationMethodScopeTestSuite))
}

var authenticationMethodScopeTestOU = testutils.OrganizationUnit{
	Handle:      "governance_scope_ou",
	Name:        "Governance AuthenticationMethod Scope Test OU",
	Description: "Organization unit for per-authentication_method lock acceptance tests",
}

var authenticationMethodScopeUserType = testutils.UserType{
	Handle:      "governance_scope_user",
	DisplayName: "governance_scope_user",
	Schema: map[string]interface{}{
		"username":      map[string]interface{}{"type": "string"},
		"password":      map[string]interface{}{"type": "string", "credential": true},
		"mobile_number": map[string]interface{}{"type": "string"},
	},
}

func (ts *AuthenticationMethodScopeTestSuite) SetupSuite() {
	applyPolicy(ts.T(), authenticationMethodScopePolicy())

	ts.mockServer = testutils.NewMockNotificationServer(0)
	ts.Require().NoError(ts.mockServer.Start(), "failed to start the mock notification server")

	ouID, err := testutils.CreateOrganizationUnit(authenticationMethodScopeTestOU)
	ts.Require().NoError(err, "failed to create the test organization unit")
	ts.ouID = ouID

	authenticationMethodScopeUserType.OUID = ts.ouID
	typeID, err := testutils.CreateUserType(authenticationMethodScopeUserType)
	ts.Require().NoError(err, "failed to create the test user type")
	ts.entityTypeID = typeID

	senderID, err := testutils.CreateNotificationSender(testutils.NotificationSender{
		Name:        "Governance Lockout SMS Sender",
		Description: "Sender for per-authentication_method lock acceptance tests",
		Provider:    "custom",
		Properties: []testutils.SenderProperty{
			{Name: "url", Value: ts.mockServer.GetSendSMSURL()},
			{Name: "http_method", Value: "POST"},
			{Name: "content_type", Value: "JSON"},
		},
	})
	ts.Require().NoError(err, "failed to create the notification sender")
	ts.senderID = senderID

	// A recovery flow that dispatches a one-time password: an atomic send has no flow type.
	recoveryFlowID, err := testutils.CreateFlow(buildOTPRecoveryFlow(senderID))
	ts.Require().NoError(err, "failed to create the recovery flow")
	ts.recoveryFlowID = recoveryFlowID

	// An isolated authentication flow, because the default one reaches the default recovery flow and
	// an application may not name two different ones.
	authFlowID, err := testutils.CreateIsolatedAuthFlow("governance-scope-isolated-auth")
	ts.Require().NoError(err, "failed to create the isolated auth flow")
	ts.authFlowID = authFlowID

	recoveryAppID, err := testutils.CreateApplication(testutils.Application{
		OUID:                  ts.ouID,
		Name:                  "Governance AuthenticationMethod Scope Recovery App",
		Description:           "Application the dispatch acceptance tests drive recovery through",
		IsRecoveryFlowEnabled: true,
		ClientID:              "governance_scope_recovery_client",
		ClientSecret:          "governance_scope_recovery_secret",
		RedirectURIs:          []string{"http://localhost:3000/callback"},
		AllowedUserTypes:      []string{authenticationMethodScopeUserType.Handle},
		AuthFlowID:            authFlowID,
		RecoveryFlowID:        recoveryFlowID,
	})
	ts.Require().NoError(err, "failed to create the recovery application")
	ts.recoveryAppID = recoveryAppID

	// An authentication flow that dispatches a one-time password. The guard is in the flow's
	// OTPExecutor; the atomic send endpoint is not guarded.
	otpAuthFlowID, err := testutils.CreateFlow(buildOTPAuthenticationFlow(senderID))
	ts.Require().NoError(err, "failed to create the OTP authentication flow")
	ts.otpAuthFlowID = otpAuthFlowID

	otpAuthAppID, err := testutils.CreateApplication(testutils.Application{
		OUID:             ts.ouID,
		Name:             "Governance AuthenticationMethod Scope OTP App",
		Description:      "Application the dispatch acceptance tests drive OTP sign-in through",
		ClientID:         "governance_scope_otp_client",
		ClientSecret:     "governance_scope_otp_secret",
		RedirectURIs:     []string{"http://localhost:3000/callback"},
		AllowedUserTypes: []string{authenticationMethodScopeUserType.Handle},
		AuthFlowID:       otpAuthFlowID,
	})
	ts.Require().NoError(err, "failed to create the OTP authentication application")
	ts.otpAuthAppID = otpAuthAppID

	ts.mfa = newMFAFixture(ts.T(), "scope", ts.ouID, authenticationMethodScopeUserType.Handle)

	ts.suffix = uniqueSuffix()
}

func (ts *AuthenticationMethodScopeTestSuite) TearDownSuite() {
	ts.mfa.tearDown(ts.T())
	for _, userID := range ts.userIDs {
		if err := testutils.DeleteUser(userID); err != nil {
			ts.T().Logf("teardown: failed to delete user %s: %v", userID, err)
		}
	}
	for _, appID := range []string{ts.recoveryAppID, ts.otpAuthAppID} {
		if appID == "" {
			continue
		}
		if err := testutils.DeleteApplication(appID); err != nil {
			ts.T().Logf("teardown: failed to delete application %s: %v", appID, err)
		}
	}
	for _, flowID := range []string{ts.recoveryFlowID, ts.authFlowID, ts.otpAuthFlowID} {
		if flowID == "" {
			continue
		}
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("teardown: failed to delete flow %s: %v", flowID, err)
		}
	}
	if ts.senderID != "" {
		if err := testutils.DeleteNotificationSender(ts.senderID); err != nil {
			ts.T().Logf("teardown: failed to delete the notification sender: %v", err)
		}
	}
	if ts.entityTypeID != "" {
		if err := testutils.DeleteUserType(ts.entityTypeID); err != nil {
			ts.T().Logf("teardown: failed to delete user type: %v", err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("teardown: failed to delete organization unit: %v", err)
		}
	}
	if ts.mockServer != nil {
		_ = ts.mockServer.Stop()
	}
}

func (ts *AuthenticationMethodScopeTestSuite) SetupTest() {
	ts.mockServer.ClearMessages()
}

// newUser creates an account with a password and a mobile number, so it can fail at either method.
// Each test gets its own so one test's lock cannot decide another's outcome.
func (ts *AuthenticationMethodScopeTestSuite) newUser(name string, mobileSeq int) (userID, username, mobile, password string) {
	ts.T().Helper()

	username = fmt.Sprintf("scope_%s_%s", name, ts.suffix)
	password = "Correct-Horse-" + name + "1!"
	mobile = fmt.Sprintf("+1600%07d", mobileSeq)
	attributes, err := json.Marshal(map[string]interface{}{
		"username":      username,
		"password":      password,
		"mobile_number": mobile,
	})
	ts.Require().NoError(err)

	userID, err = testutils.CreateUser(testutils.User{
		Type:       authenticationMethodScopeUserType.Handle,
		OUID:       ts.ouID,
		Attributes: attributes,
	})
	ts.Require().NoError(err, "failed to create the test user")
	ts.userIDs = append(ts.userIDs, userID)
	return userID, username, mobile, password
}

// sendOTP asks the direct endpoint for a code and returns the session token, the code itself, and
// the HTTP status. An empty code means the dispatch produced no message.
func (ts *AuthenticationMethodScopeTestSuite) sendOTP(recipient string) (sessionToken, code string, status int) {
	ts.T().Helper()
	return sendDirectOTP(ts.T(), ts.mockServer, ts.senderID, recipient)
}

// verifyOTP presents a code against a session and reports whether it was accepted.
func (ts *AuthenticationMethodScopeTestSuite) verifyOTP(sessionToken, code string) bool {
	ts.T().Helper()
	return verifyDirectOTP(ts.T(), sessionToken, code).succeeded()
}

// lockTheOTPAuthenticationMethod fails the one-time-password authentication method to its threshold, each attempt against its
// own freshly issued session so that nothing but the wrong code is being tested.
func (ts *AuthenticationMethodScopeTestSuite) lockTheOTPAuthenticationMethod(recipient string) {
	ts.T().Helper()
	ts.failOTP(recipient, lockThreshold)
}

// failOTP presents count wrong codes, each against its own freshly issued session.
func (ts *AuthenticationMethodScopeTestSuite) failOTP(recipient string, count int) {
	ts.T().Helper()

	for i := range count {
		sessionToken, code, status := ts.sendOTP(recipient)
		ts.Require().Equal(http.StatusOK, status, "the code must be dispatched on attempt %d", i+1)
		ts.Require().NotEmpty(sessionToken, "attempt %d produced no session", i+1)
		ts.Require().NotEmpty(code, "attempt %d dispatched no code", i+1)

		ts.Require().False(ts.verifyOTP(sessionToken, wrongCodeFor(code)),
			"the wrong code must be refused on attempt %d", i+1)
	}
}

// wrongCodeFor returns a code of the same shape that is definitely not the right one.
func wrongCodeFor(code string) string {
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

// A password lock does not withhold the one-time password.
func (ts *AuthenticationMethodScopeTestSuite) TestAPasswordLockLeavesTheOneTimePasswordUsable() {
	userID, username, mobile, _ := ts.newUser("password_lock", 1)

	failAuthentication(ts.T(), "username", username, lockThreshold)
	ts.Require().Equal("LOCKED", readStatus(ts.T(), userID).Value, "the password authentication_method must be locked")
	ts.Require().Equal([]string{"credential"}, readStatus(ts.T(), userID).lockedScopes(),
		"only the authentication_method that failed may be held")

	sessionToken, code, status := ts.sendOTP(mobile)
	ts.Require().Equal(http.StatusOK, status, "direct send precondition: the send must be accepted")
	ts.Require().NotEmpty(code, "direct send precondition: a code must be returned")

	ts.True(ts.verifyOTP(sessionToken, code),
		"the one-time password must still authenticate while the password is locked")

	// The direct send is not governed, so only a flow shows that the dispatch is not withheld.
	ts.mockServer.ClearMessages()
	step := ts.startOTPSignIn(username)
	sent := awaitSMS(ts.mockServer)
	ts.Require().NotNil(sent, "a flow must still dispatch a code while the password is locked")
	done := ts.submitOTPSignIn(step, sent.OTP)
	ts.Equal("COMPLETE", done.FlowStatus, "a one-time-password sign-in must complete while the password is locked")
}

// A one-time-password lock does not withhold the password.
func (ts *AuthenticationMethodScopeTestSuite) TestAnOTPLockLeavesThePasswordUsable() {
	userID, username, mobile, password := ts.newUser("otp_lock", 2)

	ts.lockTheOTPAuthenticationMethod(mobile)

	ts.Require().Equal("LOCKED", readStatus(ts.T(), userID).Value,
		"failing the one-time-password authentication_method must lock something")
	ts.Equal([]string{"otp"}, readStatus(ts.T(), userID).lockedScopes(),
		"only the one-time-password authentication_method may be held")

	ts.True(authenticate(ts.T(), "username", username, password).succeeded(),
		"the password must still authenticate while the one-time password is locked")
}

// At the one-time password, too: while the one-time-password method is locked the correct code
// is refused and the lock is stated, with the status a wrong code gets. The direct send is not governed,
// so the code does reach the held account's phone.
func (ts *AuthenticationMethodScopeTestSuite) TestALockedOneTimePasswordRefusesTheCorrectCode() {
	heldID, _, heldMobile, _ := ts.newUser("otp_correct_code", 11)
	_, _, liveMobile, _ := ts.newUser("otp_correct_code_live", 12)

	ts.lockTheOTPAuthenticationMethod(heldMobile)
	ts.Require().Equal([]string{"otp"}, readStatus(ts.T(), heldID).lockedScopes(),
		"the one-time-password method must be the locked one")

	heldSession, heldCode, status := ts.sendOTP(heldMobile)
	ts.Require().Equal(http.StatusOK, status, "the direct send must accept the request")
	ts.Require().NotEmpty(heldCode, "the direct send must deliver a code to the held account")
	liveSession, liveCode, status := ts.sendOTP(liveMobile)
	ts.Require().Equal(http.StatusOK, status, "the direct send must accept the request")
	ts.Require().NotEmpty(liveCode, "the control account must receive a code")

	held := verifyDirectOTP(ts.T(), heldSession, heldCode)
	wrong := verifyDirectOTP(ts.T(), liveSession, wrongCodeFor(liveCode))

	ts.False(held.succeeded(), "the correct code must be refused while the method is locked")
	requireStatedHold(ts.T(), wrong, held, statedLockCode)
	ts.NotContains(held.body, "otp", "the refusal must not name the locked sign-in method")
}

// Counter isolation. Failures at one authentication method are not counted against
// another. The password is failed to its threshold and the one-time password to one short of its own,
// so any cross-counting would lock the one-time password and refuse the correct code.
func (ts *AuthenticationMethodScopeTestSuite) TestFailuresAtOneAuthenticationMethodDoNotCountAgainstAnother() {
	userID, username, mobile, _ := ts.newUser("counter_isolation", 8)

	failAuthentication(ts.T(), "username", username, lockThreshold)
	ts.failOTP(mobile, lockThreshold-1)

	ts.Require().Equal([]string{"credential"}, readStatus(ts.T(), userID).lockedScopes(),
		"only the password may be locked at this point")

	sessionToken, code, status := ts.sendOTP(mobile)
	ts.Require().Equal(http.StatusOK, status, "direct send precondition: the send must be accepted")
	ts.Require().NotEmpty(code, "direct send precondition: a code must be returned")

	ts.True(ts.verifyOTP(sessionToken, code),
		"the password's failures must not have spent the one-time password's allowance")
}

// A held one-time-password authentication method is sent no challenge, and the step states the lock
// instead of prompting for a code. Withholding counts nothing, so the lock is not extended.
func (ts *AuthenticationMethodScopeTestSuite) TestNoChallengeIsSentToAHeldAuthenticationMethod() {
	heldID, heldUsername, heldMobile, _ := ts.newUser("dispatch", 3)
	_, liveUsername, _, _ := ts.newUser("dispatch_live", 6)

	ts.lockTheOTPAuthenticationMethod(heldMobile)
	before, reported := readStatus(ts.T(), heldID).expiryOf("otp")
	ts.Require().True(reported, "the one-time-password lock must report an expiry")

	// Dispatch is synchronous, so whatever the held sign-in sent is already at the mock.
	ts.mockServer.ClearMessages()
	held := ts.startOTPSignIn(heldUsername)
	ts.Equal(0, smsCount(ts.T(), ts.mockServer), "no code may be dispatched to a held authentication_method")
	requireStatedFlowLock(ts.T(), held)

	live := ts.startOTPSignIn(liveUsername)
	sent := awaitSMS(ts.mockServer)
	ts.Require().NotNil(sent, "the control account must actually receive a code")
	ts.Require().NotEmpty(sent.OTP)
	ts.Require().Equal("INCOMPLETE", live.FlowStatus, "the control account must reach the code prompt")

	after := readStatus(ts.T(), heldID)
	ts.Equal([]string{"otp"}, after.lockedScopes(), "withholding must not reach another method")
	afterExpiry, reported := after.expiryOf("otp")
	ts.Require().True(reported, "the one-time-password lock must still be reported")
	ts.Equal(before, afterExpiry, "a withheld dispatch must not extend the lock")
}

// At a second factor: after a correct password, a held one-time-password method is sent no code, and
// the step states the lock.
func (ts *AuthenticationMethodScopeTestSuite) TestASecondFactorIsNotSentToAHeldAuthenticationMethod() {
	heldID, heldUsername, heldMobile, heldPassword := ts.newUser("mfa_held", 9)
	_, liveUsername, _, livePassword := ts.newUser("mfa_live", 10)

	ts.lockTheOTPAuthenticationMethod(heldMobile)
	ts.Require().Equal([]string{"otp"}, readStatus(ts.T(), heldID).lockedScopes(),
		"the one-time-password method must be the locked one")

	held := ts.mfa.submitPassword(ts.T(), heldUsername, heldPassword)
	ts.mfa.requireNoCodeSent(ts.T(), "no second factor may be dispatched to a held authentication_method")
	requireStatedFlowLock(ts.T(), held)

	ts.mfa.passPassword(ts.T(), liveUsername, livePassword)
	ts.NotEmpty(ts.mfa.sentCode(ts.T()), "the control account must receive its second factor")
}

// startOTPSignIn submits a username to the OTP authentication flow and returns the resulting step.
func (ts *AuthenticationMethodScopeTestSuite) startOTPSignIn(username string) *common.FlowStep {
	ts.T().Helper()

	step, err := common.InitiateAuthenticationFlow(ts.otpAuthAppID, false, nil, "")
	ts.Require().NoError(err, "failed to initiate the OTP authentication flow")
	step, err = common.CompleteFlow(step.ExecutionID,
		map[string]string{"username": username}, "action_submit_username", step.ChallengeToken)
	ts.Require().NoError(err, "failed to submit the username")
	return step
}

// submitOTPSignIn submits a code at the OTP authentication flow's prompt.
func (ts *AuthenticationMethodScopeTestSuite) submitOTPSignIn(step *common.FlowStep, code string) *common.FlowStep {
	ts.T().Helper()

	next, err := common.CompleteFlow(step.ExecutionID, map[string]string{"otp": code}, "action_submit_otp",
		step.ChallengeToken)
	ts.Require().NoError(err, "failed to submit the code")
	return next
}

// requireStatedFlowLock checks a flow step ended on the stated lock.
func requireStatedFlowLock(t *testing.T, step *common.FlowStep) {
	t.Helper()

	require.Equal(t, "ERROR", step.FlowStatus, "a withheld challenge must end the step")
	require.NotNil(t, step.Error, "a withheld challenge must state the hold")
	require.Equal(t, flowStatedLockCode, step.Error.Code, "the step must state the lock")
}

// Asking for codes never moves a counter. A step that only sends one must not be able to lock an
// account, or generation becomes the denial of service the whole design removes.
func (ts *AuthenticationMethodScopeTestSuite) TestRequestingCodesLocksNobody() {
	userID, username, mobile, password := ts.newUser("dispatch_count", 4)

	for range lockThreshold * 2 {
		_, code, status := ts.sendOTP(mobile)
		ts.Require().Equal(http.StatusOK, status, "each dispatch must be accepted")
		ts.Require().NotEmpty(code, "each dispatch must produce a code")
	}

	ts.Equal("ACTIVE", readStatus(ts.T(), userID).Value, "sending codes must lock nothing")
	ts.True(authenticate(ts.T(), "username", username, password).succeeded(),
		"the account must still be usable")
}

// A recovery flow's challenge still goes out to a locked account, since recovery is the user's
// only exit from the ladder. The flow type is the only difference from the sign-in dispatch test.
func (ts *AuthenticationMethodScopeTestSuite) TestARecoveryFlowStillChallengesALockedAccount() {
	userID, username, mobile, _ := ts.newUser("recovery_dispatch", 7)

	ts.lockTheOTPAuthenticationMethod(mobile)
	ts.Require().Equal([]string{"otp"}, readStatus(ts.T(), userID).lockedScopes(),
		"the one-time-password authentication_method must be the locked one")

	ts.mockServer.ClearMessages()

	step, err := common.InitiateRecoveryFlow(ts.recoveryAppID, false, nil, "")
	ts.Require().NoError(err, "failed to initiate the recovery flow")
	ts.Require().Equal("INCOMPLETE", step.FlowStatus)

	step, err = common.CompleteFlow(step.ExecutionID,
		map[string]string{"username": username}, "action_submit_username", step.ChallengeToken)
	ts.Require().NoError(err, "failed to submit the username")
	ts.Require().Equal("INCOMPLETE", step.FlowStatus,
		"a locked account must still be identified by a recovery flow")

	message := awaitSMS(ts.mockServer)
	ts.Require().NotNil(message, "a recovery flow must still dispatch to a locked account")
	ts.NotEmpty(message.OTP, "the dispatched challenge must carry a usable code")
}

// buildOTPRecoveryFlow is a recovery flow that dispatches a one-time password after identifying the
// account: identify, generate, send, prompt. It stops at the prompt, since the test is about the
// dispatch.
func buildOTPRecoveryFlow(senderID string) testutils.Flow {
	return testutils.Flow{
		Name:     "Governance Dispatch Recovery Test Flow",
		Handle:   "governance-dispatch-recovery-test",
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
				"executor": map[string]interface{}{
					"name": "IdentifyingExecutor",
					"mode": "identify",
					"inputs": []map[string]interface{}{
						{
							"ref":        "input_username",
							"identifier": "username",
							"type":       "TEXT_INPUT",
							"required":   true,
						},
					},
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
				},
				"onSuccess": "sms_send",
				"onFailure": "otp_sent_status",
			},
			{
				"id":   "sms_send",
				"type": "TASK_EXECUTION",
				"properties": map[string]interface{}{
					"senderId":    senderID,
					"smsTemplate": "otp",
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
							"nextNode": "end",
						},
					},
				},
			},
			{
				"id":   "end",
				"type": "END",
			},
		},
	}
}

// buildOTPAuthenticationFlow is a sign-in flow that identifies the account by username and sends it a
// one-time password: prompt, generate, send, prompt, verify, assert.
func buildOTPAuthenticationFlow(senderID string) testutils.Flow {
	return testutils.Flow{
		Name:     "Governance Dispatch Authentication Test Flow",
		Handle:   "governance-dispatch-auth-test",
		FlowType: "AUTHENTICATION",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "prompt_username"},
			{
				"id":   "prompt_username",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{"ref": "input_username", "identifier": "username", "type": "TEXT_INPUT", "required": true},
						},
						"action": map[string]interface{}{"ref": "action_submit_username", "nextNode": "generate_otp"},
					},
				},
			},
			{
				"id":   "generate_otp",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "OTPExecutor",
					"mode": "generate",
					"inputs": []map[string]interface{}{
						{"ref": "input_username", "identifier": "username", "type": "TEXT_INPUT", "required": true},
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
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{"ref": "input_otp", "identifier": "otp", "type": "OTP_INPUT", "required": true},
						},
						"action": map[string]interface{}{"ref": "action_submit_otp", "nextNode": "verify_otp"},
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
