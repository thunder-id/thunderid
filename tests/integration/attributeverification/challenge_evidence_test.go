// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package attributeverification

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// actionRecordNow is the action of a called flow's prompt that would record its own pin.
const actionRecordNow = "action_record_now"

// ChallengeEvidenceTestSuite checks which challenge a recording accepts in self flows whose
// nodes run in other orders than the shipped flow's: only a code passed after the pin and sent since
// the pin to the pinned contact alone counts, and that evidence is shared with, not restored by, a
// called flow.
type ChallengeEvidenceTestSuite struct {
	suite.Suite
	fixture *verificationFixture
}

func TestChallengeEvidenceTestSuite(t *testing.T) {
	suite.Run(t, new(ChallengeEvidenceTestSuite))
}

func (ts *ChallengeEvidenceTestSuite) SetupSuite() {
	ts.fixture = newVerificationFixture(ts.T(), "evidence")
}

func (ts *ChallengeEvidenceTestSuite) SetupTest() {
	ts.fixture.smtp.ClearEmails()
}

// createSelfFlow creates a self flow from nodes until the test ends and returns its
// handle and ID.
func (ts *ChallengeEvidenceTestSuite) createSelfFlow(name string,
	nodes []map[string]interface{}) (string, string) {
	ts.T().Helper()

	handle := "verification-evidence-" + name + "-" + ts.fixture.suffix
	flowID := createFlow(ts.T(), testutils.Flow{Name: "Attribute Verification Evidence " + name,
		FlowType: selfFlowType, Handle: handle, Nodes: nodes})
	return handle, flowID
}

// start starts the self flow the handle names with the user's token.
func (ts *ChallengeEvidenceTestSuite) start(handle string, user testUser) *common.FlowExecutionResult {
	ts.T().Helper()

	result := execute(ts.T(), ts.fixture.token(ts.T(), user), selfFlowStart(handle, map[string]string{}))
	ts.Require().Equal(http.StatusOK, result.StatusCode, "the start must be accepted: %s", result.Body)
	return result
}

// giveEmail answers the prompt for an address with the user's stored one, and requires the flow to wait
// for the code it then emails there.
func (ts *ChallengeEvidenceTestSuite) giveEmail(step *common.FlowStep, user testUser) *common.FlowStep {
	ts.T().Helper()

	ts.Require().True(common.HasInput(step.Data.Inputs, emailAttribute),
		"precondition: the flow asks for the address: %+v", step.Data)
	next := continueRaw(ts.T(), step, actionSubmitEmail, map[string]string{emailAttribute: user.email})
	ts.Require().Equal(flowStatusIncomplete, next.Step.FlowStatus, "precondition: the flow waits for the code: %s",
		next.Body)
	ts.Require().True(common.HasInput(next.Step.Data.Inputs, "otp"), "precondition: the flow asks for the code: %s",
		next.Body)
	return next.Step
}

// The pin voids a code issued before it, so that code is answered as an invalid code and nothing is
// recorded.
func (ts *ChallengeEvidenceTestSuite) TestACodeIssuedBeforeThePinIsVoid() {
	code := emailCodeNodes(ts.fixture.senderID)
	handle, _ := ts.createSelfFlow("code_before_pin", linearNodes(
		promptNode("prompt_email", actionSubmitEmail, emailAttribute),
		code[0], code[1], prepareNode(emailAttribute), code[2], code[3], recordNode("")))
	user := ts.fixture.newUser(ts.T(), "code_before_pin")

	step := ts.giveEmail(ts.start(handle, user).Step, user)
	refused := submitCode(ts.T(), step, actionSubmitOTP, ts.fixture.awaitCode(ts.T(), user.email))

	ts.Require().Equal(flowStatusIncomplete, refused.FlowStatus, "a void code must return to the prompt")
	ts.Require().NotNil(refused.Error, "a void code must be reported")
	ts.Equal(errCodeInvalidOTP, refused.Error.Code, "the pin voids the code issued before it")
	ts.True(common.HasInput(refused.Data.Inputs, "otp"), "the prompt must ask for the code again")
	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "a code issued before the pin must record nothing")
}

