// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package group

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/entity"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/filter"
	"github.com/thunder-id/thunderid/internal/system/mcp/tool"
)

// groupTools carries the services the group MCP tools call.
type groupTools struct {
	groupService  GroupServiceInterface
	entityService entity.EntityServiceInterface
	ouService     oupkg.OrganizationUnitServiceInterface
	// revokesOnRemoval is false on a server that issues no tokens, where a removal revokes nothing.
	revokesOnRemoval bool
}

// groupMemberMCPInput names a user, app, agent or group.
//
//nolint:lll
type groupMemberMCPInput struct {
	ID   string `json:"id" jsonschema:"ID of the user, app, agent or group"`
	Type string `json:"type" jsonschema:"What the ID names: user, app, agent or group"`
}

// listGroupsMCPInput is the input of the list_groups tool.
//
//nolint:lll
type listGroupsMCPInput struct {
	Name   string               `json:"name,omitempty" jsonschema:"Exact name of the group"`
	Member *groupMemberMCPInput `json:"member,omitempty" jsonschema:"A user, app, agent or group, to list the groups it belongs to, directly or through a group nested in another"`
	OUID   string               `json:"ouId,omitempty" jsonschema:"ID of an organization unit, to list the groups it owns. Narrows a name or member search."`
	Limit  int                  `json:"limit,omitempty" jsonschema:"Maximum number of groups to return, at most 100"`
	Offset int                  `json:"offset,omitempty" jsonschema:"Number of groups to skip"`
}

// getGroupMCPInput is the input of the get_group tool.
//
//nolint:lll
type getGroupMCPInput struct {
	GroupID string   `json:"groupId" jsonschema:"ID of the group, from list_groups or create_group"`
	Include []string `json:"include,omitempty" jsonschema:"Details to add. members: the users, apps, agents and groups directly in the group."`
	Limit   int      `json:"limit,omitempty" jsonschema:"Maximum number of members to return, at most 100"`
	Offset  int      `json:"offset,omitempty" jsonschema:"Number of members to skip"`
}

// createGroupMCPInput is the input of the create_group tool.
//
//nolint:lll
type createGroupMCPInput struct {
	Name        string                `json:"name" jsonschema:"Name of the group, unique in its organization unit"`
	Description string                `json:"description,omitempty" jsonschema:"What the group is for"`
	OUID        string                `json:"ouId,omitempty" jsonschema:"ID of the organization unit that owns the group. Defaults to the Default organization unit."`
	Members     []groupMemberMCPInput `json:"members,omitempty" jsonschema:"Users, apps, agents and groups to put in the group when it is created"`
}

// updateGroupMembershipsMCPInput is the input of the update_group_memberships tool.
//
//nolint:lll
type updateGroupMembershipsMCPInput struct {
	Members []groupMemberMCPInput `json:"members" jsonschema:"Users, apps, agents and groups whose memberships change. Each joins every group in add and leaves every group in remove."`
	Add     []string              `json:"add,omitempty" jsonschema:"IDs of groups to add the members to, from list_groups"`
	Remove  []string              `json:"remove,omitempty" jsonschema:"IDs of groups to remove the members from, from list_groups or get_group"`
}

// groupMCPItem is one group in a list.
type groupMCPItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	OUID        string `json:"ouId"`
	ReadOnly    bool   `json:"readOnly"`
	// Direct is set when the list answers which groups a member belongs to, and tells a group the member
	// is in directly from one it is in through a nested group.
	Direct *bool `json:"direct,omitempty"`
}

// groupListMCPResponse is the list_groups output.
type groupListMCPResponse struct {
	TotalResults int            `json:"totalResults"`
	Groups       []groupMCPItem `json:"groups"`
}

// groupMemberMCPItem is one member of a group.
type groupMemberMCPItem struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

// groupMCPDetails is one group, as get_group and create_group return it.
type groupMCPDetails struct {
	ID           string                `json:"id"`
	Name         string                `json:"name"`
	Description  string                `json:"description,omitempty"`
	OUID         string                `json:"ouId"`
	ReadOnly     bool                  `json:"readOnly"`
	Members      *[]groupMemberMCPItem `json:"members,omitempty"`
	MembersTotal int                   `json:"membersTotal,omitempty"`
	NotIncluded  []string              `json:"notIncluded,omitempty"`
}

