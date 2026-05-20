/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

const (
	testServerURL = "https://localhost:8095"
	agentBasePath = "/agents"
)

var (
	testOU = testutils.OrganizationUnit{
		Handle:      "test_agent_ou",
		Name:        "Test Organization Unit for Agents",
		Description: "Organization unit created for agent API testing",
		Parent:      nil,
	}

	// agentSchema is reused from the entity type subsystem. Agents need a type that maps
	// to a user type so attribute validation and credential extraction work correctly.
	agentSchema = testutils.UserType{
		Name: "default",
		Schema: map[string]interface{}{
			"description": map[string]interface{}{"type": "string"},
		},
	}

	// entityOnlyAgent has no inbound auth fields — only the entity row is created.
	entityOnlyAgent = Agent{
		Type:        "default",
		Name:        "entity-only-agent",
		Description: "Agent with entity row only",
	}

	// inboundAgent has an auth flow ID — entity + inbound client rows are created.
	inboundAgent = Agent{
		Type:        "default",
		Name:        "inbound-agent",
		Description: "Agent with inbound auth profile",
	}

	// oauthAgent has an inbound auth config with CC grant — entity + inbound + OAuth profile rows.
	oauthAgent = Agent{
		Type:        "default",
		Name:        "oauth-agent",
		Description: "Agent with OAuth client credentials profile",
		InboundAuthConfig: []InboundAuthConfig{
			{
				Type: "oauth2",
				Config: &OAuthAgentConfig{
					ClientID:                "agent_oauth_client",
					ClientSecret:            "agent_oauth_secret",
					GrantTypes:              []string{"client_credentials"},
					TokenEndpointAuthMethod: "client_secret_basic",
					PublicClient:            false,
				},
			},
		},
	}
)

var (
	testOUID          string
	agentSchemaID     string
	defaultAuthFlowID string

	// IDs set during SetupSuite for the primary agent used across multiple tests.
	createdAgentID   string
	createdAgentName string
)

// AgentAPITestSuite covers the full agent CRUD lifecycle, three creation modes,
// OAuth CC token issuance, tree-path endpoints, group membership, and error paths.
type AgentAPITestSuite struct {
	suite.Suite
}

func TestAgentAPITestSuite(t *testing.T) {
	suite.Run(t, new(AgentAPITestSuite))
}

// SetupSuite creates the shared OU, user type, auth flow, and a primary entity-only agent
// that most CRUD tests operate against.
func (ts *AgentAPITestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(testOU)
	ts.Require().NoError(err, "Failed to create test organization unit")
	testOUID = ouID

	agentSchema.OUID = testOUID
	schemaID, err := testutils.CreateAgentType(agentSchema)
	ts.Require().NoError(err, "Failed to create agent schema (user type)")
	agentSchemaID = schemaID

	defaultAuthFlowID, err = testutils.GetFlowIDByHandle("default-basic-flow", "AUTHENTICATION")
	ts.Require().NoError(err, "Failed to get default auth flow ID")

	// Create the primary agent used by list/get/update/groups tests.
	primaryAgent := entityOnlyAgent
	primaryAgent.OUID = testOUID
	id, err := createAgent(primaryAgent)
	ts.Require().NoError(err, "Failed to create primary entity-only agent")
	createdAgentID = id
	createdAgentName = entityOnlyAgent.Name
}

// TearDownSuite removes all resources created during the suite.
func (ts *AgentAPITestSuite) TearDownSuite() {
	if createdAgentID != "" {
		if err := deleteAgent(createdAgentID); err != nil {
			ts.T().Logf("Failed to delete primary agent during teardown: %v", err)
		}
	}
	if agentSchemaID != "" {
		if err := testutils.DeleteAgentType(agentSchemaID); err != nil {
			ts.T().Logf("Failed to delete agent schema during teardown: %v", err)
		}
	}
	if testOUID != "" {
		if err := testutils.DeleteOrganizationUnit(testOUID); err != nil {
			ts.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}
}

// --- CRUD lifecycle ---

func (ts *AgentAPITestSuite) TestAgentListing() {
	client := testutils.GetHTTPClient()
	req, err := http.NewRequest("GET", testServerURL+agentBasePath, nil)
	ts.Require().NoError(err)

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusOK, resp.StatusCode, readBody(resp))

	var listResp AgentListResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&listResp))
	ts.Assert().GreaterOrEqual(listResp.TotalResults, 1)
	ts.Assert().Equal(1, listResp.StartIndex)
	ts.Assert().Equal(len(listResp.Agents), listResp.Count)

	found := false
	for _, a := range listResp.Agents {
		if a.ID == createdAgentID {
			found = true
			ts.Assert().Equal(createdAgentName, a.Name)
			break
		}
	}
	ts.Assert().True(found, "Primary agent not found in list response")
}