// A code generated, sent and passed before the pin does not let the recording that follows the pin
// record anything.
func (ts *ChallengeEvidenceTestSuite) TestACodePassedBeforeThePinRecordsNothing() {
	handle, _ := ts.createSelfFlow("passed_before_pin", linearNodes(append(append(
		[]map[string]interface{}{promptNode("prompt_email", actionSubmitEmail, emailAttribute)},
		emailCodeNodes(ts.fixture.senderID)...), prepareNode(emailAttribute), recordNode(""))...))
	user := ts.fixture.newUser(ts.T(), "passed_before_pin")

	step := ts.giveEmail(ts.start(handle, user).Step, user)
	done := submitCode(ts.T(), step, actionSubmitOTP, ts.fixture.awaitCode(ts.T(), user.email))

	ts.Equal(flowStatusError, done.FlowStatus, "a code passed before the pin must not complete the flow")
	ts.Require().NotNil(done.Error, "the refusal must be reported")
	ts.Equal(errCodeVerificationRefused, done.Error.Code, "recording must refuse a code passed before the pin")
	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "a code passed before the pin must record nothing")
}

// A flow that reaches the recording after sending the code but without checking it records nothing.
func (ts *ChallengeEvidenceTestSuite) TestRecordingWithoutAPassedCodeIsRefused() {
	code := emailCodeNodes(ts.fixture.senderID)
	handle, _ := ts.createSelfFlow("no_check", linearNodes(
		prepareNode(emailAttribute), code[0], code[1], recordNode("")))
	user := ts.fixture.newUser(ts.T(), "no_check")

	result := ts.start(handle, user)
	ts.fixture.awaitCode(ts.T(), user.email)

	requireRefused(ts.T(), result, errCodeVerificationRefused, "recording must refuse a code nobody submitted")
	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "a code nobody submitted must record nothing")
}

// A code generated, sent and passed in a called flow after the caller pinned the email lets the
// caller's recording verify it once the called flow returns.
func (ts *ChallengeEvidenceTestSuite) TestAChallengePassedInACalledFlowIsRecordedByTheCaller() {
	_, calleeID := ts.createSelfFlow("callee_challenge", linearNodes(append(
		[]map[string]interface{}{promptNode("prompt_email", actionSubmitEmail, emailAttribute)},
		emailCodeNodes(ts.fixture.senderID)...)...))
	handle, _ := ts.createSelfFlow("caller_pin", linearNodes(
		prepareNode(emailAttribute), callNode("call_challenge", calleeID), recordNode("")))
	user := ts.fixture.newUser(ts.T(), "called_challenge")
	started := time.Now()

	step := ts.giveEmail(ts.start(handle, user).Step, user)
	done := submitCode(ts.T(), step, actionSubmitOTP, ts.fixture.awaitCode(ts.T(), user.email))

	ts.Require().Equal(flowStatusComplete, done.FlowStatus, "the caller must record once the called flow returns: %+v",
		done.Error)
	requireVerifiedSince(ts.T(), user.id, emailAttribute, started)
}

// A code passed in the caller does not survive a called flow that pins again: returning to the caller
// does not bring the earlier evidence back, and the caller's recording records nothing.
func (ts *ChallengeEvidenceTestSuite) TestACalledFlowsNewPinClearsTheCallersEvidence() {
	repin := promptNode("prompt_after_pin", actionContinue)
	repin["prompts"] = []map[string]interface{}{
		{"action": map[string]interface{}{"ref": actionContinue, "nextNode": "end"}},
		{"action": map[string]interface{}{"ref": actionRecordNow, "nextNode": "record_verification"}},
	}
	_, calleeID := ts.createSelfFlow("callee_repin", linearNodes(
		prepareNode(emailAttribute), repin, recordNode("")))
	handle, _ := ts.createSelfFlow("caller_passed", linearNodes(append(append(
		[]map[string]interface{}{prepareNode(emailAttribute)}, emailCodeNodes(ts.fixture.senderID)...),
		callNode("call_repin", calleeID), recordNode(""))...))
	user := ts.fixture.newUser(ts.T(), "called_repin")

	result := ts.start(handle, user)
	step := requireCodePrompt(ts.T(), result, actionSubmitOTP)
	pinned := submitCode(ts.T(), step, actionSubmitOTP, ts.fixture.awaitCode(ts.T(), user.email))
	ts.Require().Equal(flowStatusIncomplete, pinned.FlowStatus, "precondition: the called flow pins again: %+v",
		pinned.Error)
	ts.Require().True(common.HasAction(pinned.Data.Actions, actionContinue),
		"precondition: the called flow pinned and waits: %+v", pinned.Data)

	done := continueRaw(ts.T(), pinned, actionContinue, nil)

	requireRefused(ts.T(), done, errCodeVerificationRefused,
		"the caller's recording must not use evidence a later pin replaced")
	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "evidence a later pin replaced must record nothing")
}

