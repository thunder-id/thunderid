// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"encoding/json"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const roleToolsAPIIdentifier = "https://mcp-role-tools.example.com/orders"

// roleToolsRole is the role as get_role and the role-changing tools return it.
type roleToolsRole struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	OUID        string `json:"ouId"`
	ReadOnly    bool   `json:"readOnly"`
	Permissions []struct {
		API         string   `json:"api"`
		Permissions []string `json:"permissions"`
	} `json:"permissions"`
	Holders *[]struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	} `json:"holders"`
}

// decodeToolResult turns a tool's structured content into out, failing the test when the call failed.
func (ts *MCPTestSuite) decodeToolResult(result mcpToolResult, raw string, out any) {
	ts.Require().Falsef(result.Result.IsError, "tool call failed: %s", raw)
	encoded, err := json.Marshal(result.Result.StructuredContent)
	ts.Require().NoError(err)
	ts.Require().NoError(json.Unmarshal(encoded, out))
}

// rolePermissions returns the permissions the role grants on the test API.
func rolePermissions(role roleToolsRole) []string {
	for _, api := range role.Permissions {
		if api.API == roleToolsAPIIdentifier {
			return api.Permissions
		}
	}
	return nil
}

// The role tools carry a role through its life over MCP: they find the API's permissions, create a
// role with some of them, find it by permission, swap a permission, the role service running its
// permission removal system flow, then grant it to a user and revoke it again, the role service running
// its assignment removal system flow.
func (ts *MCPTestSuite) TestRoleTools_RoleLifecycle() {
	token := ts.mcpScopedAdminToken()

	rsID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name: "MCP Role Tools Orders API", Identifier: roleToolsAPIIdentifier, OUID: ts.ouID,
	}, []testutils.Action{
		{Name: "Read", Handle: "read"},
		{Name: "Delete", Handle: "delete"},
		{Name: "Refund", Handle: "refund"},
	})
	ts.Require().NoError(err, "failed to create the API")
	defer func() { _ = testutils.DeleteResourceServerWithChildren(rsID) }()

	result, raw := ts.callMCPTool(token, "list_api_permissions", map[string]any{"api": roleToolsAPIIdentifier})
	var listed struct {
		Permissions []struct {
			Permission string `json:"permission"`
		} `json:"permissions"`
	}
	ts.decodeToolResult(result, raw, &listed)
	permissions := make([]string, 0, len(listed.Permissions))
	for _, p := range listed.Permissions {
		permissions = append(permissions, p.Permission)
	}
	ts.Require().ElementsMatchf([]string{"read", "delete", "refund"}, permissions, "permissions: %s", raw)

	result, raw = ts.callMCPTool(token, "create_role", map[string]any{
		"name": "MCP Order Support", "ouId": ts.ouID,
		"api": roleToolsAPIIdentifier, "permissions": []string{"read", "delete"},
	})
	var created roleToolsRole
	ts.decodeToolResult(result, raw, &created)
	ts.Require().NotEmpty(created.ID)
	defer func() { _ = testutils.DeleteRole(created.ID) }()
	ts.Equal(ts.ouID, created.OUID)
	ts.ElementsMatch([]string{"read", "delete"}, rolePermissions(created))

	result, raw = ts.callMCPTool(token, "list_roles", map[string]any{
		"api": roleToolsAPIIdentifier, "permission": "delete",
	})
	var found struct {
		TotalResults int `json:"totalResults"`
		Roles        []struct {
			ID string `json:"id"`
		} `json:"roles"`
	}
	ts.decodeToolResult(result, raw, &found)
	ts.Require().Equalf(1, found.TotalResults, "list_roles by permission: %s", raw)
	ts.Equal(created.ID, found.Roles[0].ID)

	result, raw = ts.callMCPTool(token, "update_role_permissions", map[string]any{
		"roleId": created.ID, "api": roleToolsAPIIdentifier,
		"add": []string{"refund"}, "remove": []string{"delete"},
	})
	var changed struct {
		Role    roleToolsRole `json:"role"`
		Added   []string      `json:"added"`
		Removed []string      `json:"removed"`
		Notice  string        `json:"notice"`
	}
	ts.decodeToolResult(result, raw, &changed)
	ts.Equal([]string{"refund"}, changed.Added)
	ts.Equal([]string{"delete"}, changed.Removed)
	ts.NotEmpty(changed.Notice, "a removal says what it revoked")
	ts.ElementsMatch([]string{"read", "refund"}, rolePermissions(changed.Role))

	userID := ts.createOperator("mcp-role-tools-user", "McpRoleTools@123")
	assignee := []map[string]any{{"id": userID, "type": "user"}}

	result, raw = ts.callMCPTool(token, "update_role_assignments", map[string]any{
		"assignees": assignee, "grant": []string{created.ID},
	})
	ts.Require().Falsef(result.Result.IsError, "grant failed: %s", raw)

	result, raw = ts.callMCPTool(token, "get_role", map[string]any{
		"roleId": created.ID, "include": []string{"holders"},
	})
	var held roleToolsRole
	ts.decodeToolResult(result, raw, &held)
	ts.Require().NotNilf(held.Holders, "holders were not included: %s", raw)
	ts.Require().Lenf(*held.Holders, 1, "holders: %s", raw)
	ts.Equal(userID, (*held.Holders)[0].ID)
	ts.Equal("user", (*held.Holders)[0].Type)

	result, raw = ts.callMCPTool(token, "update_role_assignments", map[string]any{
		"assignees": assignee, "revoke": []string{created.ID},
	})
	var revoked struct {
		Revoked []struct {
			RoleID     string `json:"roleId"`
			AssigneeID string `json:"assigneeId"`
		} `json:"revoked"`
	}
	ts.decodeToolResult(result, raw, &revoked)
	ts.Require().Lenf(revoked.Revoked, 1, "revoke: %s", raw)

	assignments, err := testutils.GetRoleAssignments(created.ID)
	ts.Require().NoError(err)
	ts.Empty(assignments, "the revocation must remove the assignment")
}

