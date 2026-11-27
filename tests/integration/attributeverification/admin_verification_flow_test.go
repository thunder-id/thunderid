// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package attributeverification

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
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
	agentOwnerFlowHandle        = "default-agent-owner-verification-flow"
	agentOwnerSMSSendNode       = "send_sms_link"
)

// AdminVerificationFlowTestSuite drives the shipped administrator attribute verification flow.
type AdminVerificationFlowTestSuite struct {
	suite.Suite
	fixture         *verificationFixture
	flowHandle      string
	ownerFlowHandle string
}

func TestAdminVerificationFlowTestSuite(t *testing.T) {
	suite.Run(t, new(AdminVerificationFlowTestSuite))
}

func (ts *AdminVerificationFlowTestSuite) SetupSuite() {
	ts.fixture = newVerificationFixture(ts.T(), "admin")
	ts.flowHandle = copyAdminFlow(ts.T(), "admin-"+ts.fixture.suffix, ts.fixture.senderID)
	ts.ownerFlowHandle = copyAgentOwnerFlow(ts.T(), "agent-owner-"+ts.fixture.suffix,
		map[string]string{adminLinkSendNode: ts.fixture.senderID})
}

func (ts *AdminVerificationFlowTestSuite) SetupTest() {
	ts.fixture.smtp.ClearEmails()
}

// copyAdminFlow copies the shipped flow with senderID wired in and returns the copy's handle.
func copyAdminFlow(t *testing.T, name, senderID string) string {
	t.Helper()

	return copyAdminFlowIn(t, name, "", senderID)
}

// copyAdminFlowIn is copyAdminFlow with the copy owned by ouID; an empty ouID keeps the default unit.
func copyAdminFlowIn(t *testing.T, name, ouID, senderID string) string {
	t.Helper()

	handle := "verification-" + name
	flowID, err := testutils.CopyFlowWithSendersInOU(adminVerificationFlowHandle, adminInitiatedFlowType, handle,
		ouID, map[string]string{adminLinkSendNode: senderID}, nil)
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

// start starts the suite's flow copy for the user with the given token.
func (ts *AdminVerificationFlowTestSuite) start(token string, user testUser) *common.FlowExecutionResult {
	ts.T().Helper()

	return execute(ts.T(), token, adminFlowStart(ts.flowHandle,
		map[string]string{"subject": user.id}))
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
		map[string]string{"subject": user.id}))
	ts.Require().Equal(http.StatusOK, result.StatusCode, "the administrator's start must be accepted: %s", result.Body)
	ts.Require().Equal(flowStatusIncomplete, result.Step.FlowStatus, "the flow must wait once the link is sent: %s",
		result.Body)
	email := ts.fixture.awaitEmailTo(ts.T(), user.email)
	ts.Contains(email.Body, "An administrator asked you to verify this email address",
		"the link must be sent with the admin-initiated-email-verification template")
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

	for name, test := range map[string]struct {
		token  string
		status int
		code   string
	}{
		"no token":               {"", http.StatusUnauthorized, errCodeAuthenticationRequired},
		"an altered admin token": {alteredToken(adminToken(ts.T())), http.StatusUnauthorized, errCodeAuthenticationRequired},
		"the user's token":       {ts.fixture.token(ts.T(), user), http.StatusForbidden, errCodeAdministrationPermission},
		"another user's token":   {ts.fixture.token(ts.T(), other), http.StatusForbidden, errCodeAdministrationPermission},
	} {
		for _, handle := range []string{ts.flowHandle, "no-such-admin-flow"} {
			ts.Run(name+" for "+handle, func() {
				result := execute(ts.T(), test.token, adminFlowStart(handle,
					map[string]string{"subject": user.id}))

				requireStartRefused(ts.T(), result, test.status, test.code,
					"a start without an administrator's access token must be refused")
				ts.fixture.requireNoEmail(ts.T(), "nothing may be sent when the start is refused")
			})
		}
	}
	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "nothing may be recorded")
}

// An organization unit administrator sends a link to a user of its own unit, and is refused for a suspended
// user outside it without learning of the suspension.
func (ts *AdminVerificationFlowTestSuite) TestAnOrganizationUnitAdministratorIsConfinedToItsUnit() {
	token, ouID := ts.ouAdministratorToken("ou_admin", "user")
	handle := copyAdminFlowIn(ts.T(), "admin-ou-"+ts.fixture.suffix, ouID, ts.fixture.senderID)
	outside := ts.fixture.newUser(ts.T(), "admin_ou_outside")
	suspend(ts.T(), outside)
	inside := ts.fixture.newUserIn(ts.T(), "admin_ou_inside", ouID)
	started := time.Now()

	refused := execute(ts.T(), token, adminFlowStart(handle, map[string]string{"subject": outside.id}))

	requireRefused(ts.T(), refused, errCodeVerificationRefused,
		"a user outside the administrator's unit must be refused without stating its suspension")
	ts.fixture.requireNoEmail(ts.T(), "nothing may be sent to a user outside the unit")
	ts.Empty(verifiedAt(ts.T(), outside.id, emailAttribute), "nothing may be recorded")

	sent := execute(ts.T(), token, adminFlowStart(handle, map[string]string{"subject": inside.id}))

	ts.Require().NotNil(sent.Step, "the start must run: %s", sent.Body)
	ts.Require().Equal(flowStatusIncomplete, sent.Step.FlowStatus, "a user of the unit is sent a link: %s",
		sent.Body)
	link := ts.fixture.awaitEmailTo(ts.T(), inside.email).ExtractRecoveryLink()
	ts.Require().NotEmpty(link, "the email must carry the link")
	ts.Empty(verifiedAt(ts.T(), inside.id, emailAttribute), "sending a link verifies nothing")

	done := confirmLink(ts.T(), link)

	ts.Require().Equal(flowStatusComplete, done.FlowStatus, "confirming must verify the email: %+v", done.Error)
	requireVerifiedSince(ts.T(), inside.id, emailAttribute, started)
	ts.Empty(verifiedAt(ts.T(), outside.id, emailAttribute), "the user outside the unit must stay unverified")
}

