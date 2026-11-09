// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	oupkg "github.com/thunder-id/thunderid/internal/ou"
	resourcepkg "github.com/thunder-id/thunderid/internal/resource"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/filter"
	"github.com/thunder-id/thunderid/internal/system/mcp/tool"
	"github.com/thunder-id/thunderid/internal/system/security"
)

// roleTools carries the services the role MCP tools call.
type roleTools struct {
	roleService       RoleServiceInterface
	assignmentService RoleAssignmentServiceInterface
	resourceService   resourcepkg.ResourceServiceInterface
	ouService         oupkg.OrganizationUnitServiceInterface
	// revokesOnRemoval is false on a server that issues no tokens, where a removal revokes nothing.
	revokesOnRemoval bool
}

// listRolesMCPInput is the input of the list_roles tool.
//
//nolint:lll
type listRolesMCPInput struct {
	Name       string `json:"name,omitempty" jsonschema:"Exact name of the role"`
	API        string `json:"api,omitempty" jsonschema:"Identifier of an API, such as https://api.example.com/orders. With permission, lists the roles that grant that permission on it."`
	Permission string `json:"permission,omitempty" jsonschema:"A permission on the API given in api, from list_api_permissions"`
	OUID       string `json:"ouId,omitempty" jsonschema:"ID of an organization unit, to list the roles it owns and then the ones shared with it"`
	Limit      int    `json:"limit,omitempty" jsonschema:"Maximum number of roles to return, at most 100, and 30 when not given. Only without name, api and permission: a search returns every match."`
	Offset     int    `json:"offset,omitempty" jsonschema:"Number of roles to skip. Only without name, api and permission."`
}

// getRoleMCPInput is the input of the get_role tool.
//
//nolint:lll
type getRoleMCPInput struct {
	RoleID  string   `json:"roleId" jsonschema:"ID of the role, from list_roles or create_role"`
	OUID    string   `json:"ouId,omitempty" jsonschema:"ID of an organization unit the role is shared with, to read it as that unit sees it"`
	Include []string `json:"include,omitempty" jsonschema:"Details to add. holders: the users, apps, agents and groups the role is assigned to."`
	Limit   int      `json:"limit,omitempty" jsonschema:"Maximum number of holders to return, at most 100"`
	Offset  int      `json:"offset,omitempty" jsonschema:"Number of holders to skip"`
}

// listAPIPermissionsMCPInput is the input of the list_api_permissions tool.
type listAPIPermissionsMCPInput struct {
	API string `json:"api" jsonschema:"Identifier of the API, such as https://api.example.com/orders"`
}

// listAPIsMCPInput is the input of the list_apis tool.
//
//nolint:lll
type listAPIsMCPInput struct {
	Name   string `json:"name,omitempty" jsonschema:"Part of the API's name or identifier, in any case, such as orders"`
	Limit  int    `json:"limit,omitempty" jsonschema:"Maximum number of APIs to return, at most 100, and 30 when not given. Only without name: a name search returns every match."`
	Offset int    `json:"offset,omitempty" jsonschema:"Number of APIs to skip. Only without name."`
}

