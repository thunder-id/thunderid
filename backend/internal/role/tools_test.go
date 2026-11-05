// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	oupkg "github.com/thunder-id/thunderid/internal/ou"
	resourcepkg "github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
	"github.com/thunder-id/thunderid/tests/mocks/resourcemock"
)

const (
	toolOrdersAPI = "https://api.example.com/orders"
	toolOrdersRS  = "rs-orders"
	toolRoleID    = "role-1"
)

type RoleToolsTestSuite struct {
	suite.Suite
	roles       *RoleServiceInterfaceMock
	assignments *RoleAssignmentServiceInterfaceMock
	resources   *resourcemock.ResourceServiceInterfaceMock
	ous         *oumock.OrganizationUnitServiceInterfaceMock
	session     *mcp.ClientSession
}

func TestRoleToolsTestSuite(t *testing.T) {
	suite.Run(t, new(RoleToolsTestSuite))
}

func (s *RoleToolsTestSuite) SetupTest() {
	s.connect(true)
}

// connect registers the tools over mocked services and connects an in-memory MCP client. revokes is true
// on a server that issues tokens, as a data plane does.
func (s *RoleToolsTestSuite) connect(revokes bool) {
	s.roles = NewRoleServiceInterfaceMock(s.T())
	s.assignments = NewRoleAssignmentServiceInterfaceMock(s.T())
	s.resources = resourcemock.NewResourceServiceInterfaceMock(s.T())
	s.ous = oumock.NewOrganizationUnitServiceInterfaceMock(s.T())
	s.resources.On("GetResourceServerByIdentifier", mock.Anything, toolOrdersAPI).Return(
		&providers.ResourceServer{ID: toolOrdersRS, Identifier: toolOrdersAPI, Name: "Orders"},
		(*tidcommon.ServiceError)(nil)).Maybe()
	s.resources.On("GetResourceServer", mock.Anything, toolOrdersRS).Return(
		&providers.ResourceServer{ID: toolOrdersRS, Identifier: toolOrdersAPI, Name: "Orders"},
		(*tidcommon.ServiceError)(nil)).Maybe()

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "1.0.0"}, nil)
	RegisterMCPTools(server, s.roles, s.assignments, s.resources, s.ous, revokes)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = serverSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	s.session, err = client.Connect(context.Background(), clientTransport, nil)
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = s.session.Close() })
}

// existingRole makes the role read back with the given permissions on the orders API.
func toolRole(permissions ...string) *RoleWithPermissions {
	return &RoleWithPermissions{ID: toolRoleID, Name: "Order Support", Description: "Supports orders",
		OUID: "ou-1", IsOwnedByOU: true,
		Permissions: []ResourcePermissions{{ResourceServerID: toolOrdersRS, Permissions: permissions}}}
}

func (s *RoleToolsTestSuite) existingRole(permissions ...string) *RoleWithPermissions {
	role := toolRole(permissions...)
	s.roles.On("GetRoleWithPermissions", mock.Anything, toolRoleID).Return(role, (*tidcommon.ServiceError)(nil))
	s.roles.On("IsRoleDeclarative", mock.Anything, toolRoleID).Return(false, (*tidcommon.ServiceError)(nil)).Maybe()
	return role
}

func (s *RoleToolsTestSuite) call(name string, args map[string]any) *mcp.CallToolResult {
	result, err := s.session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	s.Require().NoError(err)
	return result
}

func (s *RoleToolsTestSuite) structured(result *mcp.CallToolResult, out any) {
	s.Require().False(result.IsError, "tool returned an error: %v", result.Content)
	raw, err := json.Marshal(result.StructuredContent)
	s.Require().NoError(err)
	s.Require().NoError(json.Unmarshal(raw, out))
}

func (s *RoleToolsTestSuite) errorText(result *mcp.CallToolResult) string {
	s.Require().True(result.IsError)
	text, ok := result.Content[0].(*mcp.TextContent)
	s.Require().True(ok)
	return text.Text
}

