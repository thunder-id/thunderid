// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package group

import (
	"context"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/mcp/tool"
)

// groupTools provides MCP tools for managing groups.
type groupTools struct {
	groupService GroupServiceInterface
}

// createGroupInput represents the input for creating a group.
type createGroupInput struct {
	Name        string   `json:"name" jsonschema:"Name of the group"`
	Description string   `json:"description,omitempty" jsonschema:"Description of the group"`
	OUID        string   `json:"ouId" jsonschema:"Organization unit ID"`
	Members     []Member `json:"members,omitempty" jsonschema:"Initial list of members"`
}

// listGroupsInput represents the input for listing groups.
type listGroupsInput struct {
	tool.PaginationInput
	IncludeDisplay bool `json:"includeDisplay,omitempty" jsonschema:"Whether to include member display names"`
}

// updateGroupInput represents the input for updating a group.
type updateGroupInput struct {
	ID          string `json:"id" jsonschema:"The unique identifier of the group"`
	Name        string `json:"name" jsonschema:"Name of the group"`
	Description string `json:"description,omitempty" jsonschema:"Description of the group"`
	OUID        string `json:"ouId" jsonschema:"Organization unit ID"`
}

// groupMemberInput represents the input for adding or removing members from a group.
type groupMemberInput struct {
	GroupID string   `json:"groupId" jsonschema:"The unique identifier of the group"`
	Members []Member `json:"members" jsonschema:"List of members"`
}

// deleteGroupResult represents the output for deleting a group.
type deleteGroupResult struct {
	Success bool `json:"success"`
}

func getCreateGroupSchema() *jsonschema.Schema {
	return tool.GenerateSchema[createGroupInput](
		tool.WithRequired("", "name", "ouId"),
		tool.WithEnum("members", "type", []string{
			string(MemberTypeUser), string(MemberTypeApp), string(MemberTypeAgent), string(MemberTypeGroup),
		}),
	)
}

func getGetGroupSchema() *jsonschema.Schema {
	return tool.GenerateSchema[tool.IDInput](
		tool.WithRequired("", "id"),
	)
}

func getListGroupsSchema() *jsonschema.Schema {
	return tool.GenerateSchema[listGroupsInput]()
}

func getUpdateGroupSchema() *jsonschema.Schema {
	return tool.GenerateSchema[updateGroupInput](
		tool.WithRequired("", "id", "name", "ouId"),
	)
}

func getDeleteGroupSchema() *jsonschema.Schema {
	return tool.GenerateSchema[tool.IDInput](
		tool.WithRequired("", "id"),
	)
}

func getAddGroupMemberSchema() *jsonschema.Schema {
	return tool.GenerateSchema[groupMemberInput](
		tool.WithRequired("", "groupId", "members"),
		tool.WithEnum("members", "type", []string{
			string(MemberTypeUser), string(MemberTypeApp), string(MemberTypeAgent), string(MemberTypeGroup),
		}),
	)
}

func getRemoveGroupMemberSchema() *jsonschema.Schema {
	return tool.GenerateSchema[groupMemberInput](
		tool.WithRequired("", "groupId", "members"),
		tool.WithEnum("members", "type", []string{
			string(MemberTypeUser), string(MemberTypeApp), string(MemberTypeAgent), string(MemberTypeGroup),
		}),
	)
}

