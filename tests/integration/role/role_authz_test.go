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
//	harmless role       — confers nothing
//	privileged role     — confers system:user, which the scoped administrator does not hold
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

	// errCodeInsufficientPermissions is returned by the security middleware when the caller lacks
	// the permission the requested path requires.
	errCodeInsufficientPermissions = "AUTH-4030"
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
	for _, id := range []string{ts.scopedAdminRole, ts.harmlessRoleID, ts.privilegedRoleID} {
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