// groupMembershipChangeMCPItem is one member added to or removed from one group.
type groupMembershipChangeMCPItem struct {
	GroupID  string `json:"groupId"`
	MemberID string `json:"memberId"`
}

// updateGroupMembershipsMCPResponse is the update_group_memberships output.
type updateGroupMembershipsMCPResponse struct {
	Removed []groupMembershipChangeMCPItem `json:"removed"`
	Added   []groupMembershipChangeMCPItem `json:"added"`
	Notice  string                         `json:"notice,omitempty"`
}

// revokingRemovalNotice describes what removing a member does on a server that issues tokens.
const revokingRemovalNotice = "Removing a member revokes the tokens that carried the access the group gave."

// directRemovalNotice describes what removing a member does on a server that issues no tokens, such as a
// control plane.
const directRemovalNotice = "This server issues no tokens, so removals are written directly, " +
	"without the token revocation a data plane performs."

// removalNotice describes what a removal does on this server.
func (t *groupTools) removalNotice() string {
	if !t.revokesOnRemoval {
		return directRemovalNotice
	}
	return revokingRemovalNotice
}

// RegisterMCPTools registers the group MCP tools with the server. revokesOnRemoval is false on a server
// that issues no tokens, where a removal revokes nothing.
func RegisterMCPTools(server *mcp.Server, groupService GroupServiceInterface,
	entityService entity.EntityServiceInterface, ouService oupkg.OrganizationUnitServiceInterface,
	revokesOnRemoval bool) {
	tools := &groupTools{
		groupService:     groupService,
		entityService:    entityService,
		ouService:        ouService,
		revokesOnRemoval: revokesOnRemoval,
	}
	memberTypes := []string{string(MemberTypeUser), string(MemberTypeApp), string(MemberTypeAgent),
		string(MemberTypeGroup)}

	mcp.AddTool(server, &mcp.Tool{
		Name: "list_groups",
		Description: `Find groups. name matches a group's exact name. member lists the groups a user, app, ` +
			`agent or group belongs to, directly or through a nested group, and direct tells them apart, so ` +
			`it answers "is Alice in the support team?". ouId lists the groups an organization unit owns, ` +
			`and narrows a name or member search. Results are paged with limit and offset, and totalResults ` +
			`counts every match. Returns each group's ID, name, organization unit and whether it is ` +
			`read-only. Use get_group for who is in a group.`,
		InputSchema: tool.GenerateSchema[listGroupsMCPInput](
			tool.WithEnum("member", "type", memberTypes),
			tool.WithDefault("", "limit", defaultGroupPageSize)),
		Annotations: &mcp.ToolAnnotations{Title: "List Groups", ReadOnlyHint: true, OpenWorldHint: new(false)},
	}, tools.listGroups)

	mcp.AddTool(server, &mcp.Tool{
		Name: "get_group",
		Description: `Get one group and whether it is read-only. With include members, also the users, apps, ` +
			`agents and groups directly in it, a page at a time with limit and offset, and membersTotal counting ` +
			`them all; members of a nested group belong through it. notIncluded ` +
			`lists the sections left out, so a missing section doesn't mean the group is empty.`,
		InputSchema: tool.GenerateSchema[getGroupMCPInput](
			tool.WithEnum("", "include", []string{groupDetailMembers})),
		Annotations: &mcp.ToolAnnotations{Title: "Get Group", ReadOnlyHint: true, OpenWorldHint: new(false)},
	}, tools.getGroup)

	mcp.AddTool(server, &mcp.Tool{
		Name: "create_group",
		Description: `Create a group, and optionally put its first members in it. If a group with this name ` +
			`already exists in the organization unit, nothing is created and the error names it, to add ` +
			`members to with update_group_memberships instead. Without ouId, the group goes in the Default ` +
			`organization unit. Grant the group roles with update_role_assignments, and change its members ` +
			`later with update_group_memberships.`,
		InputSchema: tool.GenerateSchema[createGroupMCPInput](tool.WithEnum("members", "type", memberTypes)),
		Annotations: &mcp.ToolAnnotations{
			Title: "Create Group", DestructiveHint: new(false), IdempotentHint: false, OpenWorldHint: new(false)},
	}, tools.createGroup)

	mcp.AddTool(server, &mcp.Tool{
		Name: "update_group_memberships",
		Description: `Add users, apps, agents and groups to groups and remove them from others. Each member ` +
			`joins every group in add and leaves every group in remove, so one call moves someone from one ` +
			`team to another. Removals run first, and nothing is added if one is refused. ` + tools.removalNotice(),
		InputSchema: tool.GenerateSchema[updateGroupMembershipsMCPInput](
			tool.WithEnum("members", "type", memberTypes)),
		Annotations: &mcp.ToolAnnotations{
			Title: "Update Group Memberships", DestructiveHint: new(true), IdempotentHint: true,
			OpenWorldHint: new(false)},
	}, tools.updateGroupMemberships)
}