func (s *RoleToolsTestSuite) TestRegisterMCPTools() {
	result, err := s.session.ListTools(context.Background(), nil)
	s.Require().NoError(err)
	tools := map[string]*mcp.Tool{}
	for _, t := range result.Tools {
		tools[t.Name] = t
		s.NotContains(t.Description, "—", t.Name)
		s.NotContains(t.Description, "--", t.Name)
		s.False(*t.Annotations.OpenWorldHint, t.Name)
	}
	s.Len(tools, 7)
	for _, name := range []string{"list_roles", "get_role", "list_api_permissions"} {
		s.True(tools[name].Annotations.ReadOnlyHint, name)
	}
	s.False(*tools["create_role"].Annotations.DestructiveHint)
	for _, name := range []string{"update_role_permissions", "update_role_assignments"} {
		s.True(*tools[name].Annotations.DestructiveHint, name)
		s.True(tools[name].Annotations.IdempotentHint, name)
	}
}

func (s *RoleToolsTestSuite) TestListRoles() {
	s.Run("by permission, narrowed by name", func() {
		s.roles.On("GetRolesByPermission", mock.Anything, toolOrdersRS, "orders:refund").Return([]Role{
			{ID: "r1", Name: "Refunds", OUID: "ou-1"}, {ID: "r2", Name: "Order Admin", OUID: "ou-1"},
		}, (*tidcommon.ServiceError)(nil)).Once()
		var out roleListMCPResponse
		s.structured(s.call("list_roles", map[string]any{
			"api": toolOrdersAPI, "permission": "orders:refund", "name": "Refunds"}), &out)
		s.Equal(1, out.TotalResults)
		s.Equal("r1", out.Roles[0].ID)
	})

	s.Run("by permission in a unit keeps the roles shared with it", func() {
		s.roles.On("GetRolesByPermission", mock.Anything, toolOrdersRS, "orders:refund").Return([]Role{
			{ID: "r1", Name: "Refunds", OUID: "ou-2", IsReadOnly: true},
			{ID: "r2", Name: "Shared Refunds", OUID: "ou-9"},
			{ID: "r3", Name: "Elsewhere", OUID: "ou-9"},
		}, (*tidcommon.ServiceError)(nil)).Once()
		s.roles.On("GetRoleList", mock.Anything, 100, 0, map[string]interface{}{"ouId": "ou-2"}).
			Return(&RoleList{TotalResults: 2, Roles: []Role{{ID: "r1", Name: "Refunds", OUID: "ou-2"}}},
				(*tidcommon.ServiceError)(nil)).Once()
		s.roles.On("GetRoleList", mock.Anything, 100, 1, map[string]interface{}{"ouId": "ou-2"}).
			Return(&RoleList{TotalResults: 2, Roles: []Role{
				{ID: "r2", Name: "Shared Refunds", OUID: "ou-9", IsReadOnly: true}}},
				(*tidcommon.ServiceError)(nil)).Once()
		var out roleListMCPResponse
		s.structured(s.call("list_roles", map[string]any{
			"api": toolOrdersAPI, "permission": "orders:refund", "ouId": "ou-2"}), &out)
		s.Equal(2, out.TotalResults)
		s.Equal("r1", out.Roles[0].ID)
		s.Equal("r2", out.Roles[1].ID)
		s.True(out.Roles[0].ReadOnly, "a declared role the search found stays read-only")
		s.True(out.Roles[1].ReadOnly, "the unit's listing says it doesn't own a shared role")
	})

	s.Run("by organization unit", func() {
		s.roles.On("GetRoleList", mock.Anything, mock.Anything, 0, map[string]interface{}{"ouId": "ou-1"}).
			Return(&RoleList{TotalResults: 5, Roles: []Role{{ID: "r1", Name: "Refunds", IsReadOnly: true}}},
				(*tidcommon.ServiceError)(nil)).Once()
		var out roleListMCPResponse
		s.structured(s.call("list_roles", map[string]any{"ouId": "ou-1"}), &out)
		s.Equal(5, out.TotalResults)
		s.True(out.Roles[0].ReadOnly)
	})

	s.Run("a permission search is paged, and reports how many it found", func() {
		s.roles.On("GetRolesByPermission", mock.Anything, toolOrdersRS, "orders:refund").Return([]Role{
			{ID: "r1", Name: "A"}, {ID: "r2", Name: "B"}, {ID: "r3", Name: "C"},
		}, (*tidcommon.ServiceError)(nil)).Once()
		var out roleListMCPResponse
		s.structured(s.call("list_roles", map[string]any{
			"api": toolOrdersAPI, "permission": "orders:refund", "limit": 2, "offset": 1}), &out)
		s.Equal(3, out.TotalResults)
		s.Require().Len(out.Roles, 2)
		s.Equal("r2", out.Roles[0].ID)
		s.Equal("r3", out.Roles[1].ID)
	})

	s.Run("an offset past the matches returns none", func() {
		s.roles.On("GetRolesByNames", mock.Anything, []string{"Refunds"}).Return(map[string][]*Role{
			"Refunds": {{ID: "r1", Name: "Refunds"}},
		}, (*tidcommon.ServiceError)(nil)).Once()
		var out roleListMCPResponse
		s.structured(s.call("list_roles", map[string]any{"name": "Refunds", "offset": 5}), &out)
		s.Equal(1, out.TotalResults)
		s.Empty(out.Roles)
	})

	s.Run("a limit over the page size is refused", func() {
		s.Contains(s.errorText(s.call("list_roles", map[string]any{"name": "Refunds", "limit": 500})),
			"limit must be 1 to 100")
	})

	s.Run("permission without api", func() {
		s.Contains(s.errorText(s.call("list_roles", map[string]any{"permission": "orders:refund"})),
			"api and permission together")
	})

	s.Run("unknown api", func() {
		s.resources.On("GetResourceServerByIdentifier", mock.Anything, "https://nope.example.com").
			Return((*providers.ResourceServer)(nil), &resourcepkg.ErrorResourceServerNotFound).Once()
		s.Contains(s.errorText(s.call("list_roles", map[string]any{
			"api": "https://nope.example.com", "permission": "x"})), "no API was found")
	})
}

