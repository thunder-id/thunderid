// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/mcp/tool"
)

type roleTools struct {
	roleService       RoleServiceInterface
	assignmentService RoleAssignmentServiceInterface
}

type roleListMCPResponse struct {
	TotalResults int                   `json:"totalResults"`
	Roles        []RoleSummaryResponse `json:"roles"`
}

type roleUpdateInput struct {
	ID          string                `json:"id"`
	Name        string                `json:"name"`
	Description string                `json:"description,omitempty"`
	OUID        string                `json:"ouId"`
	Permissions []ResourcePermissions `json:"permissions"`
}

type roleAssignmentsInput struct {
	ID             string       `json:"id"`
	Limit          int          `json:"limit,omitempty"`
	Offset         int          `json:"offset,omitempty"`
	IncludeDisplay bool         `json:"includeDisplay,omitempty"`
	Type           AssigneeType `json:"type,omitempty"`
}

type addRoleAssignmentsInput struct {
	ID          string              `json:"id"`
	Assignments []AssignmentRequest `json:"assignments"`
}

type assignmentListMCPResponse struct {
	TotalResults int                  `json:"totalResults"`
	Assignments  []AssignmentResponse `json:"assignments"`
}

type roleActionResponse struct {
	Success bool `json:"success"`
}

// registerMCPTools exposes role and role-assignment operations with their input schemas and safety hints.
func registerMCPTools(
	server *mcp.Server,
	roleService RoleServiceInterface,
	assignmentService RoleAssignmentServiceInterface,
) {
	tools := &roleTools{
		roleService:       roleService,
		assignmentService: assignmentService,
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "thunderid_list_roles",
		Description: "List roles with pagination.",
		InputSchema: tool.GenerateSchema[tool.PaginationInput](
			tool.WithDefault("", "limit", serverconst.DefaultPageSize),
		),
		Annotations: &mcp.ToolAnnotations{
			Title:        "List Roles",
			ReadOnlyHint: true,
		},
	}, tools.listRoles)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "thunderid_get_role",
		Description: "Retrieve a role by ID.",
		InputSchema: tool.GenerateSchema[tool.IDInput](
			tool.WithRequired("", "id"),
		),
		Annotations: &mcp.ToolAnnotations{
			Title:        "Get Role",
			ReadOnlyHint: true,
		},
	}, tools.getRole)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "thunderid_create_role",
		Description: "Create a role with permissions and optional assignments.",
		InputSchema: getCreateRoleSchema(),
		Annotations: &mcp.ToolAnnotations{
			Title:          "Create Role",
			IdempotentHint: false,
		},
	}, tools.createRole)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "thunderid_update_role",
		Description: "Update an existing role using full replacement.",
		InputSchema: getUpdateRoleSchema(),
		Annotations: &mcp.ToolAnnotations{
			Title:          "Update Role",
			IdempotentHint: true,
		},
	}, tools.updateRole)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "thunderid_delete_role",
		Description: "Delete a role by ID.",
		InputSchema: tool.GenerateSchema[tool.IDInput](
			tool.WithRequired("", "id"),
		),
		Annotations: &mcp.ToolAnnotations{
			Title: "Delete Role",
		},
	}, tools.deleteRole)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "thunderid_get_role_assignments",
		Description: "Retrieve entities assigned to a role.",
		InputSchema: getRoleAssignmentsSchema(),
		Annotations: &mcp.ToolAnnotations{
			Title:        "Get Role Assignments",
			ReadOnlyHint: true,
		},
	}, tools.getRoleAssignments)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "thunderid_add_role_assignments",
		Description: "Assign a role to users, applications, agents, or groups.",
		InputSchema: getAddRoleAssignmentsSchema(),
		Annotations: &mcp.ToolAnnotations{
			Title:          "Add Role Assignments",
			IdempotentHint: false,
		},
	}, tools.addRoleAssignments)
}

