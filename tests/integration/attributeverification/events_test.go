// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package attributeverification

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// Event types and data keys this file asserts. They mirror the server's event package, which a
// separate module cannot import; the names are the contract an event consumer reads.
const (
	eventVerificationRequested   = "ATTRIBUTE_VERIFICATION_REQUESTED"
	eventVerificationVerified    = "ATTRIBUTE_VERIFICATION_VERIFIED"
	eventVerificationRefused     = "ATTRIBUTE_VERIFICATION_REFUSED"
	eventVerificationInvalidated = "ATTRIBUTE_VERIFICATION_INVALIDATED"

	eventKeySubject   = "sub"
	eventKeyActorSub  = "act_sub"
	eventKeyAttribute = "attribute"
	eventKeyReason    = "reason"
)

// EventsTestSuite checks what preparation, recording and invalidation publish, read back from the
// observability file sink: the user, the attribute and the initiator, and never the contact value, the
// code or the link.
type EventsTestSuite struct {
	suite.Suite
	fixture  *verificationFixture
	sms      *smsProvider
	reader   *testutils.EventLogReader
	adminSub string
}

func TestEventsTestSuite(t *testing.T) {
	suite.Run(t, new(EventsTestSuite))
}

func (ts *EventsTestSuite) SetupSuite() {
	t := ts.T()
	original, err := testutils.ReadDeploymentConfigKey("observability")
	ts.Require().NoError(err, "failed to snapshot observability configuration")
	logPath := testutils.EventLogPath("attribute-verification-events.log")
	// Restore the original sink after the suite's resources are removed.
	t.Cleanup(func() {
		if err := testutils.PatchDeploymentConfig(map[string]interface{}{"observability": original}); err != nil {
			t.Errorf("cleanup: failed to restore observability: %v", err)
		} else if err := testutils.RestartServer(); err != nil {
			t.Errorf("cleanup: failed to restart the server after restoring observability: %v", err)
		} else if err := testutils.ObtainAdminAccessToken(); err != nil {
			t.Errorf("cleanup: failed to re-obtain the admin token: %v", err)
		}
	})
	ts.Require().NoError(testutils.PatchDeploymentConfig(testutils.ObservabilityFileSinkPatch(logPath, true)),
		"failed to enable the observability file sink")
	ts.Require().NoError(testutils.RestartServer(), "failed to restart with observability enabled")
	ts.Require().NoError(testutils.ObtainAdminAccessToken(), "failed to re-obtain the admin token")

	claims, err := testutils.DecodeJWT(adminToken(ts.T()))
	ts.Require().NoError(err, "the admin token must be a JWT")
	ts.Require().NotEmpty(claims.Sub, "the admin token must name its subject")
	ts.adminSub = claims.Sub

	ts.reader = testutils.NewEventLogReader(logPath)
	ts.fixture = newVerificationFixture(t, "events")
	ts.sms = newSMSProvider(t, "Events")
	ts.fixture.nameSender(t, mobileSendNode, ts.sms.senderID)
}

func (ts *EventsTestSuite) SetupTest() {
	ts.fixture.smtp.ClearEmails()
	ts.sms.server.ClearMessages()
}

// The self-service flow publishes the preparation and the recording with the user, the attribute and
// the user as the initiator, and never the address or the code.
func (ts *EventsTestSuite) TestTheSelfServiceFlowPublishesTheUserAsInitiator() {
	user := ts.fixture.newUser(ts.T(), "events_self")
	ts.Require().NoError(ts.reader.Reset())

	step := requireCodePrompt(ts.T(), ts.fixture.startVerification(ts.T(), ts.fixture.token(ts.T(), user),
		map[string]string{"attribute": emailAttribute}), actionEmailVerify)
	code := ts.fixture.awaitCode(ts.T(), user.email)
	requested := ts.awaitEvent(eventVerificationRequested, user.id)
	ts.Equal("success", requested.Status)
	ts.Equal(emailAttribute, requested.Str(eventKeyAttribute), "the attribute must be published")
	ts.Equal(user.id, requested.Str(eventKeyActorSub), "the user must be act_sub")

	ts.Require().Equal(flowStatusComplete, submitCode(ts.T(), step, actionEmailVerify, code).FlowStatus,
		"precondition: the code verifies the email")
	verified := ts.awaitEvent(eventVerificationVerified, user.id)
	ts.Equal("success", verified.Status)
	ts.Equal(emailAttribute, verified.Str(eventKeyAttribute), "the attribute must be published")
	ts.Equal(user.id, verified.Str(eventKeyActorSub), "the user must be act_sub")

	ts.requireNoSecretsPublished(user.email, code)
}

