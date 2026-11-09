// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	invitationUserType  = "invitation-type"
	invitationOtherType = "invitation-other-type"
	invitationPassword  = "Invitation@123"
	// invitationTemplates is the designer's template file, so the flow under test is the shipped sample.
	invitationTemplates = "../../../../frontend/packages/configure-flows/src/data/templates.json"

	errCodeInvitationAuthRequired      = "FES-1017"
	errCodeInvitationPermission        = "FES-1019"
	errCodeTargetNotDirectChild        = "SHR-1030"
	errCodeFlowNotShared               = "FES-1029"
	errCodeNotTheInviter               = "FET-1136"
	errCodeInvitationOUMissing         = "FES-1027"
	errCodeInvitedRoleNotAllowed       = "FET-1133"
	errCodeInvitedTypeNotAllowed       = "FET-1054"
	errCodeInvitationInsufficientPerms = "FET-1027"

	// The template's two branches, which suffix the actions each one prompts with.
	branchExisting = "existing"
	branchNew      = "new"
)

// InvitationTestSuite runs the sample invitation flow. The user type lives in parent; the admins
// hold the org permission through a role in their own unit, and their tokens carry that unit as ouId.
type InvitationTestSuite struct {
	suite.Suite

	smtp     *testutils.MockSMTPServer
	senderID string
	// The admins live outside the default OU, and a sign-in is bounded to its application's OU.
	signIn testutils.OUSignInApps
	// flowID is the flow the parent owns and shares with the child's subtree; unsharedFlowID it shares with no one.
	flowID         string
	unsharedFlowID string

	parentOUID     string
	childOUID      string
	grandchildOUID string
	// siblingOUID is another child of the parent, which the flow's sharing policy does not name.
	siblingOUID string
	userTypeID  string
	otherTypeID string

	childAdminToken      string
	grandchildAdminToken string
	siblingAdminToken    string
	limitedAdminToken    string
	// childRSID is the resource server the child admin's org permission and ownedRoleID's permission are on.
	childRSID string

	ownedRoleID  string
	closedRoleID string
	// inviteTokens holds the token each emailed invite carries, by execution.
	inviteTokens map[string]string

	userIDs []string
	roleIDs []string
	rsIDs   []string
}

func TestInvitationTestSuite(t *testing.T) {
	suite.Run(t, new(InvitationTestSuite))
}

