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

package entitytype

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
)

type entityTypeTools struct {
	entityTypeService EntityTypeServiceInterface
}

// entityTypeMCPItem is the MCP list-response item. HTTP pagination fields
// (StartIndex, Count, Links) and the raw Schema blob are excluded.
type entityTypeMCPItem struct {
	ID                    string            `json:"id,omitempty"`
	Name                  string            `json:"name,omitempty"`
	OUID                  string            `json:"ouId"`
	OUHandle              string            `json:"ouHandle,omitempty"`
	AllowSelfRegistration bool              `json:"allowSelfRegistration"`
	SystemAttributes      *SystemAttributes `json:"systemAttributes,omitempty"`
	IsReadOnly            bool              `json:"isReadOnly"`
}

// entityTypeListMCPResponse is the MCP list tool output.
type entityTypeListMCPResponse struct {
	TotalResults int                 `json:"totalResults"`
	Types        []entityTypeMCPItem `json:"types"`
}

// registerMCPTools registers all entity type MCP tools with the server.
func registerMCPTools(server *mcp.Server, entityTypeService EntityTypeServiceInterface) {
	tools := &entityTypeTools{entityTypeService: entityTypeService}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "thunderid_list_user_types",
		Description: `List all user types.`,
		Annotations: &mcp.ToolAnnotations{
			Title:        "List User Types",
			ReadOnlyHint: true,
		},
	}, tools.listUserTypes)
}

// listUserTypes handles the list_user_types tool call.
func (t *entityTypeTools) listUserTypes(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	_ any,
) (*mcp.CallToolResult, *entityTypeListMCPResponse, error) {
	resp, svcErr := t.entityTypeService.GetEntityTypeList(ctx, TypeCategoryUser, serverconst.MaxPageSize, 0, false)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to list user types: %s", svcErr.ErrorDescription)
	}

	items := make([]entityTypeMCPItem, len(resp.Types))
	for i, et := range resp.Types {
		items[i] = entityTypeMCPItem{
			ID:                    et.ID,
			Name:                  et.Name,
			OUID:                  et.OUID,
			OUHandle:              et.OUHandle,
			AllowSelfRegistration: et.AllowSelfRegistration,
			SystemAttributes:      et.SystemAttributes,
			IsReadOnly:            et.IsReadOnly,
		}
	}

	return nil, &entityTypeListMCPResponse{
		TotalResults: resp.TotalResults,
		Types:        items,
	}, nil
}
