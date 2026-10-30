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

// ChallengeEvidenceTestSuite checks which challenge a recording accepts in self-service flows whose
// nodes run in other orders than the shipped flow's: only a code issued after the pin, passed and
// delivered to the pinned address counts, that evidence is shared with, not restored by, a called
// flow, and a sender of one channel sends nothing to a contact pinned on the other. It also checks when
// a prompt with preselectAction offers its actions instead of taking one.
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

// createSelfServiceFlow creates a self-service flow from nodes until the test ends and returns its
// handle and ID.
func (ts *ChallengeEvidenceTestSuite) createSelfServiceFlow(name string,
	nodes []map[string]interface{}) (string, string) {
	ts.T().Helper()

	handle := "verification-evidence-" + name + "-" + ts.fixture.suffix
	flowID := createFlow(ts.T(), testutils.Flow{Name: "Attribute Verification Evidence " + name,
		FlowType: selfServiceFlowType, Handle: handle, Nodes: nodes})
	return handle, flowID
}

// start starts the self-service flow the handle names with the user's token.
func (ts *ChallengeEvidenceTestSuite) start(handle string, user testUser) *common.FlowExecutionResult {
	ts.T().Helper()

	result := execute(ts.T(), ts.fixture.token(ts.T(), user), selfServiceStart(handle, map[string]string{}))
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

// A code generated and sent before the pin, submitted after it, is refused as a wrong code, and nothing
// is recorded.
func (ts *ChallengeEvidenceTestSuite) TestACodeIssuedBeforeThePinIsRefused() {
	code := emailCodeNodes(ts.fixture.senderID)
	handle, _ := ts.createSelfServiceFlow("code_before_pin", linearNodes(
		promptNode("prompt_email", actionSubmitEmail, emailAttribute),
		code[0], code[1], prepareNode(emailAttribute), code[2], code[3], recordNode("")))
	user := ts.fixture.newUser(ts.T(), "code_before_pin")

	step := ts.giveEmail(ts.start(handle, user).Step, user)
	done := submitCode(ts.T(), step, actionSubmitOTP, ts.fixture.awaitCode(ts.T(), user.email))

	ts.Equal(flowStatusError, done.FlowStatus, "a code issued before the pin must not complete the flow")
	ts.Require().NotNil(done.Error, "the refusal must be reported")
	ts.Equal(errCodeInvalidOTP, done.Error.Code, "the code check must refuse a code issued before the pin")
	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "a code issued before the pin must record nothing")
}

