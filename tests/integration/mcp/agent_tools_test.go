// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// agentToolsKey returns a public EC key for an agent's key set.
func agentToolsKey(kid string) map[string]any {
	return map[string]any{"kty": "EC", "crv": "P-256", "kid": kid,
		"x": "f83OJ3D2xF1Bg8vub9tLe1gHMzV76e8Tus9uPHvRVEU", "y": "x_FEzRu9m36HLN_tue659LNpXW6pCyStikYjKIWI5a0"}
}

// executeFlow calls execute_flow and returns its result, failing the test when the call fails.
func (ts *MCPTestSuite) executeFlow(token string, args map[string]any) map[string]any {
	result, raw := ts.callMCPTool(token, "execute_flow", args)
	ts.Require().Falsef(result.Result.IsError, "execute_flow failed: %s", raw)
	return result.Result.StructuredContent
}

// defaultAgentType returns the default agent type as list_agent_types lists it. Other agent types can
// exist beside it, one per organization unit.
func (ts *MCPTestSuite) defaultAgentType(token string) (map[string]any, string) {
	listed, raw := ts.callMCPTool(token, "list_agent_types", map[string]any{})
	ts.Require().Falsef(listed.Result.IsError, "list_agent_types failed: %s", raw)
	types, _ := listed.Result.StructuredContent["types"].([]any)
	for _, t := range types {
		if agentType, _ := t.(map[string]any); agentType["handle"] == "default" {
			return agentType, raw
		}
	}
	ts.Require().Failf("the default agent type is not listed", "%s", raw)
	return nil, raw
}

// agentTypeChildOU creates an organization unit under the default agent type's own unit, the only units
// the agent onboarding flow places an agent of that type in, and deletes it when the test ends.
func (ts *MCPTestSuite) agentTypeChildOU(token string) string {
	agentType, raw := ts.defaultAgentType(token)
	parent, _ := agentType["ouId"].(string)
	ts.Require().NotEmpty(parent, raw)
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: fmt.Sprintf("mcp-agents-%d", time.Now().UnixNano()), Name: "MCP agents", Parent: &parent})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteOrganizationUnit(ouID) })
	return ouID
}

// onboardAgentForTest onboards an agent of the default agent type through the data plane's agent
// onboarding flow, in a unit under the agent type's own, and deletes it when the test ends. The flow
// offers every agent type, so the default one is chosen up front. A step that asks only for optional
// inputs, such as the owner, is passed by going on with nothing, as an agent leaving it to the caller
// would. It returns the flow's final data (agentId, clientId and clientSecret) and the agent's unit.
func (ts *MCPTestSuite) onboardAgentForTest(token, name string) (map[string]any, string) {
	ouID := ts.agentTypeChildOU(token)
	out := ts.executeFlow(token, map[string]any{
		"flow": "agent_onboarding", "inputs": map[string]any{"name": name, "agentType": "default", "ouId": ouID}})
	for range 3 {
		if out["status"] != "needs_input" {
			break
		}
		inputs, _ := out["inputs"].([]any)
		for _, i := range inputs {
			input, _ := i.(map[string]any)
			ts.Require().NotEqualf(true, input["required"], "the flow asks again for %v: %v", input, out)
		}
		next := map[string]any{"executionId": out["executionId"], "challengeToken": out["challengeToken"]}
		if actions, _ := out["actions"].([]any); len(actions) == 1 {
			next["action"] = actions[0]
		}
		out = ts.executeFlow(token, next)
	}
	ts.Require().Equalf("complete", out["status"], "the flow must finish: %v", out)
	data, _ := out["data"].(map[string]any)
	agentID, _ := data["agentId"].(string)
	ts.Require().NotEmptyf(agentID, "the flow must return the agent: %v", out)
	ts.T().Cleanup(func() { _ = testutils.DeleteAgent(agentID) })
	ts.NotEmpty(out["notice"], "a returned secret comes with the one-time notice")
	return data, ouID
}

// A data plane onboards agents through its agent onboarding flow, so it offers execute_flow and not
// onboard_agent.
func (ts *MCPTestSuite) TestAgentTools_DataPlaneOnboardsThroughTheFlow() {
	message, body := ts.postStatelessMCP(ts.mcpScopedAdminToken(), "tools/list", "", map[string]any{})
	var listed struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	ts.Require().NoErrorf(json.Unmarshal([]byte(message), &listed), "tools/list returned: %s", body)
	names := make([]string, 0, len(listed.Result.Tools))
	for _, tool := range listed.Result.Tools {
		names = append(names, tool.Name)
	}
	ts.Contains(names, "execute_flow")
	ts.Contains(names, "update_agent")
	ts.NotContains(names, "onboard_agent")
}

