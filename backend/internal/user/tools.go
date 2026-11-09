// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/role"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/filter"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/mcp/tool"
)

// userTools carries the services the user MCP tools call.
type userTools struct {
	logger            *log.Logger
	userService       UserServiceInterface
	entityService     entity.EntityServiceInterface
	entityTypeService entitytype.EntityTypeServiceInterface
	roleService       role.RoleServiceInterface
	resourceService   resource.ResourceServiceInterface
}

// listUsersMCPInput is the input of the list_users tool.
//
//nolint:lll
type listUsersMCPInput struct {
	Attribute string `json:"attribute,omitempty" jsonschema:"An attribute to match, such as email or username. Give it with value."`
	Value     string `json:"value,omitempty" jsonschema:"The exact value the attribute has, such as alice@example.com"`
	OUID      string `json:"ouId,omitempty" jsonschema:"ID of an organization unit, to list its users. Can't be combined with attribute."`
	ProjectID string `json:"projectId,omitempty" jsonschema:"ID of a project, from list_projects, to list its users. Can't be combined with attribute."`
	Limit     int    `json:"limit,omitempty" jsonschema:"Maximum number of users to return, at most 100"`
	Offset    int    `json:"offset,omitempty" jsonschema:"Number of users to skip"`
}

// getUserMCPInput is the input of the get_user tool.
//
//nolint:lll
type getUserMCPInput struct {
	UserID  string   `json:"userId" jsonschema:"ID of the user, from list_users"`
	Include []string `json:"include,omitempty" jsonschema:"Details to add. groups: the groups the user is in, directly or through a nested group. roles: the roles the user holds, directly or through a group. permissions: what those roles let the user do, by API."`
}

// updateUserMCPInput is the input of the update_user tool.
//
//nolint:lll
type updateUserMCPInput struct {
	UserID     string         `json:"userId" jsonschema:"ID of the user, from list_users"`
	Attributes map[string]any `json:"attributes" jsonschema:"Attributes to change, merged into the current ones. A value of null removes that attribute."`
}

// userMCPItem is one user in a list.
type userMCPItem struct {
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	Type      string `json:"type"`
	OUID      string `json:"ouId"`
	ProjectID string `json:"projectId,omitempty"`
	Status    string `json:"status,omitempty"`
}

// userListMCPResponse is the list_users output.
type userListMCPResponse struct {
	TotalResults int           `json:"totalResults"`
	Users        []userMCPItem `json:"users"`
}

// userGroupMCPItem is one group a user is in.
type userGroupMCPItem struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	OUID       string `json:"ouId,omitempty"`
	Membership string `json:"membership"`
}

// userRoleMCPItem is one role a user holds.
type userRoleMCPItem struct {
	Name    string `json:"name"`
	Granted string `json:"granted"`
}

// userPermissionMCPItem is what a user's roles let them do on one API.
type userPermissionMCPItem struct {
	API         string   `json:"api"`
	Name        string   `json:"name,omitempty"`
	Permissions []string `json:"permissions"`
}

// userMCPDetails is one user, as get_user and update_user return it.
type userMCPDetails struct {
	ID          string                   `json:"id"`
	Name        string                   `json:"name,omitempty"`
	Type        string                   `json:"type"`
	OUID        string                   `json:"ouId"`
	ProjectID   string                   `json:"projectId,omitempty"`
	Status      string                   `json:"status,omitempty"`
	ReadOnly    bool                     `json:"readOnly"`
	Attributes  map[string]any           `json:"attributes,omitempty"`
	Groups      *[]userGroupMCPItem      `json:"groups,omitempty"`
	Roles       *[]userRoleMCPItem       `json:"roles,omitempty"`
	Permissions *[]userPermissionMCPItem `json:"permissions,omitempty"`
	NotIncluded []string                 `json:"notIncluded,omitempty"`
}

