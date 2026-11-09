// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"fmt"
	"time"
)

// The user tools find a user by an attribute, describe them without their password, change an
// attribute, and refuse to change the password.
func (ts *MCPTestSuite) TestUserTools_FindDescribeAndUpdate() {
	token := ts.mcpScopedAdminToken()
	username := fmt.Sprintf("mcp-user-%d", time.Now().UnixNano())
	userID := ts.createOperator(username, "McpUserTools@123")

	listed, raw := ts.callMCPTool(token, "list_users", map[string]any{"attribute": "username", "value": username})
	ts.Require().Falsef(listed.Result.IsError, "list_users failed: %s", raw)
	ts.EqualValues(1, listed.Result.StructuredContent["totalResults"], raw)
	ts.Contains(raw, userID)

	got, raw := ts.callMCPTool(token, "get_user", map[string]any{"userId": userID,
		"include": []string{"groups", "roles"}})
	ts.Require().Falsef(got.Result.IsError, "get_user failed: %s", raw)
	ts.NotContains(raw, "McpUserTools@123")
	attributes, _ := got.Result.StructuredContent["attributes"].(map[string]any)
	ts.Equal(username, attributes["username"])
	ts.NotContains(attributes, "password", "a credential is never returned")
	ts.Equal([]any{"permissions"}, got.Result.StructuredContent["notIncluded"])

	renamed := username + "-renamed"
	updated, raw := ts.callMCPTool(token, "update_user", map[string]any{"userId": userID,
		"attributes": map[string]any{"username": renamed}})
	ts.Require().Falsef(updated.Result.IsError, "update_user failed: %s", raw)
	attributes, _ = updated.Result.StructuredContent["attributes"].(map[string]any)
	ts.Equal(renamed, attributes["username"])

	refused, raw := ts.callMCPTool(token, "update_user", map[string]any{"userId": userID,
		"attributes": map[string]any{"password": "Other@12345"}})
	ts.Truef(refused.Result.IsError, "a password change must be refused: %s", raw)
	ts.Contains(raw, "password is a credential")

	message, body := ts.postStatelessMCP(token, "tools/list", "", map[string]any{})
	ts.Contains(message, `"update_user"`, body)
	ts.NotContains(message, `"create_user"`, "no tool creates users")
}
