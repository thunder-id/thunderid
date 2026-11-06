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

const (
	adminInitiatedFlowType      = "ADMIN_INITIATED"
	adminVerificationFlowHandle = "default-admin-attribute-verification-flow"
	adminLinkSendNode           = "send_link"
	adminLinkTemplateHandle     = "admin-attribute-verification-email-link-flow"
)

// AdminVerificationFlowTestSuite drives the shipped administrator attribute verification flow.
type AdminVerificationFlowTestSuite struct {
	suite.Suite
	fixture    *verificationFixture
	flowHandle string
}

func TestAdminVerificationFlowTestSuite(t *testing.T) {
	suite.Run(t, new(AdminVerificationFlowTestSuite))
}

func (ts *AdminVerificationFlowTestSuite) SetupSuite() {
	ts.fixture = newVerificationFixture(ts.T(), "admin")
	ts.flowHandle = copyAdminFlow(ts.T(), "admin-"+ts.fixture.suffix, ts.fixture.senderID)
}

func (ts *AdminVerificationFlowTestSuite) SetupTest() {
	ts.fixture.smtp.ClearEmails()
}

// copyAdminFlow copies the shipped flow with senderID wired in and returns the copy's handle.
func copyAdminFlow(t *testing.T, name, senderID string) string {
	t.Helper()

	handle := "verification-" + name
	flowID, err := testutils.CopyFlowWithSenders(adminVerificationFlowHandle, adminInitiatedFlowType, handle,
		map[string]string{adminLinkSendNode: senderID}, nil)
	require.NoError(t, err, "failed to copy the shipped administrator verification flow")
	t.Cleanup(func() {
		if err := testutils.DeleteFlow(flowID); err != nil {
			t.Errorf("cleanup: failed to delete flow %s: %v", flowID, err)
		}
	})
	return handle
}

// adminFlowStart builds the start request for the flow the handle names.
func adminFlowStart(handle string, inputs map[string]string) map[string]interface{} {
	return map[string]interface{}{
		"flowType": adminInitiatedFlowType,
		"handle":   handle,
		"inputs":   inputs,
		"verbose":  true,
	}
}

// start starts the suite's flow copy for the user's email with the given token.
func (ts *AdminVerificationFlowTestSuite) start(token string, user testUser) *common.FlowExecutionResult {
	ts.T().Helper()

	return execute(ts.T(), token, adminFlowStart(ts.flowHandle,
		map[string]string{"subject": user.id, "attribute": emailAttribute}))
}

// sendLink has the administrator start the flow and returns the response and the emailed link.
func (ts *AdminVerificationFlowTestSuite) sendLink(user testUser) (*common.FlowExecutionResult, string) {
	ts.T().Helper()

	result := ts.start(adminToken(ts.T()), user)
	ts.Require().Equal(http.StatusOK, result.StatusCode, "the administrator's start must be accepted: %s",
		result.Body)
	ts.Require().Equal(flowStatusIncomplete, result.Step.FlowStatus, "the flow must wait once the link is sent: %s",
		result.Body)
	link := ts.fixture.awaitEmailTo(ts.T(), user.email).ExtractRecoveryLink()
	ts.Require().NotEmpty(link, "the email must carry the link")
	return result, link
}

