// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"strings"
	"unicode"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/thunder-id/thunderid/internal/system/security"
)

// newServer creates an MCP server that reports a name derived from productName to clients.
func newServer(productName string) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    serverName(productName),
		Version: "1.0.0",
	}, nil)
	server.AddReceivingMiddleware(toolPermissionMiddleware)
	return server
}

// serverName derives the MCP server name from the product name. Ex: "Acme IAM" becomes "acme-iam-mcp".
func serverName(productName string) string {
	words := strings.FieldsFunc(strings.ToLower(productName), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	return strings.Join(append(words, "mcp"), "-")
}

// toolPermissionMiddleware refuses a tool call unless the caller holds the permission the tool is
// mapped to, as the route table does for a REST call.
func toolPermissionMiddleware(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		if call, ok := req.(*mcp.CallToolRequest); ok {
			required := security.ResolveMCPToolPermission(call.Params.Name)
			if !security.HasSufficientPermission(security.GetPermissions(ctx), required) {
				return &mcp.CallToolResult{
					IsError: true,
					Content: []mcp.Content{&mcp.TextContent{
						Text: "The caller is not authorized to call this tool",
					}},
				}, nil
			}
		}
		return next(ctx, method, req)
	}
}
