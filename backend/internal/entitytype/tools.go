// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entitytype

import (
	"context"
	"fmt"
	"slices"
	"strings"

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

// agentTypeMCPItem is an agent type in the list_agent_types output, with its attributes.
type agentTypeMCPItem struct {
	ID               string                      `json:"id"`
	Handle           string                      `json:"handle"`
	OUID             string                      `json:"ouId"`
	DisplayAttribute string                      `json:"displayAttribute,omitempty"`
	Attributes       []agentTypeAttributeMCPItem `json:"attributes"`
}

// agentTypeAttributeMCPItem is one attribute of an agent type.
type agentTypeAttributeMCPItem struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"displayName,omitempty"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Unique      bool     `json:"unique"`
	Credential  bool     `json:"credential"`
	Enum        []string `json:"enum,omitempty"`
}

// agentTypeListMCPResponse is the list_agent_types output.
type agentTypeListMCPResponse struct {
	Types []agentTypeMCPItem `json:"types"`
}

// registerMCPTools registers all entity type MCP tools with the server.
func registerMCPTools(server *mcp.Server, entityTypeService EntityTypeServiceInterface) {
	tools := &entityTypeTools{entityTypeService: entityTypeService}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_user_types",
		Description: `List all user types.`,
		Annotations: &mcp.ToolAnnotations{
			Title:        "List User Types",
			ReadOnlyHint: true,
		},
	}, tools.listUserTypes)

	mcp.AddTool(server, &mcp.Tool{
		Name: "list_agent_types",
		Description: `List the agent type and its attributes: which are required, unique or credentials, and ` +
			`the values an attribute allows. Call it before creating an agent to ask the administrator for the ` +
			`attributes an agent needs. An agent whose type requires a credential attribute has to be created ` +
			`in the Console, since no credential goes through the MCP tools.`,
		Annotations: &mcp.ToolAnnotations{
			Title:        "List Agent Types",
			ReadOnlyHint: true,
		},
	}, tools.listAgentTypes)
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

// listAgentTypes handles the list_agent_types tool call. The list is read first because it checks the
// caller may read agent types, which reading attributes doesn't.
func (t *entityTypeTools) listAgentTypes(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	_ any,
) (*mcp.CallToolResult, *agentTypeListMCPResponse, error) {
	resp, svcErr := t.entityTypeService.GetEntityTypeList(ctx, TypeCategoryAgent, serverconst.MaxPageSize, 0, false)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to list agent types: %s", svcErr.ErrorDescription)
	}

	items := make([]agentTypeMCPItem, 0, len(resp.Types))
	for _, et := range resp.Types {
		attributes, attrErr := t.entityTypeService.GetAttributes(ctx, TypeCategoryAgent, et.Handle,
			AttributeFilter{AllowCredential: true, AllowNonCredential: true})
		if attrErr != nil {
			return nil, nil, fmt.Errorf("failed to list agent types: %s", attrErr.ErrorDescription)
		}
		slices.SortFunc(attributes, func(a, b AttributeInfo) int {
			return strings.Compare(a.Attribute, b.Attribute)
		})

		item := agentTypeMCPItem{
			ID:         et.ID,
			Handle:     et.Handle,
			OUID:       et.OUID,
			Attributes: make([]agentTypeAttributeMCPItem, 0, len(attributes)),
		}
		if et.SystemAttributes != nil {
			item.DisplayAttribute = et.SystemAttributes.Display
		}
		for _, attribute := range attributes {
			item.Attributes = append(item.Attributes, agentTypeAttributeMCPItem{
				Name:        attribute.Attribute,
				DisplayName: attribute.DisplayName,
				Type:        attribute.Type,
				Required:    attribute.Required,
				Unique:      attribute.Unique,
				Credential:  attribute.Credential,
				Enum:        attribute.Enum,
			})
		}
		items = append(items, item)
	}

	return nil, &agentTypeListMCPResponse{Types: items}, nil
}
