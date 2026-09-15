// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// The shipped role administration flow. Executed by handle so the test does not depend on the seeded
// flow id, which is a bootstrap detail.
const roleAssignmentRemovalFlowHandle = "default-role-assignment-removal-flow"

// The input identifiers the role executors read their target from. Like the application ones they
// deliberately avoid names the execution request already uses at its top level.
const (
	roleTargetInput     = "targetRoleId"
	assigneeTargetInput = "targetAssigneeId"
)

// The refusal code the preparatory node carries out of the role validator, rather than collapsing it
// into the executor's own. It is what lets a console tell an unknown role from a refused removal.
const errCodeRoleNotFound = "ROL-1003"

const (
	roleFlowClientID     = "role_admin_flow_client"
	roleFlowClientSecret = "role_admin_flow_secret" // #nosec G101 -- test fixture, not a credential
	roleFlowResourceID   = "https://roleflow.dmv.test"
	// #nosec G101 -- test fixtures, not credentials
	roleFlowOtherClientID     = "role_admin_flow_other_client"
	roleFlowOtherClientSecret = "role_admin_flow_other_secret"
)

// Each test uses its own scope. The criterion is keyed on (entity, audience, scope) and every test
// here shares one machine client and one resource server, so a shared scope would let one test's
// revocation deny another test's token.
const (
	// scopeOtherAssignee is held by a second principal through the same role, so unassigning one
	// assignee can be shown not to reach the other.
	scopeOtherAssignee = "boat-license"
	scopeRevocation    = "license"
	scopeRegrant       = "permits"
	scopeEmptyRole     = "voter-roll"
	scopeUntouched     = "elections"
	scopeProtected     = "vital-records"
)

// allRoleFlowScopes is every scope the suite defines on its resource server.
var allRoleFlowScopes = []string{
	scopeRevocation, scopeRegrant, scopeEmptyRole, scopeUntouched, scopeProtected, scopeOtherAssignee,
}

var roleAdminFlowTestOU = testutils.OrganizationUnit{
	Handle:      "role_admin_flow_test_ou",
	Name:        "Test OU for Role Administration Flows",
	Description: "Organization unit created for role administration flow testing",
	Parent:      nil,
}

type RoleAdministrationFlowTestSuite struct {
	suite.Suite
	client           *http.Client
	ouID             string
	flowID           string
	resourceServerID string
	resourceIDs      []string
	appID            string
	createdRoleIDs   []string
	// A second machine client, so a revocation aimed at one assignee can be shown not to reach another.
	// A criterion is keyed on the entity, so this needs a different principal rather than a second
	// token of the same one.
	otherAppID string
}

func TestRoleAdministrationFlowTestSuite(t *testing.T) {
	suite.Run(t, new(RoleAdministrationFlowTestSuite))
}

func (ts *RoleAdministrationFlowTestSuite) SetupSuite() {
	ts.client = testutils.GetRawHTTPClient()

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

	// A machine client is the cheapest principal that can hold a role and obtain a token carrying its
	// scopes, with no login flow in the way.
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
	// The resources must go first: a resource server with dependencies cannot be deleted.
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

// createRoleAssignedToApp creates a role granting the given permissions to the test application and
// registers it for teardown.
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

// tokenWithScope obtains a client credentials token bound to the test resource server, and returns it
// with the scopes actually granted.
func (ts *RoleAdministrationFlowTestSuite) tokenWithScope(scope string) (string, string) {
	ts.T().Helper()

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("scope", scope)
	form.Set("resource", roleFlowResourceID)

	req, err := http.NewRequest(http.MethodPost, testServerURL+"/oauth2/token",
		strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(roleFlowClientID, roleFlowClientSecret)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err, "Token request failed")
	defer resp.Body.Close()

	var body struct {
		AccessToken string `json:"access_token"`
		Scope       string `json:"scope"`
	}
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&body))
	return body.AccessToken, body.Scope
}

// tokenIsActive reports what the Authorization Server says about the token now. Introspection is used
// rather than a Resource Server call because it reflects a revocation immediately, while the Resource
// Server cache is only eventually consistent.
func (ts *RoleAdministrationFlowTestSuite) tokenIsActive(token string) bool {
	ts.T().Helper()

	form := url.Values{}
	form.Set("token", token)

	req, err := http.NewRequest(http.MethodPost, testServerURL+"/oauth2/introspect",
		strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(roleFlowClientID, roleFlowClientSecret)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err, "Introspection request failed")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, resp.StatusCode, "Introspection response: %s", string(body))

	var result struct {
		Active bool `json:"active"`
	}
	ts.Require().NoError(json.Unmarshal(body, &result), "Introspection response: %s", string(body))
	return result.Active
}