func (s *RoleToolsTestSuite) TestGetRole() {
	s.existingRole("orders:read", "orders:update")
	// Holders come a page at a time: this page starts at the second holder.
	s.assignments.On("GetRoleAssignments", mock.Anything, toolRoleID, "", 2, 1, true).Return(&AssignmentList{
		TotalResults: 3, Assignments: []RoleAssignmentWithDisplay{
			{ID: "u1", Type: AssigneeTypeUser, Display: "Bob", OUID: "ou-1"},
			{ID: "g1", Type: AssigneeTypeGroup, Display: "Support", OUID: "ou-1", PolicyID: "p1"},
		}}, (*tidcommon.ServiceError)(nil)).Once()

	var out roleMCPDetails
	s.structured(s.call("get_role", map[string]any{"roleId": toolRoleID}), &out)
	s.Equal([]roleAPIPermissionsMCPItem{{API: toolOrdersAPI, Name: "Orders",
		Permissions: []string{"orders:read", "orders:update"}}}, out.Permissions)
	s.Equal([]string{"holders"}, out.NotIncluded)
	s.Nil(out.Holders)

	var withHolders roleMCPDetails
	s.structured(s.call("get_role", map[string]any{"roleId": toolRoleID, "include": []string{"holders"},
		"limit": 2, "offset": 1}), &withHolders)
	s.Require().NotNil(withHolders.Holders)
	s.Equal([]roleHolderMCPItem{{ID: "u1", Type: "user", Name: "Bob", OUID: "ou-1"},
		{ID: "g1", Type: "group", Name: "Support", OUID: "ou-1", Mandated: true}}, *withHolders.Holders)
	s.Equal(3, withHolders.HoldersTotal, "the total counts every holder, not only this page")
	s.Contains(s.errorText(s.call("get_role", map[string]any{"roleId": toolRoleID, "include": []string{"holders"},
		"limit": 500})), "limit must be 1 to 100")
}

