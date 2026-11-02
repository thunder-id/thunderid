// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agenttype

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// AgentTypeAuthzTestSuite verifies that /agent-types is gated by the `agenttype` category
// permission, which had no integration coverage: entity-type authorization is unit-tested against
// mocks, and integration-tested only for /user-types.
//
// The caller here holds `system:usertype` and nothing else. Every /agent-types route refuses it at
// the security middleware with 403 AUTH-4030 (`system/security/middleware.go:51`), which is a
// route-level permission gate that runs before the handler — so the two categories are separated
// before any OU or resource logic is reached. The same token reads /user-types successfully
// throughout, which is what makes the refusals attributable to the missing category permission
// rather than to an invalid token.
type AgentTypeAuthzTestSuite struct {
	suite.Suite

	agentTypeID string

	ouID       string
	userTypeID string
	userID     string
	roleID     string
	scopedRSID string

	// scopedClient carries a token with system:usertype but no agenttype permission.
	scopedClient *http.Client
}

const (
	agentTypeAuthzOUHandle = "agent-type-authz-ou"

	agentTypeAuthzUserTypeName = "agent-type-authz-person"
	agentTypeAuthzUsername     = "agent-type-authz-user"
	agentTypeAuthzPassword     = "AgentTypeAuthz@123"

	agentTypeAuthzClientID    = "CONSOLE"
	agentTypeAuthzRedirectURI = "https://localhost:8095/console"
	agentTypeAuthzRSIdentity  = "https://authz-test.example.com/agenttype"
)

func TestAgentTypeAuthzTestSuite(t *testing.T) {
	suite.Run(t, new(AgentTypeAuthzTestSuite))
}

func (s *AgentTypeAuthzTestSuite) SetupSuite() {
	// Read-only: the suite needs the singleton's ID to address it, and never mutates it.
	snapshot, err := testutils.SnapshotAgentType()
	s.Require().NoError(err, "the default agent type must exist before this suite runs")
	s.agentTypeID = snapshot.ID

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      agentTypeAuthzOUHandle,
		Name:        "Agent Type Authz OU",
		Description: "OU holding the scoped user for the agent type authz test",
	})
	s.Require().NoError(err)
	s.ouID = ouID

	userTypeID, err := testutils.CreateUserType(testutils.UserType{
		Handle:      agentTypeAuthzUserTypeName,
		DisplayName: "Agent Type Authz Person",
		OUID:        s.ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string", "unique": true},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	})
	s.Require().NoError(err)
	s.userTypeID = userTypeID

	userID, err := testutils.CreateUser(testutils.User{
		Type: agentTypeAuthzUserTypeName,
		OUID: s.ouID,
		Attributes: json.RawMessage(fmt.Sprintf(
			`{"username": %q, "password": %q}`, agentTypeAuthzUsername, agentTypeAuthzPassword)),
	})
	s.Require().NoError(err)
	s.userID = userID

	// Declare both category permissions so the tree is complete, then grant only `usertype`.
	// Granting nothing at all would not distinguish "no agenttype permission" from "no permissions".
	rsID, err := testutils.CreateSystemScopedResourceServer(
		s.ouID, "Authz Test RS (agenttype)", agentTypeAuthzRSIdentity, "usertype", "agenttype")
	s.Require().NoError(err)
	s.scopedRSID = rsID

	roleID, err := testutils.CreateRole(testutils.Role{
		Name: "Agent Type Authz Role",
		OUID: s.ouID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: rsID, Permissions: []string{"system:usertype"}},
		},
		Assignments: []testutils.Assignment{{ID: s.userID, Type: "user"}},
	})
	s.Require().NoError(err)
	s.roleID = roleID

	tokenResp, err := testutils.ObtainAccessTokenWithPassword(
		agentTypeAuthzClientID,
		agentTypeAuthzRedirectURI,
		"system system:usertype",
		agentTypeAuthzUsername,
		agentTypeAuthzPassword,
		true,
		"",
		agentTypeAuthzRSIdentity,
	)
	s.Require().NoError(err)
	s.Require().NotEmpty(tokenResp.AccessToken)

	granted := strings.Fields(tokenResp.Scope)
	s.Require().Contains(granted, "system:usertype", "token must carry the usertype scope")
	for _, scope := range granted {
		s.Require().False(strings.HasPrefix(scope, "system:agenttype"),
			"the token must carry no agenttype scope, or the suite proves nothing; got %v", granted)
	}

	s.scopedClient = testutils.GetHTTPClientWithToken(tokenResp.AccessToken)
}

func (s *AgentTypeAuthzTestSuite) TearDownSuite() {
	// Cleanup failures are asserted, not swallowed. This suite builds a resource server with a
	// nested permission tree, and a discarded error here would leave that tree in the shared
	// database while the suite still reported PASS.
	if s.roleID != "" {
		s.NoError(testutils.DeleteRole(s.roleID), "teardown: delete role")
	}
	if s.scopedRSID != "" {
		// A plain resource-server delete is refused with RES-1006 while it still owns resources.
		s.NoError(testutils.DeleteResourceServerWithChildren(s.scopedRSID),
			"teardown: delete scoped resource server and its resource tree")
	}
	if s.userID != "" {
		s.NoError(testutils.DeleteUser(s.userID), "teardown: delete scoped user")
	}
	if s.userTypeID != "" {
		s.NoError(testutils.DeleteUserType(s.userTypeID), "teardown: delete user type")
	}
	if s.ouID != "" {
		s.NoError(testutils.DeleteOrganizationUnit(s.ouID), "teardown: delete OU")
	}
}