// An organization unit administrator cannot start a flow its unit neither owns nor was shared.
func (ts *AdminVerificationFlowTestSuite) TestAnOrganizationUnitAdministratorCannotStartAFlowNotSharedWithItsUnit() {
	token, ouID := ts.ouAdministratorToken("ou_admin_unshared", "user")
	inside := ts.fixture.newUserIn(ts.T(), "admin_ou_unshared", ouID)

	result := ts.start(token, inside)

	requireStartRefused(ts.T(), result, http.StatusForbidden, errCodeFlowNotSharedWithOU,
		"a flow the administrator's unit neither owns nor was shared must be refused")
	ts.fixture.requireNoEmail(ts.T(), "nothing may be sent when the flow is not shared with the unit")
	ts.Empty(verifiedAt(ts.T(), inside.id, emailAttribute), "nothing may be recorded")
}

// A link the email could not carry is not handed to the administrator in its place. A sender that no longer
// exists is the not-configured failure that hands the invite link to the screen in the other flow types.
func (ts *AdminVerificationFlowTestSuite) TestAFailedSendDoesNotHandTheAdministratorTheLink() {
	removed, err := testutils.CreateSMTPEmailProvider("Attribute Verification admin removed "+ts.fixture.suffix,
		"127.0.0.1", 2525, "noreply@thunderid.test")
	ts.Require().NoError(err, "create the sender to remove")
	ts.Require().NoError(testutils.DeleteNotificationSender(removed), "remove the sender the flow will name")
	handle := copyAdminFlow(ts.T(), "admin-failing-removed-"+ts.fixture.suffix, removed)
	user := ts.fixture.newUser(ts.T(), "admin_failed_removed")

	result := execute(ts.T(), adminToken(ts.T()), adminFlowStart(handle, map[string]string{"subject": user.id}))

	ts.Equal(http.StatusOK, result.StatusCode, "the failure must be answered in the flow: %s", result.Body)
	ts.Require().NotNil(result.Step, "the failure must be answered in the flow: %s", result.Body)
	ts.Equal("ERROR", result.Step.FlowStatus, "the flow must end in error: %s", result.Body)
	ts.Contains(string(result.Body), "FET-1038", "the failure must carry its code: %s", result.Body)
	ts.NotContains(string(result.Body), "inviteLink", "the administrator must not get the link")
	ts.NotContains(string(result.Body), "inviteToken", "the administrator must not get the token")
	ts.NotContains(string(result.Body), "executionId=", "the administrator must not get a link to the flow")
	ts.fixture.requireNoEmail(ts.T(), "nothing may be delivered")
	ts.Empty(verifiedAt(ts.T(), user.id, emailAttribute), "nothing may be recorded")
}

// A start must name an administrator-initiated flow by handle.
func (ts *AdminVerificationFlowTestSuite) TestAStartMustNameAnAdminInitiatedFlow() {
	user := ts.fixture.newUser(ts.T(), "admin_handle")
	inputs := map[string]string{"subject": user.id}
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
		"inputs": map[string]string{"subject": user.id},
	})

	requireStartRefused(ts.T(), result, http.StatusForbidden, errCodeFlowIDNotPermitted,
		"an administrator-initiated flow ID must be refused")
	ts.fixture.requireNoEmail(ts.T(), "nothing may be sent when the start is refused")
}

