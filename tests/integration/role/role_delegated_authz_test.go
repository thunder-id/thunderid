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

// RoleDelegatedAuthzTestSuite pins what an organization unit's role administrator may do.
//
// The role administrator holds system:role and system:role:view, but not the root "system"
// permission, so it clears the security middleware on /roles. Two checks then remain:
//
//	SSE-4030 — the role service: the role is in an organization unit the caller does not belong to.
//	SAZ-4030 — sysauthz's grant guard: the operation would confer permissions the caller lacks.
//
// Fixture topology:
//
//	role administrator — in the home OU; holds system:role, system:role:view and system:ou:view
//	harmless role      — home OU; confers nothing
//	privileged role    — home OU; confers system:user, which the role administrator does not hold
//	foreign role       — other OU; confers nothing
type RoleDelegatedAuthzTestSuite struct {
	suite.Suite

	homeOUID       string
	otherOUID      string
	userTypeID     string
	roleAdminID    string
	assigneeUserID string
	rsID           string

	roleAdminRoleID  string
	harmlessRoleID   string
	privilegedRoleID string
	foreignRoleID    string

	roleAdminClient *http.Client
}

const (
	delegatedRSIdentifier = "https://authz-test.example.com/delegated-role"

	delegatedHomeOUHandle  = "authz-delegated-role-ou"
	delegatedOtherOUHandle = "authz-delegated-role-other-ou"
	delegatedTypeName      = "authz-delegated-role-type"

	delegatedAdminUsername = "authz-delegated-role-admin"
	delegatedAdminPassword = "RoleAdmin@123"

	delegatedAssigneeUsername = "authz-delegated-role-assignee"
	delegatedAssigneePassword = "Assignee@123"

	// The permissions the role administrator holds and requests in its token. Deliberately excludes
	// both the root "system" permission and "system:user".
	delegatedRoleAdminPermissions = "system:ou:view system:role system:role:view"

	// errCodeOutsideOrganizationUnit is returned by the role service when the role is in an
	// organization unit the caller does not belong to.
	errCodeOutsideOrganizationUnit = "SSE-4030"
)

func TestRoleDelegatedAuthzTestSuite(t *testing.T) {
	suite.Run(t, new(RoleDelegatedAuthzTestSuite))
}

func (ts *RoleDelegatedAuthzTestSuite) SetupSuite() {
	homeOUID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      delegatedHomeOUHandle,
		Name:        "Delegated Role Authz OU",
		Description: "Organization unit whose roles the role administrator manages",
	})
	ts.Require().NoError(err, "create home OU")
	ts.homeOUID = homeOUID

	otherOUID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      delegatedOtherOUHandle,
		Name:        "Delegated Role Authz Other OU",
		Description: "Organization unit the role administrator does not belong to",
	})
	ts.Require().NoError(err, "create other OU")
	ts.otherOUID = otherOUID

	typeID, err := testutils.CreateUserType(testutils.UserType{
		Handle:      delegatedTypeName,
		DisplayName: "Delegated Role Authz Type",
		OUID:        ts.homeOUID,
		Schema: map[string]interface{}{
			"username":     map[string]interface{}{"type": "string"},
			"password":     map[string]interface{}{"type": "string", "credential": true},
			"display_name": map[string]interface{}{"type": "string"},
		},
	})
	ts.Require().NoError(err, "create user type")
	ts.userTypeID = typeID

	ts.roleAdminID = ts.createUser(delegatedAdminUsername, delegatedAdminPassword, "Role Admin")
	ts.assigneeUserID = ts.createUser(delegatedAssigneeUsername, delegatedAssigneePassword, "Assignee")

	// The fine-grained system permissions are reproduced on a resource server of the suite's own, so
	// the suite does not depend on the System resource server's shape.
	rsID, err := testutils.CreateSystemScopedResourceServer(
		ts.homeOUID, "Authz Test RS (delegated role)", delegatedRSIdentifier, "ou", "role", "user")
	ts.Require().NoError(err, "create scoped resource server")
	ts.rsID = rsID

	ts.roleAdminRoleID = ts.createRole(testutils.Role{
		Name: "authz-delegated-role-admin-role",
		OUID: ts.homeOUID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: rsID, Permissions: []string{"system:ou:view", "system:role", "system:role:view"}},
		},
		Assignments: []testutils.Assignment{{ID: ts.roleAdminID, Type: "user"}},
	})
	ts.harmlessRoleID = ts.createRole(testutils.Role{Name: "authz-delegated-harmless", OUID: ts.homeOUID})
	ts.privilegedRoleID = ts.createRole(testutils.Role{
		Name: "authz-delegated-privileged",
		OUID: ts.homeOUID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: rsID, Permissions: []string{"system:user"}},
		},
	})
	ts.foreignRoleID = ts.createRole(testutils.Role{Name: "authz-delegated-foreign", OUID: ts.otherOUID})

	tokenResp, err := testutils.ObtainAccessTokenWithPassword(
		roleAuthzClientID,
		roleAuthzRedirectURI,
		delegatedRoleAdminPermissions,
		delegatedAdminUsername,
		delegatedAdminPassword,
		true,
		"",
		delegatedRSIdentifier,
	)
	ts.Require().NoError(err, "obtain role administrator token")
	ts.Require().Contains(tokenResp.Scope, "system:role",
		"the role administrator must hold system:role, or the suite proves nothing")
	ts.Require().NotContains(tokenResp.Scope, "system:user",
		"the role administrator must not hold system:user, or the suite proves nothing")

	ts.roleAdminClient = testutils.GetHTTPClientWithToken(tokenResp.AccessToken)
}

