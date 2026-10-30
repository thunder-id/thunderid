// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package governance

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// The shipped self attribute verification flow, its send nodes, and the action that submits each
// branch's code.
const (
	selfVerificationFlowHandle = "default-self-attribute-verification-flow"
	selfServiceFlowType        = "SELF_SERVICE"
	selfVerificationEmailNode  = "send_email_otp"
	selfVerificationMobileNode = "send_mobile_otp"
)

// The contacts a verification here targets, and the action that submits each one's code.
var selfVerificationActions = map[string]string{
	"email":         "action_email_verify",
	"mobile_number": "action_mobile_verify",
}

// errCodeInvalidOTP is the flow's answer to a wrong code.
const errCodeInvalidOTP = "FET-1008"

// ContactVerificationTestSuite checks that a code proving control of an email or mobile number is not
// a sign-in: it is sent and admitted under an automatic lock and leaves the lock as it was, it is
// refused for a suspended user, and neither a wrong nor a passed code changes the sign-in failure
// count. Failures are counted through the direct one-time-password endpoint, which counts under the
// same authentication method.
type ContactVerificationTestSuite struct {
	suite.Suite
	users       *userFixture
	sms         *testutils.MockNotificationServer
	smsSenderID string
	smtp        *testutils.MockSMTPServer
	flowHandle  string
	mobileSeq   int
}

func TestContactVerificationTestSuite(t *testing.T) {
	suite.Run(t, new(ContactVerificationTestSuite))
}

// SetupSuite removes each resource when the suite ends, a failed setup included. Cleanups run newest
// first, so the flow copy is deleted before the senders it names.
func (ts *ContactVerificationTestSuite) SetupSuite() {
	t := ts.T()
	applyPolicy(t, longRungLockoutPolicy())

	ts.users = newUserFixture(t, "contact_verification", testutils.UserType{Schema: credentialsSchema(
		map[string]interface{}{
			"email":         map[string]interface{}{"type": "email"},
			"mobile_number": map[string]interface{}{"type": "mobile"},
		})})

	ts.sms = testutils.NewMockNotificationServer(0)
	ts.Require().NoError(ts.sms.Start(), "failed to start the mock notification server")
	t.Cleanup(func() { _ = ts.sms.Stop() })
	var err error
	ts.smsSenderID, err = testutils.CreateNotificationSender(testutils.NotificationSender{
		Name:        "Governance Contact Verification SMS Sender",
		Description: "Sender of the direct one-time-password endpoint and the verification flow's SMS",
		Provider:    "custom",
		Properties: []testutils.SenderProperty{
			{Name: "url", Value: ts.sms.GetSendSMSURL()},
			{Name: "http_method", Value: "POST"},
			{Name: "content_type", Value: "JSON"},
		},
	})
	ts.Require().NoError(err, "failed to create the SMS sender")
	ts.deleteSenderOnCleanup(ts.smsSenderID)

	ts.smtp = testutils.NewMockSMTPServer(0)
	ts.Require().NoError(ts.smtp.Start(), "failed to start the mock SMTP server")
	t.Cleanup(func() { _ = ts.smtp.Stop() })
	emailSenderID, err := testutils.CreateSMTPEmailProvider("Governance Contact Verification Provider",
		"localhost", ts.smtp.GetPort(), "noreply@thunderid.test")
	ts.Require().NoError(err, "failed to create the SMTP email provider")
	ts.deleteSenderOnCleanup(emailSenderID)

	_, ts.flowHandle = copyShippedFlowWithSenders(t, selfVerificationFlowHandle, selfServiceFlowType,
		map[string]string{selfVerificationEmailNode: emailSenderID, selfVerificationMobileNode: ts.smsSenderID},
		nil)
}

// deleteSenderOnCleanup deletes the sender when the suite ends.
func (ts *ContactVerificationTestSuite) deleteSenderOnCleanup(senderID string) {
	t := ts.T()
	t.Cleanup(func() {
		if err := testutils.DeleteNotificationSender(senderID); err != nil {
			t.Logf("cleanup: failed to delete sender %s: %v", senderID, err)
		}
	})
}

