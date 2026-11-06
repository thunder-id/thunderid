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
	runner      *administrationFlowRunnerMock
	session     *mcp.ClientSession
}

func TestRoleToolsTestSuite(t *testing.T) {
	suite.Run(t, new(RoleToolsTestSuite))
}

func (s *RoleToolsTestSuite) SetupTest() {
	s.connect(true)
}

// connect registers the tools over mocked services and connects an in-memory MCP client. withRunner gives
// the tools a flow runner, as on a data plane.
func (s *RoleToolsTestSuite) connect(withRunner bool) {
	s.roles = NewRoleServiceInterfaceMock(s.T())
	s.assignments = NewRoleAssignmentServiceInterfaceMock(s.T())
	s.resources = resourcemock.NewResourceServiceInterfaceMock(s.T())
	s.ous = oumock.NewOrganizationUnitServiceInterfaceMock(s.T())
	s.runner = newAdministrationFlowRunnerMock(s.T())
	s.resources.On("GetResourceServerByIdentifier", mock.Anything, toolOrdersAPI).Return(
		&providers.ResourceServer{ID: toolOrdersRS, Identifier: toolOrdersAPI, Name: "Orders"},
		(*tidcommon.ServiceError)(nil)).Maybe()
	s.resources.On("GetResourceServer", mock.Anything, toolOrdersRS).Return(
		&providers.ResourceServer{ID: toolOrdersRS, Identifier: toolOrdersAPI, Name: "Orders"},
		(*tidcommon.ServiceError)(nil)).Maybe()

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "1.0.0"}, nil)
	var runner administrationFlowRunner
	if withRunner {
		runner = s.runner
	}
	RegisterMCPTools(server, s.roles, s.assignments, s.resources, s.ous, runner)
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