func (ts *InvitationTestSuite) SetupSuite() {
	ts.inviteTokens = map[string]string{}
	var err error
	ts.smtp = testutils.NewMockSMTPServer(0)
	ts.Require().NoError(ts.smtp.Start(), "start the mock SMTP server")
	ts.senderID, err = testutils.CreateSMTPEmailProvider("User Invitation Test Provider",
		"localhost", ts.smtp.GetPort(), "noreply@thunderid.test")
	ts.Require().NoError(err, "create the SMTP email provider")

	ts.parentOUID, err = testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "invitation-parent", Name: "User Invitation Parent",
	})
	ts.Require().NoError(err, "create parent OU")
	ts.childOUID, err = testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "invitation-child", Name: "User Invitation Child", Parent: &ts.parentOUID,
	})
	ts.Require().NoError(err, "create child OU")
	ts.grandchildOUID, err = testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "invitation-grandchild", Name: "User Invitation Grandchild", Parent: &ts.childOUID,
	})
	ts.Require().NoError(err, "create grandchild OU")
	ts.siblingOUID, err = testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "invitation-sibling", Name: "User Invitation Sibling", Parent: &ts.parentOUID,
	})
	ts.Require().NoError(err, "create sibling OU")

	ts.userTypeID, err = testutils.CreateUserType(testutils.UserType{
		Handle:      invitationUserType,
		DisplayName: "User Invitation Test Type",
		OUID:        ts.parentOUID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"email":    map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	})
	ts.Require().NoError(err, "create user type")
	ts.otherTypeID, err = testutils.CreateUserType(testutils.UserType{
		Handle: invitationOtherType, DisplayName: "User Invitation Other Type", OUID: ts.parentOUID,
		Schema: map[string]interface{}{"email": map[string]interface{}{"type": "string"}},
	})
	ts.Require().NoError(err, "create the other user type")

	childAdmin := ts.adminToken(ts.childOUID, "child", []string{"org"})
	ts.childAdminToken = childAdmin.token
	ts.childRSID = childAdmin.rsID
	ts.grandchildAdminToken = ts.adminToken(ts.grandchildOUID, "grandchild", []string{"org"}).token
	ts.siblingAdminToken = ts.adminToken(ts.siblingOUID, "sibling", []string{"org"}).token
	ts.limitedAdminToken = ts.adminToken(ts.childOUID, "limited", []string{"org:user"}).token

	s := &ts.Suite
	ts.ownedRoleID = createRole(s, &ts.roleIDs, ts.childOUID, childAdmin.rsID, "User Invitation Viewer",
		[]string{"org:user:view"})
	ts.closedRoleID = createRole(s, &ts.roleIDs, ts.parentOUID, "", "User Invitation Closed", nil)
	status, raw := ts.request(testutils.GetHTTPClient(), http.MethodPost,
		"/roles/"+ts.closedRoleID+"/sharing-policies", map[string]interface{}{
			"childUnits": map[string]interface{}{"mode": "selected", "units": []map[string]interface{}{{
				"ouId": ts.childOUID, "reach": "unit",
				"overlayRules": map[string]interface{}{"assignments": map[string]interface{}{"editable": false}},
			}}},
		})
	ts.Require().Equal(http.StatusCreated, status, "share the closed role: %s", raw)

	ts.flowID, err = testutils.CreateFlow(testutils.Flow{
		Name: "User Invitation Test Flow", FlowType: "INVITATION", Handle: "invitation-test-flow",
		Nodes: ts.templateNodes(), OUID: ts.parentOUID,
	})
	ts.Require().NoError(err, "create the flow from the designer template")
	status, raw = ts.request(testutils.GetHTTPClient(), http.MethodPost,
		"/flows/"+ts.flowID+"/sharing-policies", map[string]interface{}{
			"childUnits": map[string]interface{}{"mode": "selected", "units": []map[string]interface{}{{
				"ouId": ts.childOUID, "reach": "subtree",
			}}},
		})
	ts.Require().Equal(http.StatusCreated, status, "share the flow with the child's subtree: %s", raw)
	ts.unsharedFlowID, err = testutils.CreateFlow(testutils.Flow{
		Name: "User Invitation Unshared Flow", FlowType: "INVITATION", Handle: "invitation-unshared-flow",
		Nodes: ts.templateNodes(), OUID: ts.parentOUID,
	})
	ts.Require().NoError(err, "create the flow shared with no one")
}

func (ts *InvitationTestSuite) SetupTest() {
	ts.smtp.ClearEmails()
}

func (ts *InvitationTestSuite) TearDownSuite() {
	if err := ts.signIn.DeleteAll(); err != nil {
		ts.T().Logf("teardown: delete sign-in applications: %v", err)
	}
	for _, id := range []string{ts.flowID, ts.unsharedFlowID} {
		if id != "" {
			if err := testutils.DeleteFlow(id); err != nil {
				ts.T().Logf("teardown: delete flow %s: %v", id, err)
			}
		}
	}
	for _, id := range ts.roleIDs {
		if err := testutils.DeleteRole(id); err != nil {
			ts.T().Logf("teardown: delete role %s: %v", id, err)
		}
	}
	for _, id := range ts.userIDs {
		if err := testutils.DeleteUser(id); err != nil {
			ts.T().Logf("teardown: delete user %s: %v", id, err)
		}
	}
	for _, id := range ts.rsIDs {
		if err := testutils.DeleteResourceServerWithChildren(id); err != nil {
			ts.T().Logf("teardown: delete resource server %s: %v", id, err)
		}
	}
	for _, id := range []string{ts.userTypeID, ts.otherTypeID} {
		if id != "" {
			if err := testutils.DeleteUserType(id); err != nil {
				ts.T().Logf("teardown: delete user type %s: %v", id, err)
			}
		}
	}
	for _, id := range []string{ts.grandchildOUID, ts.childOUID, ts.siblingOUID, ts.parentOUID} {
		if id != "" {
			if err := testutils.DeleteOrganizationUnit(id); err != nil {
				ts.T().Logf("teardown: delete OU %s: %v", id, err)
			}
		}
	}
	if ts.senderID != "" {
		if err := testutils.DeleteNotificationSender(ts.senderID); err != nil {
			ts.T().Logf("teardown: delete email provider: %v", err)
		}
	}
	if ts.smtp != nil {
		if err := ts.smtp.Stop(); err != nil {
			ts.T().Logf("teardown: stop the mock SMTP server: %v", err)
		}
	}
}