// listRoles returns a page of role summaries and applies the server default when limit is omitted.
func (t *roleTools) listRoles(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input tool.PaginationInput,
) (*mcp.CallToolResult, *roleListMCPResponse, error) {
	limit := input.Limit
	if limit == 0 {
		limit = serverconst.DefaultPageSize
	}

	roleList, svcErr := t.roleService.GetRoleList(
		ctx,
		limit,
		input.Offset,
	)
	if svcErr != nil {
		return nil, nil, fmt.Errorf(
			"failed to list roles: %s",
			svcErr.ErrorDescription,
		)
	}

	roles := make([]RoleSummaryResponse, len(roleList.Roles))

	for i, role := range roleList.Roles {
		roles[i] = RoleSummaryResponse(role)
	}

	return nil, &roleListMCPResponse{
		TotalResults: roleList.TotalResults,
		Roles:        roles,
	}, nil
}

// getRole returns one role with its organization unit and resource permissions.
func (t *roleTools) getRole(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input tool.IDInput,
) (*mcp.CallToolResult, *RoleResponse, error) {
	serviceRole, svcErr := t.roleService.GetRoleWithPermissions(
		ctx,
		input.ID,
	)
	if svcErr != nil {
		return nil, nil, fmt.Errorf(
			"failed to get role: %s",
			svcErr.ErrorDescription,
		)
	}

	return nil, &RoleResponse{
		ID:          serviceRole.ID,
		Name:        serviceRole.Name,
		Description: serviceRole.Description,
		OUID:        serviceRole.OUID,
		OUHandle:    serviceRole.OUHandle,
		Permissions: serviceRole.Permissions,
	}, nil
}

// createRole creates a role and converts assignment inputs into service-layer assignments.
func (t *roleTools) createRole(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input CreateRoleRequest,
) (*mcp.CallToolResult, *CreateRoleResponse, error) {
	assignments := make([]RoleAssignment, len(input.Assignments))

	for i, assignment := range input.Assignments {
		assignments[i] = RoleAssignment{
			ID:   assignment.ID,
			Type: assignment.Type,
		}
	}

	serviceRole, svcErr := t.roleService.CreateRole(
		ctx,
		RoleCreationDetail{
			Name:        input.Name,
			Description: input.Description,
			OUID:        input.OUID,
			Permissions: input.Permissions,
			Assignments: assignments,
		},
	)
	if svcErr != nil {
		return nil, nil, fmt.Errorf(
			"failed to create role: %s",
			svcErr.ErrorDescription,
		)
	}

	responseAssignments := make(
		[]AssignmentResponse,
		len(serviceRole.Assignments),
	)

	for i, assignment := range serviceRole.Assignments {
		responseAssignments[i] = AssignmentResponse{
			ID:   assignment.ID,
			Type: assignment.Type,
		}
	}

	return nil, &CreateRoleResponse{
		ID:          serviceRole.ID,
		Name:        serviceRole.Name,
		Description: serviceRole.Description,
		OUID:        serviceRole.OUID,
		OUHandle:    serviceRole.OUHandle,
		Permissions: serviceRole.Permissions,
		Assignments: responseAssignments,
	}, nil
}

// updateRole fully replaces a role's details and resource permissions.
func (t *roleTools) updateRole(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input roleUpdateInput,
) (*mcp.CallToolResult, *RoleResponse, error) {
	serviceRole, svcErr := t.roleService.UpdateRoleWithPermissions(
		ctx,
		input.ID,
		RoleUpdateDetail{
			Name:        input.Name,
			Description: input.Description,
			OUID:        input.OUID,
			Permissions: input.Permissions,
		},
	)
	if svcErr != nil {
		return nil, nil, fmt.Errorf(
			"failed to update role: %s",
			svcErr.ErrorDescription,
		)
	}

	return nil, &RoleResponse{
		ID:          serviceRole.ID,
		Name:        serviceRole.Name,
		Description: serviceRole.Description,
		OUID:        serviceRole.OUID,
		OUHandle:    serviceRole.OUHandle,
		Permissions: serviceRole.Permissions,
	}, nil
}

