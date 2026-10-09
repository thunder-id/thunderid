// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const roleAssignmentRemovalFlowHandle = "default-role-assignment-removal-flow"

const (
	roleTargetInput     = "targetRoleId"
	assigneeTargetInput = "targetAssigneeId"
)

const (
	errCodeRoleNotFound                         = "ROL-1003"
	errCodeRoleAssignmentNotFound               = "ROL-1020"
	errCodeAdministrationAuthenticationRequired = "FES-1017"
)

const (
	roleFlowClientID     = "role_admin_flow_client"
	roleFlowClientSecret = "role_admin_flow_secret" // #nosec G101 -- test fixture, not a credential
	roleFlowResourceID   = "https://roleflow.dmv.test"
	// #nosec G101 -- test fixtures, not credentials
	roleFlowOtherClientID     = "role_admin_flow_other_client"
	roleFlowOtherClientSecret = "role_admin_flow_other_secret"
)

const (
	scopeOtherAssignee = "boat-license"
	scopeRevocation    = "license"
	scopeRegrant       = "permits"
	scopeEmptyRole     = "voter-roll"
	scopeUntouched     = "elections"
	scopeProtected     = "vital-records"
	// Held only by the second principal, through a role the first was never given.
	scopeNotAssigned   = "fishing-license"
	scopeGroupAssignee = "hunting-license"
)

var allRoleFlowScopes = []string{
	scopeRevocation, scopeRegrant, scopeEmptyRole, scopeUntouched, scopeProtected, scopeOtherAssignee,
	scopeNotAssigned, scopeGroupAssignee,
}

var roleAdminFlowTestOU = testutils.OrganizationUnit{
	Handle:      "role_admin_flow_test_ou",
	Name:        "Test OU for Role Administration Flows",
	Description: "Organization unit created for role administration flow testing",
	Parent:      nil,
}

type RoleAdministrationFlowTestSuite struct {
	suite.Suite
	ouID             string
	flowID           string
	resourceServerID string
	resourceIDs      []string
	appID            string
	createdRoleIDs   []string
	createdGroupIDs  []string
	otherAppID       string
}

func TestRoleAdministrationFlowTestSuite(t *testing.T) {
	suite.Run(t, new(RoleAdministrationFlowTestSuite))
}

func (ts *RoleAdministrationFlowTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(roleAdminFlowTestOU)
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

	flowID, err := testutils.GetFlowIDByHandle(roleAssignmentRemovalFlowHandle, administrationFlowType)
	ts.Require().NoError(err, "Failed to resolve the shipped role assignment removal flow")
	ts.Require().NotEmpty(flowID, "The shipped role assignment removal flow must be present")
	ts.flowID = flowID

	rsID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:        "Role Flow DMV API",
		Description: "Resource server created for role administration flow testing",
		Identifier:  roleFlowResourceID,
		OUID:        ts.ouID,
	}, nil)
	ts.Require().NoError(err, "Failed to create test resource server")
	ts.resourceServerID = rsID

	for _, scope := range allRoleFlowScopes {
		resourceID, resErr := testutils.CreateResource(ts.resourceServerID, scope, scope, "")
		ts.Require().NoError(resErr, "Failed to create test resource %s", scope)
		ts.resourceIDs = append(ts.resourceIDs, resourceID)
	}

	// A machine client can hold a role and get scoped tokens without a login flow.
	appID, err := testutils.CreateApplication(testutils.Application{
		Name:        "Role Flow Machine Client",
		Description: "Application created for role administration flow testing",
		OUID:        ts.ouID,
		InboundAuthConfig: []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                roleFlowClientID,
					"clientSecret":            roleFlowClientSecret,
					"grantTypes":              []string{"client_credentials"},
					"tokenEndpointAuthMethod": "client_secret_basic",
				},
			},
		},
	})
	ts.Require().NoError(err, "Failed to create test application")
	ts.appID = appID

	otherAppID, err := testutils.CreateApplication(testutils.Application{
		Name:        "Role Flow Second Machine Client",
		Description: "Second principal, used to prove an unassignment did not reach another assignee",
		OUID:        ts.ouID,
		InboundAuthConfig: []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                roleFlowOtherClientID,
					"clientSecret":            roleFlowOtherClientSecret,
					"grantTypes":              []string{"client_credentials"},
					"tokenEndpointAuthMethod": "client_secret_basic",
				},
			},
		},
	})
	ts.Require().NoError(err, "Failed to create the second test application")
	ts.otherAppID = otherAppID
}