// listGroups handles the list_groups tool call.
func (t *groupTools) listGroups(ctx context.Context, _ *mcp.CallToolRequest, input listGroupsMCPInput) (
	*mcp.CallToolResult, *groupListMCPResponse, error) {
	limit, err := pageLimit(input.Limit, input.Offset)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list groups: %w", err)
	}
	if input.Name == "" && input.Member == nil {
		var filters map[string]interface{}
		if input.OUID != "" {
			filters = map[string]interface{}{filter.AttributeOUID: input.OUID}
		}
		list, svcErr := t.groupService.GetGroupList(ctx, limit, input.Offset, filters, false)
		if svcErr != nil {
			return nil, nil, fmt.Errorf("failed to list groups: %s", svcErr.ErrorDescription)
		}
		groups := make([]groupMCPItem, 0, len(list.Groups))
		for _, g := range list.Groups {
			groups = append(groups, groupMCPItem{ID: g.ID, Name: g.Name, Description: g.Description, OUID: g.OUID,
				ReadOnly: g.IsReadOnly})
		}
		return nil, &groupListMCPResponse{TotalResults: list.TotalResults, Groups: groups}, nil
	}

	candidates, err := t.searchGroupIDs(ctx, input)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list groups: %w", err)
	}
	groups := []groupMCPItem{}
	for _, id := range candidates {
		// Looking groups up by name or member doesn't authorize the caller, so each one is read, which
		// does, and the ones the caller can't read are left out.
		g, svcErr := t.groupService.GetGroup(ctx, id, false)
		if svcErr != nil {
			if svcErr.Type == tidcommon.ClientErrorType {
				continue
			}
			return nil, nil, fmt.Errorf("failed to list groups: %s", svcErr.ErrorDescription)
		}
		if (input.Name != "" && g.Name != input.Name) || (input.OUID != "" && g.OUID != input.OUID) {
			continue
		}
		item := groupMCPItem{ID: g.ID, Name: g.Name, Description: g.Description, OUID: g.OUID,
			ReadOnly: g.IsReadOnly}
		if input.Member != nil {
			member, svcErr := t.groupService.GetDirectGroupMember(ctx, id, input.Member.ID)
			if svcErr != nil {
				return nil, nil, fmt.Errorf("failed to list groups: %s", svcErr.ErrorDescription)
			}
			direct := member != nil
			item.Direct = &direct
		}
		groups = append(groups, item)
	}
	slices.SortFunc(groups, func(a, b groupMCPItem) int { return strings.Compare(a.Name, b.Name) })
	return nil, &groupListMCPResponse{TotalResults: len(groups), Groups: pageOf(groups, limit, input.Offset)}, nil
}