// deleteRole deletes a role and reports whether the mutation completed successfully.
func (t *roleTools) deleteRole(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input tool.IDInput,
) (*mcp.CallToolResult, *roleActionResponse, error) {
	svcErr := t.roleService.DeleteRole(ctx, input.ID)
	if svcErr != nil {
		return nil, nil, fmt.Errorf(
			"failed to delete role: %s",
			svcErr.ErrorDescription,
		)
	}

	return nil, &roleActionResponse{
		Success: true,
	}, nil
}

// getRoleAssignments returns a filtered, paginated assignment list with optional display details.
func (t *roleTools) getRoleAssignments(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input roleAssignmentsInput,
) (*mcp.CallToolResult, *assignmentListMCPResponse, error) {
	limit := input.Limit
	if limit == 0 {
		limit = serverconst.DefaultPageSize
	}

	serviceResponse, svcErr :=
		t.assignmentService.GetRoleAssignmentsByType(
			ctx,
			input.ID,
			limit,
			input.Offset,
			input.IncludeDisplay,
			string(input.Type),
		)

	if svcErr != nil {
		return nil, nil, fmt.Errorf(
			"failed to get role assignments: %s",
			svcErr.ErrorDescription,
		)
	}

	assignments := make(
		[]AssignmentResponse,
		len(serviceResponse.Assignments),
	)

	for i, assignment := range serviceResponse.Assignments {
		assignments[i] = AssignmentResponse{
			ID:      assignment.ID,
			Type:    assignment.Type,
			Display: assignment.Display,
		}
	}

	return nil, &assignmentListMCPResponse{
		TotalResults: serviceResponse.TotalResults,
		Assignments:  assignments,
	}, nil
}

// addRoleAssignments assigns the role to the supplied users, applications, agents, or groups.
func (t *roleTools) addRoleAssignments(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input addRoleAssignmentsInput,
) (*mcp.CallToolResult, *roleActionResponse, error) {
	assignments := make(
		[]RoleAssignment,
		len(input.Assignments),
	)

	for i, assignment := range input.Assignments {
		assignments[i] = RoleAssignment{
			ID:   assignment.ID,
			Type: assignment.Type,
		}
	}

	svcErr := t.assignmentService.AddAssignments(
		ctx,
		input.ID,
		assignments,
	)
	if svcErr != nil {
		return nil, nil, fmt.Errorf(
			"failed to add role assignments: %s",
			svcErr.ErrorDescription,
		)
	}

	return nil, &roleActionResponse{
		Success: true,
	}, nil
}

func getCreateRoleSchema() *jsonschema.Schema {
	return tool.GenerateSchema[CreateRoleRequest](
		tool.WithRequired("", "name", "ouId"),
		tool.WithRequired("assignments", "id", "type"),
		tool.WithEnum(
			"assignments",
			"type",
			[]string{
				string(AssigneeTypeUser),
				string(AssigneeTypeApp),
				string(AssigneeTypeAgent),
				string(AssigneeTypeGroup),
			},
		),
	)
}

func getUpdateRoleSchema() *jsonschema.Schema {
	return tool.GenerateSchema[roleUpdateInput](
		tool.WithRequired(
			"",
			"id",
			"name",
			"ouId",
			"permissions",
		),
	)
}

func getRoleAssignmentsSchema() *jsonschema.Schema {
	return tool.GenerateSchema[roleAssignmentsInput](
		tool.WithRequired("", "id"),
		tool.WithDefault(
			"",
			"limit",
			serverconst.DefaultPageSize,
		),
		tool.WithEnum(
			"",
			"type",
			[]string{
				string(AssigneeTypeUser),
				string(AssigneeTypeApp),
				string(AssigneeTypeAgent),
				string(AssigneeTypeGroup),
			},
		),
	)
}

func getAddRoleAssignmentsSchema() *jsonschema.Schema {
	return tool.GenerateSchema[addRoleAssignmentsInput](
		tool.WithRequired("", "id", "assignments"),
		tool.WithRequired("assignments", "id", "type"),
		tool.WithEnum(
			"assignments",
			"type",
			[]string{
				string(AssigneeTypeUser),
				string(AssigneeTypeApp),
				string(AssigneeTypeAgent),
				string(AssigneeTypeGroup),
			},
		),
	)
}
