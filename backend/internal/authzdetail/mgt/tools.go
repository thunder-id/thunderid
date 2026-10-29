// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzdetailmgt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/thunder-id/thunderid/internal/system/mcp/tool"
)

// typeTools provides MCP tools for managing authorization details types.
type typeTools struct {
	service AuthorizationDetailMgtServiceInterface
}

// typeMCPItem is the MCP list-response item.
type typeMCPItem struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Version     int    `json:"version"`
	Reuse       bool   `json:"reuse"`
	IsReadOnly  bool   `json:"isReadOnly"`
}

// typeListMCPResponse is the MCP list tool output.
type typeListMCPResponse struct {
	TotalResults int           `json:"totalResults"`
	Types        []typeMCPItem `json:"types"`
}

// updateTypeInput is the input of the update tool.
type updateTypeInput struct {
	ID string `json:"id" jsonschema:"The unique identifier of the authorization details type to update."`
	AuthorizationDetailTypeRequest
}

// registerMCPTools registers the authorization details type tools with the MCP server.
func registerMCPTools(server *mcp.Server, service AuthorizationDetailMgtServiceInterface) {
	tools := &typeTools{service: service}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_authorization_detail_types",
		Description: `List the registered RFC 9396 authorization details types.`,
		Annotations: &mcp.ToolAnnotations{
			Title:        "List Authorization Detail Types",
			ReadOnlyHint: true,
		},
	}, tools.listTypes)

	server.AddTool(&mcp.Tool{
		Name: "get_authorization_detail_type_by_id",
		Description: `Retrieve the complete definition of an authorization details type by its unique ID, ` +
			`including its schema with field policies, consent template, processor and reuse setting.`,
		InputSchema: tool.GenerateSchema[tool.IDInput](tool.WithRequired("", "id")),
		OutputSchema: tool.GenerateSchema[AuthorizationDetailTypeResponse](
			withSchemaObject("The JSON Schema 2020-12 of the type, with a field policy on " +
				"each property that may be supplied by the processor or change after the request.")),
		Annotations: &mcp.ToolAnnotations{
			Title:        "Get Authorization Detail Type",
			ReadOnlyHint: true,
		},
	}, rawTool(tools.getType))

	server.AddTool(&mcp.Tool{
		Name: "create_authorization_detail_type",
		Description: `Register an RFC 9396 authorization details type that clients can request in ` +
			`authorization_details.

Key Requirements:
- type: The value clients send, unique in the deployment; often a URI.
- schema: A JSON Schema 2020-12 object schema. Objects are closed to undeclared fields. A property may ` +
			`carry a "policy": {"suppliedBy": "client" | "processor", "modification": "none" | "decrease" | ` +
			`"increase" | "removeItems" | "addItems", "modifiableBy": ["user", "client", "processor"]}. A ` +
			`property without a policy is supplied by the client and cannot change.
- consentTemplate: The consent summary, with {{field.path}} placeholders.
- processor: Required when any field is supplied or changed by the processor.
- reuse: Whether an approval may stand in for later prompts and be renewed by refresh (default false).

Use get_authorization_detail_type_by_id on an existing type to see a complete example.`,
		InputSchema: tool.GenerateSchema[AuthorizationDetailTypeRequest](
			withSchemaObject("The JSON Schema 2020-12 object schema of the type, with field "+
				"policies."),
			tool.WithRequired("", "type", "name", "schema")),
		OutputSchema: tool.GenerateSchema[AuthorizationDetailTypeResponse](
			withSchemaObject("The JSON Schema 2020-12 of the type.")),
		Annotations: &mcp.ToolAnnotations{
			Title:          "Create Authorization Detail Type",
			IdempotentHint: false,
		},
	}, rawTool(tools.createType, readOnlyFields...))

	server.AddTool(&mcp.Tool{
		Name: "update_authorization_detail_type",
		Description: `Update an authorization details type (full replacement of its definition). The type ` +
			`value cannot change.

Use get_authorization_detail_type_by_id first to get the current definition, change it, and send ` +
			`the complete definition back. The version increments when the schema or consent template changes, ` +
			`or the processor is added or removed, and earlier approvals then no longer cover new requests.`,
		InputSchema: tool.GenerateSchema[updateTypeInput](
			withSchemaObject("The JSON Schema 2020-12 object schema of the type, with field "+
				"policies."),
			tool.WithRequired("", "id", "type", "name", "schema")),
		OutputSchema: tool.GenerateSchema[AuthorizationDetailTypeResponse](
			withSchemaObject("The JSON Schema 2020-12 of the type.")),
		Annotations: &mcp.ToolAnnotations{
			Title:          "Update Authorization Detail Type",
			IdempotentHint: true,
		},
	}, rawTool(tools.updateType, "version", "isReadOnly"))
}

