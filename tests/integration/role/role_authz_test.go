// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// RoleAuthzTestSuite pins the authorization boundary of the role management API.
//
// Unlike /groups and /users, /roles has no entry in the API permission table
// (internal/system/security/permissions.go), so every role path falls back to the root "system"
// permission. A scoped administrator holding the fine-grained system permissions is therefore
// refused on every role endpoint, read and write alike, by the security middleware (AUTH-4030) and
// never reaches the handler.
//
// The refusals are asserted on the error code, not just the status, because two different layers
// answer 403 here:
//
//	AUTH-4030 — the security middleware: the caller lacks the permission the path requires.
//	SAZ-4030  — sysauthz's grant guard: the operation would confer permissions the caller lacks.
//
// Distinguishing them matters. Because the role API admits only root callers, and
// sysauthz.CanGrantMembership short-circuits for root, the role privilege-escalation guard
// (CanGrantMembership with PrincipalTypeRole, and CanGrantPermissions on role create/update) cannot
// fire for any HTTP caller in the shipped configuration. Should /roles later gain fine-grained
// permission entries, TestScopedAdministratorCannotAddAssignmentToPrivilegedRole starts exercising
// the guard, and the expected code becomes SAZ-4030 rather than AUTH-4030.
//
// Fixture topology, all within one OU:
//
//	scoped administrator — holds system:group, system:ou:view, and other non-root system permissions
//	                       directly; holds system:ou only through its own group membership; is also
//	                       assigned a role defined in the declarative file store
//	harmless role        — confers nothing
//	privileged role      — confers system:user, which the scoped administrator does not hold
//	inherited group      — the scoped administrator is a member, and its role confers system:ou
//	privileged group     — its role confers system:user
type RoleAuthzTestSuite struct {
	suite.Suite

	authzOUID        string
	authzTypeID      string
	scopedAdminID    string
	assigneeUserID   string
	scopedRSID       string
	scopedAdminRole  string
	harmlessRoleID   string
	privilegedRoleID string

	// The group conferring system:ou on the scoped administrator, and a group conferring a
	// permission the scoped administrator was never granted.
	inheritedGroupID      string
	inheritedRoleID       string
	privilegedGroupID     string
	privilegedGroupRoleID string

	// HTTP client carrying the scoped administrator's non-root token.
	scopedClient *http.Client
}

const (
	roleAuthzRSIdentifier = "https://authz-test.example.com/role"

	roleAuthzOUHandle = "authz-role-ou"
	roleAuthzTypeName = "authz-role-type"

	roleAuthzAdminUsername = "authz-role-scoped-admin"
	roleAuthzAdminPassword = "ScopedAdmin@123"

	roleAuthzAssigneeUsername = "authz-role-assignee"
	roleAuthzAssigneePassword = "Assignee@123"

	roleAuthzClientID    = "CONSOLE"
	roleAuthzRedirectURI = "https://localhost:8095/console"

	// The permissions the scoped administrator holds and requests in its token. Deliberately
	// excludes both the root "system" permission and "system:user".
	roleAuthzScopedPermissions = "system:ou:view system:group system:group:view"

	// The role loaded from the declarative resources. Its definition lives in the file store while
	// the scoped administrator's assignment to it lives in the database, so resolving the caller's
	// permissions has to read across both.
	roleAuthzDeclarativeRoleID = "decl-role-1"

	// errCodeInsufficientPermissions is returned by the security middleware when the caller lacks
	// the permission the requested path requires.
	errCodeInsufficientPermissions = "AUTH-4030"

	// errCodeGrantNotPermitted is returned by sysauthz when the caller cleared the middleware but the
	// operation would confer a permission it does not itself hold.
	errCodeGrantNotPermitted = "SAZ-4030"
)

func TestRoleAuthzTestSuite(t *testing.T) {
	suite.Run(t, new(RoleAuthzTestSuite))
}

// ---------------------------------------------------------------------------
// Suite setup
// ---------------------------------------------------------------------------

