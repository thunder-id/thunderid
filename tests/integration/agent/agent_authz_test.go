// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

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

// AgentAuthzTestSuite validates that agent CRUD operations respect OU-scoped authz.
//
// Permission model:
//
//	system:agent      → create, update, delete agents
//	system:agent:view → list, get agents and their groups and roles (implied by system:agent)
//
// An agent-manager living in OU1 holds the system:agent permission. The suite verifies that:
//
//   - Read operations on agents in OU1 are allowed (200)
//   - Read operations on agents in OU2 (sibling) are denied (403)
//   - Write operations on agents in OU1 are allowed (201/200/204)
//   - Write operations on agents in OU2 are denied (403)
//   - Moving an agent from OU1 into OU2 is denied, even though the caller may write to OU1
//   - Listing agents only returns agents from the accessible OU
//
// Fixture topology:
//
//	OU1 (handle: authz-agent-ou1) ← agent-manager and target agents belong here
//	OU2 (handle: authz-agent-ou2) ← sibling OU with its own target agent
type AgentAuthzTestSuite struct {
	suite.Suite

	// Admin-created OUs
	agentOU1ID string
	agentOU2ID string

	// User type for the manager. The `default` agent type is a singleton shared with every other
	// suite, so it is snapshotted and restored rather than created per OU.
	userTypeOU1ID     string
	agentTypeSnapshot *testutils.AgentTypeSnapshot

	// Test role, manager and target agents
	agentMgrRoleID      string
	agentMgrUserID      string
	scopedRSID          string
	targetAgentOU1ID    string
	deletableAgentOU1ID string
	targetAgentOU2ID    string

	// HTTP client carrying the agent-manager's system:agent scoped token
	agentAdminClient *http.Client
}

const (
	agentAuthzServerURL = "https://localhost:8095"

	agentAuthzOU1Handle = "authz-agent-ou1"
	agentAuthzOU2Handle = "authz-agent-ou2"

	agentMgrUsername   = "authz-agent-manager"
	agentMgrPassword   = "AgentMgr@123"
	agentMgrRoleName   = "Agent Admin (agent-authz-test)"
	agentMgrUserType   = "authz-agent-mgr-type"
	scopedRSIdentifier = "https://authz-test.example.com/agent"

	// The server restricts agent types to a single `default` schema and refuses deletion.
	agentTypeName = "default"

	agentAuthzClientID = "CONSOLE"
)

func TestAgentAuthzTestSuite(t *testing.T) {
	suite.Run(t, new(AgentAuthzTestSuite))
}

// ---------------------------------------------------------------------------
// Suite setup
// ---------------------------------------------------------------------------

