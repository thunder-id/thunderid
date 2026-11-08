// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"bytes"
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

// RoleOUConfinementTestSuite verifies that role management on a data plane confines a caller holding
// `system:role` (and `system:ou:view`, to reach the organization unit routes) to the organization unit its
// token names.
//
//	A
//	└── A1 ← caller, owns RA1, and RA is shared into it with assignments editable
//	B      ← caller, owns RB
//
// RA is owned by A.
type RoleOUConfinementTestSuite struct {
	suite.Suite

	signIn testutils.OUSignInApps

	ouA, ouA1, ouB string

	userTypeIDs []string
	userIDs     []string
	rsIDs       []string
	callerRoles []string

	a1UserID string

	roleA, roleA1, roleB string
	createdRoles         []string

	a1Client *http.Client
	bClient  *http.Client
}

const roleConfinementPassword = "RoleConfinement@123"

func TestRoleOUConfinementTestSuite(t *testing.T) {
	suite.Run(t, new(RoleOUConfinementTestSuite))
}

func (s *RoleOUConfinementTestSuite) SetupSuite() {
	var err error
	s.ouA, err = testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "role-confinement-a", Name: "Role Confinement A"})
	s.Require().NoError(err)
	s.ouA1, err = testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "role-confinement-a1", Name: "Role Confinement A1", Parent: &s.ouA})
	s.Require().NoError(err)
	s.ouB, err = testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "role-confinement-b", Name: "Role Confinement B"})
	s.Require().NoError(err)

	s.roleA = s.adminCreateRole("Role Confinement RA", s.ouA)
	s.roleA1 = s.adminCreateRole("Role Confinement RA1", s.ouA1)
	s.roleB = s.adminCreateRole("Role Confinement RB", s.ouB)
	s.adminShareIntoA1(s.roleA)

	s.a1Client, s.a1UserID = s.scopedCaller(s.ouA1, "a1")
	s.bClient, _ = s.scopedCaller(s.ouB, "b")
}

func (s *RoleOUConfinementTestSuite) TearDownSuite() {
	s.NoError(s.signIn.DeleteAll(), "teardown: delete sign-in applications")
	for _, id := range append(append([]string{}, s.callerRoles...), s.createdRoles...) {
		s.NoError(testutils.DeleteRole(id), "teardown: delete role %s", id)
	}
	for _, id := range s.rsIDs {
		s.NoError(testutils.DeleteResourceServerWithChildren(id), "teardown: delete resource server")
	}
	for _, id := range s.userIDs {
		s.NoError(testutils.DeleteUser(id), "teardown: delete user")
	}
	for _, id := range s.userTypeIDs {
		s.NoError(testutils.DeleteUserType(id), "teardown: delete user type")
	}
	for _, id := range []string{s.ouA1, s.ouA, s.ouB} {
		if id != "" {
			s.NoError(testutils.DeleteOrganizationUnit(id), "teardown: delete organization unit")
		}
	}
}

// ---- Lists ----

func (s *RoleOUConfinementTestSuite) TestConfinedListReturnsOwnedThenShared() {
	status, body := s.send(s.a1Client, http.MethodGet, rolesBasePath+"?limit=100", nil)
	s.Require().Equal(http.StatusOK, status, string(body))
	roles := s.decodeList(body)

	ids := roleIDs(roles)
	s.Contains(ids, s.roleA1)
	s.Contains(ids, s.roleA)
	s.NotContains(ids, s.roleB)
	s.Less(indexOf(ids, s.roleA1), indexOf(ids, s.roleA), "owned roles come before shared ones")
	for _, r := range roles {
		switch r.ID {
		case s.roleA1:
			s.True(r.IsOwnedByOU)
			s.Equal(s.ouA1, r.OUID)
		case s.roleA:
			s.False(r.IsOwnedByOU)
			s.Equal(s.ouA1, r.OUID, "a shared role carries the listed organization unit")
			s.True(r.IsReadOnly)
		}
	}
}