// apiMCPItem is one API in a list.
type apiMCPItem struct {
	API         string `json:"api"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	OUID        string `json:"ouId"`
}

// apiListMCPResponse is the list_apis output.
type apiListMCPResponse struct {
	TotalResults int          `json:"totalResults"`
	APIs         []apiMCPItem `json:"apis"`
}

// roleAssigneeMCPInput is a user, app, agent or group a role is granted to or revoked from.
type roleAssigneeMCPInput struct {
	ID   string `json:"id" jsonschema:"ID of the user, app, agent or group"`
	Type string `json:"type" jsonschema:"What the ID names: user, app, agent or group"`
}

// createRoleMCPInput is the input of the create_role tool.
//
//nolint:lll
type createRoleMCPInput struct {
	Name        string                 `json:"name" jsonschema:"Name of the role, unique in its organization unit"`
	Description string                 `json:"description,omitempty" jsonschema:"What the role is for"`
	OUID        string                 `json:"ouId,omitempty" jsonschema:"ID of the organization unit that owns the role. Defaults to the Default organization unit."`
	API         string                 `json:"api,omitempty" jsonschema:"Identifier of the API the permissions are on, such as https://api.example.com/orders"`
	Permissions []string               `json:"permissions,omitempty" jsonschema:"Permissions on the API to grant, from list_api_permissions"`
	Assignees   []roleAssigneeMCPInput `json:"assignees,omitempty" jsonschema:"Users, apps, agents and groups to assign the role to when it is created"`
	// AllowDuplicate lets a second role grant exactly what one in the unit already grants.
	AllowDuplicate bool `json:"allowDuplicate,omitempty" jsonschema:"Create the role even when one in the organization unit already grants exactly these permissions. Leave it out to be told about that role instead."`
}

// updateRolePermissionsMCPInput is the input of the update_role_permissions tool.
//
//nolint:lll
type updateRolePermissionsMCPInput struct {
	RoleID string   `json:"roleId" jsonschema:"ID of the role, from list_roles"`
	API    string   `json:"api" jsonschema:"Identifier of the API the permissions are on, such as https://api.example.com/orders"`
	Add    []string `json:"add,omitempty" jsonschema:"Permissions on the API to add, from list_api_permissions"`
	Remove []string `json:"remove,omitempty" jsonschema:"Permissions on the API to remove"`
}

// updateRoleAssignmentsMCPInput is the input of the update_role_assignments tool.
//
//nolint:lll
type updateRoleAssignmentsMCPInput struct {
	Assignees []roleAssigneeMCPInput `json:"assignees" jsonschema:"Users, apps, agents and groups whose roles change. Each gets every role in grant and loses every role in revoke."`
	Grant     []string               `json:"grant,omitempty" jsonschema:"IDs of roles to grant, from list_roles"`
	Revoke    []string               `json:"revoke,omitempty" jsonschema:"IDs of roles to revoke, from list_roles or get_role"`
	OUID      string                 `json:"ouId,omitempty" jsonschema:"ID of the organization unit the assignments are made in. Defaults to each role's own organization unit."`
}

// roleMCPItem is a role in the list_roles output.
type roleMCPItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	OUID        string `json:"ouId"`
	ReadOnly    bool   `json:"readOnly"`
}

// roleListMCPResponse is the list_roles output.
type roleListMCPResponse struct {
	TotalResults int           `json:"totalResults"`
	Roles        []roleMCPItem `json:"roles"`
}

// roleAPIPermissionsMCPItem is what a role grants on one API.
type roleAPIPermissionsMCPItem struct {
	API         string   `json:"api"`
	Name        string   `json:"name,omitempty"`
	Permissions []string `json:"permissions"`
}

// roleHolderMCPItem is a user, app, agent or group a role is assigned to.
type roleHolderMCPItem struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Name     string `json:"name,omitempty"`
	OUID     string `json:"ouId,omitempty"`
	Mandated bool   `json:"mandated"`
}

// roleMCPDetails is one role, with its permissions grouped by API.
type roleMCPDetails struct {
	ID           string                      `json:"id"`
	Name         string                      `json:"name"`
	Description  string                      `json:"description,omitempty"`
	OUID         string                      `json:"ouId"`
	ReadOnly     bool                        `json:"readOnly"`
	Permissions  []roleAPIPermissionsMCPItem `json:"permissions"`
	Holders      *[]roleHolderMCPItem        `json:"holders,omitempty"`
	HoldersTotal int                         `json:"holdersTotal,omitempty"`
	NotIncluded  []string                    `json:"notIncluded,omitempty"`
}

// apiPermissionMCPItem is one permission an API offers.
type apiPermissionMCPItem struct {
	Permission  string `json:"permission"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// apiPermissionsMCPResponse is the list_api_permissions output.
type apiPermissionsMCPResponse struct {
	API          string                 `json:"api"`
	Name         string                 `json:"name"`
	TotalResults int                    `json:"totalResults"`
	Permissions  []apiPermissionMCPItem `json:"permissions"`
}

// updateRolePermissionsMCPResponse is the update_role_permissions output.
type updateRolePermissionsMCPResponse struct {
	Role    roleMCPDetails `json:"role"`
	Added   []string       `json:"added"`
	Removed []string       `json:"removed"`
	Notice  string         `json:"notice,omitempty"`
}

// roleAssignmentChangeMCPItem is one role granted to or revoked from one assignee.
type roleAssignmentChangeMCPItem struct {
	RoleID     string `json:"roleId"`
	AssigneeID string `json:"assigneeId"`
}

// updateRoleAssignmentsMCPResponse is the update_role_assignments output.
type updateRoleAssignmentsMCPResponse struct {
	Revoked []roleAssignmentChangeMCPItem `json:"revoked"`
	Granted []roleAssignmentChangeMCPItem `json:"granted"`
	Notice  string                        `json:"notice,omitempty"`
}

// revokingRemovalNotice describes what removing access does on a server that issues tokens.
const revokingRemovalNotice = "Removing a permission or revoking a role revokes the tokens that carried the " +
	"removed access."

// directRemovalNotice describes what removing access does on a server that issues no tokens, such as a
// control plane.
const directRemovalNotice = "This server issues no tokens, so removals are written directly, " +
	"without the token revocation a data plane performs."

// removalNotice describes what a removal does on this server.
func (t *roleTools) removalNotice() string {
	if !t.revokesOnRemoval {
		return directRemovalNotice
	}
	return revokingRemovalNotice
}

// RegisterMCPTools registers the role MCP tools with the server. revokesOnRemoval is false on a server
// that issues no tokens, where a removal revokes nothing.
func RegisterMCPTools(server *mcp.Server, roleService RoleServiceInterface,
	assignmentService RoleAssignmentServiceInterface, resourceService resourcepkg.ResourceServiceInterface,
	ouService oupkg.OrganizationUnitServiceInterface, revokesOnRemoval bool) {
	tools := &roleTools{
		roleService:       roleService,
		assignmentService: assignmentService,
		resourceService:   resourceService,
		ouService:         ouService,
		revokesOnRemoval:  revokesOnRemoval,
	}
	assigneeTypes := []string{string(AssigneeTypeUser), string(AssigneeTypeApp), string(AssigneeTypeAgent),
		string(AssigneeTypeGroup)}

	mcp.AddTool(server, &mcp.Tool{
		Name: "list_roles",
		Description: `Find roles. With api, the identifier of an API (a resource server, as list_apis returns ` +
			`it), and permission, one of the permissions defined on it (as list_api_permissions returns them), ` +
			`lists the roles that grant that permission, so you can reuse one instead of creating a ` +
			`near-duplicate. name matches a role's exact name. ouId lists the roles an organization unit ` +
			`owns, then the ones shared with it. name and ouId narrow a permission search. A search by name ` +
			`or permission returns every match. Without one, the roles are paged with limit and offset, and ` +
			`totalResults counts them all. Returns each role's ID, name, organization unit and whether it ` +
			`is read-only. Use get_role for what a role grants and who holds it.`,
		InputSchema: tool.GenerateSchema[listRolesMCPInput](),
		Annotations: &mcp.ToolAnnotations{Title: "List Roles", ReadOnlyHint: true, OpenWorldHint: new(false)},
	}, tools.listRoles)

	mcp.AddTool(server, &mcp.Tool{
		Name: "get_role",
		Description: `Get one role: its permissions grouped by API, and whether it is read-only. With include ` +
			`holders, also the users, apps, agents and groups it is assigned to, a page at a time with limit and ` +
			`offset, and holdersTotal counting them all. Members of a holder group ` +
			`hold the role through it. notIncluded lists the sections left out, so a missing section doesn't ` +
			`mean the role has none. Check the holders before changing or revoking a role others depend on.`,
		InputSchema: tool.GenerateSchema[getRoleMCPInput](
			tool.WithEnum("", "include", []string{roleDetailHolders})),
		Annotations: &mcp.ToolAnnotations{Title: "Get Role", ReadOnlyHint: true, OpenWorldHint: new(false)},
	}, tools.getRole)

	mcp.AddTool(server, &mcp.Tool{
		Name: "list_apis",
		Description: `Find the APIs whose permissions roles grant, such as "the orders API". name matches ` +
			`part of an API's name or identifier in any case. Returns each API's identifier, which ` +
			`list_api_permissions, list_roles, create_role and update_role_permissions take. A name search ` +
			`returns every match. Without name, the APIs are paged with limit and offset, and totalResults ` +
			`counts them all.`,
		InputSchema: tool.GenerateSchema[listAPIsMCPInput](),
		Annotations: &mcp.ToolAnnotations{Title: "List APIs", ReadOnlyHint: true, OpenWorldHint: new(false)},
	}, tools.listAPIs)

	mcp.AddTool(server, &mcp.Tool{
		Name: "list_api_permissions",
		Description: `List the permissions an API offers, by its identifier, such as ` +
			`https://api.example.com/orders; find the identifier with list_apis. Use these exact strings in ` +
			`create_role and update_role_permissions instead of guessing them. Returns every permission, and ` +
			`totalResults counts them.`,
		InputSchema: tool.GenerateSchema[listAPIPermissionsMCPInput](),
		Annotations: &mcp.ToolAnnotations{
			Title: "List API Permissions", ReadOnlyHint: true, OpenWorldHint: new(false)},
	}, tools.listAPIPermissions)

	mcp.AddTool(server, &mcp.Tool{
		Name: "create_role",
		Description: `Create a role that grants permissions on an API, and optionally assign it right away. ` +
			`Permissions are strings from list_api_permissions. If a role in the organization unit already ` +
			`grants exactly these permissions, or has this name, nothing is created and the error names that ` +
			`role, to grant with update_role_assignments instead. Without ouId, the role goes in the Default ` +
			`organization unit. Use update_role_assignments to grant it later.`,
		InputSchema: tool.GenerateSchema[createRoleMCPInput](
			tool.WithEnum("assignees", "type", assigneeTypes)),
		Annotations: &mcp.ToolAnnotations{
			Title: "Create Role", DestructiveHint: new(false), IdempotentHint: false, OpenWorldHint: new(false)},
	}, tools.createRole)

	mcp.AddTool(server, &mcp.Tool{
		Name: "update_role_permissions",
		Description: `Add and remove a role's permissions on one API, without resending the rest. It changes ` +
			`access for every holder, so check them with get_role first. Removals run first, and nothing is ` +
			`added if a removal is refused. ` + tools.removalNotice(),
		InputSchema: tool.GenerateSchema[updateRolePermissionsMCPInput](),
		Annotations: &mcp.ToolAnnotations{
			Title: "Update Role Permissions", DestructiveHint: new(true), IdempotentHint: true,
			OpenWorldHint: new(false)},
	}, tools.updateRolePermissions)

	mcp.AddTool(server, &mcp.Tool{
		Name: "update_role_assignments",
		Description: `Grant and revoke roles for users, apps, agents and groups. Each assignee gets every role ` +
			`in grant and loses every role in revoke, so one call moves someone from one role to another. ` +
			`Grant a team a role by granting it to their group. Revocations run first, and nothing is granted ` +
			`if one is refused. A role mandated by a sharing policy can't be revoked here. ` + tools.removalNotice(),
		InputSchema: tool.GenerateSchema[updateRoleAssignmentsMCPInput](
			tool.WithEnum("assignees", "type", assigneeTypes)),
		Annotations: &mcp.ToolAnnotations{
			Title: "Update Role Assignments", DestructiveHint: new(true), IdempotentHint: true,
			OpenWorldHint: new(false)},
	}, tools.updateRoleAssignments)
}

// listRoles handles the list_roles tool call.
func (t *roleTools) listRoles(ctx context.Context, _ *mcp.CallToolRequest, input listRolesMCPInput) (
	*mcp.CallToolResult, *roleListMCPResponse, error) {
	if (input.API == "") != (input.Permission == "") {
		return nil, nil, errors.New("failed to list roles: give api and permission together")
	}

	if input.API != "" || input.Name != "" {
		// The searches the role service offers by name and by permission aren't paged, so a search
		// returns every match.
		if input.Limit != 0 || input.Offset != 0 {
			return nil, nil, errors.New("failed to list roles: a search by name or permission returns every " +
				"match, so limit and offset can't be used with it")
		}
		// With an organization unit, the unit's own listing decides which roles are returned, and it holds
		// the roles shared into the unit, which the caller may not read where they are owned. So the search
		// runs without the caller's authorization.
		searchCtx := ctx
		if input.OUID != "" {
			searchCtx = security.WithRuntimeContext(ctx)
		}
		var roles []Role
		if input.API != "" {
			server, err := t.apiByIdentifier(ctx, input.API)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to list roles: %w", err)
			}
			found, svcErr := t.roleService.GetRolesByPermission(searchCtx, server.ID, input.Permission)
			if svcErr != nil {
				return nil, nil, fmt.Errorf("failed to list roles: %s", svcErr.ErrorDescription)
			}
			roles = found
		} else {
			byName, svcErr := t.roleService.GetRolesByNames(searchCtx, []string{input.Name})
			if svcErr != nil {
				return nil, nil, fmt.Errorf("failed to list roles: %s", svcErr.ErrorDescription)
			}
			for _, role := range byName[input.Name] {
				roles = append(roles, *role)
			}
		}
		roles = slices.DeleteFunc(roles, func(r Role) bool { return input.Name != "" && r.Name != input.Name })
		if input.OUID != "" {
			visible, err := t.rolesVisibleInOU(ctx, input.OUID, roles)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to list roles: %w", err)
			}
			roles = visible
		}
		return nil, roleListOf(len(roles), roles), nil
	}

	limit, err := pageLimit(input.Limit, input.Offset)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list roles: %w", err)
	}
	var filters map[string]interface{}
	if input.OUID != "" {
		filters = map[string]interface{}{filter.AttributeOUID: input.OUID}
	}
	list, svcErr := t.roleService.GetRoleList(ctx, limit, input.Offset, filters)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to list roles: %s", svcErr.ErrorDescription)
	}
	return nil, roleListOf(list.TotalResults, list.Roles), nil
}

