// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ou

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
)

type ouTools struct {
	ouService OrganizationUnitServiceInterface
}

// ouMCPItem is the MCP list-response item. HTTP pagination fields
// (StartIndex, Count, Links) are excluded, and time.Time fields are
// serialized as strings to produce a stable JSON schema.
type ouMCPItem struct {
	ID          string `json:"id"`
	Handle      string `json:"handle"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	ProjectID   string `json:"projectId,omitempty"`
	LogoURL     string `json:"logoUrl,omitempty"`
	IsReadOnly  bool   `json:"isReadOnly"`
	CreatedAt   string `json:"createdAt,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
}

// ouListMCPInput is the MCP list tool input.
type ouListMCPInput struct {
	ProjectID string `json:"projectId,omitempty" jsonschema:"Project ID. Lists the project's top organization units."`
}

// ouListMCPResponse is the MCP list tool output.
type ouListMCPResponse struct {
	TotalResults      int         `json:"totalResults"`
	OrganizationUnits []ouMCPItem `json:"organizationUnits"`
}

// registerMCPTools registers all OU MCP tools with the server.
func registerMCPTools(server *mcp.Server, ouService OrganizationUnitServiceInterface) {
	tools := &ouTools{ouService: ouService}

	mcp.AddTool(server, &mcp.Tool{
		Name: "list_organization_units",
		Description: `List the root organization units.

Behavior:
- With projectId, lists the project's top organization units: its root organization units, and any unit
  whose parent is not in the project. A project that does not exist has no organization units.`,
		Annotations: &mcp.ToolAnnotations{
			Title:        "List Organization Units",
			ReadOnlyHint: true,
		},
	}, tools.listOrganizationUnits)
}

// listOrganizationUnits handles the list_organization_units tool call.
func (t *ouTools) listOrganizationUnits(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input ouListMCPInput,
) (*mcp.CallToolResult, *ouListMCPResponse, error) {
	var resp *OrganizationUnitListResponse
	var svcErr *tidcommon.ServiceError
	if input.ProjectID != "" {
		resp, svcErr = t.ouService.GetOrganizationUnitListByProject(ctx, input.ProjectID, serverconst.MaxPageSize,
			0, nil)
	} else {
		resp, svcErr = t.ouService.GetOrganizationUnitList(ctx, serverconst.MaxPageSize, 0, nil)
	}
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to list organization units: %s", svcErr.ErrorDescription)
	}

	items := make([]ouMCPItem, len(resp.OrganizationUnits))
	for i, ou := range resp.OrganizationUnits {
		items[i] = ouMCPItem{
			ID:          ou.ID,
			Handle:      ou.Handle,
			Name:        ou.Name,
			Description: ou.Description,
			ProjectID:   ou.ProjectID,
			LogoURL:     ou.LogoURL,
			IsReadOnly:  ou.IsReadOnly,
			CreatedAt:   ou.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:   ou.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	return nil, &ouListMCPResponse{
		TotalResults:      resp.TotalResults,
		OrganizationUnits: items,
	}, nil
}