func (ts *AgentAPITestSuite) TestAgentPagination() {
	client := testutils.GetHTTPClient()

	// limit=1 must return exactly one agent
	req, err := http.NewRequest("GET", testServerURL+agentBasePath+"?limit=1&offset=0", nil)
	ts.Require().NoError(err)
	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode)

	var page1 AgentListResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&page1))
	ts.Assert().Equal(1, page1.Count)
	ts.Assert().Equal(1, page1.StartIndex)

	// limit=invalid must return 400
	req2, err := http.NewRequest("GET", testServerURL+agentBasePath+"?limit=invalid", nil)
	ts.Require().NoError(err)
	resp2, err := client.Do(req2)
	ts.Require().NoError(err)
	defer resp2.Body.Close()
	ts.Assert().Equal(http.StatusBadRequest, resp2.StatusCode)
}

func (ts *AgentAPITestSuite) TestAgentGetByID() {
	ts.Require().NotEmpty(createdAgentID, "agent ID not available")

	resp, err := doGet(testServerURL+agentBasePath+"/"+createdAgentID)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode, readBody(resp))

	var agent Agent
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&agent))
	ts.Assert().Equal(createdAgentID, agent.ID)
	ts.Assert().Equal(createdAgentName, agent.Name)
	ts.Assert().Equal(testOUID, agent.OUID)
	ts.Assert().Equal("default", agent.Type)
	// GET must never return clientSecret
	if len(agent.InboundAuthConfig) > 0 && agent.InboundAuthConfig[0].Config != nil {
		ts.Assert().Empty(agent.InboundAuthConfig[0].Config.ClientSecret,
			"clientSecret must be scrubbed on GET")
	}
}

func (ts *AgentAPITestSuite) TestAgentGetByID_NotFound() {
	resp, err := doGet(testServerURL + agentBasePath + "/non-existent-id")
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Assert().Equal(http.StatusNotFound, resp.StatusCode)
}

func (ts *AgentAPITestSuite) TestAgentUpdate() {
	ts.Require().NotEmpty(createdAgentID)

	updatePayload := Agent{
		OUID:        testOUID,
		Type:        "default",
		Name:        "entity-only-agent-updated",
		Description: "Updated description",
	}
	body, _ := json.Marshal(updatePayload)
	client := testutils.GetHTTPClient()
	req, err := http.NewRequest("PUT", testServerURL+agentBasePath+"/"+createdAgentID, bytes.NewReader(body))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode, readBody(resp))

	var updated Agent
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&updated))
	ts.Assert().Equal("entity-only-agent-updated", updated.Name)
	ts.Assert().Equal("Updated description", updated.Description)

	// Restore the name so subsequent tests see the expected state.
	restore := Agent{OUID: testOUID, Type: "default", Name: createdAgentName, Description: "Agent with entity row only"}
	restoreBody, _ := json.Marshal(restore)
	req2, err := http.NewRequest("PUT", testServerURL+agentBasePath+"/"+createdAgentID, bytes.NewReader(restoreBody))
	ts.Require().NoError(err)
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := client.Do(req2)
	ts.Require().NoError(err)
	resp2.Body.Close()
}

func (ts *AgentAPITestSuite) TestAgentUpdate_NameConflict() {
	// Create a second agent, then try to rename it to the primary agent's name.
	other := Agent{OUID: testOUID, Type: "default", Name: "agent-conflict-temp"}
	otherID, err := createAgent(other)
	ts.Require().NoError(err)
	defer func() { _ = deleteAgent(otherID) }()

	updatePayload := Agent{Type: "default", Name: createdAgentName}
	body, _ := json.Marshal(updatePayload)
	client := testutils.GetHTTPClient()
	req, err := http.NewRequest("PUT", testServerURL+agentBasePath+"/"+otherID, bytes.NewReader(body))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Assert().Equal(http.StatusConflict, resp.StatusCode)
}