// searchGroupIDs returns the IDs of the groups with the given name, the groups the given member belongs
// to, or the groups that match both.
func (t *groupTools) searchGroupIDs(ctx context.Context, input listGroupsMCPInput) ([]string, error) {
	var byName, byMember []string
	if input.Name != "" {
		found, svcErr := t.groupService.GetGroupsByNames(ctx, []string{input.Name})
		if svcErr != nil {
			return nil, errors.New(svcErr.ErrorDescription.String())
		}
		for _, g := range found[input.Name] {
			byName = append(byName, g.ID)
		}
		if input.Member == nil {
			return byName, nil
		}
	}

	if MemberType(input.Member.Type) == MemberTypeGroup {
		ancestors, svcErr := t.groupService.GetTransitiveAncestorGroups(ctx, input.Member.ID)
		if svcErr != nil {
			return nil, errors.New(svcErr.ErrorDescription.String())
		}
		byMember = ancestors
	} else {
		entityGroups, err := t.entityService.GetTransitiveEntityGroups(ctx, input.Member.ID)
		if err != nil {
			return nil, errors.New("the member's groups could not be read")
		}
		for _, g := range entityGroups {
			byMember = append(byMember, g.ID)
		}
	}
	if input.Name == "" {
		return byMember, nil
	}
	return slices.DeleteFunc(byName, func(id string) bool { return !slices.Contains(byMember, id) }), nil
}

// getGroup handles the get_group tool call.
func (t *groupTools) getGroup(ctx context.Context, _ *mcp.CallToolRequest, input getGroupMCPInput) (
	*mcp.CallToolResult, *groupMCPDetails, error) {
	g, svcErr := t.groupService.GetGroup(ctx, input.GroupID, false)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to get group: %s", svcErr.ErrorDescription)
	}
	details := describeGroup(g)
	if !slices.Contains(input.Include, groupDetailMembers) {
		details.NotIncluded = []string{groupDetailMembers}
		return nil, details, nil
	}

	limit, err := pageLimit(input.Limit, input.Offset)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get group: %w", err)
	}
	page, svcErr := t.groupService.GetGroupMembers(ctx, input.GroupID, limit, input.Offset, true)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to get group members: %s", svcErr.ErrorDescription)
	}
	members := make([]groupMemberMCPItem, 0, len(page.Members))
	for _, m := range page.Members {
		members = append(members, groupMemberMCPItem{ID: m.ID, Type: string(m.Type), Name: m.Display})
	}
	total := page.TotalResults
	details.Members = &members
	details.MembersTotal = total
	return nil, details, nil
}

// createGroup handles the create_group tool call.
func (t *groupTools) createGroup(ctx context.Context, _ *mcp.CallToolRequest, input createGroupMCPInput) (
	*mcp.CallToolResult, *groupMCPDetails, error) {
	ouID := input.OUID
	if ouID == "" {
		ou, svcErr := t.ouService.GetOrganizationUnitByPath(ctx, defaultOUHandle)
		if svcErr != nil {
			return nil, nil, errors.New("failed to create group: the Default organization unit wasn't found, " +
				"so give ouId")
		}
		ouID = ou.ID
	}
	created, svcErr := t.groupService.CreateGroup(ctx, CreateGroupRequest{
		Name: input.Name, Description: input.Description, OUID: ouID, Members: membersOf(input.Members)})
	if svcErr != nil {
		if svcErr.Code == ErrorGroupNameConflict.Code {
			if id := t.groupNamed(ctx, ouID, input.Name); id != "" {
				return nil, nil, fmt.Errorf("no group was created: a group named %q already exists in this "+
					"organization unit (%s), so add members to it with update_group_memberships, or choose "+
					"another name", input.Name, id)
			}
		}
		return nil, nil, fmt.Errorf("failed to create group: %s", svcErr.ErrorDescription)
	}
	details := describeGroup(created)
	members := make([]groupMemberMCPItem, 0, len(created.Members))
	for _, m := range created.Members {
		members = append(members, groupMemberMCPItem{ID: m.ID, Type: string(m.Type), Name: m.Display})
	}
	details.Members = &members
	details.MembersTotal = len(members)
	return nil, details, nil
}

// groupNamed returns the ID of the group with the given name in the organization unit, or "" when there
// is none or it can't be read.
func (t *groupTools) groupNamed(ctx context.Context, ouID, name string) string {
	found, svcErr := t.groupService.GetGroupsByNames(ctx, []string{name})
	if svcErr != nil {
		return ""
	}
	for _, group := range found[name] {
		if group.OUID == ouID {
			return group.ID
		}
	}
	return ""
}

