// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package attributeverification

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// shippedCodeGenerationLimit is maxAttempts on the shipped flow's generate_email_otp node: the codes
// one execution may generate, the first included.
const shippedCodeGenerationLimit = 3

// SelfVerificationFlowTestSuite drives the shipped self attribute verification flow as a signed-in
// user would, from the bearer token to the recorded verification. The email node sends through the
// suite's provider; the SMS node keeps what it ships with, so the mobile branch has no provider.
type SelfVerificationFlowTestSuite struct {
	suite.Suite
	fixture *verificationFixture
	sms     *smsProvider
}

func TestSelfVerificationFlowTestSuite(t *testing.T) {
	suite.Run(t, new(SelfVerificationFlowTestSuite))
}

func (ts *SelfVerificationFlowTestSuite) SetupSuite() {
	ts.fixture = newVerificationFixture(ts.T(), "self")
	// A provider no node names, so an SMS the flow sent anyway would reach it.
	ts.sms = newSMSProvider(ts.T(), "Unnamed")
}

func (ts *SelfVerificationFlowTestSuite) SetupTest() {
	ts.fixture.smtp.ClearEmails()
	ts.sms.server.ClearMessages()
}

// startEmail starts the shipped flow for the user's email with their token and returns the code prompt.
func (ts *SelfVerificationFlowTestSuite) startEmail(user testUser) *common.FlowStep {
	ts.T().Helper()

	return requireCodePrompt(ts.T(), ts.fixture.startVerification(ts.T(), ts.fixture.token(ts.T(), user),
		map[string]string{"attribute": emailAttribute}), actionEmailVerify)
}

// The start sends a code to the stored email and returns the email branch's code prompt, which shows a
// masked destination but neither the address nor the code.
func (ts *SelfVerificationFlowTestSuite) TestStartSendsACodeToTheStoredEmail() {
	user := ts.fixture.newUser(ts.T(), "start")

	result := ts.fixture.startVerification(ts.T(), ts.fixture.token(ts.T(), user),
		map[string]string{"attribute": emailAttribute})
	step := requireCodePrompt(ts.T(), result, actionEmailVerify)
	code := ts.fixture.awaitCode(ts.T(), user.email)

	ts.Equal("email", step.Data.AdditionalData["channel"])
	ts.Equal("true", step.Data.AdditionalData["emailSent"])
	ts.Equal(maskedEmail(user.email), step.Data.AdditionalData["maskedDestination"])
	requireNothingDisclosed(ts.T(), "start", result, []string{user.email}, []string{code})
	ts.sms.requireNone(ts.T(), "an email verification must send no SMS")
	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "sending a code verifies nothing")
}

// The correct code, submitted without the bearer token, completes the flow, and the user's own read
// reports the email verified.
func (ts *SelfVerificationFlowTestSuite) TestTheCorrectCodeVerifiesTheEmail() {
	user := ts.fixture.newUser(ts.T(), "correct")
	token := ts.fixture.token(ts.T(), user)
	started := time.Now()

	step := requireCodePrompt(ts.T(),
		ts.fixture.startVerification(ts.T(), token, map[string]string{"attribute": emailAttribute}), actionEmailVerify)
	done := submitCode(ts.T(), step, actionEmailVerify, ts.fixture.awaitCode(ts.T(), user.email))

	ts.Require().Equal(flowStatusComplete, done.FlowStatus, "the correct code must complete the flow")
	requireVerifiedSince(ts.T(), user.id, emailAttribute, started)
	ts.Empty(verifiedAt(ts.T(), user.id, mobileAttribute), "verifying the email must not verify the mobile number")

	var self struct {
		VerifiedAttributes map[string]testutils.VerifiedAttribute `json:"verifiedAttributes"`
	}
	ts.Require().NoError(json.Unmarshal(readSelfRaw(ts.T(), token), &self))
	ts.NotEmpty(self.VerifiedAttributes[emailAttribute].VerifiedAt, "GET /users/me must report the verification")
}