// A start naming a subject the flow cannot verify is refused and nothing is sent.
func (ts *AdminVerificationFlowTestSuite) TestAStartForASubjectItCannotVerifyIsRefusedBeforeSending() {
	suspended := ts.fixture.newUser(ts.T(), "admin_suspended")
	suspend(ts.T(), suspended)
	agentID := ts.newAgent("admin_agent")

	for name, test := range map[string]struct {
		subject string
		code    string
		checked string
	}{
		"a suspended user":   {suspended.id, errCodeAccountSuspended, suspended.id},
		"a missing subject":  {"019a0000-0000-7000-8000-000000000001", errCodeVerificationRefused, ""},
		"an agent":           {agentID, errCodeVerificationRefused, ""},
		"a declarative user": {declarativeUser.id, errCodeVerificationRefused, ""},
	} {
		ts.Run(name, func() {
			ts.fixture.smtp.ClearEmails()

			result := execute(ts.T(), adminToken(ts.T()), adminFlowStart(ts.flowHandle,
				map[string]string{"subject": test.subject}))

			requireRefused(ts.T(), result, test.code, "preparation must refuse "+name)
			ts.fixture.requireNoEmail(ts.T(), "nothing may be sent for "+name)
			if test.checked != "" {
				ts.Empty(verifiedAt(ts.T(), test.checked, emailAttribute), "nothing may be recorded")
			}
		})
	}
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

// A continuation after a prompt never names the user: the flow verifies the user bound at the start, and
// only the administrator's token continues it.
func (ts *AdminVerificationFlowTestSuite) TestAContinuationCannotNameAnotherUser() {
	handle := "verification-admin-prompted-" + ts.fixture.suffix
	createFlow(ts.T(), testutils.Flow{Name: "Attribute Verification Prompted Admin",
		FlowType: adminInitiatedFlowType, Handle: handle, Nodes: linearNodes(append(append([]map[string]interface{}{
			promptNode("prompt_continue", "action_continue"), prepareNode(emailAttribute)},
			emailCodeNodes(ts.fixture.senderID)...), recordNode(""))...)})
	user := ts.fixture.newUser(ts.T(), "admin_prompted")
	other := ts.fixture.newUser(ts.T(), "admin_prompted_other")

	for name, test := range map[string]struct {
		token   string
		subject string
		refused bool
	}{
		"no token, naming another user":          {"", other.id, false},
		"the administrator, naming another user": {adminToken(ts.T()), other.id, false},
		"the administrator, naming no one":       {adminToken(ts.T()), "", false},
		"the user's token, naming the user":      {ts.fixture.token(ts.T(), user), user.id, true},
		"another user's token, naming that user": {ts.fixture.token(ts.T(), other), other.id, true},
	} {
		ts.Run(name, func() {
			ts.fixture.smtp.ClearEmails()
			started := execute(ts.T(), adminToken(ts.T()), adminFlowStart(handle,
				map[string]string{"subject": user.id}))
			ts.Require().Equal(flowStatusIncomplete, started.Step.FlowStatus,
				"precondition: the flow waits at the prompt: %s", started.Body)
			request := map[string]interface{}{
				"executionId": started.Step.ExecutionID, "challengeToken": started.Step.ChallengeToken,
				"action": "action_continue",
			}
			if test.subject != "" {
				request["inputs"] = map[string]string{"subject": test.subject}
			}

			result := execute(ts.T(), test.token, request)

			if test.refused {
				requireStartRefused(ts.T(), result, http.StatusForbidden, errCodeFlowStartedByAnotherCaller,
					"another caller's token must not continue the administrator's flow")
				ts.fixture.requireNoEmail(ts.T(), "nothing may be sent")
				return
			}
			ts.Require().NotNil(result.Step, "the continuation must be answered in the flow: %s", result.Body)
			ts.Require().Equal(flowStatusIncomplete, result.Step.FlowStatus,
				"the continuation must pass preparation for the bound user: %s", result.Body)
			ts.fixture.awaitEmailTo(ts.T(), user.email)
			ts.fixture.requireNoEmail(ts.T(), "only the user bound at the start may be emailed")
			ts.NotContains(string(result.Body), user.email, "the response must not carry the stored address")
		})
	}
}

// newAgent creates an agent with a stored email in an agent type of its own in the fixture's unit and
// returns its ID, so only its category keeps a verification from preparing.
func (ts *AdminVerificationFlowTestSuite) newAgent(name string) string {
	t := ts.T()
	t.Helper()

	handle := "verification-agent-" + ts.fixture.suffix
	typeID, err := testutils.CreateNewAgentType(testutils.UserType{
		Handle: handle, DisplayName: "Attribute Verification Agent", OUID: ts.fixture.ouID,
		Schema: map[string]interface{}{
			"name":  map[string]interface{}{"type": "string", "required": true, "unique": true},
			"email": map[string]interface{}{"type": "email"},
		},
	})
	ts.Require().NoError(err, "create the agent type")
	t.Cleanup(func() {
		if err := testutils.DeleteAgentType(typeID); err != nil {
			t.Logf("cleanup: delete the agent type: %v", err)
		}
	})
	agentID, err := testutils.CreateAgent(testutils.Agent{
		OUID: ts.fixture.ouID, Type: handle,
		Attributes: map[string]interface{}{"name": name + "-" + ts.fixture.suffix,
			"email": name + "." + ts.fixture.suffix + "@example.com"},
	})
	ts.Require().NoError(err, "create the agent")
	t.Cleanup(func() {
		if err := testutils.DeleteAgent(agentID); err != nil {
			t.Logf("cleanup: delete the agent: %v", err)
		}
	})
	return agentID
}

// ouAdministratorToken returns a child-unit user's token holding org:<resource> but not the root system
// permission, and that unit.
func (ts *AdminVerificationFlowTestSuite) ouAdministratorToken(name, resource string) (string, string) {
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
	rsID, err := testutils.CreateOrgScopedResourceServer(ouID, "Verification "+name+" RS", identifier, resource)
	ts.Require().NoError(err, "create the resource server")
	t.Cleanup(func() {
		if err := testutils.DeleteResourceServerWithChildren(rsID); err != nil {
			t.Logf("cleanup: delete the resource server: %v", err)
		}
	})
	roleID, err := testutils.CreateRole(testutils.Role{
		Name: "Verification " + name + " Admin", OUID: ouID,
		Assignments: []testutils.Assignment{{ID: userID, Type: "user"}},
		Permissions: []testutils.ResourcePermissions{{ResourceServerID: rsID, Permissions: []string{"org:" + resource}}},
	})
	ts.Require().NoError(err, "create the role")
	t.Cleanup(func() {
		if err := testutils.DeleteRole(roleID); err != nil {
			t.Logf("cleanup: delete the role: %v", err)
		}
	})

	token, err := signInApps.ObtainAccessToken(ouID, "system org:"+resource, username, password, identifier)
	ts.Require().NoError(err, "sign the administrator in")
	return token.AccessToken, ouID
}

// The Console reads which flow to start from adminAttributeVerificationFlow, and for an agent's owner from
// adminAgentOwnerVerificationFlow, so bootstrap names the shipped flows there.
func (ts *AdminVerificationFlowTestSuite) TestServerConfigNamesTheShippedFlow() {
	resp, err := testutils.GetHTTPClient().Get(testutils.TestServerURL + "/server-config/flow")
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode)

	var layers struct {
		Merged struct {
			AdminAttributeVerificationFlow struct {
				DefaultHandle string `json:"defaultHandle"`
			} `json:"adminAttributeVerificationFlow"`
			AdminAgentOwnerVerificationFlow struct {
				DefaultHandle string `json:"defaultHandle"`
			} `json:"adminAgentOwnerVerificationFlow"`
		} `json:"merged"`
	}
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&layers))
	ts.Equal(adminVerificationFlowHandle, layers.Merged.AdminAttributeVerificationFlow.DefaultHandle)
	ts.Equal(agentOwnerFlowHandle, layers.Merged.AdminAgentOwnerVerificationFlow.DefaultHandle)
}

