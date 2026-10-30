// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"
	"fmt"
	"slices"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/thunder-id/thunderid/internal/system/mcp/tool"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// roleAssigneePageSize caps the assignees the get tool reports for one role.
const roleAssigneePageSize = 100

// roleTools provides MCP tools for managing roles and who holds them. Each tool handles one access task
// and goes through the role services, so validation and authorization are the API's.
type roleTools struct {
	roleService       RoleServiceInterface
	assignmentService RoleAssignmentServiceInterface
}

// roleMCPPermissions is a set of permissions on one resource server.
type roleMCPPermissions struct {
	ResourceServerID string   `json:"resourceServerId" jsonschema:"ID of the resource server defining the permissions"`
	Permissions      []string `json:"permissions" jsonschema:"Permission handles on the resource server"`
}

// roleMCPAssignee is a user, group, agent or application that holds a role.
type roleMCPAssignee struct {
	ID      string `json:"id" jsonschema:"ID of the user, group, agent or application"`
	Type    string `json:"type" jsonschema:"Kind of assignee"`
	Display string `json:"display,omitempty" jsonschema:"Display name, returned only"`
}

// roleListMCPInput is the list tool input.
type roleListMCPInput struct {
	tool.PaginationInput
}

// roleMCPItem is the list tool item.
type roleMCPItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	OUID        string `json:"ouId"`
	OUHandle    string `json:"ouHandle,omitempty"`
	ReadOnly    bool   `json:"readOnly"`
}

// roleListMCPResponse is the list tool output.
type roleListMCPResponse struct {
	TotalResults int           `json:"totalResults"`
	Roles        []roleMCPItem `json:"roles"`
}

// roleMCPOutput describes a role: what it grants and, from the get and create tools, who holds it.
type roleMCPOutput struct {
	ID             string               `json:"id"`
	Name           string               `json:"name"`
	Description    string               `json:"description,omitempty"`
	OUID           string               `json:"ouId"`
	OUHandle       string               `json:"ouHandle,omitempty"`
	Permissions    []roleMCPPermissions `json:"permissions"`
	Assignees      []roleMCPAssignee    `json:"assignees,omitempty"`
	TotalAssignees int                  `json:"totalAssignees,omitempty"`
}

// roleCreateMCPInput is the create tool input.
//
//nolint:lll
type roleCreateMCPInput struct {
	Name        string               `json:"name" jsonschema:"Role name, unique within the organization unit, at most 100 characters"`
	Description string               `json:"description,omitempty" jsonschema:"What the role is for"`
	OUID        string               `json:"ouId" jsonschema:"ID of the organization unit the role belongs to"`
	Permissions []roleMCPPermissions `json:"permissions,omitempty" jsonschema:"Permissions the role grants, grouped by resource server"`
	Assignees   []roleMCPAssignee    `json:"assignees,omitempty" jsonschema:"Users, groups, agents or applications that hold the role from the start"`
}

// roleAssigneesMCPInput is the grant and revoke tool input.
type roleAssigneesMCPInput struct {
	ID        string            `json:"id" jsonschema:"ID of the role"`
	Assignees []roleMCPAssignee `json:"assignees" jsonschema:"Users, groups, agents or applications"`
}

// roleAssigneesMCPResponse is the grant and revoke tool output.
type roleAssigneesMCPResponse struct {
	ID        string            `json:"id"`
	Assignees []roleMCPAssignee `json:"assignees"`
}

// rolePermissionsMCPInput is the change permissions tool input.
type rolePermissionsMCPInput struct {
	ID     string               `json:"id" jsonschema:"ID of the role"`
	Add    []roleMCPPermissions `json:"add,omitempty" jsonschema:"Permissions the role starts granting"`
	Remove []roleMCPPermissions `json:"remove,omitempty" jsonschema:"Permissions the role stops granting"`
}