// The agent tools run end to end: the secret the onboarding flow returns works at the token endpoint and
// is never returned again, the reads describe the agent, an update keeps the secret, and a rotation
// replaces it.
func (ts *MCPTestSuite) TestAgentTools_OnboardUpdateAndRotateSecret() {
	token := ts.mcpScopedAdminToken()
	name := fmt.Sprintf("mcp-agent-%d", time.Now().UnixNano())

	data, ouID := ts.onboardAgentForTest(token, name)
	agentID, _ := data["agentId"].(string)
	clientID, _ := data["clientId"].(string)
	secret, _ := data["clientSecret"].(string)
	ts.Require().NotEmpty(clientID)
	ts.Require().NotEmpty(secret)

	issued, err := testutils.RequestClientCredentialsToken(clientID, secret, "", "")
	ts.Require().NoError(err)
	ts.Equalf(http.StatusOK, issued.StatusCode, "the onboarded secret must work: %s", issued.Body)

	got, raw := ts.callMCPTool(token, "get_agent", map[string]any{"clientId": clientID})
	ts.Require().Falsef(got.Result.IsError, "get_agent failed: %s", raw)
	ts.NotContains(raw, secret)
	agent := got.Result.StructuredContent
	ts.Equal(agentID, agent["id"])
	ts.Equal(name, agent["name"])
	ts.Equal("ACTIVE", agent["status"])
	ts.Equal("client_secret", agent["credential"])
	ts.Equal(ouID, agent["ouId"])
	ts.NotEmpty(agent["owner"], "the calling user owns the agent")

	listed, raw := ts.callMCPTool(token, "list_agents", map[string]any{"name": name})
	ts.Require().Falsef(listed.Result.IsError, "list_agents failed: %s", raw)
	ts.Contains(raw, agentID)
	byOwner, raw := ts.callMCPTool(token, "list_agents", map[string]any{"owner": agent["owner"], "name": name})
	ts.Require().Falsef(byOwner.Result.IsError, "list_agents by owner failed: %s", raw)
	ts.Contains(raw, agentID)

	updated, raw := ts.callMCPTool(token, "update_agent", map[string]any{
		"agentId": agentID, "actsForUsers": []string{"token_exchange"}, "description": "Syncs stock"})
	ts.Require().Falsef(updated.Result.IsError, "update_agent failed: %s", raw)
	ts.NotContains(raw, "clientSecret", "an update that keeps the credential issues no secret")
	updatedAgent, _ := updated.Result.StructuredContent["changed"].(map[string]any)
	ts.Equal([]any{"token_exchange"}, updatedAgent["actsForUsers"])
	stored, err := testutils.GetAgent(agentID)
	ts.Require().NoError(err)
	ts.Equal("Syncs stock", stored.Description)
	ts.Equal(name, stored.Attributes.(map[string]any)["name"], "the update keeps the name")
	issued, err = testutils.RequestClientCredentialsToken(clientID, secret, "", "")
	ts.Require().NoError(err)
	ts.Equalf(http.StatusOK, issued.StatusCode, "the update must keep the secret: %s", issued.Body)

	rotated, raw := ts.callMCPTool(token, "rotate_agent_credentials", map[string]any{"agentId": agentID})
	ts.Require().Falsef(rotated.Result.IsError, "rotate_agent_credentials failed: %s", raw)
	newSecret, _ := rotated.Result.StructuredContent["clientSecret"].(string)
	ts.Require().NotEmpty(newSecret)
	ts.NotEqual(secret, newSecret)
	issued, err = testutils.RequestClientCredentialsToken(clientID, secret, "", "")
	ts.Require().NoError(err)
	ts.Equal(http.StatusUnauthorized, issued.StatusCode, "the old secret must stop working")
	issued, err = testutils.RequestClientCredentialsToken(clientID, newSecret, "", "")
	ts.Require().NoError(err)
	ts.Equalf(http.StatusOK, issued.StatusCode, "the new secret must work: %s", issued.Body)
}