// getRole handles the get_role tool call.
func (t *roleTools) getRole(ctx context.Context, _ *mcp.CallToolRequest, input getRoleMCPInput) (
	*mcp.CallToolResult, *roleMCPDetails, error) {
	details, err := t.describeRole(ctx, input.RoleID, input.OUID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get role: %w", err)
	}
	if !slices.Contains(input.Include, roleDetailHolders) {
		details.NotIncluded = []string{roleDetailHolders}
		return nil, details, nil
	}

	limit, err := pageLimit(input.Limit, input.Offset)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get role: %w", err)
	}
	assignments, svcErr := t.assignmentService.GetRoleAssignments(
		ctx, input.RoleID, input.OUID, limit, input.Offset, true)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to get role: %s", svcErr.ErrorDescription)
	}
	holders := make([]roleHolderMCPItem, 0, len(assignments.Assignments))
	for _, a := range assignments.Assignments {
		holders = append(holders, roleHolderMCPItem{
			ID: a.ID, Type: string(a.Type), Name: a.Display, OUID: a.OUID, Mandated: a.PolicyID != ""})
	}
	total := assignments.TotalResults
	details.Holders = &holders
	details.HoldersTotal = total
	return nil, details, nil
}

// rolesVisibleInOU keeps the roles the organization unit lists, owned by it or shared with it, and
// returns that listing's copies, which say whether the unit owns each one. It reads every page.
func (t *roleTools) rolesVisibleInOU(ctx context.Context, ouID string, roles []Role) ([]Role, error) {
	visible := map[string]Role{}
	for offset := 0; ; {
		page, svcErr := t.roleService.GetRoleList(ctx, serverconst.MaxPageSize, offset,
			map[string]interface{}{filter.AttributeOUID: ouID})
		if svcErr != nil {
			return nil, errors.New(svcErr.ErrorDescription.String())
		}
		for _, r := range page.Roles {
			visible[r.ID] = r
		}
		offset += len(page.Roles)
		if len(page.Roles) == 0 || offset >= page.TotalResults {
			break
		}
	}
	kept := []Role{}
	for _, r := range roles {
		if listed, ok := visible[r.ID]; ok {
			// A role the search found read-only stays read-only, whatever the unit's listing says.
			listed.IsReadOnly = listed.IsReadOnly || r.IsReadOnly
			kept = append(kept, listed)
		}
	}
	return kept, nil
}