// roleToolsSecondAPIIdentifier is a second API, so a role can grant permissions on two.
const roleToolsSecondAPIIdentifier = "https://mcp-role-tools.example.com/billing"

// createRoleToolsAPI creates an API with read, delete and refund permissions, and a resource carrying
// an action of its own, and deletes it when the test ends.
func (ts *MCPTestSuite) createRoleToolsAPI(name, identifier string) {
	rsID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name: name, Identifier: identifier, OUID: ts.ouID,
	}, []testutils.Action{
		{Name: "Read", Handle: "read"},
		{Name: "Delete", Handle: "delete"},
		{Name: "Refund", Handle: "refund"},
	})
	ts.Require().NoError(err, "failed to create the API")
	ts.T().Cleanup(func() { _ = testutils.DeleteResourceServerWithChildren(rsID) })

	resourceID, err := testutils.CreateResource(rsID, "Invoices", "invoices", "")
	ts.Require().NoError(err, "failed to create the resource")
	_, err = testutils.CreateResourceAction(rsID, resourceID, testutils.Action{Name: "Export", Handle: "export"})
	ts.Require().NoError(err, "failed to create the resource action")
}

// requireToolError calls a tool that must refuse, and returns its message.
func (ts *MCPTestSuite) requireToolError(token, name string, arguments map[string]any) string {
	result, raw := ts.callMCPTool(token, name, arguments)
	ts.Require().Truef(result.Result.IsError, "%s must refuse %v: %s", name, arguments, raw)
	return raw
}