func (ts *RoleAuthzTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      roleAuthzOUHandle,
		Name:        "Role Authz Test OU",
		Description: "Organization unit for the role authorization integration test",
	})
	ts.Require().NoError(err, "create role-authz OU")
	ts.authzOUID = ouID

	typeID, err := testutils.CreateUserType(testutils.UserType{
		Name: roleAuthzTypeName,
		OUID: ts.authzOUID,
		Schema: map[string]interface{}{
			"username":     map[string]interface{}{"type": "string"},
			"password":     map[string]interface{}{"type": "string", "credential": true},
			"display_name": map[string]interface{}{"type": "string"},
		},
	})
	ts.Require().NoError(err, "create user type")
	ts.authzTypeID = typeID

	adminID, err := testutils.CreateUser(testutils.User{
		Type: roleAuthzTypeName,
		OUID: ts.authzOUID,
		Attributes: json.RawMessage(fmt.Sprintf(
			`{"username": %q, "password": %q, "display_name": "Scoped Admin"}`,
			roleAuthzAdminUsername, roleAuthzAdminPassword,
		)),
	})
	ts.Require().NoError(err, "create scoped administrator")
	ts.scopedAdminID = adminID

	assigneeID, err := testutils.CreateUser(testutils.User{
		Type: roleAuthzTypeName,
		OUID: ts.authzOUID,
		Attributes: json.RawMessage(fmt.Sprintf(
			`{"username": %q, "password": %q, "display_name": "Assignee"}`,
			roleAuthzAssigneeUsername, roleAuthzAssigneePassword,
		)),
	})
	ts.Require().NoError(err, "create assignee user")
	ts.assigneeUserID = assigneeID

	// The product ships only the root "system" scope, so the fine-grained system permissions the
	// scoped administrator holds are reproduced on a resource server of the suite's own.
	rsID, err := testutils.CreateSystemScopedResourceServer(
		ts.authzOUID, "Authz Test RS (role)", roleAuthzRSIdentifier, "ou", "group", "user")
	ts.Require().NoError(err, "create scoped resource server")
	ts.scopedRSID = rsID

	adminRoleID, err := testutils.CreateRole(testutils.Role{
		Name: "authz-role-scoped-admin-role",
		OUID: ts.authzOUID,
		Permissions: []testutils.ResourcePermissions{
			{
				ResourceServerID: rsID,
				Permissions:      []string{"system:ou:view", "system:group", "system:group:view"},
			},
		},
		Assignments: []testutils.Assignment{
			{ID: ts.scopedAdminID, Type: "user"},
		},
	})
	ts.Require().NoError(err, "create scoped administrator role")
	ts.scopedAdminRole = adminRoleID

	// Confers nothing, so the grant guard would allow managing it. Only the middleware stands in
	// the way, which is what makes this fixture the discriminating one.
	harmlessID, err := testutils.CreateRole(testutils.Role{
		Name:        "authz-role-harmless",
		Description: "Role conferring no permissions",
		OUID:        ts.authzOUID,
	})
	ts.Require().NoError(err, "create harmless role")
	ts.harmlessRoleID = harmlessID

	// Confers system:user, which the scoped administrator was never granted. Assigning anyone to it
	// would transfer a permission the caller does not hold.
	privilegedID, err := testutils.CreateRole(testutils.Role{
		Name:        "authz-role-privileged",
		Description: "Role conferring system:user",
		OUID:        ts.authzOUID,
		Permissions: []testutils.ResourcePermissions{
			{
				ResourceServerID: rsID,
				Permissions:      []string{"system:user"},
			},
		},
	})
	ts.Require().NoError(err, "create privileged role")
	ts.privilegedRoleID = privilegedID

	// A group the scoped administrator belongs to. Its role confers system:ou, which the scoped
	// administrator holds nowhere else, so any decision that turns on system:ou can only be reached
	// by resolving the caller's permissions through its own group membership.
	inheritedGroupID, err := testutils.CreateGroup(testutils.Group{
		Name:        "authz-role-inherited-group",
		Description: "Group conferring system:ou on the scoped administrator",
		OUID:        ts.authzOUID,
		Members:     []testutils.Member{{Id: ts.scopedAdminID, Type: "user"}},
	})
	ts.Require().NoError(err, "create inherited group")
	ts.inheritedGroupID = inheritedGroupID

	inheritedRoleID, err := testutils.CreateRole(testutils.Role{
		Name:        "authz-role-inherited",
		Description: "Role conferring system:ou on the inherited group",
		OUID:        ts.authzOUID,
		Permissions: []testutils.ResourcePermissions{
			{
				ResourceServerID: rsID,
				Permissions:      []string{"system:ou"},
			},
		},
		Assignments: []testutils.Assignment{
			{ID: ts.inheritedGroupID, Type: "group"},
		},
	})
	ts.Require().NoError(err, "create inherited role")
	ts.inheritedRoleID = inheritedRoleID

	// A group whose role confers system:user, so joining anyone to it would transfer a permission the
	// scoped administrator was never granted.
	privilegedGroupID, err := testutils.CreateGroup(testutils.Group{
		Name:        "authz-role-privileged-group",
		Description: "Group conferring system:user",
		OUID:        ts.authzOUID,
	})
	ts.Require().NoError(err, "create privileged group")
	ts.privilegedGroupID = privilegedGroupID

	privilegedGroupRoleID, err := testutils.CreateRole(testutils.Role{
		Name:        "authz-role-privileged-group-role",
		Description: "Role conferring system:user on the privileged group",
		OUID:        ts.authzOUID,
		Permissions: []testutils.ResourcePermissions{
			{
				ResourceServerID: rsID,
				Permissions:      []string{"system:user"},
			},
		},
		Assignments: []testutils.Assignment{
			{ID: ts.privilegedGroupID, Type: "group"},
		},
	})
	ts.Require().NoError(err, "create privileged group role")
	ts.privilegedGroupRoleID = privilegedGroupRoleID

	// Also assign the scoped administrator to the declarative role, so resolving its permissions
	// spans the database and file stores. The permissions this adds are on an unrelated resource
	// server and so change no decision below; a store that failed to resolve would surface as a 500
	// where TestScopedAdministratorCanAddMemberToInheritedGroup expects success.
	ts.Require().NoError(ts.assignScopedAdminToDeclarativeRole(),
		"assign the scoped administrator to the declarative role")

	tokenResp, err := testutils.ObtainAccessTokenWithPassword(
		roleAuthzClientID,
		roleAuthzRedirectURI,
		roleAuthzScopedPermissions,
		roleAuthzAdminUsername,
		roleAuthzAdminPassword,
		true,
		"",
		roleAuthzRSIdentifier,
	)
	ts.Require().NoError(err, "obtain scoped administrator token")
	ts.Require().NotEmpty(tokenResp.AccessToken, "scoped administrator token must be non-empty")
	ts.Require().NotContains(tokenResp.Scope, "system:user",
		"the scoped administrator must not hold system:user, or the suite proves nothing")

	ts.scopedClient = testutils.GetHTTPClientWithToken(tokenResp.AccessToken)
}