func (s *RoleToolsTestSuite) TestListAPIPermissions() {
	// The API's own actions come back over two pages.
	s.resources.On("GetActionList", mock.Anything, toolOrdersRS, (*string)(nil), providers.ActionKind(""), 100, 0).
		Return(&resourcepkg.ActionList{TotalResults: 2, Actions: []providers.Action{
			{Name: "Read", Permission: "orders:read"}}}, (*tidcommon.ServiceError)(nil)).Once()
	s.resources.On("GetActionList", mock.Anything, toolOrdersRS, (*string)(nil), providers.ActionKind(""), 100, 1).
		Return(&resourcepkg.ActionList{TotalResults: 2, Actions: []providers.Action{
			{Name: "Cancel", Permission: "orders:cancel"}}}, (*tidcommon.ServiceError)(nil)).Once()
	s.resources.On("GetAllResourceList", mock.Anything, toolOrdersRS).Return([]providers.Resource{
		{ID: "res-1", Name: "Refunds", Permission: "orders:refunds"}}, (*tidcommon.ServiceError)(nil)).Once()
	s.resources.On("GetActionList", mock.Anything, toolOrdersRS, mock.MatchedBy(func(id *string) bool {
		return id != nil && *id == "res-1"
	}), providers.ActionKind(""), 100, 0).Return(&resourcepkg.ActionList{Actions: []providers.Action{
		{Name: "Issue", Permission: "orders:refunds:issue"}}}, (*tidcommon.ServiceError)(nil)).Once()

	var out apiPermissionsMCPResponse
	s.structured(s.call("list_api_permissions", map[string]any{"api": toolOrdersAPI}), &out)
	s.Equal("Orders", out.Name)
	permissions := make([]string, 0, len(out.Permissions))
	for _, p := range out.Permissions {
		permissions = append(permissions, p.Permission)
	}
	s.Equal([]string{"orders:cancel", "orders:read", "orders:refunds", "orders:refunds:issue"}, permissions)
}

func (s *RoleToolsTestSuite) TestCreateRole() {
	s.Run("in the Default organization unit, with first holders", func() {
		s.ous.On("GetOrganizationUnitByPath", mock.Anything, defaultOUHandle).
			Return(oupkg.OrganizationUnit{ID: "ou-default"}, (*tidcommon.ServiceError)(nil)).Once()
		var detail RoleCreationDetail
		s.roles.On("CreateRole", mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) { detail = args.Get(1).(RoleCreationDetail) }).
			Return(&RoleWithPermissionsAndAssignments{ID: toolRoleID}, (*tidcommon.ServiceError)(nil)).Once()
		s.roles.On("GetRolesByPermission", mock.Anything, toolOrdersRS, "orders:read").
			Return([]Role{}, (*tidcommon.ServiceError)(nil)).Once()
		s.existingRole("orders:read")

		var out roleMCPDetails
		s.structured(s.call("create_role", map[string]any{"name": "Order Support", "api": toolOrdersAPI,
			"permissions": []string{"orders:read"}, "assignees": []any{map[string]any{"id": "g1", "type": "group"}}}),
			&out)
		s.Equal("ou-default", detail.OUID)
		s.Equal([]ResourcePermissions{{ResourceServerID: toolOrdersRS, Permissions: []string{"orders:read"}}},
			detail.Permissions)
		s.Equal([]RoleAssignment{{ID: "g1", Type: AssigneeTypeGroup}}, detail.Assignments)
		s.Equal(toolRoleID, out.ID)
	})

	s.Run("assignee type outside the enum", func() {
		s.Contains(s.errorText(s.call("create_role", map[string]any{"name": "x", "ouId": "ou-1",
			"assignees": []any{map[string]any{"id": "g1", "type": "team"}}})), "type")
	})
}