// An onboarded agent that update_agent sets up to sign users in and exchange tokens gets the matching
// grants, and every problem with such a change comes back in one error with nothing changed. Approval is
// refused while the agent's flow only signs in a user who is present, as the onboarding flow's default
// flow does, and the refusal says why.
func (ts *MCPTestSuite) TestAgentTools_UpdateToActForUsers() {
	token := ts.mcpScopedAdminToken()
	flowID, err := testutils.GetFlowIDByHandle("default-flow", "AUTHENTICATION")
	ts.Require().NoError(err)
	data, _ := ts.onboardAgentForTest(token, fmt.Sprintf("mcp-delegated-agent-%d", time.Now().UnixNano()))
	agentID, _ := data["agentId"].(string)

	refused, raw := ts.callMCPTool(token, "update_agent", map[string]any{
		"agentId": agentID, "credential": "none", "actsForUsers": []string{"approval", "sign_in"},
	})
	ts.Truef(refused.Result.IsError, "the change must be refused: %s", raw)
	ts.Contains(raw, "nothing was changed")
	ts.Contains(raw, "need a credential")
	ts.Contains(raw, "redirectUris")

	noApproval, raw := ts.callMCPTool(token, "update_agent", map[string]any{
		"agentId": agentID, "actsForUsers": []string{"approval"}})
	ts.Truef(noApproval.Result.IsError, "approval must be refused with the default flow: %s", raw)
	ts.Contains(raw, "can't reach a user who isn't there")

	updated, raw := ts.callMCPTool(token, "update_agent", map[string]any{
		"agentId": agentID, "credential": "private_key_jwt", "jwksUri": "https://agent.example.com/jwks",
		"actsForUsers": []string{"sign_in", "token_exchange"}, "redirectUris": []string{"https://agent.example.com/cb"},
		"authFlowId": flowID,
	})
	ts.Require().Falsef(updated.Result.IsError, "update_agent failed: %s", raw)
	ts.Nil(updated.Result.StructuredContent["clientSecret"], "a key-based agent has no secret")
	agent, _ := updated.Result.StructuredContent["changed"].(map[string]any)
	ts.Equal("private_key_jwt", agent["credential"])
	ts.Equal([]any{"sign_in", "token_exchange"}, agent["actsForUsers"])

	stored, err := testutils.GetAgent(agentID)
	ts.Require().NoError(err)
	ts.Require().Len(stored.InboundAuthConfig, 1)
	cfg := stored.InboundAuthConfig[0].Config
	ts.Require().NotNil(cfg)
	ts.Equal("private_key_jwt", cfg.TokenEndpointAuthMethod)
	ts.True(cfg.PKCERequired)
	for _, grant := range []string{"client_credentials", "authorization_code", "refresh_token",
		"urn:ietf:params:oauth:grant-type:token-exchange"} {
		ts.Truef(slices.Contains(cfg.GrantTypes, grant), "missing grant %s in %v", grant, cfg.GrantTypes)
	}
	ts.Equal(flowID, stored.AuthFlowID)
}

// A registered key set is rotated by adding the new key next to the old one, then retiring the old one.
func (ts *MCPTestSuite) TestAgentTools_RotateKeySet() {
	token := ts.mcpScopedAdminToken()
	data, _ := ts.onboardAgentForTest(token, fmt.Sprintf("mcp-keyed-agent-%d", time.Now().UnixNano()))
	agentID, _ := data["agentId"].(string)
	keyed, raw := ts.callMCPTool(token, "update_agent", map[string]any{"agentId": agentID,
		"credential": "private_key_jwt", "jwks": map[string]any{"keys": []any{agentToolsKey("old")}}})
	ts.Require().Falsef(keyed.Result.IsError, "switching to a key set failed: %s", raw)
	agent, _ := keyed.Result.StructuredContent["changed"].(map[string]any)
	ts.Equal([]any{"old"}, agent["keyIds"])

	added, raw := ts.callMCPTool(token, "rotate_agent_credentials", map[string]any{
		"agentId": agentID, "addKey": agentToolsKey("new")})
	ts.Require().Falsef(added.Result.IsError, "adding a key failed: %s", raw)
	ts.Equal([]any{"old", "new"}, added.Result.StructuredContent["keyIds"])

	retired, raw := ts.callMCPTool(token, "rotate_agent_credentials", map[string]any{
		"agentId": agentID, "removeKeyId": "old"})
	ts.Require().Falsef(retired.Result.IsError, "retiring a key failed: %s", raw)
	ts.Equal([]any{"new"}, retired.Result.StructuredContent["keyIds"])

	got, raw := ts.callMCPTool(token, "get_agent", map[string]any{"agentId": agentID})
	ts.Require().Falsef(got.Result.IsError, "get_agent failed: %s", raw)
	ts.Equal([]any{"new"}, got.Result.StructuredContent["keyIds"])
}