// A wrong code returns the prompt with an error and records nothing; the correct code still works.
func (ts *SelfVerificationFlowTestSuite) TestAWrongCodeReturnsThePromptAndRecordsNothing() {
	user := ts.fixture.newUser(ts.T(), "wrong")
	started := time.Now()

	step := ts.startEmail(user)
	code := ts.fixture.awaitCode(ts.T(), user.email)

	refused := submitCode(ts.T(), step, actionEmailVerify, wrongCode(code))
	ts.Require().Equal(flowStatusIncomplete, refused.FlowStatus, "a wrong code must return to the prompt")
	ts.Require().NotNil(refused.Error, "a wrong code must be reported")
	ts.Equal(errCodeInvalidOTP, refused.Error.Code)
	ts.True(common.HasInput(refused.Data.Inputs, "otp"), "the prompt must ask for the code again")
	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "a wrong code must record nothing")

	done := submitCode(ts.T(), refused, actionEmailVerify, code)
	ts.Require().Equal(flowStatusComplete, done.FlowStatus, "the correct code must still complete the flow")
	requireVerifiedSince(ts.T(), user.id, emailAttribute, started)
}

// A correct code sent before the email changed records nothing, because the email it proves is no
// longer the stored one.
func (ts *SelfVerificationFlowTestSuite) TestACodeForAChangedEmailRecordsNothing() {
	user := ts.fixture.newUser(ts.T(), "changed")
	step := ts.startEmail(user)
	code := ts.fixture.awaitCode(ts.T(), user.email)
	ts.fixture.adminUpdate(ts.T(), user, ts.fixture.userType, profile(user, otherAddress(user)))

	done := submitCode(ts.T(), step, actionEmailVerify, code)

	ts.Equal(flowStatusError, done.FlowStatus, "a code for the earlier email must not complete the flow")
	ts.Require().NotNil(done.Error, "the refusal must be reported")
	ts.Equal(errCodeVerificationRefused, done.Error.Code, "recording must refuse a proof of the earlier email")
	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "a code for the earlier email must record nothing")
}

// Preparation refuses what it cannot verify before anything is sent: an attribute that is not a
// contact, one the user type does not have, and a contact with no stored value.
func (ts *SelfVerificationFlowTestSuite) TestAnIneligibleAttributeIsRefusedBeforeSending() {
	for name, tc := range map[string]struct{ user, attribute string }{
		"not a contact":         {"string_attribute", "displayName"},
		"not in the schema":     {"unknown_attribute", "workEmail"},
		"contact with no value": {"removed_email", emailAttribute},
	} {
		ts.Run(name, func() {
			ts.fixture.smtp.ClearEmails()
			user := ts.fixture.newUser(ts.T(), tc.user)
			attribute := tc.attribute
			// Only the email is removed; the other cases keep every attribute.
			if attribute == emailAttribute {
				ts.fixture.adminUpdate(ts.T(), user, ts.fixture.userType,
					map[string]interface{}{"username": user.username, "displayName": "No Email"})
			}

			result := ts.fixture.startVerification(ts.T(), ts.fixture.token(ts.T(), user),
				map[string]string{"attribute": attribute})

			requireRefused(ts.T(), result, errCodeVerificationRefused, "preparation must refuse the attribute")
			ts.fixture.requireNoEmail(ts.T(), "nothing may be sent for an ineligible attribute")
			ts.sms.requireNone(ts.T(), "nothing may be sent for an ineligible attribute")
		})
	}
}

// Preparation refuses an address that is not a top-level attribute before anything is sent: a list of
// addresses, an object holding one, and the address inside that object.
func (ts *SelfVerificationFlowTestSuite) TestAnAddressInsideAnArrayOrObjectIsRefusedBeforeSending() {
	schema := verificationUserTypeSchema()
	schema["workEmails"] = map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "email"}}
	schema["office"] = map[string]interface{}{"type": "object", "properties": map[string]interface{}{
		"email": map[string]interface{}{"type": "email"},
	}}
	_, userType := ts.fixture.newUserType(ts.T(), "nested_contacts", schema)
	user := ts.fixture.newUserOfType(ts.T(), "nested_contacts", userType)
	attributes := contacts(user, user.email, user.mobile)
	attributes["workEmails"] = []string{"work." + user.email}
	attributes["office"] = map[string]interface{}{"email": "office." + user.email}
	ts.fixture.adminUpdate(ts.T(), user, userType, attributes)
	token := ts.fixture.token(ts.T(), user)

	for _, attribute := range []string{"workEmails", "office", "office.email"} {
		ts.Run(attribute, func() {
			ts.fixture.smtp.ClearEmails()
			ts.sms.server.ClearMessages()

			result := ts.fixture.startVerification(ts.T(), token, map[string]string{"attribute": attribute})

			requireRefused(ts.T(), result, errCodeVerificationRefused,
				"preparation must refuse an address that is not a top-level attribute")
			ts.fixture.requireNoEmail(ts.T(), "nothing may be sent for a nested address")
			ts.sms.requireNone(ts.T(), "nothing may be sent for a nested address")
		})
	}
	ts.Empty(verifiedAttributes(ts.T(), user.id), "nothing may be recorded")
}

