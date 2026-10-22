// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// methodLockSeconds is this suite's only rung. No case here waits for a lock to lapse, so a rung far
// longer than any case keeps every "still locked" assertion independent of how fast the run is.
const methodLockSeconds = 300

// AuthenticationMethodTestSuite checks that a lock stops at the authentication method it was
// earned at, using password and OTP. Counter cases use the atomic endpoints.
type AuthenticationMethodTestSuite struct {
	suite.Suite
	mockServer    *testutils.MockNotificationServer
	users         *userFixture
	senderID      string
	otpAuthFlowID string
	otpAuthAppID  string
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

	// An authentication flow that dispatches a one-time password.
	otpAuthFlowID, err := testutils.CreateFlow(otpSignInFlow(senderID))
	ts.Require().NoError(err, "failed to create the OTP authentication flow")
	ts.otpAuthFlowID = otpAuthFlowID

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
}

func (ts *AuthenticationMethodTestSuite) TearDownSuite() {
	if ts.otpAuthAppID != "" {
		if err := testutils.DeleteApplication(ts.otpAuthAppID); err != nil {
			ts.T().Logf("teardown: failed to delete application %s: %v", ts.otpAuthAppID, err)
		}
	}
	if ts.otpAuthFlowID != "" {
		if err := testutils.DeleteFlow(ts.otpAuthFlowID); err != nil {
			ts.T().Logf("teardown: failed to delete flow %s: %v", ts.otpAuthFlowID, err)
		}
	}
	if ts.senderID != "" {
		if err := testutils.DeleteNotificationSender(ts.senderID); err != nil {
			ts.T().Logf("teardown: failed to delete the notification sender: %v", err)
		}
	}
	if ts.mockServer != nil {
		if err := ts.mockServer.Stop(); err != nil {
			ts.T().Logf("teardown: failed to stop the mock notification server: %v", err)
		}
	}
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

// The wrong code is not counted, so the runtime row is not written.
func (ts *AuthenticationMethodTestSuite) TestAWrongCodeOnAHeldAccountStatesTheHold() {
	lockedID, _, lockedMobile, _ := ts.newUser("otp_wrong_locked", 14)
	_, _, liveMobile, _ := ts.newUser("otp_wrong_live", 15)

	ts.failOTP(lockedMobile, lockThreshold)
	ts.Require().Equal([]string{"otp"}, readStatus(ts.T(), lockedID).lockedMethods(),
		"the one-time-password method must be the locked one")

	liveSession, liveCode, status := sendDirectOTP(ts.T(), ts.mockServer, ts.senderID, liveMobile)
	ts.Require().Equal(http.StatusOK, status, "the direct send must accept the request")
	wrong := verifyDirectOTP(ts.T(), liveSession, wrongCodeFor(liveCode))

	session, code, status := sendDirectOTP(ts.T(), ts.mockServer, ts.senderID, lockedMobile)
	ts.Require().Equal(http.StatusOK, status, "the direct send must accept the request")
	revision := runtimeRevision(ts.T(), lockedID)

	refused := verifyDirectOTP(ts.T(), session, wrongCodeFor(code))
	requireStatedHold(ts.T(), wrong, refused, statedLockCode, "otp")
	ts.NotContains(refused.body, statedSuspensionCode, "the refusal must state the hold the account has")
	ts.Equal(revision, runtimeRevision(ts.T(), lockedID), "the wrong code must not be counted")
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
