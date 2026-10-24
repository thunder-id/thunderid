// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package connection

import (
	"context"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/thunder-id/thunderid/internal/connection/httpconnection"
	"github.com/thunder-id/thunderid/internal/system/mcp/tool"
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	"github.com/thunder-id/thunderid/internal/system/outboundauth/httpauth"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// httpConnectionTools provides MCP tools for managing HTTP endpoint connections. They go through the
// same facade and request mapping as the /connections/http handlers, so validation, secret
// retention and masking are the API's.
type httpConnectionTools struct {
	svc *service
}

// httpConnectionMCPHeader is a plain header the connection sends on every call.
//
//nolint:lll
type httpConnectionMCPHeader struct {
	Name  string `json:"name" jsonschema:"Header name. Names that carry credentials (Authorization, Proxy-Authorization, Cookie, X-Api-Key, Api-Key, X-Auth-Token, X-Access-Token), headers the HTTP client manages (such as Host and Content-Type) and any header the api_key authentication sets are refused, in any case."`
	Value string `json:"value" jsonschema:"Header value. Sent in plain text and returned in plain text, so never put a credential here."`
}

// httpConnectionMCPAuthentication is the credential the connection presents.
//
//nolint:lll
type httpConnectionMCPAuthentication struct {
	Type       string            `json:"type" jsonschema:"Authentication method. none sends no credential; basic sends HTTP Basic authentication; bearer sends Authorization: Bearer with the token; api_key sends each configured header."`
	Properties map[string]string `json:"properties,omitempty" jsonschema:"Method fields. basic: username and password. bearer: token. api_key: one entry per header, the header name as the key and its value as the value. Secrets (password, token and every api_key value) are returned masked as ******; api_key header names are returned in clear."`
}

// httpConnectionMCPInput is the create and update tool input. ID is required on update and removed
// from the create schema.
//
//nolint:lll
type httpConnectionMCPInput struct {
	ID             string                           `json:"id,omitempty" jsonschema:"ID of the connection to update"`
	Name           string                           `json:"name" jsonschema:"Display name, unique among HTTP connections, at most 255 characters"`
	Description    string                           `json:"description,omitempty" jsonschema:"Optional description, at most 500 characters"`
	URL            string                           `json:"url" jsonschema:"Absolute http or https URL of the service. Must be https when authentication is not none."`
	TimeoutMS      int                              `json:"timeoutMs,omitempty" jsonschema:"Request timeout in milliseconds, 1 to 20000. Defaults to 10000 when omitted."`
	Headers        []httpConnectionMCPHeader        `json:"headers,omitempty" jsonschema:"Plain headers sent on every call. Names must be unique."`
	Authentication *httpConnectionMCPAuthentication `json:"authentication,omitempty" jsonschema:"Credential presented to the service. Omit for none."`
}

// httpConnectionMCPOutput is a connection as the tools return it, with every secret masked.
type httpConnectionMCPOutput struct {
	ID             string                          `json:"id"`
	Name           string                          `json:"name"`
	Description    string                          `json:"description,omitempty"`
	Type           string                          `json:"type"`
	URL            string                          `json:"url"`
	TimeoutMS      int                             `json:"timeoutMs"`
	Headers        []httpConnectionMCPHeader       `json:"headers"`
	Authentication httpConnectionMCPAuthentication `json:"authentication"`
}

// httpConnectionMCPItem is the list tool item.
type httpConnectionMCPItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// httpConnectionListMCPResponse is the list tool output.
type httpConnectionListMCPResponse struct {
	TotalResults int                     `json:"totalResults"`
	Connections  []httpConnectionMCPItem `json:"connections"`
}

// registerMCPTools registers the HTTP endpoint connection MCP tools with the server.
func registerMCPTools(server *mcp.Server, svc *service) {
	tools := &httpConnectionTools{svc: svc}

	mcp.AddTool(server, &mcp.Tool{
		Name: "thunderid_list_http_connections",
		Description: `List HTTP connections (Service Endpoint connections in the console). A Service Endpoint ` +
			`connection holds the URL, timeout, headers and credential an extension step in a flow uses to ` +
			`call an HTTP service.`,
		Annotations: &mcp.ToolAnnotations{
			Title:        "List HTTP Connections",
			ReadOnlyHint: true,
		},
	}, tools.listHTTPConnections)

	mcp.AddTool(server, &mcp.Tool{
		Name: "thunderid_get_http_connection_by_id",
		Description: `Retrieve a Service Endpoint connection by ID: its URL, timeout, headers and ` +
			`authentication. Every secret (password, token, api_key header value) is returned masked as ******.`,
		Annotations: &mcp.ToolAnnotations{
			Title:        "Get HTTP Connection by ID",
			ReadOnlyHint: true,
		},
	}, tools.getHTTPConnectionByID)

	mcp.AddTool(server, &mcp.Tool{
		Name: "thunderid_create_http_connection",
		Description: `Create a Service Endpoint connection: the URL, timeout, headers and credential an ` +
			`extension step in a flow uses to call an HTTP service.

Rules:
- The URL must be https when authentication is basic, bearer or api_key. Plain http is accepted only ` +
			`with authentication none.
- Put credentials in authentication, never in headers. Credential-carrying and HTTP-managed header names are refused.
- Secrets are stored encrypted and are never returned; responses show them as ******. Never send ****** as a secret.`,
		InputSchema: getCreateHTTPConnectionSchema(),
		Annotations: &mcp.ToolAnnotations{
			Title:          "Create HTTP Connection",
			IdempotentHint: false,
		},
	}, tools.createHTTPConnection)

	mcp.AddTool(server, &mcp.Tool{
		Name: "thunderid_update_http_connection",
		Description: `Update a Service Endpoint connection (full replacement).

Workflow:
1. Use thunderid_get_http_connection_by_id to get the current state.
2. Change the fields you need and send the complete object back without its type field. Any field not provided is reset.

Secrets:
- Omit the password, token or API key value to keep the stored one. It is kept only while the ` +
			`authentication type, the username and the URL origin (scheme, host and port) are unchanged; ` +
			`otherwise the update must supply it again.
- Sending ****** as the password or token is refused: remove the field from the object you read instead.
- For api_key, send every header to keep. A header sent with the value ****** keeps its stored value ` +
			`under the same rule; a header left out is removed.`,
		InputSchema: getUpdateHTTPConnectionSchema(),
		Annotations: &mcp.ToolAnnotations{
			Title:          "Update HTTP Connection",
			IdempotentHint: true,
		},
	}, tools.updateHTTPConnection)
}

// listHTTPConnections handles the list_http_connections tool call.
func (t *httpConnectionTools) listHTTPConnections(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	_ any,
) (*mcp.CallToolResult, *httpConnectionListMCPResponse, error) {
	connections, svcErr := t.svc.listHTTP(ctx)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to list HTTP connections: %s", svcErr.ErrorDescription)
	}

	items := make([]httpConnectionMCPItem, len(connections))
	for i, connection := range connections {
		items[i] = httpConnectionMCPItem{
			ID:          connection.ID,
			Name:        connection.Name,
			Description: connection.Description,
		}
	}

	return nil, &httpConnectionListMCPResponse{
		TotalResults: len(items),
		Connections:  items,
	}, nil
}

