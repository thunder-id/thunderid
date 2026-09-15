// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// The shipped flows this suite exercises. Each is executed by handle so the tests do not depend on the
// seeded flow ids, which are a bootstrap detail.
const (
	roleDeletionFlowHandle           = "default-role-deletion-flow"
	rolePermissionRemovalFlowHandle  = "default-role-permission-removal-flow"
	groupDeletionFlowHandle          = "default-group-deletion-flow"
	groupMembershipRemovalFlowHandle = "default-group-membership-removal-flow"
	scopeDeletionFlowHandle          = "default-scope-deletion-flow"
)

// The input identifiers the executors read their targets from. Like the role assignment ones they
// deliberately avoid names the execution request already uses at its top level.
const (
	groupTargetInput          = "targetGroupId"
	memberTargetInput         = "targetMemberId"
	permissionsTargetInput    = "targetPermissions"
	resourceServerTargetInput = "targetResourceServerId"
	actionTargetInput         = "targetActionId"
)

const (
	authzFlowClientID     = "authz_admin_flow_client"
	authzFlowClientSecret = "authz_admin_flow_secret" // #nosec G101 -- test fixture, not a credential
	authzFlowResourceID   = "https://authzflow.dmv.test"
	// A second machine client stands in for a bystander principal. A criterion is keyed on the entity,
	// so proving a revocation did not reach someone else needs a genuinely different principal, not a
	// second token of the same one.
	authzBystanderClientID     = "authz_admin_flow_bystander"
	authzBystanderClientSecret = "authz_bystander_secret" // #nosec G101 -- test fixture, not a credential
	// A second resource server gives the same permission string a second audience. That pairing is what
	// the (entity, audience, scope) digest exists to keep apart.
	authzFlowOtherResourceID = "https://authzflow-other.dmv.test"
)

// Each test uses its own scope. The criterion is keyed on (entity, audience, scope) and every test here
// shares one machine client and one resource server, so a shared scope would let one test's revocation
// deny another test's token.
const (
	scopeRoleDeletion      = "role-deletion"
	scopePermissionKept    = "permission-kept"
	scopePermissionDropped = "permission-dropped"
	scopePermissionCleared = "permission-cleared"
	scopeGroupDeletion     = "group-deletion"
	scopeMemberRemoval     = "member-removal"
	scopeAncestorInherited = "ancestor-inherited"
	scopeRetired           = "retired"
	scopeRetiredNeighbor   = "retired-neighbor"
	// Scopes the bystander assertions hold. Each is granted by a role the flow under test does not
	// touch, so revoking the target scope must leave them alone.
	scopeUntouchedByRoleDeletion = "untouched-by-role-deletion"
	scopeUntouchedByPermEdit     = "untouched-by-perm-edit"
	scopeUntouchedByGroupDelete  = "untouched-by-group-delete"
	scopeSharedByMembers         = "shared-by-members"
	// scopeTwoAudiences is defined on both resource servers, so one principal can hold it twice under
	// different audiences.
	scopeTwoAudiences = "two-audiences"
	// The target scopes of the bystander tests. They cannot share the happy path's scopes: a criterion
	// is keyed on (entity, audience, scope) and both would name the same principal on the same server,
	// so whichever test ran first would deny the other's token.
	scopeDoomedRole       = "doomed-role"
	scopeDoomedPermission = "doomed-permission"
	scopeDoomedGroup      = "doomed-group"
)

// authzFlowResourceScopes are the scopes backed by a resource, which is the shape a role permission
// normally takes.
var authzFlowResourceScopes = []string{
	scopeRoleDeletion, scopePermissionKept, scopePermissionDropped, scopePermissionCleared,
	scopeGroupDeletion, scopeMemberRemoval, scopeAncestorInherited,
	scopeUntouchedByRoleDeletion, scopeUntouchedByPermEdit, scopeUntouchedByGroupDelete,
	scopeSharedByMembers, scopeTwoAudiences,
	scopeDoomedRole, scopeDoomedPermission, scopeDoomedGroup,
}

// authzFlowActionScopes are the scopes backed by an action on the resource server itself. The scope
// deletion flow retires an action, so its scopes have to be defined that way.
var authzFlowActionScopes = []string{scopeRetired, scopeRetiredNeighbor}