// list_agent_types returns the agent types with their attributes marked.
func (ts *MCPTestSuite) TestAgentTools_ListAgentTypes() {
	agentType, raw := ts.defaultAgentType(ts.mcpScopedAdminToken())
	ts.Equal("name", agentType["displayAttribute"])
	attributes, _ := agentType["attributes"].([]any)
	ts.Require().NotEmpty(attributes)
	found := false
	for _, a := range attributes {
		attribute, _ := a.(map[string]any)
		if attribute["name"] == "name" {
			found = true
			ts.Equal(true, attribute["required"])
		}
	}
	ts.True(found, raw)
}

// get_agent's include describes what an agent can do: its groups, each role and whether it holds it
// directly or through a group, and the permissions those roles grant, named by API.
func (ts *MCPTestSuite) TestAgentTools_GetAgentIncludesItsAccess() {
	token := ts.mcpScopedAdminToken()
	suffix := time.Now().UnixNano()
	data, ouID := ts.onboardAgentForTest(token, fmt.Sprintf("mcp-access-agent-%d", suffix))
	agentID, _ := data["agentId"].(string)

	apiIdentifier := fmt.Sprintf("https://mcp-agent-access-%d.example.com", suffix)
	rsID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name: "MCP Agent Access API", Identifier: apiIdentifier, OUID: ouID,
	}, []testutils.Action{{Name: "Read", Handle: "read"}, {Name: "Write", Handle: "write"}})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteResourceServerWithChildren(rsID) })

	groupID, err := testutils.CreateGroup(testutils.Group{Name: fmt.Sprintf("mcp-access-group-%d", suffix),
		OUID: ouID, Members: []testutils.Member{{Id: agentID, Type: "agent"}}})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.DeleteGroup(groupID) })

	directRole := fmt.Sprintf("mcp-access-reader-%d", suffix)
	groupRole := fmt.Sprintf("mcp-access-writer-%d", suffix)
	for name, grant := range map[string]struct {
		permission string
		assignee   testutils.Assignment
	}{
		directRole: {"read", testutils.Assignment{ID: agentID, Type: "agent"}},
		groupRole:  {"write", testutils.Assignment{ID: groupID, Type: "group"}},
	} {
		roleID, err := testutils.CreateRole(testutils.Role{Name: name, OUID: ouID,
			Permissions: []testutils.ResourcePermissions{{ResourceServerID: rsID, Permissions: []string{grant.permission}}},
			Assignments: []testutils.Assignment{grant.assignee}})
		ts.Require().NoError(err)
		ts.T().Cleanup(func() { _ = testutils.DeleteRole(roleID) })
	}

	got, raw := ts.callMCPTool(token, "get_agent", map[string]any{"agentId": agentID,
		"include": []string{"groups", "roles", "permissions"}})
	ts.Require().Falsef(got.Result.IsError, "get_agent failed: %s", raw)
	encoded, err := json.Marshal(got.Result.StructuredContent)
	ts.Require().NoError(err)
	var agent struct {
		Groups []struct {
			ID         string `json:"id"`
			Membership string `json:"membership"`
		} `json:"groups"`
		Roles []struct {
			Name    string `json:"name"`
			Granted string `json:"granted"`
		} `json:"roles"`
		Permissions []struct {
			API         string   `json:"api"`
			Name        string   `json:"name"`
			Permissions []string `json:"permissions"`
		} `json:"permissions"`
		NotIncluded []string `json:"notIncluded"`
	}
	ts.Require().NoError(json.Unmarshal(encoded, &agent))

	ts.Require().Lenf(agent.Groups, 1, "groups: %s", raw)
	ts.Equal(groupID, agent.Groups[0].ID)
	ts.Equal("direct", agent.Groups[0].Membership)
	granted := map[string]string{}
	for _, role := range agent.Roles {
		granted[role.Name] = role.Granted
	}
	ts.Equalf(map[string]string{directRole: "directly", groupRole: "through_group"}, granted, "roles: %s", raw)
	ts.Require().Lenf(agent.Permissions, 1, "permissions: %s", raw)
	ts.Equal(apiIdentifier, agent.Permissions[0].API)
	ts.Equal("MCP Agent Access API", agent.Permissions[0].Name)
	ts.Equal([]string{"read", "write"}, agent.Permissions[0].Permissions)
	ts.Empty(agent.NotIncluded)
}
