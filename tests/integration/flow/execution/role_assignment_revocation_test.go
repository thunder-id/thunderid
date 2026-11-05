// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const roleTargetInput = "targetRoleId"

const (
	errCodeRoleNotFound                         = "ROL-1003"
	errCodeAdministrationAuthenticationRequired = "FES-1017"
)

const roleFlowResourceID = "https://roleflow.dmv.test"

var (
	roleFlowClient = machineClient{
		id:       "role_admin_flow_client",
		secret:   "role_admin_flow_secret", // #nosec G101 -- test fixture, not a credential
		audience: roleFlowResourceID,
	}
	roleFlowOtherClient = machineClient{
		id:       "role_admin_flow_other_client",
		secret:   "role_admin_flow_other_secret", // #nosec G101 -- test fixture, not a credential
		audience: roleFlowResourceID,
	}
	roleFlowChildClient = machineClient{
		id:       "role_admin_flow_child_client",
		secret:   "role_admin_flow_child_secret", // #nosec G101 -- test fixture, not a credential
		audience: roleFlowResourceID,
	}
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
	// Held only through an assignment made in an organization unit the role is shared with.
	scopeSharedOU = "boating-permit"
)

var allRoleFlowScopes = []string{
	scopeRevocation, scopeRegrant, scopeEmptyRole, scopeUntouched, scopeProtected, scopeOtherAssignee,
	scopeNotAssigned, scopeGroupAssignee, scopeSharedOU,
}

var roleAdminFlowTestOU = testutils.OrganizationUnit{
	Handle:      "role_admin_flow_test_ou",
	Name:        "Test OU for Role Assignment Revocation",
	Description: "Organization unit created for role assignment revocation testing",
	Parent:      nil,
}

// RoleAssignmentRevocationTestSuite removes role assignments through the management API, which revokes
// what each removed assignment granted.
type RoleAssignmentRevocationTestSuite struct {
	suite.Suite
	ouID             string
	resourceServerID string
	resourceIDs      []string
	appID            string
	createdRoleIDs   []string
	createdGroupIDs  []string
	otherAppID       string
}

func TestRoleAssignmentRevocationTestSuite(t *testing.T) {
	suite.Run(t, new(RoleAssignmentRevocationTestSuite))
}

func (ts *RoleAssignmentRevocationTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(roleAdminFlowTestOU)
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

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
					"clientId":                roleFlowClient.id,
					"clientSecret":            roleFlowClient.secret,
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
					"clientId":                roleFlowOtherClient.id,
					"clientSecret":            roleFlowOtherClient.secret,
					"grantTypes":              []string{"client_credentials"},
					"tokenEndpointAuthMethod": "client_secret_basic",
				},
			},
		},
	})
	ts.Require().NoError(err, "Failed to create the second test application")
	ts.otherAppID = otherAppID
}