func (ts *AgentAuthzTestSuite) SetupSuite() {
	// ---- 1. Create the two OUs ----
	ou1ID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      agentAuthzOU1Handle,
		Name:        "Agent Authz Test OU1",
		Description: "Primary OU for agent authz integration test",
	})
	ts.Require().NoError(err, "create agent-authz OU1")
	ts.agentOU1ID = ou1ID

	ou2ID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      agentAuthzOU2Handle,
		Name:        "Agent Authz Test OU2",
		Description: "Sibling OU for agent authz integration test",
	})
	ts.Require().NoError(err, "create agent-authz OU2")
	ts.agentOU2ID = ou2ID

	// ---- 2. Create a user type in OU1 for the agent-manager ----
	userTypeID, err := testutils.CreateUserType(testutils.UserType{
		Name: agentMgrUserType,
		OUID: ts.agentOU1ID,
		Schema: map[string]interface{}{
			"username":     map[string]interface{}{"type": "string"},
			"password":     map[string]interface{}{"type": "string", "credential": true},
			"display_name": map[string]interface{}{"type": "string"},
		},
	})
	ts.Require().NoError(err, "create user type for agent-manager")
	ts.userTypeOU1ID = userTypeID

	// ---- 3. Point the shared `default` agent type at OU1 ----
	// The singleton is shared with every other suite, so snapshot it before repointing and restore
	// it in teardown. Entity creation validates attributes against the schema without requiring the
	// type's OU to match the entity's, so agents in both OUs can use this one type.
	snapshot, err := testutils.SnapshotAgentType()
	ts.Require().NoError(err, "snapshot the default agent type")
	ts.agentTypeSnapshot = snapshot

	_, err = testutils.CreateAgentType(testutils.UserType{
		OUID: ts.agentOU1ID,
		Schema: map[string]interface{}{
			"purpose": map[string]interface{}{"type": "string"},
		},
	})
	ts.Require().NoError(err, "point the default agent type at OU1")

	// ---- 4. Create the agent-manager in OU1 (needs username+password for the token grant) ----
	agentMgrID, err := testutils.CreateUser(testutils.User{
		Type: agentMgrUserType,
		OUID: ts.agentOU1ID,
		Attributes: json.RawMessage(fmt.Sprintf(
			`{"username": %q, "password": %q, "display_name": "Agent Manager"}`,
			agentMgrUsername, agentMgrPassword,
		)),
	})
	ts.Require().NoError(err, "create agent-manager user")
	ts.agentMgrUserID = agentMgrID

	// ---- 5. Create target agents ----
	targetOU1ID, err := testutils.CreateAgent(testutils.Agent{
		Name:       "authz-target-agent-ou1",
		Type:       agentTypeName,
		OUID:       ts.agentOU1ID,
		Attributes: map[string]interface{}{"purpose": "target in OU1"},
	})
	ts.Require().NoError(err, "create target agent in OU1")
	ts.targetAgentOU1ID = targetOU1ID

	deletableID, err := testutils.CreateAgent(testutils.Agent{
		Name:       "authz-deletable-agent-ou1",
		Type:       agentTypeName,
		OUID:       ts.agentOU1ID,
		Attributes: map[string]interface{}{"purpose": "deletable in OU1"},
	})
	ts.Require().NoError(err, "create deletable agent in OU1")
	ts.deletableAgentOU1ID = deletableID

	targetOU2ID, err := testutils.CreateAgent(testutils.Agent{
		Name:       "authz-target-agent-ou2",
		Type:       agentTypeName,
		OUID:       ts.agentOU2ID,
		Attributes: map[string]interface{}{"purpose": "target in OU2"},
	})
	ts.Require().NoError(err, "create target agent in OU2")
	ts.targetAgentOU2ID = targetOU2ID

	// ---- 6. Create a custom resource server declaring the fine-grained system scopes ----
	// The product ships only the root "system" scope; this reproduces "system:agent" and
	// "system:agenttype:view" so the suite can verify resource-level enforcement when configured.
	systemRSID, err := testutils.CreateSystemScopedResourceServer(
		ts.agentOU1ID, "Authz Test RS (agent)", scopedRSIdentifier, "agent", "agenttype")
	ts.Require().NoError(err, "create scoped resource server")
	ts.scopedRSID = systemRSID

	// ---- 7. Create a role with system:agent permission and assign to the agent-manager ----
	roleID, err := testutils.CreateRole(testutils.Role{
		Name: agentMgrRoleName,
		OUID: ts.agentOU1ID,
		Permissions: []testutils.ResourcePermissions{
			{
				ResourceServerID: systemRSID,
				Permissions:      []string{"system:agent", "system:agenttype:view"},
			},
		},
		Assignments: []testutils.Assignment{
			{ID: ts.agentMgrUserID, Type: "user"},
		},
	})
	ts.Require().NoError(err, "create agent-manager role")
	ts.agentMgrRoleID = roleID

	// ---- 8. Obtain a scoped access token for the agent-manager ----
	tokenResp, err := testutils.ObtainAccessTokenWithPassword(
		agentAuthzClientID,
		agentAuthzServerURL+"/console",
		"system system:agent system:agenttype:view",
		agentMgrUsername,
		agentMgrPassword,
		true,
		"",
		scopedRSIdentifier,
	)
	ts.Require().NoError(err, "obtain agent-manager token")
	ts.Require().NotEmpty(tokenResp.AccessToken, "agent-manager token must be non-empty")

	ts.agentAdminClient = testutils.GetHTTPClientWithToken(tokenResp.AccessToken)
}