func (s *RoleToolsTestSuite) TestUpdateRolePermissions() {
	s.Run("removes, then adds", func() {
		s.existingRole("orders:read", "orders:delete")
		var updates []RoleUpdateDetail
		s.roles.On("UpdateRoleWithPermissions", mock.Anything, toolRoleID, mock.Anything).
			Run(func(args mock.Arguments) { updates = append(updates, args.Get(2).(RoleUpdateDetail)) }).
			Return(toolRole(), (*tidcommon.ServiceError)(nil)).Twice()

		var out updateRolePermissionsMCPResponse
		s.structured(s.call("update_role_permissions", map[string]any{"roleId": toolRoleID, "api": toolOrdersAPI,
			"add": []string{"orders:refund", "orders:read"}, "remove": []string{"orders:delete", "orders:none"}}), &out)

		s.Require().Len(updates, 2)
		s.Equal([]ResourcePermissions{{ResourceServerID: toolOrdersRS, Permissions: []string{"orders:read"}}},
			updates[0].Permissions, "the removal is written first, on its own")
		s.Equal([]ResourcePermissions{{ResourceServerID: toolOrdersRS,
			Permissions: []string{"orders:read", "orders:refund"}}}, updates[1].Permissions)
		for _, update := range updates {
			s.Equal("Order Support", update.Name)
			s.Equal("Supports orders", update.Description)
			s.Equal("ou-1", update.OUID)
		}
		s.Equal([]string{"orders:refund"}, out.Added)
		s.Equal([]string{"orders:delete"}, out.Removed)
		s.Equal(revokingRemovalNotice, out.Notice)
	})

	s.Run("a refused removal adds nothing", func() {
		s.connect(true)
		s.existingRole("orders:delete")
		s.roles.On("UpdateRoleWithPermissions", mock.Anything, toolRoleID, mock.Anything).
			Return((*RoleWithPermissions)(nil),
				&tidcommon.ServiceError{ErrorDescription: tidcommon.I18nMessage{DefaultValue: "refused"}}).Once()
		text := s.errorText(s.call("update_role_permissions", map[string]any{"roleId": toolRoleID,
			"api": toolOrdersAPI, "add": []string{"orders:read"}, "remove": []string{"orders:delete"}}))
		s.Contains(text, "nothing was added")
		s.Contains(text, "refused")
		s.roles.AssertNumberOfCalls(s.T(), "UpdateRoleWithPermissions", 1)
	})

	s.Run("a server that issues no tokens says removals are written directly", func() {
		s.connect(false)
		role := s.existingRole("orders:delete")
		s.roles.On("UpdateRoleWithPermissions", mock.Anything, toolRoleID, mock.Anything).
			Return(role, (*tidcommon.ServiceError)(nil)).Once()
		var out updateRolePermissionsMCPResponse
		s.structured(s.call("update_role_permissions", map[string]any{"roleId": toolRoleID,
			"api": toolOrdersAPI, "remove": []string{"orders:delete"}}), &out)
		s.Equal([]string{"orders:delete"}, out.Removed)
		s.Equal(directRemovalNotice, out.Notice)
	})
}