func (ts *RoleAssignmentRevocationTestSuite) TearDownSuite() {
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

func (ts *RoleAssignmentRevocationTestSuite) TestRemoveAssignment_RevokesTokenAndRemovesAssignment() {
	roleID := ts.createRoleAssignedToApp("Role Flow Revoking Administrator", []string{scopeRevocation})

	token := grantedToken(&ts.Suite, roleFlowClient, scopeRevocation)

	ts.removeAssignment(roleID, "", testutils.Assignment{ID: ts.appID, Type: "app"})

	ts.False(tokenIsActive(&ts.Suite, roleFlowClient, token), "The token carrying the lost scope must be rejected")
	ts.Equal(0, ts.assignmentCount(roleID), "The assignment must be removed")
}

func (ts *RoleAssignmentRevocationTestSuite) TestRemoveAssignment_LaterTokenIsUnaffected() {
	roleID := ts.createRoleAssignedToApp("Role Flow Reassigned Administrator", []string{scopeRegrant})
	firstToken := grantedToken(&ts.Suite, roleFlowClient, scopeRegrant)

	ts.removeAssignment(roleID, "", testutils.Assignment{ID: ts.appID, Type: "app"})
	ts.Require().False(tokenIsActive(&ts.Suite, roleFlowClient, firstToken),
		"The token issued before the unassignment must be rejected")

	secondRoleID := ts.createRoleAssignedToApp("Role Flow Restored Administrator", []string{scopeRegrant})
	ts.Require().NotEmpty(secondRoleID)

	// A token's iat has second granularity while the cutoff is sub-second, and the comparison is at or before.
	time.Sleep(1100 * time.Millisecond)

	// Being accepted when issued is the proof: the revocation row is bounded by its cutoff, not terminal.
	grantedToken(&ts.Suite, roleFlowClient, scopeRegrant)
}

func (ts *RoleAssignmentRevocationTestSuite) TestRemoveAssignment_RoleWithoutPermissions() {
	roleID := ts.createRoleAssignedToApp("Role Flow Empty Administrator", nil)
	ts.Require().Equal(1, ts.assignmentCount(roleID))

	ts.removeAssignment(roleID, "", testutils.Assignment{ID: ts.appID, Type: "app"})

	ts.Equal(0, ts.assignmentCount(roleID), "A role granting nothing must still be unassigned")
}

func (ts *RoleAssignmentRevocationTestSuite) TestRemoveAssignment_UnknownRoleIsNotFound() {
	status, code := removeAssignmentThroughAPI(&ts.Suite, "01900000-0000-7000-8000-0000000000ff", "",
		testutils.Assignment{ID: ts.appID, Type: "app"})

	ts.Equal(http.StatusNotFound, status, "An unknown role must be reported as not found")
	ts.Equal(errCodeRoleNotFound, code)
}

func (ts *RoleAssignmentRevocationTestSuite) TestRemoveAssignment_LeavesOtherAssigneesAlone() {
	roleID := ts.createRoleAssignedToApps("Role Flow Shared Administrator",
		[]string{scopeOtherAssignee},
		testutils.Assignment{ID: ts.appID, Type: "app"},
		testutils.Assignment{ID: ts.otherAppID, Type: "app"})

	departing := grantedToken(&ts.Suite, roleFlowClient, scopeOtherAssignee)
	remaining := grantedToken(&ts.Suite, roleFlowOtherClient, scopeOtherAssignee)

	ts.removeAssignment(roleID, "", testutils.Assignment{ID: ts.appID, Type: "app"})

	ts.False(tokenIsActive(&ts.Suite, roleFlowClient, departing), "The unassigned principal's token must be rejected")
	ts.True(tokenIsActive(&ts.Suite, roleFlowOtherClient, remaining),
		"An assignee that keeps the role must keep the scope it conveys")
	ts.Equal(1, ts.assignmentCount(roleID), "Only the named assignment may be removed")
}

// Removing an assignment the role does not have changes nothing, so it revokes nothing.
func (ts *RoleAssignmentRevocationTestSuite) TestRemoveAssignment_UnassignedPrincipalRevokesNothing() {
	roleID := ts.createRoleAssignedToApps("Role Flow Other Only Administrator",
		[]string{scopeNotAssigned}, testutils.Assignment{ID: ts.otherAppID, Type: "app"})
	// The principal holds the scope through another role, so a wrongly planned revocation would reject this token.
	ts.createRoleAssignedToApp("Role Flow Other Path Administrator", []string{scopeNotAssigned})
	named := grantedToken(&ts.Suite, roleFlowClient, scopeNotAssigned)

	ts.removeAssignment(roleID, "", testutils.Assignment{ID: ts.appID, Type: "app"})

	ts.True(tokenIsActive(&ts.Suite, roleFlowClient, named),
		"Removing an assignment the role does not have must not revoke the scope held through another role")
	assignments, err := testutils.GetRoleAssignments(roleID)
	ts.Require().NoError(err, "Failed to read role assignments")
	ts.Require().Len(assignments, 1, "The role's assignments must be left as they were")
	ts.Equal(ts.otherAppID, assignments[0].ID, "The role's actual assignee must remain assigned")
}

func (ts *RoleAssignmentRevocationTestSuite) TestRemoveAssignment_GroupAssigneeRevokesItsMembers() {
	groupID, err := testutils.CreateGroup(testutils.Group{
		Name:        "Role Flow Assigned Group",
		Description: "Group created for role assignment revocation testing",
		OUID:        ts.ouID,
		Members:     []testutils.Member{{Id: ts.appID, Type: "app"}},
	})
	ts.Require().NoError(err, "Failed to create test group")
	ts.createdGroupIDs = append(ts.createdGroupIDs, groupID)
	roleID := ts.createRoleAssignedToApps("Role Flow Group Assigned Administrator",
		[]string{scopeGroupAssignee}, testutils.Assignment{ID: groupID, Type: "group"})

	token := grantedToken(&ts.Suite, roleFlowClient, scopeGroupAssignee)

	ts.removeAssignment(roleID, "", testutils.Assignment{ID: groupID, Type: "group"})

	ts.False(tokenIsActive(&ts.Suite, roleFlowClient, token),
		"A member of the unassigned group must lose the scope the role conveyed")
	ts.Equal(0, ts.assignmentCount(roleID), "The group's assignment must be removed")
	members, err := testutils.GetGroupMembers(groupID)
	ts.Require().NoError(err, "Failed to read the group's members")
	ts.Len(members, 1, "Unassigning the group must not change its membership")
}

// An assignment made in an organization unit the role is shared with is removed where it was made, and
// only when the request names that unit: without it the removal looks in the role's owner and finds
// nothing to remove or revoke.
func (ts *RoleAssignmentRevocationTestSuite) TestRemoveAssignment_RemovesAnAssignmentMadeInASharedOU() {
	childOUID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "role_admin_flow_child_ou",
		Name:        "Child OU for Role Assignment Revocation",
		Description: "Organization unit a role is shared with, for role assignment revocation testing",
		Parent:      &ts.ouID,
	})
	ts.Require().NoError(err, "Failed to create the child organization unit")
	defer func() {
		if err := testutils.DeleteOrganizationUnit(childOUID); err != nil {
			ts.T().Logf("Failed to delete the child organization unit: %v", err)
		}
	}()

	childAppID, err := testutils.CreateApplication(testutils.Application{
		Name:        "Role Flow Child Machine Client",
		Description: "Principal owned by the organization unit the role is shared with",
		OUID:        childOUID,
		InboundAuthConfig: []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                roleFlowChildClient.id,
					"clientSecret":            roleFlowChildClient.secret,
					"grantTypes":              []string{"client_credentials"},
					"tokenEndpointAuthMethod": "client_secret_basic",
				},
			},
		},
	})
	ts.Require().NoError(err, "Failed to create the child application")
	defer func() {
		if err := testutils.DeleteApplication(childAppID); err != nil {
			ts.T().Logf("Failed to delete the child application: %v", err)
		}
	}()

	roleID, err := testutils.CreateRole(testutils.Role{
		Name:        "Role Flow Shared OU Administrator",
		Description: "Role created for role assignment revocation testing",
		OUID:        ts.ouID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: ts.resourceServerID, Permissions: []string{scopeSharedOU}},
		},
	})
	ts.Require().NoError(err, "Failed to create test role")
	defer func() {
		if err := testutils.DeleteRole(roleID); err != nil {
			ts.T().Logf("Failed to delete test role %s: %v", roleID, err)
		}
	}()
	ts.postRoleJSON(roleID, "sharing-policies", http.StatusCreated, map[string]interface{}{
		"childUnits": map[string]interface{}{
			"mode": "selected", "units": []map[string]string{{"ouId": childOUID, "reach": "unit"}},
		},
	})
	ts.postRoleJSON(roleID, "assignments/add", http.StatusNoContent, map[string]interface{}{
		"ouId": childOUID, "assignments": []map[string]string{{"id": childAppID, "type": "app"}},
	})

	token := grantedToken(&ts.Suite, roleFlowChildClient, scopeSharedOU)
	child := testutils.Assignment{ID: childAppID, Type: "app"}

	ts.removeAssignment(roleID, "", child)

	ts.True(tokenIsActive(&ts.Suite, roleFlowChildClient, token),
		"Without the organization unit nothing is removed, so nothing may be revoked")
	ts.Equal(1, ts.assignmentCount(roleID), "The assignment made in the shared organization unit must remain")

	ts.removeAssignment(roleID, childOUID, child)

	ts.False(tokenIsActive(&ts.Suite, roleFlowChildClient, token), "The token carrying the lost scope must be rejected")
	ts.Equal(0, ts.assignmentCount(roleID), "The assignment made in the shared organization unit must be removed")
}