// Pinning the contact and sending the code write nothing to the user: the stored rows keep their
// modification times, and every read of the user is the same before and after.
func (ts *SelfVerificationFlowTestSuite) TestPreparationWritesNothingToTheUser() {
	user := ts.fixture.newUser(ts.T(), "prepare_no_write")
	// Signed in first, since a sign-in records itself on the user.
	token := ts.fixture.token(ts.T(), user)
	before := readUserRaw(ts.T(), user.id)
	beforeSelf := readSelfRaw(ts.T(), token)
	beforeRows := storedRows(ts.T(), user.id)

	requireCodePrompt(ts.T(), ts.fixture.startVerification(ts.T(), token, map[string]string{"attribute": emailAttribute}),
		actionEmailVerify)
	ts.fixture.awaitCode(ts.T(), user.email)

	ts.Equal(beforeRows, storedRows(ts.T(), user.id), "preparation must not write the user's rows")
	ts.JSONEq(string(before), string(readUserRaw(ts.T(), user.id)), "preparation must not change the user")
	ts.JSONEq(string(beforeSelf), string(readSelfRaw(ts.T(), token)),
		"preparation must not change the user's own read")
}

// Recording a verification moves the user's modification time and keeps the email indexed as an
// identifier, and the user still signs in with the email afterwards.
func (ts *SelfVerificationFlowTestSuite) TestRecordingMovesTheModificationTimeAndKeepsTheIdentifier() {
	user := ts.fixture.newUser(ts.T(), "record_identifier")
	// Signed in first, so the stored time read next is the one the recording must move.
	token := ts.fixture.token(ts.T(), user)
	before := entityUpdatedAt(ts.T(), user.id)

	step := requireCodePrompt(ts.T(), ts.fixture.startVerification(ts.T(), token,
		map[string]string{"attribute": emailAttribute}), actionEmailVerify)
	done := submitCode(ts.T(), step, actionEmailVerify, ts.fixture.awaitCode(ts.T(), user.email))
	ts.Require().Equal(flowStatusComplete, done.FlowStatus, "precondition: the user verifies their email")

	ts.NotEqual(before, entityUpdatedAt(ts.T(), user.id), "recording must move the user's modification time")
	ts.Equal("1", emailIdentifierRows(ts.T(), user), "recording must keep the email indexed as an identifier")
	signIn := taskNode("credentials_auth", "CredentialsAuthExecutor", "")
	signIn["executor"].(map[string]interface{})["inputs"] = []map[string]interface{}{
		{"ref": "input_email", "identifier": emailAttribute, "type": "EMAIL_INPUT", "required": true},
		{"ref": "input_password", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
	}
	flowID := createFlow(ts.T(), testutils.Flow{
		Name:     "Attribute Verification Email Sign-in",
		FlowType: "AUTHENTICATION",
		Handle:   "verification-email-sign-in-" + ts.fixture.suffix,
		Nodes:    linearNodes(signIn, taskNode("auth_assert", "AuthAssertExecutor", "")),
	})
	appID := createApp(ts.T(), testutils.Application{
		OUID:             ts.fixture.ouID,
		Name:             "Attribute Verification Email Sign-in App " + ts.fixture.suffix,
		ClientID:         "verification_email_sign_in_" + ts.fixture.suffix,
		ClientSecret:     "Verification-Email-Sign-In-Secret1!",
		AllowedUserTypes: []string{ts.fixture.userType},
		AuthFlowID:       flowID,
	})
	signedIn, err := common.InitiateAuthenticationFlow(appID, false,
		map[string]string{emailAttribute: user.email, "password": user.password}, "")
	ts.Require().NoError(err)

	ts.Require().Equal(flowStatusComplete, signedIn.FlowStatus, "the email must still identify the user: %+v",
		signedIn)
	claims, err := testutils.DecodeJWTPayloadMap(signedIn.Assertion)
	ts.Require().NoError(err)
	ts.Equal(user.id, claims["sub"], "the email must identify the user who verified it")
}

// entityUpdatedAt returns the modification time stored on the user's row.
func entityUpdatedAt(t *testing.T, userID string) string {
	t.Helper()

	updatedAt, err := testutils.QueryEntityDB(fmt.Sprintf(`SELECT UPDATED_AT FROM "ENTITY" WHERE ID = '%s';`, userID))
	require.NoError(t, err, "the user's row must be readable")
	require.NotEmpty(t, updatedAt, "the user must have a stored row")
	return updatedAt
}

// emailIdentifierRows returns how many identifier rows index the user's email.
func emailIdentifierRows(t *testing.T, user testUser) string {
	t.Helper()

	count, err := testutils.QueryEntityDB(fmt.Sprintf(`SELECT COUNT(*) FROM "ENTITY_IDENTIFIER" `+
		`WHERE ENTITY_ID = '%s' AND NAME = 'email' AND VALUE = '%s';`, user.id, user.email))
	require.NoError(t, err, "the identifier rows must be readable")
	return count
}

// An email link flow sends a link to the stored email; opening the link records nothing until the user
// confirms, and confirming without the bearer token verifies the email.
func (ts *SelfVerificationFlowTestSuite) TestAnEmailLinkVerifiesOnlyOnceConfirmed() {
	send := emailCodeNodes(ts.fixture.senderID)[1]
	send["properties"].(map[string]interface{})["emailTemplate"] = "password-recovery"
	verify := taskNode("verify_link", "InviteExecutor", "verify")
	verify["executor"].(map[string]interface{})["inputs"] = []map[string]interface{}{
		{"ref": "input_invite_token", "identifier": "inviteToken", "type": "HIDDEN", "required": true}}
	confirm := promptNode("confirm", actionConfirm)
	handle := "verification-email-link-" + ts.fixture.suffix
	createFlow(ts.T(), testutils.Flow{Name: "Attribute Verification Email Link", FlowType: selfServiceFlowType,
		Handle: handle, Nodes: linearNodes(
			taskNode("prepare_verification", "AttributeVerificationExecutor", "prepare"),
			taskNode("generate_link", "InviteExecutor", "generate"), send,
			map[string]interface{}{"id": "link_sent", "type": "PROMPT"},
			verify, confirm, recordNode(""))})
	user := ts.fixture.newUser(ts.T(), "email_link")
	started := time.Now()

	result := execute(ts.T(), ts.fixture.token(ts.T(), user),
		selfServiceStart(handle, map[string]string{"attribute": emailAttribute}))
	ts.Require().Equal(http.StatusOK, result.StatusCode, "the start must be accepted: %s", result.Body)
	ts.Require().Equal(flowStatusIncomplete, result.Step.FlowStatus, "the flow must wait once the link is sent")
	link := ts.fixture.awaitEmailTo(ts.T(), user.email).ExtractRecoveryLink()
	ts.Require().NotEmpty(link, "the email must carry the link")
	inviteToken := testutils.ExtractQueryParam(link, "inviteToken")
	ts.Require().NotEmpty(inviteToken, "the link must carry its token")
	requireNothingDisclosed(ts.T(), "start", result, []string{user.email, inviteToken}, nil)

	opened := openLink(ts.T(), link, inviteToken)

	ts.Require().Equal(flowStatusIncomplete, opened.FlowStatus, "the link must lead to the confirmation: %+v",
		opened)
	ts.Require().True(common.HasAction(opened.Data.Actions, actionConfirm), "the user must be asked to confirm")
	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "opening the link alone must record nothing")

	done, err := common.CompleteFlow(opened.ExecutionID, nil, actionConfirm, opened.ChallengeToken)
	ts.Require().NoError(err)

	ts.Require().Equal(flowStatusComplete, done.FlowStatus, "confirming must verify the email: %+v", done.Error)
	requireVerifiedSince(ts.T(), user.id, emailAttribute, started)
}