// The administrator's start discloses neither the address nor the link; the user verifies from the link.
func (ts *AdminVerificationFlowTestSuite) TestTheUserVerifiesFromTheLinkTheAdministratorSent() {
	user := ts.fixture.newUser(ts.T(), "admin_link")
	started := time.Now()

	result, link := ts.sendLink(user)
	inviteToken := testutils.ExtractQueryParam(link, "inviteToken")
	ts.Require().NotEmpty(inviteToken, "the link must carry its token")
	ts.Contains(link, "/invite?", "the link must open the invite page")
	ts.Equal(result.Step.ExecutionID, testutils.ExtractQueryParam(link, "executionId"),
		"the link must continue the execution the administrator started")
	ts.Empty(result.Step.Data.Inputs, "the administrator is not asked for anything once the link is sent")
	ts.Equal("true", result.Step.Data.AdditionalData["emailSent"])
	ts.Equal(maskedEmail(user.email), result.Step.Data.AdditionalData["maskedDestination"])
	ts.NotContains(result.Step.Data.AdditionalData, "inviteLink", "the administrator must not get the link")
	requireNothingDisclosed(ts.T(), "start", result, []string{user.email, inviteToken}, nil)
	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "sending a link verifies nothing")

	confirm := openLink(ts.T(), link, inviteToken)
	ts.Require().Equal(flowStatusIncomplete, confirm.FlowStatus, "the link must lead to the confirmation: %+v",
		confirm)
	ts.Require().True(common.HasAction(confirm.Data.Actions, actionConfirm), "the user must be asked to confirm")
	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "opening the link alone must record nothing")

	done := execute(ts.T(), "", map[string]interface{}{
		"executionId": confirm.ExecutionID, "challengeToken": confirm.ChallengeToken,
		"action": actionConfirm, "verbose": true,
	})
	ts.Require().NotNil(done.Step, "the confirmation must be answered in the flow: %s", done.Body)
	ts.Require().Equal(flowStatusComplete, done.Step.FlowStatus, "confirming must verify the email: %s", done.Body)
	ts.Equal([]string{"verification_complete_heading", "verification_complete_message"},
		metaComponentIDs(ts.T(), done.Step), "the user must be told the email is verified")
	requireVerifiedSince(ts.T(), user.id, emailAttribute, started)
	ts.Empty(verifiedAt(ts.T(), user.id, mobileAttribute), "verifying the email must not verify the mobile number")
}

// The Console's administrator template flow sends the link and verifies on confirmation.
func (ts *AdminVerificationFlowTestSuite) TestTheConsoleTemplateVerifiesFromTheLink() {
	flow := consoleTemplateFlow(ts.T(), adminLinkTemplateHandle)
	ts.Require().Equal(adminInitiatedFlowType, flow.FlowType, "the template must create an administrator-initiated flow")
	flow.Handle = "verification-admin-template-" + ts.fixture.suffix
	ts.Require().NoError(testutils.SetFlowNodeProperty(&flow, adminLinkSendNode, "senderId", ts.fixture.senderID))
	createFlow(ts.T(), flow)
	user := ts.fixture.newUser(ts.T(), "admin_template")
	started := time.Now()

	result := execute(ts.T(), adminToken(ts.T()), adminFlowStart(flow.Handle,
		map[string]string{"subject": user.id, "attribute": emailAttribute}))
	ts.Require().Equal(http.StatusOK, result.StatusCode, "the administrator's start must be accepted: %s", result.Body)
	ts.Require().Equal(flowStatusIncomplete, result.Step.FlowStatus, "the flow must wait once the link is sent: %s",
		result.Body)
	email := ts.fixture.awaitEmailTo(ts.T(), user.email)
	ts.Contains(email.Body, "An administrator asked you to verify this email address",
		"the link must be sent with the admin-email-verification template")
	link := email.ExtractRecoveryLink()
	ts.Require().NotEmpty(link, "the email must carry the link")
	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "sending a link verifies nothing")

	done := confirmLink(ts.T(), link)
	ts.Require().Equal(flowStatusComplete, done.FlowStatus, "confirming must verify the email")
	requireVerifiedSince(ts.T(), user.id, emailAttribute, started)
}