var authzAdminFlowTestOU = testutils.OrganizationUnit{
	Handle:      "authz_admin_flow_test_ou",
	Name:        "Test OU for Authorization Administration Flows",
	Description: "Organization unit created for authorization administration flow testing",
	Parent:      nil,
}

type AuthorizationAdministrationFlowTestSuite struct {
	suite.Suite
	client           *http.Client
	ouID             string
	flowIDs          map[string]string
	resourceServerID string
	resourceIDs      []string
	actionIDs        map[string]string
	appID            string
	createdRoleIDs   []string
	createdGroupIDs  []string
	// The bystander principal and the second audience, used only by the tests that assert a revocation
	// stayed within its criterion.
	bystanderAppID        string
	otherResourceServerID string
	otherResourceIDs      []string
}

func TestAuthorizationAdministrationFlowTestSuite(t *testing.T) {
	suite.Run(t, new(AuthorizationAdministrationFlowTestSuite))
}

func (ts *AuthorizationAdministrationFlowTestSuite) SetupSuite() {
	ts.client = testutils.GetRawHTTPClient()
	ts.flowIDs = map[string]string{}
	ts.actionIDs = map[string]string{}

	ouID, err := testutils.CreateOrganizationUnit(authzAdminFlowTestOU)
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

	for _, handle := range []string{
		roleDeletionFlowHandle, rolePermissionRemovalFlowHandle, groupDeletionFlowHandle,
		groupMembershipRemovalFlowHandle, scopeDeletionFlowHandle,
	} {
		flowID, flowErr := testutils.GetFlowIDByHandle(handle, administrationFlowType)
		ts.Require().NoError(flowErr, "Failed to resolve the shipped %s", handle)
		ts.Require().NotEmpty(flowID, "The shipped %s must be present", handle)
		ts.flowIDs[handle] = flowID
	}

	rsID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:        "Authz Flow DMV API",
		Description: "Resource server created for authorization administration flow testing",
		Identifier:  authzFlowResourceID,
		OUID:        ts.ouID,
	}, nil)
	ts.Require().NoError(err, "Failed to create test resource server")
	ts.resourceServerID = rsID

	for _, scope := range authzFlowResourceScopes {
		resourceID, resErr := testutils.CreateResource(ts.resourceServerID, scope, scope, "")
		ts.Require().NoError(resErr, "Failed to create test resource %s", scope)
		ts.resourceIDs = append(ts.resourceIDs, resourceID)
	}
	// An action defined on the resource server itself takes its handle as its permission, so these read
	// exactly like the resource-backed scopes above.
	for _, scope := range authzFlowActionScopes {
		actionID, actErr := testutils.CreateAction(ts.resourceServerID, testutils.Action{
			Name:   scope,
			Handle: scope,
		})
		ts.Require().NoError(actErr, "Failed to create test action %s", scope)
		ts.actionIDs[scope] = actionID
	}

	// A machine client is the cheapest principal that can hold a role, belong to a group, and obtain a
	// token carrying the resulting scopes, with no login flow in the way.
	appID, err := testutils.CreateApplication(testutils.Application{
		Name:        "Authz Flow Machine Client",
		Description: "Application created for authorization administration flow testing",
		OUID:        ts.ouID,
		InboundAuthConfig: []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                authzFlowClientID,
					"clientSecret":            authzFlowClientSecret,
					"grantTypes":              []string{"client_credentials"},
					"tokenEndpointAuthMethod": "client_secret_basic",
				},
			},
		},
	})
	ts.Require().NoError(err, "Failed to create test application")
	ts.appID = appID

	// A second audience for the same permission string.
	otherRS, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:        "Authz Flow Other API",
		Description: "Second resource server, so one scope name can exist under two audiences",
		Identifier:  authzFlowOtherResourceID,
		OUID:        ts.ouID,
	}, nil)
	ts.Require().NoError(err, "Failed to create the second test resource server")
	ts.otherResourceServerID = otherRS
	otherResourceID, err := testutils.CreateResource(otherRS, scopeTwoAudiences, scopeTwoAudiences, "")
	ts.Require().NoError(err, "Failed to create the second resource server's resource")
	ts.otherResourceIDs = append(ts.otherResourceIDs, otherResourceID)

	bystanderID, err := testutils.CreateApplication(testutils.Application{
		Name:        "Authz Flow Bystander Client",
		Description: "Second principal, used to prove a revocation did not reach anyone else",
		OUID:        ts.ouID,
		InboundAuthConfig: []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                authzBystanderClientID,
					"clientSecret":            authzBystanderClientSecret,
					"grantTypes":              []string{"client_credentials"},
					"tokenEndpointAuthMethod": "client_secret_basic",
				},
			},
		},
	})
	ts.Require().NoError(err, "Failed to create the bystander application")
	ts.bystanderAppID = bystanderID
}

