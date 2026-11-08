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

const (
	scopedAdminUserTypeName = "agent-type-scoped-admin-person"
	scopedAdminUsername     = "agent-type-scoped-admin"
	scopedAdminPassword     = "AgentTypeScoped@123"
	scopedAdminRSIdentity   = "https://scoped-admin-test.example.com/agenttype"
	// scopedAdminClientID is the sign-in application of the administrator's unit: a sign-in finds only the
	// users of the application's organization unit.
	scopedAdminClientID = "agent_type_scoped_admin_sign_in"

	scopedParentAgentType  = "scoped-parent-agents"
	scopedChildAgentType   = "scoped-child-agents"
	scopedSiblingAgentType = "scoped-sibling-agents"
)

// AgentTypeScopedAdminTestSuite covers what an administrator scoped to one organization unit sees of agent
// types, and which ones it can create agents of. The administrator belongs to a child unit and holds the
// agent permission and the agent type view permission. It reads the agent types of its own unit and of
// its ancestors, and no others. An agent type it cannot read is answered the way an unknown one is, so
// creating an agent reveals nothing about agent types outside its reach.
type AgentTypeScopedAdminTestSuite struct {
	suite.Suite

	parentOUID  string
	childOUID   string
	siblingOUID string
	typeIDs     []string
	userTypeID  string
	userID      string
	roleID      string
	scopedRSID  string
	signInApp   string
	agentIDs    []string

	scopedClient *http.Client
}

func TestAgentTypeScopedAdminTestSuite(t *testing.T) {
	suite.Run(t, new(AgentTypeScopedAdminTestSuite))
}

func (s *AgentTypeScopedAdminTestSuite) SetupSuite() {
	s.parentOUID = s.createOU("agent-type-scoped-parent-ou", nil)
	s.childOUID = s.createOU("agent-type-scoped-child-ou", &s.parentOUID)
	s.siblingOUID = s.createOU("agent-type-scoped-sibling-ou", nil)

	for handle, ouID := range map[string]string{
		scopedParentAgentType: s.parentOUID, scopedChildAgentType: s.childOUID, scopedSiblingAgentType: s.siblingOUID,
	} {
		id, err := testutils.CreateNewAgentType(testutils.UserType{
			Handle: handle, DisplayName: handle, OUID: ouID,
			Schema: map[string]interface{}{"name": map[string]interface{}{"type": "string", "required": true}},
		})
		s.Require().NoError(err)
		s.typeIDs = append(s.typeIDs, id)
	}

	userTypeID, err := testutils.CreateUserType(testutils.UserType{
		Handle:      scopedAdminUserTypeName,
		DisplayName: "Agent Type Scoped Admin Person",
		OUID:        s.childOUID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string", "unique": true},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	})
	s.Require().NoError(err)
	s.userTypeID = userTypeID

	userID, err := testutils.CreateUser(testutils.User{
		Type: scopedAdminUserTypeName,
		OUID: s.childOUID,
		Attributes: json.RawMessage(fmt.Sprintf(
			`{"username": %q, "password": %q}`, scopedAdminUsername, scopedAdminPassword)),
	})
	s.Require().NoError(err)
	s.userID = userID

	rsID, err := testutils.CreateOrgScopedResourceServer(
		s.childOUID, "Scoped Admin Test RS (agenttype)", scopedAdminRSIdentity, "agent", "agenttype")
	s.Require().NoError(err)
	s.scopedRSID = rsID

	roleID, err := testutils.CreateRole(testutils.Role{
		Name: "Agent Type Scoped Admin Role",
		OUID: s.childOUID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: rsID, Permissions: []string{"org:agent", "org:agenttype:view"}},
		},
		Assignments: []testutils.Assignment{{ID: s.userID, Type: "user"}},
	})
	s.Require().NoError(err)
	s.roleID = roleID

	s.signInApp, err = testutils.CreateOUSignInApplication(s.childOUID, scopedAdminClientID)
	s.Require().NoError(err)

	tokenResp, err := testutils.ObtainAccessTokenWithPassword(
		scopedAdminClientID, testutils.OUSignInRedirectURI, "org:agent org:agenttype:view",
		scopedAdminUsername, scopedAdminPassword, true, "", scopedAdminRSIdentity)
	s.Require().NoError(err)
	granted := strings.Fields(tokenResp.Scope)
	s.Require().Contains(granted, "org:agenttype:view", "token must carry the agent type view scope")
	s.Require().Contains(granted, "org:agent", "token must carry the agent scope")
	s.scopedClient = testutils.GetHTTPClientWithToken(tokenResp.AccessToken)
}