// assignmentCount returns how many assignees the role still has.
func (ts *RoleAdministrationFlowTestSuite) assignmentCount(roleID string) int {
	ts.T().Helper()

	assignments, err := testutils.GetRoleAssignments(roleID)
	ts.Require().NoError(err, "Failed to read role assignments")
	return len(assignments)
}

// executeRoleFlow runs the role assignment removal flow as an administrator.
func (ts *RoleAdministrationFlowTestSuite) executeRoleFlow(
	roleID, assigneeID string) (int, map[string]interface{}) {
	ts.T().Helper()
	return executeAdminFlowRequest(ts.T(), ts.client, ts.flowID, map[string]string{
		roleTargetInput: roleID, assigneeTargetInput: assigneeID,
	}, true)
}

// The flow's whole purpose: the token minted before the unassignment must stop being accepted, and the
// assignment must be gone.
func (ts *RoleAdministrationFlowTestSuite) TestRoleAssignmentRemovalFlow_RevokesTokenAndRemovesAssignment() {
	roleID := ts.createRoleAssignedToApp("Role Flow Revoking Administrator", []string{scopeRevocation})

	token, scope := ts.tokenWithScope(scopeRevocation)
	ts.Require().NotEmpty(token, "The client must obtain a token")
	ts.Require().Contains(scope, scopeRevocation, "The token must carry the scope the role grants")
	ts.Require().True(ts.tokenIsActive(token), "The token must be accepted before the flow runs")

	status, step := ts.executeRoleFlow(roleID, ts.appID)

	ts.Require().Equal(http.StatusOK, status)
	ts.Equal("COMPLETE", step["flowStatus"], "The flow must complete")
	ts.False(ts.tokenIsActive(token), "The token carrying the lost scope must be rejected")
	ts.Equal(0, ts.assignmentCount(roleID), "The assignment must be removed")
}

// A token minted after the unassignment is legitimately entitled to whatever the principal still
// holds, so the boundary cutoff must let it through rather than denying the scope name outright.
func (ts *RoleAdministrationFlowTestSuite) TestRoleAssignmentRemovalFlow_LaterTokenIsUnaffected() {
	roleID := ts.createRoleAssignedToApp("Role Flow Reassigned Administrator", []string{scopeRegrant})
	firstToken, _ := ts.tokenWithScope(scopeRegrant)
	ts.Require().True(ts.tokenIsActive(firstToken))

	status, step := ts.executeRoleFlow(roleID, ts.appID)
	ts.Require().Equal(http.StatusOK, status)
	ts.Require().Equal("COMPLETE", step["flowStatus"])
	ts.Require().False(ts.tokenIsActive(firstToken))

	// Restore the grant, exactly as an operator correcting a mistake would.
	secondRoleID := ts.createRoleAssignedToApp("Role Flow Restored Administrator", []string{scopeRegrant})
	ts.Require().NotEmpty(secondRoleID)

	// A token's iat has second granularity while the cutoff is recorded with sub-second precision, and
	// the comparison is "at or before". A token minted in the same second as the revocation is
	// therefore denied, which errs toward over-revocation and is the safe direction. Wait past the
	// boundary so this asserts the regrant rather than that rounding.
	time.Sleep(1100 * time.Millisecond)

	laterToken, scope := ts.tokenWithScope(scopeRegrant)
	ts.Require().Contains(scope, scopeRegrant, "The regranted scope must be issued again")
	ts.True(ts.tokenIsActive(laterToken),
		"A token established after the revocation must pass: the row is bounded, not terminal")
}

// A role that grants nothing has no scopes to deny, but the unassignment must still happen. This is
// the plan's NothingToRevoke path.
func (ts *RoleAdministrationFlowTestSuite) TestRoleAssignmentRemovalFlow_RoleWithoutPermissions() {
	roleID := ts.createRoleAssignedToApp("Role Flow Empty Administrator", nil)
	ts.Require().Equal(1, ts.assignmentCount(roleID))

	status, step := ts.executeRoleFlow(roleID, ts.appID)

	ts.Require().Equal(http.StatusOK, status)
	ts.Equal("COMPLETE", step["flowStatus"], "A role granting nothing must still be unassigned")
	ts.Equal(0, ts.assignmentCount(roleID))
}

// The validator's own code must survive out of the preparatory node, so a console can say which of the
// refusals happened rather than showing one generic message.
func (ts *RoleAdministrationFlowTestSuite) TestRoleAssignmentRemovalFlow_UnknownRoleCarriesValidatorCode() {
	status, step := ts.executeRoleFlow("01900000-0000-7000-8000-0000000000ff", ts.appID)

	ts.Require().Equal(http.StatusOK, status)
	ts.NotEqual("COMPLETE", step["flowStatus"], "An unknown role must not complete")
	errObj, ok := step["error"].(map[string]interface{})
	ts.Require().True(ok, "The refusal must be reported in the error envelope")
	ts.Equal(errCodeRoleNotFound, errObj["code"],
		"The validator's own code must reach the caller, not the executor's generic one")
}