// A declarative user is refused at preparation, and nothing is sent.
func (ts *SelfVerificationFlowTestSuite) TestADeclarativeUserIsRefusedBeforeSending() {
	result := ts.fixture.startVerification(ts.T(), ts.fixture.token(ts.T(), declarativeUser),
		map[string]string{"attribute": emailAttribute})

	requireRefused(ts.T(), result, errCodeVerificationRefused, "preparation must refuse a declarative user")
	ts.fixture.requireNoEmail(ts.T(), "nothing may be sent to a declarative user")
	ts.Empty(verifiedAt(ts.T(), declarativeUser.id, emailAttribute), "nothing may be recorded")
}

// A user suspended after signing in is refused at preparation with the token they still hold, and
// nothing is sent.
func (ts *SelfVerificationFlowTestSuite) TestASuspendedUserIsRefusedBeforeSending() {
	user := ts.fixture.newUser(ts.T(), "suspended")
	token := ts.fixture.token(ts.T(), user)
	suspend(ts.T(), user)

	result := ts.fixture.startVerification(ts.T(), token, map[string]string{"attribute": emailAttribute})

	requireRefused(ts.T(), result, errCodeVerificationRefused, "preparation must refuse a suspended user")
	ts.fixture.requireNoEmail(ts.T(), "nothing may be sent to a suspended user")
	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "nothing may be recorded")
}

