// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package attributeverification

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
)

// SelfVerificationMobileTestSuite drives the shipped self attribute verification flow with an SMS
// sender selected on its SMS node, as an operator does: a mobile number takes the mobile branch, which
// sends the code by SMS, and the branch follows the pinned contact whatever action the client sends.
type SelfVerificationMobileTestSuite struct {
	suite.Suite
	fixture *verificationFixture
	sms     *smsProvider
}

func TestSelfVerificationMobileTestSuite(t *testing.T) {
	suite.Run(t, new(SelfVerificationMobileTestSuite))
}

func (ts *SelfVerificationMobileTestSuite) SetupSuite() {
	ts.fixture = newVerificationFixture(ts.T(), "mobile")
	ts.sms = newSMSProvider(ts.T(), "Mobile")
	ts.fixture.nameSender(ts.T(), mobileSendNode, ts.sms.senderID)
}

func (ts *SelfVerificationMobileTestSuite) SetupTest() {
	ts.fixture.smtp.ClearEmails()
	ts.sms.server.ClearMessages()
}

// Starting with the mobile attribute returns the mobile branch's code prompt and sends a code by SMS
// and no email. The response shows the channel and a masked destination but neither the number nor
// the code, and the code verifies the mobile number only.
func (ts *SelfVerificationMobileTestSuite) TestAMobileNumberIsVerifiedBySMS() {
	user := ts.fixture.newUser(ts.T(), "mobile")
	started := time.Now()

	result := ts.fixture.startVerification(ts.T(), ts.fixture.token(ts.T(), user),
		map[string]string{"attribute": mobileAttribute})
	step := requireCodePrompt(ts.T(), result, actionMobileVerify)
	code := ts.sms.awaitCode(ts.T())

	ts.Equal("sms", step.Data.AdditionalData["channel"], "a mobile number must take the mobile branch")
	ts.Equal("true", step.Data.AdditionalData["smsSent"])
	ts.Equal(maskedMobile(user.mobile), step.Data.AdditionalData["maskedDestination"],
		"the code must go to the stored number")
	requireNothingDisclosed(ts.T(), "start", result, []string{user.mobile}, []string{code})
	ts.fixture.requireNoEmail(ts.T(), "a mobile verification must send no email")
	ts.Empty(verifiedAt(ts.T(), user.id, mobileAttribute), "sending a code verifies nothing")

	done := submitCode(ts.T(), step, actionMobileVerify, code)

	ts.Require().Equal(flowStatusComplete, done.FlowStatus, "the SMS code must complete the flow")
	requireVerifiedSince(ts.T(), user.id, mobileAttribute, started)
	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "verifying the mobile number must not verify the email")
}

// The flow takes the branch of the pinned contact, whatever action the start sends: a mobile number
// started with the email action is sent an SMS, and an email started with the mobile action is sent an
// email.
func (ts *SelfVerificationMobileTestSuite) TestTheBranchFollowsThePinnedContact() {
	for name, tc := range map[string]struct {
		attribute, action, verifyAction string
	}{
		"mobile number with the email action": {mobileAttribute, "email", actionMobileVerify},
		"email with the mobile action":        {emailAttribute, "mobile", actionEmailVerify},
	} {
		ts.Run(name, func() {
			ts.fixture.smtp.ClearEmails()
			ts.sms.server.ClearMessages()
			user := ts.fixture.newUser(ts.T(), "branch_"+tc.attribute)
			started := time.Now()
			request := selfServiceStart(ts.fixture.flowHandle, map[string]string{"attribute": tc.attribute})
			request["action"] = tc.action

			step := requireCodePrompt(ts.T(), execute(ts.T(), ts.fixture.token(ts.T(), user), request),
				tc.verifyAction)

			var code string
			if tc.attribute == mobileAttribute {
				code = ts.sms.awaitCode(ts.T())
				ts.fixture.requireNoEmail(ts.T(), "the email action must not send an email for a mobile number")
			} else {
				code = ts.fixture.awaitCode(ts.T(), user.email)
				ts.sms.requireNone(ts.T(), "the mobile action must not send an SMS for an email")
			}
			done := submitCode(ts.T(), step, tc.verifyAction, code)
			ts.Require().Equal(flowStatusComplete, done.FlowStatus, "the code must complete the flow")
			requireVerifiedSince(ts.T(), user.id, tc.attribute, started)
		})
	}
}

// A wrong SMS code returns the prompt with an error and records nothing; the correct code still works.
func (ts *SelfVerificationMobileTestSuite) TestAWrongSMSCodeReturnsThePromptAndRecordsNothing() {
	user := ts.fixture.newUser(ts.T(), "mobile_wrong")
	started := time.Now()

	step := requireCodePrompt(ts.T(), ts.fixture.startVerification(ts.T(), ts.fixture.token(ts.T(), user),
		map[string]string{"attribute": mobileAttribute}), actionMobileVerify)
	code := ts.sms.awaitCode(ts.T())

	refused := submitCode(ts.T(), step, actionMobileVerify, wrongCode(code))
	ts.Require().Equal(flowStatusIncomplete, refused.FlowStatus, "a wrong code must return to the prompt")
	ts.Require().NotNil(refused.Error, "a wrong code must be reported")
	ts.Equal(errCodeInvalidOTP, refused.Error.Code)
	ts.True(common.HasInput(refused.Data.Inputs, "otp"), "the prompt must ask for the code again")
	ts.Empty(verifiedAt(ts.T(), user.id, mobileAttribute), "a wrong code must record nothing")

	done := submitCode(ts.T(), refused, actionMobileVerify, code)
	ts.Require().Equal(flowStatusComplete, done.FlowStatus, "the correct code must still complete the flow")
	requireVerifiedSince(ts.T(), user.id, mobileAttribute, started)
}

// A mobile_number input naming another number, on the start and on a resend, does not redirect the
// code: each SMS is the only one sent, the flow shows the stored number, and the resent code verifies
// it.
func (ts *SelfVerificationMobileTestSuite) TestAMobileNumberInputDoesNotRedirectTheCode() {
	user := ts.fixture.newUser(ts.T(), "mobile_redirect")
	started := time.Now()

	step := requireCodePrompt(ts.T(), ts.fixture.startVerification(ts.T(), ts.fixture.token(ts.T(), user),
		map[string]string{"attribute": mobileAttribute, mobileAttribute: ts.fixture.uniqueMobile()}),
		actionMobileVerify)
	ts.Equal(maskedMobile(user.mobile), step.Data.AdditionalData["maskedDestination"],
		"the code must go to the stored number")
	ts.sms.awaitCode(ts.T())

	resent, err := common.CompleteFlow(step.ExecutionID,
		map[string]string{mobileAttribute: ts.fixture.uniqueMobile()}, actionMobileResend, step.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal(flowStatusIncomplete, resent.FlowStatus, "a resend must return to the prompt: %+v",
		resent.Error)
	ts.Require().True(common.HasAction(resent.Data.Actions, actionMobileVerify),
		"a resend must return to the mobile branch's prompt")
	ts.Equal(maskedMobile(user.mobile), resent.Data.AdditionalData["maskedDestination"],
		"the resent code must go to the stored number")
	done := submitCode(ts.T(), resent, actionMobileVerify, ts.sms.awaitCode(ts.T()))

	ts.Require().Equal(flowStatusComplete, done.FlowStatus, "the resent code must verify the stored number")
	requireVerifiedSince(ts.T(), user.id, mobileAttribute, started)
}