// A wrong link token is refused and records nothing.
func (ts *AdminVerificationFlowTestSuite) TestAWrongLinkTokenRecordsNothing() {
	user := ts.fixture.newUser(ts.T(), "admin_wrong")
	_, link := ts.sendLink(user)

	refused := openLink(ts.T(), link, "019a0000-0000-7000-8000-000000000000")

	ts.NotEqual(flowStatusComplete, refused.FlowStatus, "a wrong token must not complete the flow")
	ts.Require().NotNil(refused.Error, "a wrong token must be refused")
	ts.False(common.HasAction(refused.Data.Actions, actionConfirm), "a wrong token must not reach confirmation")
	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "a wrong token must record nothing")
}

// A start without an administrator's token is refused before the handle is resolved.
func (ts *AdminVerificationFlowTestSuite) TestAStartWithoutAnAdministratorIsRefused() {
	user := ts.fixture.newUser(ts.T(), "admin_not_admin")
	other := ts.fixture.newUser(ts.T(), "admin_other")

	for name, token := range map[string]string{
		"no token":               "",
		"the user's token":       ts.fixture.token(ts.T(), user),
		"another user's token":   ts.fixture.token(ts.T(), other),
		"an altered admin token": alteredToken(adminToken(ts.T())),
	} {
		for _, handle := range []string{ts.flowHandle, "no-such-admin-flow"} {
			ts.Run(name+" for "+handle, func() {
				result := execute(ts.T(), token, adminFlowStart(handle,
					map[string]string{"subject": user.id, "attribute": emailAttribute}))

				requireStartRefused(ts.T(), result, http.StatusUnauthorized, errCodeAuthenticationRequired,
					"a start without an administrator's access token must be refused")
				ts.fixture.requireNoEmail(ts.T(), "nothing may be sent when the start is refused")
			})
		}
	}
	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "nothing may be recorded")
}

// An organization unit administrator lacks the root system permission and is refused.
func (ts *AdminVerificationFlowTestSuite) TestAnOrganizationUnitAdministratorIsRefused() {
	user := ts.fixture.newUser(ts.T(), "admin_ou_admin")
	token := ts.ouAdministratorToken("ou_admin")

	result := ts.start(token, user)

	requireStartRefused(ts.T(), result, http.StatusUnauthorized, errCodeAuthenticationRequired,
		"a token without the root system permission must be refused")
	ts.fixture.requireNoEmail(ts.T(), "nothing may be sent when the start is refused")
	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "nothing may be recorded")
}

// A start must name an administrator-initiated flow by handle.
func (ts *AdminVerificationFlowTestSuite) TestAStartMustNameAnAdminInitiatedFlow() {
	user := ts.fixture.newUser(ts.T(), "admin_handle")
	inputs := map[string]string{"subject": user.id, "attribute": emailAttribute}
	noHandle := adminFlowStart("", inputs)
	delete(noHandle, "handle")

	requireStartRefused(ts.T(), execute(ts.T(), adminToken(ts.T()), noHandle), http.StatusBadRequest,
		errCodeFlowHandleRequired, "a start without a handle must be refused")
	for name, handle := range map[string]string{
		"unknown handle": "no-such-admin-flow",
		"self flow":      verificationFlowHandle,
	} {
		ts.Run(name, func() {
			requireStartRefused(ts.T(), execute(ts.T(), adminToken(ts.T()), adminFlowStart(handle, inputs)),
				http.StatusBadRequest, errCodeFlowHandleNotFound, "a handle naming no administrator-initiated flow")
		})
	}
	ts.fixture.requireNoEmail(ts.T(), "nothing may be sent when the start is refused")
	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "nothing may be recorded")
}

// The flow cannot be started by flow ID.
func (ts *AdminVerificationFlowTestSuite) TestAStartByFlowIDIsRefused() {
	user := ts.fixture.newUser(ts.T(), "admin_by_id")
	flowID, err := testutils.GetFlowIDByHandle(adminVerificationFlowHandle, adminInitiatedFlowType)
	ts.Require().NoError(err, "the administrator attribute verification flow must ship")

	result := execute(ts.T(), adminToken(ts.T()), map[string]interface{}{
		"flowId": flowID,
		"inputs": map[string]string{"subject": user.id, "attribute": emailAttribute},
	})

	requireStartRefused(ts.T(), result, http.StatusForbidden, errCodeFlowIDNotPermitted,
		"an administrator-initiated flow ID must be refused")
	ts.fixture.requireNoEmail(ts.T(), "nothing may be sent when the start is refused")
}

