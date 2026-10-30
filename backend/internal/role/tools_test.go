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

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

const toolsRoleID = "role-1"

type RoleToolsTestSuite struct {
	suite.Suite
	mockService           *RoleServiceInterfaceMock
	mockAssignmentService *RoleAssignmentServiceInterfaceMock
	session               *mcp.ClientSession
}

func TestRoleToolsTestSuite(t *testing.T) {
	suite.Run(t, new(RoleToolsTestSuite))
}

// SetupTest registers the tools over mocked role services and connects an in-memory MCP client, so
// every call crosses the SDK's schema validation and output encoding as an agent's would.
func (s *RoleToolsTestSuite) SetupTest() {
	s.mockService = NewRoleServiceInterfaceMock(s.T())
	s.mockAssignmentService = NewRoleAssignmentServiceInterfaceMock(s.T())

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "1.0.0"}, nil)
	registerMCPTools(server, s.mockService, s.mockAssignmentService)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx := context.Background()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = serverSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	s.session, err = client.Connect(ctx, clientTransport, nil)
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = s.session.Close() })
}

func (s *RoleToolsTestSuite) call(name string, args map[string]any) *mcp.CallToolResult {
	result, err := s.session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	s.Require().NoError(err)
	return result
}

func (s *RoleToolsTestSuite) errorText(result *mcp.CallToolResult) string {
	s.Require().True(result.IsError)
	s.Require().NotEmpty(result.Content)
	text, ok := result.Content[0].(*mcp.TextContent)
	s.Require().True(ok)
	return text.Text
}

func (s *RoleToolsTestSuite) structured(result *mcp.CallToolResult, out any) {
	s.Require().False(result.IsError, "tool returned an error: %v", result.Content)
	raw, err := json.Marshal(result.StructuredContent)
	s.Require().NoError(err)
	s.Require().NoError(json.Unmarshal(raw, out))
}

func serviceError(description string) *tidcommon.ServiceError {
	return &tidcommon.ServiceError{ErrorDescription: tidcommon.I18nMessage{DefaultValue: description}}
}

func storedRole() *RoleWithPermissions {
	return &RoleWithPermissions{
		ID: toolsRoleID, Name: "Support", Description: "Handles tickets", OUID: "ou-1", OUHandle: "support",
		Permissions: []ResourcePermissions{
			{ResourceServerID: "rs-tickets", Permissions: []string{"tickets:read", "tickets:write"}},
			{ResourceServerID: "rs-billing", Permissions: []string{"invoices:read"}},
		},
	}
}

func (s *RoleToolsTestSuite) TestRegisterMCPTools() {
	result, err := s.session.ListTools(context.Background(), nil)
	s.Require().NoError(err)

	tools := map[string]*mcp.Tool{}
	for _, registered := range result.Tools {
		tools[registered.Name] = registered
	}
	s.Len(tools, 7)
	for _, name := range []string{"list_roles", "get_role"} {
		s.Require().Contains(tools, name)
		s.True(tools[name].Annotations.ReadOnlyHint, name)
	}
	for _, name := range []string{"revoke_role", "change_role_permissions",
		"delete_role"} {
		s.Require().Contains(tools, name)
		s.Require().NotNil(tools[name].Annotations.DestructiveHint, name)
		s.True(*tools[name].Annotations.DestructiveHint, name)
	}
	s.Contains(tools, "create_role")
	s.Contains(tools, "grant_role")

	grantSchema, err := json.Marshal(tools["grant_role"].InputSchema)
	s.Require().NoError(err)
	s.Contains(string(grantSchema), `"enum":["user","group","agent","app"]`)
	s.NotContains(string(grantSchema), `"display"`)
}