// listAPIs handles the list_apis tool call. Without a name, the resource service pages the APIs. The
// resource service has no name search, so a name search reads every page, matches here, and returns
// every match.
func (t *roleTools) listAPIs(ctx context.Context, _ *mcp.CallToolRequest, input listAPIsMCPInput) (
	*mcp.CallToolResult, *apiListMCPResponse, error) {
	wanted := strings.ToLower(strings.TrimSpace(input.Name))
	if wanted == "" {
		limit, err := pageLimit(input.Limit, input.Offset)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list APIs: %w", err)
		}
		page, svcErr := t.resourceService.GetResourceServerList(ctx, limit, input.Offset)
		if svcErr != nil {
			return nil, nil, fmt.Errorf("failed to list APIs: %s", svcErr.ErrorDescription)
		}
		apis := make([]apiMCPItem, 0, len(page.ResourceServers))
		for _, server := range page.ResourceServers {
			apis = append(apis, apiMCPItem{API: server.Identifier, Name: server.Name,
				Description: server.Description, OUID: server.OUID})
		}
		return nil, &apiListMCPResponse{TotalResults: page.TotalResults, APIs: apis}, nil
	}
	if input.Limit != 0 || input.Offset != 0 {
		return nil, nil, errors.New("failed to list APIs: a name search returns every match, so limit and " +
			"offset can't be used with it")
	}

	apis := []apiMCPItem{}
	for offset := 0; ; {
		page, svcErr := t.resourceService.GetResourceServerList(ctx, serverconst.MaxPageSize, offset)
		if svcErr != nil {
			return nil, nil, fmt.Errorf("failed to list APIs: %s", svcErr.ErrorDescription)
		}
		for _, server := range page.ResourceServers {
			if strings.Contains(strings.ToLower(server.Name), wanted) ||
				strings.Contains(strings.ToLower(server.Identifier), wanted) {
				apis = append(apis, apiMCPItem{API: server.Identifier, Name: server.Name,
					Description: server.Description, OUID: server.OUID})
			}
		}
		offset += len(page.ResourceServers)
		if len(page.ResourceServers) == 0 || offset >= page.TotalResults {
			break
		}
	}
	slices.SortFunc(apis, func(a, b apiMCPItem) int { return strings.Compare(a.Name, b.Name) })
	return nil, &apiListMCPResponse{TotalResults: len(apis), APIs: apis}, nil
}