// The setting takes an administrator-initiated flow and refuses one of any other type.
func (ts *AdminVerificationFlowTestSuite) TestServerConfigAcceptsOnlyAdminInitiatedFlows() {
	original, err := testutils.GetWritableServerConfig("flow")
	ts.Require().NoError(err)
	ts.T().Cleanup(func() {
		ts.NoError(testutils.PutWritableServerConfig("flow", original))
	})

	withHandle := func(handle string) []byte {
		section := map[string]interface{}{}
		ts.Require().NoError(json.Unmarshal(original, &section))
		section["adminAttributeVerificationFlow"] = map[string]string{"defaultHandle": handle}
		body, err := json.Marshal(section)
		ts.Require().NoError(err)
		return body
	}

	status, body, err := testutils.PutWritableServerConfigWithStatus("flow", withHandle(ts.flowHandle))
	ts.Require().NoError(err)
	ts.Equal(http.StatusOK, status, string(body))

	status, body, err = testutils.PutWritableServerConfigWithStatus("flow", withHandle(userSuspendFlowHandle))
	ts.Require().NoError(err)
	ts.Equal(http.StatusBadRequest, status, string(body))
}

// copyAgentOwnerFlow copies the shipped agent owner verification flow with the given senders wired in and returns
// the copy's handle.
func copyAgentOwnerFlow(t *testing.T, name string, senders map[string]string) string {
	t.Helper()

	return copyAgentOwnerFlowIn(t, name, "", senders)
}

// copyAgentOwnerFlowIn is copyAgentOwnerFlow with the copy owned by ouID; an empty ouID keeps the default unit.
func copyAgentOwnerFlowIn(t *testing.T, name, ouID string, senders map[string]string) string {
	t.Helper()

	handle := "verification-" + name
	flowID, err := testutils.CopyFlowWithSendersInOU(agentOwnerFlowHandle, adminInitiatedFlowType, handle, ouID,
		senders, nil)
	require.NoError(t, err, "failed to copy the shipped agent owner verification flow")
	t.Cleanup(func() {
		if err := testutils.DeleteFlow(flowID); err != nil {
			t.Errorf("cleanup: failed to delete flow %s: %v", flowID, err)
		}
	})
	return handle
}

// defaultAgentType returns the handle and organization unit of the shipped default agent type.
func defaultAgentType(t *testing.T) (string, string) {
	t.Helper()

	status, body := sendJSON(t, testutils.GetHTTPClient(), http.MethodGet, "/agent-types?limit=100", nil)
	require.Equal(t, http.StatusOK, status, "the agent types must be listed: %s", body)
	var list struct {
		Types []struct {
			Handle string `json:"handle"`
			OUID   string `json:"ouId"`
		} `json:"types"`
	}
	require.NoError(t, json.Unmarshal(body, &list))
	for _, agentType := range list.Types {
		if agentType.Handle == "default" {
			return agentType.Handle, agentType.OUID
		}
	}
	require.FailNow(t, "the default agent type must ship")
	return "", ""
}

// agentBody is a create or update of an agent of the default type with the given name and owner.
func agentBody(t *testing.T, name string, owner *testutils.AgentOwner) map[string]interface{} {
	t.Helper()

	agentType, ouID := defaultAgentType(t)
	return map[string]interface{}{"ouId": ouID, "type": agentType, "attributes": map[string]string{"name": name},
		"owner": owner}
}