// The role tools find APIs and roles in the ways the descriptions offer, refuse a duplicate or a name
// already taken, move permissions between APIs, and report what a refused change left done.
func (ts *MCPTestSuite) TestRoleTools_FindRefuseAndReport() {
	token := ts.mcpScopedAdminToken()
	ts.createRoleToolsAPI("MCP Role Tools Orders API", roleToolsAPIIdentifier)
	ts.createRoleToolsAPI("MCP Role Tools Billing API", roleToolsSecondAPIIdentifier)

	result, raw := ts.callMCPTool(token, "list_apis", map[string]any{"name": "mcp role tools"})
	var apis struct {
		APIs []struct {
			API string `json:"api"`
		} `json:"apis"`
	}
	ts.decodeToolResult(result, raw, &apis)
	found := []string{}
	for _, api := range apis.APIs {
		found = append(found, api.API)
	}
	ts.ElementsMatchf([]string{roleToolsSecondAPIIdentifier, roleToolsAPIIdentifier}, found, "list_apis: %s", raw)

	result, raw = ts.callMCPTool(token, "list_api_permissions", map[string]any{"api": roleToolsAPIIdentifier})
	var permissions struct {
		Permissions []struct {
			Name string `json:"name"`
		} `json:"permissions"`
	}
	ts.decodeToolResult(result, raw, &permissions)
	names := []string{}
	for _, p := range permissions.Permissions {
		names = append(names, p.Name)
	}
	ts.Subsetf(names, []string{"Invoices", "Export"}, "a resource and its own actions are listed: %s", raw)
	ts.Contains(ts.requireToolError(token, "list_api_permissions", map[string]any{"api": "https://nowhere.example.com"}),
		"no API was found")

	ts.Contains(ts.requireToolError(token, "create_role", map[string]any{
		"name": "MCP Half Role", "api": roleToolsAPIIdentifier}), "give api and permissions together")
	ts.Contains(ts.requireToolError(token, "create_role", map[string]any{
		"name": "MCP Lost Role", "api": "https://nowhere.example.com", "permissions": []string{"read"}}),
		"no API was found")

	result, raw = ts.callMCPTool(token, "create_role", map[string]any{
		"name": "MCP Order Reader", "ouId": ts.ouID, "api": roleToolsAPIIdentifier, "permissions": []string{"read"},
	})
	var reader roleToolsRole
	ts.decodeToolResult(result, raw, &reader)
	ts.T().Cleanup(func() { _ = testutils.DeleteRole(reader.ID) })

	ts.Contains(ts.requireToolError(token, "create_role", map[string]any{
		"name": "MCP Order Reader Copy", "ouId": ts.ouID, "api": roleToolsAPIIdentifier,
		"permissions": []string{"read"}}), reader.ID, "a role granting the same permissions is named")
	ts.Contains(ts.requireToolError(token, "create_role", map[string]any{
		"name": "MCP Order Reader", "ouId": ts.ouID}), reader.ID, "the role holding the name is named")

	result, raw = ts.callMCPTool(token, "create_role", map[string]any{
		"name": "MCP Order Reader Copy", "ouId": ts.ouID, "api": roleToolsAPIIdentifier,
		"permissions": []string{"read"}, "allowDuplicate": true,
	})
	var copied roleToolsRole
	ts.decodeToolResult(result, raw, &copied)
	ts.T().Cleanup(func() { _ = testutils.DeleteRole(copied.ID) })

	result, raw = ts.callMCPTool(token, "create_role", map[string]any{"name": "MCP Default Unit Role"})
	var inDefault roleToolsRole
	ts.decodeToolResult(result, raw, &inDefault)
	ts.T().Cleanup(func() { _ = testutils.DeleteRole(inDefault.ID) })
	defaultOUID, err := testutils.DefaultOUID()
	ts.Require().NoError(err)
	ts.Equal(defaultOUID, inDefault.OUID, "a role without ouId goes to the Default unit")

	ts.Contains(ts.requireToolError(token, "list_roles", map[string]any{"api": roleToolsAPIIdentifier}),
		"give api and permission together")
	ts.Contains(ts.requireToolError(token, "list_roles", map[string]any{
		"api": "https://nowhere.example.com", "permission": "read"}), "no API was found")

	var listed struct {
		TotalResults int `json:"totalResults"`
		Roles        []struct {
			ID string `json:"id"`
		} `json:"roles"`
	}
	result, raw = ts.callMCPTool(token, "list_roles", map[string]any{"name": "MCP Order Reader", "ouId": ts.ouID})
	ts.decodeToolResult(result, raw, &listed)
	ts.Require().Equalf(1, listed.TotalResults, "list_roles by name in the unit: %s", raw)
	ts.Equal(reader.ID, listed.Roles[0].ID)

	result, raw = ts.callMCPTool(token, "list_roles", map[string]any{"ouId": ts.ouID})
	ts.decodeToolResult(result, raw, &listed)
	inUnit := []string{}
	for _, r := range listed.Roles {
		inUnit = append(inUnit, r.ID)
	}
	ts.Subsetf(inUnit, []string{reader.ID, copied.ID}, "list_roles in the unit: %s", raw)

	result, raw = ts.callMCPTool(token, "get_role", map[string]any{"roleId": reader.ID, "ouId": ts.ouID})
	var described struct {
		NotIncluded []string `json:"notIncluded"`
	}
	ts.decodeToolResult(result, raw, &described)
	ts.Equal([]string{"holders"}, described.NotIncluded)
	ts.requireToolError(token, "get_role", map[string]any{"roleId": "01900000-0000-7000-8000-00000000dead"})

	ts.Contains(ts.requireToolError(token, "update_role_permissions", map[string]any{
		"roleId": reader.ID, "api": roleToolsAPIIdentifier}), "give add, remove, or both")
	ts.Contains(ts.requireToolError(token, "update_role_permissions", map[string]any{
		"roleId": reader.ID, "api": "https://nowhere.example.com", "add": []string{"read"}}), "no API was found")
	ts.requireToolError(token, "update_role_permissions", map[string]any{
		"roleId": "01900000-0000-7000-8000-00000000dead", "api": roleToolsAPIIdentifier, "add": []string{"read"}})

	result, raw = ts.callMCPTool(token, "update_role_permissions", map[string]any{
		"roleId": reader.ID, "api": roleToolsSecondAPIIdentifier, "add": []string{"refund"}})
	var changed struct {
		Role roleToolsRole `json:"role"`
	}
	ts.decodeToolResult(result, raw, &changed)
	ts.Lenf(changed.Role.Permissions, 2, "the role grants on both APIs: %s", raw)

	result, raw = ts.callMCPTool(token, "update_role_permissions", map[string]any{
		"roleId": reader.ID, "api": roleToolsAPIIdentifier, "remove": []string{"read"}})
	ts.decodeToolResult(result, raw, &changed)
	ts.Require().Lenf(changed.Role.Permissions, 1, "only the second API is left: %s", raw)
	ts.Equal(roleToolsSecondAPIIdentifier, changed.Role.Permissions[0].API)

	userID := ts.createOperator("mcp-role-tools-reporter", "McpRoleTools@123")
	assignee := []map[string]any{{"id": userID, "type": "user"}}
	ts.Contains(ts.requireToolError(token, "update_role_assignments", map[string]any{"assignees": assignee}),
		"give assignees, and grant, revoke, or both")
	ts.Contains(ts.requireToolError(token, "update_role_assignments", map[string]any{
		"assignees": assignee, "revoke": []string{reader.ID}}), "Revoked so far: none")

	result, raw = ts.callMCPTool(token, "update_role_assignments", map[string]any{
		"assignees": assignee, "grant": []string{reader.ID}})
	ts.Require().Falsef(result.Result.IsError, "grant failed: %s", raw)
	ts.Contains(ts.requireToolError(token, "update_role_assignments", map[string]any{
		"assignees": assignee, "revoke": []string{reader.ID},
		"grant": []string{"01900000-0000-7000-8000-00000000dead"}}), reader.ID+" for "+userID,
		"a refused grant reports the revocation already made")
}
