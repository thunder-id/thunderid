// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// methodLockSeconds is this suite's only rung. No case here waits for a lock to lapse, so a rung far
// longer than any case keeps every "still locked" assertion independent of how fast the run is.
const methodLockSeconds = 300

// AuthenticationMethodTestSuite checks that a lock stops at the authentication method it was
// earned at, using password and OTP, and that a held method is never sent a fresh challenge.
// Counter cases use the atomic endpoints; the dispatch cases drive flows.
type AuthenticationMethodTestSuite struct {
	suite.Suite
	mockServer    *testutils.MockNotificationServer
	users         *userFixture
	senderID      string
	recoveryAppID string
	otpAuthAppID  string
	mfa           *mfaFixture
}

func TestAuthenticationMethodTestSuite(t *testing.T) {
	suite.Run(t, new(AuthenticationMethodTestSuite))
}

func (ts *AuthenticationMethodTestSuite) SetupSuite() {
	applyPolicy(ts.T(), lockoutPolicyWith(userDefaults(map[string]interface{}{
		"lockDurationsSeconds": []int{methodLockSeconds},
	})))

	ts.mockServer = testutils.NewMockNotificationServer(0)
	ts.Require().NoError(ts.mockServer.Start(), "failed to start the mock notification server")
	t := ts.T()
	t.Cleanup(func() {
		if err := ts.mockServer.Stop(); err != nil {
			t.Logf("teardown: failed to stop the mock notification server: %v", err)
		}
	})

	ts.users = newUserFixture(ts.T(), "method",
		testutils.UserType{Schema: credentialsSchema(map[string]interface{}{"mobile_number": stringAttribute})})

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
	t.Cleanup(func() {
		if err := testutils.DeleteNotificationSender(senderID); err != nil {
			t.Logf("teardown: failed to delete the notification sender: %v", err)
		}
	})

	// A recovery flow that dispatches a one-time password: an atomic send has no flow type.
	recoveryFlowID, err := testutils.CreateFlow(otpDispatchRecoveryFlow(senderID))
	ts.Require().NoError(err, "failed to create the recovery flow")
	cleanupFlow(t, recoveryFlowID)

	// An isolated authentication flow, because the default one reaches the default recovery flow and
	// an application may not name two different ones.
	authFlowID, err := testutils.CreateIsolatedAuthFlow("governance-method-isolated-auth")
	ts.Require().NoError(err, "failed to create the isolated auth flow")
	cleanupFlow(t, authFlowID)

	recoveryAppID, err := testutils.CreateApplication(testutils.Application{
		OUID:                  ts.users.ouID,
		Name:                  "Governance Authentication Method Recovery App",
		Description:           "Application the dispatch acceptance tests drive recovery through",
		IsRecoveryFlowEnabled: true,
		ClientID:              "governance_method_recovery_client",
		ClientSecret:          "governance_method_recovery_secret",
		RedirectURIs:          []string{"http://localhost:3000/callback"},
		AllowedUserTypes:      []string{ts.users.userType},
		AuthFlowID:            authFlowID,
		RecoveryFlowID:        recoveryFlowID,
	})
	ts.Require().NoError(err, "failed to create the recovery application")
	ts.recoveryAppID = recoveryAppID
	cleanupApplication(t, recoveryAppID)

	// An authentication flow that dispatches a one-time password. The guard is in the flow's
	// OTPExecutor; the atomic send endpoint is not guarded.
	otpAuthFlowID, err := testutils.CreateFlow(otpSignInFlow(senderID))
	ts.Require().NoError(err, "failed to create the OTP authentication flow")
	cleanupFlow(t, otpAuthFlowID)

	otpAuthAppID, err := testutils.CreateApplication(testutils.Application{
		OUID:             ts.users.ouID,
		Name:             "Governance Authentication Method OTP App",
		Description:      "Application the dispatch acceptance tests drive OTP sign-in through",
		ClientID:         "governance_method_otp_client",
		ClientSecret:     "governance_method_otp_secret",
		RedirectURIs:     []string{"http://localhost:3000/callback"},
		AllowedUserTypes: []string{ts.users.userType},
		AuthFlowID:       otpAuthFlowID,
	})
	ts.Require().NoError(err, "failed to create the OTP authentication application")
	ts.otpAuthAppID = otpAuthAppID
	cleanupApplication(t, otpAuthAppID)

	ts.mfa = newMFAFixture(ts.T(), "method", ts.users.ouID, ts.users.userType)
}

func (ts *AuthenticationMethodTestSuite) SetupTest() {
	ts.mockServer.ClearMessages()
}