func (ts *RoleAdministrationFlowTestSuite) TearDownSuite() {
	for _, roleID := range ts.createdRoleIDs {
		if err := testutils.DeleteRole(roleID); err != nil {
			ts.T().Logf("Failed to delete test role %s during teardown: %v", roleID, err)
		}
	}
	for _, groupID := range ts.createdGroupIDs {
		if err := testutils.DeleteGroup(groupID); err != nil {
			ts.T().Logf("Failed to delete test group %s during teardown: %v", groupID, err)
		}
	}
	if ts.appID != "" {
		if err := testutils.DeleteApplication(ts.appID); err != nil {
			ts.T().Logf("Failed to delete test application during teardown: %v", err)
		}
	}
	if ts.otherAppID != "" {
		if err := testutils.DeleteApplication(ts.otherAppID); err != nil {
			ts.T().Logf("Failed to delete the second test application during teardown: %v", err)
		}
	}
	// Resources first: a resource server with dependencies cannot be deleted.
	for _, resourceID := range ts.resourceIDs {
		if err := testutils.DeleteResource(ts.resourceServerID, resourceID); err != nil {
			ts.T().Logf("Failed to delete test resource %s during teardown: %v", resourceID, err)
		}
	}
	if ts.resourceServerID != "" {
		if err := testutils.DeleteResourceServer(ts.resourceServerID); err != nil {
			ts.T().Logf("Failed to delete test resource server during teardown: %v", err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}
}

func (ts *RoleAdministrationFlowTestSuite) createRoleAssignedToApp(
	name string, permissions []string) string {
	ts.T().Helper()

	role := testutils.Role{
		Name:        name,
		Description: "Role created for role administration flow testing",
		OUID:        ts.ouID,
		Assignments: []testutils.Assignment{{ID: ts.appID, Type: "app"}},
	}
	if len(permissions) > 0 {
		role.Permissions = []testutils.ResourcePermissions{
			{ResourceServerID: ts.resourceServerID, Permissions: permissions},
		}
	}

	roleID, err := testutils.CreateRole(role)
	ts.Require().NoError(err, "Failed to create test role")
	ts.createdRoleIDs = append(ts.createdRoleIDs, roleID)
	return roleID
}

func (ts *RoleAdministrationFlowTestSuite) tokenFor(clientID, clientSecret, scope string) (string, string) {
	ts.T().Helper()

	result, err := testutils.RequestClientCredentialsToken(clientID, clientSecret, scope, roleFlowResourceID)
	ts.Require().NoError(err, "Token request failed")
	ts.Require().Equal(http.StatusOK, result.StatusCode, "Token response: %s", string(result.Body))
	return result.Token.AccessToken, result.Token.Scope
}

func (ts *RoleAdministrationFlowTestSuite) tokenWithScope(scope string) (string, string) {
	ts.T().Helper()
	return ts.tokenFor(roleFlowClientID, roleFlowClientSecret, scope)
}

func (ts *RoleAdministrationFlowTestSuite) tokenIsActiveForClient(token, clientID, clientSecret string) bool {
	ts.T().Helper()

	active, err := testutils.IntrospectToken(token, clientID, clientSecret)
	ts.Require().NoError(err, "Introspection request failed")
	return active
}

func (ts *RoleAdministrationFlowTestSuite) tokenIsActive(token string) bool {
	ts.T().Helper()
	return ts.tokenIsActiveForClient(token, roleFlowClientID, roleFlowClientSecret)
}

func (ts *RoleAdministrationFlowTestSuite) assignmentCount(roleID string) int {
	ts.T().Helper()

	assignments, err := testutils.GetRoleAssignments(roleID)
	ts.Require().NoError(err, "Failed to read role assignments")
	return len(assignments)
}

func (ts *RoleAdministrationFlowTestSuite) executeRoleFlow(roleID, assigneeID string) *common.FlowStep {
	ts.T().Helper()
	return ts.executeAsAdmin(map[string]string{roleTargetInput: roleID, assigneeTargetInput: assigneeID})
}

func (ts *RoleAdministrationFlowTestSuite) executeAsAdmin(inputs map[string]string) *common.FlowStep {
	ts.T().Helper()

	result, err := common.ExecuteAdministrationFlow(ts.flowID, inputs, true)
	ts.Require().NoError(err, "Failed to execute the role assignment removal flow")
	ts.Require().Equal(http.StatusOK, result.StatusCode, "Flow error response: %+v", result.Error)
	return result.Step
}

func (ts *RoleAdministrationFlowTestSuite) TestRoleAssignmentRemovalFlow_RevokesTokenAndRemovesAssignment() {
	roleID := ts.createRoleAssignedToApp("Role Flow Revoking Administrator", []string{scopeRevocation})

	token, scope := ts.tokenWithScope(scopeRevocation)
	ts.Require().NotEmpty(token, "The client must obtain a token")
	ts.Require().Contains(scope, scopeRevocation, "The token must carry the scope the role grants")
	ts.Require().True(ts.tokenIsActive(token), "The token must be accepted before the flow runs")

	step := ts.executeRoleFlow(roleID, ts.appID)

	ts.Equal("COMPLETE", step.FlowStatus, "The flow must complete")
	ts.False(ts.tokenIsActive(token), "The token carrying the lost scope must be rejected")
	ts.Equal(0, ts.assignmentCount(roleID), "The assignment must be removed")
}

func (ts *RoleAdministrationFlowTestSuite) TestRoleAssignmentRemovalFlow_LaterTokenIsUnaffected() {
	roleID := ts.createRoleAssignedToApp("Role Flow Reassigned Administrator", []string{scopeRegrant})
	firstToken, _ := ts.tokenWithScope(scopeRegrant)
	ts.Require().True(ts.tokenIsActive(firstToken))

	step := ts.executeRoleFlow(roleID, ts.appID)
	ts.Require().Equal("COMPLETE", step.FlowStatus)
	ts.Require().False(ts.tokenIsActive(firstToken))

	secondRoleID := ts.createRoleAssignedToApp("Role Flow Restored Administrator", []string{scopeRegrant})
	ts.Require().NotEmpty(secondRoleID)

	// A token's iat has second granularity while the cutoff is sub-second, and the comparison is at or before.
	time.Sleep(1100 * time.Millisecond)

	laterToken, scope := ts.tokenWithScope(scopeRegrant)
	ts.Require().Contains(scope, scopeRegrant, "The regranted scope must be issued again")
	ts.True(ts.tokenIsActive(laterToken),
		"A token established after the revocation must pass: the row is bounded, not terminal")
}

func (ts *RoleAdministrationFlowTestSuite) TestRoleAssignmentRemovalFlow_RoleWithoutPermissions() {
	roleID := ts.createRoleAssignedToApp("Role Flow Empty Administrator", nil)
	ts.Require().Equal(1, ts.assignmentCount(roleID))

	step := ts.executeRoleFlow(roleID, ts.appID)

	ts.Equal("COMPLETE", step.FlowStatus, "A role granting nothing must still be unassigned")
	ts.Equal(0, ts.assignmentCount(roleID))
}

func (ts *RoleAdministrationFlowTestSuite) TestRoleAssignmentRemovalFlow_UnknownRoleCarriesValidatorCode() {
	step := ts.executeRoleFlow("01900000-0000-7000-8000-0000000000ff", ts.appID)

	ts.NotEqual("COMPLETE", step.FlowStatus, "An unknown role must not complete")
	ts.Require().NotNil(step.Error, "The refusal must be reported in the error envelope")
	ts.Equal(errCodeRoleNotFound, step.Error.Code,
		"The validator's own code must reach the caller, not the executor's generic one")
}

func (ts *RoleAdministrationFlowTestSuite) TestRoleAssignmentRemovalFlow_MissingTargetDoesNotComplete() {
	roleID := ts.createRoleAssignedToApp("Role Flow Untouched Administrator", []string{scopeUntouched})

	step := ts.executeAsAdmin(map[string]string{})

	ts.NotEqual("COMPLETE", step.FlowStatus, "A flow with no target must not complete")
	ts.Equal(1, ts.assignmentCount(roleID), "The assignment must be untouched")
}

// Refused at the flow execution boundary, before the PermissionValidator node runs.
func (ts *RoleAdministrationFlowTestSuite) TestRoleAssignmentRemovalFlow_RejectsAnonymousCaller() {
	roleID := ts.createRoleAssignedToApp("Role Flow Protected Administrator", []string{scopeProtected})

	result, err := common.ExecuteAdministrationFlow(ts.flowID, map[string]string{
		roleTargetInput: roleID, assigneeTargetInput: ts.appID,
	}, false)
	ts.Require().NoError(err, "Failed to execute the role assignment removal flow anonymously")

	ts.Equal(http.StatusUnauthorized, result.StatusCode, "An anonymous caller must be refused as unauthenticated")
	ts.Nil(result.Step, "An anonymous caller must not be given a flow step")
	ts.Require().NotNil(result.Error, "The refusal must be reported as an API error")
	ts.Equal(errCodeAdministrationAuthenticationRequired, result.Error.Code,
		"The refusal must be the missing-authentication one, not a refusal for any other reason")
	ts.Equal(1, ts.assignmentCount(roleID), "The assignment must be untouched")
}

func (ts *RoleAdministrationFlowTestSuite) TestRoleAssignmentRemovalFlow_LeavesOtherAssigneesAlone() {
	roleID := ts.createRoleAssignedToApps("Role Flow Shared Administrator",
		[]string{scopeOtherAssignee},
		testutils.Assignment{ID: ts.appID, Type: "app"},
		testutils.Assignment{ID: ts.otherAppID, Type: "app"})

	departing, scope := ts.tokenWithScope(scopeOtherAssignee)
	ts.Require().Contains(scope, scopeOtherAssignee)
	remaining, otherScope := ts.tokenFor(roleFlowOtherClientID, roleFlowOtherClientSecret, scopeOtherAssignee)
	ts.Require().Contains(otherScope, scopeOtherAssignee, "The second assignee must be granted the scope too")
	ts.Require().True(ts.tokenIsActive(departing))
	ts.Require().True(
		ts.tokenIsActiveForClient(remaining, roleFlowOtherClientID, roleFlowOtherClientSecret))

	step := ts.executeRoleFlow(roleID, ts.appID)

	ts.Require().Equal("COMPLETE", step.FlowStatus)
	ts.False(ts.tokenIsActive(departing), "The unassigned principal's token must be rejected")
	ts.True(ts.tokenIsActiveForClient(remaining, roleFlowOtherClientID, roleFlowOtherClientSecret),
		"An assignee that keeps the role must keep the scope it conveys")
	ts.Equal(1, ts.assignmentCount(roleID), "Only the named assignment may be removed")
}

func (ts *RoleAdministrationFlowTestSuite) createRoleAssignedToApps(
	name string, permissions []string, assignments ...testutils.Assignment) string {
	ts.T().Helper()

	roleID, err := testutils.CreateRole(testutils.Role{
		Name:        name,
		Description: "Role created for role administration flow testing",
		OUID:        ts.ouID,
		Assignments: assignments,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: ts.resourceServerID, Permissions: permissions},
		},
	})
	ts.Require().NoError(err, "Failed to create test role")
	ts.createdRoleIDs = append(ts.createdRoleIDs, roleID)
	return roleID
}