// roleDeleteMCPResponse is the delete tool output.
type roleDeleteMCPResponse struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// registerMCPTools registers the role MCP tools with the server.
func registerMCPTools(server *mcp.Server, roleService RoleServiceInterface,
	assignmentService RoleAssignmentServiceInterface) {
	tools := &roleTools{roleService: roleService, assignmentService: assignmentService}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_roles",
		Description: `List roles with their organization units. Declarative roles are read-only.`,
		Annotations: &mcp.ToolAnnotations{
			Title:        "List Roles",
			ReadOnlyHint: true,
		},
	}, tools.listRoles)

	mcp.AddTool(server, &mcp.Tool{
		Name: "get_role",
		Description: `Describe a role: the permissions it grants on each resource server and the users, ` +
			`groups, agents and applications that hold it.`,
		Annotations: &mcp.ToolAnnotations{
			Title:        "Get Role",
			ReadOnlyHint: true,
		},
	}, tools.getRole)

	mcp.AddTool(server, &mcp.Tool{
		Name: "create_role",
		Description: `Create a role that grants a set of permissions, and optionally give it to its first ` +
			`holders in the same step. Permissions are grouped by resource server and must exist on it.`,
		InputSchema: getCreateRoleSchema(),
		Annotations: &mcp.ToolAnnotations{
			Title:          "Create Role",
			IdempotentHint: false,
		},
	}, tools.createRole)

	mcp.AddTool(server, &mcp.Tool{
		Name: "grant_role",
		Description: `Give a role to users, groups, agents or applications. A group passes the role to ` +
			`its members.`,
		InputSchema: getRoleAssigneesSchema(),
		Annotations: &mcp.ToolAnnotations{
			Title:          "Grant Role",
			IdempotentHint: true,
		},
	}, tools.grantRole)

	mcp.AddTool(server, &mcp.Tool{
		Name: "revoke_role",
		Description: `Take a role away from users, groups, agents or applications. Tokens already issued ` +
			`keep the role's permissions until they expire.`,
		InputSchema: getRoleAssigneesSchema(),
		Annotations: &mcp.ToolAnnotations{
			Title:           "Revoke Role",
			IdempotentHint:  true,
			DestructiveHint: new(true),
		},
	}, tools.revokeRole)

	mcp.AddTool(server, &mcp.Tool{
		Name: "change_role_permissions",
		Description: `Add permissions to a role or remove them from it. Only the listed permissions change; ` +
			`the role's other permissions, name and holders are kept. Removing permissions affects every ` +
			`holder; tokens already issued keep them until they expire.`,
		Annotations: &mcp.ToolAnnotations{
			Title:           "Change Role Permissions",
			IdempotentHint:  true,
			DestructiveHint: new(true),
		},
	}, tools.changeRolePermissions)

	mcp.AddTool(server, &mcp.Tool{
		Name: "delete_role",
		Description: `Permanently delete a role and take it away from everyone who holds it. This cannot ` +
			`be undone. Confirm with the user first.`,
		Annotations: &mcp.ToolAnnotations{
			Title:           "Delete Role",
			IdempotentHint:  true,
			DestructiveHint: new(true),
		},
	}, tools.deleteRole)
}

// listRoles handles the list_roles tool call.
func (t *roleTools) listRoles(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input roleListMCPInput,
) (*mcp.CallToolResult, *roleListMCPResponse, error) {
	limit := input.Limit
	if limit == 0 {
		limit = 20
	}
	list, svcErr := t.roleService.GetRoleList(ctx, limit, input.Offset)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to list roles: %s", svcErr.ErrorDescription)
	}

	items := make([]roleMCPItem, len(list.Roles))
	for i, role := range list.Roles {
		items[i] = roleMCPItem{
			ID:          role.ID,
			Name:        role.Name,
			Description: role.Description,
			OUID:        role.OUID,
			OUHandle:    role.OUHandle,
			ReadOnly:    role.IsReadOnly,
		}
	}
	return nil, &roleListMCPResponse{TotalResults: list.TotalResults, Roles: items}, nil
}