// ---------------------------------------------------------------------------
// Suite teardown
// ---------------------------------------------------------------------------

func (ts *RoleAuthzTestSuite) TearDownSuite() {
	// Asserted rather than logged: the declarative role is shared with RoleAPITestSuite, which counts
	// its assignments, so a leaked assignment must fail here instead of there.
	if ts.scopedAdminID != "" {
		ts.NoError(ts.unassignScopedAdminFromDeclarativeRole(),
			"teardown: remove the declarative role assignment")
	}
	// Roles go first: while they still reference the scoped resource server's permissions, it cannot
	// be deleted.
	for _, id := range []string{
		ts.scopedAdminRole, ts.harmlessRoleID, ts.privilegedRoleID,
		ts.inheritedRoleID, ts.privilegedGroupRoleID,
	} {
		if id != "" {
			if err := testutils.DeleteRole(id); err != nil {
				ts.T().Logf("teardown: delete role %s: %v", id, err)
			}
		}
	}
	if ts.scopedRSID != "" {
		if err := testutils.DeleteResourceServer(ts.scopedRSID); err != nil {
			ts.T().Logf("teardown: delete scoped resource server: %v", err)
		}
	}
	for _, id := range []string{ts.inheritedGroupID, ts.privilegedGroupID} {
		if id != "" {
			if err := testutils.DeleteGroup(id); err != nil {
				ts.T().Logf("teardown: delete group %s: %v", id, err)
			}
		}
	}
	for _, id := range []string{ts.scopedAdminID, ts.assigneeUserID} {
		if id != "" {
			if err := testutils.DeleteUser(id); err != nil {
				ts.T().Logf("teardown: delete user %s: %v", id, err)
			}
		}
	}
	if ts.authzTypeID != "" {
		if err := testutils.DeleteUserType(ts.authzTypeID); err != nil {
			ts.T().Logf("teardown: delete user type: %v", err)
		}
	}
	if ts.authzOUID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.authzOUID); err != nil {
			ts.T().Logf("teardown: delete role-authz OU: %v", err)
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// doScoped issues a request as the scoped administrator.
func (ts *RoleAuthzTestSuite) doScoped(method, path string, body []byte) *http.Response {
	ts.T().Helper()

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, testServerURL+path, bodyReader)
	ts.Require().NoError(err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := ts.scopedClient.Do(req)
	ts.Require().NoError(err)
	return resp
}

// requireRefusedWithCode asserts a 403 carrying the given error code, so the test states which
// enforcement layer answered rather than accepting any refusal.
func (ts *RoleAuthzTestSuite) requireRefusedWithCode(resp *http.Response, code string) {
	ts.T().Helper()

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)

	ts.Equalf(http.StatusForbidden, resp.StatusCode, "expected a refusal, body: %s", body)

	// Decoded loosely: the message and description are i18n objects, not the plain strings
	// ErrorResponse declares, and only the code identifies the layer that refused.
	var errResp map[string]interface{}
	ts.Require().NoError(json.Unmarshal(body, &errResp))
	ts.Equalf(code, errResp["code"], "unexpected refusal layer, body: %s", body)
}