// readOnlyFields are the fields the get tool returns that a create ignores, so a fetched definition can be sent
// to it.
var readOnlyFields = []string{"id", "version", "isReadOnly"}

// rawTool adapts a handler to raw JSON input and output so a schema keeps its property order. The ignored
// fields are dropped before the input is decoded, so a definition the get tool returned can be sent back.
func rawTool[In any](handle func(context.Context, In) (*AuthorizationDetailTypeResponse, error),
	ignored ...string) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var input In
		if len(req.Params.Arguments) > 0 {
			arguments, err := withoutFields(req.Params.Arguments, ignored)
			if err != nil {
				return toolError(fmt.Errorf("invalid arguments: %w", err)), nil
			}
			// A misspelled key, such as one for the processor, is refused rather than dropped.
			decoder := json.NewDecoder(bytes.NewReader(arguments))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&input); err != nil {
				return toolError(fmt.Errorf("invalid arguments: %w", err)), nil
			}
		}
		output, err := handle(ctx, input)
		if err != nil {
			return toolError(err), nil
		}
		encoded, err := json.Marshal(output)
		if err != nil {
			return nil, fmt.Errorf("marshaling output: %w", err)
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(encoded)}},
			StructuredContent: json.RawMessage(encoded)}, nil
	}
}

// toolError reports a failed tool call.
func toolError(err error) *mcp.CallToolResult {
	var result mcp.CallToolResult
	result.SetError(err)
	return &result
}

// withSchemaObject describes the raw JSON schema property as an object instead of a byte array.
func withSchemaObject(description string) func(*jsonschema.Schema) {
	return func(root *jsonschema.Schema) {
		if root.Properties != nil {
			root.Properties["schema"] = &jsonschema.Schema{Type: "object", Description: description}
		}
	}
}

// listTypes handles the list_authorization_detail_types tool call.
func (t *typeTools) listTypes(ctx context.Context, _ *mcp.CallToolRequest,
	_ any) (*mcp.CallToolResult, *typeListMCPResponse, error) {
	types, svcErr := t.service.ListAllAuthorizationDetailTypes(ctx)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to list authorization details types: %s", svcErr.ErrorDescription)
	}
	items := make([]typeMCPItem, 0, len(types))
	for _, typ := range types {
		items = append(items, typeMCPItem{ID: typ.ID, Type: typ.Type, Name: typ.Name,
			Description: typ.Description, Version: typ.Version, Reuse: typ.Reuse, IsReadOnly: typ.IsReadOnly})
	}
	return nil, &typeListMCPResponse{TotalResults: len(items), Types: items}, nil
}

// getType handles the get_authorization_detail_type tool call.
func (t *typeTools) getType(ctx context.Context, input tool.IDInput) (*AuthorizationDetailTypeResponse, error) {
	typ, svcErr := t.service.GetAuthorizationDetailType(ctx, input.ID)
	if svcErr != nil {
		return nil, fmt.Errorf("failed to get authorization details type: %s", svcErr.ErrorDescription)
	}
	resp := toTypeResponse(*typ)
	return &resp, nil
}

// createType handles the create_authorization_detail_type tool call.
func (t *typeTools) createType(ctx context.Context,
	input AuthorizationDetailTypeRequest) (*AuthorizationDetailTypeResponse, error) {
	typ, svcErr := toType(&input)
	if svcErr != nil {
		return nil, fmt.Errorf("failed to create authorization details type: %s", svcErr.ErrorDescription)
	}
	created, svcErr := t.service.CreateAuthorizationDetailType(ctx, typ)
	if svcErr != nil {
		return nil, fmt.Errorf("failed to create authorization details type: %s", svcErr.ErrorDescription)
	}
	resp := toTypeResponse(*created)
	return &resp, nil
}

// updateType handles the update_authorization_detail_type tool call.
func (t *typeTools) updateType(ctx context.Context, input updateTypeInput) (*AuthorizationDetailTypeResponse, error) {
	typ, svcErr := toType(&input.AuthorizationDetailTypeRequest)
	if svcErr != nil {
		return nil, fmt.Errorf("failed to update authorization details type: %s", svcErr.ErrorDescription)
	}
	updated, svcErr := t.service.UpdateAuthorizationDetailType(ctx, input.ID, typ)
	if svcErr != nil {
		return nil, fmt.Errorf("failed to update authorization details type: %s", svcErr.ErrorDescription)
	}
	resp := toTypeResponse(*updated)
	return &resp, nil
}

// withoutFields returns a JSON object without the given members, or the object unchanged when there are none
// to drop.
func withoutFields(raw json.RawMessage, fields []string) (json.RawMessage, error) {
	if len(fields) == 0 {
		return raw, nil
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(raw, &members); err != nil {
		return nil, err
	}
	for _, field := range fields {
		delete(members, field)
	}
	return json.Marshal(members)
}
