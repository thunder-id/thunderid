// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// AgentAuthzTestSuite validates agent CRUD authz on a deployment-bounded server, where a token's
// scopes apply to every OU in the deployment, not only to the OU the token names.
//
// Permission model:
//
//	system:agent      → create, update, delete agents
//	system:agent:view → list, get agents and their groups and roles (implied by system:agent)
//
// An agent-manager living in OU1 holds the system:agent permission. The suite verifies that:
//
//   - Read operations on agents in OU1 and OU2 (sibling) are allowed (200)
//   - Write operations on agents in OU1 and OU2 are allowed (201/200/204)
//   - Moving an agent from OU1 into OU2 is allowed
//   - Listing agents returns agents from both OUs
//
// AgentAuthzDPTestSuite covers the OU-bounded behavior.
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
		Handle:      agentMgrUserType,
		DisplayName: "Authz Agent Mgr Type",
		OUID:        ts.agentOU1ID,
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
		DisplayName: "Default",
		OUID:        ts.agentOU1ID,
		Schema: map[string]interface{}{
			"name":    map[string]interface{}{"type": "string", "required": true, "unique": true},
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
		Type:       agentTypeName,
		OUID:       ts.agentOU1ID,
		Attributes: map[string]interface{}{"name": "authz-target-agent-ou1", "purpose": "target in OU1"},
	})
	ts.Require().NoError(err, "create target agent in OU1")
	ts.targetAgentOU1ID = targetOU1ID

	deletableID, err := testutils.CreateAgent(testutils.Agent{
		Type:       agentTypeName,
		OUID:       ts.agentOU1ID,
		Attributes: map[string]interface{}{"name": "authz-deletable-agent-ou1", "purpose": "deletable in OU1"},
	})
	ts.Require().NoError(err, "create deletable agent in OU1")
	ts.deletableAgentOU1ID = deletableID

	targetOU2ID, err := testutils.CreateAgent(testutils.Agent{
		Type:       agentTypeName,
		OUID:       ts.agentOU2ID,
		Attributes: map[string]interface{}{"name": "authz-target-agent-ou2", "purpose": "target in OU2"},
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
	// referencing a deleted OU and a later suite's restore fails. A failed restore keeps the OU:
	// leaking one is cheaper than every later suite inheriting a dangling reference.
	agentTypeRestored := true
	if ts.agentTypeSnapshot != nil {
		if err := testutils.RestoreAgentType(ts.agentTypeSnapshot); err != nil {
			ts.T().Errorf("teardown: restore the default agent type: %v", err)
			agentTypeRestored = false
		}
	}
	if ts.userTypeOU1ID != "" {
		if err := testutils.DeleteUserType(ts.userTypeOU1ID); err != nil {
			ts.T().Logf("teardown: delete agent-manager user type: %v", err)
		}
	}
	if ts.agentOU2ID != "" && agentTypeRestored {
		if err := testutils.DeleteOrganizationUnit(ts.agentOU2ID); err != nil {
			ts.T().Logf("teardown: delete agent-authz OU2: %v", err)
		}
	}
	if ts.agentOU1ID != "" && agentTypeRestored {
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

// TestListAgents verifies the list contains agents from both OUs. Each target agent is looked up by
// name, so the result does not depend on how many agents fit on the first page.
func (ts *AgentAuthzTestSuite) TestListAgents() {
	for name, id := range map[string]string{
		"authz-target-agent-ou1": ts.targetAgentOU1ID,
		"authz-target-agent-ou2": ts.targetAgentOU2ID,
	} {
		q := url.Values{}
		q.Set("filter", fmt.Sprintf("name eq %q", name))
		resp := ts.doAgent(http.MethodGet, "/agents?"+q.Encode(), nil)

		ts.Equal(http.StatusOK, resp.StatusCode, "list agents should succeed")

		var listResp testutils.AgentListResponse
		ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&listResp))
		resp.Body.Close()

		ids := make([]string, 0, len(listResp.Agents))
		for _, a := range listResp.Agents {
			ids = append(ids, a.ID)
		}
		ts.Containsf(ids, id, "list must include %s, got IDs: %v", name, ids)
	}
}

// TestGetAgentInOwnOU verifies the agent-manager can read an agent in their own OU.
func (ts *AgentAuthzTestSuite) TestGetAgentInOwnOU() {
	resp := ts.doAgent(http.MethodGet, "/agents/"+ts.targetAgentOU1ID, nil)
	defer resp.Body.Close()

	ts.Equal(http.StatusOK, resp.StatusCode,
		"agent-manager should be able to read an agent in their own OU")
}

// TestGetAgentInOtherOU verifies the agent-manager can read an agent in OU2.
func (ts *AgentAuthzTestSuite) TestGetAgentInOtherOU() {
	resp := ts.doAgent(http.MethodGet, "/agents/"+ts.targetAgentOU2ID, nil)
	defer resp.Body.Close()

	ts.Equal(http.StatusOK, resp.StatusCode,
		"agent-manager should be able to read an agent in a different OU")
}

// TestGetAgentGroupsInOtherOU verifies the agent-manager can read the groups of an agent in OU2.
func (ts *AgentAuthzTestSuite) TestGetAgentGroupsInOtherOU() {
	resp := ts.doAgent(http.MethodGet, "/agents/"+ts.targetAgentOU2ID+"/groups", nil)
	defer resp.Body.Close()

	ts.Equal(http.StatusOK, resp.StatusCode,
		"agent-manager should be able to read groups of an agent in a different OU")
}

// TestGetAgentRolesInOtherOU verifies the agent-manager can read the roles of an agent in OU2.
func (ts *AgentAuthzTestSuite) TestGetAgentRolesInOtherOU() {
	resp := ts.doAgent(http.MethodGet, "/agents/"+ts.targetAgentOU2ID+"/roles", nil)
	defer resp.Body.Close()

	ts.Equal(http.StatusOK, resp.StatusCode,
		"agent-manager should be able to read roles of an agent in a different OU")
}

// ---------------------------------------------------------------------------
// Tests — WRITE operations (system:agent)
// ---------------------------------------------------------------------------

// TestCreateAgentInOwnOU verifies the agent-manager can create an agent in their own OU.
func (ts *AgentAuthzTestSuite) TestCreateAgentInOwnOU() {
	payload, err := json.Marshal(map[string]interface{}{
		"type":       agentTypeName,
		"ouId":       ts.agentOU1ID,
		"attributes": map[string]interface{}{"name": "authz-created-agent", "purpose": "created by agent-manager"},
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

// TestCreateAgentInOtherOU verifies the agent-manager can create an agent in OU2.
func (ts *AgentAuthzTestSuite) TestCreateAgentInOtherOU() {
	payload, err := json.Marshal(map[string]interface{}{
		"type":       agentTypeName,
		"ouId":       ts.agentOU2ID,
		"attributes": map[string]interface{}{"name": "authz-created-agent-ou2", "purpose": "created in OU2"},
	})
	ts.Require().NoError(err)

	resp := ts.doAgent(http.MethodPost, "/agents", payload)
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusCreated, resp.StatusCode,
		"agent-manager should be able to create an agent in a different OU")

	var created testutils.Agent
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&created))
	ts.Require().NotEmpty(created.ID)
	defer func() {
		if delErr := testutils.DeleteAgent(created.ID); delErr != nil {
			ts.T().Logf("cleanup: failed to delete created agent %s: %v", created.ID, delErr)
		}
	}()

	ts.Equal(ts.agentOU2ID, created.OUID, "created agent must belong to OU2")
}

// TestUpdateAgentInOwnOU verifies the agent-manager can update an agent in their own OU.
func (ts *AgentAuthzTestSuite) TestUpdateAgentInOwnOU() {
	payload, err := json.Marshal(map[string]interface{}{
		"type":       agentTypeName,
		"ouId":       ts.agentOU1ID,
		"attributes": map[string]interface{}{"name": "authz-target-agent-ou1", "purpose": "updated purpose"},
	})
	ts.Require().NoError(err)

	resp := ts.doAgent(http.MethodPut, "/agents/"+ts.targetAgentOU1ID, payload)
	defer resp.Body.Close()

	ts.Equal(http.StatusOK, resp.StatusCode,
		"agent-manager should be able to update an agent in their own OU")
}

// TestUpdateAgentInOtherOU verifies the agent-manager can update an agent in OU2. It targets a
// dedicated agent so the shared OU2 fixture stays unchanged.
func (ts *AgentAuthzTestSuite) TestUpdateAgentInOtherOU() {
	agentID := ts.createDisposableAgent(ts.agentOU2ID, "authz-update-agent-ou2")

	payload, err := json.Marshal(map[string]interface{}{
		"type":       agentTypeName,
		"ouId":       ts.agentOU2ID,
		"attributes": map[string]interface{}{"name": "authz-update-agent-ou2", "purpose": "updated purpose"},
	})
	ts.Require().NoError(err)

	resp := ts.doAgent(http.MethodPut, "/agents/"+agentID, payload)
	defer resp.Body.Close()

	ts.Equal(http.StatusOK, resp.StatusCode,
		"agent-manager should be able to update an agent in a different OU")
}

// TestMoveAgentToOtherOU verifies the agent-manager can move an agent from OU1 into OU2. It moves a
// dedicated agent so the shared OU1 fixture stays in OU1.
func (ts *AgentAuthzTestSuite) TestMoveAgentToOtherOU() {
	agentID := ts.createDisposableAgent(ts.agentOU1ID, "authz-move-agent")

	payload, err := json.Marshal(map[string]interface{}{
		"type":       agentTypeName,
		"ouId":       ts.agentOU2ID,
		"attributes": map[string]interface{}{"name": "authz-move-agent", "purpose": "moved to OU2"},
	})
	ts.Require().NoError(err)

	resp := ts.doAgent(http.MethodPut, "/agents/"+agentID, payload)
	defer resp.Body.Close()

	ts.Equal(http.StatusOK, resp.StatusCode,
		"agent-manager should be able to move an agent into a different OU")

	fetched, err := testutils.GetAgent(agentID)
	ts.Require().NoError(err, "moved agent must still be readable")
	ts.Equal(ts.agentOU2ID, fetched.OUID, "move must change the agent's OU to OU2")
}

// TestDeleteAgentInOtherOU verifies the agent-manager can delete an agent in OU2. It targets a
// dedicated agent so the shared OU2 fixture survives.
func (ts *AgentAuthzTestSuite) TestDeleteAgentInOtherOU() {
	agentID := ts.createDisposableAgent(ts.agentOU2ID, "authz-delete-agent-ou2")

	resp := ts.doAgent(http.MethodDelete, "/agents/"+agentID, nil)
	defer resp.Body.Close()

	ts.Equal(http.StatusNoContent, resp.StatusCode,
		"agent-manager should be able to delete an agent in a different OU")

	ts.False(ts.agentExists(agentID), "deleted agent must be gone")
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
// Helpers using the root admin client
// ---------------------------------------------------------------------------

// createDisposableAgent creates an agent with the root admin client for a test that changes or
// deletes it. The agent is deleted when the test ends, unless the test already deleted it.
func (ts *AgentAuthzTestSuite) createDisposableAgent(ouID, name string) string {
	ts.T().Helper()

	agentID, err := testutils.CreateAgent(testutils.Agent{
		Type:       agentTypeName,
		OUID:       ouID,
		Attributes: map[string]interface{}{"name": name, "purpose": "disposable"},
	})
	ts.Require().NoError(err, "create disposable agent %s", name)

	t := ts.T()
	t.Cleanup(func() {
		if _, getErr := testutils.GetAgent(agentID); getErr != nil {
			return
		}
		if delErr := testutils.DeleteAgent(agentID); delErr != nil {
			t.Logf("cleanup: delete agent %s: %v", agentID, delErr)
		}
	})
	return agentID
}

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