func (ts *AuthorizationAdministrationFlowTestSuite) TearDownSuite() {
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
	if ts.bystanderAppID != "" {
		if err := testutils.DeleteApplication(ts.bystanderAppID); err != nil {
			ts.T().Logf("Failed to delete the bystander application during teardown: %v", err)
		}
	}
	for _, resourceID := range ts.otherResourceIDs {
		if err := testutils.DeleteResource(ts.otherResourceServerID, resourceID); err != nil {
			ts.T().Logf("Failed to delete second-server resource %s during teardown: %v", resourceID, err)
		}
	}
	if ts.otherResourceServerID != "" {
		if err := testutils.DeleteResourceServer(ts.otherResourceServerID); err != nil {
			ts.T().Logf("Failed to delete the second test resource server during teardown: %v", err)
		}
	}
	for _, actionID := range ts.actionIDs {
		if err := testutils.DeleteAction(ts.resourceServerID, actionID); err != nil {
			ts.T().Logf("Failed to delete test action %s during teardown: %v", actionID, err)
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

// createRole creates a role granting the given permissions to the given assignees and registers it for
// teardown.
func (ts *AuthorizationAdministrationFlowTestSuite) createRole(
	name string, permissions []string, assignments ...testutils.Assignment) string {
	ts.T().Helper()

	role := testutils.Role{
		Name:        name,
		Description: "Role created for authorization administration flow testing",
		OUID:        ts.ouID,
		Assignments: assignments,
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

// createGroup creates a group holding the given members and registers it for teardown.
func (ts *AuthorizationAdministrationFlowTestSuite) createGroup(
	name string, members ...testutils.Member) string {
	ts.T().Helper()

	groupID, err := testutils.CreateGroup(testutils.Group{
		Name:        name,
		Description: "Group created for authorization administration flow testing",
		OUID:        ts.ouID,
		Members:     members,
	})
	ts.Require().NoError(err, "Failed to create test group")
	ts.createdGroupIDs = append(ts.createdGroupIDs, groupID)
	return groupID
}

// tokenWithScope obtains a client credentials token bound to the test resource server, and returns it
// with the scopes actually granted.
func (ts *AuthorizationAdministrationFlowTestSuite) tokenWithScope(scope string) (string, string) {
	ts.T().Helper()

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("scope", scope)
	form.Set("resource", authzFlowResourceID)

	req, err := http.NewRequest(http.MethodPost, testServerURL+"/oauth2/token",
		strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(authzFlowClientID, authzFlowClientSecret)

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

// tokenFor obtains a client credentials token for an arbitrary client and audience, which the
// bystander assertions need: they turn on the token belonging to a different principal, or being bound
// to a different resource server, than the one the flow acted on.
func (ts *AuthorizationAdministrationFlowTestSuite) tokenFor(
	clientID, clientSecret, audience, scope string) (string, string) {
	ts.T().Helper()

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("scope", scope)
	form.Set("resource", audience)

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
	return body.AccessToken, body.Scope
}

// grantedTokenFor is grantedToken for an arbitrary client and audience.
func (ts *AuthorizationAdministrationFlowTestSuite) grantedTokenFor(
	clientID, clientSecret, audience, scope string) string {
	ts.T().Helper()

	token, granted := ts.tokenFor(clientID, clientSecret, audience, scope)
	ts.Require().NotEmpty(token, "The client must obtain a token for %s on %s", scope, audience)
	ts.Require().Contains(granted, scope, "The token must carry the scope under test")
	ts.Require().True(ts.tokenIsActiveFor(token, clientID, clientSecret),
		"The token must be accepted before the flow runs")
	return token
}

// tokenIsActiveFor is tokenIsActive for an arbitrary client. Introspection authenticates the client, so
// a bystander's token has to be introspected by the client that was issued it.
func (ts *AuthorizationAdministrationFlowTestSuite) tokenIsActiveFor(
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

// createRoleOn creates a role granting permissions on a named resource server.
func (ts *AuthorizationAdministrationFlowTestSuite) createRoleOn(
	name, resourceServerID string, permissions []string, assignments ...testutils.Assignment) string {
	ts.T().Helper()

	roleID, err := testutils.CreateRole(testutils.Role{
		Name:        name,
		Description: "Role created for authorization administration flow testing",
		OUID:        ts.ouID,
		Assignments: assignments,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: resourceServerID, Permissions: permissions},
		},
	})
	ts.Require().NoError(err, "Failed to create test role")
	ts.createdRoleIDs = append(ts.createdRoleIDs, roleID)
	return roleID
}

// grantedToken obtains a token and asserts it actually carries the scope, so a test that later asserts
// a revocation cannot pass against a token that never had the scope in the first place.
func (ts *AuthorizationAdministrationFlowTestSuite) grantedToken(scope string) string {
	ts.T().Helper()

	token, granted := ts.tokenWithScope(scope)
	ts.Require().NotEmpty(token, "The client must obtain a token")
	ts.Require().Contains(granted, scope, "The token must carry the scope under test")
	ts.Require().True(ts.tokenIsActive(token), "The token must be accepted before the flow runs")
	return token
}

// tokenIsActive reports what the Authorization Server says about the token now. Introspection is used
// rather than a Resource Server call because it reflects a revocation immediately, while the Resource
// Server cache is only eventually consistent.
func (ts *AuthorizationAdministrationFlowTestSuite) tokenIsActive(token string) bool {
	ts.T().Helper()

	form := url.Values{}
	form.Set("token", token)

	req, err := http.NewRequest(http.MethodPost, testServerURL+"/oauth2/introspect",
		strings.NewReader(form.Encode()))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(authzFlowClientID, authzFlowClientSecret)

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

// adminGet issues an authenticated read and returns the status and body, for the assertions that need
// to look at a resource the testutils helpers do not expose.
func (ts *AuthorizationAdministrationFlowTestSuite) adminGet(path string) (int, []byte) {
	ts.T().Helper()

	req, err := http.NewRequest(http.MethodGet, testServerURL+path, nil)
	ts.Require().NoError(err)

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err, "Administrative read failed")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	return resp.StatusCode, body
}

// rolePermissions returns the permissions the role grants on the test resource server.
func (ts *AuthorizationAdministrationFlowTestSuite) rolePermissions(roleID string) []string {
	ts.T().Helper()

	status, body := ts.adminGet("/roles/" + roleID)
	ts.Require().Equal(http.StatusOK, status, "Role read response: %s", string(body))

	var role struct {
		Permissions []struct {
			ResourceServerID string   `json:"resourceServerId"`
			Permissions      []string `json:"permissions"`
		} `json:"permissions"`
	}
	ts.Require().NoError(json.Unmarshal(body, &role), "Role read response: %s", string(body))
	for _, resource := range role.Permissions {
		if resource.ResourceServerID == ts.resourceServerID {
			return resource.Permissions
		}
	}
	return nil
}

func (ts *AuthorizationAdministrationFlowTestSuite) roleExists(roleID string) bool {
	ts.T().Helper()
	status, _ := ts.adminGet("/roles/" + roleID)
	return status == http.StatusOK
}

func (ts *AuthorizationAdministrationFlowTestSuite) groupExists(groupID string) bool {
	ts.T().Helper()
	status, _ := ts.adminGet("/groups/" + groupID)
	return status == http.StatusOK
}

func (ts *AuthorizationAdministrationFlowTestSuite) actionExists(actionID string) bool {
	ts.T().Helper()
	status, _ := ts.adminGet(
		fmt.Sprintf("/resource-servers/%s/actions/%s", ts.resourceServerID, actionID))
	return status == http.StatusOK
}

// execute runs one of the shipped flows as an administrator.
func (ts *AuthorizationAdministrationFlowTestSuite) execute(
	handle string, inputs map[string]string) (int, map[string]interface{}) {
	ts.T().Helper()
	return executeAdminFlowRequest(ts.T(), ts.client, ts.flowIDs[handle], inputs, true)
}

// requireCompleted asserts the flow ran to the end, reporting the refusal when it did not.
func (ts *AuthorizationAdministrationFlowTestSuite) requireCompleted(
	status int, step map[string]interface{}) {
	ts.T().Helper()
	ts.Require().Equal(http.StatusOK, status)
	ts.Require().Equal("COMPLETE", step["flowStatus"], "Flow response: %v", step)
}

// ---- default-role-deletion-flow ----

// Deleting a role takes its scopes from everyone holding it, so the token minted before the deletion
// must stop being accepted and the role must be gone.
func (ts *AuthorizationAdministrationFlowTestSuite) TestRoleDeletionFlow_RevokesTokenAndDeletesRole() {
	roleID := ts.createRole("Authz Flow Deleted Administrator", []string{scopeRoleDeletion},
		testutils.Assignment{ID: ts.appID, Type: "app"})
	token := ts.grantedToken(scopeRoleDeletion)

	status, step := ts.execute(roleDeletionFlowHandle, map[string]string{roleTargetInput: roleID})

	ts.requireCompleted(status, step)
	ts.False(ts.tokenIsActive(token), "The token carrying the deleted role's scope must be rejected")
	ts.False(ts.roleExists(roleID), "The role must be deleted")
}

// The validator's own code must survive out of the preparatory node, so a console can say which of the
// refusals happened rather than showing one generic message.
func (ts *AuthorizationAdministrationFlowTestSuite) TestRoleDeletionFlow_UnknownRoleCarriesValidatorCode() {
	status, step := ts.execute(roleDeletionFlowHandle, map[string]string{
		roleTargetInput: "01900000-0000-7000-8000-0000000000fe",
	})

	ts.Require().Equal(http.StatusOK, status)
	ts.NotEqual("COMPLETE", step["flowStatus"], "An unknown role must not complete")
	errObj, ok := step["error"].(map[string]interface{})
	ts.Require().True(ok, "The refusal must be reported in the error envelope")
	ts.Equal(errCodeRoleNotFound, errObj["code"],
		"The validator's own code must reach the caller, not the executor's generic one")
}

// ---- default-role-permission-removal-flow ----

// Editing a role to grant fewer scopes revokes every scope it granted, not only the dropped one. That
// is the over-revoke decision: the kept scope costs its holders one refresh, and the boundary cutoff
// lets the next token through.
func (ts *AuthorizationAdministrationFlowTestSuite) TestRolePermissionRemovalFlow_RevokesAndAppliesTheNewSet() {
	roleID := ts.createRole("Authz Flow Narrowed Administrator",
		[]string{scopePermissionKept, scopePermissionDropped},
		testutils.Assignment{ID: ts.appID, Type: "app"})
	token := ts.grantedToken(scopePermissionDropped)

	newSet, err := json.Marshal([]map[string]interface{}{
		{"resourceServerId": ts.resourceServerID, "permissions": []string{scopePermissionKept}},
	})
	ts.Require().NoError(err)

	status, step := ts.execute(rolePermissionRemovalFlowHandle, map[string]string{
		roleTargetInput:        roleID,
		permissionsTargetInput: string(newSet),
	})

	ts.requireCompleted(status, step)
	ts.False(ts.tokenIsActive(token), "The token carrying the dropped scope must be rejected")
	ts.Equal([]string{scopePermissionKept}, ts.rolePermissions(roleID),
		"The role must grant exactly the permissions the flow was given")
}

// An edit that leaves the role granting nothing is the largest permission removal there is, and must be
// applied rather than treated as an omission.
func (ts *AuthorizationAdministrationFlowTestSuite) TestRolePermissionRemovalFlow_EmptySetClearsTheRole() {
	roleID := ts.createRole("Authz Flow Cleared Administrator", []string{scopePermissionCleared},
		testutils.Assignment{ID: ts.appID, Type: "app"})
	token := ts.grantedToken(scopePermissionCleared)

	status, step := ts.execute(rolePermissionRemovalFlowHandle, map[string]string{
		roleTargetInput:        roleID,
		permissionsTargetInput: "[]",
	})

	ts.requireCompleted(status, step)
	ts.False(ts.tokenIsActive(token))
	ts.Empty(ts.rolePermissions(roleID), "The role must be left granting nothing")
}

// ---- default-group-deletion-flow ----

// A group holds no tokens; its members do. Deleting the group must reach the member that held its
// roles. This is the group-assignee shape the role assignment suite does not cover.
func (ts *AuthorizationAdministrationFlowTestSuite) TestGroupDeletionFlow_RevokesMemberTokenAndDeletesGroup() {
	groupID := ts.createGroup("Authz Flow Deleted Group",
		testutils.Member{Id: ts.appID, Type: "app"})
	ts.createRole("Authz Flow Group Deletion Administrator", []string{scopeGroupDeletion},
		testutils.Assignment{ID: groupID, Type: "group"})
	token := ts.grantedToken(scopeGroupDeletion)

	status, step := ts.execute(groupDeletionFlowHandle, map[string]string{groupTargetInput: groupID})

	ts.requireCompleted(status, step)
	ts.False(ts.tokenIsActive(token),
		"The member's token must be rejected: the group was the only path to the scope")
	ts.False(ts.groupExists(groupID), "The group must be deleted")
}

// ---- default-group-membership-removal-flow ----

// Removing one member costs that member the group's scopes, and the group itself must survive.
func (ts *AuthorizationAdministrationFlowTestSuite) TestGroupMembershipRemovalFlow_RevokesDepartingMemberToken() {
	groupID := ts.createGroup("Authz Flow Membership Group",
		testutils.Member{Id: ts.appID, Type: "app"})
	ts.createRole("Authz Flow Membership Administrator", []string{scopeMemberRemoval},
		testutils.Assignment{ID: groupID, Type: "group"})
	token := ts.grantedToken(scopeMemberRemoval)

	status, step := ts.execute(groupMembershipRemovalFlowHandle, map[string]string{
		groupTargetInput:  groupID,
		memberTargetInput: ts.appID,
	})

	ts.requireCompleted(status, step)
	ts.False(ts.tokenIsActive(token), "The departing member's token must be rejected")
	ts.True(ts.groupExists(groupID), "Removing a member must not delete the group")

	members, err := testutils.GetGroupMembers(groupID)
	ts.Require().NoError(err)
	ts.Empty(members, "The member must be gone from the group")
}

// Membership conveys a parent group's roles as well as the group's own. Resolving only the group's own
// roles would leave the departing member's inherited scopes live, which is the under-revocation this
// flow exists to prevent.
func (ts *AuthorizationAdministrationFlowTestSuite) TestGroupMembershipRemovalFlow_RevokesScopesInheritedFromAnAncestor() {
	childID := ts.createGroup("Authz Flow Child Group",
		testutils.Member{Id: ts.appID, Type: "app"})
	parentID := ts.createGroup("Authz Flow Parent Group",
		testutils.Member{Id: childID, Type: "group"})
	// The role hangs off the parent only, so the member holds the scope purely through the nesting.
	ts.createRole("Authz Flow Ancestor Administrator", []string{scopeAncestorInherited},
		testutils.Assignment{ID: parentID, Type: "group"})
	token := ts.grantedToken(scopeAncestorInherited)

	status, step := ts.execute(groupMembershipRemovalFlowHandle, map[string]string{
		groupTargetInput:  childID,
		memberTargetInput: ts.appID,
	})

	ts.requireCompleted(status, step)
	ts.False(ts.tokenIsActive(token),
		"A scope held through an ancestor group must be revoked when the path to it is cut")
}

// ---- default-scope-deletion-flow ----

// Retiring a scope denies it deployment-wide rather than per principal, because a scope that no longer
// exists should be held by nobody and there is no set of holders to enumerate.
func (ts *AuthorizationAdministrationFlowTestSuite) TestScopeDeletionFlow_RevokesTheScopeAndDeletesTheAction() {
	ts.createRole("Authz Flow Retiring Administrator", []string{scopeRetired, scopeRetiredNeighbor},
		testutils.Assignment{ID: ts.appID, Type: "app"})
	retired := ts.grantedToken(scopeRetired)
	neighbor := ts.grantedToken(scopeRetiredNeighbor)

	status, step := ts.execute(scopeDeletionFlowHandle, map[string]string{
		resourceServerTargetInput: ts.resourceServerID,
		actionTargetInput:         ts.actionIDs[scopeRetired],
	})

	ts.requireCompleted(status, step)
	ts.False(ts.tokenIsActive(retired), "A token carrying the retired scope must be rejected")
	ts.True(ts.tokenIsActive(neighbor),
		"The row is keyed on one scope, so a token carrying a different one must be untouched")
	ts.False(ts.actionExists(ts.actionIDs[scopeRetired]), "The action must be deleted")
	delete(ts.actionIDs, scopeRetired)
}

// Without a resource server the preparatory node cannot resolve the audience the scope belongs to, so
// the flow must not proceed to the deletion.
func (ts *AuthorizationAdministrationFlowTestSuite) TestScopeDeletionFlow_MissingTargetDoesNotComplete() {
	_, step := ts.execute(scopeDeletionFlowHandle, map[string]string{
		actionTargetInput: ts.actionIDs[scopeRetiredNeighbor],
	})

	ts.NotEqual("COMPLETE", step["flowStatus"], "A flow with no resource server must not complete")
	ts.True(ts.actionExists(ts.actionIDs[scopeRetiredNeighbor]), "The action must be untouched")
}

// The PermissionValidator node is the only thing standing between an anonymous caller and a privileged
// change, so it must refuse one on every flow this suite covers.
func (ts *AuthorizationAdministrationFlowTestSuite) TestFlows_RejectAnonymousCallers() {
	groupID := ts.createGroup("Authz Flow Protected Group")
	roleID := ts.createRole("Authz Flow Protected Administrator", nil)

	cases := map[string]map[string]string{
		roleDeletionFlowHandle:           {roleTargetInput: roleID},
		rolePermissionRemovalFlowHandle:  {roleTargetInput: roleID, permissionsTargetInput: "[]"},
		groupDeletionFlowHandle:          {groupTargetInput: groupID},
		groupMembershipRemovalFlowHandle: {groupTargetInput: groupID, memberTargetInput: ts.appID},
		scopeDeletionFlowHandle: {
			resourceServerTargetInput: ts.resourceServerID,
			actionTargetInput:         ts.actionIDs[scopeRetiredNeighbor],
		},
	}
	for handle, inputs := range cases {
		ts.Run(handle, func() {
			_, step := executeAdminFlowRequest(ts.T(), ts.client, ts.flowIDs[handle], inputs, false)
			ts.NotEqual("COMPLETE", step["flowStatus"],
				"An anonymous caller must not complete %s", handle)
		})
	}

	ts.True(ts.roleExists(roleID), "The role must be untouched")
	ts.True(ts.groupExists(groupID), "The group must be untouched")
	ts.True(ts.actionExists(ts.actionIDs[scopeRetiredNeighbor]), "The action must be untouched")
}

// ---- negative cases: a revocation must stay inside its criterion ----

// A criterion names one scope, so deleting a role must not touch the other scopes its assignees hold.
// This is the over-revocation the whole per-scope dimension exists to prevent: a coarser design would
// deny the principal outright and take everything with it.
func (ts *AuthorizationAdministrationFlowTestSuite) TestRoleDeletionFlow_LeavesAnotherRoleOfTheSamePrincipalAlone() {
	doomed := ts.createRole("Authz Flow Doomed Administrator", []string{scopeDoomedRole},
		testutils.Assignment{ID: ts.appID, Type: "app"})
	ts.createRole("Authz Flow Surviving Administrator", []string{scopeUntouchedByRoleDeletion},
		testutils.Assignment{ID: ts.appID, Type: "app"})

	target := ts.grantedToken(scopeDoomedRole)
	bystander := ts.grantedToken(scopeUntouchedByRoleDeletion)

	status, step := ts.execute(roleDeletionFlowHandle, map[string]string{roleTargetInput: doomed})

	ts.requireCompleted(status, step)
	ts.False(ts.tokenIsActive(target), "The deleted role's scope must be revoked")
	ts.True(ts.tokenIsActive(bystander),
		"A scope the same principal holds through another role must survive")
}

// The same permission string on two resource servers is two different scopes. Nothing else in the
// integration suite proves that end to end, and it is the case the digest is built for: a criterion
// keyed on the scope name alone would deny both audiences.
func (ts *AuthorizationAdministrationFlowTestSuite) TestRoleDeletionFlow_LeavesTheSameScopeOnAnotherAudienceAlone() {
	doomed := ts.createRoleOn("Authz Flow Two Audience Doomed", ts.resourceServerID,
		[]string{scopeTwoAudiences}, testutils.Assignment{ID: ts.appID, Type: "app"})
	ts.createRoleOn("Authz Flow Two Audience Surviving", ts.otherResourceServerID,
		[]string{scopeTwoAudiences}, testutils.Assignment{ID: ts.appID, Type: "app"})

	onFirst := ts.grantedToken(scopeTwoAudiences)
	onSecond := ts.grantedTokenFor(authzFlowClientID, authzFlowClientSecret,
		authzFlowOtherResourceID, scopeTwoAudiences)

	status, step := ts.execute(roleDeletionFlowHandle, map[string]string{roleTargetInput: doomed})

	ts.requireCompleted(status, step)
	ts.False(ts.tokenIsActive(onFirst), "The scope must be revoked on the audience the role granted it")
	ts.True(ts.tokenIsActive(onSecond),
		"The identically named scope on another resource server is a different scope and must survive")
}

// Editing a role revokes every scope that role granted, but nothing beyond it.
func (ts *AuthorizationAdministrationFlowTestSuite) TestRolePermissionRemovalFlow_LeavesScopesOfOtherRolesAlone() {
	edited := ts.createRole("Authz Flow Edited Administrator", []string{scopeDoomedPermission},
		testutils.Assignment{ID: ts.appID, Type: "app"})
	ts.createRole("Authz Flow Unedited Administrator", []string{scopeUntouchedByPermEdit},
		testutils.Assignment{ID: ts.appID, Type: "app"})

	target := ts.grantedToken(scopeDoomedPermission)
	bystander := ts.grantedToken(scopeUntouchedByPermEdit)

	status, step := ts.execute(rolePermissionRemovalFlowHandle, map[string]string{
		roleTargetInput:        edited,
		permissionsTargetInput: "[]",
	})

	ts.requireCompleted(status, step)
	ts.False(ts.tokenIsActive(target), "The dropped scope must be revoked")
	ts.True(ts.tokenIsActive(bystander), "A scope granted by an untouched role must survive")
}

// Deleting one group must not cut the principal's other memberships.
func (ts *AuthorizationAdministrationFlowTestSuite) TestGroupDeletionFlow_LeavesOtherGroupMembershipsAlone() {
	doomedGroup := ts.createGroup("Authz Flow Doomed Group",
		testutils.Member{Id: ts.appID, Type: "app"})
	survivingGroup := ts.createGroup("Authz Flow Surviving Group",
		testutils.Member{Id: ts.appID, Type: "app"})
	ts.createRole("Authz Flow Doomed Group Administrator", []string{scopeDoomedGroup},
		testutils.Assignment{ID: doomedGroup, Type: "group"})
	ts.createRole("Authz Flow Surviving Group Administrator", []string{scopeUntouchedByGroupDelete},
		testutils.Assignment{ID: survivingGroup, Type: "group"})

	target := ts.grantedToken(scopeDoomedGroup)
	bystander := ts.grantedToken(scopeUntouchedByGroupDelete)

	status, step := ts.execute(groupDeletionFlowHandle, map[string]string{groupTargetInput: doomedGroup})

	ts.requireCompleted(status, step)
	ts.False(ts.tokenIsActive(target), "The deleted group's scope must be revoked")
	ts.True(ts.tokenIsActive(bystander),
		"A scope held through a group that still exists must survive")
	ts.True(ts.groupExists(survivingGroup), "The other group must be untouched")
}

// The blast radius of a member removal is decided by a live expansion the provider computes, which
// makes this the case most exposed to silent over-revocation: expanding the group instead of the one
// departing member would revoke for everyone who is still in it.
func (ts *AuthorizationAdministrationFlowTestSuite) TestGroupMembershipRemovalFlow_LeavesRemainingMembersAlone() {
	group := ts.createGroup("Authz Flow Shared Group",
		testutils.Member{Id: ts.appID, Type: "app"},
		testutils.Member{Id: ts.bystanderAppID, Type: "app"})
	ts.createRole("Authz Flow Shared Group Administrator", []string{scopeSharedByMembers},
		testutils.Assignment{ID: group, Type: "group"})

	departing := ts.grantedToken(scopeSharedByMembers)
	remaining := ts.grantedTokenFor(authzBystanderClientID, authzBystanderClientSecret,
		authzFlowResourceID, scopeSharedByMembers)

	status, step := ts.execute(groupMembershipRemovalFlowHandle, map[string]string{
		groupTargetInput:  group,
		memberTargetInput: ts.appID,
	})

	ts.requireCompleted(status, step)
	ts.False(ts.tokenIsActive(departing), "The departing member's token must be revoked")
	ts.True(ts.tokenIsActiveFor(remaining, authzBystanderClientID, authzBystanderClientSecret),
		"A member who is still in the group must keep the scope the group conveys")

	members, err := testutils.GetGroupMembers(group)
	ts.Require().NoError(err)
	ts.Len(members, 1, "Only the departing member may be removed")
}