// newOwnedAgent creates an agent with the given owner and returns its ID. The agent is deleted when the
// test ends.
func (ts *AdminVerificationFlowTestSuite) newOwnedAgent(name string, owner *testutils.AgentOwner) string {
	ts.T().Helper()

	return ts.createAgent(agentBody(ts.T(), name+"-"+ts.fixture.suffix, owner))
}

// createAgent creates the agent the body describes and returns its ID. The agent is deleted when the test ends.
func (ts *AdminVerificationFlowTestSuite) createAgent(agent map[string]interface{}) string {
	ts.T().Helper()

	status, body := sendJSON(ts.T(), testutils.GetHTTPClient(), http.MethodPost, "/agents", agent)
	ts.Require().Equal(http.StatusCreated, status, "the agent must be created: %s", body)
	var created struct {
		ID string `json:"id"`
	}
	ts.Require().NoError(json.Unmarshal(body, &created))
	ts.T().Cleanup(func() {
		if status, body := sendJSON(ts.T(), testutils.GetHTTPClient(), http.MethodDelete, "/agents/"+created.ID,
			nil); status != http.StatusNoContent {
			ts.T().Logf("cleanup: failed to delete agent %s: %d %s", created.ID, status, body)
		}
	})
	return created.ID
}

// ownerVerifiedAt returns when the agent's owner was verified, as the agent API reports it, or "".
func ownerVerifiedAt(t *testing.T, agentID string) string {
	t.Helper()

	status, body := sendJSON(t, testutils.GetHTTPClient(), http.MethodGet, "/agents/"+agentID, nil)
	require.Equal(t, http.StatusOK, status, "the agent must be read: %s", body)
	var agent struct {
		Owner *struct {
			VerifiedAt string `json:"verifiedAt"`
		} `json:"owner"`
	}
	require.NoError(t, json.Unmarshal(body, &agent))
	require.NotNil(t, agent.Owner, "the agent must have an owner: %s", body)
	return agent.Owner.VerifiedAt
}

// startOwnerVerification has the administrator start the agent owner flow for the agent.
func (ts *AdminVerificationFlowTestSuite) startOwnerVerification(agentID string) *common.FlowExecutionResult {
	ts.T().Helper()

	return execute(ts.T(), adminToken(ts.T()), adminFlowStart(ts.ownerFlowHandle,
		map[string]string{"subject": agentID}))
}

// verifyOwner sends the agent owner flow's link to the owner and confirms it.
func (ts *AdminVerificationFlowTestSuite) verifyOwner(agentID, ownerEmail string) {
	ts.T().Helper()

	result := ts.startOwnerVerification(agentID)
	ts.Require().Equal(http.StatusOK, result.StatusCode, "the administrator's start must be accepted: %s",
		result.Body)
	link := ts.fixture.awaitEmailTo(ts.T(), ownerEmail).ExtractRecoveryLink()
	ts.Require().NotEmpty(link, "the email must carry the link")
	ts.Require().Equal(flowStatusComplete, confirmLink(ts.T(), link).FlowStatus, "confirming must verify the owner")
}

// An agent's owner managed in another identity system, with no account here, verifies its email address from
// the link the administrator had sent, and the agent reports its owner verified.
func (ts *AdminVerificationFlowTestSuite) TestTheAgentsExternalOwnerVerifiesFromTheLink() {
	ownerEmail := "agent-owner-" + ts.fixture.suffix + "@example.com"
	agentID := ts.newOwnedAgent("owner-link", testutils.ExternalAgentOwner(ownerEmail, "email"))
	started := time.Now()

	result := ts.startOwnerVerification(agentID)
	ts.Require().Equal(http.StatusOK, result.StatusCode, "the administrator's start must be accepted: %s",
		result.Body)
	ts.Require().Equal(flowStatusIncomplete, result.Step.FlowStatus, "the flow must wait once the link is sent: %s",
		result.Body)
	ts.Equal(maskedEmail(ownerEmail), result.Step.Data.AdditionalData["maskedDestination"])
	ts.NotContains(result.Step.Data.AdditionalData, "inviteLink", "the administrator must not get the link")
	email := ts.fixture.awaitEmailTo(ts.T(), ownerEmail)
	ts.Contains(email.Body, "An administrator named this email address as the owner of an agent",
		"the link must be sent with the agent-owner-verification template")
	link := email.ExtractRecoveryLink()
	ts.Require().NotEmpty(link, "the email must carry the link")
	requireNothingDisclosed(ts.T(), "start", result, []string{ownerEmail,
		testutils.ExtractQueryParam(link, "inviteToken")}, nil)
	ts.Empty(ownerVerifiedAt(ts.T(), agentID), "sending a link verifies nothing")

	done := confirmLink(ts.T(), link)
	ts.Require().Equal(flowStatusComplete, done.FlowStatus, "confirming must verify the owner")
	verified, err := time.Parse(time.RFC3339Nano, ownerVerifiedAt(ts.T(), agentID))
	ts.Require().NoError(err, "the agent must report when its owner was verified")
	ts.False(verified.Before(started.Add(-time.Second)), "the verification must be the one just made")
}