// mustMarshal encodes a JSON request body, failing the test on error.
func (ts *RoleAuthzTestSuite) mustMarshal(v any) []byte {
	ts.T().Helper()
	payload, err := json.Marshal(v)
	ts.Require().NoError(err)
	return payload
}

// assigneePayload builds an assignments request naming the assignee user.
func (ts *RoleAuthzTestSuite) assigneePayload() []byte {
	ts.T().Helper()
	return ts.mustMarshal(AssignmentsRequest{
		Assignments: []Assignment{{ID: ts.assigneeUserID, Type: AssigneeTypeUser}},
	})
}

// membersPayload builds a group members request naming the given users.
func (ts *RoleAuthzTestSuite) membersPayload(userIDs ...string) []byte {
	ts.T().Helper()

	members := make([]Member, 0, len(userIDs))
	for _, id := range userIDs {
		members = append(members, Member{ID: id, Type: string(AssigneeTypeUser)})
	}
	return ts.mustMarshal(MembersRequest{Members: members})
}

// assignScopedAdminToDeclarativeRole binds the scoped administrator to the file-defined role using
// the root client, since the scoped administrator cannot reach the role API itself.
func (ts *RoleAuthzTestSuite) assignScopedAdminToDeclarativeRole() error {
	payload, err := json.Marshal(AssignmentsRequest{
		Assignments: []Assignment{{ID: ts.scopedAdminID, Type: AssigneeTypeUser}},
	})
	if err != nil {
		return err
	}

	url := testServerURL + rolesBasePath + "/" + roleAuthzDeclarativeRoleID + "/assignments/add"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("add declarative role assignment failed with status %d: %s",
			resp.StatusCode, body)
	}
	return nil
}