// Without a target the preparatory node has nothing to plan against, so the flow must not proceed to
// the change.
func (ts *RoleAdministrationFlowTestSuite) TestRoleAssignmentRemovalFlow_MissingTargetDoesNotComplete() {
	roleID := ts.createRoleAssignedToApp("Role Flow Untouched Administrator", []string{scopeUntouched})

	_, step := executeAdminFlowRequest(ts.T(), ts.client, ts.flowID, map[string]string{}, true)

	ts.NotEqual("COMPLETE", step["flowStatus"], "A flow with no target must not complete")
	ts.Equal(1, ts.assignmentCount(roleID), "The assignment must be untouched")
}

// The flow's PermissionValidator node is the only thing standing between an anonymous caller and a
// privileged change, so it must refuse one.
func (ts *RoleAdministrationFlowTestSuite) TestRoleAssignmentRemovalFlow_RejectsAnonymousCaller() {
	roleID := ts.createRoleAssignedToApp("Role Flow Protected Administrator", []string{scopeProtected})

	_, step := executeAdminFlowRequest(ts.T(), ts.client, ts.flowID, map[string]string{
		roleTargetInput: roleID, assigneeTargetInput: ts.appID,
	}, false)

	ts.NotEqual("COMPLETE", step["flowStatus"], "An anonymous caller must not complete the flow")
	ts.Equal(1, ts.assignmentCount(roleID), "The assignment must be untouched")
}

// A criterion is keyed on the principal, so unassigning one assignee must leave every other assignee
// of the same role holding the same scope. A design that revoked on the role rather than on the
// principal-scope pair would take them all down together.
func (ts *RoleAdministrationFlowTestSuite) TestRoleAssignmentRemovalFlow_LeavesOtherAssigneesAlone() {
	roleID := ts.createRoleAssignedToApps("Role Flow Shared Administrator",
		[]string{scopeOtherAssignee},
		testutils.Assignment{ID: ts.appID, Type: "app"},
		testutils.Assignment{ID: ts.otherAppID, Type: "app"})

	departing, scope := ts.tokenWithScope(scopeOtherAssignee)
	ts.Require().Contains(scope, scopeOtherAssignee)
	remaining := ts.tokenForClient(roleFlowOtherClientID, roleFlowOtherClientSecret, scopeOtherAssignee)
	ts.Require().True(ts.tokenIsActive(departing))
	ts.Require().True(
		ts.tokenIsActiveForClient(remaining, roleFlowOtherClientID, roleFlowOtherClientSecret))

	status, step := ts.executeRoleFlow(roleID, ts.appID)

	ts.Require().Equal(http.StatusOK, status)
	ts.Require().Equal("COMPLETE", step["flowStatus"])
	ts.False(ts.tokenIsActive(departing), "The unassigned principal's token must be rejected")
	ts.True(ts.tokenIsActiveForClient(remaining, roleFlowOtherClientID, roleFlowOtherClientSecret),
		"An assignee that keeps the role must keep the scope it conveys")
	ts.Equal(1, ts.assignmentCount(roleID), "Only the named assignment may be removed")
}

// createRoleAssignedToApps creates a role granting the permissions to the given assignees.
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

// tokenForClient obtains a client credentials token for an arbitrary client, which the second
// assignee needs: a criterion is keyed on the entity, so the bystander must be its own principal.
func (ts *RoleAdministrationFlowTestSuite) tokenForClient(
	clientID, clientSecret, scope string) string {
	ts.T().Helper()

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("scope", scope)
	form.Set("resource", roleFlowResourceID)

	req, err := http.NewRequest(http.MethodPost, testServerURL+"/oauth2/token",
		strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err, "Token request failed")
	defer resp.Body.Close()

	var body struct {
		AccessToken string `json:"access_token"`
		Scope       string `json:"scope"`
	}
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&body))
	ts.Require().Contains(body.Scope, scope, "The second assignee must be granted the scope too")
	return body.AccessToken
}

// tokenIsActiveForClient is tokenIsActive for an arbitrary client. Introspection authenticates the
// client, so a token has to be introspected by the client it was issued to.
func (ts *RoleAdministrationFlowTestSuite) tokenIsActiveForClient(
	token, clientID, clientSecret string) bool {
	ts.T().Helper()

	form := url.Values{}
	form.Set("token", token)

	req, err := http.NewRequest(http.MethodPost, testServerURL+"/oauth2/introspect",
		strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err, "Introspection request failed")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, resp.StatusCode, "Introspection response: %s", string(body))

	var result struct {
		Active bool `json:"active"`
	}
	ts.Require().NoError(json.Unmarshal(body, &result), "Introspection response: %s", string(body))
	return result.Active
}