// An update that keeps the owner keeps its verification; one that changes the owner removes it.
func (ts *AdminVerificationFlowTestSuite) TestChangingTheAgentsOwnerRemovesItsVerification() {
	ownerEmail := "agent-owner-change-" + ts.fixture.suffix + "@example.com"
	owner := testutils.ExternalAgentOwner(ownerEmail, "email")
	agentID := ts.newOwnedAgent("owner-change", owner)
	ts.verifyOwner(agentID, ownerEmail)
	ts.Require().NotEmpty(ownerVerifiedAt(ts.T(), agentID))

	status, body := sendJSON(ts.T(), testutils.GetHTTPClient(), http.MethodPut, "/agents/"+agentID,
		agentBody(ts.T(), "owner-change-renamed-"+ts.fixture.suffix, owner))
	ts.Require().Equal(http.StatusOK, status, "the agent must be updated: %s", body)
	ts.NotEmpty(ownerVerifiedAt(ts.T(), agentID), "an update that keeps the owner must keep its verification")

	status, body = sendJSON(ts.T(), testutils.GetHTTPClient(), http.MethodPut, "/agents/"+agentID,
		agentBody(ts.T(), "owner-change-renamed-"+ts.fixture.suffix,
			testutils.ExternalAgentOwner("other-"+ownerEmail, "email")))
	ts.Require().Equal(http.StatusOK, status, "the agent must be updated: %s", body)
	ts.Empty(ownerVerifiedAt(ts.T(), agentID), "a new owner must not inherit the old owner's verification")
}

// A link sent to an owner the agent no longer has records nothing when it is confirmed.
func (ts *AdminVerificationFlowTestSuite) TestALinkForAChangedOwnerRecordsNothing() {
	ownerEmail := "agent-owner-moved-" + ts.fixture.suffix + "@example.com"
	agentID := ts.newOwnedAgent("owner-moved", testutils.ExternalAgentOwner(ownerEmail, "email"))
	result := ts.startOwnerVerification(agentID)
	ts.Require().Equal(http.StatusOK, result.StatusCode, "the administrator's start must be accepted: %s",
		result.Body)
	link := ts.fixture.awaitEmailTo(ts.T(), ownerEmail).ExtractRecoveryLink()
	ts.Require().NotEmpty(link, "the email must carry the link")

	status, body := sendJSON(ts.T(), testutils.GetHTTPClient(), http.MethodPut, "/agents/"+agentID,
		agentBody(ts.T(), "owner-moved-"+ts.fixture.suffix,
			testutils.ExternalAgentOwner("other-"+ownerEmail, "email")))
	ts.Require().Equal(http.StatusOK, status, "the agent must be updated: %s", body)

	done := confirmLink(ts.T(), link)

	ts.Equal(flowStatusError, done.FlowStatus, "a link for the earlier owner must not complete the flow")
	ts.Require().NotNil(done.Error, "the refusal must be reported")
	ts.Equal(errCodeVerificationRefused, done.Error.Code, "recording must refuse a proof of the earlier owner")
	ts.Empty(ownerVerifiedAt(ts.T(), agentID), "a link for the earlier owner must record nothing")
}

// Only an owner managed in another identity system and named by an email address or mobile number can be
// verified by link.
func (ts *AdminVerificationFlowTestSuite) TestAnAgentOwnerThatCannotBeVerifiedIsRefused() {
	localOwner := ts.fixture.newUser(ts.T(), "agent_local_owner")
	for name, owner := range map[string]*testutils.AgentOwner{
		"local owner":      testutils.LocalAgentOwner(localOwner.id),
		"external subject": testutils.ExternalAgentOwner("subject-"+ts.fixture.suffix, "uuid"),
	} {
		ts.Run(name, func() {
			agentID := ts.newOwnedAgent("owner-refused-"+strings.ReplaceAll(name, " ", "-"), owner)

			requireRefused(ts.T(), ts.startOwnerVerification(agentID), errCodeVerificationRefused,
				"preparation must refuse an owner that cannot be verified")
			ts.fixture.requireNoEmail(ts.T(), "nothing may be sent for an owner that cannot be verified")
			ts.Empty(ownerVerifiedAt(ts.T(), agentID), "nothing may be recorded")
		})
	}
}

// suspendAgent suspends the agent through the shipped agent suspend flow.
func suspendAgent(t *testing.T, agentID string) {
	t.Helper()

	flowID, err := testutils.GetFlowIDByHandle("default-agent-suspend-flow", "ADMINISTRATION")
	require.NoError(t, err)
	result, err := common.ExecuteFlowWithToken(adminToken(t),
		map[string]interface{}{"flowId": flowID, "inputs": map[string]string{"subject": agentID}})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, result.StatusCode, "precondition: the agent is suspended: %s", result.Body)
	require.Equal(t, flowStatusComplete, result.Step.FlowStatus, "precondition: the agent is suspended")
}