func (ts *RoleAdministrationFlowTestSuite) TestRoleAssignmentRemovalFlow_UnassignedPrincipalIsRefused() {
	roleID := ts.createRoleAssignedToApps("Role Flow Other Only Administrator",
		[]string{scopeNotAssigned}, testutils.Assignment{ID: ts.otherAppID, Type: "app"})
	// The principal holds the scope through another role, so a wrongly planned revocation would reject this token.
	ts.createRoleAssignedToApp("Role Flow Other Path Administrator", []string{scopeNotAssigned})
	named, scope := ts.tokenWithScope(scopeNotAssigned)
	ts.Require().Contains(scope, scopeNotAssigned, "The named principal must hold the scope through its own role")
	ts.Require().True(ts.tokenIsActive(named), "The token must be accepted before the flow runs")

	step := ts.executeRoleFlow(roleID, ts.appID)

	ts.NotEqual("COMPLETE", step.FlowStatus, "Removing an assignment that does not exist must not complete")
	ts.Require().NotNil(step.Error, "The refusal must be reported in the error envelope")
	ts.Equal(errCodeRoleAssignmentNotFound, step.Error.Code,
		"The validator's own code must reach the caller, not the executor's generic one")
	ts.True(ts.tokenIsActive(named),
		"A refused unassignment must not revoke the scope the named principal holds through another role")
	assignments, err := testutils.GetRoleAssignments(roleID)
	ts.Require().NoError(err, "Failed to read role assignments")
	ts.Require().Len(assignments, 1, "A refused unassignment must leave the role's assignments as they were")
	ts.Equal(ts.otherAppID, assignments[0].ID, "The role's actual assignee must remain assigned")
}