// An existing user accepts: the parent shares them into the child, and the role is assigned there as the
// admin, although the acceptance carries no token.
func (ts *InvitationTestSuite) TestExistingUser_AcceptSharesIntoAAndAssignsRoles() {
	email := "invitation-existing@thunderid.test"
	userID := ts.createUser(ts.parentOUID, "invitation-existing", email)

	executionID := ts.invite(ts.childAdminToken, email, ts.ownedRoleID, branchExisting)
	ts.Empty(ts.policyUnits(userID), "nothing is shared before acceptance")

	step := ts.accept(executionID, "action_accept_"+branchExisting)

	ts.NotEqual("ERROR", step.FlowStatus, "%+v", step)
	ts.Contains(ts.policyUnits(userID), ts.childOUID)
	ts.Contains(ts.assigneesInChild(ts.ownedRoleID), userID)
}

// An admin who loses the org permission after inviting assigns no roles: the roles were selected with a copy
// of their token, which acceptance checks against what they hold now. The share, on the owner's authority, stays.
func (ts *InvitationTestSuite) TestAdminWhoLostTheOrgPermission_AssignsNoRoles() {
	// The admin holds the org permission on the resource server the owned role grants on, as an organization's
	// admins all hold it on the System resource server.
	admin := ts.adminTokenOn(ts.childOUID, "revoked", []string{"org"}, ts.childRSID, invitationRSIdentifier("child"))
	email := "invitation-revoked@thunderid.test"
	userID := ts.createUser(ts.parentOUID, "invitation-revoked", email)
	executionID := ts.invite(admin.token, email, ts.ownedRoleID, branchExisting)

	ts.Require().NoError(testutils.RemoveRoleAssignments(admin.roleID,
		[]testutils.Assignment{{ID: admin.userID, Type: "user"}}), "revoke the admin's org permission")
	step := ts.accept(executionID, "action_accept_"+branchExisting)

	ts.NotEqual("ERROR", step.FlowStatus, "%+v", step)
	ts.Contains(ts.policyUnits(userID), ts.childOUID)
	ts.NotContains(ts.assigneesInChild(ts.ownedRoleID), userID)
}

// A new user is created at acceptance in the user type's unit, then shared into the child with the role. The
// type's unit is the child's parent, which only the deployment-wide permission reaches.
func (ts *InvitationTestSuite) TestNewUser_CreatedAtAcceptanceInTheRoot() {
	username, email := "invitation-new", "invitation-new@thunderid.test"
	admin := ts.adminTokenOn(ts.childOUID, "new-user", []string{"system"}, ts.childRSID,
		invitationRSIdentifier("child"))

	executionID := ts.invite(admin.token, email, ts.ownedRoleID, branchNew)
	ts.Nil(ts.findUser(username), "no user exists before acceptance")

	step := ts.accept(executionID, "action_accept_"+branchNew)
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "%+v", step)
	step = ts.continueAsInvitee(executionID, step.ChallengeToken, "action_user_details",
		map[string]string{"username": username, "password": invitationPassword})
	ts.NotEqual("ERROR", step.FlowStatus, "%+v", step)

	user := ts.findUser(username)
	ts.Require().NotNil(user, "the user is created at acceptance")
	ts.userIDs = append(ts.userIDs, user.ID)
	ts.Equal(ts.parentOUID, user.OUID)
	ts.Contains(ts.policyUnits(user.ID), ts.childOUID)
	ts.Contains(ts.assigneesInChild(ts.ownedRoleID), user.ID)
}