func (s *RoleToolsTestSuite) TestUpdateRoleAssignments() {
	assignees := []any{map[string]any{"id": "u1", "type": "user"}}

	s.Run("revokes, then grants", func() {
		s.assignments.On("ValidateRemoveAssignment", mock.Anything, "viewer", "", "u1").
			Return(&revocation.AccessRevocationTarget{}, (*tidcommon.ServiceError)(nil)).Once()
		s.assignments.On("RemoveAssignments", mock.Anything, "viewer", "",
			[]RoleAssignment{{ID: "u1", Type: AssigneeTypeUser}}).Return((*tidcommon.ServiceError)(nil)).Once()
		s.assignments.On("AddAssignments", mock.Anything, "support", "",
			[]RoleAssignment{{ID: "u1", Type: AssigneeTypeUser}}).Return((*tidcommon.ServiceError)(nil)).Once()

		var out updateRoleAssignmentsMCPResponse
		s.structured(s.call("update_role_assignments", map[string]any{"assignees": assignees,
			"grant": []string{"support"}, "revoke": []string{"viewer"}}), &out)
		s.Equal([]roleAssignmentChangeMCPItem{{RoleID: "viewer", AssigneeID: "u1"}}, out.Revoked)
		s.Equal([]roleAssignmentChangeMCPItem{{RoleID: "support", AssigneeID: "u1"}}, out.Granted)
		s.Equal(revokingRemovalNotice, out.Notice)
	})

	s.Run("a role held only through a sharing policy is refused, and nothing is granted", func() {
		s.connect(true)
		s.assignments.On("ValidateRemoveAssignment", mock.Anything, "viewer", "", "u1").
			Return((*revocation.AccessRevocationTarget)(nil), &ErrorAssignmentMandated).Once()
		text := s.errorText(s.call("update_role_assignments", map[string]any{"assignees": assignees,
			"grant": []string{"support"}, "revoke": []string{"viewer"}}))
		s.Contains(text, ErrorAssignmentMandated.ErrorDescription.DefaultValue)
		s.Contains(text, "Nothing was granted")
		s.assignments.AssertNotCalled(s.T(), "RemoveAssignments", mock.Anything, mock.Anything, mock.Anything,
			mock.Anything)
		s.assignments.AssertNotCalled(s.T(), "AddAssignments", mock.Anything, mock.Anything, mock.Anything,
			mock.Anything)
	})

	s.Run("a role the assignee doesn't hold is refused rather than reported revoked", func() {
		s.connect(true)
		s.assignments.On("ValidateRemoveAssignment", mock.Anything, "viewer", "", "u1").
			Return((*revocation.AccessRevocationTarget)(nil), &ErrorRoleAssignmentNotFound).Once()
		text := s.errorText(s.call("update_role_assignments", map[string]any{"assignees": assignees,
			"revoke": []string{"viewer"}}))
		s.Contains(text, "Revoked so far: none")
		s.assignments.AssertNotCalled(s.T(), "RemoveAssignments", mock.Anything, mock.Anything, mock.Anything,
			mock.Anything)
	})

	s.Run("revokes in the given unit", func() {
		s.connect(true)
		s.assignments.On("ValidateRemoveAssignment", mock.Anything, "viewer", "ou-2", "u1").
			Return(&revocation.AccessRevocationTarget{}, (*tidcommon.ServiceError)(nil)).Once()
		s.assignments.On("RemoveAssignments", mock.Anything, "viewer", "ou-2",
			[]RoleAssignment{{ID: "u1", Type: AssigneeTypeUser}}).Return((*tidcommon.ServiceError)(nil)).Once()
		var out updateRoleAssignmentsMCPResponse
		s.structured(s.call("update_role_assignments", map[string]any{"assignees": assignees,
			"revoke": []string{"viewer"}, "ouId": "ou-2"}), &out)
		s.Len(out.Revoked, 1)
		s.Empty(out.Granted)
	})

	s.Run("needs a change", func() {
		s.connect(true)
		s.Contains(s.errorText(s.call("update_role_assignments", map[string]any{"assignees": assignees})),
			"grant, revoke, or both")
	})
}

// The removal tools say what a removal does on this server: a data plane revokes tokens, and a control
// plane writes removals directly.
func (s *RoleToolsTestSuite) TestRemovalNoticeFollowsThePlane() {
	for _, revokes := range []bool{true, false} {
		s.connect(revokes)
		result, err := s.session.ListTools(context.Background(), nil)
		s.Require().NoError(err)
		expected := revokingRemovalNotice
		if !revokes {
			expected = directRemovalNotice
		}
		for _, t := range result.Tools {
			if t.Name == "update_role_permissions" || t.Name == "update_role_assignments" {
				s.Contains(t.Description, expected, t.Name)
			}
		}
	}
}