func (s *RoleToolsTestSuite) TestListRoles() {
	s.mockService.On("GetRoleList", mock.Anything, 20, 0).Return(&RoleList{
		TotalResults: 1,
		Roles:        []Role{{ID: toolsRoleID, Name: "Support", OUID: "ou-1", OUHandle: "support", IsReadOnly: true}},
	}, (*tidcommon.ServiceError)(nil)).Once()

	var out roleListMCPResponse
	s.structured(s.call("list_roles", map[string]any{}), &out)

	s.Equal(1, out.TotalResults)
	s.Equal([]roleMCPItem{{ID: toolsRoleID, Name: "Support", OUID: "ou-1", OUHandle: "support", ReadOnly: true}},
		out.Roles)
}

func (s *RoleToolsTestSuite) TestListRoles_Error() {
	s.mockService.On("GetRoleList", mock.Anything, 5, 10).Return(nil, serviceError("Invalid limit")).Once()

	result := s.call("list_roles", map[string]any{"limit": 5, "offset": 10})

	s.Equal("failed to list roles: Invalid limit", s.errorText(result))
}

func (s *RoleToolsTestSuite) TestGetRole() {
	s.mockService.On("GetRoleWithPermissions", mock.Anything, toolsRoleID).
		Return(storedRole(), (*tidcommon.ServiceError)(nil)).Once()
	s.mockAssignmentService.On("GetRoleAssignments", mock.Anything, toolsRoleID, roleAssigneePageSize, 0, true).
		Return(&AssignmentList{TotalResults: 2, Assignments: []RoleAssignmentWithDisplay{
			{ID: "user-1", Type: AssigneeTypeUser, Display: "alice"},
			{ID: "group-1", Type: AssigneeTypeGroup, Display: "Support Team"},
		}}, (*tidcommon.ServiceError)(nil)).Once()

	var out roleMCPOutput
	s.structured(s.call("get_role", map[string]any{"id": toolsRoleID}), &out)

	s.Equal("Support", out.Name)
	s.Equal("support", out.OUHandle)
	s.Len(out.Permissions, 2)
	s.Equal([]roleMCPAssignee{
		{ID: "user-1", Type: "user", Display: "alice"},
		{ID: "group-1", Type: "group", Display: "Support Team"},
	}, out.Assignees)
	s.Equal(2, out.TotalAssignees)
}

func (s *RoleToolsTestSuite) TestGetRole_Errors() {
	s.Run("role", func() {
		s.mockService.On("GetRoleWithPermissions", mock.Anything, "missing").
			Return(nil, serviceError("Role not found")).Once()
		s.Equal("failed to get role: Role not found",
			s.errorText(s.call("get_role", map[string]any{"id": "missing"})))
	})
	s.Run("assignments", func() {
		s.mockService.On("GetRoleWithPermissions", mock.Anything, toolsRoleID).
			Return(storedRole(), (*tidcommon.ServiceError)(nil)).Once()
		s.mockAssignmentService.On("GetRoleAssignments", mock.Anything, toolsRoleID, roleAssigneePageSize, 0, true).
			Return(nil, &tidcommon.InternalServerError).Once()
		s.Contains(s.errorText(s.call("get_role", map[string]any{"id": toolsRoleID})),
			"failed to get role: ")
	})
}

func (s *RoleToolsTestSuite) TestCreateRole() {
	var received RoleCreationDetail
	s.mockService.On("CreateRole", mock.Anything, mock.Anything).
		Return(func(_ context.Context, detail RoleCreationDetail) (
			*RoleWithPermissionsAndAssignments, *tidcommon.ServiceError) {
			received = detail
			return &RoleWithPermissionsAndAssignments{
				ID: toolsRoleID, Name: detail.Name, OUID: detail.OUID, Permissions: detail.Permissions,
				Assignments: detail.Assignments,
			}, nil
		}).Once()

	var out roleMCPOutput
	s.structured(s.call("create_role", map[string]any{
		"name": "  Auditor ", "ouId": "ou-1",
		"permissions": []any{map[string]any{"resourceServerId": "rs-reports", "permissions": []any{"reports:read"}}},
		"assignees":   []any{map[string]any{"id": "group-1", "type": "group"}},
	}), &out)

	s.Equal("Auditor", received.Name)
	s.Equal("ou-1", received.OUID)
	s.Equal([]ResourcePermissions{{ResourceServerID: "rs-reports", Permissions: []string{"reports:read"}}},
		received.Permissions)
	s.Equal([]RoleAssignment{{ID: "group-1", Type: AssigneeTypeGroup}}, received.Assignments)
	s.Equal(toolsRoleID, out.ID)
	s.Equal([]roleMCPAssignee{{ID: "group-1", Type: "group"}}, out.Assignees)
	s.Equal(1, out.TotalAssignees)
}

