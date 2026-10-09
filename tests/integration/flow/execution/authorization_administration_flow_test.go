// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	roleDeletionFlowHandle           = "default-role-deletion-flow"
	rolePermissionRemovalFlowHandle  = "default-role-permission-removal-flow"
	groupDeletionFlowHandle          = "default-group-deletion-flow"
	groupMembershipRemovalFlowHandle = "default-group-membership-removal-flow"
	actionDeletionFlowHandle         = "default-action-deletion-flow"
)

const (
	groupTargetInput          = "targetGroupId"
	memberTargetInput         = "targetMemberId"
	permissionsTargetInput    = "targetPermissions"
	resourceServerTargetInput = "targetResourceServerId"
	resourceTargetInput       = "targetResourceId"
	actionTargetInput         = "targetActionId"
)

const (
	authzFlowClientID          = "authz_admin_flow_client"
	authzFlowClientSecret      = "authz_admin_flow_secret" // #nosec G101 -- test fixture, not a credential
	authzFlowResourceID        = "https://authzflow.dmv.test"
	authzBystanderClientID     = "authz_admin_flow_bystander"
	authzBystanderClientSecret = "authz_bystander_secret" // #nosec G101 -- test fixture, not a credential
	authzFlowOtherResourceID   = "https://authzflow-other.dmv.test"
)

const (
	scopeRoleDeletion            = "role-deletion"
	scopePermissionKept          = "permission-kept"
	scopePermissionDropped       = "permission-dropped"
	scopePermissionCleared       = "permission-cleared"
	scopeGroupDeletion           = "group-deletion"
	scopeMemberRemoval           = "member-removal"
	scopeAncestorInherited       = "ancestor-inherited"
	scopeRetired                 = "retired"
	scopeRetiredNeighbor         = "retired-neighbor"
	scopeUntouchedByRoleDeletion = "untouched-by-role-deletion"
	scopeUntouchedByPermEdit     = "untouched-by-perm-edit"
	scopeUntouchedByGroupDelete  = "untouched-by-group-delete"
	scopeSharedByMembers         = "shared-by-members"
	scopeTwoAudiences            = "two-audiences"
	scopeDoomedRole              = "doomed-role"
	scopeDoomedPermission        = "doomed-permission"
	scopeDoomedGroup             = "doomed-group"
	scopeRefusedPermEdit         = "refused-permission-edit"
	scopeRefusedMemberRemoval    = "refused-member-removal"
	scopeHeldThroughNesting      = "held-through-nesting"
	scopeHeldTwice               = "held-twice"
	scopeNestedGroupDeletion     = "nested-group-deletion"
	scopeNestedMemberRemoval     = "nested-member-removal"
	scopeReachedSeveralWays      = "reached-several-ways"
)

const (
	authzFlowNestedResource     = "licensing"
	authzFlowNestedRetireAction = "retire"
	authzFlowNestedKeepAction   = "keep"
)

const (
	errCodeInvalidRolePermissions   = "ROL-1012"
	errCodeImmutableRole            = "ROL-1013"
	errCodeGroupNestingTooDeep      = "ROL-1021"
	errCodeGroupNotFound            = "GRP-1003"
	errCodeInvalidGroupMember       = "GRP-1007"
	errCodeImmutableGroup           = "GRP-1015"
	errCodeResourceServerNotFound   = "RES-1003"
	errCodeResourceNotFound         = "RES-1008"
	errCodeActionNotFound           = "RES-1009"
	errCodeImmutableAction          = "RES-1020"
	errCodeMalformedRolePermissions = "FET-1105"
)

// File-backed fixtures under resources/declarative_resources; the API treats them as read-only.
const (
	declarativeRoleID           = "decl-role-1"
	declarativeGroupID          = "decl-group-1"
	declarativeNestedGroupID    = "decl-group-2"
	declarativeResourceServerID = "decl-rs-1"
)

const unknownID = "01900000-0000-7000-8000-0000000000fd"

// Mirrors the validator's nested-group descent limit.
const groupDescentLimit = 32

var authzFlowResourceScopes = []string{
	scopeRoleDeletion, scopePermissionKept, scopePermissionDropped, scopePermissionCleared,
	scopeGroupDeletion, scopeMemberRemoval, scopeAncestorInherited,
	scopeUntouchedByRoleDeletion, scopeUntouchedByPermEdit, scopeUntouchedByGroupDelete,
	scopeSharedByMembers, scopeTwoAudiences,
	scopeDoomedRole, scopeDoomedPermission, scopeDoomedGroup,
	scopeRefusedPermEdit, scopeRefusedMemberRemoval,
	scopeHeldThroughNesting, scopeHeldTwice, scopeNestedGroupDeletion, scopeNestedMemberRemoval,
	scopeReachedSeveralWays,
}

var authzFlowActionScopes = []string{scopeRetired, scopeRetiredNeighbor}

var authzAdminFlowTestOU = testutils.OrganizationUnit{
	Handle:      "authz_admin_flow_test_ou",
	Name:        "Test OU for Authorization Administration Flows",
	Description: "Organization unit created for authorization administration flow testing",
	Parent:      nil,
}

type AuthorizationAdministrationFlowTestSuite struct {
	suite.Suite
	ouID                  string
	flowIDs               map[string]string
	resourceServerID      string
	resourceIDs           []string
	actionIDs             map[string]string
	appID                 string
	createdRoleIDs        []string
	createdGroupIDs       []string
	bystanderAppID        string
	otherResourceServerID string
	otherResourceIDs      []string
	nestedResourceID      string
	nestedActionIDs       map[string]string
	nestedPermissions     map[string]string
}

func TestAuthorizationAdministrationFlowTestSuite(t *testing.T) {
	suite.Run(t, new(AuthorizationAdministrationFlowTestSuite))
}