func (ts *AgentAPITestSuite) TestAgentDelete() {
	// Create a transient agent to delete.
	transient := Agent{OUID: testOUID, Type: "default", Name: "agent-to-delete"}
	id, err := createAgent(transient)
	ts.Require().NoError(err)

	client := testutils.GetHTTPClient()
	req, err := http.NewRequest("DELETE", testServerURL+agentBasePath+"/"+id, nil)
	ts.Require().NoError(err)

	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Assert().Equal(http.StatusNoContent, resp.StatusCode)

	// Confirm it's gone.
	getResp, err := doGet(testServerURL + agentBasePath + "/" + id)
	ts.Require().NoError(err)
	getResp.Body.Close()
	ts.Assert().Equal(http.StatusNotFound, getResp.StatusCode)
}

// --- creation mode: entity only ---

func (ts *AgentAPITestSuite) TestCreateAgentEntityOnly() {
	agent := Agent{OUID: testOUID, Type: "default", Name: "create-entity-only"}
	id, err := createAgent(agent)
	ts.Require().NoError(err, "entity-only agent creation must succeed")
	defer func() { _ = deleteAgent(id) }()

	resp, err := doGet(testServerURL + agentBasePath + "/" + id)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode)

	var fetched Agent
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&fetched))
	ts.Assert().Empty(fetched.InboundAuthConfig, "entity-only agent must have no inbound auth config")
	ts.Assert().Empty(fetched.AuthFlowID, "entity-only agent must have no auth flow ID")
}

// --- creation mode: entity + inbound ---

func (ts *AgentAPITestSuite) TestCreateAgentWithInboundProfile() {
	agent := inboundAgent
	agent.OUID = testOUID
	agent.AuthFlowID = defaultAuthFlowID

	resp, err := doPost(testServerURL+agentBasePath, agent)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusCreated, resp.StatusCode, readBodyBytes(resp))

	var created Agent
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&created))
	ts.Assert().NotEmpty(created.ID)
	ts.Assert().Equal(defaultAuthFlowID, created.AuthFlowID)
	defer func() { _ = deleteAgent(created.ID) }()

	// GET must return the inbound fields too.
	getResp, err := doGet(testServerURL + agentBasePath + "/" + created.ID)
	ts.Require().NoError(err)
	defer getResp.Body.Close()
	var fetched Agent
	ts.Require().NoError(json.NewDecoder(getResp.Body).Decode(&fetched))
	ts.Assert().Equal(defaultAuthFlowID, fetched.AuthFlowID)
}

// --- creation mode: entity + inbound + OAuth ---

func (ts *AgentAPITestSuite) TestCreateAgentWithOAuth() {
	agent := oauthAgent
	agent.OUID = testOUID
	agent.AuthFlowID = defaultAuthFlowID

	resp, err := doPost(testServerURL+agentBasePath, agent)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	ts.Require().Equal(http.StatusCreated, resp.StatusCode, string(bodyBytes))

	var created Agent
	ts.Require().NoError(json.Unmarshal(bodyBytes, &created))
	ts.Require().NotEmpty(created.ID)
	defer func() { _ = deleteAgent(created.ID) }()

	// Create response must include clientId and clientSecret.
	ts.Require().Len(created.InboundAuthConfig, 1)
	cfg := created.InboundAuthConfig[0].Config
	ts.Require().NotNil(cfg)
	ts.Assert().Equal("agent_oauth_client", cfg.ClientID)
	ts.Assert().NotEmpty(cfg.ClientSecret, "clientSecret must be returned on create")

	// GET must scrub clientSecret.
	getResp, err := doGet(testServerURL + agentBasePath + "/" + created.ID)
	ts.Require().NoError(err)
	defer getResp.Body.Close()
	var fetched Agent
	ts.Require().NoError(json.NewDecoder(getResp.Body).Decode(&fetched))
	if len(fetched.InboundAuthConfig) > 0 && fetched.InboundAuthConfig[0].Config != nil {
		ts.Assert().Empty(fetched.InboundAuthConfig[0].Config.ClientSecret,
			"clientSecret must be scrubbed on GET")
	}
}

// --- creation validation ---