// getRole handles the get_role tool call.
func (t *roleTools) getRole(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input tool.IDInput,
) (*mcp.CallToolResult, *roleMCPOutput, error) {
	const failure = "failed to get role"
	role, svcErr := t.roleService.GetRoleWithPermissions(ctx, input.ID)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("%s: %s", failure, svcErr.ErrorDescription)
	}
	assignments, svcErr := t.assignmentService.GetRoleAssignments(ctx, input.ID, roleAssigneePageSize, 0, true)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("%s: %s", failure, svcErr.ErrorDescription)
	}

	out := roleOutput(role.ID, role.Name, role.Description, role.OUID, role.OUHandle, role.Permissions)
	out.Assignees = make([]roleMCPAssignee, len(assignments.Assignments))
	for i, assignment := range assignments.Assignments {
		out.Assignees[i] = roleMCPAssignee{
			ID: assignment.ID, Type: string(assignment.Type), Display: assignment.Display,
		}
	}
	out.TotalAssignees = assignments.TotalResults
	return nil, out, nil
}

// createRole handles the create_role tool call.
func (t *roleTools) createRole(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input roleCreateMCPInput,
) (*mcp.CallToolResult, *roleMCPOutput, error) {
	role, svcErr := t.roleService.CreateRole(ctx, RoleCreationDetail{
		Name:        sysutils.SanitizeString(input.Name),
		Description: sysutils.SanitizeString(input.Description),
		OUID:        input.OUID,
		Permissions: toolPermissions(input.Permissions),
		Assignments: toRoleAssignments(input.Assignees),
	})
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to create role: %s", svcErr.ErrorDescription)
	}

	out := roleOutput(role.ID, role.Name, role.Description, role.OUID, role.OUHandle, role.Permissions)
	out.Assignees = make([]roleMCPAssignee, len(role.Assignments))
	for i, assignment := range role.Assignments {
		out.Assignees[i] = roleMCPAssignee{ID: assignment.ID, Type: string(assignment.Type)}
	}
	out.TotalAssignees = len(role.Assignments)
	return nil, out, nil
}

// grantRole handles the grant_role tool call.
func (t *roleTools) grantRole(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input roleAssigneesMCPInput,
) (*mcp.CallToolResult, *roleAssigneesMCPResponse, error) {
	if svcErr := t.assignmentService.AddAssignments(ctx, input.ID, toRoleAssignments(input.Assignees)); svcErr != nil {
		return nil, nil, fmt.Errorf("failed to grant role: %s", svcErr.ErrorDescription)
	}
	return nil, &roleAssigneesMCPResponse{ID: input.ID, Assignees: input.Assignees}, nil
}

// revokeRole handles the revoke_role tool call.
func (t *roleTools) revokeRole(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input roleAssigneesMCPInput,
) (*mcp.CallToolResult, *roleAssigneesMCPResponse, error) {
	if svcErr := t.assignmentService.RemoveAssignments(
		ctx, input.ID, toRoleAssignments(input.Assignees)); svcErr != nil {
		return nil, nil, fmt.Errorf("failed to revoke role: %s", svcErr.ErrorDescription)
	}
	return nil, &roleAssigneesMCPResponse{ID: input.ID, Assignees: input.Assignees}, nil
}

// changeRolePermissions handles the change_role_permissions tool call. It reads the role, applies the
// additions and removals to its permissions and writes the role back with its name and organization
// unit unchanged.
func (t *roleTools) changeRolePermissions(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input rolePermissionsMCPInput,
) (*mcp.CallToolResult, *roleMCPOutput, error) {
	const failure = "failed to change role permissions"
	if len(input.Add) == 0 && len(input.Remove) == 0 {
		return nil, nil, fmt.Errorf("%s: give the permissions to add or remove", failure)
	}
	role, svcErr := t.roleService.GetRoleWithPermissions(ctx, input.ID)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("%s: %s", failure, svcErr.ErrorDescription)
	}

	updated, svcErr := t.roleService.UpdateRoleWithPermissions(ctx, input.ID, RoleUpdateDetail{
		Name:        role.Name,
		Description: role.Description,
		OUID:        role.OUID,
		Permissions: changePermissions(role.Permissions,
			toolPermissions(input.Add), toolPermissions(input.Remove)),
	})
	if svcErr != nil {
		return nil, nil, fmt.Errorf("%s: %s", failure, svcErr.ErrorDescription)
	}
	return nil, roleOutput(updated.ID, updated.Name, updated.Description, updated.OUID, updated.OUHandle,
		updated.Permissions), nil
}