// A code emailed after the pinned mobile number's SMS, to the address the email node resolves, passes
// the code check, but the recording refuses the proof: a send on the other channel is not the pinned
// contact and records nothing.
func (ts *ChallengeEvidenceTestSuite) TestAnEmailSentForAPinnedMobileNumberRecordsNothing() {
	sms := newSMSProvider(ts.T(), "evidence channel mismatch")
	mobile, email := mobileCodeNodes(sms.senderID), emailCodeNodes(ts.fixture.senderID)
	handle, _ := ts.createSelfFlow("channel_mismatch", linearNodes(prepareNode(mobileAttribute),
		mobile[0], mobile[1], email[0], email[1], mobile[2], mobile[3], recordNode("")))
	user := ts.fixture.newUser(ts.T(), "channel_mismatch")

	step := requireCodePrompt(ts.T(), ts.start(handle, user), actionSubmitOTP)
	sms.awaitMessage(ts.T())
	done := submitCode(ts.T(), step, actionSubmitOTP, ts.fixture.awaitCode(ts.T(), user.email))

	ts.Equal(flowStatusError, done.FlowStatus, "a proof delivered on the other channel must not complete the flow")
	ts.Require().NotNil(done.Error, "the refusal must be reported")
	ts.Equal(errCodeVerificationRefused, done.Error.Code,
		"recording must refuse a proof delivered on the other channel")
	ts.Empty(verifiedAt(ts.T(), user.id, mobileAttribute), "the mobile number must not be recorded")
	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "the email must not be recorded")
}

// An SMS sent after the pin, before the code emailed to the pinned address, makes the recording refuse
// the proof that code passes: every send since the pin must go to the pinned contact.
func (ts *ChallengeEvidenceTestSuite) TestASendToAnotherContactBeforeThePinnedOneRecordsNothing() {
	sms := newSMSProvider(ts.T(), "evidence earlier send")
	mobile, email := mobileCodeNodes(sms.senderID), emailCodeNodes(ts.fixture.senderID)
	handle, _ := ts.createSelfFlow("earlier_send", linearNodes(prepareNode(emailAttribute),
		mobile[0], mobile[1], email[0], email[1], email[2], email[3], recordNode("")))
	user := ts.fixture.newUser(ts.T(), "earlier_send")

	step := requireCodePrompt(ts.T(), ts.start(handle, user), actionSubmitOTP)
	sms.awaitMessage(ts.T())
	done := submitCode(ts.T(), step, actionSubmitOTP, ts.fixture.awaitCode(ts.T(), user.email))

	ts.Equal(flowStatusError, done.FlowStatus, "a proof also sent elsewhere since the pin must not complete the flow")
	ts.Require().NotNil(done.Error, "the refusal must be reported")
	ts.Equal(errCodeVerificationRefused, done.Error.Code,
		"recording must refuse a proof with a send to another contact since the pin")
	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "the email must not be recorded")
}

// callNode runs the flow flowID and continues once it returns.
func callNode(id, flowID string) map[string]interface{} {
	return map[string]interface{}{"id": id, "type": "CALL", "flow": map[string]interface{}{"ref": flowID}}
}