// create_role names an existing role instead of creating a near-duplicate of it.
func (s *RoleToolsTestSuite) TestCreateRole_PointsToAnExistingRole() {
	s.Run("same permissions in the unit", func() {
		s.roles.On("GetRolesByPermission", mock.Anything, toolOrdersRS, "orders:read").Return([]Role{
			{ID: "elsewhere", OUID: "ou-9"}, {ID: "role-1", OUID: "ou-1"}}, (*tidcommon.ServiceError)(nil)).Once()
		s.roles.On("GetRoleWithPermissions", mock.Anything, "role-1").Return(&RoleWithPermissions{
			ID: "role-1", Name: "Order Readers", OUID: "ou-1",
			Permissions: []ResourcePermissions{{ResourceServerID: toolOrdersRS,
				Permissions: []string{"orders:refund", "orders:read"}}}}, (*tidcommon.ServiceError)(nil)).Once()
		text := s.errorText(s.call("create_role", map[string]any{"name": "Refunds", "ouId": "ou-1",
			"api": toolOrdersAPI, "permissions": []string{"orders:read", "orders:refund"}}))
		s.Contains(text, `the role "Order Readers" (role-1)`)
		s.Contains(text, "update_role_assignments")
		s.roles.AssertNotCalled(s.T(), "CreateRole", mock.Anything, mock.Anything)
	})

	s.Run("a role that grants more is no duplicate", func() {
		s.connect(true)
		s.roles.On("GetRolesByPermission", mock.Anything, toolOrdersRS, "orders:read").Return([]Role{
			{ID: "role-1", OUID: "ou-1"}}, (*tidcommon.ServiceError)(nil)).Once()
		s.roles.On("GetRoleWithPermissions", mock.Anything, "role-1").Return(&RoleWithPermissions{
			ID: "role-1", OUID: "ou-1", Permissions: []ResourcePermissions{{ResourceServerID: toolOrdersRS,
				Permissions: []string{"orders:read", "orders:delete"}}}}, (*tidcommon.ServiceError)(nil)).Once()
		s.roles.On("CreateRole", mock.Anything, mock.Anything).Return(
			&RoleWithPermissionsAndAssignments{ID: toolRoleID}, (*tidcommon.ServiceError)(nil)).Once()
		s.existingRole("orders:read")
		var out roleMCPDetails
		s.structured(s.call("create_role", map[string]any{"name": "Readers", "ouId": "ou-1",
			"api": toolOrdersAPI, "permissions": []string{"orders:read"}}), &out)
		s.Equal(toolRoleID, out.ID)
	})

	s.Run("allowDuplicate skips the check", func() {
		s.connect(true)
		s.roles.On("CreateRole", mock.Anything, mock.Anything).Return(
			&RoleWithPermissionsAndAssignments{ID: toolRoleID}, (*tidcommon.ServiceError)(nil)).Once()
		s.existingRole("orders:read")
		var out roleMCPDetails
		s.structured(s.call("create_role", map[string]any{"name": "Readers 2", "ouId": "ou-1",
			"api": toolOrdersAPI, "permissions": []string{"orders:read"}, "allowDuplicate": true}), &out)
		s.roles.AssertNotCalled(s.T(), "GetRolesByPermission", mock.Anything, mock.Anything, mock.Anything)
	})

	s.Run("the name is taken in the unit", func() {
		s.connect(true)
		s.roles.On("CreateRole", mock.Anything, mock.Anything).Return(
			(*RoleWithPermissionsAndAssignments)(nil), &ErrorRoleNameConflict).Once()
		s.roles.On("GetRolesByNames", mock.Anything, []string{"Support"}).Return(map[string][]*Role{
			"Support": {{ID: "other", OUID: "ou-9"}, {ID: "role-7", OUID: "ou-1"}}},
			(*tidcommon.ServiceError)(nil)).Once()
		text := s.errorText(s.call("create_role", map[string]any{"name": "Support", "ouId": "ou-1"}))
		s.Contains(text, `a role named "Support" already exists in this organization unit (role-7)`)
	})
}