// newUser creates an account with a password and a mobile number, so it can fail at either method.
// Each test gets its own so one test's lock cannot decide another's outcome.
func (ts *AuthenticationMethodTestSuite) newUser(name string, mobileSeq int) (
	userID, username, mobile, password string) {
	ts.T().Helper()

	mobile = fmt.Sprintf("+1600%07d", mobileSeq)
	userID, username, password = ts.users.newUser(ts.T(), name, map[string]interface{}{"mobile_number": mobile})
	return userID, username, mobile, password
}

// failOTP presents count wrong codes, each against its own freshly issued session so that nothing but
// the wrong code is tested.
func (ts *AuthenticationMethodTestSuite) failOTP(recipient string, count int) {
	ts.T().Helper()

	for i := range count {
		sessionToken, code, status := sendDirectOTP(ts.T(), ts.mockServer, ts.senderID, recipient)
		ts.Require().Equal(http.StatusOK, status, "the code must be dispatched on attempt %d", i+1)
		ts.Require().NotEmpty(sessionToken, "attempt %d produced no session", i+1)

		ts.Require().False(verifyDirectOTP(ts.T(), sessionToken, wrongCodeFor(code)).succeeded(),
			"the wrong code must be refused on attempt %d", i+1)
	}
}

func (ts *AuthenticationMethodTestSuite) TestAPasswordLockLeavesTheOneTimePasswordUsable() {
	userID, username, mobile, _ := ts.newUser("password_lock", 1)

	failAuthentication(ts.T(), "username", username, lockThreshold)
	ts.Require().Equal("LOCKED", readStatus(ts.T(), userID).Value, "the password authentication_method must be locked")
	ts.Require().Equal([]string{"credential"}, readStatus(ts.T(), userID).lockedMethods(),
		"only the authentication_method that failed may be held")

	sessionToken, code, status := sendDirectOTP(ts.T(), ts.mockServer, ts.senderID, mobile)
	ts.Require().Equal(http.StatusOK, status, "direct send precondition: the send must be accepted")

	ts.True(verifyDirectOTP(ts.T(), sessionToken, code).succeeded(),
		"the one-time password must still authenticate while the password is locked")

	// The direct send is not governed, so only a flow shows that the dispatch is not withheld.
	ts.mockServer.ClearMessages()
	step := ts.startOTPSignIn(username)
	sent := awaitDelivered(ts.T(), ts.mockServer.GetLastMessage,
		"a flow must still dispatch a code while the password is locked")
	done := ts.submitOTPSignIn(step, sent.OTP)
	ts.Equal("COMPLETE", done.FlowStatus, "a one-time-password sign-in must complete while the password is locked")
}

func (ts *AuthenticationMethodTestSuite) TestAnOTPLockLeavesThePasswordUsable() {
	userID, username, mobile, password := ts.newUser("otp_lock", 2)

	ts.failOTP(mobile, lockThreshold)

	ts.Require().Equal("LOCKED", readStatus(ts.T(), userID).Value,
		"failing the one-time-password authentication_method must lock something")
	ts.Equal([]string{"otp"}, readStatus(ts.T(), userID).lockedMethods(),
		"only the one-time-password authentication_method may be held")

	ts.True(authenticate(ts.T(), "username", username, password).succeeded(),
		"the password must still authenticate while the one-time password is locked")
}

// The direct send is not governed, so the held account still receives a correct code to present.
func (ts *AuthenticationMethodTestSuite) TestALockedOneTimePasswordRefusesTheCorrectCode() {
	heldID, _, heldMobile, _ := ts.newUser("otp_correct_code", 11)
	_, _, liveMobile, _ := ts.newUser("otp_correct_code_live", 12)

	ts.failOTP(heldMobile, lockThreshold)
	ts.Require().Equal([]string{"otp"}, readStatus(ts.T(), heldID).lockedMethods(),
		"the one-time-password method must be the locked one")

	heldSession, heldCode, status := sendDirectOTP(ts.T(), ts.mockServer, ts.senderID, heldMobile)
	ts.Require().Equal(http.StatusOK, status, "the direct send must accept the request")
	liveSession, liveCode, status := sendDirectOTP(ts.T(), ts.mockServer, ts.senderID, liveMobile)
	ts.Require().Equal(http.StatusOK, status, "the direct send must accept the request")

	held := verifyDirectOTP(ts.T(), heldSession, heldCode)
	wrong := verifyDirectOTP(ts.T(), liveSession, wrongCodeFor(liveCode))

	requireStatedHold(ts.T(), wrong, held, statedLockCode, "otp")
}