func (ts *InvitationTestSuite) TestDecline_WritesNothing() {
	email := "invitation-declines@thunderid.test"
	userID := ts.createUser(ts.parentOUID, "invitation-declines", email)
	executionID := ts.invite(ts.childAdminToken, email, ts.ownedRoleID, branchExisting)

	step := ts.accept(executionID, "action_decline_"+branchExisting)

	ts.Nil(step.Error, "%+v", step)
	ts.Equal("Invitation declined", step.Data.AdditionalData["message"], "the flow reaches the declined screen")
	ts.Empty(ts.policyUnits(userID))
	ts.NotContains(ts.assigneesInChild(ts.ownedRoleID), userID)
}

// A token that names no organization unit names none for the invitation to act for.
func (ts *InvitationTestSuite) TestTokenWithoutAnOU_Refused() {
	const clientID = "invitation_no_ou_sign_in"
	appID, err := testutils.CreateApplication(testutils.Application{
		Name: "Invitation No OU Sign-in", OUID: ts.childOUID, Type: "browser",
		InboundAuthConfig: []map[string]interface{}{{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId": clientID, "redirectUris": []string{testutils.OUSignInRedirectURI},
				"grantTypes": []string{"authorization_code"}, "responseTypes": []string{"code"},
				"pkceRequired": true, "tokenEndpointAuthMethod": "none", "publicClient": true,
			},
		}},
	})
	ts.Require().NoError(err, "create a sign-in application whose tokens carry no ouId")
	defer func() {
		if err := testutils.DeleteApplication(appID); err != nil {
			ts.T().Logf("delete application %s: %v", appID, err)
		}
	}()
	token, err := testutils.ObtainAccessTokenWithPassword(clientID, testutils.OUSignInRedirectURI,
		"org org:user", "invitation-child-admin", invitationPassword, true, "",
		invitationRSIdentifier("child"))
	ts.Require().NoError(err, "sign the child admin in")

	status, _, errResp := ts.execute(token.AccessToken, map[string]interface{}{"flowId": ts.flowID})

	ts.Equal(http.StatusForbidden, status)
	ts.Require().NotNil(errResp)
	ts.Equal(errCodeInvitationOUMissing, errResp.Code)
}

// What the invitee submits with the accept action cannot choose the roles or the unit: both come from the admin's
// half, recorded in the flow.
func (ts *InvitationTestSuite) TestInviteeInputs_CannotChooseRolesOrUnit() {
	email := "invitation-tampered@thunderid.test"
	userID := ts.createUser(ts.parentOUID, "invitation-tampered", email)
	executionID := ts.invite(ts.childAdminToken, email, ts.ownedRoleID, branchExisting)
	opened := ts.continueAsInvitee(executionID, "", "",
		map[string]string{"inviteToken": ts.inviteTokens[executionID]})
	ts.Require().Nil(opened.Error, "%+v", opened)

	step := ts.continueAsInvitee(executionID, opened.ChallengeToken, "action_accept_"+branchExisting,
		map[string]string{"roles": ts.closedRoleID, "ouId": ts.siblingOUID})

	ts.Nil(step.Error, "%+v", step)
	ts.Equal([]string{ts.childOUID}, ts.policyUnits(userID))
	ts.Contains(ts.assigneesInChild(ts.ownedRoleID), userID)
	ts.NotContains(ts.assigneesInChild(ts.closedRoleID), userID)
}

func (ts *InvitationTestSuite) TestAnonymousStart_Refused() {
	status, _, errResp := ts.execute("", map[string]interface{}{"flowId": ts.flowID})

	ts.Equal(http.StatusUnauthorized, status)
	ts.Require().NotNil(errResp)
	ts.Equal(errCodeInvitationAuthRequired, errResp.Code)
}

// The template requires the org permission, so an admin holding only the user permission is refused at the start.
func (ts *InvitationTestSuite) TestNarrowAdmin_RefusedByTheTemplatesValidator() {
	step := ts.start(ts.limitedAdminToken, "invitation-narrow@thunderid.test")

	ts.Require().NotNil(step.Error, "%+v", step)
	ts.Equal(errCodeInvitationInsufficientPerms, step.Error.Code)
	ts.Empty(ts.smtp.GetEmails(), "no invite is sent")
}