// getHTTPConnectionByID handles the get_http_connection_by_id tool call.
func (t *httpConnectionTools) getHTTPConnectionByID(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input tool.IDInput,
) (*mcp.CallToolResult, *httpConnectionMCPOutput, error) {
	connection, svcErr := t.svc.getHTTP(ctx, input.ID)
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to get HTTP connection: %s", svcErr.ErrorDescription)
	}

	return httpConnectionToolOutput(connection, "failed to get HTTP connection")
}

// createHTTPConnection handles the create_http_connection tool call.
func (t *httpConnectionTools) createHTTPConnection(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input httpConnectionMCPInput,
) (*mcp.CallToolResult, *httpConnectionMCPOutput, error) {
	dto, svcErr := httpMCPInputToDTO(input, httpToDTO)
	if svcErr == nil {
		dto, svcErr = t.svc.createHTTP(ctx, *dto)
	}
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to create HTTP connection: %s", svcErr.ErrorDescription)
	}

	return httpConnectionToolOutput(dto, "failed to create HTTP connection")
}

// updateHTTPConnection handles the update_http_connection tool call with complete replacement.
func (t *httpConnectionTools) updateHTTPConnection(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input httpConnectionMCPInput,
) (*mcp.CallToolResult, *httpConnectionMCPOutput, error) {
	dto, svcErr := httpMCPInputToDTO(input, httpUpdateToDTO)
	if svcErr == nil {
		dto, svcErr = t.svc.updateHTTP(ctx, input.ID, *dto)
	}
	if svcErr != nil {
		return nil, nil, fmt.Errorf("failed to update HTTP connection: %s", svcErr.ErrorDescription)
	}

	return httpConnectionToolOutput(dto, "failed to update HTTP connection")
}