// Neither wrong code is counted, so neither runtime row is written.
func (ts *AuthenticationMethodTestSuite) TestAWrongCodeOnAHeldAccountStatesTheHold() {
	suspendedID, _, suspendedMobile, _ := ts.newUser("otp_wrong_suspended", 13)
	lockedID, _, lockedMobile, _ := ts.newUser("otp_wrong_locked", 14)
	_, _, liveMobile, _ := ts.newUser("otp_wrong_live", 15)

	runAdministrationFlow(ts.T(), suspendFlowHandle, suspendedID)
	ts.failOTP(lockedMobile, lockThreshold)
	ts.Require().Equal([]string{"otp"}, readStatus(ts.T(), lockedID).lockedMethods(),
		"the one-time-password method must be the locked one")

	liveSession, liveCode, status := sendDirectOTP(ts.T(), ts.mockServer, ts.senderID, liveMobile)
	ts.Require().Equal(http.StatusOK, status, "the direct send must accept the request")
	wrong := verifyDirectOTP(ts.T(), liveSession, wrongCodeFor(liveCode))

	for _, held := range []struct{ name, id, mobile, code, other string }{
		{"suspended", suspendedID, suspendedMobile, statedSuspensionCode, statedLockCode},
		{"locked", lockedID, lockedMobile, statedLockCode, statedSuspensionCode},
	} {
		session, code, status := sendDirectOTP(ts.T(), ts.mockServer, ts.senderID, held.mobile)
		ts.Require().Equal(http.StatusOK, status, "%s: the direct send must accept the request", held.name)
		revision := runtimeRevision(ts.T(), held.id)

		refused := verifyDirectOTP(ts.T(), session, wrongCodeFor(code))
		requireStatedHold(ts.T(), wrong, refused, held.code, "otp")
		ts.NotContains(refused.body, held.other, "%s: the refusal must state the hold the account has",
			held.name)
		ts.Equal(revision, runtimeRevision(ts.T(), held.id), "%s: the wrong code must not be counted",
			held.name)
	}
}

// The one-time password is failed to one short of its threshold, so any cross-counting would lock it.
func (ts *AuthenticationMethodTestSuite) TestFailuresAtOneAuthenticationMethodDoNotCountAgainstAnother() {
	userID, username, mobile, _ := ts.newUser("counter_isolation", 8)

	failAuthentication(ts.T(), "username", username, lockThreshold)
	ts.failOTP(mobile, lockThreshold-1)

	ts.Require().Equal([]string{"credential"}, readStatus(ts.T(), userID).lockedMethods(),
		"only the password may be locked at this point")

	sessionToken, code, status := sendDirectOTP(ts.T(), ts.mockServer, ts.senderID, mobile)
	ts.Require().Equal(http.StatusOK, status, "direct send precondition: the send must be accepted")

	ts.True(verifyDirectOTP(ts.T(), sessionToken, code).succeeded(),
		"the password's failures must not have spent the one-time password's allowance")
}

// Withholding writes nothing, so the lock is not extended either.
func (ts *AuthenticationMethodTestSuite) TestNoChallengeIsSentToAHeldAuthenticationMethod() {
	heldID, heldUsername, heldMobile, _ := ts.newUser("dispatch", 3)
	_, liveUsername, _, _ := ts.newUser("dispatch_live", 6)

	ts.failOTP(heldMobile, lockThreshold)
	before, reported := readStatus(ts.T(), heldID).expiryOf("otp")
	ts.Require().True(reported, "the one-time-password lock must report an expiry")
	revision := runtimeRevision(ts.T(), heldID)

	ts.mockServer.ClearMessages()
	held := ts.startOTPSignIn(heldUsername)
	requireNoCodeSent(ts.T(), ts.mockServer, "no code may be dispatched to a held authentication_method")
	requireStatedFlowLock(ts.T(), held)
	ts.Equal(revision, runtimeRevision(ts.T(), heldID), "a withheld dispatch must write nothing")

	live := ts.startOTPSignIn(liveUsername)
	sent := awaitDelivered(ts.T(), ts.mockServer.GetLastMessage, "the control account must actually receive a code")
	ts.Require().NotEmpty(sent.OTP)
	ts.Require().Equal("INCOMPLETE", live.FlowStatus, "the control account must reach the code prompt")

	after := readStatus(ts.T(), heldID)
	ts.Equal([]string{"otp"}, after.lockedMethods(), "withholding must not reach another method")
	afterExpiry, reported := after.expiryOf("otp")
	ts.Require().True(reported, "the one-time-password lock must still be reported")
	ts.Equal(before, afterExpiry, "a withheld dispatch must not extend the lock")
}