// A subject input naming another user is ignored: the code goes to the caller and only the caller is
// verified.
func (ts *SelfVerificationFlowTestSuite) TestASubjectInputIsIgnored() {
	caller := ts.fixture.newUser(ts.T(), "caller")
	other := ts.fixture.newUser(ts.T(), "other")
	started := time.Now()

	step := requireCodePrompt(ts.T(), ts.fixture.startVerification(ts.T(), ts.fixture.token(ts.T(), caller),
		map[string]string{"attribute": emailAttribute, "subject": other.id}), actionEmailVerify)
	ts.Equal(maskedEmail(caller.email), step.Data.AdditionalData["maskedDestination"])
	done := submitCode(ts.T(), step, actionEmailVerify, ts.fixture.awaitCode(ts.T(), caller.email))

	ts.Require().Equal(flowStatusComplete, done.FlowStatus)
	requireVerifiedSince(ts.T(), caller.id, emailAttribute, started)
	ts.Empty(verifiedAt(ts.T(), other.id, emailAttribute), "the user a subject input names must not be verified")
}

// An attribute or subject input on a resend and on the code submission does not move the pin: the
// resent code goes to the caller's email, and the code verifies only the caller's email.
func (ts *SelfVerificationFlowTestSuite) TestALaterAttributeOrSubjectInputDoesNotMoveThePin() {
	for _, input := range []string{"attribute", "subject"} {
		ts.Run(input, func() {
			ts.fixture.smtp.ClearEmails()
			ts.sms.server.ClearMessages()
			caller := ts.fixture.newUser(ts.T(), "later_"+input+"_caller")
			other := ts.fixture.newUser(ts.T(), "later_"+input+"_other")
			later := map[string]string{"attribute": mobileAttribute, "subject": other.id}[input]
			started := time.Now()

			step := ts.startEmail(caller)
			ts.fixture.awaitCode(ts.T(), caller.email)
			ts.fixture.smtp.ClearEmails()

			resent := continueRaw(ts.T(), step, actionEmailResend, map[string]string{input: later})
			ts.Require().Equal(flowStatusIncomplete, resent.Step.FlowStatus, "a resend must return to the prompt: %s",
				resent.Body)
			ts.Equal(maskedEmail(caller.email), resent.Step.Data.AdditionalData["maskedDestination"],
				"the resent code must go to the pinned email")
			code := ts.fixture.awaitCode(ts.T(), caller.email)
			ts.sms.requireNone(ts.T(), "a later attribute input must not send an SMS")

			done := continueRaw(ts.T(), resent.Step, actionEmailVerify, map[string]string{"otp": code, input: later})

			ts.Require().Equal(flowStatusComplete, done.Step.FlowStatus, "the code must verify the pinned email: %s",
				done.Body)
			requireVerifiedSince(ts.T(), caller.id, emailAttribute, started)
			ts.Empty(verifiedAt(ts.T(), caller.id, mobileAttribute), "a later attribute input must not be recorded")
			ts.Empty(verifiedAttributes(ts.T(), other.id), "a later subject input must not select its user")
		})
	}
}