func (s *RoleToolsTestSuite) TestCreateRole_Errors() {
	s.Run("service", func() {
		s.mockService.On("CreateRole", mock.Anything, mock.Anything).
			Return(nil, serviceError("Role name already exists")).Once()
		s.Equal("failed to create role: Role name already exists", s.errorText(
			s.call("create_role", map[string]any{"name": "Support", "ouId": "ou-1"})))
	})
	s.Run("unknown assignee type", func() {
		result := s.call("create_role", map[string]any{"name": "Support", "ouId": "ou-1",
			"assignees": []any{map[string]any{"id": "x", "type": "robot"}}})
		s.True(result.IsError)
	})
}

func (s *RoleToolsTestSuite) TestGrantRole() {
	assignments := []RoleAssignment{{ID: "agent-1", Type: AssigneeTypeAgent}, {ID: "user-1", Type: AssigneeTypeUser}}
	s.mockAssignmentService.On("AddAssignments", mock.Anything, toolsRoleID, assignments).
		Return((*tidcommon.ServiceError)(nil)).Once()

	var out roleAssigneesMCPResponse
	s.structured(s.call("grant_role", map[string]any{"id": toolsRoleID, "assignees": []any{
		map[string]any{"id": "agent-1", "type": "agent"}, map[string]any{"id": "user-1", "type": "user"},
	}}), &out)

	s.Equal(toolsRoleID, out.ID)
	s.Len(out.Assignees, 2)
}

func (s *RoleToolsTestSuite) TestGrantRole_Error() {
	s.mockAssignmentService.On("AddAssignments", mock.Anything, toolsRoleID, mock.Anything).
		Return(serviceError("Assignee not found")).Once()

	result := s.call("grant_role", map[string]any{"id": toolsRoleID,
		"assignees": []any{map[string]any{"id": "nobody", "type": "user"}}})

	s.Equal("failed to grant role: Assignee not found", s.errorText(result))
}

func (s *RoleToolsTestSuite) TestRevokeRole() {
	s.mockAssignmentService.On("RemoveAssignments", mock.Anything, toolsRoleID,
		[]RoleAssignment{{ID: "group-1", Type: AssigneeTypeGroup}}).Return((*tidcommon.ServiceError)(nil)).Once()

	var out roleAssigneesMCPResponse
	s.structured(s.call("revoke_role", map[string]any{"id": toolsRoleID,
		"assignees": []any{map[string]any{"id": "group-1", "type": "group"}}}), &out)

	s.Equal([]roleMCPAssignee{{ID: "group-1", Type: "group"}}, out.Assignees)
}

func (s *RoleToolsTestSuite) TestRevokeRole_Error() {
	s.mockAssignmentService.On("RemoveAssignments", mock.Anything, toolsRoleID, mock.Anything).
		Return(serviceError("Cannot modify a declarative role")).Once()

	result := s.call("revoke_role", map[string]any{"id": toolsRoleID,
		"assignees": []any{map[string]any{"id": "group-1", "type": "group"}}})

	s.Equal("failed to revoke role: Cannot modify a declarative role", s.errorText(result))
}