// httpMCPInputToDTO maps a tool input onto the API request and through the API's create or update
// mapping, so a tool call is mapped and rejected exactly as the REST request is.
func httpMCPInputToDTO(input httpConnectionMCPInput,
	mapRequest func(httpConnectionRequest) (*httpconnection.HTTPConnection, error)) (
	*httpconnection.HTTPConnection, *tidcommon.ServiceError) {
	req := httpConnectionRequest{
		Name:        input.Name,
		Description: input.Description,
		URL:         input.URL,
		TimeoutMS:   input.TimeoutMS,
	}
	for _, header := range input.Headers {
		req.Headers = append(req.Headers, httpconnection.Header{Name: header.Name, Value: header.Value})
	}
	if input.Authentication != nil {
		req.Authentication = &outboundauth.Authentication{
			Type:       input.Authentication.Type,
			Properties: input.Authentication.Properties,
		}
	}

	dto, err := mapRequest(req)
	if err != nil {
		return nil, senderMappingServiceError(err)
	}
	return dto, nil
}

// httpConnectionToolOutput renders a connection through httpFromDTO, which masks every secret.
func httpConnectionToolOutput(connection *httpconnection.HTTPConnection, failure string) (
	*mcp.CallToolResult, *httpConnectionMCPOutput, error) {
	resp, err := httpFromDTO(*connection)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %s", failure, tidcommon.InternalServerError.ErrorDescription)
	}

	headers := make([]httpConnectionMCPHeader, len(resp.Headers))
	for i, header := range resp.Headers {
		headers[i] = httpConnectionMCPHeader{Name: header.Name, Value: header.Value}
	}
	return nil, &httpConnectionMCPOutput{
		ID:          resp.ID,
		Name:        resp.Name,
		Description: resp.Description,
		Type:        resp.Type,
		URL:         resp.URL,
		TimeoutMS:   resp.TimeoutMS,
		Headers:     headers,
		Authentication: httpConnectionMCPAuthentication{
			Type:       resp.Authentication.Type,
			Properties: resp.Authentication.Properties,
		},
	}, nil
}

// getHTTPConnectionSchemaModifiers limits the authentication type to the methods HTTP connections
// support.
func getHTTPConnectionSchemaModifiers() []func(*jsonschema.Schema) {
	supported := httpauth.SupportedTypes()
	authTypes := make([]string, len(supported))
	for i, authType := range supported {
		authTypes[i] = string(authType)
	}
	return []func(*jsonschema.Schema){tool.WithEnum("authentication", "type", authTypes)}
}

// getCreateHTTPConnectionSchema generates the schema for the create_http_connection tool.
func getCreateHTTPConnectionSchema() *jsonschema.Schema {
	modifiers := append(getHTTPConnectionSchemaModifiers(), tool.WithRemove("", "id"))
	return tool.GenerateSchema[httpConnectionMCPInput](modifiers...)
}

// getUpdateHTTPConnectionSchema generates the schema for the update_http_connection tool.
func getUpdateHTTPConnectionSchema() *jsonschema.Schema {
	modifiers := append(getHTTPConnectionSchemaModifiers(), tool.WithRequired("", "id"))
	return tool.GenerateSchema[httpConnectionMCPInput](modifiers...)
}
