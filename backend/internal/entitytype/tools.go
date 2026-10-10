// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
	Handle                string            `json:"handle,omitempty"`
	DisplayName           string            `json:"displayName,omitempty"`
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
			Handle:                et.Handle,
			DisplayName:           et.DisplayName,
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