// deleteRole handles the delete_role tool call.
func (t *roleTools) deleteRole(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input tool.IDInput,
) (*mcp.CallToolResult, *roleDeleteMCPResponse, error) {
	if svcErr := t.roleService.DeleteRole(ctx, input.ID); svcErr != nil {
		return nil, nil, fmt.Errorf("failed to delete role: %s", svcErr.ErrorDescription)
	}
	return nil, &roleDeleteMCPResponse{ID: input.ID, Deleted: true}, nil
}

// changePermissions returns the permissions with the additions merged in and the removals taken out, in
// a stable order. A resource server left with no permissions is dropped.
func changePermissions(current, add, remove []ResourcePermissions) []ResourcePermissions {
	var order []string
	byServer := map[string][]string{}
	for _, set := range slices.Concat(current, add) {
		if _, seen := byServer[set.ResourceServerID]; !seen {
			order = append(order, set.ResourceServerID)
		}
		for _, permission := range set.Permissions {
			if !slices.Contains(byServer[set.ResourceServerID], permission) {
				byServer[set.ResourceServerID] = append(byServer[set.ResourceServerID], permission)
			}
		}
	}
	for _, set := range remove {
		byServer[set.ResourceServerID] = slices.DeleteFunc(byServer[set.ResourceServerID],
			func(permission string) bool { return slices.Contains(set.Permissions, permission) })
	}

	changed := make([]ResourcePermissions, 0, len(order))
	for _, serverID := range order {
		if len(byServer[serverID]) > 0 {
			changed = append(changed, ResourcePermissions{ResourceServerID: serverID, Permissions: byServer[serverID]})
		}
	}
	return changed
}

// toolPermissions maps tool permissions to the service model, trimming each value as the API does.
func toolPermissions(sets []roleMCPPermissions) []ResourcePermissions {
	permissions := make([]ResourcePermissions, len(sets))
	for i, set := range sets {
		values := make([]string, len(set.Permissions))
		for j, permission := range set.Permissions {
			values[j] = sysutils.SanitizeString(permission)
		}
		permissions[i] = ResourcePermissions{
			ResourceServerID: sysutils.SanitizeString(set.ResourceServerID),
			Permissions:      values,
		}
	}
	return permissions
}

// toRoleAssignments maps tool assignees to the service model.
func toRoleAssignments(assignees []roleMCPAssignee) []RoleAssignment {
	assignments := make([]RoleAssignment, len(assignees))
	for i, assignee := range assignees {
		assignments[i] = RoleAssignment{ID: sysutils.SanitizeString(assignee.ID), Type: AssigneeType(assignee.Type)}
	}
	return assignments
}

// roleOutput builds the tool output from a role's fields.
func roleOutput(id, name, description, ouID, ouHandle string, permissions []ResourcePermissions) *roleMCPOutput {
	sets := make([]roleMCPPermissions, len(permissions))
	for i, set := range permissions {
		sets[i] = roleMCPPermissions(set)
	}
	return &roleMCPOutput{
		ID: id, Name: name, Description: description, OUID: ouID, OUHandle: ouHandle, Permissions: sets,
	}
}

// assigneeTypes are the kinds of assignee a role can be given to.
func assigneeTypes() []string {
	return []string{string(AssigneeTypeUser), string(AssigneeTypeGroup), string(AssigneeTypeAgent),
		string(AssigneeTypeApp)}
}

// getCreateRoleSchema generates the schema for the create_role tool.
func getCreateRoleSchema() *jsonschema.Schema {
	return tool.GenerateSchema[roleCreateMCPInput](
		tool.WithEnum("assignees", "type", assigneeTypes()),
		tool.WithRemove("assignees", "display"),
	)
}

// getRoleAssigneesSchema generates the schema for the grant_role and revoke_role tools.
func getRoleAssigneesSchema() *jsonschema.Schema {
	return tool.GenerateSchema[roleAssigneesMCPInput](
		tool.WithEnum("assignees", "type", assigneeTypes()),
		tool.WithRemove("assignees", "display"),
	)
}