// The owner of a suspended agent is sent no link, and the refusal states the suspension, as for a suspended user.
func (ts *AdminVerificationFlowTestSuite) TestASuspendedAgentsOwnerIsRefused() {
	ownerEmail := "agent-owner-suspended-" + ts.fixture.suffix + "@example.com"
	agentID := ts.newOwnedAgent("owner-suspended", testutils.ExternalAgentOwner(ownerEmail, "email"))
	suspendAgent(ts.T(), agentID)

	requireRefused(ts.T(), ts.startOwnerVerification(agentID), errCodeAccountSuspended,
		"the owner of a suspended agent must be refused, stating the suspension")
	ts.fixture.requireNoEmail(ts.T(), "nothing may be sent for a suspended agent")
	ts.Empty(ownerVerifiedAt(ts.T(), agentID), "nothing may be recorded")
}

// ownerVerification returns the agent's owner verification as the agent API states it.
func ownerVerification(t *testing.T, agentID string) (*bool, string) {
	t.Helper()

	status, body := sendJSON(t, testutils.GetHTTPClient(), http.MethodGet, "/agents/"+agentID, nil)
	require.Equal(t, http.StatusOK, status, "the agent must be read: %s", body)
	var agent struct {
		Owner struct {
			Verified   *bool  `json:"verified"`
			VerifiedAt string `json:"verifiedAt"`
		} `json:"owner"`
	}
	require.NoError(t, json.Unmarshal(body, &agent))
	return agent.Owner.Verified, agent.Owner.VerifiedAt
}

// Only the verification flow verifies an owner: a verification sent with the owner in a create, an update or an
// import is ignored, and the agent reads as not verified.
func (ts *AdminVerificationFlowTestSuite) TestAnOwnerVerificationCannotBeSupplied() {
	forged := func(email string) map[string]interface{} {
		return map[string]interface{}{"id": email, "idType": "email", "isLocal": false, "verified": true,
			"verifiedAt": "2026-11-17T06:19:41Z"}
	}
	requireUnverified := func(agentID, how string) {
		verified, verifiedAt := ownerVerification(ts.T(), agentID)
		ts.Require().NotNil(verified, "an external email owner must state its verification (%s)", how)
		ts.False(*verified, "a verification sent in %s must be ignored", how)
		ts.Empty(verifiedAt, "a verification time sent in %s must be ignored", how)
	}

	created := agentBody(ts.T(), "owner-forged-"+ts.fixture.suffix, nil)
	created["owner"] = forged("forged-create-" + ts.fixture.suffix + "@example.com")
	agentID := ts.createAgent(created)
	requireUnverified(agentID, "a create")

	updated := agentBody(ts.T(), "owner-forged-"+ts.fixture.suffix, nil)
	updated["owner"] = forged("forged-update-" + ts.fixture.suffix + "@example.com")
	status, body := sendJSON(ts.T(), testutils.GetHTTPClient(), http.MethodPut, "/agents/"+agentID, updated)
	ts.Require().Equal(http.StatusOK, status, "the agent must be updated: %s", body)
	requireUnverified(agentID, "an update")

	imported := agentBody(ts.T(), "owner-forged-import-"+ts.fixture.suffix, nil)
	imported["owner"] = forged("forged-import-" + ts.fixture.suffix + "@example.com")
	importedID, err := testutils.ImportResource("agent", imported, false)
	ts.Require().NoError(err, "the agent must be imported")
	ts.T().Cleanup(func() {
		if status, body := sendJSON(ts.T(), testutils.GetHTTPClient(), http.MethodDelete, "/agents/"+importedID,
			nil); status != http.StatusNoContent {
			ts.T().Logf("cleanup: failed to delete agent %s: %d %s", importedID, status, body)
		}
	})
	requireUnverified(importedID, "an import")
}

// smsLinkPattern finds the link an SMS carries.
var smsLinkPattern = regexp.MustCompile(`https?://\S+`)

// An agent's owner named by a mobile number gets the link by SMS, and verifies from it.
func (ts *AdminVerificationFlowTestSuite) TestTheAgentsMobileOwnerVerifiesFromTheSMSLink() {
	sms := newSMSProvider(ts.T(), "agent-owner-"+ts.fixture.suffix)
	handle := copyAgentOwnerFlow(ts.T(), "agent-owner-sms-"+ts.fixture.suffix,
		map[string]string{adminLinkSendNode: ts.fixture.senderID, agentOwnerSMSSendNode: sms.senderID})
	agentID := ts.newOwnedAgent("owner-sms", testutils.ExternalAgentOwner("+94771234567", "mobile"))

	result := execute(ts.T(), adminToken(ts.T()), adminFlowStart(handle, map[string]string{"subject": agentID}))
	ts.Require().Equal(http.StatusOK, result.StatusCode, "the administrator's start must be accepted: %s",
		result.Body)
	ts.Require().Equal(flowStatusIncomplete, result.Step.FlowStatus, "the flow must wait once the link is sent: %s",
		result.Body)
	ts.NotContains(result.Step.Data.AdditionalData, "inviteLink", "the administrator must not get the link")
	ts.fixture.requireNoEmail(ts.T(), "a mobile owner must not be emailed")
	link := smsLinkPattern.FindString(sms.awaitMessage(ts.T()).Message)
	ts.Require().NotEmpty(link, "the SMS must carry the link")
	ts.Empty(ownerVerifiedAt(ts.T(), agentID), "sending a link verifies nothing")

	ts.Require().Equal(flowStatusComplete, confirmLink(ts.T(), link).FlowStatus, "confirming must verify the owner")
	ts.NotEmpty(ownerVerifiedAt(ts.T(), agentID), "the agent must report its owner verified")
}