// ---------------------------------------------------------------------------
// Suite teardown
// ---------------------------------------------------------------------------

func (ts *AgentAuthzTestSuite) TearDownSuite() {
	if ts.agentMgrRoleID != "" {
		if err := testutils.DeleteRole(ts.agentMgrRoleID); err != nil {
			ts.T().Logf("teardown: delete agent-manager role: %v", err)
		}
	}
	if ts.scopedRSID != "" {
		// The scoped resource server owns a nested resource tree, and a plain delete is refused with
		// RES-1006 while those resources exist.
		if err := testutils.DeleteResourceServerWithChildren(ts.scopedRSID); err != nil {
			ts.T().Errorf("teardown: delete scoped resource server: %v", err)
		}
	}
	for _, id := range []string{ts.targetAgentOU1ID, ts.deletableAgentOU1ID, ts.targetAgentOU2ID} {
		if id != "" {
			if err := testutils.DeleteAgent(id); err != nil {
				ts.T().Logf("teardown: delete agent %s: %v", id, err)
			}
		}
	}
	if ts.agentMgrUserID != "" {
		if err := testutils.DeleteUser(ts.agentMgrUserID); err != nil {
			ts.T().Logf("teardown: delete agent-manager user: %v", err)
		}
	}
	// Restore the shared agent type before deleting the OU it points at, or the singleton is left
	// referencing a deleted OU and a later suite's restore fails.
	if ts.agentTypeSnapshot != nil {
		if err := testutils.RestoreAgentType(ts.agentTypeSnapshot); err != nil {
			ts.T().Errorf("teardown: restore the default agent type: %v", err)
		}
	}
	if ts.userTypeOU1ID != "" {
		if err := testutils.DeleteUserType(ts.userTypeOU1ID); err != nil {
			ts.T().Logf("teardown: delete agent-manager user type: %v", err)
		}
	}
	if ts.agentOU2ID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.agentOU2ID); err != nil {
			ts.T().Logf("teardown: delete agent-authz OU2: %v", err)
		}
	}
	if ts.agentOU1ID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.agentOU1ID); err != nil {
			ts.T().Logf("teardown: delete agent-authz OU1: %v", err)
		}
	}
}

// ---------------------------------------------------------------------------
// Helper — issue a request via the agent-manager's scoped client
// ---------------------------------------------------------------------------

func (ts *AgentAuthzTestSuite) doAgent(method, path string, body []byte) *http.Response {
	ts.T().Helper()

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, agentAuthzServerURL+path, bodyReader)
	ts.Require().NoError(err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := ts.agentAdminClient.Do(req)
	ts.Require().NoError(err)
	return resp
}

// ---------------------------------------------------------------------------
// Tests — READ operations (system:agent:view implied by system:agent)
// ---------------------------------------------------------------------------

// TestListAgents verifies the list contains agents from the accessible OU only.
func (ts *AgentAuthzTestSuite) TestListAgents() {
	resp := ts.doAgent(http.MethodGet, "/agents", nil)
	defer resp.Body.Close()

	ts.Equal(http.StatusOK, resp.StatusCode, "list agents should succeed")

	var listResp testutils.AgentListResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&listResp))

	ids := make([]string, 0, len(listResp.Agents))
	for _, a := range listResp.Agents {
		ids = append(ids, a.ID)
	}

	ts.Containsf(ids, ts.targetAgentOU1ID,
		"list must include target agent in OU1, got IDs: %v", ids)
	ts.NotContainsf(ids, ts.targetAgentOU2ID,
		"list must NOT include target agent in OU2 (sibling), got IDs: %v", ids)
}

// TestGetAgentInOwnOU verifies the agent-manager can read an agent in their own OU.
func (ts *AgentAuthzTestSuite) TestGetAgentInOwnOU() {
	resp := ts.doAgent(http.MethodGet, "/agents/"+ts.targetAgentOU1ID, nil)
	defer resp.Body.Close()

	ts.Equal(http.StatusOK, resp.StatusCode,
		"agent-manager should be able to read an agent in their own OU")
}