// unassignScopedAdminFromDeclarativeRole withdraws the assignment added during setup.
func (ts *RoleAuthzTestSuite) unassignScopedAdminFromDeclarativeRole() error {
	return testutils.RemoveRoleAssignments(roleAuthzDeclarativeRoleID,
		[]testutils.Assignment{{ID: ts.scopedAdminID, Type: "user"}})
}

// rootRoleStatus fetches a role with the root client and reports the response status, so a refusal
// can be shown to have left the role untouched.
func (ts *RoleAuthzTestSuite) rootRoleStatus(roleID string) int {
	ts.T().Helper()

	req, err := http.NewRequest(http.MethodGet, testServerURL+rolesBasePath+"/"+roleID, nil)
	ts.Require().NoError(err)

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	return resp.StatusCode
}

// ---------------------------------------------------------------------------
// Reads
// ---------------------------------------------------------------------------

// TestScopedAdministratorCannotListRoles verifies role listing is closed to non-root callers.
func (ts *RoleAuthzTestSuite) TestScopedAdministratorCannotListRoles() {
	resp := ts.doScoped(http.MethodGet, rolesBasePath, nil)
	defer resp.Body.Close()

	ts.requireRefusedWithCode(resp, errCodeInsufficientPermissions)
}

// TestScopedAdministratorCannotGetRole verifies reading a single role is closed to non-root callers.
func (ts *RoleAuthzTestSuite) TestScopedAdministratorCannotGetRole() {
	resp := ts.doScoped(http.MethodGet, rolesBasePath+"/"+ts.harmlessRoleID, nil)
	defer resp.Body.Close()

	ts.requireRefusedWithCode(resp, errCodeInsufficientPermissions)
}

// TestScopedAdministratorCannotListRoleAssignments verifies that reading who holds a role, which
// discloses the privileges of other principals, is closed to non-root callers.
func (ts *RoleAuthzTestSuite) TestScopedAdministratorCannotListRoleAssignments() {
	resp := ts.doScoped(http.MethodGet, rolesBasePath+"/"+ts.privilegedRoleID+"/assignments", nil)
	defer resp.Body.Close()

	ts.requireRefusedWithCode(resp, errCodeInsufficientPermissions)
}

// ---------------------------------------------------------------------------
// Writes
// ---------------------------------------------------------------------------

// TestScopedAdministratorCannotCreateRoleConferringUnheldPermissions covers the escalation a
// scoped administrator would attempt first: minting a role that confers more than the caller holds,
// then assigning itself to it.
func (ts *RoleAuthzTestSuite) TestScopedAdministratorCannotCreateRoleConferringUnheldPermissions() {
	payload := ts.mustMarshal(CreateRoleRequest{
		Name: "authz-role-escalating",
		OUID: ts.authzOUID,
		Permissions: []ResourcePermissions{
			{ResourceServerID: ts.scopedRSID, Permissions: []string{"system:user"}},
		},
		Assignments: []Assignment{{ID: ts.scopedAdminID, Type: AssigneeTypeUser}},
	})

	resp := ts.doScoped(http.MethodPost, rolesBasePath, payload)
	defer resp.Body.Close()

	ts.requireRefusedWithCode(resp, errCodeInsufficientPermissions)
}

// TestScopedAdministratorCannotUpdateRoleToConferUnheldPermissions covers the same escalation
// through an update, which replaces the permission list wholesale.
func (ts *RoleAuthzTestSuite) TestScopedAdministratorCannotUpdateRoleToConferUnheldPermissions() {
	payload := ts.mustMarshal(UpdateRoleRequest{
		Name: "authz-role-harmless",
		OUID: ts.authzOUID,
		Permissions: []ResourcePermissions{
			{ResourceServerID: ts.scopedRSID, Permissions: []string{"system:user"}},
		},
	})

	resp := ts.doScoped(http.MethodPut, rolesBasePath+"/"+ts.harmlessRoleID, payload)
	defer resp.Body.Close()

	ts.requireRefusedWithCode(resp, errCodeInsufficientPermissions)
}