// A suspended user is refused and nothing is sent.
func (ts *AdminVerificationFlowTestSuite) TestASuspendedUserIsRefusedBeforeSending() {
	user := ts.fixture.newUser(ts.T(), "admin_suspended")
	suspend(ts.T(), user)

	result := ts.start(adminToken(ts.T()), user)

	requireRefused(ts.T(), result, errCodeAccountSuspended, "preparation must refuse a suspended user")
	ts.fixture.requireNoEmail(ts.T(), "nothing may be sent to a suspended user")
	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "nothing may be recorded")
}

// A subject sent from the link cannot retarget the verification.
func (ts *AdminVerificationFlowTestSuite) TestTheLinkCannotRetargetTheVerification() {
	user := ts.fixture.newUser(ts.T(), "admin_target")
	other := ts.fixture.newUser(ts.T(), "admin_retarget")
	_, link := ts.sendLink(user)

	confirm, err := common.CompleteFlow(testutils.ExtractQueryParam(link, "executionId"),
		map[string]string{"inviteToken": testutils.ExtractQueryParam(link, "inviteToken"), "subject": other.id},
		"", "")
	ts.Require().NoError(err)
	ts.Require().True(common.HasAction(confirm.Data.Actions, actionConfirm), "the link must lead to the confirmation")
	done, err := common.CompleteFlow(confirm.ExecutionID, map[string]string{"subject": other.id}, actionConfirm,
		confirm.ChallengeToken)
	ts.Require().NoError(err)

	ts.Require().Equal(flowStatusComplete, done.FlowStatus, "confirming must verify the email: %+v", done.Error)
	ts.NotEmpty(verifiedAt(ts.T(), user.id, emailAttribute), "the user the administrator named must be verified")
	ts.Empty(verifiedAt(ts.T(), other.id, emailAttribute), "the user the continuation named must not be")
}

// Only the administrator's continuation can pass preparation after a prompt.
func (ts *AdminVerificationFlowTestSuite) TestOnlyTheAdministratorNamesTheUser() {
	handle := "verification-admin-prompted-" + ts.fixture.suffix
	createFlow(ts.T(), testutils.Flow{Name: "Attribute Verification Prompted Admin",
		FlowType: adminInitiatedFlowType, Handle: handle, Nodes: linearNodes(append(append([]map[string]interface{}{
			promptNode("prompt_continue", "action_continue"),
			taskNode("prepare_verification", "AttributeVerificationExecutor", "prepare")},
			emailCodeNodes(ts.fixture.senderID)...), recordNode(""))...)})
	user := ts.fixture.newUser(ts.T(), "admin_prompted")
	other := ts.fixture.newUser(ts.T(), "admin_prompted_other")

	for name, test := range map[string]struct {
		token   string
		subject string
	}{
		"no token, naming another user":          {"", other.id},
		"no token, naming the user":              {"", user.id},
		"the user's token, naming the user":      {ts.fixture.token(ts.T(), user), user.id},
		"another user's token, naming that user": {ts.fixture.token(ts.T(), other), other.id},
	} {
		ts.Run(name, func() {
			ts.fixture.smtp.ClearEmails()
			started := execute(ts.T(), adminToken(ts.T()), adminFlowStart(handle,
				map[string]string{"subject": user.id, "attribute": emailAttribute}))
			ts.Require().Equal(flowStatusIncomplete, started.Step.FlowStatus,
				"precondition: the flow waits at the prompt: %s", started.Body)

			result := execute(ts.T(), test.token, map[string]interface{}{
				"executionId": started.Step.ExecutionID, "challengeToken": started.Step.ChallengeToken,
				"action": "action_continue", "inputs": map[string]string{"subject": test.subject},
			})

			requireRefused(ts.T(), result, errCodeVerificationUnavailable,
				"a continuation without the administrator must not name the user")
			ts.fixture.requireNoEmail(ts.T(), "nothing may be sent")
		})
	}

	started := execute(ts.T(), adminToken(ts.T()), adminFlowStart(handle,
		map[string]string{"subject": user.id, "attribute": emailAttribute}))
	result := execute(ts.T(), adminToken(ts.T()), map[string]interface{}{
		"executionId": started.Step.ExecutionID, "challengeToken": started.Step.ChallengeToken,
		"action": "action_continue",
	})
	ts.Require().Equal(flowStatusIncomplete, result.Step.FlowStatus,
		"the administrator's continuation must pass preparation: %s", result.Body)
	ts.fixture.awaitEmailTo(ts.T(), user.email)
	ts.NotContains(string(result.Body), user.email, "the response must not carry the stored address")
}

