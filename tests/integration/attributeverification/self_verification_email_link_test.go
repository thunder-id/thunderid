// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package attributeverification

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// SelfVerificationEmailLinkTestSuite drives the flow the Console's Self Attribute Verification with
// Email Link template creates: the user starts it by its handle with their token, and the link
// continues it as Gate does, with no token.
type SelfVerificationEmailLinkTestSuite struct {
	suite.Suite
	fixture *verificationFixture
}

func TestSelfVerificationEmailLinkTestSuite(t *testing.T) {
	suite.Run(t, new(SelfVerificationEmailLinkTestSuite))
}

func (ts *SelfVerificationEmailLinkTestSuite) SetupSuite() {
	ts.fixture = newVerificationFixture(ts.T(), "link")
	ts.fixture.createLinkFlow(ts.T())
}

func (ts *SelfVerificationEmailLinkTestSuite) SetupTest() {
	ts.fixture.smtp.ClearEmails()
}

// The start sends a link to the stored email and waits with no inputs; the response shows a masked
// destination but neither the address, the link nor its token.
func (ts *SelfVerificationEmailLinkTestSuite) TestStartSendsALinkToTheStoredEmail() {
	user := ts.fixture.newUser(ts.T(), "link_start")

	result, link := ts.fixture.sendLink(ts.T(), user)
	inviteToken := testutils.ExtractQueryParam(link, "inviteToken")
	ts.Require().NotEmpty(inviteToken, "the link must carry its token")

	ts.Empty(result.Step.Data.Inputs, "the user is not asked for anything until the link is opened")
	ts.Equal("email", result.Step.Data.AdditionalData["channel"])
	ts.Equal("true", result.Step.Data.AdditionalData["emailSent"])
	ts.Equal(maskedEmail(user.email), result.Step.Data.AdditionalData["maskedDestination"])
	ts.NotContains(result.Step.Data.AdditionalData, "inviteLink", "the start must not return the link")
	ts.NotContains(string(result.Body), inviteToken, "the start must not return the link's token")
	ts.NotContains(string(result.Body), user.email, "the response must not carry the stored address")
	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "sending a link verifies nothing")
}

// Opening the link reaches the confirmation and records nothing; the confirm action, sent without the
// bearer token, verifies the email.
func (ts *SelfVerificationEmailLinkTestSuite) TestTheLinkAndTheConfirmationVerifyTheEmail() {
	user := ts.fixture.newUser(ts.T(), "link_confirm")
	started := time.Now()
	_, link := ts.fixture.sendLink(ts.T(), user)

	confirm := openLink(ts.T(), link, testutils.ExtractQueryParam(link, "inviteToken"))
	ts.Require().Equal(flowStatusIncomplete, confirm.FlowStatus, "the link must lead to the confirmation")
	ts.Require().True(common.HasAction(confirm.Data.Actions, actionConfirm), "the user must be asked to confirm")
	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "opening the link alone must record nothing")

	done, err := common.CompleteFlow(confirm.ExecutionID, nil, actionConfirm, confirm.ChallengeToken)
	ts.Require().NoError(err)
	ts.Equal(flowStatusComplete, done.FlowStatus, "confirming must complete the flow")
	ts.Nil(done.Error, "confirming must succeed")
	requireVerifiedSince(ts.T(), user.id, emailAttribute, started)
}

// A link with the wrong token does not reach the confirmation and records nothing.
func (ts *SelfVerificationEmailLinkTestSuite) TestAWrongLinkTokenRecordsNothing() {
	user := ts.fixture.newUser(ts.T(), "link_wrong")
	_, link := ts.fixture.sendLink(ts.T(), user)

	refused := openLink(ts.T(), link, "019a0000-0000-7000-8000-000000000000")

	ts.NotEqual(flowStatusComplete, refused.FlowStatus, "a wrong token must not complete the flow")
	ts.Require().NotNil(refused.Error, "a wrong token must be refused")
	ts.False(common.HasAction(refused.Data.Actions, actionConfirm), "a wrong token must not reach confirmation")
	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "a wrong token must record nothing")
}

// A user suspended after the link was sent is refused when the link holder confirms, and nothing is
// recorded. The continuation carries no bearer token, so the suspension's revocation does not stop
// the flow before recording.
func (ts *SelfVerificationEmailLinkTestSuite) TestAUserSuspendedAfterTheLinkIsSentIsRefusedOnConfirmation() {
	user := ts.fixture.newUser(ts.T(), "link_suspended_later")
	_, link := ts.fixture.sendLink(ts.T(), user)
	suspend(ts.T(), user)

	done := confirmLink(ts.T(), link)

	ts.Equal(flowStatusError, done.FlowStatus, "a suspended user's confirmation must be refused")
	ts.Require().NotNil(done.Error, "the refusal must be reported")
	ts.Equal(errCodeAccountSuspended, done.Error.Code, "recording must state the suspension")
	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "nothing may be recorded")
}