// A code for an email changed since the code was sent is refused at recording, and the refusal is
// published with the user and the attribute, and without either address or the code.
func (ts *EventsTestSuite) TestARefusedRecordingIsPublished() {
	user := ts.fixture.newUser(ts.T(), "events_refused")
	step := requireCodePrompt(ts.T(), ts.fixture.startVerification(ts.T(), ts.fixture.token(ts.T(), user),
		map[string]string{"attribute": emailAttribute}), actionEmailVerify)
	code := ts.fixture.awaitCode(ts.T(), user.email)
	changed := otherAddress(user)
	ts.fixture.adminUpdate(ts.T(), user, ts.fixture.userType, profile(user, changed))
	ts.Require().NoError(ts.reader.Reset())

	done := submitCode(ts.T(), step, actionEmailVerify, code)
	ts.Require().Equal(flowStatusError, done.FlowStatus, "precondition: the code for the earlier email is refused")

	refused := ts.awaitEvent(eventVerificationRefused, user.id)
	ts.Equal("failure", refused.Status)
	ts.Equal(emailAttribute, refused.Str(eventKeyAttribute), "the attribute must be published")
	ts.Equal(user.id, refused.Str(eventKeyActorSub), "the user must be act_sub")
	ts.requireNoSecretsPublished(user.email, changed, code)
}

// A write that clears a verified email is published once committed, with the reason and the caller
// of the write, and without the old or the new address.
func (ts *EventsTestSuite) TestAClearedVerificationIsPublishedWithItsReason() {
	user := ts.fixture.newUser(ts.T(), "events_cleared")
	ts.fixture.verifyEmail(ts.T(), user)
	changed := user
	changed.email = otherAddress(user)
	ts.Require().NoError(ts.reader.Reset())

	ts.fixture.adminUpdate(ts.T(), user, ts.fixture.userType, profile(user, changed.email))
	invalidated := ts.awaitEvent(eventVerificationInvalidated, user.id)
	ts.Equal("success", invalidated.Status)
	ts.Equal(emailAttribute, invalidated.Str(eventKeyAttribute), "the attribute must be published")
	ts.Equal("value_changed", invalidated.Str(eventKeyReason), "the reason must be published")
	ts.Equal(ts.adminSub, invalidated.Str(eventKeyActorSub), "the caller of the write must be act_sub")
	ts.requireNoSecretsPublished(user.email, changed.email)

	ts.fixture.verifyEmail(ts.T(), changed)
	ts.Require().NoError(ts.reader.Reset())
	ts.fixture.adminUpdate(ts.T(), user, ts.fixture.userType, map[string]interface{}{"username": user.username})
	removed := ts.awaitEvent(eventVerificationInvalidated, user.id)
	ts.Equal("removed", removed.Str(eventKeyReason), "the reason must be published")
	ts.requireNoSecretsPublished(changed.email)
}

// The mobile branch returns and publishes neither the number nor any code: not when a code is sent, a
// wrong one submitted, another sent or the right one submitted.
func (ts *EventsTestSuite) TestTheMobileFlowDisclosesNoNumberOrCode() {
	user := ts.fixture.newUser(ts.T(), "events_mobile")
	token := ts.fixture.token(ts.T(), user)
	ts.Require().NoError(ts.reader.Reset())

	started := ts.fixture.startVerification(ts.T(), token, map[string]string{"attribute": mobileAttribute})
	step := requireCodePrompt(ts.T(), started, actionMobileVerify)
	first := ts.sms.awaitCode(ts.T())
	wrong := continueRaw(ts.T(), step, actionMobileVerify, map[string]string{"otp": wrongCode(first)})
	ts.Require().Equal(flowStatusIncomplete, wrong.Step.FlowStatus, "precondition: a wrong code returns the prompt: %s",
		wrong.Body)
	resent := continueRaw(ts.T(), wrong.Step, actionMobileResend, nil)
	second := ts.sms.awaitCode(ts.T())
	done := continueRaw(ts.T(), resent.Step, actionMobileVerify, map[string]string{"otp": second})
	ts.Require().Equal(flowStatusComplete, done.Step.FlowStatus, "precondition: the code verifies the number: %s",
		done.Body)

	for name, result := range map[string]*common.FlowExecutionResult{
		"start": started, "wrong code": wrong, "resend": resent, "correct code": done,
	} {
		requireNothingDisclosed(ts.T(), name, result, []string{user.mobile}, []string{first, second})
	}
	ts.awaitEvent(eventVerificationVerified, user.id)
	ts.requireNoSecretsPublished(user.mobile, first, second)
}

// awaitEvent waits for an event of the type about the user.
func (ts *EventsTestSuite) awaitEvent(eventType, userID string) testutils.ObservabilityEvent {
	ts.T().Helper()

	evt, err := ts.reader.Await(testutils.EventOfTypeWith(eventType, eventKeySubject, userID))
	ts.Require().NoError(err, "no %s for %s was published", eventType, userID)
	return evt
}

// requireNoSecretsPublished requires that the data of no event read since the last reset carries any
// of the values. The data is checked rather than the whole record so that a numeric code cannot match
// a timestamp or an event ID. The caller first awaits the last event it expects, so the events before
// it have been read.
func (ts *EventsTestSuite) requireNoSecretsPublished(values ...string) {
	ts.T().Helper()

	events, err := ts.reader.Read()
	ts.Require().NoError(err)
	for _, evt := range events {
		for key, published := range evt.Data {
			for _, value := range values {
				ts.NotContains(fmt.Sprint(published), value,
					"%s must not carry a contact value, code or link in %s", evt.Type, key)
			}
		}
	}
}
