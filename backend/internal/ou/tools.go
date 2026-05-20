/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package ou

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

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
	LogoURL     string `json:"logoUrl,omitempty"`
	IsReadOnly  bool   `json:"isReadOnly"`
	CreatedAt   string `json:"createdAt,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
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
		Name:        "thunderid_list_organization_units",
		Description: `List all organization units.`,
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
	_ any,
) (*mcp.CallToolResult, *ouListMCPResponse, error) {
	resp, svcErr := t.ouService.GetOrganizationUnitList(ctx, serverconst.MaxPageSize, 0, nil)
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