func (ts *RoleDelegatedAuthzTestSuite) TearDownSuite() {
	// Roles go first: while they still reference the scoped resource server's permissions, it cannot
	// be deleted.
	for _, id := range []string{ts.roleAdminRoleID, ts.harmlessRoleID, ts.privilegedRoleID, ts.foreignRoleID} {
		if id != "" {
			if err := testutils.DeleteRole(id); err != nil {
				ts.T().Logf("teardown: delete role %s: %v", id, err)
			}
		}
	}
	if ts.rsID != "" {
		if err := testutils.DeleteResourceServer(ts.rsID); err != nil {
			ts.T().Logf("teardown: delete scoped resource server: %v", err)
		}
	}
	for _, id := range []string{ts.roleAdminID, ts.assigneeUserID} {
		if id != "" {
			if err := testutils.DeleteUser(id); err != nil {
				ts.T().Logf("teardown: delete user %s: %v", id, err)
			}
		}
	}
	if ts.userTypeID != "" {
		if err := testutils.DeleteUserType(ts.userTypeID); err != nil {
			ts.T().Logf("teardown: delete user type: %v", err)
		}
	}
	for _, id := range []string{ts.homeOUID, ts.otherOUID} {
		if id != "" {
			if err := testutils.DeleteOrganizationUnit(id); err != nil {
				ts.T().Logf("teardown: delete OU %s: %v", id, err)
			}
		}
	}
}

func (ts *RoleDelegatedAuthzTestSuite) createUser(username, password, displayName string) string {
	ts.T().Helper()
	id, err := testutils.CreateUser(testutils.User{
		Type: delegatedTypeName,
		OUID: ts.homeOUID,
		Attributes: json.RawMessage(fmt.Sprintf(
			`{"username": %q, "password": %q, "display_name": %q}`, username, password, displayName,
		)),
	})
	ts.Require().NoError(err, "create user %s", username)
	return id
}

func (ts *RoleDelegatedAuthzTestSuite) createRole(role testutils.Role) string {
	ts.T().Helper()
	id, err := testutils.CreateRole(role)
	ts.Require().NoError(err, "create role %s", role.Name)
	return id
}

// do issues a request as the role administrator and returns the status and body.
func (ts *RoleDelegatedAuthzTestSuite) do(method, path string, payload any) (int, []byte) {
	ts.T().Helper()

	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		ts.Require().NoError(err)
		body = bytes.NewReader(encoded)
	}

	req, err := http.NewRequest(method, testServerURL+path, body)
	ts.Require().NoError(err)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := ts.roleAdminClient.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	return resp.StatusCode, respBody
}

// requireRefusedWithCode asserts a 403 carrying the given error code, so the test states which
// check answered rather than accepting any refusal.
func (ts *RoleDelegatedAuthzTestSuite) requireRefusedWithCode(status int, body []byte, code string) {
	ts.T().Helper()

	ts.Equalf(http.StatusForbidden, status, "expected a refusal, body: %s", body)

	var errResp map[string]interface{}
	ts.Require().NoError(json.Unmarshal(body, &errResp))
	ts.Equalf(code, errResp["code"], "unexpected refusal layer, body: %s", body)
}

func (ts *RoleDelegatedAuthzTestSuite) assigneePayload() AssignmentsRequest {
	return AssignmentsRequest{Assignments: []Assignment{{ID: ts.assigneeUserID, Type: AssigneeTypeUser}}}
}

// TestRoleAdministratorListsOnlyItsOrganizationUnitRoles verifies the list is scoped to the caller's
// organization unit rather than refused or left unfiltered.
func (ts *RoleDelegatedAuthzTestSuite) TestRoleAdministratorListsOnlyItsOrganizationUnitRoles() {
	status, body := ts.do(http.MethodGet, rolesBasePath+"?limit=100", nil)
	ts.Require().Equalf(http.StatusOK, status, "body: %s", body)

	var list RoleListResponse
	ts.Require().NoError(json.Unmarshal(body, &list))

	ids := make([]string, 0, len(list.Roles))
	for _, role := range list.Roles {
		ts.Equalf(ts.homeOUID, role.OUID, "role %s is outside the caller's organization unit", role.ID)
		ids = append(ids, role.ID)
	}
	ts.Contains(ids, ts.harmlessRoleID)
	ts.NotContains(ids, ts.foreignRoleID)
	ts.Equal(len(list.Roles), list.TotalResults)
}