// doScoped issues a request as the scoped user.
func (s *AgentTypeAuthzTestSuite) doScoped(method, path string, body interface{}) *http.Response {
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

	resp, err := s.scopedClient.Do(req)
	s.Require().NoError(err)
	return resp
}

// requireForbidden asserts the response is the middleware's permission refusal.
func (s *AgentTypeAuthzTestSuite) requireForbidden(resp *http.Response) {
	s.T().Helper()

	s.Require().Equal(http.StatusForbidden, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)

	var errResp ErrorResponse
	s.Require().NoError(json.Unmarshal(body, &errResp), "error body: %s", string(body))
	s.Equal("AUTH-4030", errResp.Code)
	s.Equal("Forbidden", errResp.Message.DefaultValue)
}

// ---------------------------------------------------------------------------
// Scenario N1 — the agenttype category permission gates /agent-types
// ---------------------------------------------------------------------------

// TestReadAgentTypeWithoutCategoryPermissionIsRefused verifies that holding the `usertype`
// permission does not carry over to the `agenttype` category on the detail route.
func (s *AgentTypeAuthzTestSuite) TestReadAgentTypeWithoutCategoryPermissionIsRefused() {
	resp := s.doScoped(http.MethodGet, agentTypeBasePath+"/"+s.agentTypeID, nil)
	defer closeBody(resp)

	s.requireForbidden(resp)
}

// TestListAgentTypesWithoutCategoryPermissionIsRefused verifies the list route is gated too, so a
// caller without the permission cannot enumerate agent types.
func (s *AgentTypeAuthzTestSuite) TestListAgentTypesWithoutCategoryPermissionIsRefused() {
	resp := s.doScoped(http.MethodGet, agentTypeBasePath, nil)
	defer closeBody(resp)

	s.requireForbidden(resp)
}

// TestUpdateAgentTypeWithoutCategoryPermissionIsRefused verifies the write path is gated, and that
// the refused write leaves the stored schema untouched.
func (s *AgentTypeAuthzTestSuite) TestUpdateAgentTypeWithoutCategoryPermissionIsRefused() {
	resp := s.doScoped(http.MethodPut, agentTypeBasePath+"/"+s.agentTypeID, AgentTypeRequest{
		Handle:      defaultAgentTypeName,
		DisplayName: "Default",
		OUID:        s.ouID,
		Schema:      json.RawMessage(`{"injected": {"type": "string"}}`),
	})
	defer closeBody(resp)

	s.requireForbidden(resp)

	var schema map[string]interface{}
	s.Require().NoError(json.Unmarshal(s.adminGetAgentType().Schema, &schema))
	s.NotContains(schema, "injected", "a refused update must not reach the store")
}

// TestCreateAgentTypeWithoutCategoryPermissionIsRefused verifies the create route is gated, and
// that the refusal is the permission gate rather than the `default`-only rule — the payload uses a
// non-`default` handle, which an authorized caller would be told about via USRS-1014 instead.
func (s *AgentTypeAuthzTestSuite) TestCreateAgentTypeWithoutCategoryPermissionIsRefused() {
	resp := s.doScoped(http.MethodPost, agentTypeBasePath, AgentTypeRequest{
		Handle:      "authz-injected-agent-type",
		DisplayName: "Authz Injected Agent Type",
		OUID:        s.ouID,
		Schema:      json.RawMessage(`{"description": {"type": "string"}}`),
	})
	defer closeBody(resp)

	s.requireForbidden(resp)
}

// TestDeleteAgentTypeWithoutCategoryPermissionIsRefused verifies the delete route is gated by the
// permission as well, and that the refusal is the permission gate rather than the never-delete rule.
// DELETE carries its own entry in the permission table, so this is a distinct middleware rule from
// the routes above. An authorized caller receives USRS-1015 for the same request, so the code proves
// which check ran first.
func (s *AgentTypeAuthzTestSuite) TestDeleteAgentTypeWithoutCategoryPermissionIsRefused() {
	resp := s.doScoped(http.MethodDelete, agentTypeBasePath+"/"+s.agentTypeID, nil)
	defer closeBody(resp)

	s.requireForbidden(resp)

	s.Equal(defaultAgentTypeName, s.adminGetAgentType().Handle,
		"the agent type must still exist after a refused delete")
}

// TestListUserTypesRemainsPermitted is the control for the refusals above: the same token reads
// its own category successfully, so those refusals are about the missing agenttype permission and
// not about a broken or unscoped token.
func (s *AgentTypeAuthzTestSuite) TestListUserTypesRemainsPermitted() {
	resp := s.doScoped(http.MethodGet, "/user-types", nil)
	defer closeBody(resp)

	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var list AgentTypeListResponse
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&list))

	ids := make([]string, 0, len(list.Types))
	for _, t := range list.Types {
		ids = append(ids, t.ID)
	}
	s.Contains(ids, s.userTypeID, "the scoped user must still read its own category, got: %v", ids)
}

// adminGetAgentType reads the agent type with the unrestricted client.
func (s *AgentTypeAuthzTestSuite) adminGetAgentType() AgentType {
	s.T().Helper()

	req, err := http.NewRequest(http.MethodGet,
		testutils.TestServerURL+agentTypeBasePath+"/"+s.agentTypeID, nil)
	s.Require().NoError(err)

	resp, err := testutils.GetHTTPClient().Do(req)
	s.Require().NoError(err)
	defer closeBody(resp)
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var agentType AgentType
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&agentType))
	return agentType
}