// An organization unit administrator sends a link to the owner of an agent of its own unit, and is refused for
// an agent outside it.
func (ts *AdminVerificationFlowTestSuite) TestAnOrganizationUnitAdministratorVerifiesOnlyTheOwnersOfItsAgents() {
	token, ouID := ts.ouAdministratorToken("ou_agent_admin", "agent")
	handle := copyAgentOwnerFlowIn(ts.T(), "agent-owner-ou-"+ts.fixture.suffix, ouID,
		map[string]string{adminLinkSendNode: ts.fixture.senderID})
	outsideEmail := "agent-owner-outside-" + ts.fixture.suffix + "@example.com"
	outside := ts.newOwnedAgent("owner-ou-outside", testutils.ExternalAgentOwner(outsideEmail, "email"))
	insideEmail := "agent-owner-inside-" + ts.fixture.suffix + "@example.com"
	// An agent must be of a type its unit sees, so the agent of the administrator's unit gets a type of its own there.
	insideType := "owner-ou-agent-" + ts.fixture.suffix
	typeID, err := testutils.CreateNewAgentType(testutils.UserType{
		Handle: insideType, DisplayName: "Owner Verification Agent", OUID: ouID,
		Schema: map[string]interface{}{"name": map[string]interface{}{"type": "string", "required": true}},
	})
	ts.Require().NoError(err, "create the agent type")
	ts.T().Cleanup(func() {
		if err := testutils.DeleteAgentType(typeID); err != nil {
			ts.T().Logf("cleanup: delete the agent type: %v", err)
		}
	})
	inside := ts.createAgent(map[string]interface{}{"ouId": ouID, "type": insideType,
		"attributes": map[string]string{"name": "owner-ou-inside-" + ts.fixture.suffix},
		"owner":      testutils.ExternalAgentOwner(insideEmail, "email")})

	refused := execute(ts.T(), token, adminFlowStart(handle, map[string]string{"subject": outside}))

	requireRefused(ts.T(), refused, errCodeVerificationRefused,
		"an agent outside the administrator's unit must be refused")
	ts.fixture.requireNoEmail(ts.T(), "nothing may be sent to the owner of an agent outside the unit")
	ts.Empty(ownerVerifiedAt(ts.T(), outside), "nothing may be recorded")

	sent := execute(ts.T(), token, adminFlowStart(handle, map[string]string{"subject": inside}))

	ts.Require().NotNil(sent.Step, "the start must run: %s", sent.Body)
	ts.Require().Equal(flowStatusIncomplete, sent.Step.FlowStatus,
		"the owner of an agent of the unit is sent a link: %s", sent.Body)
	link := ts.fixture.awaitEmailTo(ts.T(), insideEmail).ExtractRecoveryLink()
	ts.Require().NotEmpty(link, "the email must carry the link")

	ts.Require().Equal(flowStatusComplete, confirmLink(ts.T(), link).FlowStatus, "confirming must verify the owner")
	ts.NotEmpty(ownerVerifiedAt(ts.T(), inside), "the agent must report its owner verified")
	ts.Empty(ownerVerifiedAt(ts.T(), outside), "the agent outside the unit must keep its owner unverified")
}

// An agent that selects owner_verified states on its own tokens whether its owner is verified: false until the
// owner confirms the link, then true.
func (ts *AdminVerificationFlowTestSuite) TestTheAgentsTokenStatesWhetherItsOwnerIsVerified() {
	ownerEmail := "agent-owner-token-" + ts.fixture.suffix + "@example.com"
	clientID := "owner-verified-" + ts.fixture.suffix
	const clientSecret = "Owner-Verified-Secret-1!"
	agent := agentBody(ts.T(), "owner-token-"+ts.fixture.suffix, testutils.ExternalAgentOwner(ownerEmail, "email"))
	agent["inboundAuthConfig"] = []map[string]interface{}{{
		"type": "oauth2",
		"config": map[string]interface{}{
			"clientId": clientID, "clientSecret": clientSecret,
			"grantTypes": []string{"client_credentials"}, "tokenEndpointAuthMethod": "client_secret_basic",
			"token": map[string]interface{}{"accessToken": map[string]interface{}{
				"clientConfig": map[string]interface{}{"attributes": []string{"owner_verified"}}}},
		},
	}}
	agentID := ts.createAgent(agent)

	ownerVerifiedClaim := func() interface{} {
		result, err := testutils.RequestClientCredentialsToken(clientID, clientSecret, "", "")
		ts.Require().NoError(err)
		ts.Require().Equal(http.StatusOK, result.StatusCode, "the agent must get a token: %s", result.Body)
		ts.Require().NotNil(result.Token)
		claims, err := testutils.DecodeJWTPayloadMap(result.Token.AccessToken)
		ts.Require().NoError(err)
		return claims["owner_verified"]
	}

	ts.Equal(false, ownerVerifiedClaim(), "an owner that has not confirmed must be stated as not verified")

	ts.verifyOwner(agentID, ownerEmail)

	ts.Equal(true, ownerVerifiedClaim(), "an owner that confirmed must be stated as verified")
}