// TestScopedAdministratorCannotDeleteRole verifies deletion, which silently strips privileges from
// every assignee, is closed to non-root callers.
func (ts *RoleAuthzTestSuite) TestScopedAdministratorCannotDeleteRole() {
	resp := ts.doScoped(http.MethodDelete, rolesBasePath+"/"+ts.harmlessRoleID, nil)
	defer resp.Body.Close()

	ts.requireRefusedWithCode(resp, errCodeInsufficientPermissions)

	// The status alone would not distinguish a refusal from a deletion reported as one, so the role
	// is read back with the root client.
	ts.Equal(http.StatusOK, ts.rootRoleStatus(ts.harmlessRoleID),
		"the harmless role must survive the refused delete")
}

// TestScopedAdministratorCannotAddAssignmentToPrivilegedRole is the escalation the grant guard
// exists to stop: assigning a principal to a role conferring system:user, which the caller does not
// hold. The refusal currently comes from the middleware, so the guard itself never runs.
func (ts *RoleAuthzTestSuite) TestScopedAdministratorCannotAddAssignmentToPrivilegedRole() {
	resp := ts.doScoped(http.MethodPost,
		rolesBasePath+"/"+ts.privilegedRoleID+"/assignments/add", ts.assigneePayload())
	defer resp.Body.Close()

	ts.requireRefusedWithCode(resp, errCodeInsufficientPermissions)
}

// TestScopedAdministratorCannotRemoveAssignmentFromPrivilegedRole covers the other direction.
// Stripping an assignment is guarded to the same standard, since it changes who holds the role's
// privileges.
func (ts *RoleAuthzTestSuite) TestScopedAdministratorCannotRemoveAssignmentFromPrivilegedRole() {
	resp := ts.doScoped(http.MethodPost,
		rolesBasePath+"/"+ts.privilegedRoleID+"/assignments/remove", ts.assigneePayload())
	defer resp.Body.Close()

	ts.requireRefusedWithCode(resp, errCodeInsufficientPermissions)
}

// TestScopedAdministratorCannotAddAssignmentToHarmlessRole is what shows the boundary is drawn at
// the path and not at the conferred permissions. The harmless role confers nothing, so the grant
// guard would permit this; the middleware refuses it anyway.
func (ts *RoleAuthzTestSuite) TestScopedAdministratorCannotAddAssignmentToHarmlessRole() {
	resp := ts.doScoped(http.MethodPost,
		rolesBasePath+"/"+ts.harmlessRoleID+"/assignments/add", ts.assigneePayload())
	defer resp.Body.Close()

	ts.requireRefusedWithCode(resp, errCodeInsufficientPermissions)
}

// ---------------------------------------------------------------------------
// Escalation through group membership
// ---------------------------------------------------------------------------
//
// These are the only requests in this suite that clear the middleware, since /groups/** is gated on
// system:group rather than the root permission. They therefore reach sysauthz and are the sole
// coverage of what a role confers being weighed against what the caller holds.

// TestScopedAdministratorCannotAddMemberToPrivilegedGroup covers the escalation left open by the
// role API being closed: the scoped administrator cannot mint or reassign roles, but it can manage
// group membership, and a group's roles confer their permissions on every member. Joining anyone to
// a group whose role confers system:user would hand out a permission the caller never held, so the
// grant guard refuses, this time with SAZ-4030 rather than AUTH-4030.
func (ts *RoleAuthzTestSuite) TestScopedAdministratorCannotAddMemberToPrivilegedGroup() {
	resp := ts.doScoped(http.MethodPost,
		"/groups/"+ts.privilegedGroupID+"/members/add", ts.membersPayload(ts.assigneeUserID))
	defer resp.Body.Close()

	ts.requireRefusedWithCode(resp, errCodeGrantNotPermitted)
}

// TestScopedAdministratorCannotAddSelfToPrivilegedGroup is the same escalation aimed at the caller
// itself, which is the shorter route to the same privileges and must be refused on the same grounds.
func (ts *RoleAuthzTestSuite) TestScopedAdministratorCannotAddSelfToPrivilegedGroup() {
	resp := ts.doScoped(http.MethodPost,
		"/groups/"+ts.privilegedGroupID+"/members/add", ts.membersPayload(ts.scopedAdminID))
	defer resp.Body.Close()

	ts.requireRefusedWithCode(resp, errCodeGrantNotPermitted)
}