func (s *AgentTypeScopedAdminTestSuite) TearDownSuite() {
	for _, id := range s.agentIDs {
		s.NoError(testutils.DeleteAgent(id), "teardown: delete agent")
	}
	if s.signInApp != "" {
		s.NoError(testutils.DeleteApplication(s.signInApp), "teardown: delete sign-in application")
	}
	if s.roleID != "" {
		s.NoError(testutils.DeleteRole(s.roleID), "teardown: delete role")
	}
	if s.scopedRSID != "" {
		s.NoError(testutils.DeleteResourceServerWithChildren(s.scopedRSID), "teardown: delete resource server")
	}
	if s.userID != "" {
		s.NoError(testutils.DeleteUser(s.userID), "teardown: delete user")
	}
	if s.userTypeID != "" {
		s.NoError(testutils.DeleteUserType(s.userTypeID), "teardown: delete user type")
	}
	for _, id := range s.typeIDs {
		s.NoError(testutils.DeleteAgentType(id), "teardown: delete agent type")
	}
	for _, id := range []string{s.childOUID, s.parentOUID, s.siblingOUID} {
		if id != "" {
			s.NoError(testutils.DeleteOrganizationUnit(id), "teardown: delete OU")
		}
	}
}

// createOU creates an organization unit for the suite.
func (s *AgentTypeScopedAdminTestSuite) createOU(handle string, parent *string) string {
	s.T().Helper()
	id, err := testutils.CreateOrganizationUnit(
		testutils.OrganizationUnit{Handle: handle, Name: handle, Parent: parent})
	s.Require().NoError(err)
	return id
}

// doScoped issues a request as the scoped administrator and returns the status and body.
func (s *AgentTypeScopedAdminTestSuite) doScoped(method, path string, body interface{}) (int, []byte) {
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
	defer closeBody(resp)
	respBody, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)
	return resp.StatusCode, respBody
}

// createAgentAs posts an agent of the given type in the administrator's unit.
func (s *AgentTypeScopedAdminTestSuite) createAgentAs(agentType, name string) (int, []byte) {
	s.T().Helper()
	return s.doScoped(http.MethodPost, agentBasePath, map[string]interface{}{
		"type": agentType, "ouId": s.childOUID, "attributes": map[string]interface{}{"name": name},
	})
}

func (s *AgentTypeScopedAdminTestSuite) TestListShowsOwnAndAncestorAgentTypes() {
	status, body := s.doScoped(http.MethodGet, agentTypeBasePath+"?limit=100", nil)
	s.Require().Equal(http.StatusOK, status, "body: %s", string(body))

	var list AgentTypeListResponse
	s.Require().NoError(json.Unmarshal(body, &list))
	handles := make([]string, 0, len(list.Types))
	for _, t := range list.Types {
		handles = append(handles, t.Handle)
	}
	s.Contains(handles, scopedChildAgentType, "the administrator's own unit is listed")
	s.Contains(handles, scopedParentAgentType, "an ancestor unit is listed")
	s.NotContains(handles, scopedSiblingAgentType, "a unit outside the administrator's line is not listed")
	s.NotContains(handles, defaultAgentTypeName, "the bootstrap unit is not an ancestor, so it is not listed")
}

func (s *AgentTypeScopedAdminTestSuite) TestCreateAgentOfAnAncestorAgentType() {
	status, body := s.createAgentAs(scopedParentAgentType, "scoped-admin-ancestor-type-agent")
	s.Require().Equal(http.StatusCreated, status, "body: %s", string(body))

	var created struct {
		ID string `json:"id"`
	}
	s.Require().NoError(json.Unmarshal(body, &created))
	s.agentIDs = append(s.agentIDs, created.ID)
}

func (s *AgentTypeScopedAdminTestSuite) TestUnreadableAgentTypeAnswersAsAnUnknownOne() {
	unreadableStatus, unreadableBody := s.createAgentAs(scopedSiblingAgentType, "scoped-admin-sibling-type-agent")
	unknownStatus, unknownBody := s.createAgentAs("no-such-agents", "scoped-admin-unknown-type-agent")

	s.Equal(http.StatusBadRequest, unreadableStatus, "body: %s", string(unreadableBody))
	s.Equal(unknownStatus, unreadableStatus)
	s.JSONEq(string(unknownBody), string(unreadableBody),
		"an agent type out of reach must be indistinguishable from one that does not exist")
	var refusal struct {
		Code string `json:"code"`
	}
	s.Require().NoError(json.Unmarshal(unknownBody, &refusal))
	s.Equal("AGT-1054", refusal.Code, "an unknown agent type is reported as not found: %s", string(unknownBody))
}