// RegisterMCPTools registers the user MCP tools.
func RegisterMCPTools(server *mcp.Server, userService UserServiceInterface,
	entityService entity.EntityServiceInterface, entityTypeService entitytype.EntityTypeServiceInterface,
	roleService role.RoleServiceInterface, resourceService resource.ResourceServiceInterface) {
	tools := &userTools{
		logger:            log.GetLogger().With(log.String(log.LoggerKeyComponentName, "UserMCPTools")),
		userService:       userService,
		entityService:     entityService,
		entityTypeService: entityTypeService,
		roleService:       roleService,
		resourceService:   resourceService,
	}
	details := make([]string, 0, len(userDetails))
	for _, detail := range userDetails {
		details = append(details, string(detail))
	}

	mcp.AddTool(server, &mcp.Tool{
		Name: "list_users",
		Description: `Find users. attribute and value match one attribute exactly, such as email ` +
			`alice@example.com or username alice; list_user_types shows the attributes a user type has. ` +
			`ouId lists the users an organization unit owns. Returns each user's ID, name, type, ` +
			`organization unit and status, 30 per page unless limit says otherwise, and the total. When ` +
			`more than one user matches, ask which one is meant. Use get_user for one user's attributes, ` +
			`groups, roles and permissions, list_agents for agents, and list_groups for groups.`,
		InputSchema: tool.GenerateSchema[listUsersMCPInput](tool.WithDefault("", "limit", defaultUserPageSize)),
		Annotations: &mcp.ToolAnnotations{Title: "List Users", ReadOnlyHint: true, OpenWorldHint: new(false)},
	}, tools.listUsers)

	mcp.AddTool(server, &mcp.Tool{
		Name: "get_user",
		Description: `Get one user: their name, type, organization unit, status and attributes. It never ` +
			`returns a password or other credential. Groups, roles and permissions come back only when ` +
			`include asks for them; notIncluded lists the ones left out, so a section missing for that reason ` +
			`doesn't mean the user has none. Use it to answer "what can Alice do?" or "is Alice in the ` +
			`support team?".`,
		InputSchema: tool.GenerateSchema[getUserMCPInput](tool.WithEnum("", "include", details)),
		Annotations: &mcp.ToolAnnotations{Title: "Get User", ReadOnlyHint: true, OpenWorldHint: new(false)},
	}, tools.getUser)

	mcp.AddTool(server, &mcp.Tool{
		Name: "update_user",
		Description: `Change a user's attributes, such as their department or display name. Attributes are ` +
			`merged into the current ones, and null removes one. A required attribute can't be removed, and ` +
			`a password or other credential can't be changed here: the user resets it, or an administrator ` +
			`sets it in the Console. Change the user's roles with update_role_assignments and their groups ` +
			`with update_group_memberships.`,
		InputSchema: tool.GenerateSchema[updateUserMCPInput](),
		Annotations: &mcp.ToolAnnotations{
			Title: "Update User", DestructiveHint: new(true), IdempotentHint: true, OpenWorldHint: new(false)},
	}, tools.updateUser)
}

// listUsers handles the list_users tool call.
func (t *userTools) listUsers(ctx context.Context, _ *mcp.CallToolRequest, input listUsersMCPInput) (
	*mcp.CallToolResult, *userListMCPResponse, error) {
	if (input.Attribute == "") != (input.Value == "") {
		return nil, nil, errors.New("failed to list users: give attribute and value together")
	}
	if input.Attribute != "" && input.OUID != "" {
		return nil, nil, errors.New("failed to list users: ouId can't be combined with attribute")
	}
	if input.Attribute != "" && input.ProjectID != "" {
		return nil, nil, errors.New("failed to list users: projectId can't be combined with attribute")
	}
	limit := input.Limit
	if limit == 0 {
		limit = defaultUserPageSize
	}
	filters := map[string]interface{}{}
	if input.Attribute != "" {
		filters[input.Attribute] = input.Value
	}
	if input.OUID != "" {
		filters[filter.AttributeOUID] = input.OUID
	}
	var list *UserListResponse
	var svcErr *tidcommon.ServiceError
	if input.ProjectID != "" {
		list, svcErr = t.userService.GetUserListByProject(ctx, input.ProjectID, limit, input.Offset, filters, true)
	} else {
		list, svcErr = t.userService.GetUserList(ctx, limit, input.Offset, filters, true)
	}
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to list users: %s", svcErr.ErrorDescription)
	}
	users := make([]userMCPItem, 0, len(list.Users))
	for _, u := range list.Users {
		users = append(users, userMCPItem{ID: u.ID, Name: u.Display, Type: u.Type, OUID: u.OUID,
			ProjectID: u.ProjectID, Status: statusOf(u.Status)})
	}
	return nil, &userListMCPResponse{TotalResults: list.TotalResults, Users: users}, nil
}