// list_apis and list_api_permissions return a page at a time, and count every match.
func (s *RoleToolsTestSuite) TestAPIListsArePaged() {
	s.resources.On("GetResourceServerList", mock.Anything, 100, 0).Return(&resourcepkg.ResourceServerList{
		TotalResults: 3, ResourceServers: []providers.ResourceServer{
			{Name: "A", Identifier: "https://a"}, {Name: "B", Identifier: "https://b"},
			{Name: "C", Identifier: "https://c"},
		}}, (*tidcommon.ServiceError)(nil)).Once()
	var apis apiListMCPResponse
	s.structured(s.call("list_apis", map[string]any{"limit": 1, "offset": 1}), &apis)
	s.Equal(3, apis.TotalResults)
	s.Equal([]apiMCPItem{{API: "https://b", Name: "B"}}, apis.APIs)

	s.resources.On("GetActionList", mock.Anything, toolOrdersRS, (*string)(nil), providers.ActionKind(""), 100, 0).
		Return(&resourcepkg.ActionList{TotalResults: 3, Actions: []providers.Action{
			{Name: "Cancel", Permission: "orders:cancel"}, {Name: "Read", Permission: "orders:read"},
			{Name: "Refund", Permission: "orders:refund"}}}, (*tidcommon.ServiceError)(nil)).Once()
	s.resources.On("GetAllResourceList", mock.Anything, toolOrdersRS).
		Return([]providers.Resource{}, (*tidcommon.ServiceError)(nil)).Once()
	var permissions apiPermissionsMCPResponse
	s.structured(s.call("list_api_permissions", map[string]any{"api": toolOrdersAPI, "limit": 2}), &permissions)
	s.Equal(3, permissions.TotalResults)
	s.Equal([]apiPermissionMCPItem{{Permission: "orders:cancel", Name: "Cancel"},
		{Permission: "orders:read", Name: "Read"}}, permissions.Permissions)

	s.Contains(s.errorText(s.call("list_apis", map[string]any{"offset": -1})), "offset 0 or more")
}

func (s *RoleToolsTestSuite) TestListAPIs() {
	s.resources.On("GetResourceServerList", mock.Anything, 100, 0).Return(&resourcepkg.ResourceServerList{
		TotalResults: 3, ResourceServers: []providers.ResourceServer{
			{Name: "Orders", Identifier: toolOrdersAPI, OUID: "ou-1"},
			{Name: "System", Identifier: "https://localhost:8090/mcp", OUID: "ou-1"},
		}}, (*tidcommon.ServiceError)(nil)).Once()
	s.resources.On("GetResourceServerList", mock.Anything, 100, 2).Return(&resourcepkg.ResourceServerList{
		TotalResults: 3, ResourceServers: []providers.ResourceServer{
			{Name: "Refund Orders", Identifier: "https://api.example.com/refunds", OUID: "ou-1"},
		}}, (*tidcommon.ServiceError)(nil)).Once()

	var out apiListMCPResponse
	s.structured(s.call("list_apis", map[string]any{"name": "ORDERS"}), &out)
	s.Equal(2, out.TotalResults, "the name matches in any case, across every page")
	s.Equal([]apiMCPItem{{API: toolOrdersAPI, Name: "Orders", OUID: "ou-1"},
		{API: "https://api.example.com/refunds", Name: "Refund Orders", OUID: "ou-1"}}, out.APIs)
}

// Every role tool needs the permission its API needs, so none falls back to root by accident.
func (s *RoleToolsTestSuite) TestToolsRequireTheAPIPermissions() {
	security.InitSystemPermissions("")
	p := security.GetSystemPermissions()
	tools, err := s.session.ListTools(context.Background(), nil)
	s.Require().NoError(err)
	s.Require().NotEmpty(tools.Tools)
	for _, registered := range tools.Tools {
		s.Contains([]string{p.RoleView, p.RoleCreate, p.RoleUpdate, p.ResourceServerView},
			security.ResolveMCPToolPermission(registered.Name),
			"tool %q must require a role or resource server permission", registered.Name)
	}
}