// listAPIPermissions handles the list_api_permissions tool call. An API's permissions are its actions and
// those of each of its resources, which the resource service lists separately, so no single paged list
// holds them all. It returns every permission.
func (t *roleTools) listAPIPermissions(ctx context.Context, _ *mcp.CallToolRequest,
	input listAPIPermissionsMCPInput) (*mcp.CallToolResult, *apiPermissionsMCPResponse, error) {
	server, err := t.apiByIdentifier(ctx, input.API)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list API permissions: %w", err)
	}

	permissions := []apiPermissionMCPItem{}
	addActions := func(resourceID *string) *tidcommon.ServiceError {
		for offset := 0; ; {
			actions, svcErr := t.resourceService.GetActionList(ctx, server.ID, resourceID, "",
				serverconst.MaxPageSize, offset)
			if svcErr != nil {
				return svcErr
			}
			for _, action := range actions.Actions {
				permissions = append(permissions, apiPermissionMCPItem{
					Permission: action.Permission, Name: action.Name, Description: action.Description})
			}
			offset += len(actions.Actions)
			if len(actions.Actions) == 0 || offset >= actions.TotalResults {
				return nil
			}
		}
	}
	if svcErr := addActions(nil); svcErr != nil {
		return nil, nil, fmt.Errorf("failed to list API permissions: %s", svcErr.ErrorDescription)
	}
	resources, svcErr := t.resourceService.GetAllResourceList(ctx, server.ID)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to list API permissions: %s", svcErr.ErrorDescription)
	}
	for _, resource := range resources {
		permissions = append(permissions, apiPermissionMCPItem{
			Permission: resource.Permission, Name: resource.Name, Description: resource.Description})
		if svcErr := addActions(&resource.ID); svcErr != nil {
			return nil, nil, fmt.Errorf("failed to list API permissions: %s", svcErr.ErrorDescription)
		}
	}
	slices.SortFunc(permissions, func(a, b apiPermissionMCPItem) int {
		return strings.Compare(a.Permission, b.Permission)
	})
	return nil, &apiPermissionsMCPResponse{API: server.Identifier, Name: server.Name,
		TotalResults: len(permissions), Permissions: permissions}, nil
}