// getUser handles the get_user tool call.
func (t *userTools) getUser(ctx context.Context, _ *mcp.CallToolRequest, input getUserMCPInput) (
	*mcp.CallToolResult, *userMCPDetails, error) {
	u, svcErr := t.userService.GetUser(ctx, input.UserID, true)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to get user: %s", svcErr.ErrorDescription)
	}
	schema, err := t.schemaOf(ctx, u.Type)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get user: %w", err)
	}
	details := describeUser(u, schema)
	if err := t.addUserDetails(ctx, details, input.Include); err != nil {
		return nil, nil, fmt.Errorf("failed to get user: %w", err)
	}
	return nil, details, nil
}

// updateUser handles the update_user tool call. The service replaces a user's attributes, so the change
// is merged into the current ones first.
func (t *userTools) updateUser(ctx context.Context, _ *mcp.CallToolRequest, input updateUserMCPInput) (
	*mcp.CallToolResult, *userMCPDetails, error) {
	if len(input.Attributes) == 0 {
		return nil, nil, errors.New("failed to update user: give the attributes to change")
	}
	current, svcErr := t.userService.GetUser(ctx, input.UserID, false)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to update user: %s", svcErr.ErrorDescription)
	}
	schema, err := t.schemaOf(ctx, current.Type)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to update user: %w", err)
	}
	attributes := decodeAttributes(current.Attributes)
	for name, value := range input.Attributes {
		if value == nil {
			delete(attributes, name)
			continue
		}
		attributes[name] = value
	}
	if problems := attributeProblems(schema, attributes, input.Attributes); len(problems) > 0 {
		return nil, nil, fmt.Errorf("cannot update the user, so nothing was changed: %s",
			strings.Join(problems, "; "))
	}
	raw, err := json.Marshal(withoutCredentials(schema, attributes))
	if err != nil {
		return nil, nil, errors.New("failed to update user: the attributes are not valid JSON")
	}
	updated, svcErr := t.userService.UpdateUserAttributes(ctx, input.UserID, raw)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to update user: %s", svcErr.ErrorDescription)
	}
	return nil, describeUser(updated, schema), nil
}

// schemaOf returns a user type's attributes, credentials included.
func (t *userTools) schemaOf(ctx context.Context, userType string) ([]entitytype.AttributeInfo, error) {
	if userType == "" {
		return nil, errors.New("give userType, from list_user_types")
	}
	schema, svcErr := t.entityTypeService.GetAttributes(ctx, entitytype.TypeCategoryUser, userType,
		entitytype.AttributeFilter{AllowCredential: true, AllowNonCredential: true})
	if svcErr != nil {
		if svcErr.Code == entitytype.ErrorEntityTypeNotFound.Code {
			return nil, fmt.Errorf("no user type has the handle %q; list_user_types shows the ones that exist",
				userType)
		}
		return nil, errors.New(svcErr.ErrorDescription.String())
	}
	return schema, nil
}

// attributeProblems checks a user's attributes against their type: required attributes must be present,
// and no credential is accepted, since no credential goes through these tools.
func attributeProblems(schema []entitytype.AttributeInfo, attributes, given map[string]any) []string {
	problems := []string{}
	for _, info := range schema {
		if info.Credential {
			if _, ok := given[info.Attribute]; ok {
				problems = append(problems, fmt.Sprintf("%s is a credential, which can't be set through these "+
					"tools: the user sets it through account recovery, or an administrator sets it in the "+
					"Console", info.Attribute))
			}
			continue
		}
		if value, ok := attributes[info.Attribute]; info.Required && (!ok || value == nil || value == "") {
			problems = append(problems, fmt.Sprintf("%s is required", info.Attribute))
		}
	}
	return problems
}

// withoutCredentials drops the credential attributes from a set of attributes.
func withoutCredentials(schema []entitytype.AttributeInfo, attributes map[string]any) map[string]any {
	out := maps.Clone(attributes)
	for _, info := range schema {
		if info.Credential {
			delete(out, info.Attribute)
		}
	}
	return out
}