// updateGroupMemberships handles the update_group_memberships tool call. Removals run first, through the
// configured administration flow when there is one, and nothing is added if one is refused.
func (t *groupTools) updateGroupMemberships(ctx context.Context, _ *mcp.CallToolRequest,
	input updateGroupMembershipsMCPInput) (*mcp.CallToolResult, *updateGroupMembershipsMCPResponse, error) {
	if len(input.Members) == 0 || (len(input.Add) == 0 && len(input.Remove) == 0) {
		return nil, nil, errors.New("failed to update group memberships: give members, and add, remove, or both")
	}

	out := &updateGroupMembershipsMCPResponse{
		Removed: []groupMembershipChangeMCPItem{}, Added: []groupMembershipChangeMCPItem{}}
	for _, groupID := range input.Remove {
		for _, member := range input.Members {
			if err := t.removeMember(ctx, groupID, member); err != nil {
				return nil, nil, fmt.Errorf("cannot remove %s from group %s: %w. Removed so far: %s. Nothing was "+
					"added", member.ID, groupID, err, describeChanges(out.Removed))
			}
			out.Removed = append(out.Removed, groupMembershipChangeMCPItem{GroupID: groupID, MemberID: member.ID})
		}
	}

	members := membersOf(input.Members)
	for _, groupID := range input.Add {
		if _, svcErr := t.groupService.AddGroupMembers(ctx, groupID, members); svcErr != nil {
			return nil, nil, fmt.Errorf("cannot add members to group %s: %s. Removed: %s. Added so far: %s",
				groupID, svcErr.ErrorDescription, describeChanges(out.Removed), describeChanges(out.Added))
		}
		for _, member := range input.Members {
			out.Added = append(out.Added, groupMembershipChangeMCPItem{GroupID: groupID, MemberID: member.ID})
		}
	}
	if len(out.Removed) > 0 {
		out.Notice = t.removalNotice()
	}
	return nil, out, nil
}

// removeMember removes one member from one group. Only a direct member can be removed, so one who isn't, or
// who belongs only through a nested group, is refused rather than reported removed.
func (t *groupTools) removeMember(ctx context.Context, groupID string, member groupMemberMCPInput) error {
	existing, svcErr := t.groupService.GetDirectGroupMember(ctx, groupID, member.ID)
	if svcErr != nil {
		return errors.New(svcErr.ErrorDescription.String())
	}
	if existing == nil || string(existing.Type) != member.Type {
		return fmt.Errorf("no %s with this ID is directly in the group, so there is nothing to remove; a member "+
			"who belongs through a nested group leaves when removed from that group", member.Type)
	}
	if _, svcErr := t.groupService.RemoveGroupMembers(ctx, groupID,
		[]Member{{ID: member.ID, Type: MemberType(member.Type)}}); svcErr != nil {
		return errors.New(svcErr.ErrorDescription.String())
	}
	return nil
}

// pageLimit checks a limit and an offset, and returns the limit, the default page size when none is given.
func pageLimit(limit, offset int) (int, error) {
	if limit == 0 {
		limit = defaultGroupPageSize
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

// describeGroup describes a group without its members.
func describeGroup(g *Group) *groupMCPDetails {
	return &groupMCPDetails{ID: g.ID, Name: g.Name, Description: g.Description, OUID: g.OUID,
		ReadOnly: g.IsReadOnly}
}

// membersOf turns the tool's member inputs into group members.
func membersOf(inputs []groupMemberMCPInput) []Member {
	members := make([]Member, 0, len(inputs))
	for _, m := range inputs {
		members = append(members, Member{ID: m.ID, Type: MemberType(m.Type)})
	}
	return members
}

// describeChanges lists membership changes for an error message.
func describeChanges(changes []groupMembershipChangeMCPItem) string {
	if len(changes) == 0 {
		return "none"
	}
	parts := make([]string, 0, len(changes))
	for _, c := range changes {
		parts = append(parts, c.MemberID+" in "+c.GroupID)
	}
	return strings.Join(parts, ", ")
}
