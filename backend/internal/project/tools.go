// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/mcp/tool"
)

type projectTools struct {
	service ProjectServiceInterface
}

// projectMCPItem is the MCP project output. HTTP pagination fields are excluded, and the creation
// time is serialized as a string to produce a stable JSON schema.
type projectMCPItem struct {
	ID          string `json:"id"`
	Handle      string `json:"handle"`
	DisplayName string `json:"displayName"`
	Description string `json:"description,omitempty"`
	RootOUID    string `json:"rootOuId"`
	CreatedAt   string `json:"createdAt"`
}

// projectListMCPResponse is the MCP list tool output.
type projectListMCPResponse struct {
	TotalResults int              `json:"totalResults"`
	Projects     []projectMCPItem `json:"projects"`
}

// createProjectMCPInput is the input of the create_project tool.
//
//nolint:lll
type createProjectMCPInput struct {
	ID          string `json:"id,omitempty" jsonschema:"Optional UUID to use as the project ID. Omit it to have one generated."`
	Handle      string `json:"handle" jsonschema:"Immutable identifier of the project. It becomes the root organization unit's handle, so organization unit handle rules apply and it must not contain /."`
	DisplayName string `json:"displayName" jsonschema:"Display name of the project, at most 100 characters"`
	Description string `json:"description,omitempty" jsonschema:"Optional description, at most 255 characters"`
}

// updateProjectMCPInput is the input of the update_project tool.
//
//nolint:lll
type updateProjectMCPInput struct {
	ID          string `json:"id" jsonschema:"ID of the project to update"`
	DisplayName string `json:"displayName" jsonschema:"New display name of the project, at most 100 characters"`
	Description string `json:"description,omitempty" jsonschema:"New description, at most 255 characters. Omit it to clear the description."`
}

// registerMCPTools registers all project MCP tools with the server.
func registerMCPTools(server *mcp.Server, service ProjectServiceInterface) {
	tools := &projectTools{service: service}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_projects",
		Description: `List all projects, oldest first. Each project has exactly one root organization unit.`,
		Annotations: &mcp.ToolAnnotations{
			Title:        "List Projects",
			ReadOnlyHint: true,
		},
	}, tools.listProjects)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_project_by_id",
		Description: `Retrieve a project by ID, including the ID of its root organization unit.`,
		Annotations: &mcp.ToolAnnotations{
			Title:        "Get Project by ID",
			ReadOnlyHint: true,
		},
	}, tools.getProjectByID)

	mcp.AddTool(server, &mcp.Tool{
		Name: "create_project",
		Description: `Create a project together with its root organization unit. The organization unit ` +
			`takes the project's handle, display name and description. A handle already used by another ` +
			`root organization unit is refused.`,
		Annotations: &mcp.ToolAnnotations{
			Title:          "Create Project",
			IdempotentHint: false,
		},
	}, tools.createProject)

	mcp.AddTool(server, &mcp.Tool{
		Name: "update_project",
		Description: `Update the display name and description of a project (full replacement of both). ` +
			`They are the root organization unit's name and description, so the root organization unit is ` +
			`renamed too. The handle cannot change.`,
		Annotations: &mcp.ToolAnnotations{
			Title:          "Update Project",
			IdempotentHint: true,
		},
	}, tools.updateProject)
}

// listProjects handles the list_projects tool call, reading every page.
func (t *projectTools) listProjects(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	_ any,
) (*mcp.CallToolResult, *projectListMCPResponse, error) {
	items := []projectMCPItem{}
	totalResults := 0
	offset := 0

	for {
		resp, svcErr := t.service.GetProjectList(ctx, serverconst.MaxPageSize, offset)
		if svcErr != nil {
			return nil, nil, fmt.Errorf("failed to list projects: %s", svcErr.ErrorDescription)
		}
		if offset == 0 {
			totalResults = resp.TotalResults
		}
		for _, project := range resp.Projects {
			items = append(items, toProjectMCPItem(project))
		}
		offset += len(resp.Projects)
		if offset >= totalResults || len(resp.Projects) == 0 {
			break
		}
	}

	return nil, &projectListMCPResponse{
		TotalResults: totalResults,
		Projects:     items,
	}, nil
}

// getProjectByID handles the get_project_by_id tool call.
func (t *projectTools) getProjectByID(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input tool.IDInput,
) (*mcp.CallToolResult, *projectMCPItem, error) {
	project, svcErr := t.service.GetProject(ctx, input.ID)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to get project: %s", svcErr.ErrorDescription)
	}
	item := toProjectMCPItem(*project)
	return nil, &item, nil
}

// createProject handles the create_project tool call. The input is sanitized as the REST request is.
func (t *projectTools) createProject(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input createProjectMCPInput,
) (*mcp.CallToolResult, *projectMCPItem, error) {
	project, svcErr := t.service.CreateProject(ctx, sanitizeProjectRequest(ProjectRequest(input)))
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to create project: %s", svcErr.ErrorDescription)
	}
	item := toProjectMCPItem(*project)
	return nil, &item, nil
}

// updateProject handles the update_project tool call. The input is sanitized as the REST request is.
func (t *projectTools) updateProject(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input updateProjectMCPInput,
) (*mcp.CallToolResult, *projectMCPItem, error) {
	project, svcErr := t.service.UpdateProject(ctx, input.ID, sanitizeProjectRequest(ProjectRequest{
		DisplayName: input.DisplayName,
		Description: input.Description,
	}))
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to update project: %s", svcErr.ErrorDescription)
	}
	item := toProjectMCPItem(*project)
	return nil, &item, nil
}

// toProjectMCPItem converts a project to its MCP output.
func toProjectMCPItem(project Project) projectMCPItem {
	return projectMCPItem{
		ID:          project.ID,
		Handle:      project.Handle,
		DisplayName: project.DisplayName,
		Description: project.Description,
		RootOUID:    project.RootOUID,
		CreatedAt:   project.CreatedAt.Format(time.RFC3339Nano),
	}
}