// describeUser describes a user without their credentials.
func describeUser(u *providers.User, schema []entitytype.AttributeInfo) *userMCPDetails {
	return &userMCPDetails{ID: u.ID, Name: u.Display, Type: u.Type, OUID: u.OUID, ProjectID: u.ProjectID,
		Status: statusOf(u.Status), ReadOnly: u.IsReadOnly,
		Attributes: withoutCredentials(schema, decodeAttributes(u.Attributes))}
}

// addUserDetails adds the sections include asks for, and names the ones left out.
func (t *userTools) addUserDetails(ctx context.Context, details *userMCPDetails, include []string) error {
	for _, detail := range userDetails {
		if !slices.Contains(include, string(detail)) {
			details.NotIncluded = append(details.NotIncluded, string(detail))
		}
	}
	if len(include) == 0 {
		return nil
	}

	groups, err := t.entityService.GetTransitiveEntityGroups(ctx, details.ID)
	if err != nil {
		t.logger.Error(ctx, "Failed to read a user's groups", log.Error(err))
		return errors.New("their groups could not be read")
	}
	groupIDs := make([]string, 0, len(groups))
	for _, g := range groups {
		groupIDs = append(groupIDs, g.ID)
	}

	if slices.Contains(include, string(userDetailGroups)) {
		direct, err := t.directGroupsOf(ctx, details.ID)
		if err != nil {
			t.logger.Error(ctx, "Failed to read a user's direct groups", log.Error(err))
			return errors.New("their groups could not be read")
		}
		items := make([]userGroupMCPItem, 0, len(groups))
		for _, g := range groups {
			membership := "inherited"
			if slices.ContainsFunc(direct, func(d providers.EntityGroup) bool { return d.ID == g.ID }) {
				membership = "direct"
			}
			items = append(items, userGroupMCPItem{ID: g.ID, Name: g.Name, OUID: g.OUID, Membership: membership})
		}
		details.Groups = &items
	}

	if slices.Contains(include, string(userDetailRoles)) {
		direct, svcErr := t.roleService.GetUserRoles(ctx, details.ID, nil, "")
		if svcErr != nil {
			return errors.New("their roles could not be read")
		}
		all, svcErr := t.roleService.GetUserRoles(ctx, details.ID, groupIDs, "")
		if svcErr != nil {
			return errors.New("their roles could not be read")
		}
		slices.Sort(all)
		items := make([]userRoleMCPItem, 0, len(all))
		for _, name := range slices.Compact(all) {
			granted := "through_group"
			if slices.Contains(direct, name) {
				granted = "directly"
			}
			items = append(items, userRoleMCPItem{Name: name, Granted: granted})
		}
		details.Roles = &items
	}

	if slices.Contains(include, string(userDetailPermissions)) {
		permissions, svcErr := t.roleService.GetAllPermissions(ctx, details.ID, groupIDs)
		if svcErr != nil {
			return errors.New("their permissions could not be read")
		}
		items := make([]userPermissionMCPItem, 0, len(permissions))
		for _, serverID := range slices.Sorted(maps.Keys(permissions)) {
			item := userPermissionMCPItem{
				API: serverID, Permissions: slices.Sorted(slices.Values(permissions[serverID]))}
			if server, svcErr := t.resourceService.GetResourceServer(ctx, serverID); svcErr == nil && server != nil {
				item.API = server.Identifier
				item.Name = server.Name
			}
			items = append(items, item)
		}
		details.Permissions = &items
	}
	return nil
}

// directGroupsOf reads every page of the groups a user is directly in.
func (t *userTools) directGroupsOf(ctx context.Context, userID string) ([]providers.EntityGroup, error) {
	direct := []providers.EntityGroup{}
	for offset := 0; ; {
		page, err := t.entityService.GetEntityGroups(ctx, userID, serverconst.MaxPageSize, offset)
		if err != nil {
			return nil, err
		}
		direct = append(direct, page...)
		offset += len(page)
		if len(page) < serverconst.MaxPageSize {
			return direct, nil
		}
	}
}

// decodeAttributes reads a user's stored attributes, or none when they can't be read.
func decodeAttributes(raw json.RawMessage) map[string]any {
	attributes := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &attributes)
	}
	return attributes
}

// statusOf names a user's access status.
func statusOf(status *providers.AccessStatusReport) string {
	if status == nil {
		return ""
	}
	return string(status.Value)
}
