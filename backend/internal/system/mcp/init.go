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

// Package mcp provides MCP (Model Context Protocol) server functionality.
package mcp

import (
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/auth"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/log"
	mcpauth "github.com/thunder-id/thunderid/internal/system/mcp/auth"
	"github.com/thunder-id/thunderid/internal/system/security"
)

// Initialize initializes the MCP server and registers its routes with the provided mux.
func Initialize(
	mux *http.ServeMux,
	jwtService jwt.JWTServiceInterface,
) *mcpsdk.Server {
	cfg := config.GetServerRuntime().Config
	baseURL := config.GetServerURL(&cfg.Server)

	mcpURL := baseURL + MCPEndpointPath
	resourceMetadataURL := baseURL + OAuthProtectedResourceMetadataPath

	// Create MCP server and register standalone tools
	mcpServer := newServer()

	sysPerm := security.GetSystemPermissions()
	if sysPerm == nil {
		log.GetLogger().Fatal("System permissions not initialized before MCP initialization")
	}
	rootPerm := sysPerm.Root

	tokenVerifier := mcpauth.NewTokenVerifier(jwtService, cfg.JWT.Issuer, mcpURL)
	httpHandler := mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server {
		return mcpServer
	}, nil)

	// Secure MCP handler with bearer token authentication
	securedHandler := auth.RequireBearerToken(tokenVerifier, &auth.RequireBearerTokenOptions{
		ResourceMetadataURL: resourceMetadataURL,
		Scopes:              []string{rootPerm},
	})(httpHandler)

	// Register protected resource metadata endpoint
	metadata := &oauthex.ProtectedResourceMetadata{
		Resource:             mcpURL,
		AuthorizationServers: []string{cfg.JWT.Issuer},
		ScopesSupported:      []string{rootPerm},
	}
	mux.Handle(OAuthProtectedResourceMetadataPath, auth.ProtectedResourceMetadataHandler(metadata))

	// Register MCP routes
	mux.Handle(MCPEndpointPath, securedHandler)
	mux.Handle(MCPEndpointPath+"/", securedHandler)

	return mcpServer
}