// An admin's half of the flow is theirs alone: another admin of the same unit cannot send its next step.
func (ts *InvitationTestSuite) TestAnotherAdmin_CannotContinueTheInvitersSteps() {
	email := "invitation-hijacked@thunderid.test"
	ts.createUser(ts.parentOUID, "invitation-hijacked", email)
	step := ts.start(ts.childAdminToken, email)
	ts.Require().Nil(step.Error, "%+v", step)

	step = ts.continueAsAdmin(ts.limitedAdminToken, step, "action_roles_"+branchExisting,
		map[string]string{"roles": ts.ownedRoleID})

	ts.Require().NotNil(step.Error, "%+v", step)
	ts.Equal(errCodeNotTheInviter, step.Error.Code)
	ts.Empty(ts.smtp.GetEmails(), "no invite is sent")
}

// The child's admin holds the org permission, but runs only a flow its unit owns or was shared.
func (ts *InvitationTestSuite) TestFlowNotSharedWithTheAdminsUnit_Refused() {
	status, _, errResp := ts.execute(ts.childAdminToken, map[string]interface{}{"flowId": ts.unsharedFlowID})

	ts.Equal(http.StatusForbidden, status)
	ts.Require().NotNil(errResp)
	ts.Equal(errCodeFlowNotShared, errResp.Code)
}

// The flow is shared with the child's subtree only, so the admin of the child's sibling cannot run it.
func (ts *InvitationTestSuite) TestAdminOfAUnitTheFlowIsNotSharedWith_Refused() {
	status, _, errResp := ts.execute(ts.siblingAdminToken, map[string]interface{}{"flowId": ts.flowID})

	ts.Equal(http.StatusForbidden, status)
	ts.Require().NotNil(errResp)
	ts.Equal(errCodeFlowNotShared, errResp.Code)
}

// A role shared with the child whose assignments it may not change is neither offered nor accepted.
func (ts *InvitationTestSuite) TestRoleNotAssignable_NotOffered() {
	email := "invitation-closed-role@thunderid.test"
	ts.createUser(ts.parentOUID, "invitation-closed-role", email)

	step := ts.start(ts.childAdminToken, email)
	var options []struct {
		ID string `json:"id"`
	}
	ts.Require().NoError(json.Unmarshal([]byte(step.Data.AdditionalData["roleOptions"]), &options), "%+v", step)
	ids := make([]string, 0, len(options))
	for _, o := range options {
		ids = append(ids, o.ID)
	}
	ts.Contains(ids, ts.ownedRoleID)
	ts.NotContains(ids, ts.closedRoleID)

	step = ts.continueAsAdmin(ts.childAdminToken, step, "action_roles_"+branchExisting,
		map[string]string{"roles": ts.closedRoleID})
	ts.Require().NotNil(step.Error, "%+v", step)
	ts.Equal(errCodeInvitedRoleNotAllowed, step.Error.Code)
	ts.Empty(ts.smtp.GetEmails(), "no invite is sent")
}

// An admin who may invite but not change role assignments in their unit is offered no role, and the invite
// is still sent.
func (ts *InvitationTestSuite) TestAdminWhoMayNotAssignRoles_InvitesWithoutARole() {
	nodes := ts.templateNodes()
	for _, node := range nodes {
		if node["id"] == "permission_validator" {
			node["properties"].(map[string]interface{})["requiredScopes"] = []string{"org:user"}
		}
	}
	flowID, err := testutils.CreateFlow(testutils.Flow{
		Name: "User Invitation User Admin Flow", FlowType: "INVITATION", Handle: "invitation-user-admin-flow",
		Nodes: nodes, OUID: ts.parentOUID,
	})
	ts.Require().NoError(err, "create the flow requiring only the user permission")
	ts.T().Cleanup(func() { ts.NoError(testutils.DeleteFlow(flowID)) })
	status, raw := ts.request(testutils.GetHTTPClient(), http.MethodPost,
		"/flows/"+flowID+"/sharing-policies", map[string]interface{}{
			"childUnits": map[string]interface{}{"mode": "selected", "units": []map[string]interface{}{{
				"ouId": ts.childOUID, "reach": "subtree",
			}}},
		})
	ts.Require().Equal(http.StatusCreated, status, "share the flow with the child's subtree: %s", raw)
	email := "invitation-no-roles@thunderid.test"
	ts.createUser(ts.parentOUID, "invitation-no-roles", email)

	status, step, errResp := ts.execute(ts.limitedAdminToken, map[string]interface{}{
		"flowId": flowID, "action": "action_submit_email", "inputs": map[string]string{"email": email},
	})

	ts.Require().Equal(http.StatusOK, status, "%+v", errResp)
	ts.Require().Nil(step.Error, "%+v", step)
	ts.Empty(step.Data.AdditionalData["roleOptions"], "no role is offered")
	emails := ts.smtp.GetEmails()
	ts.Require().Len(emails, 1, "one invite is emailed")
	ts.Contains(emails[0].To, email)
}