// TestGetAgentInOtherOU verifies the agent-manager is denied reading an agent in OU2.
func (ts *AgentAuthzTestSuite) TestGetAgentInOtherOU() {
	resp := ts.doAgent(http.MethodGet, "/agents/"+ts.targetAgentOU2ID, nil)
	defer resp.Body.Close()

	ts.Equal(http.StatusForbidden, resp.StatusCode,
		"agent-manager must be denied access to an agent in a different OU")
}

// TestGetAgentGroupsInOtherOU verifies the sub-resource read is scoped too.
func (ts *AgentAuthzTestSuite) TestGetAgentGroupsInOtherOU() {
	resp := ts.doAgent(http.MethodGet, "/agents/"+ts.targetAgentOU2ID+"/groups", nil)
	defer resp.Body.Close()

	ts.Equal(http.StatusForbidden, resp.StatusCode,
		"agent-manager must not read groups of an agent in a different OU")
}

// TestGetAgentRolesInOtherOU verifies the role listing is scoped as well. Roles are what an agent
// is authorized by, so a cross-OU read here would disclose the other OU's privilege assignments.
func (ts *AgentAuthzTestSuite) TestGetAgentRolesInOtherOU() {
	resp := ts.doAgent(http.MethodGet, "/agents/"+ts.targetAgentOU2ID+"/roles", nil)
	defer resp.Body.Close()

	ts.Equal(http.StatusForbidden, resp.StatusCode,
		"agent-manager must not read roles of an agent in a different OU")
}

// ---------------------------------------------------------------------------
// Tests — WRITE operations (system:agent)
// ---------------------------------------------------------------------------

// TestCreateAgentInOwnOU verifies the agent-manager can create an agent in their own OU.
func (ts *AgentAuthzTestSuite) TestCreateAgentInOwnOU() {
	payload, err := json.Marshal(map[string]interface{}{
		"name":       "authz-created-agent",
		"type":       agentTypeName,
		"ouId":       ts.agentOU1ID,
		"attributes": map[string]interface{}{"purpose": "created by agent-manager"},
	})
	ts.Require().NoError(err)

	resp := ts.doAgent(http.MethodPost, "/agents", payload)
	defer resp.Body.Close()

	ts.Equal(http.StatusCreated, resp.StatusCode,
		"agent-manager should be able to create an agent in their own OU")

	// Parse the created agent ID and clean it up via the admin client.
	var created testutils.Agent
	if decodeErr := json.NewDecoder(resp.Body).Decode(&created); decodeErr == nil && created.ID != "" {
		if delErr := testutils.DeleteAgent(created.ID); delErr != nil {
			ts.T().Logf("cleanup: failed to delete created agent %s: %v", created.ID, delErr)
		}
	}
}

// TestCreateAgentInOtherOU verifies the agent-manager is denied creating an agent in OU2.
func (ts *AgentAuthzTestSuite) TestCreateAgentInOtherOU() {
	payload, err := json.Marshal(map[string]interface{}{
		"name":       "authz-denied-agent",
		"type":       agentTypeName,
		"ouId":       ts.agentOU2ID,
		"attributes": map[string]interface{}{"purpose": "denied"},
	})
	ts.Require().NoError(err)

	resp := ts.doAgent(http.MethodPost, "/agents", payload)
	defer resp.Body.Close()

	ts.Equal(http.StatusForbidden, resp.StatusCode,
		"agent-manager must not create an agent in a different OU")
}

// TestUpdateAgentInOwnOU verifies the agent-manager can update an agent in their own OU.
func (ts *AgentAuthzTestSuite) TestUpdateAgentInOwnOU() {
	payload, err := json.Marshal(map[string]interface{}{
		"name":       "authz-target-agent-ou1",
		"type":       agentTypeName,
		"ouId":       ts.agentOU1ID,
		"attributes": map[string]interface{}{"purpose": "updated purpose"},
	})
	ts.Require().NoError(err)

	resp := ts.doAgent(http.MethodPut, "/agents/"+ts.targetAgentOU1ID, payload)
	defer resp.Body.Close()

	ts.Equal(http.StatusOK, resp.StatusCode,
		"agent-manager should be able to update an agent in their own OU")
}