func (s *RoleOUConfinementTestSuite) TestFilterOnAnotherOUIsEmpty() {
	filter := url.QueryEscape(fmt.Sprintf(`ouId eq "%s"`, s.ouB))
	status, body := s.send(s.a1Client, http.MethodGet, rolesBasePath+"?filter="+filter, nil)

	s.Require().Equal(http.StatusOK, status, string(body))
	s.Empty(s.decodeList(body))
}

func (s *RoleOUConfinementTestSuite) TestOURolesRouteListsOwnOU() {
	status, body := s.send(s.a1Client, http.MethodGet, "/organization-units/"+s.ouA1+"/roles?limit=100", nil)
	s.Require().Equal(http.StatusOK, status, string(body))

	ids := roleIDs(s.decodeList(body))
	s.Contains(ids, s.roleA1)
	s.Contains(ids, s.roleA)
}

func (s *RoleOUConfinementTestSuite) TestOURolesRouteRefusesAnotherOU() {
	status, body := s.send(s.a1Client, http.MethodGet, "/organization-units/"+s.ouB+"/roles", nil)
	s.requireRefused(status, body)
}

// ---- Reads ----

func (s *RoleOUConfinementTestSuite) TestShareeReadsSharedRole() {
	status, body := s.send(s.a1Client, http.MethodGet, rolesBasePath+"/"+s.roleA, nil)
	s.Require().Equal(http.StatusOK, status, string(body))

	var role confinedRole
	s.Require().NoError(json.Unmarshal(body, &role))
	s.Equal(s.roleA, role.ID)
	s.Equal(s.ouA1, role.OUID)
	s.False(role.IsOwnedByOU)
}

func (s *RoleOUConfinementTestSuite) TestUnrelatedOUCannotReadRole() {
	status, body := s.send(s.bClient, http.MethodGet, rolesBasePath+"/"+s.roleA, nil)
	s.requireRefused(status, body)
}

// ---- Writes ----

func (s *RoleOUConfinementTestSuite) TestShareeCannotUpdateSharedRole() {
	status, body := s.send(s.a1Client, http.MethodPut, rolesBasePath+"/"+s.roleA,
		map[string]interface{}{"name": "Renamed By A1", "ouId": s.ouA, "permissions": []interface{}{}})
	s.requireRefused(status, body)
}

func (s *RoleOUConfinementTestSuite) TestCannotCreateRoleInAnotherOU() {
	status, body := s.send(s.a1Client, http.MethodPost, rolesBasePath,
		map[string]interface{}{"name": "Created By A1 In B", "ouId": s.ouB, "permissions": []interface{}{}})
	s.requireRefused(status, body)
}

func (s *RoleOUConfinementTestSuite) TestCannotMoveRoleToAnotherOU() {
	status, body := s.send(s.a1Client, http.MethodPut, rolesBasePath+"/"+s.roleA1,
		map[string]interface{}{"name": "Role Confinement RA1", "ouId": s.ouB, "permissions": []interface{}{}})
	s.requireRefused(status, body)

	status, body = s.send(testutils.GetHTTPClient(), http.MethodGet, rolesBasePath+"/"+s.roleA1, nil)
	s.Require().Equal(http.StatusOK, status, string(body))
	var role confinedRole
	s.Require().NoError(json.Unmarshal(body, &role))
	s.Equal(s.ouA1, role.OUID, "a refused move must leave the role where it was")
}

func (s *RoleOUConfinementTestSuite) TestOwnerManagesItsRole() {
	status, body := s.send(s.a1Client, http.MethodPost, rolesBasePath,
		map[string]interface{}{"name": "Created By A1", "ouId": s.ouA1, "permissions": []interface{}{}})
	s.Require().Equal(http.StatusCreated, status, string(body))
	var created confinedRole
	s.Require().NoError(json.Unmarshal(body, &created))

	status, body = s.send(s.a1Client, http.MethodPut, rolesBasePath+"/"+created.ID,
		map[string]interface{}{"name": "Renamed By A1", "ouId": s.ouA1, "permissions": []interface{}{}})
	s.Require().Equal(http.StatusOK, status, string(body))

	status, body = s.send(s.a1Client, http.MethodDelete, rolesBasePath+"/"+created.ID, nil)
	s.Require().Equal(http.StatusNoContent, status, string(body))
}