// The flow is bound to its user type, so a user of another type with the email is not invited.
func (ts *InvitationTestSuite) TestUserOfAnotherType_Refused() {
	email := "invitation-other-type@thunderid.test"
	id, err := testutils.CreateUser(testutils.User{
		Type: invitationOtherType, OUID: ts.parentOUID, Attributes: json.RawMessage(fmt.Sprintf(`{"email": %q}`, email)),
	})
	ts.Require().NoError(err, "create the user of the other type")
	ts.userIDs = append(ts.userIDs, id)

	step := ts.start(ts.childAdminToken, email)

	ts.Require().NotNil(step.Error, "%+v", step)
	ts.Equal(errCodeInvitedTypeNotAllowed, step.Error.Code)
	ts.Empty(ts.smtp.GetEmails(), "no invite is sent")
}

// A grandchild's admin cannot pull a user of the parent in: the share must go to a direct child.
func (ts *InvitationTestSuite) TestGrandchildAdmin_Refused() {
	email := "invitation-grandchild@thunderid.test"
	ts.createUser(ts.parentOUID, "invitation-grandchild", email)

	step := ts.start(ts.grandchildAdminToken, email)

	ts.Require().NotNil(step.Error, "%+v", step)
	ts.Equal(errCodeTargetNotDirectChild, step.Error.Code)
	ts.Empty(ts.smtp.GetEmails(), "no invite is sent")
}

// invitationAdmin is an admin of one unit: their token, the resource server their role grants on, the role
// and the admin user.
type invitationAdmin struct {
	token  string
	rsID   string
	roleID string
	userID string
}

// adminToken makes a user of ouID hold permissions there through a role on a resource server of its own,
// and returns them with their token, issued with ouID as its ouId, with that resource server.
func (ts *InvitationTestSuite) adminToken(ouID, name string, permissions []string) invitationAdmin {
	identifier := invitationRSIdentifier(name)
	rsID, err := testutils.CreateOrgScopedResourceServer(ouID, "User Invitation "+name+" RS", identifier, "user")
	ts.Require().NoError(err, "create resource server for %s", name)
	ts.rsIDs = append(ts.rsIDs, rsID)
	return ts.adminTokenOn(ouID, name, permissions, rsID, identifier)
}

// adminTokenOn makes a user of ouID hold permissions there through a role on the resource server rsID, identified
// by identifier, and returns them with their token, issued with ouID as its ouId, with that resource server.
func (ts *InvitationTestSuite) adminTokenOn(ouID, name string, permissions []string, rsID,
	identifier string) invitationAdmin {
	username := "invitation-" + name + "-admin"
	userID := ts.createUser(ouID, username, username+"@thunderid.test")
	roleID := createRole(&ts.Suite, &ts.roleIDs, ouID, rsID, "User Invitation "+name+" Admin", permissions,
		testutils.Assignment{ID: userID, Type: "user"})

	token, err := ts.signIn.ObtainAccessToken(ouID, strings.Join(permissions, " "), username, invitationPassword,
		identifier)
	ts.Require().NoError(err, "obtain token for %s", name)
	return invitationAdmin{token: token.AccessToken, rsID: rsID, roleID: roleID, userID: userID}
}

