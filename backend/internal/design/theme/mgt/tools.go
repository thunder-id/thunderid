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

package thememgt

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/mcp/tool"
)

type themeTools struct {
	themeMgtService ThemeMgtServiceInterface
}

// themeSummary is the MCP list-response item. It omits the raw theme JSON blob
// so the output schema is derivable without json.RawMessage (which the schema
// generator misidentifies as an array of bytes).
type themeSummary struct {
	ID          string `json:"id"`
	Handle      string `json:"handle"`
	DisplayName string `json:"displayName"`
	Description string `json:"description,omitempty"`
	IsReadOnly  bool   `json:"isReadOnly"`
	CreatedAt   string `json:"createdAt,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
}

// themeListMCPResponse is the MCP list tool output.
type themeListMCPResponse struct {
	TotalResults int            `json:"totalResults"`
	Themes       []themeSummary `json:"themes"`
}

// themeDetailMCPResponse is the MCP get tool output. The Theme config field is
// typed as map[string]interface{} so the schema generator emits {"type":"object"}
// instead of the array-of-bytes schema that json.RawMessage would produce, or
// the empty schema {} that interface{} would produce (both rejected by Claude).
type themeDetailMCPResponse struct {
	ID          string                 `json:"id"`
	Handle      string                 `json:"handle"`
	DisplayName string                 `json:"displayName"`
	Description string                 `json:"description,omitempty"`
	Theme       map[string]interface{} `json:"theme,omitempty"`
	IsReadOnly  bool                   `json:"isReadOnly"`
	CreatedAt   string                 `json:"createdAt,omitempty"`
	UpdatedAt   string                 `json:"updatedAt,omitempty"`
}

// registerMCPTools registers all theme MCP tools with the server.
func registerMCPTools(server *mcp.Server, themeMgtService ThemeMgtServiceInterface) {
	tools := &themeTools{themeMgtService: themeMgtService}

	mcp.AddTool(server, &mcp.Tool{
		Name: "thunderid_list_themes",
		Description: `List all themes with summary information (ID, handle, display name). ` +
			`Use thunderid_get_theme_by_id to retrieve full color and typography configuration for a specific theme.`,
		Annotations: &mcp.ToolAnnotations{
			Title:        "List Themes",
			ReadOnlyHint: true,
		},
	}, tools.listThemes)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "thunderid_get_theme_by_id",
		Description: `Retrieve full details of a theme by ID including color schemes and typography configuration.`,
		Annotations: &mcp.ToolAnnotations{
			Title:        "Get Theme by ID",
			ReadOnlyHint: true,
		},
	}, tools.getThemeByID)
}

// listThemes handles the list_themes tool call.
func (t *themeTools) listThemes(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	_ any,
) (*mcp.CallToolResult, *themeListMCPResponse, error) {
	var summaries []themeSummary
	var totalResults int
	offset := 0

	for {
		resp, svcErr := t.themeMgtService.GetThemeList(ctx, serverconst.MaxPageSize, offset)
		if svcErr != nil {
			return nil, nil, fmt.Errorf("failed to list themes: %s", svcErr.ErrorDescription)
		}
		if offset == 0 {
			totalResults = resp.TotalResults
		}
		for _, th := range resp.Themes {
			summaries = append(summaries, themeSummary{
				ID:          th.ID,
				Handle:      th.Handle,
				DisplayName: th.DisplayName,
				Description: th.Description,
				IsReadOnly:  th.IsReadOnly,
				CreatedAt:   th.CreatedAt,
				UpdatedAt:   th.UpdatedAt,
			})
		}
		offset += len(resp.Themes)
		if offset >= totalResults || len(resp.Themes) == 0 {
			break
		}
	}

	return nil, &themeListMCPResponse{
		TotalResults: totalResults,
		Themes:       summaries,
	}, nil
}

// getThemeByID handles the get_theme_by_id tool call.
func (t *themeTools) getThemeByID(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input tool.IDInput,
) (*mcp.CallToolResult, *themeDetailMCPResponse, error) {
	theme, svcErr := t.themeMgtService.GetTheme(ctx, input.ID)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to get theme: %s", svcErr.ErrorDescription)
	}

	detail := &themeDetailMCPResponse{
		ID:          theme.ID,
		Handle:      theme.Handle,
		DisplayName: theme.DisplayName,
		Description: theme.Description,
		IsReadOnly:  theme.IsReadOnly,
		CreatedAt:   theme.CreatedAt,
		UpdatedAt:   theme.UpdatedAt,
	}

	if len(theme.Theme) > 0 {
		var themeConfig map[string]interface{}
		if err := json.Unmarshal(theme.Theme, &themeConfig); err != nil {
			return nil, nil, fmt.Errorf("failed to parse theme config: %w", err)
		}
		detail.Theme = themeConfig
	}

	return nil, detail, nil
}