// ---- Assignments ----

func (s *RoleOUConfinementTestSuite) TestShareeAssignsSharedRoleInItsOU() {
	status, body := s.send(s.a1Client, http.MethodPost, rolesBasePath+"/"+s.roleA+"/assignments/add",
		map[string]interface{}{"ouId": s.ouA1, "assignments": []map[string]string{{"id": s.a1UserID, "type": "user"}}})
	s.Require().Equal(http.StatusNoContent, status, string(body))

	status, body = s.send(s.a1Client, http.MethodPost, rolesBasePath+"/"+s.roleA+"/assignments/remove",
		map[string]interface{}{"ouId": s.ouA1, "assignments": []map[string]string{{"id": s.a1UserID, "type": "user"}}})
	s.Require().Equal(http.StatusNoContent, status, string(body))
}

func (s *RoleOUConfinementTestSuite) TestAssignmentInAnotherOURefused() {
	status, body := s.send(s.a1Client, http.MethodPost, rolesBasePath+"/"+s.roleB+"/assignments/add",
		map[string]interface{}{"ouId": s.ouB, "assignments": []map[string]string{{"id": s.a1UserID, "type": "user"}}})
	s.requireRefused(status, body)
}

// ---- Unconfined control ----

func (s *RoleOUConfinementTestSuite) TestRootTokenStaysUnconfined() {
	ids := s.allRoleIDs(testutils.GetHTTPClient())

	s.Contains(ids, s.roleB)
	s.Contains(ids, s.roleA1)
	count := 0
	for _, id := range ids {
		if id == s.roleA {
			count++
		}
	}
	s.Equal(1, count, "an unconfined list carries every role once, without shared duplicates")
}

// ---- Helpers ----

// confinedRole is the part of a role response these tests read.
type confinedRole struct {
	ID          string `json:"id"`
	OUID        string `json:"ouId"`
	IsOwnedByOU bool   `json:"isOwnedByOU"`
	IsReadOnly  bool   `json:"isReadOnly"`
}

// adminCreateRole creates a role with no permissions in the organization unit and returns its ID.
func (s *RoleOUConfinementTestSuite) adminCreateRole(name, ouID string) string {
	s.T().Helper()
	id, err := testutils.CreateRole(testutils.Role{Name: name, OUID: ouID})
	s.Require().NoError(err, "create role %s", name)
	s.createdRoles = append(s.createdRoles, id)
	return id
}

// adminShareIntoA1 shares the role into A1 with its assignments editable there.
func (s *RoleOUConfinementTestSuite) adminShareIntoA1(roleID string) {
	s.T().Helper()
	status, body := s.send(testutils.GetHTTPClient(), http.MethodPost, rolesBasePath+"/"+roleID+"/sharing-policies",
		map[string]interface{}{"childUnits": map[string]interface{}{
			"mode": "selected", "units": []map[string]interface{}{{
				"ouId": s.ouA1, "reach": "unit",
				"overlayRules": map[string]interface{}{"assignments": map[string]bool{"editable": true}},
			}},
		}})
	s.Require().Equal(http.StatusCreated, status, "share role into A1: %s", string(body))
}

