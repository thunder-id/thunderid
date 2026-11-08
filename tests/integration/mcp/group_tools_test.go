// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// createGroupForTest creates a group through the tool and deletes it when the test ends.
func (ts *MCPTestSuite) createGroupForTest(token string, args map[string]any) map[string]any {
	result, raw := ts.callMCPTool(token, "create_group", args)
	ts.Require().Falsef(result.Result.IsError, "create_group failed: %s", raw)
	id, _ := result.Result.StructuredContent["id"].(string)
	ts.Require().NotEmpty(id, raw)
	ts.T().Cleanup(func() { _ = testutils.DeleteGroup(id) })
	return result.Result.StructuredContent
}

// The group tools carry a member between teams over MCP: a group created with a first member lists as
// that member's group, and one call moves the member to another group, the removal revoking what the
// first group gave.
func (ts *MCPTestSuite) TestGroupTools_MoveMemberBetweenTeams() {
	token := ts.mcpScopedAdminToken()
	suffix := time.Now().UnixNano()
	userID := ts.createOperator(fmt.Sprintf("mcp-group-member-%d", suffix), "McpGroupMember@123")
	member := map[string]any{"id": userID, "type": "user"}

	booking := ts.createGroupForTest(token, map[string]any{
		"name": fmt.Sprintf("MCP Booking %d", suffix), "ouId": ts.ouID, "members": []any{member}})
	bookingID, _ := booking["id"].(string)
	ts.Equal(ts.ouID, booking["ouId"])
	ts.EqualValues(1, booking["membersTotal"])
	pricing := ts.createGroupForTest(token, map[string]any{
		"name": fmt.Sprintf("MCP Pricing %d", suffix), "ouId": ts.ouID})
	pricingID, _ := pricing["id"].(string)

	listed, raw := ts.callMCPTool(token, "list_groups", map[string]any{"member": member})
	ts.Require().Falsef(listed.Result.IsError, "list_groups failed: %s", raw)
	groups, _ := listed.Result.StructuredContent["groups"].([]any)
	ts.Require().Lenf(groups, 1, "the member's groups: %s", raw)
	group, _ := groups[0].(map[string]any)
	ts.Equal(bookingID, group["id"])
	ts.Equal(true, group["direct"])

	moved, raw := ts.callMCPTool(token, "update_group_memberships", map[string]any{
		"members": []any{member}, "add": []string{pricingID}, "remove": []string{bookingID}})
	ts.Require().Falsef(moved.Result.IsError, "update_group_memberships failed: %s", raw)
	ts.NotEmpty(moved.Result.StructuredContent["notice"])

	bookingMembers, err := testutils.GetGroupMembers(bookingID)
	ts.Require().NoError(err)
	ts.Empty(bookingMembers, "the removal must take the member out of the booking team")

	got, raw := ts.callMCPTool(token, "get_group", map[string]any{"groupId": pricingID, "include": []string{"members"}})
	ts.Require().Falsef(got.Result.IsError, "get_group failed: %s", raw)
	members, _ := got.Result.StructuredContent["members"].([]any)
	ts.Require().Lenf(members, 1, "the pricing team's members: %s", raw)
	first, _ := members[0].(map[string]any)
	ts.Equal(userID, first["id"])
	ts.Equal("user", first["type"])
}
