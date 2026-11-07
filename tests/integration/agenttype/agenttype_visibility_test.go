// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agenttype

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// AgentTypeVisibilityTestSuite verifies that every organization unit reads the deployment's agent
// type, while writes stay confined to the organization unit that owns it.
//
// The deployment has a single agent type, `default`, owned by the organization unit the bootstrap
// places it in, and the agents of every organization unit take it. Two callers hold
// `system:agenttype`:
//
//	<agent type owner>
//	└── child  ← reads the agent type, cannot update it
//	other root ← in another tree, reads the agent type too
type AgentTypeVisibilityTestSuite struct {
	suite.Suite

	signIn testutils.OUSignInApps

	agentType *testutils.AgentTypeSnapshot

	childOUID string
	otherOUID string

	userTypeIDs []string
	userIDs     []string
	roleIDs     []string
	rsIDs       []string

	childClient *http.Client
	otherClient *http.Client
}

const agentTypeVisibilityPassword = "AgentTypeVisibility@123"

func TestAgentTypeVisibilityTestSuite(t *testing.T) {
	suite.Run(t, new(AgentTypeVisibilityTestSuite))
}

func (s *AgentTypeVisibilityTestSuite) SetupSuite() {
	snapshot, err := testutils.SnapshotAgentType()
	s.Require().NoError(err, "the default agent type must exist before this suite runs")
	s.agentType = snapshot

	childOUID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "agent-type-visibility-child",
		Name:   "Agent Type Visibility Child",
		Parent: &snapshot.OUID,
	})
	s.Require().NoError(err, "create the child of the agent type's organization unit")
	s.childOUID = childOUID

	otherOUID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: "agent-type-visibility-other",
		Name:   "Agent Type Visibility Other",
	})
	s.Require().NoError(err, "create the organization unit in another tree")
	s.otherOUID = otherOUID

	s.childClient = s.scopedClient(s.childOUID, "child")
	s.otherClient = s.scopedClient(s.otherOUID, "other")
}

func (s *AgentTypeVisibilityTestSuite) TearDownSuite() {
	s.NoError(s.signIn.DeleteAll(), "teardown: delete sign-in applications")
	for _, id := range s.roleIDs {
		s.NoError(testutils.DeleteRole(id), "teardown: delete role")
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
	for _, id := range []string{s.childOUID, s.otherOUID} {
		if id != "" {
			s.NoError(testutils.DeleteOrganizationUnit(id), "teardown: delete organization unit")
		}
	}
}

func (s *AgentTypeVisibilityTestSuite) TestEveryOUReadsTheAgentType() {
	for name, client := range map[string]*http.Client{"child": s.childClient, "other tree": s.otherClient} {
		resp := s.send(client, http.MethodGet, agentTypeBasePath+"/"+s.agentType.ID, nil)
		s.Require().Equal(http.StatusOK, resp.StatusCode, name)
		var agentType AgentType
		s.Require().NoError(json.NewDecoder(resp.Body).Decode(&agentType))
		closeBody(resp)
		s.Equal(s.agentType.ID, agentType.ID, name)
	}
}

func (s *AgentTypeVisibilityTestSuite) TestEveryOUListsTheAgentType() {
	s.Contains(s.listIDs(s.childClient), s.agentType.ID)
	s.Contains(s.listIDs(s.otherClient), s.agentType.ID)
}

func (s *AgentTypeVisibilityTestSuite) TestOtherOUCannotUpdateTheAgentType() {
	resp := s.send(s.childClient, http.MethodPut, agentTypeBasePath+"/"+s.agentType.ID, AgentTypeRequest{
		Handle:      defaultAgentTypeName,
		DisplayName: "Renamed by a child",
		OUID:        s.agentType.OUID,
		Schema:      json.RawMessage(`{"name": {"type": "string"}}`),
	})
	defer closeBody(resp)

	s.requireRefused(resp)
	current, err := testutils.SnapshotAgentType()
	s.Require().NoError(err)
	s.Equal(s.agentType.DisplayName, current.DisplayName, "a refused update must not reach the store")
}

// scopedClient provisions a user in the organization unit holding `system:agenttype` and returns a
// client carrying the user's token.
func (s *AgentTypeVisibilityTestSuite) scopedClient(ouID, name string) *http.Client {
	s.T().Helper()

	userTypeHandle := "agent-type-visibility-" + name
	userTypeID, err := testutils.CreateUserType(testutils.UserType{
		Handle:      userTypeHandle,
		DisplayName: "Agent Type Visibility " + name,
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
			`{"username": %q, "password": %q}`, username, agentTypeVisibilityPassword)),
	})
	s.Require().NoError(err, "create the %s user", name)
	s.userIDs = append(s.userIDs, userID)

	identifier := "https://authz-test.example.com/agenttype-visibility-" + name
	rsID, err := testutils.CreateSystemScopedResourceServer(
		ouID, "Agent Type Visibility RS "+name, identifier, "agenttype")
	s.Require().NoError(err, "create the %s resource server", name)
	s.rsIDs = append(s.rsIDs, rsID)

	roleID, err := testutils.CreateRole(testutils.Role{
		Name: "Agent Type Visibility " + name,
		OUID: ouID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: rsID, Permissions: []string{"system:agenttype"}},
		},
		Assignments: []testutils.Assignment{{ID: userID, Type: "user"}},
	})
	s.Require().NoError(err, "create the %s role", name)
	s.roleIDs = append(s.roleIDs, roleID)

	token, err := s.signIn.ObtainAccessToken(ouID, "system system:agenttype", username,
		agentTypeVisibilityPassword, identifier)
	s.Require().NoError(err, "obtain the %s token", name)
	s.Require().Contains(strings.Fields(token.Scope), "system:agenttype", "the %s token must carry the scope", name)
	return testutils.GetHTTPClientWithToken(token.AccessToken)
}

// send issues a request with the client and returns the response.
func (s *AgentTypeVisibilityTestSuite) send(client *http.Client, method, path string,
	body interface{}) *http.Response {
	s.T().Helper()

	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		s.Require().NoError(err)
		reader = strings.NewReader(string(encoded))
	}
	req, err := http.NewRequest(method, testutils.TestServerURL+path, reader)
	s.Require().NoError(err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	s.Require().NoError(err)
	return resp
}

// listIDs lists the agent types the client sees and returns their IDs.
func (s *AgentTypeVisibilityTestSuite) listIDs(client *http.Client) []string {
	s.T().Helper()

	resp := s.send(client, http.MethodGet, agentTypeBasePath, nil)
	defer closeBody(resp)
	s.Require().Equal(http.StatusOK, resp.StatusCode)

	var list AgentTypeListResponse
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&list))
	ids := make([]string, 0, len(list.Types))
	for _, item := range list.Types {
		ids = append(ids, item.ID)
	}
	return ids
}

// requireRefused asserts the response is the service's organization unit refusal, which the caller's
// permission passed the security middleware to reach.
func (s *AgentTypeVisibilityTestSuite) requireRefused(resp *http.Response) {
	s.T().Helper()

	s.Require().Equal(http.StatusForbidden, resp.StatusCode)
	var errResp ErrorResponse
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&errResp))
	s.Equal("SSE-4030", errResp.Code)
}