func (ts *ContactVerificationTestSuite) SetupTest() {
	ts.sms.ClearMessages()
	ts.smtp.ClearEmails()
}

// contactUser is an account with an email and a mobile number to verify and fail codes at.
type contactUser struct {
	id       string
	username string
	password string
	email    string
	mobile   string
}

// newUser creates an account no other test touches.
func (ts *ContactVerificationTestSuite) newUser(label string) contactUser {
	ts.T().Helper()

	ts.mobileSeq++
	suffix := ts.users.suffix
	u := contactUser{
		email:  fmt.Sprintf("contact.%s.%s@example.com", label, suffix),
		mobile: fmt.Sprintf("+1700%s%03d", suffix[len(suffix)-4:], ts.mobileSeq),
	}
	u.id, u.username, u.password = ts.users.newUser(ts.T(), label,
		map[string]interface{}{"email": u.email, "mobile_number": u.mobile})
	return u
}

// token signs the user in with their password, which no case here locks.
func (ts *ContactVerificationTestSuite) token(u contactUser) string {
	ts.T().Helper()

	tokenResp, err := testutils.ObtainAccessTokenWithPassword("CONSOLE", "https://localhost:8095/console",
		"openid", u.username, u.password, true)
	ts.Require().NoError(err, "failed to sign %s in", u.username)
	return tokenResp.AccessToken
}

// startVerification starts the self attribute verification flow for the user's attribute and returns
// the code prompt and the code sent to the stored contact.
func (ts *ContactVerificationTestSuite) startVerification(u contactUser, attribute string) (
	*common.FlowStep, string) {
	ts.T().Helper()

	result, err := common.ExecuteFlowWithToken(ts.token(u), map[string]interface{}{
		"flowType": selfServiceFlowType,
		"handle":   ts.flowHandle,
		"inputs":   map[string]string{"attribute": attribute},
	})
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, result.StatusCode, "%s", result.Body)
	ts.Require().Equal("INCOMPLETE", result.Step.FlowStatus, "the flow must wait for the code: %s", result.Body)

	if attribute == "email" {
		email := awaitDelivered(ts.T(), ts.smtp.GetLastEmail, "the code must be emailed")
		ts.Require().Equal([]string{u.email}, email.To)
		code := email.ExtractOTP()
		ts.Require().NotEmpty(code, "the email must carry a code")
		return result.Step, code
	}
	message := awaitDelivered(ts.T(), ts.sms.GetLastMessage, "the code must be sent by SMS")
	ts.Require().NotEmpty(message.OTP, "the SMS must carry a code")
	return result.Step, message.OTP
}

// submitCode continues the flow with a code for the attribute.
func (ts *ContactVerificationTestSuite) submitCode(step *common.FlowStep, attribute, code string) *common.FlowStep {
	ts.T().Helper()

	next, err := common.CompleteFlow(step.ExecutionID, map[string]string{"otp": code},
		selfVerificationActions[attribute], step.ChallengeToken)
	ts.Require().NoError(err)
	return next
}

// verifiedAt returns when the user's attribute was verified, or "" when it is not.
func (ts *ContactVerificationTestSuite) verifiedAt(u contactUser, attribute string) string {
	ts.T().Helper()

	var read struct {
		VerifiedAttributes map[string]testutils.VerifiedAttribute `json:"verifiedAttributes"`
	}
	body := getManagement(ts.T(), "/users/"+u.id)
	ts.Require().NoError(json.Unmarshal(body, &read))
	ts.Require().NotNil(read.VerifiedAttributes, "a user read must carry verifiedAttributes: %s", body)
	return read.VerifiedAttributes[attribute].VerifiedAt
}

