// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package attributeverification

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
)

// SelfVerificationMobileTestSuite drives the shipped self attribute verification flow with an SMS
// sender selected on its SMS node, as an operator does: a mobile number takes the mobile branch, which
// sends the code by SMS, and the branch follows the pinned contact whatever recipientType input the client
// sends.
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

// The mobile branch's verify node voids a code after three wrong codes, and a resend's new code
// verifies.
func (ts *SelfVerificationMobileTestSuite) TestThreeWrongSMSCodesVoidTheCode() {
	user := ts.fixture.newUser(ts.T(), "mobile_wrong_cap")
	started := time.Now()

	step := requireCodePrompt(ts.T(), ts.fixture.startVerification(ts.T(), ts.fixture.token(ts.T(), user),
		map[string]string{"attribute": mobileAttribute}), actionMobileVerify)
	code := ts.sms.awaitCode(ts.T())

	for wrong := 1; wrong <= shippedWrongCodeLimit; wrong++ {
		want := errCodeInvalidOTP
		if wrong == shippedWrongCodeLimit {
			want = errCodeMaxOTPAttempts
		}
		step = submitCode(ts.T(), step, actionMobileVerify, wrongCode(code))
		ts.Require().Equal(flowStatusIncomplete, step.FlowStatus, "wrong code %d must return to the prompt", wrong)
		ts.Require().NotNil(step.Error, "wrong code %d must be reported", wrong)
		ts.Require().Equal(want, step.Error.Code, "wrong code %d", wrong)
	}

	refused := submitCode(ts.T(), step, actionMobileVerify, code)
	ts.Require().NotNil(refused.Error, "a void code must be reported")
	ts.Equal(errCodeMaxOTPAttempts, refused.Error.Code)
	ts.Empty(verifiedAt(ts.T(), user.id, mobileAttribute), "a void code must record nothing")

	resent := resend(ts.T(), refused, actionMobileResend)
	ts.Require().Equal(flowStatusIncomplete, resent.FlowStatus, "a resend must return to the prompt")
	done := submitCode(ts.T(), resent, actionMobileVerify, ts.sms.awaitCode(ts.T()))
	ts.Require().Equal(flowStatusComplete, done.FlowStatus, "the resent code must verify the mobile number")
	requireVerifiedSince(ts.T(), user.id, mobileAttribute, started)
}

// Mobile resends share the execution's code generation limit: once it is reached, a resend sends
// nothing.
func (ts *SelfVerificationMobileTestSuite) TestSMSResendsStopAtTheGenerationLimit() {
	user := ts.fixture.newUser(ts.T(), "mobile_resend_limit")
	step := requireCodePrompt(ts.T(), ts.fixture.startVerification(ts.T(), ts.fixture.token(ts.T(), user),
		map[string]string{"attribute": mobileAttribute}), actionMobileVerify)
	ts.sms.awaitCode(ts.T())

	for generated := 1; generated < shippedCodeGenerationLimit; generated++ {
		step = resend(ts.T(), step, actionMobileResend)
		ts.Require().Equal(flowStatusIncomplete, step.FlowStatus, "resend %d is within the limit", generated)
		ts.sms.awaitCode(ts.T())
	}

	refused := resend(ts.T(), step, actionMobileResend)

	ts.Equal(flowStatusError, refused.FlowStatus, "a resend past the limit must be refused")
	ts.Require().NotNil(refused.Error, "the flow must say why it stopped")
	ts.Equal(errCodeMaxOTPAttempts, refused.Error.Code)
	ts.sms.requireNone(ts.T(), "no code may be sent past the limit")
}

// A mobile_number input naming another number, on the start and on a resend, sends one SMS each, the
// flow shows the stored number, and the resent code verifies it. The SMS mock sees no recipient, so the
// SMS executor's unit tests pin the recipient.
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

// A recipientType input naming the other channel does not change the branch: each code goes to the
// pinned contact on its own channel, nothing is sent on the other channel, and the resent code verifies
// the pinned contact only.
func (ts *SelfVerificationMobileTestSuite) TestTheOtherChannelOnTheStartDoesNotChangeTheBranch() {
	for name, tc := range map[string]struct {
		attribute, other, channel  string
		spoof                      map[string]string
		verifyAction, resendAction string
	}{
		"email with recipientType mobile": {attribute: emailAttribute, other: mobileAttribute, channel: "email",
			spoof: map[string]string{"recipientType": "mobile"}, verifyAction: actionEmailVerify,
			resendAction: actionEmailResend},
		"mobile number with recipientType email": {attribute: mobileAttribute, other: emailAttribute,
			channel: "sms", spoof: map[string]string{"recipientType": "email"}, verifyAction: actionMobileVerify,
			resendAction: actionMobileResend},
	} {
		ts.Run(name, func() {
			ts.fixture.smtp.ClearEmails()
			ts.sms.server.ClearMessages()
			user := ts.fixture.newUser(ts.T(), strings.ReplaceAll(name, " ", "_"))
			started := time.Now()

			inputs := map[string]string{"attribute": tc.attribute}
			for key, value := range tc.spoof {
				inputs[key] = value
			}
			step := requireCodePrompt(ts.T(), execute(ts.T(), ts.fixture.token(ts.T(), user),
				selfFlowStart(ts.fixture.flowHandle, inputs)), tc.verifyAction)
			ts.Equal(tc.channel, step.Data.AdditionalData["channel"], "the start must not change the branch")
			ts.awaitCodeOnlyOn(user, tc.attribute)

			resent, err := common.CompleteFlow(step.ExecutionID, tc.spoof, tc.resendAction, step.ChallengeToken)
			ts.Require().NoError(err)
			ts.Require().Equal(flowStatusIncomplete, resent.FlowStatus, "a resend must return to the prompt: %+v",
				resent.Error)
			ts.Require().True(common.HasAction(resent.Data.Actions, tc.verifyAction),
				"a resend must return to the pinned contact's branch")
			ts.Equal(tc.channel, resent.Data.AdditionalData["channel"], "the input must not change the branch")
			code := ts.awaitCodeOnlyOn(user, tc.attribute)

			done := submitCode(ts.T(), resent, tc.verifyAction, code)
			ts.Require().Equal(flowStatusComplete, done.FlowStatus, "the resent code must verify the pinned contact")
			requireVerifiedSince(ts.T(), user.id, tc.attribute, started)
			ts.Empty(verifiedAt(ts.T(), user.id, tc.other), "the other contact must not be verified")
		})
	}
}

// awaitCodeOnlyOn returns the code sent to the user's attribute on its channel, requires nothing on the
// other channel, and clears both.
func (ts *SelfVerificationMobileTestSuite) awaitCodeOnlyOn(user testUser, attribute string) string {
	ts.T().Helper()

	var code string
	if attribute == mobileAttribute {
		code = ts.sms.awaitCode(ts.T())
		ts.fixture.requireNoEmail(ts.T(), "a mobile number must be sent no email")
	} else {
		code = ts.fixture.awaitCode(ts.T(), user.email)
		ts.sms.requireNone(ts.T(), "an email must be sent no SMS")
	}

	ts.fixture.smtp.ClearEmails()
	ts.sms.server.ClearMessages()
	return code
}