// TestUpdateAgentInOtherOU verifies the agent-manager is denied updating an agent in OU2.
func (ts *AgentAuthzTestSuite) TestUpdateAgentInOtherOU() {
	payload, err := json.Marshal(map[string]interface{}{
		"name":       "authz-target-agent-ou2",
		"type":       agentTypeName,
		"ouId":       ts.agentOU2ID,
		"attributes": map[string]interface{}{"purpose": "denied update"},
	})
	ts.Require().NoError(err)

	resp := ts.doAgent(http.MethodPut, "/agents/"+ts.targetAgentOU2ID, payload)
	defer resp.Body.Close()

	ts.Equal(http.StatusForbidden, resp.StatusCode,
		"agent-manager must not update an agent in a different OU")
}

// TestMoveAgentToOtherOU verifies the destination OU is authorized independently. The caller may
// write to the agent's current OU, so without a check on the destination it could move the agent
// into an OU it does not administer.
func (ts *AgentAuthzTestSuite) TestMoveAgentToOtherOU() {
	payload, err := json.Marshal(map[string]interface{}{
		"name":       "authz-target-agent-ou1",
		"type":       agentTypeName,
		"ouId":       ts.agentOU2ID,
		"attributes": map[string]interface{}{"purpose": "attempted move"},
	})
	ts.Require().NoError(err)

	resp := ts.doAgent(http.MethodPut, "/agents/"+ts.targetAgentOU1ID, payload)
	defer resp.Body.Close()

	ts.Equal(http.StatusForbidden, resp.StatusCode,
		"agent-manager must not move an agent into an OU they do not administer")

	// The agent must remain in OU1.
	getResp := ts.doAgent(http.MethodGet, "/agents/"+ts.targetAgentOU1ID, nil)
	defer getResp.Body.Close()

	ts.Require().Equal(http.StatusOK, getResp.StatusCode, "agent must still be readable in OU1")

	var fetched testutils.Agent
	ts.Require().NoError(json.NewDecoder(getResp.Body).Decode(&fetched))
	ts.Equal(ts.agentOU1ID, fetched.OUID, "denied move must not change the agent's OU")
}

// TestDeleteAgentInOtherOU verifies the agent-manager is denied deleting an agent in OU2.
func (ts *AgentAuthzTestSuite) TestDeleteAgentInOtherOU() {
	resp := ts.doAgent(http.MethodDelete, "/agents/"+ts.targetAgentOU2ID, nil)
	defer resp.Body.Close()

	ts.Equal(http.StatusForbidden, resp.StatusCode,
		"agent-manager must not delete an agent in a different OU")

	// The agent must still exist.
	ts.True(ts.agentExists(ts.targetAgentOU2ID), "denied delete must leave the agent in place")
}

// TestDeleteAgentInOwnOU verifies the agent-manager can delete an agent in their own OU. It targets
// a dedicated fixture agent so no other assertion in this suite depends on it surviving.
func (ts *AgentAuthzTestSuite) TestDeleteAgentInOwnOU() {
	resp := ts.doAgent(http.MethodDelete, "/agents/"+ts.deletableAgentOU1ID, nil)
	defer resp.Body.Close()

	ts.Equal(http.StatusNoContent, resp.StatusCode,
		"agent-manager should be able to delete an agent in their own OU")

	ts.False(ts.agentExists(ts.deletableAgentOU1ID), "deleted agent must be gone")
	ts.deletableAgentOU1ID = ""
}

// ---------------------------------------------------------------------------
// Helper — existence check via the root admin client
// ---------------------------------------------------------------------------

// agentExists reports whether the agent is still present, queried with the root admin client so the
// answer is independent of the scoped token's own authorization.
func (ts *AgentAuthzTestSuite) agentExists(agentID string) bool {
	ts.T().Helper()

	req, err := http.NewRequest(http.MethodGet, testutils.TestServerURL+"/agents/"+agentID, nil)
	ts.Require().NoError(err)

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}