func (ts *AgentAPITestSuite) TestCreateAgent_MissingName() {
	agent := Agent{OUID: testOUID, Type: "default"}
	resp, err := doPost(testServerURL+agentBasePath, agent)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Assert().Equal(http.StatusBadRequest, resp.StatusCode)
}

func (ts *AgentAPITestSuite) TestCreateAgent_MissingType() {
	agent := Agent{OUID: testOUID, Name: "no-type-agent"}
	resp, err := doPost(testServerURL+agentBasePath, agent)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Assert().Equal(http.StatusBadRequest, resp.StatusCode)
}

func (ts *AgentAPITestSuite) TestCreateAgent_DuplicateName() {
	// Using the primary agent name which already exists.
	dup := Agent{OUID: testOUID, Type: "default", Name: createdAgentName}
	resp, err := doPost(testServerURL+agentBasePath, dup)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Assert().Equal(http.StatusConflict, resp.StatusCode)
}

// --- group membership ---

func (ts *AgentAPITestSuite) TestAgentGroups_EmptyMembership() {
	ts.Require().NotEmpty(createdAgentID)

	resp, err := doGet(testServerURL + agentBasePath + "/" + createdAgentID + "/groups")
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode, readBody(resp))

	var groupResp AgentGroupListResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&groupResp))
	ts.Assert().Equal(0, groupResp.TotalResults)
	ts.Assert().Equal(1, groupResp.StartIndex)
}

func (ts *AgentAPITestSuite) TestAgentGroupMembership() {
	// Create agent to add to a group.
	agentID, err := createAgent(Agent{OUID: testOUID, Type: "default", Name: "group-member-agent"})
	ts.Require().NoError(err)
	defer func() { _ = deleteAgent(agentID) }()

	// Create a group with the agent as a member.
	groupID, err := testutils.CreateGroup(testutils.Group{
		Name:  "agent-group-membership-test",
		OUID:  testOUID,
		Members: []testutils.Member{{Id: agentID, Type: "agent"}},
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteGroup(groupID) }()

	// Verify the agent appears in the group's member list.
	members, err := testutils.GetGroupMembers(groupID)
	ts.Require().NoError(err)
	found := false
	for _, m := range members {
		if m.ID == agentID && m.Type == "agent" {
			found = true
			break
		}
	}
	ts.Assert().True(found, "Agent should appear as a member of the group")

	// Verify the group appears in GET /agents/{id}/groups.
	resp, err := doGet(testServerURL + agentBasePath + "/" + agentID + "/groups")
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode, readBody(resp))

	var groupResp AgentGroupListResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&groupResp))
	ts.Assert().GreaterOrEqual(groupResp.TotalResults, 1)
	groupFound := false
	for _, g := range groupResp.Groups {
		if g.ID == groupID {
			groupFound = true
			break
		}
	}
	ts.Assert().True(groupFound, "Group should appear in agent's group list")
}

func (ts *AgentAPITestSuite) TestAgentGroupMembership_AddViaAPI() {
	// Create agent and an empty group.
	agentID, err := createAgent(Agent{OUID: testOUID, Type: "default", Name: "add-via-api-agent"})
	ts.Require().NoError(err)
	defer func() { _ = deleteAgent(agentID) }()

	groupID, err := testutils.CreateGroup(testutils.Group{Name: "agent-add-member-group", OUID: testOUID})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteGroup(groupID) }()

	// Call POST /groups/{id}/members/add with type "agent".
	addReq := map[string]interface{}{
		"members": []map[string]string{{"id": agentID, "type": "agent"}},
	}
	body, _ := json.Marshal(addReq)
	client := testutils.GetHTTPClient()
	req, err := http.NewRequest("POST", testServerURL+"/groups/"+groupID+"/members/add", bytes.NewReader(body))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode, readBody(resp))

	// Verify the agent is now in the group.
	members, err := testutils.GetGroupMembers(groupID)
	ts.Require().NoError(err)
	found := false
	for _, m := range members {
		if m.ID == agentID && m.Type == "agent" {
			found = true
			break
		}
	}
	ts.Assert().True(found, "Agent should appear as a member after add-via-API")
}