// postRoleJSON posts body to a path beneath the role and requires the given status.
func (ts *RoleAssignmentRevocationTestSuite) postRoleJSON(roleID, path string, status int, body interface{}) {
	ts.T().Helper()

	payload, err := json.Marshal(body)
	ts.Require().NoError(err)
	req, err := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/roles/%s/%s", testutils.TestServerURL, roleID, path), bytes.NewReader(payload))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(status, resp.StatusCode, "POST /roles/%s/%s", roleID, path)
}

func (ts *RoleAssignmentRevocationTestSuite) createRoleAssignedToApp(
	name string, permissions []string) string {
	ts.T().Helper()
	return ts.createRoleAssignedToApps(name, permissions, testutils.Assignment{ID: ts.appID, Type: "app"})
}

func (ts *RoleAssignmentRevocationTestSuite) createRoleAssignedToApps(
	name string, permissions []string, assignments ...testutils.Assignment) string {
	ts.T().Helper()
	return createRole(&ts.Suite, &ts.createdRoleIDs, ts.ouID, ts.resourceServerID, name, permissions, assignments...)
}

func (ts *RoleAssignmentRevocationTestSuite) assignmentCount(roleID string) int {
	ts.T().Helper()

	assignments, err := testutils.GetRoleAssignments(roleID)
	ts.Require().NoError(err, "Failed to read role assignments")
	return len(assignments)
}

// removeAssignment removes the assignment through the management API and requires it to succeed.
func (ts *RoleAssignmentRevocationTestSuite) removeAssignment(roleID, ouID string, assignee testutils.Assignment) {
	ts.T().Helper()

	status, code := removeAssignmentThroughAPI(&ts.Suite, roleID, ouID, assignee)
	ts.Require().Equal(http.StatusNoContent, status, "Removing the assignment failed with %s", code)
}