// After a resend only the latest code counts: the first code is refused and the second verifies.
func (ts *SelfVerificationFlowTestSuite) TestAfterAResendTheFirstCodeIsRefused() {
	user := ts.fixture.newUser(ts.T(), "resend")
	started := time.Now()

	step := ts.startEmail(user)
	first := ts.fixture.awaitCode(ts.T(), user.email)
	ts.fixture.smtp.ClearEmails()

	resent := resend(ts.T(), step, actionEmailResend)
	ts.Require().Equal(flowStatusIncomplete, resent.FlowStatus, "a resend must return to the prompt")
	ts.Require().True(common.HasAction(resent.Data.Actions, actionEmailVerify),
		"a resend must return to the email branch's prompt")
	second := ts.fixture.awaitCode(ts.T(), user.email)
	if second == first {
		// Codes are random, so a resend can repeat one; the generation limit allows one more.
		ts.fixture.smtp.ClearEmails()
		resent = resend(ts.T(), resent, actionEmailResend)
		second = ts.fixture.awaitCode(ts.T(), user.email)
	}
	if second == first {
		ts.T().Skip("both resends repeated the first code, so its refusal cannot be observed")
	}

	refused := submitCode(ts.T(), resent, actionEmailVerify, first)
	ts.Require().NotNil(refused.Error, "the first code must be refused after a resend")
	ts.Equal(errCodeInvalidOTP, refused.Error.Code)
	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "the first code must record nothing")

	done := submitCode(ts.T(), refused, actionEmailVerify, second)
	ts.Require().Equal(flowStatusComplete, done.FlowStatus, "the latest code must complete the flow")
	requireVerifiedSince(ts.T(), user.id, emailAttribute, started)
}

// A resend or a code submission that carries another user's token keeps the user the flow started
// with: the code goes to the first user's email and only that user is verified.
func (ts *SelfVerificationFlowTestSuite) TestAnotherUsersTokenDoesNotChangeTheUser() {
	for _, continuation := range []string{"resend", "submission"} {
		ts.Run(continuation, func() {
			ts.fixture.smtp.ClearEmails()
			user := ts.fixture.newUser(ts.T(), "starter_"+continuation)
			other := ts.fixture.newUser(ts.T(), "intruder_"+continuation)
			otherToken := ts.fixture.token(ts.T(), other)
			started := time.Now()

			step := ts.startEmail(user)
			code := ts.fixture.awaitCode(ts.T(), user.email)
			if continuation == "resend" {
				ts.fixture.smtp.ClearEmails()
				resent := execute(ts.T(), otherToken, map[string]interface{}{
					"executionId": step.ExecutionID, "challengeToken": step.ChallengeToken,
					"action": actionEmailResend, "verbose": true,
				})
				ts.Require().NotNil(resent.Step, "the resend must be answered in the flow: %s", resent.Body)
				ts.Require().Equal(flowStatusIncomplete, resent.Step.FlowStatus,
					"a resend must return to the prompt: %s", resent.Body)
				ts.Equal(maskedEmail(user.email), resent.Step.Data.AdditionalData["maskedDestination"],
					"the resent code must go to the first user")
				step = resent.Step
				code = ts.fixture.awaitCode(ts.T(), user.email)
			}

			done := execute(ts.T(), otherToken, map[string]interface{}{
				"executionId": step.ExecutionID, "challengeToken": step.ChallengeToken,
				"action": actionEmailVerify, "inputs": map[string]string{"otp": code},
			})

			ts.Require().NotNil(done.Step, "the submission must be answered in the flow: %s", done.Body)
			ts.Require().Equal(flowStatusComplete, done.Step.FlowStatus, "the code must complete the flow: %s",
				done.Body)
			requireVerifiedSince(ts.T(), user.id, emailAttribute, started)
			ts.Empty(verifiedAt(ts.T(), other.id, emailAttribute), "the later token must not select its user")
		})
	}
}