// scopedCaller provisions a user in the organization unit holding `system:role` and returns a client carrying
// the user's token, and the user's ID.
func (s *RoleOUConfinementTestSuite) scopedCaller(ouID, name string) (*http.Client, string) {
	s.T().Helper()

	userTypeHandle := "role-confinement-" + name
	userTypeID, err := testutils.CreateUserType(testutils.UserType{
		Handle:      userTypeHandle,
		DisplayName: "Role Confinement " + name,
		OUID:        ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string", "unique": true},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	})
	s.Require().NoError(err, "create the %s user type", name)
	s.userTypeIDs = append(s.userTypeIDs, userTypeID)

	username := userTypeHandle + "-user"
	userID, err := testutils.CreateUser(testutils.User{
		Type: userTypeHandle,
		OUID: ouID,
		Attributes: json.RawMessage(fmt.Sprintf(
			`{"username": %q, "password": %q}`, username, roleConfinementPassword)),
	})
	s.Require().NoError(err, "create the %s user", name)
	s.userIDs = append(s.userIDs, userID)

	identifier := "https://authz-test.example.com/role-confinement-" + name
	rsID, err := testutils.CreateSystemScopedResourceServer(ouID, "Role Confinement RS "+name, identifier,
		"role", "ou")
	s.Require().NoError(err, "create the %s resource server", name)
	s.rsIDs = append(s.rsIDs, rsID)

	roleID, err := testutils.CreateRole(testutils.Role{
		Name: "Role Confinement Admin " + name,
		OUID: ouID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: rsID, Permissions: []string{"system:role", "system:ou:view"}},
		},
		Assignments: []testutils.Assignment{{ID: userID, Type: "user"}},
	})
	s.Require().NoError(err, "create the %s caller role", name)
	s.callerRoles = append(s.callerRoles, roleID)

	token, err := s.signIn.ObtainAccessToken(ouID, "system system:role system:ou:view", username,
		roleConfinementPassword, identifier)
	s.Require().NoError(err, "obtain the %s token", name)
	s.Require().Contains(strings.Fields(token.Scope), "system:role", "the %s token must carry the scope", name)
	return testutils.GetHTTPClientWithToken(token.AccessToken), userID
}

// send issues a request with the client and returns the status and body.
func (s *RoleOUConfinementTestSuite) send(client *http.Client, method, path string,
	body interface{}) (int, []byte) {
	s.T().Helper()

	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		s.Require().NoError(err)
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, testutils.TestServerURL+path, reader)
	s.Require().NoError(err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)
	return resp.StatusCode, respBody
}

// decodeList decodes the roles of a list response.
func (s *RoleOUConfinementTestSuite) decodeList(body []byte) []confinedRole {
	s.T().Helper()
	var list struct {
		Roles []confinedRole `json:"roles"`
	}
	s.Require().NoError(json.Unmarshal(body, &list), string(body))
	return list.Roles
}

// allRoleIDs lists every page of roles the client sees and returns their IDs.
func (s *RoleOUConfinementTestSuite) allRoleIDs(client *http.Client) []string {
	s.T().Helper()
	var ids []string
	for offset := 0; ; offset += 100 {
		status, body := s.send(client, http.MethodGet, fmt.Sprintf("%s?limit=100&offset=%d", rolesBasePath, offset),
			nil)
		s.Require().Equal(http.StatusOK, status, string(body))
		page := s.decodeList(body)
		ids = append(ids, roleIDs(page)...)
		if len(page) < 100 {
			return ids
		}
	}
}

// requireRefused asserts the response is the organization unit refusal.
func (s *RoleOUConfinementTestSuite) requireRefused(status int, body []byte) {
	s.T().Helper()
	s.Require().Equal(http.StatusForbidden, status, string(body))
	var errResp struct {
		Code string `json:"code"`
	}
	s.Require().NoError(json.Unmarshal(body, &errResp), string(body))
	s.Equal("SSE-4030", errResp.Code)
}

// roleIDs returns the IDs of the roles.
func roleIDs(roles []confinedRole) []string {
	ids := make([]string, 0, len(roles))
	for _, r := range roles {
		ids = append(ids, r.ID)
	}
	return ids
}

// indexOf returns the position of id in ids, or -1.
func indexOf(ids []string, id string) int {
	for i, candidate := range ids {
		if candidate == id {
			return i
		}
	}
	return -1
}