func (ts *AuthorizationAdministrationFlowTestSuite) SetupSuite() {
	ts.flowIDs = map[string]string{}
	ts.actionIDs = map[string]string{}

	ouID, err := testutils.CreateOrganizationUnit(authzAdminFlowTestOU)
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

	for _, handle := range []string{
		roleDeletionFlowHandle, rolePermissionRemovalFlowHandle, groupDeletionFlowHandle,
		groupMembershipRemovalFlowHandle, actionDeletionFlowHandle, roleAssignmentRemovalFlowHandle,
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
	// A resource server's own action takes its handle as its permission.
	for _, scope := range authzFlowActionScopes {
		actionID, actErr := testutils.CreateAction(ts.resourceServerID, testutils.Action{
			Name:   scope,
			Handle: scope,
		})
		ts.Require().NoError(actErr, "Failed to create test action %s", scope)
		ts.actionIDs[scope] = actionID
	}

	// A resource action's permission is derived by the server, so read it back rather than assume it.
	ts.nestedActionIDs = map[string]string{}
	ts.nestedPermissions = map[string]string{}
	nestedResourceID, err := testutils.CreateResource(ts.resourceServerID, authzFlowNestedResource,
		authzFlowNestedResource, "")
	ts.Require().NoError(err, "Failed to create the resource carrying its own actions")
	ts.nestedResourceID = nestedResourceID
	ts.resourceIDs = append(ts.resourceIDs, nestedResourceID)
	for _, handle := range []string{authzFlowNestedRetireAction, authzFlowNestedKeepAction} {
		actionID, actErr := testutils.CreateResourceAction(ts.resourceServerID, nestedResourceID,
			testutils.Action{Name: handle, Handle: handle})
		ts.Require().NoError(actErr, "Failed to create the resource action %s", handle)
		ts.nestedActionIDs[handle] = actionID
		ts.nestedPermissions[handle] = ts.resourceActionPermission(actionID)
	}

	// A machine client can hold roles and get scoped tokens without a login flow.
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
	// A resource with actions cannot be deleted; an action already retired by a test is simply gone.
	for handle, actionID := range ts.nestedActionIDs {
		if err := testutils.DeleteResourceAction(ts.resourceServerID, ts.nestedResourceID, actionID); err != nil {
			ts.T().Logf("Failed to delete resource action %s during teardown: %v", handle, err)
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

func (ts *AuthorizationAdministrationFlowTestSuite) grantedTokenFor(
	clientID, clientSecret, audience, scope string) string {
	ts.T().Helper()

	result, err := testutils.RequestClientCredentialsToken(clientID, clientSecret, scope, audience)
	ts.Require().NoError(err, "Token request failed")
	ts.Require().Equal(http.StatusOK, result.StatusCode, "Token response: %s", string(result.Body))
	ts.Require().NotEmpty(result.Token.AccessToken, "The client must obtain a token for %s on %s", scope, audience)
	ts.Require().Contains(result.Token.Scope, scope, "The token must carry the scope under test")
	ts.Require().True(ts.tokenIsActiveFor(result.Token.AccessToken, clientID, clientSecret),
		"The token must be accepted before the flow runs")
	return result.Token.AccessToken
}

func (ts *AuthorizationAdministrationFlowTestSuite) tokenIsActiveFor(
	token, clientID, clientSecret string) bool {
	ts.T().Helper()

	active, err := testutils.IntrospectToken(token, clientID, clientSecret)
	ts.Require().NoError(err, "Introspection request failed")
	return active
}

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

func (ts *AuthorizationAdministrationFlowTestSuite) grantedToken(scope string) string {
	ts.T().Helper()
	return ts.grantedTokenFor(authzFlowClientID, authzFlowClientSecret, authzFlowResourceID, scope)
}

func (ts *AuthorizationAdministrationFlowTestSuite) tokenIsActive(token string) bool {
	ts.T().Helper()
	return ts.tokenIsActiveFor(token, authzFlowClientID, authzFlowClientSecret)
}

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

func (ts *AuthorizationAdministrationFlowTestSuite) resourceActionPath(actionID string) string {
	return fmt.Sprintf("/resource-servers/%s/resources/%s/actions/%s",
		ts.resourceServerID, ts.nestedResourceID, actionID)
}

func (ts *AuthorizationAdministrationFlowTestSuite) resourceActionExists(actionID string) bool {
	ts.T().Helper()
	status, _ := ts.adminGet(ts.resourceActionPath(actionID))
	return status == http.StatusOK
}

func (ts *AuthorizationAdministrationFlowTestSuite) resourceActionPermission(actionID string) string {
	ts.T().Helper()

	status, body := ts.adminGet(ts.resourceActionPath(actionID))
	ts.Require().Equal(http.StatusOK, status, "Resource action read response: %s", string(body))
	var action testutils.Action
	ts.Require().NoError(json.Unmarshal(body, &action), "Resource action read response: %s", string(body))
	ts.Require().NotEmpty(action.Permission, "The server must derive a permission for the resource action")
	return action.Permission
}

func (ts *AuthorizationAdministrationFlowTestSuite) groupMemberIDs(groupID string) []string {
	ts.T().Helper()

	members, err := testutils.GetGroupMembers(groupID)
	ts.Require().NoError(err, "Failed to read the members of group %s", groupID)
	ids := make([]string, 0, len(members))
	for _, member := range members {
		ids = append(ids, member.ID)
	}
	return ids
}

func (ts *AuthorizationAdministrationFlowTestSuite) declarativeRolePermissions() map[string][]string {
	ts.T().Helper()

	status, body := ts.adminGet("/roles/" + declarativeRoleID)
	ts.Require().Equal(http.StatusOK, status, "Declarative role read response: %s", string(body))
	var role struct {
		Permissions []testutils.ResourcePermissions `json:"permissions"`
	}
	ts.Require().NoError(json.Unmarshal(body, &role), "Declarative role read response: %s", string(body))
	byServer := map[string][]string{}
	for _, resource := range role.Permissions {
		byServer[resource.ResourceServerID] = resource.Permissions
	}
	return byServer
}

func (ts *AuthorizationAdministrationFlowTestSuite) requireRefused(step *common.FlowStep, code, why string) {
	ts.T().Helper()
	ts.NotEqual("COMPLETE", step.FlowStatus, "A refused change must not complete: %s", why)
	ts.Require().NotNil(step.Error, "The refusal must be reported in the error envelope: %s", why)
	ts.Equal(code, step.Error.Code, "The validator's own refusal code must reach the caller: %s", why)
}

func (ts *AuthorizationAdministrationFlowTestSuite) execute(
	handle string, inputs map[string]string) *common.FlowStep {
	ts.T().Helper()

	result, err := common.ExecuteAdministrationFlow(ts.flowIDs[handle], inputs, true)
	ts.Require().NoError(err, "Failed to execute %s", handle)
	ts.Require().Equal(http.StatusOK, result.StatusCode, "Flow error response: %+v", result.Error)
	return result.Step
}

func (ts *AuthorizationAdministrationFlowTestSuite) requireCompleted(step *common.FlowStep) {
	ts.T().Helper()
	ts.Require().Equal("COMPLETE", step.FlowStatus, "Flow response: %+v", step)
}

func (ts *AuthorizationAdministrationFlowTestSuite) TestRoleDeletionFlow_RevokesTokenAndDeletesRole() {
	roleID := ts.createRole("Authz Flow Deleted Administrator", []string{scopeRoleDeletion},
		testutils.Assignment{ID: ts.appID, Type: "app"})
	token := ts.grantedToken(scopeRoleDeletion)

	step := ts.execute(roleDeletionFlowHandle, map[string]string{roleTargetInput: roleID})

	ts.requireCompleted(step)
	ts.False(ts.tokenIsActive(token), "The token carrying the deleted role's scope must be rejected")
	ts.False(ts.roleExists(roleID), "The role must be deleted")
}

func (ts *AuthorizationAdministrationFlowTestSuite) TestRoleDeletionFlow_UnknownRoleCarriesValidatorCode() {
	step := ts.execute(roleDeletionFlowHandle, map[string]string{
		roleTargetInput: "01900000-0000-7000-8000-0000000000fe",
	})

	ts.NotEqual("COMPLETE", step.FlowStatus, "An unknown role must not complete")
	ts.Require().NotNil(step.Error, "The refusal must be reported in the error envelope")
	ts.Equal(errCodeRoleNotFound, step.Error.Code,
		"The validator's own code must reach the caller, not the executor's generic one")
}

// Revokes every scope the role granted, not only the dropped one.
func (ts *AuthorizationAdministrationFlowTestSuite) TestRolePermissionRemovalFlow_RevokesAndAppliesTheNewSet() {
	roleID := ts.createRole("Authz Flow Narrowed Administrator",
		[]string{scopePermissionKept, scopePermissionDropped},
		testutils.Assignment{ID: ts.appID, Type: "app"})
	token := ts.grantedToken(scopePermissionDropped)

	newSet, err := json.Marshal([]map[string]interface{}{
		{"resourceServerId": ts.resourceServerID, "permissions": []string{scopePermissionKept}},
	})
	ts.Require().NoError(err)

	step := ts.execute(rolePermissionRemovalFlowHandle, map[string]string{
		roleTargetInput:        roleID,
		permissionsTargetInput: string(newSet),
	})

	ts.requireCompleted(step)
	ts.False(ts.tokenIsActive(token), "The token carrying the dropped scope must be rejected")
	ts.Equal([]string{scopePermissionKept}, ts.rolePermissions(roleID),
		"The role must grant exactly the permissions the flow was given")
}

func (ts *AuthorizationAdministrationFlowTestSuite) TestRolePermissionRemovalFlow_EmptySetClearsTheRole() {
	roleID := ts.createRole("Authz Flow Cleared Administrator", []string{scopePermissionCleared},
		testutils.Assignment{ID: ts.appID, Type: "app"})
	token := ts.grantedToken(scopePermissionCleared)

	step := ts.execute(rolePermissionRemovalFlowHandle, map[string]string{
		roleTargetInput:        roleID,
		permissionsTargetInput: "[]",
	})

	ts.requireCompleted(step)
	ts.False(ts.tokenIsActive(token))
	ts.Empty(ts.rolePermissions(roleID), "The role must be left granting nothing")
}

func (ts *AuthorizationAdministrationFlowTestSuite) TestGroupDeletionFlow_RevokesMemberTokenAndDeletesGroup() {
	groupID := ts.createGroup("Authz Flow Deleted Group",
		testutils.Member{Id: ts.appID, Type: "app"})
	ts.createRole("Authz Flow Group Deletion Administrator", []string{scopeGroupDeletion},
		testutils.Assignment{ID: groupID, Type: "group"})
	token := ts.grantedToken(scopeGroupDeletion)

	step := ts.execute(groupDeletionFlowHandle, map[string]string{groupTargetInput: groupID})

	ts.requireCompleted(step)
	ts.False(ts.tokenIsActive(token),
		"The member's token must be rejected: the group was the only path to the scope")
	ts.False(ts.groupExists(groupID), "The group must be deleted")
}

func (ts *AuthorizationAdministrationFlowTestSuite) TestGroupMembershipRemovalFlow_RevokesDepartingMemberToken() {
	groupID := ts.createGroup("Authz Flow Membership Group",
		testutils.Member{Id: ts.appID, Type: "app"})
	ts.createRole("Authz Flow Membership Administrator", []string{scopeMemberRemoval},
		testutils.Assignment{ID: groupID, Type: "group"})
	token := ts.grantedToken(scopeMemberRemoval)

	step := ts.execute(groupMembershipRemovalFlowHandle, map[string]string{
		groupTargetInput:  groupID,
		memberTargetInput: ts.appID,
	})

	ts.requireCompleted(step)
	ts.False(ts.tokenIsActive(token), "The departing member's token must be rejected")
	ts.True(ts.groupExists(groupID), "Removing a member must not delete the group")

	members, err := testutils.GetGroupMembers(groupID)
	ts.Require().NoError(err)
	ts.Empty(members, "The member must be gone from the group")
}

func (ts *AuthorizationAdministrationFlowTestSuite) TestGroupMembershipRemovalFlow_RevokesScopesInheritedFromAnAncestor() {
	childID := ts.createGroup("Authz Flow Child Group",
		testutils.Member{Id: ts.appID, Type: "app"})
	parentID := ts.createGroup("Authz Flow Parent Group",
		testutils.Member{Id: childID, Type: "group"})
	// The role hangs off the parent only, so the member holds the scope purely through the nesting.
	ts.createRole("Authz Flow Ancestor Administrator", []string{scopeAncestorInherited},
		testutils.Assignment{ID: parentID, Type: "group"})
	token := ts.grantedToken(scopeAncestorInherited)

	step := ts.execute(groupMembershipRemovalFlowHandle, map[string]string{
		groupTargetInput:  childID,
		memberTargetInput: ts.appID,
	})

	ts.requireCompleted(step)
	ts.False(ts.tokenIsActive(token),
		"A scope held through an ancestor group must be revoked when the path to it is cut")
}

// Retiring a scope denies it deployment-wide rather than per principal.
func (ts *AuthorizationAdministrationFlowTestSuite) TestActionDeletionFlow_RevokesTheScopeAndDeletesTheAction() {
	ts.createRole("Authz Flow Retiring Administrator", []string{scopeRetired, scopeRetiredNeighbor},
		testutils.Assignment{ID: ts.appID, Type: "app"})
	retired := ts.grantedToken(scopeRetired)
	neighbor := ts.grantedToken(scopeRetiredNeighbor)

	step := ts.execute(actionDeletionFlowHandle, map[string]string{
		resourceServerTargetInput: ts.resourceServerID,
		actionTargetInput:         ts.actionIDs[scopeRetired],
	})

	ts.requireCompleted(step)
	ts.False(ts.tokenIsActive(retired), "A token carrying the retired scope must be rejected")
	ts.True(ts.tokenIsActive(neighbor),
		"The row is keyed on one scope, so a token carrying a different one must be untouched")
	ts.False(ts.actionExists(ts.actionIDs[scopeRetired]), "The action must be deleted")
	delete(ts.actionIDs, scopeRetired)
}

func (ts *AuthorizationAdministrationFlowTestSuite) TestActionDeletionFlow_MissingTargetDoesNotComplete() {
	step := ts.execute(actionDeletionFlowHandle, map[string]string{
		actionTargetInput: ts.actionIDs[scopeRetiredNeighbor],
	})

	ts.NotEqual("COMPLETE", step.FlowStatus, "A flow with no resource server must not complete")
	ts.True(ts.actionExists(ts.actionIDs[scopeRetiredNeighbor]), "The action must be untouched")
}

// Refused at the flow execution boundary, before the PermissionValidator node runs.
func (ts *AuthorizationAdministrationFlowTestSuite) TestFlows_RejectAnonymousCallers() {
	// A real membership, so the only possible refusal reason is the missing authentication.
	groupID := ts.createGroup("Authz Flow Protected Group",
		testutils.Member{Id: ts.appID, Type: "app"})
	roleID := ts.createRole("Authz Flow Protected Administrator", nil)

	cases := map[string]map[string]string{
		roleDeletionFlowHandle:           {roleTargetInput: roleID},
		rolePermissionRemovalFlowHandle:  {roleTargetInput: roleID, permissionsTargetInput: "[]"},
		groupDeletionFlowHandle:          {groupTargetInput: groupID},
		groupMembershipRemovalFlowHandle: {groupTargetInput: groupID, memberTargetInput: ts.appID},
		actionDeletionFlowHandle: {
			resourceServerTargetInput: ts.resourceServerID,
			actionTargetInput:         ts.actionIDs[scopeRetiredNeighbor],
		},
	}
	for handle, inputs := range cases {
		ts.Run(handle, func() {
			result, err := common.ExecuteAdministrationFlow(ts.flowIDs[handle], inputs, false)
			ts.Require().NoError(err, "Failed to execute %s anonymously", handle)
			ts.Equal(http.StatusUnauthorized, result.StatusCode,
				"An anonymous caller of %s must be refused as unauthenticated", handle)
			ts.Nil(result.Step, "An anonymous caller of %s must not be given a flow step", handle)
			ts.Require().NotNil(result.Error, "The refusal of %s must be reported as an API error", handle)
			ts.Equal(errCodeAdministrationAuthenticationRequired, result.Error.Code,
				"The refusal of %s must be the missing-authentication one", handle)
		})
	}

	ts.True(ts.roleExists(roleID), "The role must be untouched")
	ts.True(ts.groupExists(groupID), "The group must be untouched")
	members, err := testutils.GetGroupMembers(groupID)
	ts.Require().NoError(err, "Failed to read the protected group's members")
	ts.Require().Len(members, 1, "An anonymous caller must not remove the member")
	ts.Equal(ts.appID, members[0].ID, "The member the anonymous caller named must remain in the group")
	ts.True(ts.actionExists(ts.actionIDs[scopeRetiredNeighbor]), "The action must be untouched")
}

func (ts *AuthorizationAdministrationFlowTestSuite) TestRoleDeletionFlow_LeavesAnotherRoleOfTheSamePrincipalAlone() {
	doomed := ts.createRole("Authz Flow Doomed Administrator", []string{scopeDoomedRole},
		testutils.Assignment{ID: ts.appID, Type: "app"})
	ts.createRole("Authz Flow Surviving Administrator", []string{scopeUntouchedByRoleDeletion},
		testutils.Assignment{ID: ts.appID, Type: "app"})

	target := ts.grantedToken(scopeDoomedRole)
	bystander := ts.grantedToken(scopeUntouchedByRoleDeletion)

	step := ts.execute(roleDeletionFlowHandle, map[string]string{roleTargetInput: doomed})

	ts.requireCompleted(step)
	ts.False(ts.tokenIsActive(target), "The deleted role's scope must be revoked")
	ts.True(ts.tokenIsActive(bystander),
		"A scope the same principal holds through another role must survive")
}

func (ts *AuthorizationAdministrationFlowTestSuite) TestRoleDeletionFlow_LeavesTheSameScopeOnAnotherAudienceAlone() {
	doomed := ts.createRoleOn("Authz Flow Two Audience Doomed", ts.resourceServerID,
		[]string{scopeTwoAudiences}, testutils.Assignment{ID: ts.appID, Type: "app"})
	ts.createRoleOn("Authz Flow Two Audience Surviving", ts.otherResourceServerID,
		[]string{scopeTwoAudiences}, testutils.Assignment{ID: ts.appID, Type: "app"})

	onFirst := ts.grantedToken(scopeTwoAudiences)
	onSecond := ts.grantedTokenFor(authzFlowClientID, authzFlowClientSecret,
		authzFlowOtherResourceID, scopeTwoAudiences)

	step := ts.execute(roleDeletionFlowHandle, map[string]string{roleTargetInput: doomed})

	ts.requireCompleted(step)
	ts.False(ts.tokenIsActive(onFirst), "The scope must be revoked on the audience the role granted it")
	ts.True(ts.tokenIsActive(onSecond),
		"The identically named scope on another resource server is a different scope and must survive")
}

func (ts *AuthorizationAdministrationFlowTestSuite) TestRolePermissionRemovalFlow_LeavesScopesOfOtherRolesAlone() {
	edited := ts.createRole("Authz Flow Edited Administrator", []string{scopeDoomedPermission},
		testutils.Assignment{ID: ts.appID, Type: "app"})
	ts.createRole("Authz Flow Unedited Administrator", []string{scopeUntouchedByPermEdit},
		testutils.Assignment{ID: ts.appID, Type: "app"})

	target := ts.grantedToken(scopeDoomedPermission)
	bystander := ts.grantedToken(scopeUntouchedByPermEdit)

	step := ts.execute(rolePermissionRemovalFlowHandle, map[string]string{
		roleTargetInput:        edited,
		permissionsTargetInput: "[]",
	})

	ts.requireCompleted(step)
	ts.False(ts.tokenIsActive(target), "The dropped scope must be revoked")
	ts.True(ts.tokenIsActive(bystander), "A scope granted by an untouched role must survive")
}

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

	step := ts.execute(groupDeletionFlowHandle, map[string]string{groupTargetInput: doomedGroup})

	ts.requireCompleted(step)
	ts.False(ts.tokenIsActive(target), "The deleted group's scope must be revoked")
	ts.True(ts.tokenIsActive(bystander),
		"A scope held through a group that still exists must survive")
	ts.True(ts.groupExists(survivingGroup), "The other group must be untouched")
}

func (ts *AuthorizationAdministrationFlowTestSuite) TestGroupMembershipRemovalFlow_LeavesRemainingMembersAlone() {
	group := ts.createGroup("Authz Flow Shared Group",
		testutils.Member{Id: ts.appID, Type: "app"},
		testutils.Member{Id: ts.bystanderAppID, Type: "app"})
	ts.createRole("Authz Flow Shared Group Administrator", []string{scopeSharedByMembers},
		testutils.Assignment{ID: group, Type: "group"})

	departing := ts.grantedToken(scopeSharedByMembers)
	remaining := ts.grantedTokenFor(authzBystanderClientID, authzBystanderClientSecret,
		authzFlowResourceID, scopeSharedByMembers)

	step := ts.execute(groupMembershipRemovalFlowHandle, map[string]string{
		groupTargetInput:  group,
		memberTargetInput: ts.appID,
	})

	ts.requireCompleted(step)
	ts.False(ts.tokenIsActive(departing), "The departing member's token must be revoked")
	ts.True(ts.tokenIsActiveFor(remaining, authzBystanderClientID, authzBystanderClientSecret),
		"A member who is still in the group must keep the scope the group conveys")

	members, err := testutils.GetGroupMembers(group)
	ts.Require().NoError(err)
	ts.Len(members, 1, "Only the departing member may be removed")
}

func (ts *AuthorizationAdministrationFlowTestSuite) TestRolePermissionRemovalFlow_RefusedEditsChangeNothing() {
	roleID := ts.createRole("Authz Flow Refused Edit Administrator", []string{scopeRefusedPermEdit},
		testutils.Assignment{ID: ts.appID, Type: "app"})
	token := ts.grantedToken(scopeRefusedPermEdit)

	encode := func(resourceServerID string, permissions ...string) string {
		encoded, err := json.Marshal([]map[string]interface{}{
			{"resourceServerId": resourceServerID, "permissions": permissions},
		})
		ts.Require().NoError(err)
		return string(encoded)
	}
	cases := []struct {
		name        string
		permissions string
		code        string
	}{
		{"permission the resource server does not define",
			encode(ts.resourceServerID, scopeRefusedPermEdit, "no-such-permission"), errCodeInvalidRolePermissions},
		{"resource server that does not exist", encode(unknownID, scopeRefusedPermEdit),
			errCodeInvalidRolePermissions},
		{"permission set naming no resource server", encode("", scopeRefusedPermEdit),
			errCodeInvalidRolePermissions},
		{"permission set that is not JSON", "not-a-permission-set", errCodeMalformedRolePermissions},
	}
	for _, tc := range cases {
		ts.Run(tc.name, func() {
			step := ts.execute(rolePermissionRemovalFlowHandle, map[string]string{
				roleTargetInput:        roleID,
				permissionsTargetInput: tc.permissions,
			})
			ts.requireRefused(step, tc.code, tc.name)
		})
	}

	ts.True(ts.tokenIsActive(token), "A refused edit must not revoke the scope the role still grants")
	ts.Equal([]string{scopeRefusedPermEdit}, ts.rolePermissions(roleID),
		"A refused edit must leave the role granting what it granted")
}

func (ts *AuthorizationAdministrationFlowTestSuite) TestRolePermissionRemovalFlow_UnknownRoleIsRefused() {
	step := ts.execute(rolePermissionRemovalFlowHandle, map[string]string{
		roleTargetInput:        unknownID,
		permissionsTargetInput: "[]",
	})

	ts.requireRefused(step, errCodeRoleNotFound, "the role does not exist")
}

func (ts *AuthorizationAdministrationFlowTestSuite) TestRoleFlows_DeclarativeRoleIsRefused() {
	before := ts.declarativeRolePermissions()
	ts.Require().NotEmpty(before, "The declarative role must grant permissions for the comparison to mean anything")

	step := ts.execute(roleDeletionFlowHandle, map[string]string{roleTargetInput: declarativeRoleID})
	ts.requireRefused(step, errCodeImmutableRole, "a declarative role cannot be deleted")
	ts.True(ts.roleExists(declarativeRoleID), "The declarative role must survive the refused deletion")

	step = ts.execute(rolePermissionRemovalFlowHandle, map[string]string{
		roleTargetInput:        declarativeRoleID,
		permissionsTargetInput: "[]",
	})
	ts.requireRefused(step, errCodeImmutableRole, "a declarative role's permissions cannot be edited")
	ts.Equal(before, ts.declarativeRolePermissions(),
		"The declarative role must grant what it granted before the refused edit")
}

func (ts *AuthorizationAdministrationFlowTestSuite) TestGroupFlows_UnknownGroupIsRefused() {
	step := ts.execute(groupDeletionFlowHandle, map[string]string{groupTargetInput: unknownID})
	ts.requireRefused(step, errCodeGroupNotFound, "a group that does not exist cannot be deleted")

	step = ts.execute(groupMembershipRemovalFlowHandle, map[string]string{
		groupTargetInput:  unknownID,
		memberTargetInput: ts.appID,
	})
	ts.requireRefused(step, errCodeGroupNotFound, "a member cannot leave a group that does not exist")
}

func (ts *AuthorizationAdministrationFlowTestSuite) TestGroupFlows_DeclarativeGroupIsRefused() {
	ts.Require().Contains(ts.groupMemberIDs(declarativeGroupID), declarativeNestedGroupID,
		"The declarative group must hold its declared member for the comparison to mean anything")

	step := ts.execute(groupDeletionFlowHandle, map[string]string{groupTargetInput: declarativeGroupID})
	ts.requireRefused(step, errCodeImmutableGroup, "a declarative group cannot be deleted")
	ts.True(ts.groupExists(declarativeGroupID), "The declarative group must survive the refused deletion")

	step = ts.execute(groupMembershipRemovalFlowHandle, map[string]string{
		groupTargetInput:  declarativeGroupID,
		memberTargetInput: declarativeNestedGroupID,
	})
	ts.requireRefused(step, errCodeImmutableGroup, "a declarative group cannot lose a member")
	ts.Contains(ts.groupMemberIDs(declarativeGroupID), declarativeNestedGroupID,
		"The declarative group must still hold its declared member")
}

func (ts *AuthorizationAdministrationFlowTestSuite) TestGroupMembershipRemovalFlow_NonMemberIsRefused() {
	groupID := ts.createGroup("Authz Flow Non Member Group",
		testutils.Member{Id: ts.appID, Type: "app"})
	ts.createRole("Authz Flow Non Member Administrator", []string{scopeRefusedMemberRemoval},
		testutils.Assignment{ID: groupID, Type: "group"})
	token := ts.grantedToken(scopeRefusedMemberRemoval)
	// The bystander holds the scope through its own role, so a wrongly planned revocation would reject this token.
	ts.createRole("Authz Flow Non Member Direct Administrator", []string{scopeRefusedMemberRemoval},
		testutils.Assignment{ID: ts.bystanderAppID, Type: "app"})
	named := ts.grantedTokenFor(authzBystanderClientID, authzBystanderClientSecret, authzFlowResourceID,
		scopeRefusedMemberRemoval)

	step := ts.execute(groupMembershipRemovalFlowHandle, map[string]string{
		groupTargetInput:  groupID,
		memberTargetInput: ts.bystanderAppID,
	})

	ts.requireRefused(step, errCodeInvalidGroupMember, "the principal is not a member")
	ts.True(ts.tokenIsActiveFor(named, authzBystanderClientID, authzBystanderClientSecret),
		"A refused removal must not revoke the scope the named principal holds through its own role")
	ts.True(ts.tokenIsActive(token), "A refused removal must not revoke the scope the real member holds")
	ts.Equal([]string{ts.appID}, ts.groupMemberIDs(groupID), "A refused removal must leave the membership intact")
}

// Members beyond the descent limit cannot be enumerated, so every change is refused rather than planned
// against a partial member list.
func (ts *AuthorizationAdministrationFlowTestSuite) TestFlows_NestingBeyondTheDescentLimitIsRefused() {
	// Built leaf first, one group past the limit. No principal sits in the chain, since issuing a token would
	// walk the same chain upward.
	chain := make([]string, groupDescentLimit+1)
	var roleID, parentID string
	defer func() {
		// Delete the role and parent first, then the chain top down, so nothing is deleted while still referenced.
		if roleID != "" {
			if err := testutils.DeleteRole(roleID); err != nil {
				ts.T().Logf("Failed to delete the deep group's role during cleanup: %v", err)
			}
		}
		for _, groupID := range append([]string{parentID}, chain...) {
			if groupID == "" {
				continue
			}
			if err := testutils.DeleteGroup(groupID); err != nil {
				ts.T().Logf("Failed to delete nested group %s during cleanup: %v", groupID, err)
			}
		}
	}()
	for level := len(chain) - 1; level >= 0; level-- {
		group := testutils.Group{
			Name:        fmt.Sprintf("Authz Flow Nesting Level %02d", level),
			Description: "Group created for authorization administration flow testing",
			OUID:        ts.ouID,
		}
		if level < len(chain)-1 {
			group.Members = []testutils.Member{{Id: chain[level+1], Type: "group"}}
		}
		groupID, err := testutils.CreateGroup(group)
		ts.Require().NoError(err, "Failed to create nested group at level %d", level)
		chain[level] = groupID
	}
	top := chain[0]

	var err error
	parentID, err = testutils.CreateGroup(testutils.Group{
		Name:        "Authz Flow Nesting Parent",
		Description: "Group created for authorization administration flow testing",
		OUID:        ts.ouID,
		Members:     []testutils.Member{{Id: top, Type: "group"}},
	})
	ts.Require().NoError(err, "Failed to create the group holding the chain")
	roleID, err = testutils.CreateRole(testutils.Role{
		Name:        "Authz Flow Nesting Administrator",
		Description: "Role created for authorization administration flow testing",
		OUID:        ts.ouID,
		Assignments: []testutils.Assignment{{ID: top, Type: "group"}},
	})
	ts.Require().NoError(err, "Failed to create the role held by the chain")

	step := ts.execute(groupDeletionFlowHandle, map[string]string{groupTargetInput: top})
	ts.requireRefused(step, errCodeGroupNestingTooDeep, "the deleted group's members cannot all be reached")
	ts.True(ts.groupExists(top), "A refused deletion must leave the group in place")

	step = ts.execute(groupMembershipRemovalFlowHandle, map[string]string{
		groupTargetInput:  parentID,
		memberTargetInput: top,
	})
	ts.requireRefused(step, errCodeGroupNestingTooDeep, "the departing group's members cannot all be reached")
	ts.Equal([]string{top}, ts.groupMemberIDs(parentID), "A refused removal must leave the membership intact")

	step = ts.execute(roleDeletionFlowHandle, map[string]string{roleTargetInput: roleID})
	ts.requireRefused(step, errCodeGroupNestingTooDeep, "the role's holders cannot all be reached")
	ts.True(ts.roleExists(roleID), "A refused deletion must leave the role in place")

	step = ts.execute(roleAssignmentRemovalFlowHandle, map[string]string{
		roleTargetInput:     roleID,
		assigneeTargetInput: top,
	})
	ts.requireRefused(step, errCodeGroupNestingTooDeep, "the unassigned group's members cannot all be reached")
	assignments, err := testutils.GetRoleAssignments(roleID)
	ts.Require().NoError(err, "Failed to read the role's assignments")
	ts.Len(assignments, 1, "A refused unassignment must leave the assignment in place")
}

func (ts *AuthorizationAdministrationFlowTestSuite) TestGroupDeletionFlow_RevokesAPrincipalReachedAlongSeveralPaths() {
	// doomed holds the app directly and through left and right, which share the innermost group.
	shared := ts.createGroup("Authz Flow Diamond Shared", testutils.Member{Id: ts.appID, Type: "app"})
	left := ts.createGroup("Authz Flow Diamond Left", testutils.Member{Id: shared, Type: "group"})
	right := ts.createGroup("Authz Flow Diamond Right", testutils.Member{Id: shared, Type: "group"})
	doomed := ts.createGroup("Authz Flow Diamond Top",
		testutils.Member{Id: left, Type: "group"},
		testutils.Member{Id: right, Type: "group"},
		testutils.Member{Id: ts.appID, Type: "app"})
	ts.createRole("Authz Flow Diamond Administrator", []string{scopeReachedSeveralWays},
		testutils.Assignment{ID: doomed, Type: "group"})
	token := ts.grantedToken(scopeReachedSeveralWays)

	step := ts.execute(groupDeletionFlowHandle, map[string]string{groupTargetInput: doomed})

	ts.requireCompleted(step)
	ts.False(ts.tokenIsActive(token), "The principal must lose the scope however many paths reached it")
	ts.False(ts.groupExists(doomed), "The group must be deleted")
	for _, nested := range []string{left, right, shared} {
		ts.True(ts.groupExists(nested), "A nested group is a member, not a part, of the deleted group")
	}
}

func (ts *AuthorizationAdministrationFlowTestSuite) TestRoleDeletionFlow_RevokesHoldersReachedThroughNestedGroups() {
	childID := ts.createGroup("Authz Flow Nested Holder Child",
		testutils.Member{Id: ts.appID, Type: "app"})
	parentID := ts.createGroup("Authz Flow Nested Holder Parent",
		testutils.Member{Id: childID, Type: "group"})
	roleID := ts.createRole("Authz Flow Nested Holder Administrator", []string{scopeHeldThroughNesting},
		testutils.Assignment{ID: parentID, Type: "group"})
	token := ts.grantedToken(scopeHeldThroughNesting)

	step := ts.execute(roleDeletionFlowHandle, map[string]string{roleTargetInput: roleID})

	ts.requireCompleted(step)
	ts.False(ts.tokenIsActive(token),
		"A principal holding the role only through a nested group must lose the scope with the role")
	ts.False(ts.roleExists(roleID), "The role must be deleted")
}

func (ts *AuthorizationAdministrationFlowTestSuite) TestRoleDeletionFlow_RevokesAScopeHeldThroughTwoPaths() {
	groupID := ts.createGroup("Authz Flow Two Paths Group",
		testutils.Member{Id: ts.appID, Type: "app"})
	roleID := ts.createRole("Authz Flow Two Paths Administrator", []string{scopeHeldTwice},
		testutils.Assignment{ID: ts.appID, Type: "app"},
		testutils.Assignment{ID: groupID, Type: "group"})
	token := ts.grantedToken(scopeHeldTwice)

	step := ts.execute(roleDeletionFlowHandle, map[string]string{roleTargetInput: roleID})

	ts.requireCompleted(step)
	ts.False(ts.tokenIsActive(token), "The scope must be revoked although the principal held it twice")
	ts.False(ts.roleExists(roleID), "The role must be deleted")
}

func (ts *AuthorizationAdministrationFlowTestSuite) TestGroupDeletionFlow_RevokesMembersOfNestedGroups() {
	childID := ts.createGroup("Authz Flow Deleted Parent Child",
		testutils.Member{Id: ts.appID, Type: "app"})
	parentID := ts.createGroup("Authz Flow Deleted Parent",
		testutils.Member{Id: childID, Type: "group"})
	ts.createRole("Authz Flow Deleted Parent Administrator", []string{scopeNestedGroupDeletion},
		testutils.Assignment{ID: parentID, Type: "group"})
	token := ts.grantedToken(scopeNestedGroupDeletion)

	step := ts.execute(groupDeletionFlowHandle, map[string]string{groupTargetInput: parentID})

	ts.requireCompleted(step)
	ts.False(ts.tokenIsActive(token),
		"A principal in a nested group held the scope through the deleted group and must lose it")
	ts.False(ts.groupExists(parentID), "The group must be deleted")
	ts.True(ts.groupExists(childID), "A nested group is a member, not a part, of the deleted group")
}

func (ts *AuthorizationAdministrationFlowTestSuite) TestGroupMembershipRemovalFlow_RevokesMembersOfARemovedGroup() {
	childID := ts.createGroup("Authz Flow Departing Child",
		testutils.Member{Id: ts.appID, Type: "app"})
	parentID := ts.createGroup("Authz Flow Departing Child Parent",
		testutils.Member{Id: childID, Type: "group"})
	ts.createRole("Authz Flow Departing Child Administrator", []string{scopeNestedMemberRemoval},
		testutils.Assignment{ID: parentID, Type: "group"})
	token := ts.grantedToken(scopeNestedMemberRemoval)

	step := ts.execute(groupMembershipRemovalFlowHandle, map[string]string{
		groupTargetInput:  parentID,
		memberTargetInput: childID,
	})

	ts.requireCompleted(step)
	ts.False(ts.tokenIsActive(token),
		"A principal inside the removed group must lose the scope the parent conveyed")
	ts.Empty(ts.groupMemberIDs(parentID), "The nested group must be gone from the parent")
	ts.Equal([]string{ts.appID}, ts.groupMemberIDs(childID), "The removed group must keep its own members")
}

func (ts *AuthorizationAdministrationFlowTestSuite) TestActionDeletionFlow_RefusalsLeaveTheActionInPlace() {
	actionID := ts.actionIDs[scopeRetiredNeighbor]
	cases := []struct {
		name   string
		inputs map[string]string
		code   string
	}{
		{"resource server that does not exist", map[string]string{
			resourceServerTargetInput: unknownID,
			actionTargetInput:         actionID,
		}, errCodeResourceServerNotFound},
		{"action the resource server does not define", map[string]string{
			resourceServerTargetInput: ts.resourceServerID,
			actionTargetInput:         unknownID,
		}, errCodeActionNotFound},
		{"declarative resource server", map[string]string{
			resourceServerTargetInput: declarativeResourceServerID,
			actionTargetInput:         actionID,
		}, errCodeImmutableAction},
	}
	for _, tc := range cases {
		ts.Run(tc.name, func() {
			step := ts.execute(actionDeletionFlowHandle, tc.inputs)
			ts.requireRefused(step, tc.code, tc.name)
		})
	}

	ts.True(ts.actionExists(actionID), "No refused deletion may remove the action")
}

func (ts *AuthorizationAdministrationFlowTestSuite) TestActionDeletionFlow_UnknownResourceIsRefused() {
	actionID := ts.nestedActionIDs[authzFlowNestedKeepAction]
	permission := ts.nestedPermissions[authzFlowNestedKeepAction]
	ts.createRole("Authz Flow Unknown Resource Administrator", []string{permission},
		testutils.Assignment{ID: ts.appID, Type: "app"})
	token := ts.grantedToken(permission)

	step := ts.execute(actionDeletionFlowHandle, map[string]string{
		resourceServerTargetInput: ts.resourceServerID,
		resourceTargetInput:       unknownID,
		actionTargetInput:         actionID,
	})

	ts.requireRefused(step, errCodeResourceNotFound, "the resource does not exist")
	ts.True(ts.resourceActionExists(actionID), "The action must be untouched")
	ts.True(ts.tokenIsActive(token), "A refused deletion must not retire the scope")
}

func (ts *AuthorizationAdministrationFlowTestSuite) TestActionDeletionFlow_RetiresAnActionDefinedOnAResource() {
	retiredPermission := ts.nestedPermissions[authzFlowNestedRetireAction]
	keptPermission := ts.nestedPermissions[authzFlowNestedKeepAction]
	ts.createRole("Authz Flow Resource Action Administrator", []string{retiredPermission, keptPermission},
		testutils.Assignment{ID: ts.appID, Type: "app"})
	retired := ts.grantedToken(retiredPermission)
	kept := ts.grantedToken(keptPermission)

	step := ts.execute(actionDeletionFlowHandle, map[string]string{
		resourceServerTargetInput: ts.resourceServerID,
		resourceTargetInput:       ts.nestedResourceID,
		actionTargetInput:         ts.nestedActionIDs[authzFlowNestedRetireAction],
	})

	ts.requireCompleted(step)
	ts.False(ts.tokenIsActive(retired), "A token carrying the retired resource action's scope must be rejected")
	ts.True(ts.tokenIsActive(kept), "The sibling action's scope is a different scope and must be untouched")
	ts.False(ts.resourceActionExists(ts.nestedActionIDs[authzFlowNestedRetireAction]),
		"The resource action must be deleted")
	ts.True(ts.resourceActionExists(ts.nestedActionIDs[authzFlowNestedKeepAction]),
		"The sibling action must be untouched")
}