// Verifying an email that is already verified records the time of the new verification.
func (ts *SelfVerificationFlowTestSuite) TestVerifyingAgainRefreshesTheTime() {
	user := ts.fixture.newUser(ts.T(), "again")
	ts.fixture.verifyEmail(ts.T(), user)
	first := requireVerifiedSince(ts.T(), user.id, emailAttribute, time.Time{})

	ts.fixture.verifyEmail(ts.T(), user)

	second := requireVerifiedSince(ts.T(), user.id, emailAttribute, first)
	ts.True(second.After(first), "a new verification must replace the earlier time")
}

// Resends share one execution's code generation limit: once it is reached, a resend sends nothing.
func (ts *SelfVerificationFlowTestSuite) TestResendsStopAtTheGenerationLimit() {
	user := ts.fixture.newUser(ts.T(), "resend_limit")
	step := ts.startEmail(user)
	ts.fixture.awaitCode(ts.T(), user.email)

	for generated := 1; generated < shippedCodeGenerationLimit; generated++ {
		ts.fixture.smtp.ClearEmails()
		resent := resend(ts.T(), step, actionEmailResend)
		ts.Require().Equal(flowStatusIncomplete, resent.FlowStatus, "resend %d is within the limit", generated)
		ts.fixture.awaitCode(ts.T(), user.email)
		step = resent
	}
	ts.fixture.smtp.ClearEmails()

	refused := resend(ts.T(), step, actionEmailResend)

	ts.Equal(flowStatusError, refused.FlowStatus, "a resend past the limit must be refused")
	ts.NotNil(refused.Error, "the flow must say why it stopped")
	ts.fixture.requireNoEmail(ts.T(), "no code may be sent past the limit")
}

// Without an SMS provider on the mobile branch, a mobile number fails at the SMS step with nothing
// sent and nothing recorded: as shipped, with the placeholder sender, and with a sender that does not
// exist.
func (ts *SelfVerificationFlowTestSuite) TestAMobileNumberFailsWithoutAnSMSProvider() {
	flowID, err := testutils.GetFlowIDByHandle(verificationFlowHandle, selfServiceFlowType)
	ts.Require().NoError(err)
	shipped, err := testutils.GetFlow(flowID)
	ts.Require().NoError(err)
	var shippedSender interface{}
	ts.Require().NoError(testutils.ModifyFlowNode(&shipped, mobileSendNode, func(node map[string]interface{}) {
		properties, _ := node["properties"].(map[string]interface{})
		shippedSender = properties["senderId"]
	}))
	ts.Require().Equal(senderPlaceholder, shippedSender, "precondition: the shipped SMS node names the placeholder")
	unknownSenderFlow := copyShippedFlow(ts.T(), "unknown-sms-sender-"+ts.fixture.suffix, map[string]string{
		emailSendNode: ts.fixture.senderID, mobileSendNode: "019a0000-0000-7000-8000-0000000000ff"})

	for name, tc := range map[string]struct{ user, handle string }{
		"placeholder":    {"sms_placeholder", ts.fixture.flowHandle},
		"unknown sender": {"sms_unknown_sender", unknownSenderFlow},
	} {
		ts.Run(name, func() {
			ts.fixture.smtp.ClearEmails()
			ts.sms.server.ClearMessages()
			user := ts.fixture.newUser(ts.T(), tc.user)

			result := execute(ts.T(), ts.fixture.token(ts.T(), user),
				selfServiceStart(tc.handle, map[string]string{"attribute": mobileAttribute}))

			requireRefused(ts.T(), result, errCodeSMSProviderNotConfigured,
				"the SMS step must fail without a provider")
			ts.sms.requireNone(ts.T(), "no SMS may be sent without a provider on the node")
			ts.fixture.requireNoEmail(ts.T(), "a mobile number must not take the email branch")
			ts.Empty(verifiedAt(ts.T(), user.id, mobileAttribute), "nothing may be recorded")
		})
	}
}