// TestRoleAdministratorCannotReadForeignRole verifies a role in another organization unit is not
// readable by identifier either.
func (ts *RoleDelegatedAuthzTestSuite) TestRoleAdministratorCannotReadForeignRole() {
	status, body := ts.do(http.MethodGet, rolesBasePath+"/"+ts.foreignRoleID, nil)

	ts.requireRefusedWithCode(status, body, errCodeOutsideOrganizationUnit)
}

// TestRoleAdministratorCannotCreateRoleInAnotherOrganizationUnit verifies creation is limited to
// the caller's organization unit, even for a role that confers nothing.
func (ts *RoleDelegatedAuthzTestSuite) TestRoleAdministratorCannotCreateRoleInAnotherOrganizationUnit() {
	status, body := ts.do(http.MethodPost, rolesBasePath, CreateRoleRequest{
		Name:        "authz-delegated-foreign-create",
		OUID:        ts.otherOUID,
		Permissions: []ResourcePermissions{},
	})

	ts.requireRefusedWithCode(status, body, errCodeOutsideOrganizationUnit)
}

// TestRoleAdministratorCannotMoveRoleIntoAnotherOrganizationUnit verifies an update cannot carry a
// role out of the caller's organization unit.
func (ts *RoleDelegatedAuthzTestSuite) TestRoleAdministratorCannotMoveRoleIntoAnotherOrganizationUnit() {
	status, body := ts.do(http.MethodPut, rolesBasePath+"/"+ts.harmlessRoleID, UpdateRoleRequest{
		Name:        "authz-delegated-harmless",
		OUID:        ts.otherOUID,
		Permissions: []ResourcePermissions{},
	})

	ts.requireRefusedWithCode(status, body, errCodeOutsideOrganizationUnit)
}

// TestRoleAdministratorCannotDeleteForeignRole verifies deletion is limited to the caller's
// organization unit.
func (ts *RoleDelegatedAuthzTestSuite) TestRoleAdministratorCannotDeleteForeignRole() {
	status, body := ts.do(http.MethodDelete, rolesBasePath+"/"+ts.foreignRoleID, nil)

	ts.requireRefusedWithCode(status, body, errCodeOutsideOrganizationUnit)
}

// TestRoleAdministratorCannotCreateRoleConferringUnheldPermissions is the escalation the grant
// guard exists to stop: defining a role that confers system:user, which the caller does not hold.
func (ts *RoleDelegatedAuthzTestSuite) TestRoleAdministratorCannotCreateRoleConferringUnheldPermissions() {
	status, body := ts.do(http.MethodPost, rolesBasePath, CreateRoleRequest{
		Name: "authz-delegated-escalating",
		OUID: ts.homeOUID,
		Permissions: []ResourcePermissions{
			{ResourceServerID: ts.rsID, Permissions: []string{"system:user"}},
		},
		Assignments: []Assignment{{ID: ts.roleAdminID, Type: AssigneeTypeUser}},
	})

	ts.requireRefusedWithCode(status, body, errCodeGrantNotPermitted)
}

// TestRoleAdministratorCannotAssignPrivilegedRole verifies assigning a role that confers a
// permission the caller lacks is refused by the grant guard.
func (ts *RoleDelegatedAuthzTestSuite) TestRoleAdministratorCannotAssignPrivilegedRole() {
	status, body := ts.do(http.MethodPost,
		rolesBasePath+"/"+ts.privilegedRoleID+"/assignments/add", ts.assigneePayload())

	ts.requireRefusedWithCode(status, body, errCodeGrantNotPermitted)
}

// TestRoleAdministratorCanAssignAndUnassignHarmlessRole proves the refusals above are the boundary
// rather than a broken route: a role conferring nothing can be assigned and unassigned.
func (ts *RoleDelegatedAuthzTestSuite) TestRoleAdministratorCanAssignAndUnassignHarmlessRole() {
	status, body := ts.do(http.MethodPost,
		rolesBasePath+"/"+ts.harmlessRoleID+"/assignments/add", ts.assigneePayload())
	ts.Equalf(http.StatusNoContent, status, "assign harmless role, body: %s", body)

	status, body = ts.do(http.MethodPost,
		rolesBasePath+"/"+ts.harmlessRoleID+"/assignments/remove", ts.assigneePayload())
	ts.Equalf(http.StatusNoContent, status, "unassign harmless role, body: %s", body)
}