// A user under an automatic lock is sent a verification code for their email or mobile number, the code
// verifies it, and the lock and its expiry stay as they were.
func (ts *ContactVerificationTestSuite) TestALockedUserVerifiesAndTheLockIsUnchanged() {
	for _, attribute := range []string{"email", "mobile_number"} {
		ts.Run(attribute, func() {
			u := ts.newUser("locked_" + attribute)
			failDirectOTP(ts.T(), ts.sms, ts.smsSenderID, u.mobile, lockThreshold)
			before := readStatus(ts.T(), u.id)
			ts.Require().Equal([]string{"otp"}, before.lockedMethods(),
				"precondition: the one-time password is locked")
			lockedUntil, reported := before.expiryOf("otp")
			ts.Require().True(reported, "precondition: the lock reports its expiry")
			ts.sms.ClearMessages()
			ts.smtp.ClearEmails()

			step, code := ts.startVerification(u, attribute)
			done := ts.submitCode(step, attribute, code)

			ts.Equal("COMPLETE", done.FlowStatus, "a locked user's code must complete the flow: %+v", done.Error)
			ts.NotEmpty(ts.verifiedAt(u, attribute), "a locked user must be able to verify the contact")
			after := readStatus(ts.T(), u.id)
			ts.Equal([]string{"otp"}, after.lockedMethods(), "verifying must not release the lock")
			stillUntil, reported := after.expiryOf("otp")
			ts.Require().True(reported)
			ts.True(lockedUntil.Equal(stillUntil), "verifying must not change when the lock ends")
		})
	}
}

// A user suspended after the code was sent is refused when the correct code is submitted: the code
// check states the suspension, as a sign-in check does, and nothing is recorded. The submission
// carries no bearer token, so the suspension's revocation does not stop the flow first.
func (ts *ContactVerificationTestSuite) TestAUserSuspendedAfterTheCodeIsSentIsRefused() {
	u := ts.newUser("suspended")
	step, code := ts.startVerification(u, "email")
	runAdministrationFlow(ts.T(), suspendFlowHandle, u.id)
	ts.Require().Equal("SUSPENDED", readStatus(ts.T(), u.id).Value, "precondition: the user is suspended")

	refused := ts.submitCode(step, "email", code)

	ts.NotEqual("COMPLETE", refused.FlowStatus, "a suspended user's code must not complete the flow")
	ts.Require().NotNil(refused.Error, "the refusal must be reported")
	ts.Equal(flowStatedSuspensionCode, refused.Error.Code, "the code check must state the suspension")
	ts.Empty(ts.verifiedAt(u, "email"), "a suspended user must not be verified")
}

// Wrong verification codes, twice the lock threshold of them, lock nothing; the correct code then
// verifies.
func (ts *ContactVerificationTestSuite) TestWrongCodesAreNotCountedTowardTheLock() {
	u := ts.newUser("wrong_codes")
	step, code := ts.startVerification(u, "email")

	for i := range 2 * lockThreshold {
		step = ts.submitCode(step, "email", wrongCodeFor(code))
		ts.Require().NotNil(step.Error, "wrong code %d must be refused", i+1)
		ts.Require().Equal(errCodeInvalidOTP, step.Error.Code, "wrong code %d must be answered as a wrong code", i+1)
	}
	status := readStatus(ts.T(), u.id)
	ts.Equal("ACTIVE", status.Value, "wrong verification codes must not lock the user")
	ts.Empty(status.lockedMethods())

	ts.Equal("COMPLETE", ts.submitCode(step, "email", code).FlowStatus, "the correct code must still verify")
	ts.NotEmpty(ts.verifiedAt(u, "email"))
}

// A passed verification code does not clear the sign-in failures counted before it: one more failure
// reaches the threshold.
func (ts *ContactVerificationTestSuite) TestAPassedCodeDoesNotClearSignInFailures() {
	u := ts.newUser("passed_code")
	failDirectOTP(ts.T(), ts.sms, ts.smsSenderID, u.mobile, lockThreshold-1)
	ts.Require().Equal("ACTIVE", readStatus(ts.T(), u.id).Value, "precondition: one failure short of the lock")

	step, code := ts.startVerification(u, "email")
	ts.Require().Equal("COMPLETE", ts.submitCode(step, "email", code).FlowStatus,
		"precondition: the correct code verifies the email")
	failDirectOTP(ts.T(), ts.sms, ts.smsSenderID, u.mobile, 1)

	ts.Equal([]string{"otp"}, readStatus(ts.T(), u.id).lockedMethods(),
		"the failures before the verification must still count")
}