func (ts *RoleAdministrationFlowTestSuite) TestRoleAssignmentRemovalFlow_GroupAssigneeRevokesItsMembers() {
	groupID, err := testutils.CreateGroup(testutils.Group{
		Name:        "Role Flow Assigned Group",
		Description: "Group created for role administration flow testing",
		OUID:        ts.ouID,
		Members:     []testutils.Member{{Id: ts.appID, Type: "app"}},
	})
	ts.Require().NoError(err, "Failed to create test group")
	ts.createdGroupIDs = append(ts.createdGroupIDs, groupID)
	roleID := ts.createRoleAssignedToApps("Role Flow Group Assigned Administrator",
		[]string{scopeGroupAssignee}, testutils.Assignment{ID: groupID, Type: "group"})

	token, scope := ts.tokenWithScope(scopeGroupAssignee)
	ts.Require().Contains(scope, scopeGroupAssignee, "The group's member must be granted the scope")
	ts.Require().True(ts.tokenIsActive(token), "The token must be accepted before the flow runs")

	step := ts.executeRoleFlow(roleID, groupID)

	ts.Equal("COMPLETE", step.FlowStatus, "The flow must complete")
	ts.False(ts.tokenIsActive(token), "A member of the unassigned group must lose the scope the role conveyed")
	ts.Equal(0, ts.assignmentCount(roleID), "The group's assignment must be removed")
	members, err := testutils.GetGroupMembers(groupID)
	ts.Require().NoError(err, "Failed to read the group's members")
	ts.Len(members, 1, "Unassigning the group must not change its membership")
}
