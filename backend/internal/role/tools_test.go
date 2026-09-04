// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/mcp/tool"
)

type RoleToolsTestSuite struct {
	suite.Suite

	roleService       *RoleServiceInterfaceMock
	assignmentService *RoleAssignmentServiceInterfaceMock
	tools             *roleTools
}

func TestRoleToolsTestSuite(t *testing.T) {
	suite.Run(t, new(RoleToolsTestSuite))
}

func (suite *RoleToolsTestSuite) SetupTest() {
	suite.roleService = NewRoleServiceInterfaceMock(suite.T())
	suite.assignmentService = NewRoleAssignmentServiceInterfaceMock(suite.T())

	suite.tools = &roleTools{
		roleService:       suite.roleService,
		assignmentService: suite.assignmentService,
	}
}

func (suite *RoleToolsTestSuite) TestListRoles() {
	suite.roleService.On(
		"GetRoleList",
		mock.Anything,
		30,
		0,
	).Return(&RoleList{
		TotalResults: 1,
		Roles: []Role{
			{ID: "role1", Name: "Admin"},
		},
	}, nil)

	_, output, err := suite.tools.listRoles(
		context.Background(),
		&mcp.CallToolRequest{},
		tool.PaginationInput{},
	)

	suite.NoError(err)
	suite.Equal(1, output.TotalResults)
	suite.Len(output.Roles, 1)
	suite.Equal("role1", output.Roles[0].ID)
}

func (suite *RoleToolsTestSuite) TestGetRole() {
	suite.roleService.On(
		"GetRoleWithPermissions",
		mock.Anything,
		"role1",
	).Return(&RoleWithPermissions{
		ID:   "role1",
		Name: "Admin",
		OUID: "ou1",
	}, nil)

	_, output, err := suite.tools.getRole(
		context.Background(),
		&mcp.CallToolRequest{},
		tool.IDInput{ID: "role1"},
	)

	suite.NoError(err)
	suite.Equal("role1", output.ID)
	suite.Equal("Admin", output.Name)
}

func (suite *RoleToolsTestSuite) TestCreateRole() {
	suite.roleService.On(
		"CreateRole",
		mock.Anything,
		mock.AnythingOfType("RoleCreationDetail"),
	).Return(&RoleWithPermissionsAndAssignments{
		ID:   "role1",
		Name: "Admin",
		OUID: "ou1",
	}, nil)

	_, output, err := suite.tools.createRole(
		context.Background(),
		&mcp.CallToolRequest{},
		CreateRoleRequest{
			Name: "Admin",
			OUID: "ou1",
		},
	)

	suite.NoError(err)
	suite.Equal("role1", output.ID)
	suite.Equal("Admin", output.Name)
}

func (suite *RoleToolsTestSuite) TestUpdateRole() {
	suite.roleService.On(
		"UpdateRoleWithPermissions",
		mock.Anything,
		"role1",
		mock.AnythingOfType("RoleUpdateDetail"),
	).Return(&RoleWithPermissions{
		ID:   "role1",
		Name: "Updated Admin",
		OUID: "ou1",
	}, nil)

	_, output, err := suite.tools.updateRole(
		context.Background(),
		&mcp.CallToolRequest{},
		roleUpdateInput{
			ID:   "role1",
			Name: "Updated Admin",
			OUID: "ou1",
		},
	)

	suite.NoError(err)
	suite.Equal("Updated Admin", output.Name)
}

func (suite *RoleToolsTestSuite) TestDeleteRole() {
	suite.roleService.On(
		"DeleteRole",
		mock.Anything,
		"role1",
	).Return(nil)

	_, output, err := suite.tools.deleteRole(
		context.Background(),
		&mcp.CallToolRequest{},
		tool.IDInput{ID: "role1"},
	)

	suite.NoError(err)
	suite.True(output.Success)
}

func (suite *RoleToolsTestSuite) TestGetRoleAssignments() {
	suite.assignmentService.On(
		"GetRoleAssignmentsByType",
		mock.Anything,
		"role1",
		30,
		0,
		false,
		"",
	).Return(&AssignmentList{
		TotalResults: 1,
		Assignments: []RoleAssignmentWithDisplay{
			{
				ID:   "user1",
				Type: AssigneeTypeUser,
			},
		},
	}, nil)

	_, output, err := suite.tools.getRoleAssignments(
		context.Background(),
		&mcp.CallToolRequest{},
		roleAssignmentsInput{
			ID: "role1",
		},
	)

	suite.NoError(err)
	suite.Equal(1, output.TotalResults)
	suite.Len(output.Assignments, 1)
	suite.Equal("user1", output.Assignments[0].ID)
}

func (suite *RoleToolsTestSuite) TestAddRoleAssignments() {
	suite.assignmentService.On(
		"AddAssignments",
		mock.Anything,
		"role1",
		mock.AnythingOfType("[]role.RoleAssignment"),
	).Return(nil)

	_, output, err := suite.tools.addRoleAssignments(
		context.Background(),
		&mcp.CallToolRequest{},
		addRoleAssignmentsInput{
			ID: "role1",
			Assignments: []AssignmentRequest{
				{
					ID:   "user1",
					Type: AssigneeTypeUser,
				},
			},
		},
	)

	suite.NoError(err)
	suite.True(output.Success)
}

func (suite *RoleToolsTestSuite) TestRegisterMCPTools() {
	ctx := context.Background()
	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    "test-server",
			Version: "1.0.0",
		},
		nil,
	)

	registerMCPTools(
		server,
		suite.roleService,
		suite.assignmentService,
	)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	suite.Require().NoError(err)
	suite.T().Cleanup(func() {
		suite.NoError(serverSession.Close())
	})

	client := mcp.NewClient(
		&mcp.Implementation{
			Name:    "test-client",
			Version: "1.0.0",
		},
		nil,
	)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	suite.Require().NoError(err)
	suite.T().Cleanup(func() {
		suite.NoError(clientSession.Close())
	})

	result, err := clientSession.ListTools(ctx, nil)
	suite.Require().NoError(err)

	toolNames := make([]string, len(result.Tools))
	for i, registeredTool := range result.Tools {
		toolNames[i] = registeredTool.Name
	}

	suite.ElementsMatch([]string{
		"thunderid_list_roles",
		"thunderid_get_role",
		"thunderid_create_role",
		"thunderid_update_role",
		"thunderid_delete_role",
		"thunderid_get_role_assignments",
		"thunderid_add_role_assignments",
	}, toolNames)
}

func (suite *RoleToolsTestSuite) TestSchemas() {
	suite.NotNil(getCreateRoleSchema())
	suite.ElementsMatch(
		[]string{"id", "name", "ouId", "permissions"},
		getUpdateRoleSchema().Required,
	)
	suite.NotNil(getRoleAssignmentsSchema())
	suite.NotNil(getAddRoleAssignmentsSchema())
}