// TestScopedAdministratorCannotRemoveMemberFromPrivilegedGroup covers the other direction. Removal
// reshapes who holds the group's privileges, so it is held to the same requirement as addition.
func (ts *RoleAuthzTestSuite) TestScopedAdministratorCannotRemoveMemberFromPrivilegedGroup() {
	resp := ts.doScoped(http.MethodPost,
		"/groups/"+ts.privilegedGroupID+"/members/remove", ts.membersPayload(ts.assigneeUserID))
	defer resp.Body.Close()

	ts.requireRefusedWithCode(resp, errCodeGrantNotPermitted)
}

// TestScopedAdministratorCanAddMemberToInheritedGroup is the positive half of the guard, and the
// only test here that reaches a role's permissions being resolved for the caller rather than for the
// target. The inherited group's role confers system:ou, which the scoped administrator holds nowhere
// directly, so the grant is permitted only if the caller's own group membership counts towards the
// comparison. Requiring the caller's declarative role assignment to resolve as well, this fails as a
// 500 if either store is skipped and as a 403 if group nesting is not walked for the caller.
func (ts *RoleAuthzTestSuite) TestScopedAdministratorCanAddMemberToInheritedGroup() {
	resp := ts.doScoped(http.MethodPost, "/groups/"+ts.inheritedGroupID+"/members/add",
		ts.membersPayload(ts.scopedAdminID, ts.assigneeUserID))
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equalf(http.StatusOK, resp.StatusCode,
		"joining a group conferring only permissions the caller holds must be allowed, body: %s", body)

	// Restore the original membership, or the fixture stops conferring what the suite documents.
	removeResp := ts.doScoped(http.MethodPost, "/groups/"+ts.inheritedGroupID+"/members/remove",
		ts.membersPayload(ts.assigneeUserID))
	defer removeResp.Body.Close()

	removeBody, err := io.ReadAll(removeResp.Body)
	ts.Require().NoError(err)
	ts.Equalf(http.StatusOK, removeResp.StatusCode,
		"removal is held to the same requirement and must also be allowed here, body: %s", removeBody)
}

// ---------------------------------------------------------------------------
// Root caller
// ---------------------------------------------------------------------------

// TestRootAdministratorCanManageRoleAssignments proves the refusals above are the authorization
// boundary rather than a broken route or a malformed fixture: the same requests succeed for a root
// caller.
func (ts *RoleAuthzTestSuite) TestRootAdministratorCanManageRoleAssignments() {
	client := testutils.GetHTTPClient()
	payload := ts.assigneePayload()

	addReq, err := http.NewRequest(http.MethodPost,
		testServerURL+rolesBasePath+"/"+ts.privilegedRoleID+"/assignments/add",
		bytes.NewReader(payload))
	ts.Require().NoError(err)
	addReq.Header.Set("Content-Type", "application/json")

	addResp, err := client.Do(addReq)
	ts.Require().NoError(err)
	defer addResp.Body.Close()

	addBody, err := io.ReadAll(addResp.Body)
	ts.Require().NoError(err)
	ts.Equalf(http.StatusNoContent, addResp.StatusCode,
		"root caller should assign a privileged role, body: %s", addBody)

	removeReq, err := http.NewRequest(http.MethodPost,
		testServerURL+rolesBasePath+"/"+ts.privilegedRoleID+"/assignments/remove",
		bytes.NewReader(payload))
	ts.Require().NoError(err)
	removeReq.Header.Set("Content-Type", "application/json")

	removeResp, err := client.Do(removeReq)
	ts.Require().NoError(err)
	defer removeResp.Body.Close()

	removeBody, err := io.ReadAll(removeResp.Body)
	ts.Require().NoError(err)
	ts.Equalf(http.StatusNoContent, removeResp.StatusCode,
		"root caller should unassign a privileged role, body: %s", removeBody)
}