func (ts *InvitationTestSuite) createUser(ouID, username, email string) string {
	id, err := testutils.CreateUser(testutils.User{
		Type: invitationUserType,
		OUID: ouID,
		Attributes: json.RawMessage(fmt.Sprintf(`{"username": %q, "email": %q, "password": %q}`,
			username, email, invitationPassword)),
	})
	ts.Require().NoError(err, "create user %s", username)
	ts.userIDs = append(ts.userIDs, id)
	return id
}

// templateNodes reads the designer's invitation template, names the suite's user type, and points the
// email node at the mock sender, as an admin does in the designer.
func (ts *InvitationTestSuite) templateNodes() []map[string]interface{} {
	raw, err := os.ReadFile(invitationTemplates)
	ts.Require().NoError(err, "read the designer templates")
	var templates []struct {
		Type     string `json:"type"`
		FlowType string `json:"flowType"`
		Config   struct {
			Nodes []map[string]interface{} `json:"nodes"`
		} `json:"config"`
	}
	ts.Require().NoError(json.Unmarshal(raw, &templates))
	for _, t := range templates {
		if t.FlowType != "INVITATION" || t.Type != "DEFAULT" {
			continue
		}
		for _, node := range t.Config.Nodes {
			id, _ := node["id"].(string)
			switch {
			case id == "resolve_user_type":
				node["properties"].(map[string]interface{})["allowedUserTypes"] = []string{invitationUserType}
			case strings.HasPrefix(id, "send_invite_email_"):
				node["properties"].(map[string]interface{})["senderId"] = ts.senderID
			}
		}
		return t.Config.Nodes
	}
	ts.FailNow("the designer has no INVITATION template")
	return nil
}

// start begins the flow as the admin with the invitee's email and returns the step that follows it.
func (ts *InvitationTestSuite) start(token, email string) *common.FlowStep {
	status, step, errResp := ts.execute(token, map[string]interface{}{
		"flowId": ts.flowID, "action": "action_submit_email", "inputs": map[string]string{"email": email},
	})
	ts.Require().Equal(http.StatusOK, status, "%+v", errResp)
	return step
}

// invite runs the admin's half down the branch the email leads to: it picks the role and returns the execution
// the emailed link continues.
func (ts *InvitationTestSuite) invite(token, email, roleID, branch string) string {
	step := ts.start(token, email)
	ts.Require().Nil(step.Error, "%+v", step)
	if branch == branchNew {
		// No account has the email, so the admin confirms inviting a new user.
		step = ts.continueAsAdmin(token, step, "action_invite_new", nil)
		ts.Require().Nil(step.Error, "%+v", step)
	}
	// An admin who may assign no role in the unit is not asked for one.
	if roleID != "" {
		step = ts.continueAsAdmin(token, step, "action_roles_"+branch, map[string]string{"roles": roleID})
		ts.Require().Nil(step.Error, "%+v", step)
	}
	ts.Empty(step.Data.AdditionalData["inviteLink"], "the admin gets no link to accept with when the email is sent")

	emails := ts.smtp.GetEmails()
	ts.Require().Len(emails, 1, "one invite is emailed")
	ts.Contains(emails[0].To, email)
	ts.Contains(emails[0].Body, "invited you to join their organization", "the invitation invite, not the sign-up one")
	link := emails[0].ExtractRecoveryLink()
	ts.Require().NotEmpty(link, "the invite email carries a link")
	ts.Equal(step.ExecutionID, testutils.ExtractQueryParam(link, "executionId"))
	ts.inviteTokens[step.ExecutionID] = testutils.ExtractQueryParam(link, "inviteToken")
	return step.ExecutionID
}

// accept opens the invite link as the invitee, with no token, and answers the prompt with action.
func (ts *InvitationTestSuite) accept(executionID, action string) *common.FlowStep {
	step := ts.continueAsInvitee(executionID, "", "",
		map[string]string{"inviteToken": ts.inviteTokens[executionID]})
	ts.Require().Nil(step.Error, "%+v", step)
	return ts.continueAsInvitee(executionID, step.ChallengeToken, action, nil)
}

func (ts *InvitationTestSuite) continueAsAdmin(token string, step *common.FlowStep, action string,
	inputs map[string]string) *common.FlowStep {
	return ts.continueAs(token, step.ExecutionID, step.ChallengeToken, action, inputs)
}