func (ts *AgentAPITestSuite) TestAgentRoleAssignment() {
	// Create agent.
	agentID, err := createAgent(Agent{OUID: testOUID, Type: "default", Name: "role-assigned-agent"})
	ts.Require().NoError(err)
	defer func() { _ = deleteAgent(agentID) }()

	// Create a role and assign the agent directly.
	roleID, err := testutils.CreateRole(testutils.Role{
		Name: "agent-direct-role",
		OUID: testOUID,
		Assignments: []testutils.Assignment{
			{ID: agentID, Type: "agent"},
		},
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteRole(roleID) }()

	// Verify assignment appears in GET /roles/{id}/assignments.
	assignments, err := testutils.GetRoleAssignments(roleID)
	ts.Require().NoError(err)
	ts.Require().Len(assignments, 1)
	ts.Assert().Equal(agentID, assignments[0].ID)
	ts.Assert().Equal("agent", assignments[0].Type)
}

func (ts *AgentAPITestSuite) TestAgentRoleAssignment_ViaGroup() {
	// Create agent and add it to a group.
	agentID, err := createAgent(Agent{OUID: testOUID, Type: "default", Name: "group-role-agent"})
	ts.Require().NoError(err)
	defer func() { _ = deleteAgent(agentID) }()

	groupID, err := testutils.CreateGroup(testutils.Group{
		Name:    "agent-role-group",
		OUID:    testOUID,
		Members: []testutils.Member{{Id: agentID, Type: "agent"}},
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteGroup(groupID) }()

	// Assign the group to a role.
	roleID, err := testutils.CreateRole(testutils.Role{
		Name: "agent-group-role",
		OUID: testOUID,
		Assignments: []testutils.Assignment{
			{ID: groupID, Type: "group"},
		},
	})
	ts.Require().NoError(err)
	defer func() { _ = testutils.DeleteRole(roleID) }()

	// Verify the group assignment appears in the role.
	assignments, err := testutils.GetRoleAssignments(roleID)
	ts.Require().NoError(err)
	ts.Require().Len(assignments, 1)
	ts.Assert().Equal(groupID, assignments[0].ID)
	ts.Assert().Equal("group", assignments[0].Type)

	// Verify the agent is still a member of the group.
	members, err := testutils.GetGroupMembers(groupID)
	ts.Require().NoError(err)
	found := false
	for _, m := range members {
		if m.ID == agentID && m.Type == "agent" {
			found = true
			break
		}
	}
	ts.Assert().True(found, "Agent should still be a member of the group")
}

// --- transition: entity-only → inbound profile on update ---

func (ts *AgentAPITestSuite) TestUpdateAgent_AddInboundProfile() {
	// Start with entity-only.
	agentID, err := createAgent(Agent{OUID: testOUID, Type: "default", Name: "transition-agent"})
	ts.Require().NoError(err)
	defer func() { _ = deleteAgent(agentID) }()

	// Update to add inbound fields.
	withInbound := Agent{
		OUID:       testOUID,
		Type:       "default",
		Name:       "transition-agent",
		AuthFlowID: defaultAuthFlowID,
	}
	body, _ := json.Marshal(withInbound)
	client := testutils.GetHTTPClient()
	req, err := http.NewRequest("PUT", testServerURL+agentBasePath+"/"+agentID, bytes.NewReader(body))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode, readBody(resp))

	var updated Agent
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&updated))
	ts.Assert().Equal(defaultAuthFlowID, updated.AuthFlowID)
}

// --- transition: inbound profile → entity-only on update ---

func (ts *AgentAPITestSuite) TestUpdateAgent_RemoveInboundProfile() {
	// Start with inbound.
	agentID, err := createAgent(Agent{
		OUID:       testOUID,
		Type:       "default",
		Name:       "strip-inbound-agent",
		AuthFlowID: defaultAuthFlowID,
	})
	ts.Require().NoError(err)
	defer func() { _ = deleteAgent(agentID) }()

	// Update dropping all inbound fields.
	stripped := Agent{OUID: testOUID, Type: "default", Name: "strip-inbound-agent"}
	body, _ := json.Marshal(stripped)
	client := testutils.GetHTTPClient()
	req, err := http.NewRequest("PUT", testServerURL+agentBasePath+"/"+agentID, bytes.NewReader(body))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode, readBody(resp))

	var updated Agent
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&updated))
	ts.Assert().Empty(updated.AuthFlowID,
		"AuthFlowID must be cleared after removing inbound profile")
}