// A verification execution ends once its configured expiry has passed.
func (ts *SelfVerificationFlowTestSuite) TestAnExecutionLastsTheSelfServiceExpiry() {
	const expiry = 5 * time.Second
	useFlowConfigWith(ts.T(), selfServiceFlowKey, map[string]interface{}{"expirySeconds": int(expiry.Seconds())})
	user := ts.fixture.newUser(ts.T(), "expiry")
	started := time.Now()
	step := ts.startEmail(user)
	ts.fixture.awaitCode(ts.T(), user.email)

	// Each poll resumes the prompt, which neither answers it nor extends the execution.
	var expired *common.FlowExecutionResult
	ts.Require().Eventually(func() bool {
		result, err := common.ExecuteFlowWithToken("", map[string]interface{}{
			"executionId": step.ExecutionID, "challengeToken": step.ChallengeToken,
		})
		if err != nil {
			return false
		}
		if result.StatusCode == http.StatusOK && result.Step != nil {
			step = result.Step
			return false
		}
		expired = result
		return true
	}, time.Minute, 500*time.Millisecond, "the execution must end once the self-service expiry passes")
	lasted := time.Since(started)

	ts.Equal(http.StatusBadRequest, expired.StatusCode, "%s", expired.Body)
	ts.Require().NotNil(expired.Error, "%s", expired.Body)
	ts.Equal(errCodeExecutionNotFound, expired.Error.Code, "the execution must have expired: %s", expired.Body)
	ts.GreaterOrEqual(lasted, expiry-time.Second, "the execution must last the self-service expiry")
}

// The verification key takes an expiry and refuses a negative one without changing the stored layer.
func (ts *SelfVerificationFlowTestSuite) TestTheVerificationKeyTakesAnExpiry() {
	snapshotFlowSection(ts.T())

	status, body := putFlowSection(ts.T(), selfServiceFlowKey, map[string]interface{}{"expirySeconds": 600})
	ts.Require().Equal(http.StatusOK, status, "an expiry must be accepted: %s", body)
	var stored map[string]struct {
		ExpirySeconds int64 `json:"expirySeconds"`
	}
	ts.Require().NoError(json.Unmarshal(flowSectionLayer(ts.T()), &stored))
	ts.Equal(int64(600), stored[selfServiceFlowKey].ExpirySeconds, "the expiry must be stored")

	before := flowSectionLayer(ts.T())
	status, body = putFlowSection(ts.T(), selfServiceFlowKey, map[string]interface{}{"expirySeconds": -1})

	requireConfigRejected(ts.T(), before, status, body, "a negative expiry must be refused")
}

// Self-service selection requires a request handle, never an operation-specific default.
func (ts *SelfVerificationFlowTestSuite) TestTheVerificationDefaultHandleIsRejected() {
	snapshotFlowSection(ts.T())
	before := flowSectionLayer(ts.T())
	status, body := putFlowSection(ts.T(), selfServiceFlowKey,
		map[string]interface{}{"defaultHandle": verificationFlowHandle, "expirySeconds": 600})
	requireConfigRejected(ts.T(), before, status, body, "a verification default handle must be refused")
}

// snapshotFlowSection restores the flow section's writable layer as it is now when t ends.
func snapshotFlowSection(t *testing.T) {
	t.Helper()

	original := flowSectionLayer(t)
	t.Cleanup(func() {
		if err := testutils.PutWritableServerConfig(flowConfigSection, original); err != nil {
			t.Errorf("cleanup: failed to restore the flow configuration: %v", err)
		}
	})
}

// flowSectionLayer returns the flow section's writable layer as it is now.
func flowSectionLayer(t *testing.T) json.RawMessage {
	t.Helper()

	current, err := testutils.GetWritableServerConfig(flowConfigSection)
	require.NoError(t, err)
	return current
}

// putFlowSection writes the flow section's writable layer with value under key, keeping the other keys,
// and returns the status and body whatever they are.
func putFlowSection(t *testing.T, key string, value map[string]interface{}) (int, []byte) {
	t.Helper()

	section := map[string]interface{}{}
	require.NoError(t, json.Unmarshal(flowSectionLayer(t), &section))
	section[key] = value
	return sendJSON(t, testutils.GetHTTPClient(), http.MethodPut, "/server-config/"+flowConfigSection, section)
}

// requireConfigRejected requires a write to be refused as an invalid value and to leave the writable
// layer as it was before the write.
func requireConfigRejected(t *testing.T, before json.RawMessage, status int, body []byte, reason string) {
	t.Helper()

	require.Equal(t, http.StatusBadRequest, status, "%s: %s", reason, body)
	var apiErr struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(body, &apiErr))
	require.Equal(t, errCodeInvalidConfigValue, apiErr.Code, "%s: %s", reason, body)
	require.JSONEq(t, string(before), string(flowSectionLayer(t)), "a refused write must change nothing")
}
