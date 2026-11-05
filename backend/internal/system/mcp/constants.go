// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package mcp

const (
	// MCPEndpointPath is the base path for the MCP server endpoint.
	MCPEndpointPath = "/mcp"

	// OAuthProtectedResourceMetadataPath is the path for the OAuth protected resource metadata endpoint.
	OAuthProtectedResourceMetadataPath = "/.well-known/oauth-protected-resource"

	// errorInsufficientScope is the RFC 6750 error code for a token that lacks the scope a request needs.
	errorInsufficientScope = "insufficient_scope"
)