// --- helpers ---

func createAgent(agent Agent) (string, error) {
	resp, err := doPost(testServerURL+agentBasePath, agent)
	if err != nil {
		return "", fmt.Errorf("failed to send create request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("expected status 201, got %d. Response: %s", resp.StatusCode, string(bodyBytes))
	}

	var created Agent
	if err := json.Unmarshal(bodyBytes, &created); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}
	if created.ID == "" {
		return "", fmt.Errorf("response does not contain id. Response: %s", string(bodyBytes))
	}
	return created.ID, nil
}

func deleteAgent(agentID string) error {
	client := testutils.GetHTTPClient()
	req, err := http.NewRequest("DELETE", testServerURL+agentBasePath+"/"+agentID, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send delete request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("expected status 204, got %d. Response: %s", resp.StatusCode, string(body))
	}
	return nil
}

func doGet(url string) (*http.Response, error) {
	client := testutils.GetHTTPClient()
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

func doPost(url string, body interface{}) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	client := testutils.GetHTTPClient()
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return client.Do(req)
}

func readBody(resp *http.Response) string {
	b, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewReader(b))
	return string(b)
}

func readBodyBytes(resp *http.Response) string {
	b, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewReader(b))
	return string(b)
}

// ============================================================================
// AgentAttributesTestSuite — custom attribute CRUD and filter operations
// ============================================================================

var (
	attrTestOU = testutils.OrganizationUnit{
		Handle:      "agent-attr-test-ou",
		Name:        "Agent Attribute Test OU",
		Description: "Organization unit for agent attribute CRUD testing",
		Parent:      nil,
	}

	attrAgentSchema = testutils.UserType{
		Name: "default",
		Schema: map[string]interface{}{
			"region": map[string]interface{}{"type": "string"},
			"tier":   map[string]interface{}{"type": "string"},
		},
	}
)

// AgentAttributesTestSuite covers custom attribute CRUD and filter operations on agents.
type AgentAttributesTestSuite struct {
	suite.Suite
	ouID     string
	schemaID string
}

func TestAgentAttributesTestSuite(t *testing.T) {
	suite.Run(t, new(AgentAttributesTestSuite))
}

func (ts *AgentAttributesTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(attrTestOU)
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

	attrAgentSchema.OUID = ts.ouID
	schemaID, err := testutils.CreateAgentType(attrAgentSchema)
	ts.Require().NoError(err, "Failed to create agent schema")
	ts.schemaID = schemaID
}

func (ts *AgentAttributesTestSuite) TearDownSuite() {
	if ts.schemaID != "" {
		_ = testutils.DeleteAgentType(ts.schemaID)
	}
	if ts.ouID != "" {
		_ = testutils.DeleteOrganizationUnit(ts.ouID)
	}
}

// TestAgentAttributes_CreateWithAttributes verifies that custom attributes submitted on
// create are stored and returned verbatim by GET.
func (ts *AgentAttributesTestSuite) TestAgentAttributes_CreateWithAttributes() {
	attrs := json.RawMessage(`{"region":"us-east","tier":"premium"}`)
	agent := Agent{
		OUID:       ts.ouID,
		Type:       "default",
		Name:       "attr-create-agent",
		Attributes: attrs,
	}

	resp, err := doPost(testServerURL+agentBasePath, agent)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusCreated, resp.StatusCode, readBodyBytes(resp))

	var created Agent
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&created))
	ts.Require().NotEmpty(created.ID)
	defer func() { _ = deleteAgent(created.ID) }()

	getResp, err := doGet(testServerURL + agentBasePath + "/" + created.ID)
	ts.Require().NoError(err)
	defer getResp.Body.Close()
	ts.Require().Equal(http.StatusOK, getResp.StatusCode)

	var fetched Agent
	ts.Require().NoError(json.NewDecoder(getResp.Body).Decode(&fetched))

	var gotAttrs map[string]interface{}
	ts.Require().NoError(json.Unmarshal(fetched.Attributes, &gotAttrs))
	ts.Assert().Equal("us-east", gotAttrs["region"])
	ts.Assert().Equal("premium", gotAttrs["tier"])
}