// createRole handles the create_role tool call.
func (t *roleTools) createRole(ctx context.Context, _ *mcp.CallToolRequest, input createRoleMCPInput) (
	*mcp.CallToolResult, *roleMCPDetails, error) {
	if (input.API == "") != (len(input.Permissions) == 0) {
		return nil, nil, errors.New("failed to create role: give api and permissions together")
	}

	ouID := input.OUID
	if ouID == "" {
		ou, svcErr := t.ouService.GetOrganizationUnitByPath(ctx, defaultOUHandle)
		if svcErr != nil {
			return nil, nil, errors.New("failed to create role: the Default organization unit wasn't found, " +
				"so give ouId")
		}
		ouID = ou.ID
	}
	detail := RoleCreationDetail{Name: input.Name, Description: input.Description, OUID: ouID,
		Assignments: assignmentsOf(input.Assignees)}
	if input.API != "" {
		server, err := t.apiByIdentifier(ctx, input.API)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create role: %w", err)
		}
		detail.Permissions = []ResourcePermissions{{ResourceServerID: server.ID, Permissions: input.Permissions}}
		if !input.AllowDuplicate {
			existing, err := t.roleGrantingExactly(ctx, ouID, server.ID, input.Permissions)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to create role: %w", err)
			}
			if existing != nil {
				return nil, nil, fmt.Errorf("no role was created: the role %q (%s) in this organization unit already "+
					"grants exactly these permissions, so grant it with update_role_assignments, or set "+
					"allowDuplicate to create another", existing.Name, existing.ID)
			}
		}
	}

	created, svcErr := t.roleService.CreateRole(ctx, detail)
	if svcErr != nil {
		if svcErr.Code == ErrorRoleNameConflict.Code {
			if id := t.roleNamed(ctx, ouID, input.Name); id != "" {
				return nil, nil, fmt.Errorf("no role was created: a role named %q already exists in this "+
					"organization unit (%s); use it, or choose another name", input.Name, id)
			}
		}
		return nil, nil, fmt.Errorf("failed to create role: %s", svcErr.ErrorDescription)
	}
	details, err := t.describeRole(ctx, created.ID, "")
	if err != nil {
		return nil, nil, fmt.Errorf("the role was created with ID %s, but it could not be read back: %w",
			created.ID, err)
	}
	return nil, details, nil
}

// roleGrantingExactly returns a role in the organization unit whose permissions are exactly the given
// ones on the given API, or nil when there is none.
func (t *roleTools) roleGrantingExactly(ctx context.Context, ouID, serverID string, permissions []string) (
	*RoleWithPermissions, error) {
	wanted := slices.Sorted(slices.Values(slices.Compact(slices.Sorted(slices.Values(permissions)))))
	candidates, svcErr := t.roleService.GetRolesByPermission(ctx, serverID, wanted[0])
	if svcErr != nil {
		return nil, errors.New(svcErr.ErrorDescription.String())
	}
	for _, candidate := range candidates {
		if candidate.OUID != ouID {
			continue
		}
		role, svcErr := t.roleService.GetRoleWithPermissions(ctx, candidate.ID)
		if svcErr != nil {
			return nil, errors.New(svcErr.ErrorDescription.String())
		}
		if len(role.Permissions) == 1 && role.Permissions[0].ResourceServerID == serverID &&
			slices.Equal(slices.Sorted(slices.Values(role.Permissions[0].Permissions)), wanted) {
			return role, nil
		}
	}
	return nil, nil
}

// roleNamed returns the ID of the role with the given name in the organization unit, or "" when there
// is none or it can't be read.
func (t *roleTools) roleNamed(ctx context.Context, ouID, name string) string {
	found, svcErr := t.roleService.GetRolesByNames(ctx, []string{name})
	if svcErr != nil {
		return ""
	}
	for _, role := range found[name] {
		if role.OUID == ouID {
			return role.ID
		}
	}
	return ""
}