// ouAdministratorToken returns a child-unit user's token holding system:user but not the root system permission.
func (ts *AdminVerificationFlowTestSuite) ouAdministratorToken(name string) string {
	t := ts.T()
	t.Helper()

	parent := ts.fixture.ouID
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "verification-" + name + "-" + ts.fixture.suffix, Name: "Attribute Verification " + name + " OU",
		Parent: &parent,
	})
	ts.Require().NoError(err, "create the child unit")
	var signInApps testutils.OUSignInApps
	t.Cleanup(func() {
		if err := testutils.DeleteOrganizationUnit(ouID); err != nil {
			t.Logf("cleanup: delete the child unit: %v", err)
		}
	})
	t.Cleanup(func() {
		if err := signInApps.DeleteAll(); err != nil {
			t.Logf("cleanup: delete the sign-in application: %v", err)
		}
	})

	username := fmt.Sprintf("verify_%s_%s", name, ts.fixture.suffix)
	password := "Correct-Horse-" + name + "1!"
	attributes, err := json.Marshal(map[string]interface{}{
		"username": username, "password": password, "email": username + "@example.com",
		"mobile_number": ts.fixture.uniqueMobile(),
	})
	ts.Require().NoError(err)
	userID, err := testutils.CreateUser(testutils.User{OUID: ouID, Type: ts.fixture.userType, Attributes: attributes})
	ts.Require().NoError(err, "create the administrator")
	t.Cleanup(func() {
		if err := testutils.DeleteUser(userID); err != nil {
			t.Logf("cleanup: delete the administrator: %v", err)
		}
	})

	identifier := "https://verification.example.com/" + name + "/" + ts.fixture.suffix
	rsID, err := testutils.CreateSystemScopedResourceServer(ouID, "Verification "+name+" RS", identifier, "user")
	ts.Require().NoError(err, "create the resource server")
	t.Cleanup(func() {
		if err := testutils.DeleteResourceServerWithChildren(rsID); err != nil {
			t.Logf("cleanup: delete the resource server: %v", err)
		}
	})
	roleID, err := testutils.CreateRole(testutils.Role{
		Name: "Verification " + name + " Admin", OUID: ouID,
		Assignments: []testutils.Assignment{{ID: userID, Type: "user"}},
		Permissions: []testutils.ResourcePermissions{{ResourceServerID: rsID, Permissions: []string{"system:user"}}},
	})
	ts.Require().NoError(err, "create the role")
	t.Cleanup(func() {
		if err := testutils.DeleteRole(roleID); err != nil {
			t.Logf("cleanup: delete the role: %v", err)
		}
	})

	token, err := signInApps.ObtainAccessToken(ouID, "system system:user", username, password, identifier)
	ts.Require().NoError(err, "sign the administrator in")
	return token.AccessToken
}
