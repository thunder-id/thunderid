// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

type ProjectToolsTestSuite struct {
	suite.Suite
	mockService *ProjectServiceInterfaceMock
	session     *mcp.ClientSession
}

func TestProjectToolsTestSuite(t *testing.T) {
	suite.Run(t, new(ProjectToolsTestSuite))
}

// SetupTest registers the tools over a mocked project service and connects an in-memory MCP client,
// so every call crosses the SDK's schema validation and output encoding as an agent's would.
func (s *ProjectToolsTestSuite) SetupTest() {
	s.mockService = NewProjectServiceInterfaceMock(s.T())

	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "1.0.0"}, nil)
	registerMCPTools(server, s.mockService)
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

func (s *ProjectToolsTestSuite) project(id string) *Project {
	return &Project{
		ID: id, Handle: "shop", DisplayName: "Shop", Description: "Online shop", RootOUID: testRootOUID,
		CreatedAt: time.Date(2026, 10, 23, 8, 30, 0, 123456000, time.UTC),
	}
}

func (s *ProjectToolsTestSuite) call(name string, args map[string]any) *mcp.CallToolResult {
	result, err := s.session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	s.Require().NoError(err)
	return result
}

func (s *ProjectToolsTestSuite) structured(result *mcp.CallToolResult, out any) {
	s.Require().False(result.IsError, "tool returned an error: %v", result.Content)
	raw, err := json.Marshal(result.StructuredContent)
	s.Require().NoError(err)
	s.Require().NoError(json.Unmarshal(raw, out))
}

func (s *ProjectToolsTestSuite) errorText(result *mcp.CallToolResult) string {
	s.Require().True(result.IsError)
	s.Require().NotEmpty(result.Content)
	text, ok := result.Content[0].(*mcp.TextContent)
	s.Require().True(ok)
	return text.Text
}

func (s *ProjectToolsTestSuite) TestRegisterMCPTools() {
	result, err := s.session.ListTools(context.Background(), nil)
	s.Require().NoError(err)

	tools := map[string]*mcp.Tool{}
	for _, tool := range result.Tools {
		tools[tool.Name] = tool
	}
	s.Len(tools, 4)
	for _, name := range []string{"list_projects", "get_project_by_id"} {
		s.Require().Contains(tools, name)
		s.True(tools[name].Annotations.ReadOnlyHint, name)
	}
	s.Require().Contains(tools, "create_project")
	s.False(tools["create_project"].Annotations.ReadOnlyHint)
	s.False(tools["create_project"].Annotations.IdempotentHint)
	s.Require().Contains(tools, "update_project")
	s.False(tools["update_project"].Annotations.ReadOnlyHint)
	s.True(tools["update_project"].Annotations.IdempotentHint)
}

func (s *ProjectToolsTestSuite) TestListProjects() {
	s.Run("returns every project", func() {
		s.SetupTest()
		s.mockService.On("GetProjectList", mock.Anything, serverconst.MaxPageSize, 0).
			Return(&ProjectListResponse{TotalResults: 1, Projects: []Project{*s.project(testProjectID)}}, nil)

		var out projectListMCPResponse
		s.structured(s.call("list_projects", map[string]any{}), &out)

		s.Equal(1, out.TotalResults)
		s.Equal([]projectMCPItem{{
			ID: testProjectID, Handle: "shop", DisplayName: "Shop", Description: "Online shop",
			RootOUID: testRootOUID, CreatedAt: "2026-10-23T08:30:00.123456Z",
		}}, out.Projects)
	})
	s.Run("reads every page", func() {
		s.SetupTest()
		s.mockService.On("GetProjectList", mock.Anything, serverconst.MaxPageSize, 0).
			Return(&ProjectListResponse{TotalResults: 2, Projects: []Project{*s.project("p1")}}, nil)
		s.mockService.On("GetProjectList", mock.Anything, serverconst.MaxPageSize, 1).
			Return(&ProjectListResponse{TotalResults: 2, Projects: []Project{*s.project("p2")}}, nil)

		var out projectListMCPResponse
		s.structured(s.call("list_projects", map[string]any{}), &out)

		s.Equal(2, out.TotalResults)
		s.Require().Len(out.Projects, 2)
		s.Equal("p2", out.Projects[1].ID)
	})
	s.Run("no projects", func() {
		s.SetupTest()
		s.mockService.On("GetProjectList", mock.Anything, serverconst.MaxPageSize, 0).
			Return(&ProjectListResponse{Projects: []Project{}}, nil)

		var out projectListMCPResponse
		s.structured(s.call("list_projects", map[string]any{}), &out)

		s.Equal(0, out.TotalResults)
		s.Empty(out.Projects)
	})
	s.Run("service error", func() {
		s.SetupTest()
		s.mockService.On("GetProjectList", mock.Anything, serverconst.MaxPageSize, 0).
			Return(nil, &tidcommon.InternalServerError)

		s.Contains(s.errorText(s.call("list_projects", map[string]any{})), "failed to list projects")
	})
}