func (s *RoleToolsTestSuite) TestChangeRolePermissions() {
	var received RoleUpdateDetail
	s.mockService.On("GetRoleWithPermissions", mock.Anything, toolsRoleID).
		Return(storedRole(), (*tidcommon.ServiceError)(nil)).Once()
	s.mockService.On("UpdateRoleWithPermissions", mock.Anything, toolsRoleID, mock.Anything).
		Return(func(_ context.Context, _ string, detail RoleUpdateDetail) (
			*RoleWithPermissions, *tidcommon.ServiceError) {
			received = detail
			return &RoleWithPermissions{ID: toolsRoleID, Name: detail.Name, OUID: detail.OUID,
				Permissions: detail.Permissions}, nil
		}).Once()

	var out roleMCPOutput
	s.structured(s.call("change_role_permissions", map[string]any{
		"id": toolsRoleID,
		"add": []any{
			map[string]any{"resourceServerId": "rs-tickets", "permissions": []any{"tickets:refund", "tickets:read"}},
			map[string]any{"resourceServerId": "rs-reports", "permissions": []any{"reports:read"}},
		},
		"remove": []any{
			map[string]any{"resourceServerId": "rs-tickets", "permissions": []any{"tickets:write"}},
			map[string]any{"resourceServerId": "rs-billing", "permissions": []any{"invoices:read"}},
		},
	}), &out)

	s.Equal("Support", received.Name)
	s.Equal("Handles tickets", received.Description)
	s.Equal("ou-1", received.OUID)
	s.Equal([]ResourcePermissions{
		{ResourceServerID: "rs-tickets", Permissions: []string{"tickets:read", "tickets:refund"}},
		{ResourceServerID: "rs-reports", Permissions: []string{"reports:read"}},
	}, received.Permissions)
	s.Len(out.Permissions, 2)
}

func (s *RoleToolsTestSuite) TestChangeRolePermissions_Errors() {
	s.Run("nothing to change", func() {
		s.Equal("failed to change role permissions: give the permissions to add or remove",
			s.errorText(s.call("change_role_permissions", map[string]any{"id": toolsRoleID})))
	})
	s.Run("read", func() {
		s.mockService.On("GetRoleWithPermissions", mock.Anything, "missing").
			Return(nil, serviceError("Role not found")).Once()
		s.Equal("failed to change role permissions: Role not found", s.errorText(
			s.call("change_role_permissions", map[string]any{"id": "missing",
				"remove": []any{map[string]any{"resourceServerId": "rs", "permissions": []any{"p"}}}})))
	})
	s.Run("update", func() {
		s.mockService.On("GetRoleWithPermissions", mock.Anything, toolsRoleID).
			Return(storedRole(), (*tidcommon.ServiceError)(nil)).Once()
		s.mockService.On("UpdateRoleWithPermissions", mock.Anything, toolsRoleID, mock.Anything).
			Return(nil, serviceError("Invalid permissions")).Once()
		s.Equal("failed to change role permissions: Invalid permissions", s.errorText(
			s.call("change_role_permissions", map[string]any{"id": toolsRoleID,
				"add": []any{map[string]any{"resourceServerId": "rs", "permissions": []any{"unknown"}}}})))
	})
}

func (s *RoleToolsTestSuite) TestDeleteRole() {
	s.mockService.On("DeleteRole", mock.Anything, toolsRoleID).Return((*tidcommon.ServiceError)(nil)).Once()

	var out roleDeleteMCPResponse
	s.structured(s.call("delete_role", map[string]any{"id": toolsRoleID}), &out)

	s.Equal(roleDeleteMCPResponse{ID: toolsRoleID, Deleted: true}, out)
}

func (s *RoleToolsTestSuite) TestDeleteRole_Error() {
	s.mockService.On("DeleteRole", mock.Anything, toolsRoleID).
		Return(serviceError("Cannot modify a declarative role")).Once()

	result := s.call("delete_role", map[string]any{"id": toolsRoleID})

	s.Equal("failed to delete role: Cannot modify a declarative role", s.errorText(result))
}