func (ts *InvitationTestSuite) continueAsInvitee(executionID, challengeToken, action string,
	inputs map[string]string) *common.FlowStep {
	return ts.continueAs("", executionID, challengeToken, action, inputs)
}

func (ts *InvitationTestSuite) continueAs(token, executionID, challengeToken, action string,
	inputs map[string]string) *common.FlowStep {
	body := map[string]interface{}{"executionId": executionID, "challengeToken": challengeToken}
	if action != "" {
		body["action"] = action
	}
	if inputs != nil {
		body["inputs"] = inputs
	}
	status, step, errResp := ts.execute(token, body)
	ts.Require().Equal(http.StatusOK, status, "%+v", errResp)
	return step
}

// execute sends a flow request, with the token as a bearer token when one is given.
func (ts *InvitationTestSuite) execute(token string, body map[string]interface{}) (
	int, *common.FlowStep, *common.ErrorResponse) {
	raw, err := json.Marshal(body)
	ts.Require().NoError(err)
	req, err := http.NewRequest(http.MethodPost, testutils.TestServerURL+"/flow/execute", bytes.NewReader(raw))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := testutils.GetRawHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)

	if resp.StatusCode != http.StatusOK {
		var errResp common.ErrorResponse
		ts.Require().NoError(json.Unmarshal(respBody, &errResp), string(respBody))
		return resp.StatusCode, nil, &errResp
	}
	var step common.FlowStep
	ts.Require().NoError(json.Unmarshal(respBody, &step), string(respBody))
	return resp.StatusCode, &step, nil
}

// policyUnits lists the units the user's sharing policies name.
func (ts *InvitationTestSuite) policyUnits(userID string) []string {
	status, raw := ts.request(testutils.GetHTTPClient(), http.MethodGet, "/users/"+userID+"/sharing-policies", nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	var list struct {
		Policies []struct {
			ChildUnits struct {
				Units []struct {
					OUID string `json:"ouId"`
				} `json:"units"`
			} `json:"childUnits"`
		} `json:"policies"`
	}
	ts.Require().NoError(json.Unmarshal(raw, &list), string(raw))
	var units []string
	for _, p := range list.Policies {
		for _, u := range p.ChildUnits.Units {
			units = append(units, u.OUID)
		}
	}
	return units
}

// assigneesInChild lists who holds the role through an assignment made in the child.
func (ts *InvitationTestSuite) assigneesInChild(roleID string) []string {
	path := "/roles/" + roleID + "/assignments?filter=" + url.QueryEscape(`ouId eq "`+ts.childOUID+`"`)
	status, raw := ts.request(testutils.GetHTTPClient(), http.MethodGet, path, nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	var list struct {
		Assignments []struct {
			ID string `json:"id"`
		} `json:"assignments"`
	}
	ts.Require().NoError(json.Unmarshal(raw, &list), string(raw))
	var ids []string
	for _, a := range list.Assignments {
		ids = append(ids, a.ID)
	}
	return ids
}

func (ts *InvitationTestSuite) findUser(username string) *testutils.User {
	path := "/users?filter=" + url.QueryEscape(`username eq "`+username+`"`)
	status, raw := ts.request(testutils.GetHTTPClient(), http.MethodGet, path, nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	var list struct {
		Users []testutils.User `json:"users"`
	}
	ts.Require().NoError(json.Unmarshal(raw, &list), string(raw))
	idx := slices.IndexFunc(list.Users, func(u testutils.User) bool { return u.Type == invitationUserType })
	if idx < 0 {
		return nil
	}
	return &list.Users[idx]
}

func (ts *InvitationTestSuite) request(client *http.Client, method, path string, body interface{}) (
	int, []byte) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		ts.Require().NoError(err)
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, testutils.TestServerURL+path, reader)
	ts.Require().NoError(err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	return resp.StatusCode, raw
}

// invitationRSIdentifier is the identifier of the resource server the admin named name holds permissions on.
func invitationRSIdentifier(name string) string {
	return "https://invitation-test.example.com/" + name
}