func (s *ProjectToolsTestSuite) TestGetProjectByID() {
	s.Run("returns the project", func() {
		s.SetupTest()
		s.mockService.On("GetProject", mock.Anything, testProjectID).Return(s.project(testProjectID), nil)

		var out projectMCPItem
		s.structured(s.call("get_project_by_id", map[string]any{"id": testProjectID}), &out)

		s.Equal(testProjectID, out.ID)
		s.Equal(testRootOUID, out.RootOUID)
	})
	s.Run("not found", func() {
		s.SetupTest()
		s.mockService.On("GetProject", mock.Anything, testProjectID).Return(nil, &ErrorProjectNotFound)

		text := s.errorText(s.call("get_project_by_id", map[string]any{"id": testProjectID}))
		s.Contains(text, "failed to get project")
		s.Contains(text, ErrorProjectNotFound.ErrorDescription.DefaultValue)
	})
}

func (s *ProjectToolsTestSuite) TestCreateProject() {
	s.Run("creates the project from the input", func() {
		s.SetupTest()
		s.mockService.On("CreateProject", mock.Anything, ProjectRequest{
			ID: testProjectID, Handle: "shop", DisplayName: "Shop", Description: "Online shop",
		}).Return(s.project(testProjectID), nil)

		var out projectMCPItem
		s.structured(s.call("create_project", map[string]any{
			"id": testProjectID, "handle": "shop", "displayName": "Shop", "description": "Online shop",
		}), &out)

		s.Equal(testProjectID, out.ID)
		s.Equal("shop", out.Handle)
	})
	s.Run("handle and display name are required", func() {
		s.SetupTest()

		text := s.errorText(s.call("create_project", map[string]any{"displayName": "Shop"}))

		s.Contains(text, "handle")
		s.mockService.AssertNotCalled(s.T(), "CreateProject", mock.Anything, mock.Anything)
	})
	s.Run("service error", func() {
		s.SetupTest()
		s.mockService.On("CreateProject", mock.Anything, mock.Anything).Return(nil, &ErrorInvalidDisplayName)

		text := s.errorText(s.call("create_project", map[string]any{"handle": "shop", "displayName": " "}))
		s.Contains(text, "failed to create project")
	})
}

func (s *ProjectToolsTestSuite) TestUpdateProject() {
	s.Run("updates the display name and description", func() {
		s.SetupTest()
		s.mockService.On("UpdateProject", mock.Anything, testProjectID, ProjectRequest{
			DisplayName: "Shop (EU)", Description: "EU shop",
		}).Return(s.project(testProjectID), nil)

		var out projectMCPItem
		s.structured(s.call("update_project", map[string]any{
			"id": testProjectID, "displayName": "Shop (EU)", "description": "EU shop",
		}), &out)

		s.Equal(testProjectID, out.ID)
	})
	s.Run("service error", func() {
		s.SetupTest()
		s.mockService.On("UpdateProject", mock.Anything, testProjectID, mock.Anything).
			Return(nil, &ErrorProjectNotFound)

		text := s.errorText(s.call("update_project", map[string]any{
			"id": testProjectID, "displayName": "Shop",
		}))
		s.Contains(text, "failed to update project")
	})
}