// updateRolePermissions handles the update_role_permissions tool call. Removals run first, and nothing is
// added if they are refused.
func (t *roleTools) updateRolePermissions(ctx context.Context, _ *mcp.CallToolRequest,
	input updateRolePermissionsMCPInput) (*mcp.CallToolResult, *updateRolePermissionsMCPResponse, error) {
	if len(input.Add) == 0 && len(input.Remove) == 0 {
		return nil, nil, errors.New("failed to update role permissions: give add, remove, or both")
	}
	server, err := t.apiByIdentifier(ctx, input.API)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to update role permissions: %w", err)
	}
	current, svcErr := t.roleService.GetRoleWithPermissions(ctx, input.RoleID)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to update role permissions: %s", svcErr.ErrorDescription)
	}

	held := permissionsOn(current.Permissions, server.ID)
	removed := []string{}
	for _, permission := range input.Remove {
		if slices.Contains(held, permission) && !slices.Contains(removed, permission) {
			removed = append(removed, permission)
		}
	}
	permissions := current.Permissions
	if len(removed) > 0 {
		permissions = withPermissions(current.Permissions, server.ID,
			slices.DeleteFunc(slices.Clone(held), func(p string) bool { return slices.Contains(removed, p) }))
		// On a server that issues tokens the role service runs the permission removal system flow, which
		// revokes the affected tokens.
		_, svcErr = t.roleService.UpdateRoleWithPermissions(ctx, current.ID, RoleUpdateDetail{
			Name: current.Name, Description: current.Description, OUID: current.OUID, Permissions: permissions})
		if svcErr != nil {
			return nil, nil, fmt.Errorf("cannot complete the permission removal, so nothing was added: %s",
				svcErr.ErrorDescription)
		}
	}

	added := []string{}
	remaining := permissionsOn(permissions, server.ID)
	for _, permission := range input.Add {
		if !slices.Contains(remaining, permission) && !slices.Contains(added, permission) {
			added = append(added, permission)
		}
	}
	if len(added) > 0 {
		_, svcErr = t.roleService.UpdateRoleWithPermissions(ctx, current.ID, RoleUpdateDetail{
			Name: current.Name, Description: current.Description, OUID: current.OUID,
			Permissions: withPermissions(permissions, server.ID, append(remaining, added...)),
		})
		if svcErr != nil {
			return nil, nil, fmt.Errorf("removed %v, but adding failed: %s", removed, svcErr.ErrorDescription)
		}
	}

	details, err := t.describeRole(ctx, current.ID, "")
	if err != nil {
		return nil, nil, fmt.Errorf("the permissions changed, but the role could not be read back: %w", err)
	}
	out := &updateRolePermissionsMCPResponse{Role: *details, Added: added, Removed: removed}
	if len(removed) > 0 {
		out.Notice = t.removalNotice()
	}
	return nil, out, nil
}

// updateRoleAssignments handles the update_role_assignments tool call. Revocations run first, and nothing
// is granted if one is refused.
func (t *roleTools) updateRoleAssignments(ctx context.Context, _ *mcp.CallToolRequest,
	input updateRoleAssignmentsMCPInput) (*mcp.CallToolResult, *updateRoleAssignmentsMCPResponse, error) {
	if len(input.Assignees) == 0 || (len(input.Grant) == 0 && len(input.Revoke) == 0) {
		return nil, nil, errors.New("failed to update role assignments: give assignees, and grant, revoke, or both")
	}

	out := &updateRoleAssignmentsMCPResponse{
		Revoked: []roleAssignmentChangeMCPItem{}, Granted: []roleAssignmentChangeMCPItem{}}
	for _, roleID := range input.Revoke {
		for _, assignee := range input.Assignees {
			if err := t.revokeRole(ctx, roleID, input.OUID, assignee); err != nil {
				return nil, nil, fmt.Errorf("cannot revoke role %s from %s: %w. Revoked so far: %s. Nothing was "+
					"granted", roleID, assignee.ID, err, describeChanges(out.Revoked))
			}
			out.Revoked = append(out.Revoked, roleAssignmentChangeMCPItem{RoleID: roleID, AssigneeID: assignee.ID})
		}
	}

	assignments := assignmentsOf(input.Assignees)
	for _, roleID := range input.Grant {
		if svcErr := t.assignmentService.AddAssignments(ctx, roleID, input.OUID, assignments); svcErr != nil {
			return nil, nil, fmt.Errorf("cannot grant role %s: %s. Revoked: %s. Granted so far: %s", roleID,
				svcErr.ErrorDescription, describeChanges(out.Revoked), describeChanges(out.Granted))
		}
		for _, assignee := range input.Assignees {
			out.Granted = append(out.Granted, roleAssignmentChangeMCPItem{RoleID: roleID, AssigneeID: assignee.ID})
		}
	}
	if len(out.Revoked) > 0 {
		out.Notice = t.removalNotice()
	}
	return nil, out, nil
}

// revokeRole removes one role from one assignee. A removal that would change nothing succeeds in the role
// assignment service, so it is checked first: an assignee without the role, or with it only through a
// sharing policy, is refused rather than reported revoked. On a server that runs system flows the service
// runs the role assignment removal flow, which revokes the affected tokens.
func (t *roleTools) revokeRole(ctx context.Context, roleID, ouID string, assignee roleAssigneeMCPInput) error {
	if _, svcErr := t.assignmentService.ValidateRemoveAssignment(ctx, roleID, ouID, assignee.ID); svcErr != nil {
		return errors.New(svcErr.ErrorDescription.String())
	}
	if svcErr := t.assignmentService.RemoveAssignments(ctx, roleID, ouID,
		[]RoleAssignment{{ID: assignee.ID, Type: AssigneeType(assignee.Type)}}); svcErr != nil {
		return errors.New(svcErr.ErrorDescription.String())
	}
	return nil
}