// TestAgentAttributes_UpdateAttributes verifies that attributes can be replaced on update.
func (ts *AgentAttributesTestSuite) TestAgentAttributes_UpdateAttributes() {
	agentID, err := createAgent(Agent{
		OUID:       ts.ouID,
		Type:       "default",
		Name:       "attr-update-agent",
		Attributes: json.RawMessage(`{"region":"eu-west","tier":"standard"}`),
	})
	ts.Require().NoError(err)
	defer func() { _ = deleteAgent(agentID) }()

	updated := Agent{
		OUID:       ts.ouID,
		Type:       "default",
		Name:       "attr-update-agent",
		Attributes: json.RawMessage(`{"region":"ap-south","tier":"enterprise"}`),
	}
	body, _ := json.Marshal(updated)
	client := testutils.GetHTTPClient()
	req, err := http.NewRequest("PUT", testServerURL+agentBasePath+"/"+agentID, bytes.NewReader(body))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode, readBody(resp))

	getResp, err := doGet(testServerURL + agentBasePath + "/" + agentID)
	ts.Require().NoError(err)
	defer getResp.Body.Close()
	ts.Require().Equal(http.StatusOK, getResp.StatusCode, readBody(getResp))

	var fetched Agent
	ts.Require().NoError(json.NewDecoder(getResp.Body).Decode(&fetched))

	var gotAttrs map[string]interface{}
	ts.Require().NoError(json.Unmarshal(fetched.Attributes, &gotAttrs))
	ts.Assert().Equal("ap-south", gotAttrs["region"])
	ts.Assert().Equal("enterprise", gotAttrs["tier"])
}

// TestAgentAttributes_FilterByAttribute verifies that GET /agents?filter=attr eq "value"
// returns only matching agents.
func (ts *AgentAttributesTestSuite) TestAgentAttributes_FilterByAttribute() {
	idA, err := createAgent(Agent{
		OUID:       ts.ouID,
		Type:       "default",
		Name:       "filter-agent-alpha",
		Attributes: json.RawMessage(`{"region":"filter-target","tier":"gold"}`),
	})
	ts.Require().NoError(err)
	defer func() { _ = deleteAgent(idA) }()

	idB, err := createAgent(Agent{
		OUID:       ts.ouID,
		Type:       "default",
		Name:       "filter-agent-beta",
		Attributes: json.RawMessage(`{"region":"other-region","tier":"silver"}`),
	})
	ts.Require().NoError(err)
	defer func() { _ = deleteAgent(idB) }()

	q := url.Values{}
	q.Set("filter", `region eq "filter-target"`)
	resp, err := doGet(testServerURL + agentBasePath + "?" + q.Encode())
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode, readBody(resp))

	var listResp AgentListResponse
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&listResp))

	found := false
	for _, a := range listResp.Agents {
		if a.ID == idA {
			found = true
		}
		ts.Assert().NotEqual(idB, a.ID, "Beta agent must not appear in filtered results")
	}
	ts.Assert().True(found, "Alpha agent must appear in filtered results")
}

// TestAgentAttributes_NullifyAttributes verifies that omitting attributes on update
// clears previously stored attributes.
func (ts *AgentAttributesTestSuite) TestAgentAttributes_NullifyAttributes() {
	agentID, err := createAgent(Agent{
		OUID:       ts.ouID,
		Type:       "default",
		Name:       "attr-nullify-agent",
		Attributes: json.RawMessage(`{"region":"to-clear","tier":"basic"}`),
	})
	ts.Require().NoError(err)
	defer func() { _ = deleteAgent(agentID) }()

	stripped := Agent{OUID: ts.ouID, Type: "default", Name: "attr-nullify-agent"}
	body, _ := json.Marshal(stripped)
	client := testutils.GetHTTPClient()
	req, err := http.NewRequest("PUT", testServerURL+agentBasePath+"/"+agentID, bytes.NewReader(body))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusOK, resp.StatusCode, readBody(resp))

	getResp, err := doGet(testServerURL + agentBasePath + "/" + agentID)
	ts.Require().NoError(err)
	defer getResp.Body.Close()
	ts.Require().Equal(http.StatusOK, getResp.StatusCode, readBody(getResp))

	var fetched Agent
	ts.Require().NoError(json.NewDecoder(getResp.Body).Decode(&fetched))
	ts.Assert().Empty(fetched.Attributes, "Attributes should be empty after nullify update")
}