// roleChangedByFlow makes the role read as before until the removal flow runs, and as after once it has.
func (s *RoleToolsTestSuite) roleChangedByFlow(before, after *RoleWithPermissions) {
	s.roles.On("GetRoleWithPermissions", mock.Anything, toolRoleID).Return(before, (*tidcommon.ServiceError)(nil)).
		Once()
	s.roles.On("GetRoleWithPermissions", mock.Anything, toolRoleID).Return(after, (*tidcommon.ServiceError)(nil))
	s.roles.On("IsRoleDeclarative", mock.Anything, toolRoleID).Return(false, (*tidcommon.ServiceError)(nil)).Maybe()
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
	// The holders come back over two pages.
	s.assignments.On("GetRoleAssignments", mock.Anything, toolRoleID, "", 100, 0, true).Return(&AssignmentList{
		TotalResults: 2, Assignments: []RoleAssignmentWithDisplay{
			{ID: "u1", Type: AssigneeTypeUser, Display: "Bob", OUID: "ou-1"},
		}}, (*tidcommon.ServiceError)(nil)).Once()
	s.assignments.On("GetRoleAssignments", mock.Anything, toolRoleID, "", 100, 1, true).Return(&AssignmentList{
		TotalResults: 2, Assignments: []RoleAssignmentWithDisplay{
			{ID: "g1", Type: AssigneeTypeGroup, Display: "Support", OUID: "ou-1", PolicyID: "p1"},
		}}, (*tidcommon.ServiceError)(nil)).Once()

	var out roleMCPDetails
	s.structured(s.call("get_role", map[string]any{"roleId": toolRoleID}), &out)
	s.Equal([]roleAPIPermissionsMCPItem{{API: toolOrdersAPI, Name: "Orders",
		Permissions: []string{"orders:read", "orders:update"}}}, out.Permissions)
	s.Equal([]string{"holders"}, out.NotIncluded)
	s.Nil(out.Holders)

	var withHolders roleMCPDetails
	s.structured(s.call("get_role", map[string]any{"roleId": toolRoleID, "include": []string{"holders"}}),
		&withHolders)
	s.Require().NotNil(withHolders.Holders)
	s.Equal([]roleHolderMCPItem{{ID: "u1", Type: "user", Name: "Bob", OUID: "ou-1"},
		{ID: "g1", Type: "group", Name: "Support", OUID: "ou-1", Mandated: true}}, *withHolders.Holders)
	s.Equal(2, withHolders.HoldersTotal)
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
	s.Run("removes through the flow, then adds", func() {
		role := toolRole("orders:read", "orders:delete")
		s.roleChangedByFlow(role, toolRole("orders:read"))
		var inputs map[string]string
		s.runner.On("RunAdministrationFlow", mock.Anything, rolePermissionRemovalFlowKey, mock.Anything).
			Run(func(args mock.Arguments) { inputs = args.Get(2).(map[string]string) }).
			Return(true, (*tidcommon.ServiceError)(nil)).Once()
		var update RoleUpdateDetail
		s.roles.On("UpdateRoleWithPermissions", mock.Anything, toolRoleID, mock.Anything).
			Run(func(args mock.Arguments) { update = args.Get(2).(RoleUpdateDetail) }).
			Return(role, (*tidcommon.ServiceError)(nil)).Once()

		var out updateRolePermissionsMCPResponse
		s.structured(s.call("update_role_permissions", map[string]any{"roleId": toolRoleID, "api": toolOrdersAPI,
			"add": []string{"orders:refund", "orders:read"}, "remove": []string{"orders:delete", "orders:none"}}), &out)

		s.Equal(toolRoleID, inputs[flowInputRole])
		s.JSONEq(`[{"resourceServerId":"rs-orders","permissions":["orders:read"]}]`, inputs[flowInputPermissions])
		s.Equal([]ResourcePermissions{{ResourceServerID: toolOrdersRS,
			Permissions: []string{"orders:read", "orders:refund"}}}, update.Permissions)
		s.Equal("Order Support", update.Name)
		s.Equal("ou-1", update.OUID)
		s.Equal([]string{"orders:refund"}, out.Added)
		s.Equal([]string{"orders:delete"}, out.Removed)
		s.Equal(flowRemovalNotice, out.Notice)
	})

	s.Run("a flow that finishes without removing is refused", func() {
		s.connect(true)
		s.existingRole("orders:delete")
		s.runner.On("RunAdministrationFlow", mock.Anything, rolePermissionRemovalFlowKey, mock.Anything).
			Return(true, (*tidcommon.ServiceError)(nil)).Once()
		text := s.errorText(s.call("update_role_permissions", map[string]any{"roleId": toolRoleID,
			"api": toolOrdersAPI, "add": []string{"orders:read"}, "remove": []string{"orders:delete"}}))
		s.Contains(text, "nothing was added")
		s.Contains(text, "still grants [orders:delete]")
		s.roles.AssertNotCalled(s.T(), "UpdateRoleWithPermissions", mock.Anything, mock.Anything, mock.Anything)
	})

	s.Run("a refused removal adds nothing", func() {
		s.connect(true)
		s.existingRole("orders:delete")
		s.runner.On("RunAdministrationFlow", mock.Anything, rolePermissionRemovalFlowKey, mock.Anything).
			Return(false, &tidcommon.ServiceError{ErrorDescription: tidcommon.I18nMessage{DefaultValue: "refused"}}).
			Once()
		text := s.errorText(s.call("update_role_permissions", map[string]any{"roleId": toolRoleID,
			"api": toolOrdersAPI, "add": []string{"orders:read"}, "remove": []string{"orders:delete"}}))
		s.Contains(text, "nothing was added")
		s.roles.AssertNotCalled(s.T(), "UpdateRoleWithPermissions", mock.Anything, mock.Anything, mock.Anything)
	})

	s.Run("no flow runner writes the removal directly", func() {
		s.connect(false)
		role := s.existingRole("orders:delete")
		s.roles.On("UpdateRoleWithPermissions", mock.Anything, toolRoleID, mock.Anything).
			Return(role, (*tidcommon.ServiceError)(nil)).Once()
		var out updateRolePermissionsMCPResponse
		s.structured(s.call("update_role_permissions", map[string]any{"roleId": toolRoleID,
			"api": toolOrdersAPI, "remove": []string{"orders:delete"}}), &out)
		s.Equal([]string{"orders:delete"}, out.Removed)
		s.Equal(directRemovalNotice, out.Notice, "a server without flows says removals are written directly")
		s.runner.AssertNotCalled(s.T(), "RunAdministrationFlow", mock.Anything, mock.Anything, mock.Anything)
	})
}