// describeRole reads a role and names the APIs it grants permissions on.
func (t *roleTools) describeRole(ctx context.Context, roleID, ouID string) (*roleMCPDetails, error) {
	role, svcErr := t.roleService.GetRoleInOU(ctx, roleID, ouID)
	if svcErr != nil {
		return nil, errors.New(svcErr.ErrorDescription.String())
	}
	readOnly, svcErr := t.roleService.IsRoleDeclarative(ctx, roleID)
	if svcErr != nil {
		return nil, errors.New(svcErr.ErrorDescription.String())
	}

	details := &roleMCPDetails{ID: role.ID, Name: role.Name, Description: role.Description, OUID: role.OUID,
		ReadOnly: readOnly || !role.IsOwnedByOU, Permissions: []roleAPIPermissionsMCPItem{}}
	for _, granted := range role.Permissions {
		item := roleAPIPermissionsMCPItem{API: granted.ResourceServerID,
			Permissions: slices.Sorted(slices.Values(granted.Permissions))}
		if server, svcErr := t.resourceService.GetResourceServer(ctx, granted.ResourceServerID); svcErr == nil &&
			server != nil {
			item.API = server.Identifier
			item.Name = server.Name
		}
		details.Permissions = append(details.Permissions, item)
	}
	return details, nil
}

// apiByIdentifier finds the API, a resource server, with the given identifier.
func (t *roleTools) apiByIdentifier(ctx context.Context, identifier string) (*providers.ResourceServer, error) {
	server, svcErr := t.resourceService.GetResourceServerByIdentifier(ctx, identifier)
	if svcErr != nil {
		return nil, fmt.Errorf("no API was found with the identifier %s: %s", identifier, svcErr.ErrorDescription)
	}
	return server, nil
}

// pageLimit checks a limit and an offset, and returns the limit, the default page size when none is given.
func pageLimit(limit, offset int) (int, error) {
	if limit == 0 {
		limit = defaultRolePageSize
	}
	if limit < 0 || limit > serverconst.MaxPageSize || offset < 0 {
		return 0, fmt.Errorf("limit must be 1 to %d and offset 0 or more", serverconst.MaxPageSize)
	}
	return limit, nil
}

// pageOf returns the page of items that limit and offset select.
func pageOf[T any](items []T, limit, offset int) []T {
	if offset >= len(items) {
		return []T{}
	}
	return items[offset:min(offset+limit, len(items))]
}

// roleListOf builds the list_roles output.
func roleListOf(total int, roles []Role) *roleListMCPResponse {
	items := make([]roleMCPItem, 0, len(roles))
	for _, r := range roles {
		items = append(items, roleMCPItem{ID: r.ID, Name: r.Name, Description: r.Description, OUID: r.OUID,
			ReadOnly: r.IsReadOnly})
	}
	return &roleListMCPResponse{TotalResults: total, Roles: items}
}

// assignmentsOf converts tool assignees to role assignments.
func assignmentsOf(assignees []roleAssigneeMCPInput) []RoleAssignment {
	assignments := make([]RoleAssignment, 0, len(assignees))
	for _, a := range assignees {
		assignments = append(assignments, RoleAssignment{ID: a.ID, Type: AssigneeType(a.Type)})
	}
	return assignments
}

// permissionsOn returns the permissions a set grants on one resource server.
func permissionsOn(permissions []ResourcePermissions, resourceServerID string) []string {
	for _, p := range permissions {
		if p.ResourceServerID == resourceServerID {
			return slices.Clone(p.Permissions)
		}
	}
	return []string{}
}

// withPermissions returns a copy of a permission set with the permissions on one resource server
// replaced. A resource server left with none is dropped.
func withPermissions(permissions []ResourcePermissions, resourceServerID string,
	onServer []string) []ResourcePermissions {
	result := make([]ResourcePermissions, 0, len(permissions)+1)
	replaced := false
	for _, p := range permissions {
		if p.ResourceServerID != resourceServerID {
			result = append(result, p)
			continue
		}
		replaced = true
		if len(onServer) > 0 {
			result = append(result, ResourcePermissions{ResourceServerID: resourceServerID, Permissions: onServer})
		}
	}
	if !replaced && len(onServer) > 0 {
		result = append(result, ResourcePermissions{ResourceServerID: resourceServerID, Permissions: onServer})
	}
	return result
}

// describeChanges lists role changes for an error message.
func describeChanges(changes []roleAssignmentChangeMCPItem) string {
	if len(changes) == 0 {
		return "none"
	}
	parts := make([]string, 0, len(changes))
	for _, c := range changes {
		parts = append(parts, c.RoleID+" for "+c.AssigneeID)
	}
	return strings.Join(parts, ", ")
}