func (ts *AuthenticationMethodTestSuite) TestASecondFactorIsNotSentToAHeldAuthenticationMethod() {
	heldID, heldUsername, heldMobile, heldPassword := ts.newUser("mfa_held", 9)
	_, liveUsername, _, livePassword := ts.newUser("mfa_live", 10)

	ts.failOTP(heldMobile, lockThreshold)
	ts.Require().Equal([]string{"otp"}, readStatus(ts.T(), heldID).lockedMethods(),
		"the one-time-password method must be the locked one")

	held := ts.mfa.submitPassword(ts.T(), heldUsername, heldPassword)
	requireNoCodeSent(ts.T(), ts.mfa.server, "no second factor may be dispatched to a held authentication_method")
	requireStatedFlowLock(ts.T(), held)

	ts.mfa.passPassword(ts.T(), liveUsername, livePassword)
	ts.NotEmpty(ts.mfa.sentCode(ts.T()), "the control account must receive its second factor")
}

func (ts *AuthenticationMethodTestSuite) TestRequestingCodesLocksNobody() {
	userID, username, mobile, password := ts.newUser("dispatch_count", 4)

	for range lockThreshold * 2 {
		_, _, status := sendDirectOTP(ts.T(), ts.mockServer, ts.senderID, mobile)
		ts.Require().Equal(http.StatusOK, status, "each dispatch must be accepted")
	}

	ts.Equal("ACTIVE", readStatus(ts.T(), userID).Value, "sending codes must lock nothing")
	ts.True(authenticate(ts.T(), "username", username, password).succeeded(),
		"the account must still be usable")
}

// The flow type is the only difference from the sign-in dispatch test.
func (ts *AuthenticationMethodTestSuite) TestARecoveryFlowStillChallengesALockedAccount() {
	userID, username, mobile, _ := ts.newUser("recovery_dispatch", 7)

	ts.failOTP(mobile, lockThreshold)
	ts.Require().Equal([]string{"otp"}, readStatus(ts.T(), userID).lockedMethods(),
		"the one-time-password authentication_method must be the locked one")

	ts.mockServer.ClearMessages()

	step, err := common.InitiateRecoveryFlow(ts.recoveryAppID, false, nil, "")
	ts.Require().NoError(err, "failed to initiate the recovery flow")
	ts.Require().Equal("INCOMPLETE", step.FlowStatus)

	step, err = common.CompleteFlow(step.ExecutionID,
		map[string]string{"username": username}, submitUsernameAction, step.ChallengeToken)
	ts.Require().NoError(err, "failed to submit the username")
	ts.Require().Equal("INCOMPLETE", step.FlowStatus,
		"a locked account must still be identified by a recovery flow")

	message := awaitDelivered(ts.T(), ts.mockServer.GetLastMessage,
		"a recovery flow must still dispatch to a locked account")
	ts.NotEmpty(message.OTP, "the dispatched challenge must carry a usable code")
}

// startOTPSignIn submits a username to the OTP authentication flow and returns the resulting step.
func (ts *AuthenticationMethodTestSuite) startOTPSignIn(username string) *common.FlowStep {
	ts.T().Helper()

	step, err := common.InitiateAuthenticationFlow(ts.otpAuthAppID, false, nil, "")
	ts.Require().NoError(err, "failed to initiate the OTP authentication flow")
	step, err = common.CompleteFlow(step.ExecutionID,
		map[string]string{"username": username}, submitUsernameAction, step.ChallengeToken)
	ts.Require().NoError(err, "failed to submit the username")
	return step
}

// submitOTPSignIn submits a code at the OTP authentication flow's prompt.
func (ts *AuthenticationMethodTestSuite) submitOTPSignIn(step *common.FlowStep, code string) *common.FlowStep {
	ts.T().Helper()

	next, err := common.CompleteFlow(step.ExecutionID, map[string]string{"otp": code}, submitOTPAction,
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

// otpSignInFlow is a sign-in flow that identifies the account by username and sends it a one-time
// password: prompt, generate, send, prompt, verify, assert.
func otpSignInFlow(senderID string) testutils.Flow {
	return testutils.Flow{
		Name:     "Governance Dispatch Authentication Test Flow",
		Handle:   "governance-dispatch-auth-test",
		FlowType: "AUTHENTICATION",
		Nodes: []map[string]interface{}{
			startNode("prompt_username"),
			promptNode("prompt_username", submitUsernameAction, "generate_otp", usernameInput),
			withExecutorInputs(taskNode("generate_otp", "OTPExecutor", "generate", "sms_send", nil), usernameInput),
			smsSendNode(senderID, "prompt_otp", nil),
			promptNode("prompt_otp", submitOTPAction, "verify_otp", otpInput),
			taskNode("verify_otp", "OTPExecutor", "verify", "auth_assert", nil),
			taskNode("auth_assert", "AuthAssertExecutor", "", "end", nil),
			endNode(),
		},
	}
}