func (s *RoleToolsTestSuite) TestUpdateRoleAssignments() {
	assignees := []any{map[string]any{"id": "u1", "type": "user"}}

	s.Run("revokes through the flow, then grants", func() {
		s.runner.On("RunAdministrationFlow", mock.Anything, roleAssignmentRemovalFlowKey,
			map[string]string{flowInputRole: "viewer", flowInputAssignee: "u1"}).
			Return(true, (*tidcommon.ServiceError)(nil)).Once()
		// Checked before the flow runs, then again to confirm the flow removed it.
		s.assignments.On("ValidateRemoveAssignment", mock.Anything, "viewer", "", "u1").
			Return(&revocation.AccessRevocationTarget{}, (*tidcommon.ServiceError)(nil)).Once()
		s.assignments.On("ValidateRemoveAssignment", mock.Anything, "viewer", "", "u1").
			Return((*revocation.AccessRevocationTarget)(nil), &ErrorRoleAssignmentNotFound).Once()
		s.assignments.On("AddAssignments", mock.Anything, "support", "",
			[]RoleAssignment{{ID: "u1", Type: AssigneeTypeUser}}).Return((*tidcommon.ServiceError)(nil)).Once()

		var out updateRoleAssignmentsMCPResponse
		s.structured(s.call("update_role_assignments", map[string]any{"assignees": assignees,
			"grant": []string{"support"}, "revoke": []string{"viewer"}}), &out)
		s.Equal([]roleAssignmentChangeMCPItem{{RoleID: "viewer", AssigneeID: "u1"}}, out.Revoked)
		s.Equal([]roleAssignmentChangeMCPItem{{RoleID: "support", AssigneeID: "u1"}}, out.Granted)
	})

	s.Run("a flow that finishes without revoking is refused", func() {
		s.connect(true)
		s.runner.On("RunAdministrationFlow", mock.Anything, roleAssignmentRemovalFlowKey, mock.Anything).
			Return(true, (*tidcommon.ServiceError)(nil)).Once()
		s.assignments.On("ValidateRemoveAssignment", mock.Anything, "viewer", "", "u1").
			Return(&revocation.AccessRevocationTarget{}, (*tidcommon.ServiceError)(nil)).Twice()
		text := s.errorText(s.call("update_role_assignments", map[string]any{"assignees": assignees,
			"grant": []string{"support"}, "revoke": []string{"viewer"}}))
		s.Contains(text, "the assignment is still there")
		s.Contains(text, "Nothing was granted")
		s.assignments.AssertNotCalled(s.T(), "AddAssignments", mock.Anything, mock.Anything, mock.Anything,
			mock.Anything)
	})

	s.Run("a role held only through a sharing policy is refused before the flow runs", func() {
		s.connect(true)
		s.assignments.On("ValidateRemoveAssignment", mock.Anything, "viewer", "", "u1").
			Return((*revocation.AccessRevocationTarget)(nil), &ErrorAssignmentMandated).Once()
		text := s.errorText(s.call("update_role_assignments", map[string]any{"assignees": assignees,
			"grant": []string{"support"}, "revoke": []string{"viewer"}}))
		s.Contains(text, ErrorAssignmentMandated.ErrorDescription.DefaultValue)
		s.Contains(text, "Nothing was granted")
		s.runner.AssertNotCalled(s.T(), "RunAdministrationFlow", mock.Anything, mock.Anything, mock.Anything)
	})

	s.Run("a refused revocation grants nothing", func() {
		s.connect(true)
		s.assignments.On("ValidateRemoveAssignment", mock.Anything, "viewer", "", "u1").
			Return(&revocation.AccessRevocationTarget{}, (*tidcommon.ServiceError)(nil)).Once()
		s.runner.On("RunAdministrationFlow", mock.Anything, roleAssignmentRemovalFlowKey, mock.Anything).
			Return(false, &tidcommon.ServiceError{ErrorDescription: tidcommon.I18nMessage{DefaultValue: "mandated"}}).
			Once()
		text := s.errorText(s.call("update_role_assignments", map[string]any{"assignees": assignees,
			"grant": []string{"support"}, "revoke": []string{"viewer"}}))
		s.Contains(text, "mandated")
		s.Contains(text, "Nothing was granted")
		s.assignments.AssertNotCalled(s.T(), "AddAssignments", mock.Anything, mock.Anything, mock.Anything,
			mock.Anything)
	})

	s.Run("no flow configured revokes directly in the given unit", func() {
		s.connect(true)
		s.assignments.On("ValidateRemoveAssignment", mock.Anything, "viewer", "ou-2", "u1").
			Return(&revocation.AccessRevocationTarget{}, (*tidcommon.ServiceError)(nil)).Once()
		s.runner.On("RunAdministrationFlow", mock.Anything, roleAssignmentRemovalFlowKey,
			map[string]string{flowInputRole: "viewer", flowInputAssignee: "u1", flowInputAssignmentOU: "ou-2"}).
			Return(false, (*tidcommon.ServiceError)(nil)).Once()
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

// The removal tools say what a removal does on this server: a data plane runs flows that revoke tokens,
// and a control plane writes removals directly.
func (s *RoleToolsTestSuite) TestRemovalNoticeFollowsThePlane() {
	for _, withRunner := range []bool{true, false} {
		s.connect(withRunner)
		result, err := s.session.ListTools(context.Background(), nil)
		s.Require().NoError(err)
		expected := flowRemovalNotice
		if !withRunner {
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