// registerMCPTools registers all group MCP tools with the server.
func registerMCPTools(server *mcp.Server, groupService GroupServiceInterface) {
	tools := &groupTools{
		groupService: groupService,
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "thunderid_create_group",
		Description: `Create a new group under an Organization Unit.`,
		InputSchema: getCreateGroupSchema(),
		Annotations: &mcp.ToolAnnotations{
			Title:          "Create Group",
			IdempotentHint: false,
		},
	}, tools.createGroup)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "thunderid_get_group",
		Description: `Retrieve full details of a group by ID including member list.`,
		InputSchema: getGetGroupSchema(),
		Annotations: &mcp.ToolAnnotations{
			Title:        "Get Group",
			ReadOnlyHint: true,
		},
	}, tools.getGroup)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "thunderid_list_groups",
		Description: `List all groups with pagination.`,
		InputSchema: getListGroupsSchema(),
		Annotations: &mcp.ToolAnnotations{
			Title:        "List Groups",
			ReadOnlyHint: true,
		},
	}, tools.listGroups)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "thunderid_update_group",
		Description: `Update an existing group's basic details.`,
		InputSchema: getUpdateGroupSchema(),
		Annotations: &mcp.ToolAnnotations{
			Title:          "Update Group",
			IdempotentHint: true,
		},
	}, tools.updateGroup)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "thunderid_delete_group",
		Description: `Delete a group by ID.`,
		InputSchema: getDeleteGroupSchema(),
		Annotations: &mcp.ToolAnnotations{
			Title:          "Delete Group",
			IdempotentHint: true,
		},
	}, tools.deleteGroup)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "thunderid_add_group_member",
		Description: `Add one or more members (users, apps, agents, or sub-groups) to a group.`,
		InputSchema: getAddGroupMemberSchema(),
		Annotations: &mcp.ToolAnnotations{
			Title:          "Add Group Member",
			IdempotentHint: false,
		},
	}, tools.addGroupMember)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "thunderid_remove_group_member",
		Description: `Remove one or more members from a group.`,
		InputSchema: getRemoveGroupMemberSchema(),
		Annotations: &mcp.ToolAnnotations{
			Title:          "Remove Group Member",
			IdempotentHint: true,
		},
	}, tools.removeGroupMember)
}

// createGroup handles the create_group tool call.
func (t *groupTools) createGroup(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input createGroupInput,
) (*mcp.CallToolResult, *Group, error) {
	req := CreateGroupRequest{
		Name:        input.Name,
		Description: input.Description,
		OUID:        input.OUID,
		Members:     input.Members,
	}
	grp, svcErr := t.groupService.CreateGroup(ctx, req)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to create group: %s", svcErr.ErrorDescription)
	}
	return nil, grp, nil
}

// getGroup handles the get_group tool call.
func (t *groupTools) getGroup(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input tool.IDInput,
) (*mcp.CallToolResult, *Group, error) {
	grp, svcErr := t.groupService.GetGroup(ctx, input.ID, true)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to get group: %s", svcErr.ErrorDescription)
	}
	return nil, grp, nil
}

// listGroups handles the list_groups tool call.
func (t *groupTools) listGroups(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input listGroupsInput,
) (*mcp.CallToolResult, *GroupListResponse, error) {
	limit := input.Limit
	if limit <= 0 {
		limit = serverconst.MaxPageSize
	}
	resp, svcErr := t.groupService.GetGroupList(ctx, limit, input.Offset, input.IncludeDisplay)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to list groups: %s", svcErr.ErrorDescription)
	}
	return nil, resp, nil
}

// updateGroup handles the update_group tool call.
func (t *groupTools) updateGroup(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input updateGroupInput,
) (*mcp.CallToolResult, *Group, error) {
	req := UpdateGroupRequest{
		Name:        input.Name,
		Description: input.Description,
		OUID:        input.OUID,
	}
	grp, svcErr := t.groupService.UpdateGroup(ctx, input.ID, req)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to update group: %s", svcErr.ErrorDescription)
	}
	return nil, grp, nil
}

// deleteGroup handles the delete_group tool call.
func (t *groupTools) deleteGroup(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input tool.IDInput,
) (*mcp.CallToolResult, *deleteGroupResult, error) {
	svcErr := t.groupService.DeleteGroup(ctx, input.ID)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to delete group: %s", svcErr.ErrorDescription)
	}
	return nil, &deleteGroupResult{Success: true}, nil
}

// addGroupMember handles the add_group_member tool call.
func (t *groupTools) addGroupMember(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input groupMemberInput,
) (*mcp.CallToolResult, *Group, error) {
	grp, svcErr := t.groupService.AddGroupMembers(ctx, input.GroupID, input.Members)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to add group members: %s", svcErr.ErrorDescription)
	}
	return nil, grp, nil
}

// removeGroupMember handles the remove_group_member tool call.
func (t *groupTools) removeGroupMember(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input groupMemberInput,
) (*mcp.CallToolResult, *Group, error) {
	grp, svcErr := t.groupService.RemoveGroupMembers(ctx, input.GroupID, input.Members)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to remove group members: %s", svcErr.ErrorDescription)
	}
	return nil, grp, nil
}