// A code generated, sent and passed before the pin does not let the recording that follows the pin
// record anything.
func (ts *ChallengeEvidenceTestSuite) TestACodePassedBeforeThePinRecordsNothing() {
	handle, _ := ts.createSelfServiceFlow("passed_before_pin", linearNodes(append(append(
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
	handle, _ := ts.createSelfServiceFlow("no_check", linearNodes(
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
	_, calleeID := ts.createSelfServiceFlow("callee_challenge", linearNodes(append(
		[]map[string]interface{}{promptNode("prompt_email", actionSubmitEmail, emailAttribute)},
		emailCodeNodes(ts.fixture.senderID)...)...))
	handle, _ := ts.createSelfServiceFlow("caller_pin", linearNodes(
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
	_, calleeID := ts.createSelfServiceFlow("callee_repin", linearNodes(
		prepareNode(emailAttribute), repin, recordNode("")))
	handle, _ := ts.createSelfServiceFlow("caller_passed", linearNodes(append(append(
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

// A pinned mobile number that reaches an email node is refused there with nothing sent and nothing
// recorded.
func (ts *ChallengeEvidenceTestSuite) TestAnEmailNodeRefusesAPinnedMobileNumber() {
	handle, _ := ts.createSelfServiceFlow("channel_mismatch", linearNodes(
		taskNode("prepare_verification", "AttributeVerificationExecutor", "prepare"),
		emailCodeNodes(ts.fixture.senderID)[1], recordNode("")))
	user := ts.fixture.newUser(ts.T(), "channel_mismatch")

	result := execute(ts.T(), ts.fixture.token(ts.T(), user),
		selfServiceStart(handle, map[string]string{"attribute": mobileAttribute}))

	requireRefused(ts.T(), result, errCodeVerificationUnavailable,
		"an email node must refuse a contact of the other channel")
	ts.fixture.requireNoEmail(ts.T(), "nothing may be sent for a contact of the other channel")
	ts.Empty(verifiedAt(ts.T(), user.id, mobileAttribute), "nothing may be recorded")
}

// A preselectAction key that is not in runtime data is not read from the request's inputs, so the
// chooser waits with its actions.
func (ts *ChallengeEvidenceTestSuite) TestAnInputIsNotReadAsThePreselection() {
	handle, _ := ts.createSelfServiceFlow("chooser_input",
		chooserNodes(ts.fixture.senderID, "{{ctx(channelChoice)}}", "email", "mobile"))
	user := ts.fixture.newUser(ts.T(), "chooser_input")

	result := execute(ts.T(), ts.fixture.token(ts.T(), user),
		selfServiceStart(handle, map[string]string{"attribute": emailAttribute, "channelChoice": "email"}))

	ts.requireChooser(result, "an input must not preselect an action", "email", "mobile")
	ts.fixture.requireNoEmail(ts.T(), "nothing may be sent before an action is chosen")
}

// A preselectAction value that names none of the chooser's actions leaves it waiting with its actions.
func (ts *ChallengeEvidenceTestSuite) TestAValueNamingNoActionLeavesTheChoice() {
	handle, _ := ts.createSelfServiceFlow("chooser_unknown_ref",
		chooserNodes(ts.fixture.senderID, "{{ctx(contactType)}}", "first", "second"))
	user := ts.fixture.newUser(ts.T(), "chooser_unknown_ref")

	result := execute(ts.T(), ts.fixture.token(ts.T(), user),
		selfServiceStart(handle, map[string]string{"attribute": emailAttribute}))

	ts.requireChooser(result, "a value naming no action must not choose one", "first", "second")
	ts.fixture.requireNoEmail(ts.T(), "nothing may be sent before an action is chosen")
}

// When the preselected branch fails back to the chooser, the chooser waits with its actions and the
// error instead of taking that branch again.
func (ts *ChallengeEvidenceTestSuite) TestAFailedBranchReturnsToTheChoice() {
	nodes := chooserNodes(ts.fixture.senderID, "{{ctx(contactType)}}", "email", "mobile")
	for _, node := range nodes {
		if node["id"] == "send_email" {
			node["onFailure"] = "chooser"
		}
	}
	handle, _ := ts.createSelfServiceFlow("chooser_failed_branch", nodes)
	user := ts.fixture.newUser(ts.T(), "chooser_failed_branch")

	result := execute(ts.T(), ts.fixture.token(ts.T(), user),
		selfServiceStart(handle, map[string]string{"attribute": mobileAttribute}))

	step := ts.requireChooser(result, "a failed branch must return the chooser", "email", "mobile")
	ts.NotNil(step.Error, "the chooser must say why the branch stopped: %s", result.Body)
	ts.fixture.requireNoEmail(ts.T(), "the failed branch must have sent nothing")
	ts.Empty(verifiedAt(ts.T(), user.id, mobileAttribute), "nothing may be recorded")
}

// requireChooser requires the flow to wait on the chooser with each of its action refs.
func (ts *ChallengeEvidenceTestSuite) requireChooser(result *common.FlowExecutionResult,
	reason string, refs ...string) *common.FlowStep {
	ts.T().Helper()

	ts.Require().NotNil(result.Step, "the start must be answered in the flow: %s", result.Body)
	ts.Require().Equal(flowStatusIncomplete, result.Step.FlowStatus, "%s: %s", reason, result.Body)
	for _, ref := range refs {
		ts.True(common.HasAction(result.Step.Data.Actions, ref), "%s: action %s must be offered: %s", reason, ref,
			result.Body)
	}
	return result.Step
}

// chooserNodes pin the contact, then run a prompt preselecting from placeholder whose actions, one per
// ref, all lead to an email node sending through senderID, and then record.
func chooserNodes(senderID, placeholder string, refs ...string) []map[string]interface{} {
	send := emailCodeNodes(senderID)[1]
	prompts := make([]map[string]interface{}, 0, len(refs))
	for _, ref := range refs {
		prompts = append(prompts, map[string]interface{}{
			"inputs": []map[string]interface{}{},
			"action": map[string]interface{}{"ref": ref, "nextNode": send["id"]},
		})
	}
	chooser := map[string]interface{}{
		"id":         "chooser",
		"type":       "PROMPT",
		"properties": map[string]interface{}{"preselectAction": placeholder},
		"prompts":    prompts,
	}
	return linearNodes(taskNode("prepare_verification", "AttributeVerificationExecutor", "prepare"), chooser, send,
		recordNode(""))
}

// callNode runs the flow flowID and continues once it returns.
func callNode(id, flowID string) map[string]interface{} {
	return map[string]interface{}{"id": id, "type": "CALL", "flow": map[string]interface{}{"ref": flowID}}
}
